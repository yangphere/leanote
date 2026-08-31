const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const ROOT = path.resolve(__dirname, '../..');

function loadStateFactory() {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/editor-state.js'), 'utf8');
  const sandbox = { window: {} };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return sandbox.window.LeanoteEditorState;
}

test('programmatic load establishes an editor baseline without a content revision', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  assert.equal(state.snapshot().contentRevision, 0);
  assert.equal(state.snapshot().editorBaseline, '<p>A</p>');
  assert.equal(state.isDirty(), false);
  assert.equal(state.setContentProgrammatically('<p>A normalized</p>'), false);
  assert.equal(state.snapshot().contentRevision, 0);
  assert.equal(state.isDirty(), false);
});

test('first save can capture explicit new-note content even when it equals the editor baseline', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'new-note', persistedContent: '', editorContent: '<p>initial</p>' });

  const capture = state.beginSave('<p>initial</p>');
  assert.equal(capture.noteId, 'new-note');
  assert.equal(capture.loadEpoch, 1);
  assert.equal(capture.revision, 0);
  assert.equal(capture.content, '<p>initial</p>');
  assert.equal(state.confirmSave(capture, '<p>initial</p>'), true);
  assert.equal(state.snapshot().persistedContent, '<p>initial</p>');
  assert.equal(state.snapshot().editorBaseline, '<p>initial</p>');
  assert.equal(state.isDirty(), false);
});

test('save confirmation advances both baselines while preserving edits made during the request', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  state.noteContentChanged('<p>B</p>');
  const capture = state.beginSave();
  state.noteContentChanged('<p>C</p>');
  state.confirmSave(capture, '<p>C</p>');

  assert.equal(state.snapshot().persistedContent, '<p>B</p>');
  assert.equal(state.snapshot().editorBaseline, '<p>B</p>');
  assert.equal(state.snapshot().contentRevision, 2);
  assert.equal(state.isDirty(), true);
  state.markMutation('<p>B</p>');
  assert.equal(state.isDirty(), false);
  state.markMutation('<p>A</p>');
  assert.equal(state.isDirty(), true);
});

test('failed or stale save confirmation does not update baselines', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  state.noteContentChanged('<p>B</p>');
  const capture = state.beginSave();
  state.failSave(capture);
  assert.equal(state.snapshot().editorBaseline, '<p>A</p>');
  assert.equal(state.isDirty(), true);

  const stale = state.beginSave();
  state.load({ noteId: 'note-b', persistedContent: '<p>X</p>', editorContent: '<p>X</p>' });
  assert.equal(state.confirmSave(stale, '<p>X</p>'), false);
  assert.equal(state.snapshot().noteId, 'note-b');
  assert.equal(state.snapshot().editorBaseline, '<p>X</p>');
});

test('out-of-order save confirmations cannot move the baseline backwards', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  state.noteContentChanged('<p>B</p>');
  const first = state.beginSave();
  state.noteContentChanged('<p>C</p>');
  const second = state.beginSave();
  assert.equal(state.confirmSave(second, '<p>C</p>'), true);
  assert.equal(state.confirmSave(first, '<p>C</p>'), false);
  assert.equal(state.snapshot().editorBaseline, '<p>C</p>');
});

test('save confirmation fails closed when current serialization is unavailable', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  state.noteContentChanged('<p>B</p>');
  const capture = state.beginSave();

  assert.equal(state.confirmSave(capture, undefined), false);
  assert.equal(state.snapshot().persistedContent, '<p>A</p>');
  assert.equal(state.snapshot().editorBaseline, '<p>A</p>');
  assert.equal(state.snapshot().confirmedRevision, 0);
  assert.equal(state.isDirty(), true);
});

test('read-only and loading boundaries reject content mutations', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: true });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  assert.equal(state.noteContentChanged('<p>B</p>'), false);
  assert.equal(state.snapshot().contentRevision, 0);
  state.setReadOnly(false);
  assert.equal(state.noteContentChanged('<p>B</p>'), true);
  assert.equal(state.snapshot().contentRevision, 1);
});

test('stale load callbacks cannot mutate the active note', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  const firstEpoch = state.beginLoad({ noteId: 'note-a', persistedContent: '<p>A</p>' });
  const secondEpoch = state.beginLoad({ noteId: 'note-b', persistedContent: '<p>B</p>' });

  assert.equal(state.setContentProgrammatically('<p>A late</p>', firstEpoch), false);
  assert.equal(state.markMutation('<p>A late</p>', firstEpoch), false);
  assert.equal(state.completeLoad(firstEpoch, '<p>A late</p>'), false);
  assert.equal(state.completeLoad(secondEpoch, '<p>B</p>'), true);
  assert.equal(state.snapshot().noteId, 'note-b');
  assert.equal(state.snapshot().editorBaseline, '<p>B</p>');
  assert.equal(state.snapshot().contentRevision, 0);
});

test('save confirmation refreshes current serialization before recalculating dirty state', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  state.noteContentChanged('<p>B</p>');
  const capture = state.beginSave();

  assert.equal(state.confirmSave(capture, '<p>C</p>'), true);
  assert.equal(state.snapshot().currentContent, '<p>C</p>');
  assert.equal(state.snapshot().editorBaseline, '<p>B</p>');
  assert.equal(state.isDirty(), true);
});

test('save capture uses the exact serialized bytes sent to the server', () => {
  const api = loadStateFactory();
  const state = api.create({ readOnly: false });
  state.load({ noteId: 'note-a', persistedContent: '<p>A</p>', editorContent: '<p>A</p>' });
  state.noteContentChanged('<p>B data</p>');

  const capture = state.beginSave('<p>B serialized</p>');
  assert.equal(capture.content, '<p>B serialized</p>');
  assert.equal(capture.revision, 1);
});
