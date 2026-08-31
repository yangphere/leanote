const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const ROOT = path.resolve(__dirname, '../..');
const read = (relative) => fs.readFileSync(path.join(ROOT, relative), 'utf8');

test('TinyMCE 8 owns ordinary paste and no legacy Clipboard fork remains', () => {
  const core = read('public/tinymce/tinymce.js');
  const minCore = read('public/tinymce/tinymce.min.js');
  assert.match(core, /majorVersion\s*[:=]\s*["']8["']/);
  assert.match(minCore, /majorVersion.{0,30}8/);
  assert.doesNotMatch(core, /tinymce\/pasteplugin\/Clipboard|paste\/classes\/Clipboard/);
  const legacyPasteRoot = path.join(ROOT, 'public/tinymce/plugins/paste');
  const legacyFiles = fs.existsSync(legacyPasteRoot)
    ? fs.readdirSync(legacyPasteRoot, { recursive: true }).filter((entry) => /\.[^./\\]+$/.test(entry))
    : [];
  assert.deepEqual(legacyFiles, []);
});

test('Leanote paste boundary has one upload owner and observable failure handling', () => {
  const source = read('public/js/plugins/editor_drop_paste.js');
  const editorRegistrations = source.match(/#editorContent[^\n]*\.fileupload\(/g) ?? [];
  assert.equal(editorRegistrations.length, 1);
  assert.match(source, /url:\s*["']\/file\/pasteImage["']/);
  assert.match(source, /data\.result\.Ok\s*==\s*true/);
  assert.match(source, /data\.process\.remove\(\)/);
  assert.doesNotMatch(source, /tinymce\.pasteplugin|Clipboard\.js/);
});

test('paste/drop upload boundary does not enable a read-only editor', () => {
  const source = read('public/js/plugins/editor_drop_paste.js');
  assert.match(source, /function isEditorReadOnly\(\)/);
  assert.match(source, /if \(isEditorReadOnly\(\)\) \{[\s\S]{0,180}?return;/);
  assert.doesNotMatch(source, /if \(LEA\.readOnly\) \{\s*LEA\.toggleWriteable\(\);/);
  assert.match(source, /data\.leanoteLoadEpoch/);
  assert.match(source, /isCurrentEditorEpoch\(data\.leanoteLoadEpoch\)/);
  assert.match(source, /data\.process && data\.process\.remove\(\)/);
});

test('an upload completed after switching notes cannot insert into the new editor session', () => {
  let uploadOptions;
  let insertions = 0;
  const jquery = (selector) => {
    const node = {
      0: {},
      addClass() { return node; },
      append() { return node; },
      appendTo() { return node; },
      css(name) { return name === 'height' ? '0px' : node; },
      empty() { return node; },
      fileupload(options) {
        if (selector === '#upload') uploadOptions = options;
        return node;
      },
      find() { return node; },
      hide() { return node; },
      on() { return node; },
      parent() { return node; },
      remove() { return node; },
      removeClass() { return node; },
      scrollTop() { return node; },
      show() { return node; },
      trigger() { return node; },
    };
    return node;
  };
  function FakeImage() {
    this.style = {};
  }
  const session = {
    epoch: 1,
    isCurrentLoad(epoch) { return epoch === this.epoch; },
    snapshot() { return { loadEpoch: this.epoch }; },
  };
  const note = { NoteId: 'note-a', UserId: 'user-a', IsMarkdown: false };
  const sandbox = {
    $: jquery,
    Image: FakeImage,
    LEA: { readOnly: false },
    Note: {
      curNoteId: note.NoteId,
      readOnly: false,
      isReadOnly: false,
      getCurNote() { return note; },
    },
    UserInfo: { UserId: note.UserId },
    document: { body: { appendChild() {} }, createElement() { return new FakeImage(); } },
    setTimeout() { return 1; },
    tinymce: {
      activeEditor: {
        dom: { createHTML() { return '<img>'; }, get() { return {}; } },
        insertContent() { insertions += 1; },
      },
    },
  };
  sandbox.window = sandbox;
  sandbox.window.LeanoteEditorSession = session;
  sandbox.define = (_name, _dependencies, factory) => factory();
  vm.runInNewContext(read('public/js/plugins/editor_drop_paste.js'), sandbox);

  const data = {
    context: jquery('<li>'),
    files: [{ name: 'upload.png', size: 128 }],
    result: { Ok: true, Id: 'uploaded-image' },
    submit() {},
  };
  uploadOptions.add(null, data);
  sandbox.Note.curNoteId = 'note-b';
  session.epoch = 2;
  uploadOptions.done(null, data);

  assert.equal(insertions, 0);
});
