const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const ROOT = path.resolve(__dirname, '../..');
const RUNTIME_INPUT = 'node_modules/jquery/dist/jquery.min.js';
const RUNTIME_OUTPUT = 'public/js/jquery-1.9.0.min.js';

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

function execGitLsFiles(relative) {
  const { execFileSync } = require('node:child_process');
  execFileSync('git', ['ls-files', '--error-unmatch', relative], { cwd: ROOT, stdio: 'pipe' });
}
