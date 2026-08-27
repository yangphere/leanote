import fs from 'node:fs/promises';
import path from 'node:path';
import { transform } from 'esbuild';
import { resolveRepoPath } from './manifest.mjs';

export async function buildCss(entry, root, stagingRoot) {
  if (!Array.isArray(entry.inputs) || entry.inputs.length === 0) throw new Error(`empty CSS inputs for ${entry.name}`);
  const sourcePath = resolveRepoPath(root, entry.inputs[0]);
  const source = await fs.readFile(sourcePath, 'utf8');
  const result = await transform(source, {
    loader: 'css', minify: true, sourcemap: false,
    legalComments: 'none', sourcefile: entry.inputs[0],
  });
  const outputPath = resolveRepoPath(stagingRoot, entry.output);
  if (!result.code.trim()) throw new Error(`empty CSS bundle for ${entry.name}`);
  await fs.mkdir(path.dirname(outputPath), { recursive: true });
  await fs.writeFile(outputPath, `${result.code.trim()}\n`, 'utf8');
  return outputPath;
}

export async function buildCssEntries(entries, root, stagingRoot) {
  for (const entry of entries) await buildCss(entry, root, stagingRoot);
}
