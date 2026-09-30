const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.resolve(__dirname, '../../../..');
const source = fs.readFileSync(path.join(root, 'public/js/app/notebook.js'), 'utf8');
const start = source.indexOf('Notebook.changeNotebookForNewNote = function(');
const end = source.indexOf('//---------------------------', start);
assert.ok(start >= 0 && end > start, 'new-note notebook switch handler exists');

let requestedPath;
const sandbox = {
  Notebook: {
    isTrashNotebookId: () => false,
    isAllNotebookId: () => false,
    changeNotebookNav: () => {},
  },
  ajaxGet: (url) => { requestedPath = url; },
};
vm.createContext(sandbox);
vm.runInContext(source.slice(start, end), sandbox);
sandbox.Notebook.changeNotebookForNewNote('notebook-id');

const routes = fs.readFileSync(path.join(root, 'conf/routes'), 'utf8');
const registeredPath = routes.match(/^\*\s+(\S+)\s+Note\.ListNotes\s*$/m)?.[1];
assert.ok(registeredPath, 'Note.ListNotes route exists');
assert.equal(requestedPath, registeredPath, 'new-note notebook list request must match registered route');
