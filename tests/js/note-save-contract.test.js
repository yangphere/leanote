const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');
const ROOT = path.resolve(__dirname, '../..');
function harness(realChanges = false) {
  const requests = [], reads = [], messages = [];
  let content = 'old';
  let title = 'old';
  const view = { val() { return title; }, toggle() { return this; }, text() { return this; }, on() { return this; } };
  const target = { attr: () => 'n1', hide() {}, show() {}, remove() {}, find() { return this; }, html() {}, eq() { return this; } };
  const $ = (value) => value === target ? target : view;
  $.extend = Object.assign; $.param = JSON.stringify;
  const Note = { curNoteId: 'n1', isReadOnly: false, saveInProcess: {},
    cache: { n1: { NoteId: 'n1', Usn: 7, Title: 'old', Content: 'old', Tags: [], NotebookId: 'book' } },
    getNote(id) { return this.cache[id]; }, getCurNote() { return this.cache[this.curNoteId]; },
    setNoteCache(note) { this.cache[note.NoteId] = Object.assign(this.cache[note.NoteId] || {}, note); },
    clearCacheByNotebookId() {}, renderChangedNote() {},
    curHasChanged() { return this.change && { ...this.change, NoteId: 'n1', hasChanged: true, NotebookId: 'book' }; },
  };
  const window = { crypto: webcrypto };
  const context = vm.createContext({ window, Note, $, getEditorContent: () => content, isArray: Array.isArray,
    Tag: { getTags: () => [] }, arrayEqual: (a, b) => JSON.stringify(a) === JSON.stringify(b),
    showMsg: (message) => messages.push(message), getMsg: (key) => key, log() {},
    reIsOk: (ret) => ret && ret.Ok === true, Pjax: { changeNote() {} },
    isEmpty: (value) => !value || value.length === 0,
    Notebook: { getCurNotebookId: () => '0', curActiveNotebookIsAll: () => true,
      getNotebookTitle: () => 'destination', incrNotebookNumberNotes() {}, minusNotebookNumberNotes() {} },
    ajaxGet: (url, body, success, failure) => reads.push({ url, body, success, failure }),
    ajaxPost: (url, body, success, failure) => requests.push({ url, body, success, failure }) });
  vm.runInContext('Date.prototype.format = function () { return "test-date"; };', context);
  for (const file of ['editor-state-source.js', 'mutation-intents.js'])
    vm.runInContext(fs.readFileSync(path.join(ROOT, 'public/js', file), 'utf8'), context);
  const source = fs.readFileSync(path.join(ROOT, 'public/js/app/note.js'), 'utf8');
  const start = source.indexOf('Note.savePool ='), end = source.indexOf('// 样式', start);
  assert.ok(start >= 0 && end > start);
  vm.runInContext(source.slice(start, end), context);
  for (const [first, last] of [['Note.moveNote = function', '// 删除笔记标签'], ['Note.deleteNote = function', '// 显示共享信息']]) {
    vm.runInContext(source.slice(source.indexOf(first), source.indexOf(last, source.indexOf(first))), context);
  }
  Object.assign(Note, { getBatchNoteIds: () => ['n1'], batch: { reset() {} }, stopInterval() {},
    clearCurNoteId() { this.curNoteId = null; }, clearNoteInfo() {}, changeToNextSkipNotes() {},
    changeNote() { assert.fail('move must preserve the open editor and its uncaptured draft'); } });
  context.tinymce = { activeEditor: { getContent: () => content } };
  context.setEditorContent = (value, markdown, preview, done, epoch) => {
    content = value;
    window.LeanoteEditorSession.setContentProgrammatically(value, epoch);
    done();
  };
  Note.setCurNoteId = (id) => { Note.curNoteId = id; };
  Note.toggleReadOnly = () => {};
  const renderStart = source.indexOf('Note.renderNoteContent = function');
  const renderEnd = source.indexOf('// 初始化时渲染最初的notes', renderStart);
  vm.runInContext(source.slice(renderStart, renderEnd), context);
  if (realChanges) {
    const changeStart = source.indexOf('Note.curHasChanged = function');
    const changeEnd = source.indexOf('// 由content生成desc', changeStart);
    vm.runInContext(source.slice(changeStart, changeEnd), context);
    Note.genDesc = Note.genAbstract = (value) => value;
    Note.getImgSrc = () => '';
  }
  window.LeanoteEditorSession.load({ noteId: 'n1', persistedContent: 'old', editorContent: 'old' });
  return { Note, requests, reads, messages, target, session: window.LeanoteEditorSession,
    deferRender() {
      const apply = context.setEditorContent, deferred = [];
      context.setEditorContent = (...args) => deferred.push(() => apply(...args));
      return () => { while (deferred.length) deferred.shift()(); };
    },
    title(value) { title = value; },
    unavailable() { content = undefined; },
    edit(value) { content = value; window.LeanoteEditorSession.markMutation(value); Note.change = { Content: value }; } };
}
test('metadata-only uses authoritative USN and rotates identity after confirmation', () => {
  const { Note, requests } = harness();
  Note.change = { Title: 'new', Tags: 'one,two' }; Note.curChangedSaveIt(true);
  const first = typeof requests[0].body === 'string' ? JSON.parse(requests[0].body) : requests[0].body;
  assert.equal(first.ExpectedUsn, 7); assert.match(first.OperationId, /^[a-f0-9]{32}$/);
  assert.equal(Object.hasOwn(first, 'Content'), false);
  requests[0].success({ Ok: true, Usn: 19 });
  assert.equal(Note.cache.n1.Usn, 19); assert.deepEqual(Array.from(Note.cache.n1.Tags), ['one', 'two']);
  assert.equal(Object.hasOwn(Note.cache.n1, 'OperationId'), false);
  Note.change = { Title: 'next' }; Note.curChangedSaveIt(true);
  const second = JSON.parse(requests[1].body);
  assert.equal(second.ExpectedUsn, 19); assert.notEqual(second.OperationId, first.OperationId);
});
test('unknown retry freezes content then sends queued edits with committed USN', () => {
  const { Note, requests, edit, session } = harness();
  edit('first'); Note.curChangedSaveIt(true); requests[0].failure({ status: 0 });
  edit('second'); Note.curChangedSaveIt(true);
  assert.equal(requests.length, 1); assert.equal(session.isDirty(), true);
  Note.mutations.retryUnknown(); assert.equal(requests[1].body, requests[0].body);
  requests[1].success({ Ok: true, Usn: 11 }); assert.equal(session.isDirty(), true);
  assert.equal(JSON.parse(requests[2].body).Content, 'second');
  assert.equal(JSON.parse(requests[2].body).ExpectedUsn, 11);
  requests[2].success({ Ok: true, Usn: 12 });
  assert.equal(session.isDirty(), false); assert.equal(Note.cache.n1.Content, 'second');
});
test('old note epoch updates cache without clearing the active editor', () => {
  const { Note, requests, edit, session } = harness();
  edit('first'); Note.curChangedSaveIt(true); Note.curNoteId = 'n2';
  session.load({ noteId: 'n2', persistedContent: 'other', editorContent: 'other' }); session.markMutation('unsaved');
  requests[0].success({ Ok: true, Usn: 9 });
  assert.equal(Note.cache.n1.Content, 'first'); assert.equal(Note.cache.n1.Usn, 9);
  assert.equal(session.isDirty(), true); assert.equal(session.snapshot().noteId, 'n2');
});
test('missing USN preserves edits and reports a revision error', () => {
  const { Note, requests, edit, session, messages } = harness();
  edit('changed'); delete Note.cache.n1.Usn; Note.curChangedSaveIt(true);
  assert.equal(requests.length, 0); assert.equal(session.isDirty(), true); assert.ok(messages.length);
  assert.equal(messages.at(-1), 'mutationRevisionRequired');
});
test('conflict preserves edits and blocks unsafe new saves', () => {
  const { Note, requests, edit, session } = harness();
  edit('changed'); Note.curChangedSaveIt(true); requests[0].success({ Ok: false, Msg: 'conflict' });
  Note.curChangedSaveIt(true); assert.equal(requests.length, 1); assert.equal(session.isDirty(), true);
});

function movedNote(usn = 12, extra = {}) {
  return { NotebookId: 'destination', Usn: usn, Title: 'old', Tags: [], IsTrash: false, Content: 'old', ...extra };
}

test('move in all-notes reloads authoritative USN and drains edits without manual cache repair', () => {
  const { Note, requests, reads, target, edit, session } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' });
  edit('during move'); Note.curChangedSaveIt(true);
  requests[0].success(true);
  assert.equal(reads.length, 1);
  assert.equal(reads[0].url, '/note/getNoteAndContent');
  edit('during reload'); Note.curChangedSaveIt(true);
  assert.equal(requests.length, 1);
  reads[0].success(movedNote());
  const saved = JSON.parse(requests[1].body);
  assert.equal(saved.ExpectedUsn, 12);
  assert.equal(saved.NotebookId, 'destination');
  assert.equal(saved.Content, 'during reload');
  requests[1].success({ Ok: true, Usn: 15 });
  assert.equal(session.isDirty(), false);
  edit('after move'); Note.curChangedSaveIt(true);
  assert.equal(JSON.parse(requests[2].body).ExpectedUsn, 15);
});

test('move preserves uncaptured edits and resumes after a failed authoritative load', () => {
  const { Note, requests, reads, target, edit, session, messages } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' });
  edit('uncaptured'); requests[0].success(true);
  reads[0].failure({ status: 0 });
  assert.equal(session.snapshot().currentContent, 'uncaptured');
  assert.equal(messages.at(-1), 'mutationRevisionRequired');
  Note.curChangedSaveIt(true);
  reads[1].success(movedNote());
  assert.equal(JSON.parse(requests[1].body).Content, 'uncaptured');
});

test('move reload cannot authorize overwriting concurrently changed server content', () => {
  const { Note, requests, reads, target, edit, session } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' });
  edit('draft'); Note.curChangedSaveIt(true); requests[0].success(true);
  const remote = movedNote(); remote.Content = 'remote edit';
  reads[0].success(remote);
  assert.equal(requests.length, 1);
  assert.equal(Note.cache.n1.Usn, undefined);
  assert.equal(Note.savePool.n1.submitted.Content, 'draft');
  assert.equal(session.isDirty(), true);
});

for (const shared of [false, true]) test(`copy success releases source edits (shared=${shared})`, () => {
  const { Note, requests, target, edit } = harness(true);
  Note.cache.n1.UserId = 'owner';
  Note.copyNote(target, { notebookId: 'destination' }, shared);
  edit('source draft'); Note.curChangedSaveIt(true);
  requests[0].success({ Ok: true, Item: [{ NoteId: 'copy1', NotebookId: 'destination' }] });
  assert.equal(requests.length, 2);
  const saved = JSON.parse(requests[1].body);
  assert.equal(saved.ExpectedUsn, 7);
  assert.equal(saved.NotebookId, 'book');
  assert.equal(saved.Content, 'source draft');
});

test('delete confirmation removes captured edits from the leave-page queue', () => {
  const { Note, requests, target, edit } = harness(true);
  Note.deleteNote(target, null, false);
  edit('discarded with deletion'); Note.curChangedSaveIt(true);
  requests[0].success(true);
  assert.equal(Object.keys(Note.savePool).length, 0);
  assert.equal(Note.mutations.hasUnresolved(), false);
  assert.equal(requests.length, 1);
});

test('unknown move retries the frozen identity before refreshing and saving the queue', () => {
  const { Note, requests, reads, target, edit } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' });
  requests[0].failure({ status: 0 });
  edit('queued'); Note.curChangedSaveIt(true);
  assert.equal(reads.length, 0);
  Note.mutations.retryUnknown();
  assert.equal(requests[1].body, requests[0].body);
  requests[1].success(true); requests[0].success(true);
  assert.equal(reads.length, 1);
  reads[0].success(movedNote());
  assert.equal(JSON.parse(requests[2].body).Content, 'queued');
  assert.notEqual(JSON.parse(requests[2].body).OperationId, JSON.parse(requests[0].body).OperationId);
});

test('move refresh and queued save do not change a subsequently opened editor', () => {
  const { Note, requests, reads, target, edit, session } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' });
  edit('n1 draft'); Note.curChangedSaveIt(true); requests[0].success(true);
  Note.curNoteId = 'n2';
  session.load({ noteId: 'n2', persistedContent: 'other', editorContent: 'other' });
  session.markMutation('n2 draft');
  reads[0].success(movedNote()); requests[1].success({ Ok: true, Usn: 13 });
  assert.equal(session.snapshot().noteId, 'n2');
  assert.equal(session.snapshot().currentContent, 'n2 draft');
  assert.equal(session.isDirty(), true);
});

test('invalid move reads retain drafts and only a new successful read releases them', () => {
  for (const ret of [false, { Ok: false, Msg: 'storage' }, movedNote(3), movedNote(12, { NotebookId: 'wrong' }), movedNote(12, { Title: 'remote' })]) {
    const { Note, requests, reads, target, edit, messages } = harness(true);
    Note.moveNote(target, { notebookId: 'destination' });
    edit('draft'); Note.curChangedSaveIt(true); requests[0].success(true);
    reads[0].success(ret);
    assert.equal(requests.length, 1);
    assert.equal(Note.cache.n1.Usn, undefined);
    assert.equal(Note.savePool.n1.submitted.Content, 'draft');
    assert.equal(messages.at(-1), 'mutationRevisionRequired');
    Note.updatePoolNote();
    reads[0].success(movedNote(25));
    assert.equal(requests.length, 1, 'a stale read callback cannot unlock a retry');
    reads[1].success(movedNote());
    assert.equal(JSON.parse(requests[1].body).ExpectedUsn, 12);
  }
});

test('revision recovery blocks another mutation while preserving the pending read', () => {
  const { Note, requests, reads, target } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' }); requests[0].success(true);
  Note.copyNote(target, { notebookId: 'third' }); Note.deleteNote(target, null, false);
  assert.equal(requests.length, 1);
  assert.equal(reads.length, 1);
  reads[0].success(movedNote());
  Note.copyNote(target, { notebookId: 'third' });
  assert.equal(requests.length, 2);
});

test('saving an unchanged moved note retries a failed revision read without writing content', () => {
  const { Note, requests, reads, target, messages } = harness(true);
  Note.moveNote(target, { notebookId: 'destination' }); requests[0].success(true);
  reads[0].failure({ status: 0 });
  Note.curChangedSaveIt(true);
  assert.equal(reads.length, 2);
  reads[1].success(movedNote());
  const messageCount = messages.length;
  reads[1].failure({ status: 0 });
  assert.equal(messages.length, messageCount, 'settled read callbacks must be ignored');
  assert.equal(requests.length, 1);
  Note.copyNote(target, { notebookId: 'third' });
  assert.equal(requests.length, 2);
});

for (const kind of ['copyNote', 'moveNote']) test(`reverting edits during ${kind} removes the obsolete queue`, () => {
  const { Note, requests, reads, target, edit } = harness(true);
  Note[kind](target, { notebookId: 'destination' });
  edit('temporary'); Note.curChangedSaveIt(true);
  edit('old'); Note.curChangedSaveIt(true);
  requests[0].success(kind === 'moveNote' ? true : { Ok: true, Item: [{ NoteId: 'copy1' }] });
  if (reads.length) reads[0].success(movedNote());
  assert.equal(requests.length, 1);
  assert.equal(Object.keys(Note.savePool).length, 0);
});

test('batch delete clears its current note and queues while preserving unrelated edits', () => {
  const { Note, requests, target, edit } = harness(true);
  Note.inBatch = true; Note.getBatchNoteIds = () => ['n1', 'n2']; Note.$itemList = { find: () => target };
  Note.cache.n2 = { NoteId: 'n2', NotebookId: 'book' };
  Note.deleteNote(target, null, false);
  edit('discard'); Note.curChangedSaveIt(true);
  Note.savePool.n2 = { submitted: { NoteId: 'n2', Title: 'discard' }, reconcile: true };
  const unrelated = { submitted: { NoteId: 'n3', Title: 'keep' } };
  Note.savePool.n3 = unrelated;
  requests[0].success(true);
  assert.equal(Note.curNoteId, null);
  assert.deepEqual(Object.keys(Note.savePool), ['n3']);
  assert.equal(Note.savePool.n3, unrelated);
});
test('force without edits does not create an intent', () => {
  const { Note, requests } = harness(true); Note.curChangedSaveIt(true); assert.equal(requests.length, 0);
});
test('reverting metadata while a save is pending queues the reverted value', () => {
  const { Note, requests, title } = harness(true);
  title('first'); Note.curChangedSaveIt(true);
  title('old'); Note.curChangedSaveIt(true);
  assert.equal(requests.length, 1);
  requests[0].success({ Ok: true, Usn: 8 });
  assert.equal(JSON.parse(requests[1].body).Title, 'old');
  assert.equal(JSON.parse(requests[1].body).ExpectedUsn, 8);
  assert.equal(Object.hasOwn(JSON.parse(requests[1].body), 'Content'), false);
});
test('reverting content while pending preserves the newer editor revision', () => {
  const { Note, requests, edit, session } = harness(true);
  edit('first'); Note.curChangedSaveIt(true);
  edit('old'); Note.curChangedSaveIt(true);
  requests[0].success({ Ok: true, Usn: 8 });
  assert.equal(JSON.parse(requests[1].body).Content, 'old');
  assert.equal(session.isDirty(), true);
  requests[1].success({ Ok: true, Usn: 9 });
  assert.equal(session.isDirty(), false);
});
test('leaving warns for unconfirmed work and never starts a save', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/app/page.js'), 'utf8');
  const start = source.indexOf('window.onbeforeunload = function');
  const end = source.indexOf('// 全局快捷键', start);
  let pending = false, dirty = false, prevented = 0;
  const window = {};
  const Note = { mutations: { hasUnresolved: () => pending }, savePool: {}, curNoteId: 'n1',
    curHasChanged: () => dirty, curChangedSaveIt() { assert.fail('must not save during unload'); } };
  vm.runInNewContext(source.slice(start, end), { window, Note, LEA: { isLogout: true } });
  const event = { preventDefault() { prevented++; } };
  window.onbeforeunload(event); assert.equal(prevented, 0);
  pending = true; window.onbeforeunload(event); assert.equal(prevented, 1);
  pending = false; dirty = true; window.onbeforeunload(event); assert.equal(prevented, 2);
});
test('reopening a pending note restores its latest queued draft without confirming it', () => {
  const { Note, requests, edit, session } = harness(true);
  edit('first'); Note.curChangedSaveIt(true);
  edit('second'); Note.curChangedSaveIt(true);
  session.load({ noteId: 'n2', persistedContent: 'other', editorContent: 'other' });
  Note.renderNoteContent(Note.cache.n1);
  assert.equal(session.snapshot().currentContent, 'second');
  assert.equal(session.snapshot().persistedContent, 'old');
  assert.equal(session.isDirty(), true);
  assert.equal(Note.cache.n1.Content, 'old');
  requests[0].success({ Ok: true, Usn: 8 });
  requests[1].success({ Ok: true, Usn: 9 });
  Note.curChangedSaveIt(true);
  assert.equal(requests.length, 2, 'reconciliation must not create a no-op save');
  assert.equal(session.isDirty(), false);
});
test('an unsent queued save retains its captured edits for recovery', () => {
  const { Note } = harness();
  delete Note.cache.n1.Usn;
  Note.savePool.n1 = { submitted: { NoteId: 'n1', Title: 'draft' }, context: { noteId: 'n1' }, capture: null };
  Note.updatePoolNote();
  assert.equal(Note.savePool.n1.submitted.Title, 'draft');
});

test('a commit arriving during editor load cannot restore stale content', () => {
  const { Note, requests, edit, session, deferRender } = harness(true);
  edit('committed'); Note.curChangedSaveIt(true);
  session.load({ noteId: 'n2', persistedContent: 'other', editorContent: 'other' });
  Note.curNoteId = null;
  const finishLoad = deferRender();
  Note.renderNoteContent(Note.cache.n1);
  requests[0].success({ Ok: true, Usn: 8 });
  finishLoad();
  assert.equal(session.snapshot().currentContent, 'committed');
  Note.curChangedSaveIt(true);
  assert.equal(requests.length, 1, 'must not write the old loaded content back');
  assert.equal(session.isDirty(), false);
});
test('serialization failure after commit does not claim editor confirmation', () => {
  const { Note, requests, edit, unavailable, session, messages } = harness();
  edit('changed'); Note.curChangedSaveIt(true); unavailable();
  requests[0].success({ Ok: true, Usn: 8 });
  assert.equal(session.isDirty(), true);
  assert.notEqual(messages.at(-1), 'saveSuccess');
});
