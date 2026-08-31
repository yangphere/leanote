import fs from 'node:fs/promises';
import path from 'node:path';
import { resolveRepoPath } from './manifest.mjs';

export async function renderNoteHtml(root, stagingRoot, manifest) {
  const productionEntry = (name) => {
    const entry = manifest.js.find((candidate) => candidate.name === name);
    if (!entry || typeof entry.url !== 'string' || !entry.url.startsWith('/')) {
      throw new Error(`manifest is missing production URL for ${name}`);
    }
    return entry;
  };
  const dep = productionEntry('dep');
  const app = productionEntry('app');
  const markdown = productionEntry('markdown');
  const plugins = productionEntry('plugins');
  const sourcePath = resolveRepoPath(root, manifest.noteHtml.inputs[0]);
  let source = await fs.readFile(sourcePath, 'utf8');
  const devBlocks = source.match(/<!-- dev -->[\s\S]*?<!-- \/dev -->/g) ?? [];
  if (devBlocks.length !== 3) throw new Error(`expected exactly 3 dev blocks, got ${devBlocks.length}`);
  source = source.replace(/<!-- dev -->[\s\S]*?<!-- \/dev -->/g, '');
  const replacements = [
    ['<!-- pro_dep_js -->', `<script src="${dep.url}"></script>`],
    ['<!-- pro_app_js -->', `<script src="${app.url}"></script>`],
    ['<!-- pro_markdown_js -->', `<script src="${markdown.url}"></script>`],
    ['<!-- pro_tinymce_init_js -->', "var tinyMCEPreInit = {base: '/tinymce', suffix: '.min'};"],
  ];
  for (const [marker, replacement] of replacements) {
    const count = source.split(marker).length - 1;
    if (count !== 1) throw new Error(`expected one ${marker}, got ${count}`);
    source = source.replace(marker, replacement);
  }
  const tinyMatches = source.match(/(<script\s+src=")\/tinymce\/tinymce\.js("[^>]*>)/g) ?? [];
  if (tinyMatches.length !== 1) throw new Error(`expected one TinyMCE development script, got ${tinyMatches.length}`);
  source = source.replace(/(<script\s+src=")\/tinymce\/tinymce\.js("[^>]*>)/, '$1/tinymce/tinymce.min.js$2');
  const pluginMatches = source.match(/(<script\s+src=")\/public\/js\/plugins\/main\.js("[^>]*>)/g) ?? [];
  if (pluginMatches.length !== 1) throw new Error(`expected one plugin script, got ${pluginMatches.length}`);
  source = source.replace(/(<script\s+src=")\/public\/js\/plugins\/main\.js("[^>]*>)/, `$1${plugins.url}$2`);
  const consoleLogs = source.match(/console\.log\(o\);/g) ?? [];
  if (consoleLogs.length !== 1) throw new Error(`expected one console.log(o), got ${consoleLogs.length}`);
  source = source.replace('console.log(o);', '');
  if (/<!-- (?:dev|\/dev|pro_[^ ]+) -->/.test(source)) throw new Error('generated note.html contains unresolved build marker');
  const scriptSources = [...source.matchAll(/<script\s+src="([^"]+)"[^>]*>/g)].map((match) => match[1]);
  const requiredOrder = [
    dep.url,
    '/tinymce/tinymce.min.js',
    app.url,
    markdown.url,
    plugins.url,
  ];
  let previous = -1;
  for (const expected of requiredOrder) {
    const matches = scriptSources.reduce((count, value) => count + (value === expected ? 1 : 0), 0);
    if (matches !== 1) throw new Error(`expected one production script ${expected}, got ${matches}`);
    const current = scriptSources.indexOf(expected);
    if (current <= previous) throw new Error(`production script order violation at ${expected}`);
    previous = current;
  }
  const outputPath = resolveRepoPath(stagingRoot, manifest.noteHtml.output);
  await fs.mkdir(path.dirname(outputPath), { recursive: true });
  // Keep template content intact apart from deterministic whitespace normalization.
  const normalized = source.replaceAll('\r\n', '\n').replaceAll('\r', '\n').replace(/[ \t]+(?=\n)/g, '').replace(/ +\t/g, '\t');
  await fs.writeFile(outputPath, normalized.endsWith('\n') ? normalized : `${normalized}\n`, 'utf8');
  return outputPath;
}
