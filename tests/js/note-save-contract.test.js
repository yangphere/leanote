const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');
const SOURCE = fs.readFileSync(path.join(ROOT, 'public/js/app/note.js'), 'utf8');

test('successful note saves normalize tags before updating the cache', () => {
  assert.match(SOURCE, /ajaxPost\("\/note\/updateNoteOrContent", submitted/);
  assert.match(SOURCE, /beginSave\(hasChanged\.Content\)/);
  assert.match(SOURCE, /captureEditorSaveContext\(hasChanged\.NoteId\)/);
  assert.match(SOURCE, /isCurrentEditorSaveCapture\(saveCapture\)/);

  const cacheUpdateStart = SOURCE.indexOf('function cacheSavedNote(note)');
  assert.notEqual(cacheUpdateStart, -1);
  const cacheUpdate = SOURCE.slice(cacheUpdateStart, cacheUpdateStart + 500);
  assert.match(cacheUpdate, /typeof cacheUpdate\.Tags === 'string'/);
  assert.match(cacheUpdate, /cacheUpdate\.Tags = cacheUpdate\.Tags\.split\(','\)/);
  assert.match(cacheUpdate, /cacheUpdate\.UpdatedTime = \(new Date\(\)\)\.format/);
  assert.match(cacheUpdate, /Note\.setNoteCache\(cacheUpdate, false\);\s*Note\.clearCacheByNotebookId\(cacheUpdate\.NotebookId\);/);
  assert.match(SOURCE, /cacheSavedNote\(submitted\);/);
  assert.match(SOURCE, /cacheSavedNote\(queuedChange\);/);
  assert.match(SOURCE, /if \(!isCurrentEditorSaveContext\(saveContext\)\) \{[\s\S]*?cacheSavedNote\(submitted\);/);
  assert.match(SOURCE, /if \(!isCurrentEditorSaveContext\(queuedContext\)\) \{\s*cacheSavedNote\(queuedChange\);/);
  assert.match(SOURCE, /\}\)\(noteId, hasChanged, saveCapture, saveContext\);/);
});
