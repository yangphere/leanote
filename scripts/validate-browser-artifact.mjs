import { ReleaseError } from './release-error.mjs';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { validateBrowserMatrix, crossValidateBrowserEvidence } from './browser-release-evidence.mjs';

function parseArguments(argv) {
  const options = { root: null, phase: 'final', expectedCommit: null };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === '--phase') {
      options.phase = argv[++index];
    } else if (argument === '--expected-commit') {
      options.expectedCommit = argv[++index];
    } else if (!options.root) {
      options.root = argument;
    } else {
      throw new ReleaseError(`unexpected argument: ${argument}`);
    }
  }
  if (!options.root) throw new ReleaseError('usage: validate-browser-artifact.mjs <artifact-dir> [--phase final|precheck] [--expected-commit <sha>]');
  if (options.phase !== 'final' && options.phase !== 'precheck') throw new ReleaseError('--phase must be final or precheck');
  if (options.phase === 'precheck') {
    if (!/^[0-9a-f]{40}$/.test(options.expectedCommit || '') || /^0{40}$/.test(options.expectedCommit || '')) {
      throw new ReleaseError('precheck phase requires --expected-commit with the candidate 40-hex SHA');
    }
  } else if (options.expectedCommit !== null) {
    throw new ReleaseError('--expected-commit is only valid in precheck phase');
  }
  return options;
}

export async function validateBrowserArtifact(options, expected = null) {
  const context = expected || { commit: process.env.GIT_COMMIT || process.env.GITHUB_SHA, ref: process.env.GITHUB_REF, run: { id: process.env.GITHUB_RUN_ID, attempt: process.env.GITHUB_RUN_ATTEMPT } };
  const root = path.resolve(options.root);
  const commit = options.phase === 'precheck' ? options.expectedCommit : context.commit;
  const ref = context.ref;
  const runId = context.run.id;
  const attemptRaw = String(context.run.attempt || '');
  if (!/^[1-9][0-9]*$/.test(attemptRaw)) throw new ReleaseError('GITHUB_RUN_ATTEMPT must be a positive integer');
  const attempt = Number(attemptRaw);
  if (!Number.isSafeInteger(attempt) || attempt < 1) throw new ReleaseError('GITHUB_RUN_ATTEMPT must be a positive integer');
const names = (await fs.readdir(root)).sort();
  if (names.length !== 2 || names[0] !== 'provenance.json' || names[1] !== 'release-matrix.json') throw new ReleaseError('browser artifact allowlist mismatch');
  for (const name of names) {
    const stat = await fs.lstat(path.join(root, name));
    if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 4 * 1024 * 1024) throw new ReleaseError('browser evidence must be a bounded regular file');
  }
  const matrixBytes = await fs.readFile(path.join(root, 'release-matrix.json'));
  const matrix = validateBrowserMatrix(JSON.parse(matrixBytes), commit);
  const provenance = JSON.parse(await fs.readFile(path.join(root, 'provenance.json'), 'utf8'));
  crossValidateBrowserEvidence(matrix, provenance);
  if (!/^[0-9a-f]{64}$/.test(provenance.matrix_sha256 || '')) throw new ReleaseError('browser artifact provenance schema mismatch');
  if (provenance.matrix_sha256 !== crypto.createHash('sha256').update(matrixBytes).digest('hex')) throw new ReleaseError('browser artifact matrix digest mismatch');
  if (provenance.commit !== commit || provenance.ref !== ref) throw new ReleaseError('browser artifact provenance mismatch');
  if (!provenance.release_run || Object.keys(provenance.release_run).sort().join(',') !== 'attempt,id'
    || !/^[1-9][0-9]*$/.test(provenance.release_run.id || '')
    || !Number.isSafeInteger(provenance.release_run.attempt) || provenance.release_run.attempt < 1) {
    throw new ReleaseError('browser artifact release run provenance is invalid');
  }
  if (options.phase === 'final') {
    // The final release run consumes an artifact produced by itself; a reused or
    // cross-attempt artifact must block publishing.
    if (provenance.release_run.id !== runId || provenance.release_run.attempt !== attempt) {
      throw new ReleaseError('browser artifact provenance mismatch');
    }
  }
  // Precheck phase intentionally does not compare release_run against this
  // process: the validator runs in E's context, not the producer run. The tag
  // binding (commit equality with the candidate SHA) is checked above via
  // validateBrowserMatrix + provenance.commit.
  return { matrix, provenance };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const options = parseArguments(process.argv.slice(2));
  const { matrix } = await validateBrowserArtifact(options);
  process.stdout.write(`validated ${matrix.records.length} browser records (${options.phase} phase)` + String.fromCharCode(10));
}
