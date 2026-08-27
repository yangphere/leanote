import fs from 'node:fs/promises';
import path from 'node:path';
import { transform } from 'esbuild';
import { resolveRepoPath } from './manifest.mjs';

export async function buildJavaScript(entry, root, stagingRoot) {
  if (!Array.isArray(entry.inputs) || entry.inputs.length === 0) throw new Error(`empty JavaScript inputs for ${entry.name}`);
  const chunks = [];
  for (const relative of entry.inputs) {
    const sourcePath = resolveRepoPath(root, relative);
    const source = (await fs.readFile(sourcePath, 'utf8')).replace(/\r\n?/g, '\n');
    if (entry.transform === 'concat') {
      // Validate legacy concatenation inputs without changing their bytes or execution order.
      await transform(source, {
        loader: 'js',
        minify: false,
        sourcemap: false,
        target: 'es2015',
        sourcefile: relative,
      });
      chunks.push(source);
      continue;
    }
    const result = await transform(source, {
      loader: 'js',
      minify: true,
      sourcemap: false,
      target: 'es2015',
      legalComments: 'none',
      sourcefile: relative,
    });
    chunks.push(result.code);
  }
  const outputPath = resolveRepoPath(stagingRoot, entry.output);
  if (!chunks.some((chunk) => chunk.trim())) throw new Error(`empty JavaScript bundle for ${entry.name}`);
  await fs.mkdir(path.dirname(outputPath), { recursive: true });
  let output = '';
  for (const chunk of chunks) {
    if (output && !output.endsWith('\n')) output += '\n';
    output += chunk;
  }
  if (!output.endsWith('\n')) output += '\n';
  await fs.writeFile(outputPath, output, 'utf8');
  return outputPath;
}

export async function buildJavaScriptEntries(entries, root, stagingRoot) {
  for (const entry of entries) await buildJavaScript(entry, root, stagingRoot);
}
