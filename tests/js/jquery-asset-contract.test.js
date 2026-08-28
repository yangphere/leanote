const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');
const RUNTIME_INPUT = 'node_modules/jquery/dist/jquery.min.js';
const RUNTIME_OUTPUT = 'public/js/jquery-1.9.0.min.js';
const FILEUPLOAD_INPUTS = [
  'node_modules/blueimp-file-upload/js/vendor/jquery.ui.widget.js',
  'node_modules/blueimp-file-upload/js/jquery.iframe-transport.js',
  'node_modules/blueimp-file-upload/js/jquery.fileupload.js',
];
const FILEUPLOAD_COPIES = [
  'public/js/plugins/libs-min/jquery.ui.widget.js',
  'public/js/plugins/libs-min/jquery.iframe-transport.js',
  'public/js/plugins/libs-min/jquery.fileupload.js',
  'public/tinymce/plugins/leaui_image/public/js/jquery.ui.widget.js',
  'public/tinymce/plugins/leaui_image/public/js/jquery.iframe-transport.js',
  'public/tinymce/plugins/leaui_image/public/js/jquery.fileupload.js',
];

test('package.json locks jquery 3.7.1 and test-only jquery-migrate 3.6.0', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(ROOT, 'package.json'), 'utf8'));
  assert.equal(pkg.devDependencies?.jquery, '3.7.1');
  assert.equal(pkg.devDependencies?.['jquery-migrate'], '3.6.0');
  assert.equal(pkg.dependencies?.jquery, undefined);
  assert.equal(pkg.dependencies?.['jquery-migrate'], undefined);
});

test('installed jquery and jquery-migrate resolve to the locked versions', () => {
  const jquery = JSON.parse(fs.readFileSync(path.join(ROOT, 'node_modules/jquery/package.json'), 'utf8'));
  assert.equal(jquery.version, '3.7.1');
  const migrate = JSON.parse(fs.readFileSync(path.join(ROOT, 'node_modules/jquery-migrate/package.json'), 'utf8'));
  assert.equal(migrate.version, '3.6.0');
  assert.match(String(migrate.peerDependencies?.jquery), />=3\s*<4/);
});

test('manifest publishes jquery-runtime from the npm input to the legacy public URL', async () => {
  const { MANIFEST, BUILD_OUTPUTS, validateManifest } = await import('../../scripts/build/manifest.mjs');
  const runtime = MANIFEST.js.find((entry) => entry.name === 'jquery-runtime');
  assert.ok(runtime, 'jquery-runtime entry exists');
  assert.deepEqual(runtime.inputs, [RUNTIME_INPUT]);
  assert.equal(runtime.output, RUNTIME_OUTPUT);
  assert.equal(runtime.url, '/js/jquery-1.9.0.min.js');
  validateManifest(MANIFEST);
  assert.equal(BUILD_OUTPUTS.length, 34);
  assert.equal(new Set(BUILD_OUTPUTS).size, 34);
  assert.equal(BUILD_OUTPUTS.includes(RUNTIME_OUTPUT), true);
  assert.equal(MANIFEST.i18nDerivedInputExclusions.includes(RUNTIME_OUTPUT), true);
});

test('dep and album bundles consume the npm jquery input, not the generated runtime output', async () => {
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  for (const name of ['dep', 'album']) {
    const entry = MANIFEST.js.find((item) => item.name === name);
    assert.ok(entry, `${name} entry exists`);
    assert.equal(entry.inputs.includes(RUNTIME_INPUT), true, `${name} uses the npm jquery input`);
    assert.equal(entry.inputs.includes(RUNTIME_OUTPUT), false, `${name} must not feed on the generated output`);
  }
});

test('jquery-migrate stays out of the production manifest and outputs', async () => {
  const { MANIFEST, BUILD_OUTPUTS } = await import('../../scripts/build/manifest.mjs');
  const serialized = JSON.stringify(MANIFEST);
  assert.equal(serialized.includes('migrate'), false);
  assert.equal(BUILD_OUTPUTS.some((output) => output.includes('migrate')), false);
});

test('leaui_image iframe uses the canonical runtime URL and ships no private jquery copy', () => {
  const privateCopy = path.join(ROOT, 'public/tinymce/plugins/leaui_image/public/js/jquery.js');
  assert.equal(fs.existsSync(privateCopy), false, 'private 1.9.1 copy must be deleted');
  const html = fs.readFileSync(path.join(ROOT, 'public/tinymce/plugins/leaui_image/index.html'), 'utf8');
  assert.match(html, /src="\/js\/jquery-1\.9\.0\.min\.js"/);
  assert.doesNotMatch(html, /public\/js\/jquery\.js/);
});

test('generated runtime asset and bundles contain jQuery 3.7.1 only', () => {
  const runtime = fs.readFileSync(path.join(ROOT, RUNTIME_OUTPUT), 'utf8');
  assert.match(runtime, /jQuery v3\.7\.1/);
  assert.doesNotMatch(runtime, /jQuery v1\./);
  // dep concatenates the npm dist verbatim (license banner preserved); album is
  // re-minified by esbuild, which strips comments but keeps the version literal.
  assert.match(fs.readFileSync(path.join(ROOT, 'public/js/dep.min.js'), 'utf8'), /jQuery v3\.7\.1/);
  assert.match(fs.readFileSync(path.join(ROOT, 'public/album/js/main.all.js'), 'utf8'), /"3\.7\.1"/);
  for (const bundle of ['public/js/dep.min.js', 'public/album/js/main.all.js']) {
    const content = fs.readFileSync(path.join(ROOT, bundle), 'utf8');
    assert.doesNotMatch(content, /jQuery v1\./, `${bundle} must not embed a 1.9 core banner`);
    assert.doesNotMatch(content, /"1\.9\.[01]"/, `${bundle} must not embed a 1.9 version literal`);
  }
});

test('the legacy public URL keeps resolving through the tracked generated file', () => {
  const target = path.join(ROOT, RUNTIME_OUTPUT);
  assert.equal(fs.existsSync(target), true);
  execGitLsFiles(RUNTIME_OUTPUT);
});

test('package.json locks blueimp-file-upload 10.32.0 (MIT provenance)', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(ROOT, 'package.json'), 'utf8'));
  assert.equal(pkg.devDependencies?.['blueimp-file-upload'], '10.32.0');
  const upstream = JSON.parse(fs.readFileSync(path.join(ROOT, 'node_modules/blueimp-file-upload/package.json'), 'utf8'));
  assert.equal(upstream.version, '10.32.0');
  assert.match(String(upstream.license), /MIT/i);
});

test('blueimp-file-upload 10.32.0 files are consumed verbatim from the npm dist', () => {
  FILEUPLOAD_COPIES.forEach((copy, index) => {
    const input = FILEUPLOAD_INPUTS[index % 3];
    assert.equal(
      fs.readFileSync(path.join(ROOT, copy), 'utf8'),
      fs.readFileSync(path.join(ROOT, input), 'utf8'),
      `${copy} must be a byte-identical copy of ${input}`,
    );
    execGitLsFiles(copy);
  });
});

test('superseded 5.26 fileupload copies are gone', () => {
  assert.equal(fs.existsSync(path.join(ROOT, 'public/js/plugins/libs')), false, 'readable 5.26 libs directory must be deleted');
  assert.equal(fs.existsSync(path.join(ROOT, 'public/js/plugins/libs-min/fileupload.js')), false, 'self-contained 5.26 concat must be deleted');
});

test('plugins and album bundles consume the npm fileupload inputs behind the amd guard', async () => {
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  for (const name of ['plugins', 'album']) {
    const entry = MANIFEST.js.find((item) => item.name === name);
    assert.ok(entry, `${name} entry exists`);
    for (const input of FILEUPLOAD_INPUTS) {
      assert.equal(entry.inputs.includes(input), true, `${name} must consume ${input}`);
      assert.equal(entry.amdGuard?.includes(input), true, `${name} must amdGuard ${input}`);
    }
    assert.equal(entry.inputs.includes('public/js/plugins/libs-min/fileupload.js'), false, `${name} must not consume the removed 5.26 concat`);
  }
});

test('generated bundles carry the 10.32 upload stack without the 5.26 banner', () => {
  for (const bundle of ['public/js/plugins/main.min.js', 'public/album/js/main.all.js']) {
    const content = fs.readFileSync(path.join(ROOT, bundle), 'utf8');
    assert.match(content, /blueimp\.fileupload/, `${bundle} must register the blueimp fileupload widget`);
    assert.doesNotMatch(content, /jQuery File Upload Plugin 5\./, `${bundle} must not embed the 5.26 banner`);
    assert.doesNotMatch(content, /\.pipe\(/, `${bundle} must not call removed-style Deferred.pipe`);
  }
});

test('markdown sources keep jQuery 3 removed-API cleanup', () => {
  for (const file of ['public/md/main-v2.js', 'public/md/main-v2.min.js']) {
    const content = fs.readFileSync(path.join(ROOT, file), 'utf8');
    assert.doesNotMatch(content, /\.andSelf\(/, `${file} must not call andSelf`);
    assert.doesNotMatch(content, /\.bind\(\s*['"`]/, `${file} must not call jQuery .bind`);
  }
});

const FIRST_PARTY_SOURCES = [
  'public/js/home/index.js',
  'public/js/common.js',
  'public/js/app/note.js',
  'public/js/app/tag.js',
  'public/js/app/notebook.js',
  'public/js/app/share.js',
  'public/js/app/page.js',
  'public/js/plugins/note_info.js',
  'public/js/plugins/tips.js',
  'public/js/plugins/history.js',
  'public/js/plugins/attachment_upload.js',
  'public/js/plugins/editor_drop_paste.js',
  'public/js/plugins/main.js',
  'public/album/js/main.js',
  'public/tinymce/plugins/leaui_image/public/js/main.js',
  'public/member/js/avatar.js',
  'public/member/js/import_theme.js',
  'public/member/js/member.js',
  'public/admin/js/admin.js',
  'public/blog/js/common.js',
  'public/blog/js/share_comment.js',
  'public/js/app/blog/common.js',
  'public/js/app/blog/view.js',
  'public/js/contextmenu/jquery.contextmenu.js',
  'public/js/jquery.pagination.js',
  'public/tinymce/plugins/leaui_image/public/js/jquery.pagination.js',
  'public/md/main-v2.js',
];

const FIRST_PARTY_TEMPLATE_SOURCES = walkFiles(path.join(ROOT, 'app/views'), '.html');

test('the leaui pagination copy stays byte-identical to the maintained root file', () => {
  const root = fs.readFileSync(path.join(ROOT, 'public/js/jquery.pagination.js'), 'utf8');
  const copy = fs.readFileSync(path.join(ROOT, 'public/tinymce/plugins/leaui_image/public/js/jquery.pagination.js'), 'utf8');
  assert.equal(copy, root, 'leaui_image/private/jquery.pagination.js must not drift from public/js/jquery.pagination.js');
});

test('first-party sources avoid migrate-warned jQuery APIs (diagnostics contract)', () => {  const patterns = [
    [/\.bind\(\s*['"`]/, 'jQuery .bind'],
    [/\.unbind\(/, 'jQuery .unbind'],
    [/\.hover\(/, 'jQuery .hover'],
    [/\$\.trim\(/, 'jQuery.trim'],
    [/\$\.isArray\(/, 'jQuery.isArray'],
    [/\$\.isFunction\(/, 'jQuery.isFunction'],
    [/\$\.proxy\(/, 'jQuery.proxy'],
    [/\.attr\(\s*['"`]contenteditable['"`]\s*,\s*(?:false|true)\s*\)/, 'boolean contenteditable attr'],
  ];
  for (const event of ['click', 'dblclick', 'change', 'keydown', 'keyup', 'keypress', 'submit', 'scroll', 'resize', 'mousedown', 'mouseup', 'mousemove', 'mouseover', 'mouseout', 'mouseenter', 'mouseleave']) {
    // Handler-attachment forms: anonymous functions, named handler refs and
    // the string-first variant (e.g. .click('click', fn)).
    patterns.push([new RegExp(`\\.${event}\\(\\s*(?=function)`), `jQuery.fn.${event}() shorthand`]);
    patterns.push([new RegExp(`\\.${event}\\(\\s*['"\`]`), `jQuery.fn.${event}() shorthand (string-first)`]);
    patterns.push([new RegExp(`\\.${event}\\(\\s*[A-Za-z_$][\\w$.]*\\s*\\)`), `jQuery.fn.${event}() shorthand (handler reference)`]);
  }
  // .focus()/.blur() are exempt from the identifier-arg pattern: DOM-native
  // element.focus() is ubiquitous and indistinguishable statically. The
  // runtime diagnostics are the authoritative gate for them.
  for (const event of ['focus', 'blur', 'hover']) {
    patterns.push([new RegExp(`\\.${event}\\(\\s*(?=function)`), `jQuery.fn.${event}() shorthand`]);
    patterns.push([new RegExp(`\\.${event}\\(\\s*['"\`]`), `jQuery.fn.${event}() shorthand (string-first)`]);
  }
  patterns.push([/\$\.expr\[\s*['"`]:['"`]\s*\]/, '$.expr[":"] legacy pseudos access']);
  for (const event of ['click', 'dblclick', 'change', 'keydown', 'keyup', 'keypress', 'submit', 'scroll', 'resize', 'mousedown', 'mouseup', 'mousemove', 'mouseover', 'mouseout', 'mouseenter', 'mouseleave', 'focus', 'blur']) {
    patterns.push([new RegExp(`\\$\\([^\\n;]*?\\)\\.${event}\\(\\s*\\)`), `jQuery.fn.${event}() no-arg shorthand`]);
  }
  for (const file of [...FIRST_PARTY_SOURCES, ...FIRST_PARTY_TEMPLATE_SOURCES]) {
    const content = fs.readFileSync(path.join(ROOT, file), 'utf8');
    for (const [pattern, label] of patterns) {
      // main-v2 embeds the unmodified waitForImages 1.4.2 plugin. Its
      // $.isFunction and $.expr usage is covered by the ownership entry in
      // the compatibility inventory; first-party event shorthand remains
      // checked below.
      if (file.startsWith('public/md/main-v2') && (label === 'jQuery.isFunction' || label === '$.expr[":"] legacy pseudos access')) continue;
      assert.doesNotMatch(content, pattern, `${file} must not use ${label}`);
    }
  }
});

function walkFiles(directory, extension) {
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...walkFiles(absolute, extension));
    else if (entry.isFile() && entry.name.endsWith(extension)) files.push(path.relative(ROOT, absolute));
  }
  return files;
}

function execGitLsFiles(relative) {
  const { execFileSync } = require('node:child_process');
  execFileSync('git', ['ls-files', '--error-unmatch', relative], { cwd: ROOT, stdio: 'pipe' });
}
