import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
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
      throw new Error(`unexpected argument: ${argument}`);
    }
  }
  if (!options.root) throw new Error('usage: validate-browser-artifact.mjs <artifact-dir> [--phase final|precheck] [--expected-commit <sha>]');
  if (options.phase !== 'final' && options.phase !== 'precheck') throw new Error('--phase must be final or precheck');
  if (options.phase === 'precheck') {
    if (!/^[0-9a-f]{40}$/.test(options.expectedCommit || '') || /^0{40}$/.test(options.expectedCommit || '')) {
      throw new Error('precheck phase requires --expected-commit with the candidate 40-hex SHA');
    }
  } else if (options.expectedCommit !== null) {
    throw new Error('--expected-commit is only valid in precheck phase');
  }
  return options;
}

const options = parseArguments(process.argv.slice(2));
const root = path.resolve(options.root);
const commit = options.phase === 'precheck' ? options.expectedCommit : (process.env.GIT_COMMIT || process.env.GITHUB_SHA);
const ref = process.env.GITHUB_REF;
const runId = process.env.GITHUB_RUN_ID;
const attemptRaw = process.env.GITHUB_RUN_ATTEMPT || '';
if (!/^[1-9][0-9]*$/.test(attemptRaw)) throw new Error('GITHUB_RUN_ATTEMPT must be a positive integer');
const attempt = Number(attemptRaw);
if (!Number.isSafeInteger(attempt) || attempt < 1) throw new Error('GITHUB_RUN_ATTEMPT must be a positive integer');
const names = (await fs.readdir(root)).sort();
if (names.length !== 2 || names[0] !== 'provenance.json' || names[1] !== 'release-matrix.json') throw new Error('browser artifact allowlist mismatch');
const matrixBytes = await fs.readFile(path.join(root, 'release-matrix.json'));
const matrix = validateBrowserMatrix(JSON.parse(matrixBytes), commit);
const provenance = JSON.parse(await fs.readFile(path.join(root, 'provenance.json'), 'utf8'));
crossValidateBrowserEvidence(matrix, provenance);
if (!/^[0-9a-f]{64}$/.test(provenance.matrix_sha256 || '')) throw new Error('browser artifact provenance schema mismatch');
if (provenance.matrix_sha256 !== crypto.createHash('sha256').update(matrixBytes).digest('hex')) throw new Error('browser artifact matrix digest mismatch');
if (provenance.commit !== commit || provenance.ref !== ref) throw new Error('browser artifact provenance mismatch');
if (!provenance.release_run || Object.keys(provenance.release_run).sort().join(',') !== 'attempt,id'
  || !/^[1-9][0-9]*$/.test(provenance.release_run.id || '')
  || !Number.isSafeInteger(provenance.release_run.attempt) || provenance.release_run.attempt < 1) {
  throw new Error('browser artifact release run provenance is invalid');
}
if (options.phase === 'final') {
  // The final release run consumes an artifact produced by itself; a reused or
  // cross-attempt artifact must block publishing.
  if (provenance.release_run.id !== runId || provenance.release_run.attempt !== attempt) {
    throw new Error('browser artifact provenance mismatch');
  }
}
// Precheck phase intentionally does not compare release_run against this
// process: the validator runs in E's context, not the producer run. The tag
// binding (commit equality with the candidate SHA) is checked above via
// validateBrowserMatrix + provenance.commit.
process.stdout.write(`validated ${matrix.records.length} browser records (${options.phase} phase)\n`);
