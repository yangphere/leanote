const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');
const source = (name) => fs.readFileSync(path.join(ROOT, `public/tinymce/plugins/${name}/plugin.js`), 'utf8');

test('first-party plugins use TinyMCE 8 public UI and mutation boundaries', () => {
  for (const name of ['leaui_image', 'leaui_mindmap', 'leanote_code']) {
    const code = source(name);
    assert.match(code, /tinymce\.PluginManager\.add\(/, `${name} must register through PluginManager`);
    assert.match(code, /editor\.ui\.registry\.addButton\(/, `${name} must use UI registry buttons`);
    assert.match(code, /onSetup/, `${name} must react to read-only mode changes`);
    assert.match(code, /isReadOnly\(editor\)/, `${name} must gate actions`);
    assert.match(code, /LeanoteEditorSession\.markMutation/, `${name} must notify content mutations`);
    assert.match(code, /leanote_markMutation/, `${name} must accept the shared mutation adapter`);
    assert.doesNotMatch(code, /editor\.addButton\(|editor\.addMenuItem\(/, `${name} must not use TinyMCE 4 UI APIs`);
  }
});

test('leaui_image and mindmap expose local URL-dialog insertion/update contracts', () => {
  const image = source('leaui_image');
  assert.match(image, /windowManager\.openUrl\(/);
  assert.match(image, /onMessage\s*:\s*function/);
  assert.match(image, /mceAction\s*!==\s*'insertImage'/);
  assert.match(image, /editor\.insertContent\(imageHtml\(data\)\)/);
  assert.match(image, /data-mind-json/);
  assert.match(image, /javascript\|vbscript\|file/);
  assert.match(image, /undoManager\.transact/);
  assert.doesNotMatch(image, /leanote\.com|tinymce\.pasteplugin|Clipboard\.js/);

  const mindmap = source('leaui_mindmap');
  assert.match(mindmap, /editor\.ui\.registry\.addIcon\(\s*['"]mind['"]/);
  assert.match(mindmap, /icon:\s*['"]mind['"]/);
  assert.match(mindmap, /<svg[\s>]/);
  assert.match(mindmap, /mindmap\/index\.html/);
  assert.match(mindmap, /data-mind-json/);
  assert.match(mindmap, /JSON\.parse\(/);
  assert.match(mindmap, /javascript\|vbscript\|file/);
  assert.match(mindmap, /mceAction\s*!==\s*'insertMindMap'/);
  assert.match(mindmap, /editor\.insertContent\(/);
  assert.match(mindmap, /undoManager\.transact/);
  assert.doesNotMatch(mindmap, /leanote\.com|libs\/mind\/edit/);

  const imageFrame = fs.readFileSync(path.join(ROOT, 'public/tinymce/plugins/leaui_image/index.html'), 'utf8');
  assert.match(imageFrame, /mceAction:\s*'insertImage'/);
  const mindmapFrame = fs.readFileSync(path.join(ROOT, 'public/tinymce/plugins/leaui_mindmap/mindmap/index.html'), 'utf8');
  assert.match(mindmapFrame, /mceAction:\s*'insertMindMap'/);
  assert.match(mindmapFrame, /exportData\('png'\)/);
});

test('leanote_nav only writes the external navigation container', () => {
  const nav = source('leanote_nav');
  assert.match(nav, /querySelectorAll\('h1,h2,h3,h4,h5,h6'\)/);
  assert.match(nav, /getElementById\('leanoteNavContent'\)/);
  assert.match(nav, /replaceChildren\(/);
  assert.doesNotMatch(nav, /editor\.setContent|editor\.insertContent|markMutation|LeanoteEditorSession/);
  assert.doesNotMatch(nav, /target\.innerHTML\s*=|titles\s*\+=/);
});

test('code plugin keeps serialized language metadata and visible Ace failure gate', () => {
  const code = source('leanote_code');
  assert.match(code, /data-language/);
  assert.match(code, /windowManager\.open\(/);
  assert.match(code, /selectbox/);
  assert.match(code, /LeaAce\.canAce/);
  assert.match(code, /normalizeLanguage/);
  assert.match(code, /return false/);
  assert.match(code, /mceToggleFormat/);
  assert.match(code, /undoManager\.transact/);
});

test('Ace edits enter the shared content revision after hydration', () => {
  const page = fs.readFileSync(path.join(ROOT, 'public/js/app/page.js'), 'utf8');
  assert.match(page, /ed\.on\(['"]init SetContent['"]/);
  assert.match(page, /initAceFromContent\(ed\)/);
  assert.match(page, /aceEditor\.session\.on\(["']change["']/);
  assert.match(page, /markMutation\(getEditorContent\(false\), aceLoadEpoch\)/);
});
