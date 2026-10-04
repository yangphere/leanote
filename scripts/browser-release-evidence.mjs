import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import os from 'node:os';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import { jcsSha256 } from './jcs.mjs';

const execAsync = promisify(exec);

const products = ['chrome', 'edge', 'firefox', 'safari'];
const slots = ['current_major', 'previous_major'];
const coverageIds = ['business-flows', 'editor-flows', 'bootstrap-components', 'leaui-image-iframe'];
const identifierPattern = /^[a-z0-9][a-z0-9._/-]{0,79}$/;
const recordKeys = new Set([
  'commit', 'browser_product', 'release_slot', 'browser_version', 'os', 'environment',
  'coverage', 'coverage_summary_sha256', 'auth_gate', 'error_gate', 'resource_gate', 'executed_at', 'result',
]);
const summarySlotKeys = new Set(['browser_product', 'release_slot', 'coverage_summary_sha256', 'items']);
const summaryItemKeys = new Set(['id', 'discovered_count', 'executed_count', 'entrypoints', 'iframes', 'result']);

function assertKeys(value, keys, label) {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new Error(`${label} contains unknown or missing fields`);
  }
}

function assertIdentifiers(items, label, { allowEmpty }) {
  if (!Array.isArray(items) || items.length > 40 || (!allowEmpty && items.length < 1)
    || items.some((item) => typeof item !== 'string' || !identifierPattern.test(item))) {
    throw new Error(`${label} identifiers are invalid`);
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
    if (!Array.isArray(row.coverage) || row.coverage.length !== coverageIds.length || row.coverage.some((id, index) => id !== coverageIds[index])) {
      throw new Error('browser coverage must be the four stable coverage ids in fixed order');
    }
    if (!/^[0-9a-f]{64}$/.test(row.coverage_summary_sha256 || '')) throw new Error('browser coverage summary digest is invalid');
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

// Validates provenance.coverage_summaries and its binding to every matrix row:
// eight unique slots, four items per slot in fixed order, per-item constraints,
// and a recomputed RFC 8785 digest that must equal both the summary entry and
// the corresponding matrix record.
export function crossValidateBrowserEvidence(matrix, provenance) {
  if (!provenance || typeof provenance !== 'object') throw new Error('browser provenance must be an object');
  assertKeys(provenance, new Set(['schema_version', 'matrix_sha256', 'commit', 'ref', 'producer_workflow', 'release_run', 'coverage_summaries']), 'provenance');
  if (provenance.schema_version !== 'leanote.browser-smoke.release-matrix-provenance.v1') throw new Error('browser provenance schema version mismatch');
  if (provenance.commit !== matrix.commit) throw new Error('browser provenance commit does not match matrix');
  if (provenance.producer_workflow !== 'Protected browser release evidence') throw new Error('browser provenance producer workflow mismatch');
  const summaries = provenance.coverage_summaries;
  if (!Array.isArray(summaries) || summaries.length !== products.length * slots.length) throw new Error('coverage summaries must cover exactly eight slots');
  const seen = new Set();
  for (const summary of summaries) {
    if (!summary || typeof summary !== 'object') throw new Error('invalid coverage summary');
    assertKeys(summary, summarySlotKeys, 'coverage summary');
    if (!products.includes(summary.browser_product) || !slots.includes(summary.release_slot)) throw new Error('coverage summary slot identity mismatch');
    const key = `${summary.browser_product}/${summary.release_slot}`;
    if (seen.has(key)) throw new Error(`duplicate coverage summary: ${key}`);
    seen.add(key);
    if (!/^[0-9a-f]{64}$/.test(summary.coverage_summary_sha256 || '')) throw new Error('coverage summary digest is invalid');
    if (!Array.isArray(summary.items) || summary.items.length !== coverageIds.length || summary.items.some((item, index) => item?.id !== coverageIds[index])) {
      throw new Error('coverage summary items must be the four stable ids in fixed order');
    }
    for (const item of summary.items) {
      assertKeys(item, summaryItemKeys, 'coverage summary item');
      if (!Number.isSafeInteger(item.discovered_count) || item.discovered_count < 1
        || !Number.isSafeInteger(item.executed_count) || item.executed_count < 1 || item.executed_count > item.discovered_count) {
        throw new Error(`coverage counts are invalid for ${item.id}`);
      }
      assertIdentifiers(item.entrypoints, `entrypoints for ${item.id}`, { allowEmpty: false });
      assertIdentifiers(item.iframes, `iframes for ${item.id}`, { allowEmpty: true });
      if (item.result !== 'passed') throw new Error(`coverage result must be passed for ${item.id}`);
    }
    const digest = jcsSha256({
      browser_product: summary.browser_product,
      release_slot: summary.release_slot,
      items: summary.items,
    });
    if (digest !== summary.coverage_summary_sha256) throw new Error(`coverage summary digest mismatch: ${key}`);
    const row = matrix.records.find((record) => record.browser_product === summary.browser_product && record.release_slot === summary.release_slot);
    if (row.coverage_summary_sha256 !== digest) throw new Error(`matrix row digest mismatch: ${key}`);
  }
  return provenance;
}

function parseCoverageMarkers(marker, browser_product, release_slot) {
  const items = coverageIds.map((id) => {
    const key = id.replaceAll('-', '_');
    const counts = /^discovered=([0-9]+);executed=([0-9]+)$/.exec(marker(`LEANOTE_COVERAGE_${key}`));
    const entrypoints = marker(`LEANOTE_ENTRYPOINTS_${key}`);
    const iframes = marker(`LEANOTE_IFRAMES_${key}`);
    if (!counts) throw new Error(`protected ${browser_product}/${release_slot} smoke did not emit coverage counts for ${id}`);
    const discovered_count = Number(counts[1]);
    const executed_count = Number(counts[2]);
    if (!Number.isSafeInteger(discovered_count) || discovered_count < 1 || !Number.isSafeInteger(executed_count) || executed_count < 1 || executed_count > discovered_count) {
      throw new Error(`protected ${browser_product}/${release_slot} smoke emitted invalid counts for ${id}`);
    }
    if (entrypoints === '') throw new Error(`protected ${browser_product}/${release_slot} smoke must emit at least one entrypoint for ${id}`);
    const entrypointList = entrypoints.split(',');
    const iframeList = iframes === '' ? [] : iframes.split(',');
    assertIdentifiers(entrypointList, `entrypoints for ${id}`, { allowEmpty: false });
    assertIdentifiers(iframeList, `iframes for ${id}`, { allowEmpty: true });
    return { id, discovered_count, executed_count, entrypoints: entrypointList, iframes: iframeList, result: 'passed' };
  });
  const summaryInput = { browser_product, release_slot, items };
  return { items, coverage_summary_sha256: jcsSha256(summaryInput) };
}

async function runProtectedBrowserCommands(commit, env) {
  const records = [];
  const coverageSummaries = [];
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
      const { items, coverage_summary_sha256 } = parseCoverageMarkers(marker, browser_product, release_slot);
      records.push({
        commit, browser_product, release_slot, browser_version: version,
        os: marker('LEANOTE_BROWSER_OS') || `${process.platform}-${os.release()}`,
        environment: 'real-browser', coverage: [...coverageIds], coverage_summary_sha256,
        auth_gate: 'passed', error_gate: 'passed', resource_gate: 'passed',
        executed_at: new Date().toISOString(), result: 'passed',
      });
      coverageSummaries.push({ browser_product, release_slot, coverage_summary_sha256, items });
    }
  }
  return {
    matrix: { schema_version: 'leanote.browser-smoke.release-matrix.v1', commit, records },
    coverage_summaries: coverageSummaries,
  };
}

export async function buildBrowserEvidence({ source, summaries, output = path.resolve('test-results'), env = process.env } = {}) {
  const commit = env.RELEASE_COMMIT;
  if (!/^[0-9a-f]{40}$/.test(commit || '') || /^0{40}$/.test(commit)) throw new Error('RELEASE_COMMIT must be a commit SHA');
  if (!source && summaries) throw new Error('summaries input requires the matrix source; refusing to run protected commands from a partial rebuild request');
  let matrix;
  let coverageSummaries;
  if (source) {
    // Rebuild mode consumes a validated matrix plus its coverage summaries so
    // the emitted provenance stays contract-complete (coverage_summaries is
    // mandatory; emitting an artifact without it would be self-violating).
    if (!summaries) throw new Error('rebuild mode requires the coverage summaries file alongside the matrix source');
    matrix = validateBrowserMatrix(JSON.parse(await fs.readFile(source, 'utf8')), commit);
    coverageSummaries = JSON.parse(await fs.readFile(summaries, 'utf8'));
  } else {
    ({ matrix, coverage_summaries: coverageSummaries } = await runProtectedBrowserCommands(commit, env));
    validateBrowserMatrix(matrix, commit);
  }
  const ref = env.RELEASE_REF || env.GITHUB_REF;
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
    coverage_summaries: coverageSummaries,
  };
  crossValidateBrowserEvidence(matrix, provenance);
  await fs.writeFile(path.join(output, 'provenance.json'), `${JSON.stringify(provenance, null, 2)}\n`);
  return { matrix, provenance };
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  buildBrowserEvidence().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
