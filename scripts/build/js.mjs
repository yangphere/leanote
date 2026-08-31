import fs from 'node:fs/promises';
import path from 'node:path';
import { transform } from 'esbuild';
import { resolveRepoPath } from './manifest.mjs';

function stripSourceMappingURL(source) {
  return source
    .replace(/^\s*\/\/[#@]\s*sourceMappingURL=[^\r\n]+\s*$/gm, '')
    .replace(/^\s*\/\*[#@]\s*sourceMappingURL=[^*]+\*\/\s*$/gm, '')
    .replace(/\n{3,}/g, '\n\n');
}

export async function buildJavaScript(entry, root, stagingRoot) {
  if (!Array.isArray(entry.inputs) || entry.inputs.length === 0) throw new Error(`empty JavaScript inputs for ${entry.name}`);
  const guardedInputs = new Set(entry.amdGuard ?? []);
  const knownInputs = new Set(entry.inputs);
  for (const guarded of guardedInputs) {
    if (!knownInputs.has(guarded)) throw new Error(`amdGuard input is not declared for ${entry.name}: ${guarded}`);
  }
  const chunks = [];
  for (const relative of entry.inputs) {
    const sourcePath = resolveRepoPath(root, relative);
    let source = (await fs.readFile(sourcePath, 'utf8')).replace(/\r\n?/g, '\n');
    if (entry.stripSourceMappingURL) source = stripSourceMappingURL(source);
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
    let code = result.code;
    if (guardedInputs.has(relative)) {
      // Shadow `define` so upstream AMD detections resolve to the browser-globals
      // branch while the bundle executes as a plain script.
      code = `(function(define){${code}})(void 0);`;
    }
    chunks.push(code);
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

export async function buildStaticEntries(entries, root, stagingRoot) {
  for (const entry of entries) {
    if (!entry || entry.transform !== 'copy' || !Array.isArray(entry.inputs) || entry.inputs.length !== 1) {
      throw new Error(`invalid static asset ${entry?.name || 'unknown'}`);
    }
    const sourcePath = resolveRepoPath(root, entry.inputs[0]);
    const outputPath = resolveRepoPath(stagingRoot, entry.output);
    await fs.mkdir(path.dirname(outputPath), { recursive: true });
    if (entry.normalizeTrailingWhitespace) {
      const source = (await fs.readFile(sourcePath, 'utf8')).replace(/\r\n?/g, '\n');
      await fs.writeFile(outputPath, source.replace(/[ \t]+$/gm, ''), 'utf8');
    } else {
      await fs.copyFile(sourcePath, outputPath);
    }
  }
}
