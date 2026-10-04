import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { assertNoSymlinkPath, resolveRepoPath } from './manifest.mjs';

export function assertSupportedNode(version) {
  const major = Number.parseInt(String(version).split('.')[0], 10);
  if (major !== 24) throw new Error(`Node ${version} is unsupported; build requires Node 24.x (>=24 <25)`);
  return true;
}

async function readFixture(root) {
  const relative = 'tests/js/fixtures/build/i18n-contract.json';
  let fixturePath;
  try {
    fixturePath = assertNoSymlinkPath(root, relative, 'i18n contract fixture');
  } catch (error) {
    throw new Error(`invalid i18n contract fixture ${relative}: ${error.message}`);
  }
  let raw;
  try {
    raw = await fs.readFile(fixturePath, 'utf8');
  } catch (error) {
    throw new Error(`missing i18n contract fixture ${fixturePath}: ${error.message}`);
  }
  try {
    return JSON.parse(raw);
  } catch (error) {
    throw new Error(`invalid i18n contract fixture ${fixturePath}: ${error.message}`);
  }
}

async function ensureEsbuild() {
  let esbuild;
  try { esbuild = await import('esbuild'); } catch (error) { throw new Error(`local esbuild 0.28.2 is required; install dependencies with npm ci: ${error.message}`); }
  if (esbuild.version !== '0.28.2') throw new Error(`local esbuild version ${esbuild.version} is invalid; expected 0.28.2`);
}

async function removeIfExists(target) {
  let stat;
  try { stat = await fs.lstat(target); } catch (error) { if (error.code === 'ENOENT') return; throw error; }
  if (stat.isSymbolicLink()) { await fs.unlink(target); return; }
  await fs.rm(target, { recursive: true, force: true });
}

async function assertControlledDirectory(root, target, label) {
  const repository = await fs.realpath(root);
  const lexicalRoot = path.resolve(root);
  const lexicalTarget = path.resolve(target);
  if (lexicalTarget !== lexicalRoot && !lexicalTarget.startsWith(`${lexicalRoot}${path.sep}`)) throw new Error(`${label} escapes repository: ${target}`);
  const stat = await fs.lstat(target);
  if (!stat.isDirectory() || stat.isSymbolicLink()) throw new Error(`${label} must be a real directory: ${target}`);
  const realTarget = await fs.realpath(target);
  if (realTarget !== repository && !realTarget.startsWith(`${repository}${path.sep}`)) throw new Error(`${label} escapes repository: ${target}`);
  return realTarget;
}

async function createControlledDirectory(root, label, requested) {
  const target = requested ? path.resolve(requested) : await fs.mkdtemp(path.join(path.resolve(root), `.build-${label}-`));
  if (requested) {
    const lexicalRoot = path.resolve(root);
    if (target !== lexicalRoot && !target.startsWith(`${lexicalRoot}${path.sep}`)) throw new Error(`${label} escapes repository: ${target}`);
    try {
      await fs.lstat(target);
      throw new Error(`${label} already exists: ${target}`);
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
    await fs.mkdir(target);
  }
  try {
    await assertControlledDirectory(root, target, label);
  } catch (error) {
    await removeIfExists(target).catch(() => {});
    throw error;
  }
  return target;
}

async function listFiles(root, relative = '') {
  const directory = relative ? path.join(root, relative) : root;
  const names = await fs.readdir(directory, { withFileTypes: true });
  const files = [];
  for (const item of names) {
    const child = relative ? path.join(relative, item.name) : item.name;
    if (item.isDirectory()) files.push(...await listFiles(root, child));
    else if (item.isFile()) files.push(child.replaceAll('\\', '/'));
    else throw new Error(`unexpected staging entry ${child}`);
  }
  return files;
}

export async function runBuild(root = process.cwd(), options = {}) {
  assertSupportedNode(process.versions.node);
  await ensureEsbuild();
  const { MANIFEST, BUILD_OUTPUTS, assertInputsExist, assertOutputsSafe, resolveRepoPath, validateManifest } = await import('./manifest.mjs');
  const { buildJavaScriptEntries, buildStaticEntries } = await import('./js.mjs');
  const { buildCssEntries } = await import('./css.mjs');
  const { buildI18n } = await import('./i18n.mjs');
  const { renderNoteHtml } = await import('./note-html.mjs');
  validateManifest(MANIFEST);
  assertInputsExist(root);
  assertOutputsSafe(root);
  let staging;
  let backup;
  const published = [];
  const backedUp = [];
  const rename = options.rename ?? fs.rename;
  const remove = options.remove ?? removeIfExists;
  const failAfter = Number.isInteger(options.failAfter) ? options.failAfter : null;
  let failure = null;
  let rollbackErrors = [];
  try {
    staging = await createControlledDirectory(root, 'staging', options.stagingRoot);
    await assertControlledDirectory(root, staging, 'staging');
    await buildJavaScriptEntries(MANIFEST.js, root, staging);
    await buildStaticEntries(MANIFEST.assets, root, staging);
    await assertControlledDirectory(root, staging, 'staging');
    await buildCssEntries(MANIFEST.css, root, staging);
    await assertControlledDirectory(root, staging, 'staging');
    await buildI18n(root, staging, MANIFEST, await readFixture(root));
    await assertControlledDirectory(root, staging, 'staging');
    await renderNoteHtml(root, staging, MANIFEST);
    await assertControlledDirectory(root, staging, 'staging');
    const stagedFiles = new Set(await listFiles(staging));
    const expectedFiles = new Set(BUILD_OUTPUTS);
    const undeclared = [...stagedFiles].filter((relative) => !expectedFiles.has(relative));
    if (undeclared.length) throw new Error(`generator produced undeclared output ${undeclared.join(', ')}`);
    for (const relative of BUILD_OUTPUTS) {
      const staged = resolveRepoPath(staging, relative);
      try { await fs.access(staged); } catch { throw new Error(`generator did not produce declared output ${relative}`); }
    }
    backup = await createControlledDirectory(root, 'backup', options.backupRoot);
    await assertControlledDirectory(root, backup, 'backup');
    for (const relative of BUILD_OUTPUTS) {
      const destination = resolveRepoPath(root, relative);
      const staged = resolveRepoPath(staging, relative);
      const saved = resolveRepoPath(backup, relative);
      await assertControlledDirectory(root, backup, 'backup');
      await fs.mkdir(path.dirname(saved), { recursive: true });
      try { await rename(destination, saved); backedUp.push({ destination, saved }); } catch (error) {
        if (error.code !== 'ENOENT') throw new Error(`backup failed for ${relative}: ${error.message}`);
      }
      assertOutputsSafe(root);
      await fs.mkdir(path.dirname(destination), { recursive: true });
      if (failAfter !== null && published.length >= failAfter) throw new Error(`injected publish failure after ${failAfter} outputs`);
      await rename(staged, destination);
      published.push(destination);
      // Published assets are HTTP-served text tracked at 100644; the mode is
      // fixed here so umask, tarball, and checkout permissions cannot leak
      // into the generated tree (fs.writeFile's mode argument would be
      // masked by umask, so a chmod after publish is required).
      await fs.chmod(destination, 0o644);
    }
  } catch (error) {
    failure = error;
    for (const destination of published.reverse()) {
      try { await remove(destination); } catch (cleanupError) { rollbackErrors.push(`remove ${destination}: ${cleanupError.message}`); }
    }
    for (const { destination, saved } of backedUp.reverse()) {
      try {
        assertOutputsSafe(root);
        await assertControlledDirectory(root, backup, 'backup');
        await fs.mkdir(path.dirname(destination), { recursive: true });
        await rename(saved, destination);
      } catch (restoreError) {
        rollbackErrors.push(`restore ${destination}: ${restoreError.message}`);
      }
    }
  }

  const cleanupErrors = [];
  if (staging) {
    try { await remove(staging); } catch (cleanupError) { cleanupErrors.push(`remove staging ${staging}: ${cleanupError.message}`); }
  }
  // A backup is disposable only after every original output has been restored.
  // Keep it when rollback failed so an operator still has recovery material.
  if (backup && rollbackErrors.length === 0) {
    try { await remove(backup); } catch (cleanupError) { cleanupErrors.push(`remove backup ${backup}: ${cleanupError.message}`); }
  }
  if (failure) {
    const details = [failure.message];
    if (rollbackErrors.length) details.push(`rollback failed: ${rollbackErrors.join('; ')}; backup preserved at ${backup}`);
    if (cleanupErrors.length) details.push(`cleanup failed: ${cleanupErrors.join('; ')}`);
    throw new Error(`build failed: ${details.join('; ')}`);
  }
  if (cleanupErrors.length) throw new Error(`build cleanup failed: ${cleanupErrors.join('; ')}`);
  return BUILD_OUTPUTS;
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  runBuild().catch((error) => { console.error(error.message); process.exitCode = 1; });
}
