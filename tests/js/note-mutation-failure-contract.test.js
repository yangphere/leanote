const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');

const ROOT = path.resolve(__dirname, '../..');
const SOURCE = fs.readFileSync(
  path.join(ROOT, 'public/js/app/note.js'),
  'utf8',
);
const MINIFIED = fs.readFileSync(path.join(ROOT, 'public/js/app.min.js'), 'utf8');

function fakeTarget(noteId = 'note-1') {
  return {
    visible: true,
    removed: false,
    attr(name) {
      return name === 'noteId' ? noteId : undefined;
    },
    hasClass(name) {
      return name === 'item-active';
    },
    hide() {
      this.visible = false;
    },
    show() {
      this.visible = true;
    },
    remove() {
      this.removed = true;
    },
  };
}

function loadMutation(name, ajaxResult, { deferred = false, bundle = false } = {}) {
  const target = fakeTarget();
  const messages = [];
  let respond;
  const calls = {
    clearCurrent: 0,
    clearInfo: 0,
    stopInterval: 0,
    resetBatch: 0,
    nextNote: 0,
    cacheMutation: 0,
  };
  const note = {
    curNoteId: 'note-1',
    cache: { 'note-1': { NoteId: 'note-1', NotebookId: 'book-1', IsTrash: false } },
    updatePoolNote() {},
    inBatch: false,
    batch: { reset: () => { calls.resetBatch += 1; } },
    $itemList: { find: () => target },
    getBatchNoteIds: () => ['note-1'],
    getTargetById: () => target,
    getNote: (noteId) => note.cache[noteId],
    stopInterval: () => { calls.stopInterval += 1; },
    clearCurNoteId: () => { calls.clearCurrent += 1; note.curNoteId = null; },
    clearNoteInfo: () => { calls.clearInfo += 1; },
    changeToNextSkipNotes: () => { calls.nextNote += 1; },
    clearCacheByNotebookId: () => { calls.cacheMutation += 1; },
    setNoteCache: (value) => { calls.cacheMutation += 1; Object.assign(note.cache[value.NoteId], value); },
  };
  const notebook = {
    getCurNotebookId: () => 'book-1',
    curActiveNotebookIsAll: () => false,
    minusNotebookNumberNotes: () => { calls.cacheMutation += 1; },
    incrNotebookNumberNotes: () => { calls.cacheMutation += 1; },
  };
  const window = { crypto: webcrypto };
  const jquery = (value) => value;
  jquery.param = JSON.stringify;
  jquery.extend = Object.assign;
  const context = {
    window,
    Note: note,
    Notebook: notebook,
    $: jquery,
    isEmpty: (value) => !value || value.length === 0,
    getMsg: (key) => `localized:${key}`,
    showMsg: (message) => messages.push(message),
    ajaxGet: (url, params, success) => success({ NotebookId: 'book-2', Usn: 12, Tags: [] }),
    ajaxPost: (url, params, success, failure) => {
      respond = () => {
        if (ajaxResult === 'transport-error') failure(new Error('network'));
        else success(ajaxResult);
      };
      if (!deferred) respond();
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(ROOT, 'public/js/mutation-intents.js'), 'utf8'), context);
  vm.runInNewContext(SOURCE.slice(SOURCE.indexOf('Note.savePool ='), SOURCE.indexOf('Note.mutations =')), context);
  note.mutations = window.LeanoteMutationIntents.create({
    crypto: webcrypto, serialize: JSON.stringify, send: context.ajaxPost, changed() {},
  });
  const submitStart = SOURCE.indexOf('Note.submitMutation = function');
  const submitEnd = SOURCE.indexOf('Note.submitSave = function', submitStart);
  vm.runInNewContext(SOURCE.slice(submitStart, submitEnd), context);
  const source = bundle ? MINIFIED : SOURCE;
  const start = source.indexOf(bundle ? `Note.${name}=function` : `Note.${name} = function`);
  const endMarker = bundle
    ? (name === 'deleteNote' ? 'Note.listNoteShareUserInfo' : 'Note.copyNote')
    : (name === 'deleteNote' ? '// 显示共享信息' : '// 复制');
  const end = source.indexOf(endMarker, start);
  assert.ok(start >= 0 && end > start);
  vm.runInNewContext(source.slice(start, end).replace(/,\s*$/, ';'), context);
  context.Note[name](target, name === 'moveNote' ? { notebookId: 'book-2' } : null, false);
  return { target, messages, calls, note, respond };
}

for (const bundle of [false, true]) {
  const label = bundle ? 'generated bundle' : 'source';
  test(`${label}: delete success clears only the deleted current note`, () => {
    const { target, note, calls, messages } = loadMutation('deleteNote', true, { bundle });
    assert.equal(target.removed, true);
    assert.equal(note.curNoteId, null);
    assert.equal(note.cache['note-1'], undefined);
    assert.equal(calls.clearCurrent, 1);
    assert.equal(calls.clearInfo, 1);
    assert.equal(calls.resetBatch, 1);
    assert.deepEqual(messages, []);
  });

  test(`${label}: delayed delete success preserves a newly opened note`, () => {
    const { target, note, calls, respond } = loadMutation('deleteNote', true, { deferred: true, bundle });
    assert.equal(note.curNoteId, 'note-1');
    assert.equal(target.removed, false);
    note.curNoteId = 'note-2';
    note.getBatchNoteIds = () => ['note-2'];
    respond();
    assert.equal(note.curNoteId, 'note-2');
    assert.equal(calls.clearInfo, 0);
    assert.equal(calls.stopInterval, 0);
    assert.equal(calls.resetBatch, 0);
    assert.equal(calls.nextNote, 0);
    assert.equal(target.removed, true);
    assert.equal(note.cache['note-1'], undefined);
  });

  test(`${label}: move success updates the requested note and list`, () => {
    const { target, note, calls, messages } = loadMutation('moveNote', true, { bundle });
    assert.equal(note.cache['note-1'].NotebookId, 'book-2');
    assert.equal(target.removed, true);
    assert.equal(calls.resetBatch, 1);
    assert.deepEqual(messages, []);
  });
}

test('delayed move preserves a selection made while awaiting confirmation', () => {
  for (const bundle of [false, true]) {
    const { target, note, calls, respond } = loadMutation('moveNote', true, { deferred: true, bundle });
    const laterSelection = fakeTarget('note-2');
    note.$itemList.find = () => laterSelection;
    note.getBatchNoteIds = () => ['note-2'];
    respond();
    assert.equal(target.removed, true);
    assert.equal(laterSelection.removed, false);
    assert.equal(calls.resetBatch, 0);
    assert.equal(calls.nextNote, 0);
  }
});


test('delete failure leaves the item and current selection visible', () => {
  for (const result of [false, { Ok: false, Msg: 'storage' }, 'transport-error']) {
    const { target, messages, calls } = loadMutation('deleteNote', result);
    assert.equal(target.visible, true, `delete result ${String(result)} should restore visibility`);
    assert.equal(target.removed, false);
    assert.equal(calls.clearCurrent, 0);
    assert.equal(calls.clearInfo, 0);
    assert.equal(calls.resetBatch, 0);
    assert.equal(messages.at(-1), result?.Msg ?? 'localized:Error');
  }
});

test('move failure leaves cache/list state unchanged and localizes transport errors', () => {
  for (const result of [false, { Ok: false, Msg: 'storage' }, 'transport-error']) {
    const { target, messages, calls } = loadMutation('moveNote', result);
    assert.equal(target.removed, false);
    assert.equal(calls.cacheMutation, 0);
    assert.equal(calls.resetBatch, 0);
    assert.equal(messages.at(-1), result?.Msg ?? 'localized:Error');
  }
});

test('generated app bundle contains the localized strict-result handlers', () => {
  const deleteStart = MINIFIED.indexOf('Note.deleteNote=function');
  const deleteEnd = MINIFIED.indexOf('Note.listNoteShareUserInfo', deleteStart);
  const moveStart = MINIFIED.indexOf('Note.moveNote=function');
  const moveEnd = MINIFIED.indexOf('Note.copyNote', moveStart);
  assert.ok(deleteStart >= 0 && deleteEnd > deleteStart);
  assert.ok(moveStart >= 0 && moveEnd > moveStart);
  for (const code of [MINIFIED.slice(deleteStart, deleteEnd), MINIFIED.slice(moveStart, moveEnd)]) {
    assert.match(code, /===!0/);
    assert.match(code, /getMsg\("Error"\)/);
    assert.doesNotMatch(code, /\?[^:]+:"Error"/);
  }
});
