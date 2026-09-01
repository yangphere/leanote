import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import os from 'node:os';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';

const execAsync = promisify(exec);

const products = ['chrome', 'edge', 'firefox', 'safari'];
const slots = ['current_major', 'previous_major'];
const recordKeys = new Set([
  'commit', 'browser_product', 'release_slot', 'browser_version', 'os', 'environment',
  'coverage', 'auth_gate', 'error_gate', 'resource_gate', 'executed_at', 'result',
]);

function assertKeys(value, keys, label) {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new Error(`${label} contains unknown or missing fields`);
  }
}

function validateDate(value) {
  if (typeof value !== 'string') return false;
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?Z$/.exec(value);
  if (!match) return false;
  const [year, month, day, hour, minute, second] = match.slice(1, 7).map(Number);
  if (hour > 23 || minute > 59 || second > 59) return false;
  const date = new Date(0);
  date.setUTCFullYear(year, month - 1, day);
  date.setUTCHours(hour, minute, second, 0);
  return date.getUTCFullYear() === year
    && date.getUTCMonth() === month - 1
    && date.getUTCDate() === day
    && date.getUTCHours() === hour
    && date.getUTCMinutes() === minute
    && date.getUTCSeconds() === second;
}

function parseAttempt(value) {
  if (!/^[0-9]+$/.test(value || '')) throw new Error('release run attempt must be a positive integer');
  const attempt = Number(value);
  if (!Number.isSafeInteger(attempt) || attempt < 1) throw new Error('release run attempt must be a positive integer');
  return attempt;
}

export function validateBrowserMatrix(matrix, commit) {
  if (!matrix || typeof matrix !== 'object' || Array.isArray(matrix)) throw new Error('release matrix must be an object');
  assertKeys(matrix, new Set(['schema_version', 'commit', 'records']), 'matrix');
  if (!/^[0-9a-f]{40}$/.test(commit || '') || /^0{40}$/.test(commit) || matrix.schema_version !== 'leanote.browser-smoke.release-matrix.v1' || matrix.commit !== commit) {
    throw new Error('release matrix schema or commit mismatch');
  }
  if (!Array.isArray(matrix.records) || matrix.records.length !== 8) throw new Error('release matrix must contain exactly eight records');
  const seen = new Set();
  for (const row of matrix.records) {
    if (!row || typeof row !== 'object' || Array.isArray(row)) throw new Error('invalid browser record');
    assertKeys(row, recordKeys, 'browser record');
    if (row.commit !== commit || !products.includes(row.browser_product) || !slots.includes(row.release_slot)) {
      throw new Error('browser record identity mismatch');
    }
    const key = `${row.browser_product}/${row.release_slot}`;
    if (seen.has(key)) throw new Error(`duplicate browser record: ${key}`);
    seen.add(key);
    if (typeof row.browser_version !== 'string' || !/^[0-9]+(?:\.[0-9]+){1,3}$/.test(row.browser_version)) throw new Error('browser version is invalid');
    if (typeof row.os !== 'string' || row.os.length < 1 || row.os.length > 120 || row.environment !== 'real-browser') throw new Error('browser environment is invalid');
    if (!Array.isArray(row.coverage) || row.coverage.length < 1 || row.coverage.length > 40 || row.coverage.some((scope) => typeof scope !== 'string' || !/^[a-z0-9][a-z0-9._-]{0,79}$/.test(scope))) throw new Error('browser coverage is invalid');
    if (!['passed'].includes(row.auth_gate) || row.error_gate !== 'passed' || row.resource_gate !== 'passed' || row.result !== 'passed') throw new Error('browser gate failed');
    if (!validateDate(row.executed_at)) throw new Error('browser executed_at must be RFC3339 UTC');
  }
  for (const product of products) {
    for (const slot of slots) if (!seen.has(`${product}/${slot}`)) throw new Error('browser matrix coverage is incomplete');
    const current = matrix.records.find((row) => row.browser_product === product && row.release_slot === 'current_major');
    const previous = matrix.records.find((row) => row.browser_product === product && row.release_slot === 'previous_major');
    const currentMajor = Number.parseInt(current.browser_version.split('.')[0], 10);
    const previousMajor = Number.parseInt(previous.browser_version.split('.')[0], 10);
    if (!Number.isSafeInteger(currentMajor) || !Number.isSafeInteger(previousMajor) || previousMajor !== currentMajor - 1) {
      throw new Error(`browser major slots are not adjacent: ${product}`);
    }
  }
  return matrix;
}

async function runProtectedBrowserCommands(commit, env) {
	const records = [];
	for (const browser_product of products) {
		for (const release_slot of slots) {
			const suffix = `${browser_product}_${release_slot}`.toUpperCase();
			const command = env[`BROWSER_SMOKE_COMMAND_${suffix}`];
			if (!command) throw new Error(`missing protected smoke command: BROWSER_SMOKE_COMMAND_${suffix}`);
			let stdout;
			try {
				({ stdout } = await execAsync(command, { shell: true, maxBuffer: 1024 * 1024 }));
			} catch (error) {
				throw new Error(`protected ${browser_product}/${release_slot} smoke failed`);
			}
			const marker = (name) => {
				const match = stdout.match(new RegExp(`^${name}=([^\\r\\n]+)$`, 'm'));
				return match ? match[1].trim() : '';
			};
			const version = marker('LEANOTE_BROWSER_VERSION');
			if (!version || marker('LEANOTE_AUTH_GATE') !== 'passed' || marker('LEANOTE_ERROR_GATE') !== 'passed' || marker('LEANOTE_RESOURCE_GATE') !== 'passed') {
				throw new Error(`protected ${browser_product}/${release_slot} smoke did not emit required passed markers`);
			}
			records.push({
				commit, browser_product, release_slot, browser_version: version,
				os: marker('LEANOTE_BROWSER_OS') || `${process.platform}-${os.release()}`,
				environment: 'real-browser', coverage: ['build-smoke', 'auth-gate', 'error-gate', 'resource-gate'],
				auth_gate: 'passed', error_gate: 'passed', resource_gate: 'passed',
				executed_at: new Date().toISOString(), result: 'passed',
			});
		}
	}
	return { schema_version: 'leanote.browser-smoke.release-matrix.v1', commit, records };
}

export async function buildBrowserEvidence({ source, output = path.resolve('test-results'), env = process.env } = {}) {
	const commit = env.RELEASE_COMMIT;
  if (!/^[0-9a-f]{40}$/.test(commit || '') || /^0{40}$/.test(commit)) throw new Error('RELEASE_COMMIT must be a commit SHA');
	const matrix = source
		? validateBrowserMatrix(JSON.parse(await fs.readFile(source, 'utf8')), commit)
		: validateBrowserMatrix(await runProtectedBrowserCommands(commit, env), commit);
  const ref = env.RELEASE_REF || env.GITHUB_REF || `refs/tags/${env.RELEASE_TAG || ''}`;
  if (!/^refs\/tags\/v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$/.test(ref)) throw new Error('GITHUB_REF must be a strict release tag ref');
  const runId = env.GITHUB_RUN_ID;
  const attempt = parseAttempt(env.GITHUB_RUN_ATTEMPT);
  if (!/^[0-9]+$/.test(runId || '')) throw new Error('release run provenance is invalid');
  await fs.mkdir(output, { recursive: true });
  const matrixPath = path.join(output, 'release-matrix.json');
  const payload = `${JSON.stringify(matrix, null, 2)}\n`;
  await fs.writeFile(matrixPath, payload);
  const provenance = {
    schema_version: 'leanote.browser-smoke.release-matrix-provenance.v1',
    matrix_sha256: crypto.createHash('sha256').update(payload).digest('hex'),
    commit, ref, producer_workflow: 'Protected browser release evidence',
    release_run: { id: runId, attempt },
  };
  await fs.writeFile(path.join(output, 'provenance.json'), `${JSON.stringify(provenance, null, 2)}\n`);
  return { matrix, provenance };
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
	buildBrowserEvidence().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
