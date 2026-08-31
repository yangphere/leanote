const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const ROOT = path.resolve(__dirname, '../..');

function loadConfig() {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/tinymce-config.js'), 'utf8');
  const sandbox = { window: {} };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return sandbox.window.LeanoteTinyMCE;
}

test('TinyMCE profiles use the self-hosted v8 runtime and preserve visible commands', () => {
  const config = loadConfig();
  const note = config.createNoteConfig({ selector: '#editorContent', locale: 'zh-cn' });
  const member = config.createMemberConfig({ selector: '#content1', locale: 'en-us' });

  assert.equal(note.license_key, 'gpl');
  assert.equal(note.base_url, '/tinymce');
  assert.equal(note.suffix, '.min');
  assert.equal(note.language, 'zh-CN');
  assert.equal(note.language_url, '/tinymce/langs/zh-cn.js');
  assert.deepEqual(Array.from(note.plugins), [
    'autolink', 'link', 'lists', 'searchreplace', 'table',
    'leaui_image', 'leaui_mindmap', 'leanote_nav', 'leanote_code',
  ]);
  assert.match(note.toolbar, /\bblocks\b/);
  assert.match(note.toolbar, /\bfontfamily\b/);
  assert.match(note.toolbar, /\bfontsize\b/);
  assert.doesNotMatch(note.toolbar, /formatselect|fontselect|fontsizeselect/);

  assert.equal(member.license_key, 'gpl');
  assert.equal(member.language, 'en-US');
  assert.deepEqual(Array.from(member.plugins), [
    'advlist', 'autolink', 'link', 'lists', 'charmap', 'searchreplace',
    'visualblocks', 'visualchars', 'table', 'directionality',
  ]);
  assert.doesNotMatch(member.plugins.join(' '), /\b(?:paste|hr|contextmenu|textcolor|tabfocus|fullpage)\b/);
});

test('TinyMCE locale mapping uses canonical RFC 5646 codes and rejects unsupported application locales', () => {
  const config = loadConfig();
  const expected = {
    'de-de': { language: 'de-DE', language_url: '/tinymce/langs/de-de.js' },
    'en-us': { language: 'en-US', language_url: '/tinymce/langs/en-us.js' },
    'es-co': { language: 'es-CO', language_url: '/tinymce/langs/es-co.js' },
    'fr-fr': { language: 'fr-FR', language_url: '/tinymce/langs/fr-fr.js' },
    'pt-pt': { language: 'pt-PT', language_url: '/tinymce/langs/pt-pt.js' },
    'zh-cn': { language: 'zh-CN', language_url: '/tinymce/langs/zh-cn.js' },
    'zh-hk': { language: 'zh-HK', language_url: '/tinymce/langs/zh-hk.js' },
  };
  for (const [locale, resolution] of Object.entries(expected)) {
    assert.deepEqual({ ...config.resolveLocale(locale) }, resolution);
  }
  assert.throws(() => config.resolveLocale('en_us'), /Unsupported TinyMCE locale/);
});

test('TinyMCE language code matches the code registered by the generated language pack', () => {
  const config = loadConfig();
  for (const locale of ['de-de', 'en-us', 'es-co', 'fr-fr', 'pt-pt', 'zh-cn', 'zh-hk']) {
    const resolved = config.resolveLocale(locale);
    const languagePack = fs.readFileSync(path.join(ROOT, `public/tinymce/langs/${locale}.js`), 'utf8');
    assert.match(languagePack, new RegExp(`tinymce\\.addI18n\\(["']${resolved.language}["']`), `${locale} language pack must register ${resolved.language}`);
  }
});
