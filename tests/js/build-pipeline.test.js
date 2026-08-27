const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');

function copyBuildTree() {
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-tree-'));
  fs.cpSync(path.join(ROOT, 'public'), path.join(temp, 'public'), { recursive: true });
  fs.cpSync(path.join(ROOT, 'messages'), path.join(temp, 'messages'), { recursive: true });
  fs.cpSync(path.join(ROOT, 'app', 'views'), path.join(temp, 'app', 'views'), { recursive: true });
  fs.cpSync(path.join(ROOT, 'tests', 'js', 'fixtures'), path.join(temp, 'tests', 'js', 'fixtures'), { recursive: true });
  // The manifest consumes node_modules/jquery/dist/jquery.min.js as the canonical jQuery
  // input, so an isolated build tree must carry the declared dependency with it.
  fs.cpSync(path.join(ROOT, 'node_modules', 'jquery'), path.join(temp, 'node_modules', 'jquery'), {
    recursive: true,
    filter: (source) => {
      const relative = path.relative(path.join(ROOT, 'node_modules', 'jquery'), source);
      return relative === '' || relative === 'package.json' || relative.split(path.sep)[0] === 'dist';
    },
  });
  return temp;
}

test('manifest declares the complete 34-output contract', async () => {
  const { MANIFEST, BUILD_OUTPUTS, validateManifest } = await import('../../scripts/build/manifest.mjs');
  assert.equal(BUILD_OUTPUTS.length, 34);
  assert.equal(new Set(BUILD_OUTPUTS).size, 34);
  assert.equal(MANIFEST.i18nDerivedInputExclusions.includes('public/md/main-v2.min.js'), true);
  validateManifest(MANIFEST);
  for (const output of BUILD_OUTPUTS) {
    execFileSync('git', ['ls-files', '--error-unmatch', output], { cwd: ROOT, stdio: 'pipe' });
  }
  const nonCanonical = { ...MANIFEST, js: MANIFEST.js.map((entry, index) => index === 0 ? { ...entry, output: 'public//js/dep.min.js' } : entry) };
  assert.throws(() => validateManifest(nonCanonical), /invalid output/);
});

test('node guard rejects unsupported versions before build', async () => {
  const { assertSupportedNode } = await import('../../scripts/build/index.mjs');
  assert.throws(() => assertSupportedNode('23.11.0'), /Node 24/);
  assert.doesNotThrow(() => assertSupportedNode('24.19.0'));
  assert.throws(() => assertSupportedNode('25.0.0'), /Node 24/);
});

test('note template renderer applies only explicit transformations', async () => {
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const { renderNoteHtml } = await import('../../scripts/build/note-html.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  await renderNoteHtml(ROOT, temp, MANIFEST);
  const output = fs.readFileSync(path.join(temp, 'app/views/note/note.html'), 'utf8');
  assert.doesNotMatch(output, /<!-- dev -->|<!-- pro_/);
  assert.match(output, /\/js\/dep\.min\.js/);
  assert.match(output, /\/js\/app\.min\.js/);
  assert.match(output, /\/js\/markdown-v2\.min\.js/);
  assert.match(output, /\/public\/js\/plugins\/main\.min\.js/);
  assert.equal((output.match(/console\.log\(o\);/g) ?? []).length, 0);
});

test('i18n scanner excludes generated markdown derivative', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const scan = await scanI18nSources(ROOT, MANIFEST);
  assert.equal(scan.keys.some((key) => key.path === 'public/md/main-v2.min.js'), false);
  assert.equal(scan.keys.some((key) => key.key === '{{msg . '), false);
  assert.equal(scan.dynamic.some((item) => item.path === 'public/md/main-v2.js' && item.line === 17417), true);
  assert.deepEqual(scan.dynamic.map(({ path, line, column }) => ({ path, line, column })), [
    { path: 'public/js/common.js', line: 1164, column: 11 },
    { path: 'public/md/main-v2.js', line: 17417, column: 23 },
  ]);
});

test('build publication failure restores the complete output set', async () => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const { BUILD_OUTPUTS } = await import('../../scripts/build/manifest.mjs');
  const crypto = require('node:crypto');
  const temp = copyBuildTree();
  const hashes = Object.fromEntries(BUILD_OUTPUTS.map((relative) => [relative, crypto.createHash('sha256').update(fs.readFileSync(path.join(temp, relative))).digest('hex')]));
  await assert.rejects(() => runBuild(temp, { failAfter: 1 }), /injected publish failure/);
  for (const [relative, expected] of Object.entries(hashes)) {
    const actual = crypto.createHash('sha256').update(fs.readFileSync(path.join(temp, relative))).digest('hex');
    assert.equal(actual, expected, `rollback restored ${relative}`);
  }
  assert.equal(fs.readdirSync(temp).some((name) => name.startsWith('.build-backup-')), false);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n scanner fails closed when a scan root is missing', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const manifest = { ...MANIFEST, i18nScanRoots: [...MANIFEST.i18nScanRoots, 'missing-i18n-root'] };
  await assert.rejects(() => scanI18nSources(ROOT, manifest), /cannot scan i18n root missing-i18n-root/);
});

test('build rejects a missing declared input before staging', async () => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = copyBuildTree();
  const missing = MANIFEST.js[0].inputs[0];
  fs.unlinkSync(path.join(temp, missing));
  try {
    await assert.rejects(() => runBuild(temp), new RegExp(`missing input ${missing.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&')}`));
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
});

test('i18n scanner does not bless a dynamic call near a static call', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), "getMsg(mode); getMsg('Static');\n");
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  await assert.rejects(() => scanI18nSources(temp, manifest), /unregistered dynamic i18n key/);
});

test('i18n scanner ignores calls inside strings and comments', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), [
    'const text = "getMsg(mode)";',
    '// getMsg(mode)',
    'const text2 = "msg: \'fake\'";',
    'const key = getMsg("foo" /* ignored comment */);',
  ].join('\n'));
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  const scan = await scanI18nSources(temp, manifest);
  assert.equal(scan.dynamic.length, 0);
  assert.equal(scan.keys.some((item) => item.key === 'foo'), true);
  assert.equal(scan.keys.some((item) => item.key === 'fake'), false);
});

test('i18n scanner rejects string concatenation as a dynamic key with a column', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), 'getMsg("foo" + mode);\n');
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  await assert.rejects(() => scanI18nSources(temp, manifest), /src\/calls\.js:1:1/);
});

test('i18n generation consumes the manifest message input path', async () => {
  const { buildI18n } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const manifest = {
    ...MANIFEST,
    i18n: MANIFEST.i18n.map((entry) => entry.locale === 'en-us' && entry.namespace === 'msg'
      ? { ...entry, inputs: ['messages/en-us/not-declared.conf', ...entry.inputs.slice(1)] }
      : entry),
  };
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  await assert.rejects(() => buildI18n(ROOT, temp, manifest), /message inputs incomplete|not declared/);
});

test('i18n scanner rejects a symlinked scan root', async (t) => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-outside-'));
  try { fs.symlinkSync(outside, path.join(temp, 'linked-src'), 'junction'); } catch (error) {
    if (['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) return t.skip(`symlink unavailable: ${error.code}`);
    throw error;
  }
  const manifest = { ...MANIFEST, i18nScanRoots: ['linked-src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  await assert.rejects(() => scanI18nSources(temp, manifest), /symbolic link|escapes repository/);
});

test('build rejects pre-existing staging and backup symlinks', async (t) => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const temp = copyBuildTree();
  const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-outside-'));
  const staging = path.join(temp, '.staging-link');
  const backup = path.join(temp, '.backup-link');
  try {
    try { fs.symlinkSync(outside, staging, 'junction'); } catch (error) {
      if (['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) return t.skip(`symlink unavailable: ${error.code}`);
      throw error;
    }
    await assert.rejects(() => runBuild(temp, { stagingRoot: staging }), /staging already exists|staging must be a real directory/);
    fs.unlinkSync(staging);
    fs.symlinkSync(outside, backup, 'junction');
    await assert.rejects(() => runBuild(temp, { backupRoot: backup }), /backup already exists|backup must be a real directory/);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
    fs.rmSync(outside, { recursive: true, force: true });
  }
});

test('i18n generation rejects a symlinked message input', async (t) => {
  const { buildI18n } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = copyBuildTree();
  const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-outside-'));
  const message = path.join(temp, 'messages', 'en-us', 'msg.conf');
  const outsideMessage = path.join(outside, 'msg.conf');
  fs.writeFileSync(outsideMessage, fs.readFileSync(message));
  fs.unlinkSync(message);
  try {
    try { fs.symlinkSync(outsideMessage, message, 'file'); } catch (error) {
      if (['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) return t.skip(`symlink unavailable: ${error.code}`);
      throw error;
    }
    await assert.rejects(() => buildI18n(temp, temp, MANIFEST), /symbolic link|message input/);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
    fs.rmSync(outside, { recursive: true, force: true });
  }
});

test('build rejects a symlinked declared output before staging', async (t) => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const temp = copyBuildTree();
  const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-outside-'));
  const output = path.join(temp, 'public', 'js', 'dep.min.js');
  const outsideOutput = path.join(outside, 'dep.min.js');
  fs.writeFileSync(outsideOutput, 'outside');
  fs.unlinkSync(output);
  try {
    try { fs.symlinkSync(outsideOutput, output, 'file'); } catch (error) {
      if (['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) return t.skip(`symlink unavailable: ${error.code}`);
      throw error;
    }
    await assert.rejects(() => runBuild(temp), /output path is symbolic link|output escapes repository/);
    assert.equal(fs.readFileSync(outsideOutput, 'utf8'), 'outside');
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
    fs.rmSync(outside, { recursive: true, force: true });
  }
});

test('rollback failure preserves the backup for recovery', async () => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const temp = copyBuildTree();
  let renameCalls = 0;
  const rename = async (from, to) => {
    renameCalls += 1;
    if (renameCalls === 4) throw new Error('restore blocked');
    return fs.promises.rename(from, to);
  };
  try {
    await assert.rejects(() => runBuild(temp, { failAfter: 1, rename }), /restore blocked.*backup preserved/);
    assert.equal(fs.readdirSync(temp).some((name) => name.startsWith('.build-backup-')), true);
  } finally {
    for (const name of fs.readdirSync(temp).filter((item) => item.startsWith('.build-backup-'))) fs.rmSync(path.join(temp, name), { recursive: true, force: true });
    fs.rmSync(temp, { recursive: true, force: true });
  }
});

test('backup cleanup failure is reported without deleting recovery material', async () => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const temp = copyBuildTree();
  const remove = async (target) => {
    if (path.basename(target).startsWith('.build-backup-')) throw new Error('backup cleanup blocked');
    return fs.promises.rm(target, { recursive: true, force: true });
  };
  try {
    await assert.rejects(() => runBuild(temp, { remove }), /build cleanup failed: .*backup cleanup blocked/);
    assert.equal(fs.readdirSync(temp).some((name) => name.startsWith('.build-backup-')), true);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
});

test('JavaScript generator rejects an empty bundle', async () => {
  const { buildJavaScript } = await import('../../scripts/build/js.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.writeFileSync(path.join(temp, 'empty.js'), '  \n');
  await assert.rejects(() => buildJavaScript({ name: 'empty', transform: 'concat', inputs: ['empty.js'], output: 'empty.out.js' }, temp, temp), /empty JavaScript bundle/);
});

test('JavaScript concat generator rejects invalid syntax', async () => {
  const { buildJavaScript } = await import('../../scripts/build/js.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.writeFileSync(path.join(temp, 'invalid.js'), 'function ( {\n');
  await assert.rejects(() => buildJavaScript({ name: 'invalid', transform: 'concat', inputs: ['invalid.js'], output: 'invalid.out.js' }, temp, temp), /Expected|invalid/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n fixture rejects translation drift', async () => {
  const { buildI18n } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const fixture = JSON.parse(fs.readFileSync(path.join(ROOT, 'tests/js/fixtures/build/i18n-contract.json'), 'utf8'));
  fixture.messages['en-us'].msg.login = `${fixture.messages['en-us'].msg.login} drift`;
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  await assert.rejects(() => buildI18n(ROOT, temp, MANIFEST, fixture), /message contract changed for en-us\/msg/);
});

test('note template renderer rejects an invalid production script order', async () => {
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const { renderNoteHtml } = await import('../../scripts/build/note-html.mjs');
  const temp = copyBuildTree();
  const templatePath = path.join(temp, 'app/views/note/note-dev.html');
  const template = fs.readFileSync(templatePath, 'utf8')
    .replace('<!-- pro_dep_js -->', '<!-- swapped-dep-marker -->')
    .replace('<!-- pro_app_js -->', '<!-- pro_dep_js -->')
    .replace('<!-- swapped-dep-marker -->', '<!-- pro_app_js -->');
  fs.writeFileSync(templatePath, template);
  await assert.rejects(() => renderNoteHtml(temp, temp, MANIFEST), /production script order violation/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('sanitized Playwright reporter serializes concurrent writes', async () => {
  const { default: SanitizedSummaryReporter } = await import('../../tests/e2e/build/sanitized-summary-reporter.mjs');
  const reportDir = path.join(ROOT, 'test-results');
  fs.rmSync(reportDir, { recursive: true, force: true });
  const reporter = new SanitizedSummaryReporter();
  await Promise.all([
    reporter.onBegin(),
    reporter.onError(new Error('fixture failed')),
    reporter.onError(Object.assign(new Error('browser missing'), { name: 'TimeoutError' })),
  ]);
  await reporter.onEnd({ status: 'failed' });
  const summary = JSON.parse(fs.readFileSync(path.join(reportDir, 'build-smoke-summary.json'), 'utf8'));
  assert.match(summary.stage, /failed$/);
  assert.equal(summary.errors.length, 2);
  fs.rmSync(reportDir, { recursive: true, force: true });
});

test('i18n scanner handles template expressions and ignores regex/html comments', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), [
    'const a = `${getMsg(mode)}`;',
    'const b = /getMsg(mode)/;',
    '<!-- getMsg(mode) -->',
    'const c = {msg: "fake"};',
  ].join('\n'));
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  await assert.rejects(() => scanI18nSources(temp, manifest), /src\/calls\.js:1:/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n scanner ignores regex literals after expression keywords', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), [
    'function pattern() { return /getMsg(mode)/; }',
    'getMsg("real");',
  ].join('\n'));
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  const scan = await scanI18nSources(temp, manifest);
  assert.deepEqual(scan.keys.map((item) => item.key), ['real']);
  assert.deepEqual(scan.dynamic, []);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n scanner rejects extra arguments after a regex data expression', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), 'getMsg("foo", /\\),/.test(value), extra);\n');
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  await assert.rejects(() => scanI18nSources(temp, manifest), /src\/calls\.js:1:1/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n scanner ignores template-like text in comments and strings', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  const source = [
    'const actual = `${getMsg(mode)}`;',
    '// `${getMsg(other)}`',
    'const quoted = `${"getMsg(quoted)"}`;',
    'const commented = `${/* getMsg(commented) */ mode}`;',
  ].join('\n');
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), source);
  const manifest = {
    ...MANIFEST,
    i18nScanRoots: ['src'],
    i18nDerivedInputExclusions: [],
    dynamicKeyExceptions: [{ path: 'src/calls.js', line: 1, column: source.indexOf('getMsg') + 1 }],
  };
  const scan = await scanI18nSources(temp, manifest);
  assert.deepEqual(scan.dynamic, manifest.dynamicKeyExceptions);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n scanner rejects extra getMsg arguments without treating them as static', async () => {
  const { scanI18nSources } = await import('../../scripts/build/i18n.mjs');
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.mkdirSync(path.join(temp, 'src'));
  fs.writeFileSync(path.join(temp, 'src', 'calls.js'), 'getMsg("foo", a, b);\n');
  const manifest = { ...MANIFEST, i18nScanRoots: ['src'], i18nDerivedInputExclusions: [], dynamicKeyExceptions: [] };
  await assert.rejects(() => scanI18nSources(temp, manifest), /src\/calls\.js:1:/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('i18n fixture input symlink is rejected before parsing', async (t) => {
  const { runBuild } = await import('../../scripts/build/index.mjs');
  const temp = copyBuildTree();
  const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-outside-'));
  const fixture = path.join(temp, 'tests', 'js', 'fixtures', 'build', 'i18n-contract.json');
  const outsideFixture = path.join(outside, 'i18n-contract.json');
  fs.writeFileSync(outsideFixture, fs.readFileSync(fixture));
  fs.unlinkSync(fixture);
  try {
    try { fs.symlinkSync(outsideFixture, fixture, 'file'); } catch (error) {
      if (['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) return t.skip(`symlink unavailable: ${error.code}`);
      throw error;
    }
    await assert.rejects(() => runBuild(temp), /fixture.*symbolic link|fixture.*escapes repository/);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
    fs.rmSync(outside, { recursive: true, force: true });
  }
});

test('CSS generator emits a non-empty minified bundle', async () => {
  const { buildCss } = await import('../../scripts/build/css.mjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leanote-build-test-'));
  fs.writeFileSync(path.join(temp, 'style.css'), '.foo { color: red; }');
  const output = await buildCss({ name: 'css', inputs: ['style.css'], output: 'out.css' }, temp, temp);
  assert.match(fs.readFileSync(output, 'utf8'), /\.foo\{color:red\}/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('note renderer derives production URLs from manifest entries', async () => {
  const { MANIFEST } = await import('../../scripts/build/manifest.mjs');
  const { renderNoteHtml } = await import('../../scripts/build/note-html.mjs');
  const temp = copyBuildTree();
  const manifest = { ...MANIFEST, js: MANIFEST.js.map((entry) => entry.name === 'app' ? { ...entry, url: '/js/app-v2.min.js' } : entry) };
  await renderNoteHtml(temp, temp, manifest);
  const output = fs.readFileSync(path.join(temp, 'app', 'views', 'note', 'note.html'), 'utf8');
  assert.match(output, /\/js\/app-v2\.min\.js/);
  assert.doesNotMatch(output, /\/js\/app\.min\.js/);
  fs.rmSync(temp, { recursive: true, force: true });
});

test('sanitized reporter drops unknown sensitive fields', async () => {
  const { sanitizeSummary } = await import('../../tests/e2e/build/sanitized-summary-reporter.mjs');
  const input = { stage: 'runner-started', headers: { authorization: 'secret' }, body: 'private', errors: ['token=abc'], pages: [{ url: '/note', status: 200, extra: 'x' }] };
  const output = sanitizeSummary(input);
  assert.equal(Object.hasOwn(output, 'headers'), false);
  assert.equal(Object.hasOwn(output, 'body'), false);
  assert.deepEqual(output.pages, [{ url: '/note', status: 200 }]);
});
