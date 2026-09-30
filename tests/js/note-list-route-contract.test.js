const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');
const NOTEBOOK_SOURCE = fs.readFileSync(path.join(ROOT, 'public/js/app/notebook.js'), 'utf8');
const ROUTES_SOURCE = fs.readFileSync(path.join(ROOT, 'conf/routes'), 'utf8');

function functionBody(name, endMarker) {
  const start = NOTEBOOK_SOURCE.indexOf(`${name} = function(`);
  const end = NOTEBOOK_SOURCE.indexOf(endMarker, start);
  assert.ok(start >= 0 && end > start, `${name} handler exists`);
  return NOTEBOOK_SOURCE.slice(start, end);
}

test('notebook list requests use the registered route for both notebook switches', () => {
  const registeredPath = ROUTES_SOURCE.match(/^\*\s+(\S+)\s+Note\.ListNotes\s*$/m)?.[1];
  assert.ok(registeredPath, 'Note.ListNotes route exists');

  const regularSwitch = functionBody('Notebook.changeNotebook', '// 笔记列表与编辑器的mask loading');
  const newNoteSwitch = functionBody('Notebook.changeNotebookForNewNote', '//---------------------------');

  for (const [name, body] of [
    ['regular notebook switch', regularSwitch],
    ['new-note notebook switch', newNoteSwitch],
  ]) {
    const requestedPath = body.match(/var url = "([^"]+)";/)?.[1];
    assert.equal(requestedPath, registeredPath, `${name} must use the registered Note.ListNotes route`);
  }
});
