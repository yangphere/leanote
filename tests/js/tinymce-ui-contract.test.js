const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');
const read = (relative) => fs.readFileSync(path.join(ROOT, relative), 'utf8');

const applicationLess = [
  'public/css/editor/editor.less',
  'public/css/private-share-note.less',
  'public/css/blog/blog_left_fixed.less',
  'public/css/theme/basic.less',
  'public/css/theme/mobile.less',
  'public/css/theme/writting.less',
  'public/css/theme/writting-overwrite.less',
  'public/css/theme/includes/editor.less',
  'public/css/theme/includes/icon.less',
  'public/css/theme/includes/tinymce.less',
  'public/member/css/member.less',
];

test('application styles do not target the retired TinyMCE 4 chrome', () => {
  for (const relative of applicationLess) {
    const source = read(relative);
    assert.doesNotMatch(
      source,
      /\.mce-(?!item-table\b|match-marker(?:-selected)?\b)[A-Za-z0-9_-]+/,
      `${relative} must keep only content-semantic mce classes`,
    );
  }
});

test('application TinyMCE chrome overrides use stable Oxide selectors', () => {
  const source = read('public/css/theme/includes/tinymce.less');
  for (const selector of [
    '.tox-edit-area__iframe',
    '.tox-tinymce',
    '.tox .tox-tbtn',
    '.tox .tox-dialog__footer .tox-button',
    '.tox .tox-collection__item',
  ]) {
    assert.match(source, new RegExp(selector.replace(/[.]/g, '\\.'), `u`), `${selector} must be covered`);
  }
  for (const relative of [
    'public/css/theme/default.css',
    'public/css/theme/simple.css',
    'public/css/theme/writting.css',
    'public/css/theme/writting-overwrite.css',
  ]) {
    assert.match(read(relative), /\.tox-tinymce\b/, `${relative} must be generated from Oxide overrides`);
  }
});

test('TinyMCE public tree excludes retired v4 sidecars and module-loader entries', () => {
  for (const relative of [
    'public/tinymce/langs/zh.js',
    'public/tinymce/icons/default/index.js',
    'public/tinymce/models/dom/index.js',
    'public/tinymce/plugins/leanote_code/img/ace-pre.png',
    'public/tinymce/plugins/leanote_code/img/ace-pre2.png',
    'public/tinymce/plugins/leanote_code/langs/en.js',
    'public/tinymce/plugins/leanote_code/langs/zh.js',
    'public/tinymce/plugins/table/plugin.dev.js',
    'public/tinymce/plugins/visualblocks/css/visualblocks.css',
  ]) {
    assert.equal(fs.existsSync(path.join(ROOT, relative)), false, `${relative} must not be served`);
  }
  for (const relative of [
    'public/tinymce/plugins/table/classes',
    'public/tinymce/plugins/visualblocks/img',
    'public/tinymce/plugins/leanote_code/img',
    'public/tinymce/plugins/leanote_code/langs',
  ]) {
    assert.equal(fs.existsSync(path.join(ROOT, relative)), false, `${relative} must not be served`);
  }
});
