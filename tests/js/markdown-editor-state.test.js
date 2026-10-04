const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const ROOT = path.resolve(__dirname, '../..');

function harness() {
  const listeners = [];
  let content = '';
  const window = {};
  const Note = { readOnly: false, current: { IsMarkdown: true }, getCurNote() { return this.current; } };
  const MD = {
    eventMgr: { addListener(name, listener) { assert.equal(name, 'onContentChanged'); listeners.push(listener); } },
    setContent(value) { content = value; for (const listener of listeners) listener({ content: value }); },
    getContent() { return content; },
    clearUndo() {},
  };
  const context = vm.createContext({ window, Note, MD });
  vm.runInContext(fs.readFileSync(path.join(ROOT, 'public/js/editor-state-source.js'), 'utf8'), context);
  const common = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = common.indexOf('var clearIntervalForSetContent;');
  const end = common.indexOf('// preview', start);
  assert.ok(start >= 0 && end > start);
  vm.runInContext(common.slice(start, end), context);
  const session = window.LeanoteEditorSession;
  function load(value) {
    const epoch = session.beginLoad({ noteId: 'markdown-note', persistedContent: value });
    context.setEditorContent(value, true, null, () => session.completeLoad(epoch, value), epoch);
  }
  function edit(value) { MD.setContent(value); }
  return { Note, session, load, edit, listeners };
}

test('Markdown edits enter the shared session while programmatic loads stay clean', () => {
  const { session, load, edit } = harness();
  load('persisted Markdown');
  assert.equal(session.isDirty(), false);
  edit('## Changed\n\n![image](/file/outputImage?fileId=fixture)');
  assert.equal(session.isDirty(), true);
  assert.equal(session.snapshot().contentRevision, 1);
  assert.equal(session.beginSave().content, '## Changed\n\n![image](/file/outputImage?fileId=fixture)');
});

test('Markdown events do not alter read-only, loading or rich-text sessions', () => {
  const { Note, session, load, edit, listeners } = harness();
  load('persisted');
  Note.readOnly = true;
  edit('read-only edit');
  assert.equal(session.isDirty(), false);
  Note.readOnly = false;
  Note.current.IsMarkdown = false;
  edit('inactive Markdown edit');
  assert.equal(session.isDirty(), false);
  Note.current.IsMarkdown = true;
  session.beginLoad({ noteId: 'next-note', persistedContent: 'next' });
  edit('during load');
  assert.equal(session.snapshot().contentRevision, 0);
  load('next');
  listeners.forEach(listener => listener({ content: 'stale Markdown event' }));
  assert.equal(session.snapshot().currentContent, 'next');
});

test('loading another Markdown note does not duplicate mutation listeners', () => {
  const { session, load, edit, listeners } = harness();
  load('first');
  load('second');
  assert.equal(listeners.length, 1);
  assert.equal(session.isDirty(), false);
  edit('second changed');
  assert.equal(session.snapshot().currentContent, 'second changed');
  assert.equal(session.snapshot().contentRevision, 1);
});
