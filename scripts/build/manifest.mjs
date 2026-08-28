import fs from 'node:fs';
import path from 'node:path';

const posix = (...parts) => parts.join('/').replaceAll('\\', '/').replace(/^\.\//, '');

// blueimp-file-upload is consumed verbatim from the npm dist (see
// docs/modernization/fileupload-provenance.md). The AMD branches of its
// factory wrappers must stay dormant inside concatenated bundles, so the
// build wraps these inputs with a `define` shadow (scripts/build/js.mjs).
const fileuploadInputs = [
  'node_modules/blueimp-file-upload/js/vendor/jquery.ui.widget.js',
  'node_modules/blueimp-file-upload/js/jquery.iframe-transport.js',
  'node_modules/blueimp-file-upload/js/jquery.fileupload.js',
];

const js = [
  { name: 'jquery-runtime', kind: 'js', transform: 'concat', inputs: [
    'node_modules/jquery/dist/jquery.min.js',
  ], output: 'public/js/jquery-1.9.0.min.js', url: '/js/jquery-1.9.0.min.js' },
  { name: 'dep', kind: 'js', transform: 'concat', inputs: [
    'node_modules/jquery/dist/jquery.min.js',
    'public/js/jquery.ztree.all-3.5-min.js',
    'public/js/jQuery-slimScroll-1.3.0/jquery.slimscroll-min.js',
    'public/js/contextmenu/jquery.contextmenu-min.js',
    'public/js/bootstrap-min.js',
    'public/js/object_id.js',
  ], output: 'public/js/dep.min.js', url: '/js/dep.min.js' },
  { name: 'app', kind: 'js', transform: 'esbuild-concat', inputs: [
    'public/js/common.js', 'public/js/app/note.js', 'public/js/app/page.js',
    'public/js/app/tag.js', 'public/js/app/notebook.js', 'public/js/app/share.js',
  ], output: 'public/js/app.min.js', url: '/js/app.min.js' },
  { name: 'plugins', kind: 'js', transform: 'esbuild-concat', inputs: [
    ...fileuploadInputs,
    'public/js/plugins/note_info.js', 'public/js/plugins/tips.js',
    'public/js/plugins/history.js', 'public/js/plugins/attachment_upload.js',
    'public/js/plugins/editor_drop_paste.js', 'public/js/plugins/main.js',
  ], amdGuard: fileuploadInputs, output: 'public/js/plugins/main.min.js', url: '/public/js/plugins/main.min.js' },
  { name: 'markdown', kind: 'js', transform: 'esbuild-concat', inputs: [
    'public/js/require.js', 'public/md/main-v2.min.js',
  ], output: 'public/js/markdown-v2.min.js', url: '/js/markdown-v2.min.js' },
  { name: 'album', kind: 'js', transform: 'esbuild-concat', inputs: [
    'node_modules/jquery/dist/jquery.min.js', 'public/js/bootstrap-min.js',
    ...fileuploadInputs, 'public/js/jquery.pagination.js',
    'public/album/js/main.js',
  ], amdGuard: fileuploadInputs, output: 'public/album/js/main.all.js', url: '/public/album/js/main.all.js' },
];

const css = [
  { name: 'album-css', kind: 'css', inputs: ['public/album/css/style.css'], output: 'public/album/css/style-min.css', url: '/public/album/css/style-min.css' },
  { name: 'bootstrap-css', kind: 'css', inputs: ['public/css/bootstrap.css'], output: 'public/css/bootstrap-min.css', url: '/css/bootstrap-min.css' },
  { name: 'font-awesome-css', kind: 'css', inputs: ['public/css/font-awesome-4.2.0/css/font-awesome.css'], output: 'public/css/font-awesome-4.2.0/css/font-awesome-min.css', url: '/css/font-awesome-4.2.0/css/font-awesome-min.css' },
  { name: 'ztree-css', kind: 'css', inputs: ['public/css/zTreeStyle/zTreeStyle.css'], output: 'public/css/zTreeStyle/zTreeStyle-min.css', url: '/css/zTreeStyle/zTreeStyle-min.css' },
  { name: 'markdown-css', kind: 'css', inputs: ['public/md/themes/default.css'], output: 'public/md/themes/default-min.css', url: '/public/md/themes/default-min.css' },
  { name: 'contextmenu-css', kind: 'css', inputs: ['public/js/contextmenu/css/contextmenu.css'], output: 'public/js/contextmenu/css/contextmenu-min.css', url: '/js/contextmenu/css/contextmenu-min.css' },
];

const locales = ['de-de', 'en-us', 'es-co', 'fr-fr', 'pt-pt', 'zh-cn', 'zh-hk'];
const i18n = locales.flatMap((locale) => [
  { name: `msg-${locale}`, kind: 'i18n', namespace: 'msg', locale, inputs: ['msg', 'member', 'markdown', 'album'].map((name) => `messages/${locale}/${name}.conf`), output: `public/js/i18n/msg.${locale}.js`, url: `/js/i18n/msg.${locale}.js` },
  { name: `blog-${locale}`, kind: 'i18n', namespace: 'blog', locale, inputs: [`messages/${locale}/blog.conf`], output: `public/js/i18n/blog.${locale}.js`, url: `/js/i18n/blog.${locale}.js` },
  { name: `tinymce-${locale}`, kind: 'i18n', namespace: 'tinymce', locale, inputs: [`messages/${locale}/tinymce_editor.conf`], output: `public/tinymce/langs/${locale}.js`, url: `/tinymce/langs/${locale}.js` },
]);

const manifest = {
  version: 1,
  js,
  css,
  i18n,
  noteHtml: { name: 'note-html', kind: 'html', inputs: ['app/views/note/note-dev.html'], output: 'app/views/note/note.html' },
  locales,
  i18nScanRoots: ['public/admin', 'public/blog', 'public/md', 'public/js', 'public/album', 'public/libs', 'public/member', 'public/tinymce', 'app/views'],
  i18nDerivedInputExclusions: [
    ...js.map((entry) => entry.output), ...css.map((entry) => entry.output),
    ...i18n.map((entry) => entry.output), 'app/views/note/note.html', 'public/md/main-v2.min.js',
  ],
  i18nMessageFiles: ['msg', 'member', 'markdown', 'album', 'blog', 'tinymce_editor'],
  dynamicKeyExceptions: [
    { path: 'public/js/common.js', line: 1164, column: 11 },
    { path: 'public/md/main-v2.js', line: 17417, column: 23 },
  ],
};

function validateRelative(value, label) {
  if (typeof value !== 'string' || !value || path.posix.isAbsolute(value) || path.win32.isAbsolute(value)) {
    throw new Error(`invalid ${label}: ${value}`);
  }
  if (value.includes('\\') || value.includes('\0')) throw new Error(`invalid ${label}: ${value}`);
  const normalized = posix(value);
  if (normalized !== value || path.posix.normalize(value) !== value || normalized.split('/').includes('..')) {
    throw new Error(`invalid ${label}: ${value}`);
  }
  return normalized;
}

export function validateManifest(input = manifest) {
  const outputs = [];
  const canonicalOutputs = new Set([...manifest.js, ...manifest.css, ...manifest.i18n, manifest.noteHtml].map((entry) => entry.output));
  const entries = [...input.js, ...input.css, ...input.i18n, input.noteHtml];
  for (const entry of entries) {
    const output = validateRelative(entry.output, 'output');
    if (!canonicalOutputs.has(output)) throw new Error(`output is outside canonical manifest: ${output}`);
    if (!Array.isArray(entry.inputs) || entry.inputs.length === 0) throw new Error(`empty inputs for ${entry.name}`);
    if (outputs.includes(output)) throw new Error(`duplicate output: ${output}`);
    outputs.push(output);
    for (const source of entry.inputs ?? []) validateRelative(source, 'input');
    if (entry.amdGuard !== undefined) {
      if (!Array.isArray(entry.amdGuard)) throw new Error(`amdGuard must be an array for ${entry.name}`);
      for (const guarded of entry.amdGuard) {
        validateRelative(guarded, 'amdGuard input');
        if (!(entry.inputs ?? []).includes(guarded)) throw new Error(`amdGuard input is not declared for ${entry.name}: ${guarded}`);
      }
    }
  }
  if (outputs.length !== 34) throw new Error(`expected 34 outputs, got ${outputs.length}`);
  for (const root of input.i18nScanRoots) validateRelative(root, 'scan root');
  const exclusions = input.i18nDerivedInputExclusions ?? [];
  const normalizedExclusions = exclusions.map((item) => validateRelative(item, 'i18n derived input exclusion'));
  if (new Set(normalizedExclusions).size !== normalizedExclusions.length) throw new Error('duplicate i18n derived input exclusion');
  for (const output of outputs) {
    if (!normalizedExclusions.includes(output)) throw new Error(`i18n scan must exclude generated output: ${output}`);
  }
  if (!normalizedExclusions.includes('public/md/main-v2.min.js')) throw new Error('i18n scan must exclude public/md/main-v2.min.js');
  for (const item of input.dynamicKeyExceptions ?? []) {
    validateRelative(item.path, 'dynamic key exception path');
    if (!Number.isInteger(item.line) || item.line < 1 || !Number.isInteger(item.column) || item.column < 1) throw new Error(`invalid dynamic key exception locator: ${item.path}:${item.line}:${item.column}`);
  }
  return input;
}

validateManifest(manifest);
export const BUILD_OUTPUTS = Object.freeze([...manifest.js, ...manifest.css, ...manifest.i18n, manifest.noteHtml].map((entry) => entry.output));
function deepFreeze(value) {
  if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
  Object.freeze(value);
  for (const child of Object.values(value)) deepFreeze(child);
  return value;
}
export const MANIFEST = deepFreeze(manifest);
export function resolveRepoPath(root, relative) {
  const value = validateRelative(relative, 'path');
  const resolved = path.resolve(root, ...value.split('/'));
  const rootResolved = path.resolve(root) + path.sep;
  if (resolved !== path.resolve(root) && !resolved.startsWith(rootResolved)) throw new Error(`path escapes repository: ${relative}`);
  return resolved;
}

export function assertNoSymlinkPath(root, relative, label = 'path') {
  const repository = fs.realpathSync(root);
  const lexicalRoot = path.resolve(root);
  const target = resolveRepoPath(root, relative);
  for (let current = target; current.startsWith(lexicalRoot); current = path.dirname(current)) {
    if (!fs.existsSync(current)) {
      if (current === lexicalRoot) break;
      continue;
    }
    const stat = fs.lstatSync(current);
    if (stat.isSymbolicLink()) throw new Error(`${label} is symbolic link: ${relative}`);
    const real = fs.realpathSync(current);
    if (real !== repository && !real.startsWith(`${repository}${path.sep}`)) {
      throw new Error(`${label} escapes repository: ${relative}`);
    }
    if (current === lexicalRoot) break;
  }
  return target;
}

export function assertInputsExist(root, entries = [...manifest.js, ...manifest.css, ...manifest.i18n, manifest.noteHtml]) {
  const repository = fs.realpathSync(root);
  if (fs.lstatSync(root).isSymbolicLink()) throw new Error('repository root is symbolic link');
  for (const entry of entries) for (const source of entry.inputs ?? []) {
    const file = assertNoSymlinkPath(root, source, `input for ${entry.name}`);
    if (!fs.existsSync(file) || !fs.lstatSync(file).isFile()) throw new Error(`missing input ${source} for ${entry.name}`);
    const real = fs.realpathSync(file);
    if (real !== repository && !real.startsWith(repository + path.sep)) throw new Error(`input escapes repository ${source} for ${entry.name}`);
  }
}

export function assertOutputsSafe(root, entries = [...manifest.js, ...manifest.css, ...manifest.i18n, manifest.noteHtml]) {
  const repository = fs.realpathSync(root);
  const lexicalRoot = path.resolve(root);
  for (const entry of entries) {
    const output = resolveRepoPath(root, entry.output);
    for (let current = output; current.startsWith(lexicalRoot); current = path.dirname(current)) {
      if (fs.existsSync(current)) {
        const stat = fs.lstatSync(current);
        if (stat.isSymbolicLink()) throw new Error(`output path is symbolic link: ${entry.output}`);
        const real = fs.realpathSync(current);
        if (real !== repository && !real.startsWith(repository + path.sep)) {
          throw new Error(`output escapes repository ${entry.output}`);
        }
      }
    }
  }
}
