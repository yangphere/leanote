import fs from 'node:fs/promises';
import path from 'node:path';

const allowedJobs = new Set([
  'go-1_26_7', 'go-1_27_0', 'mongo-8_0', 'node-build',
  'chromium-e2e', 'package-smoke', 'container-smoke', 'summary',
]);
const failureCategories = new Set([
  'none', 'job_not_started', 'checkout', 'setup', 'dependency', 'compile', 'lint',
  'test', 'discovery_zero', 'service_readiness', 'drift', 'package', 'container',
  'pdf', 'version', 'cleanup', 'schema', 'permission', 'timeout', 'unknown',
]);

function count(name) {
  const raw = process.env[name];
  if (raw == null || raw === '') return null;
  if (!/^[0-9]+$/.test(raw)) throw new Error(`${name} must be a non-negative integer`);
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < 0) throw new Error(`${name} must be a non-negative integer`);
  return value;
}

function optionalStatus(name, fallback, allowed) {
  const value = process.env[name] || fallback;
  if (!allowed.includes(value)) throw new Error(`${name} has an invalid value`);
  return value;
}

function runAttempt() {
  const raw = process.env.GITHUB_RUN_ATTEMPT;
  if (!/^[0-9]+$/.test(raw)) throw new Error('GITHUB_RUN_ATTEMPT must be a positive integer');
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < 1) throw new Error('GITHUB_RUN_ATTEMPT must be a positive integer');
  return value;
}

function requiredProvenance(name, pattern, description) {
  const value = process.env[name];
  if (!value || !pattern.test(value)) throw new Error(`${name} must be ${description}`);
  return value;
}

function safeMessage(value, fallback) {
  if (!value) return fallback;
  const message = value.trim().replace(/[\r\n\t]+/g, ' ').slice(0, 500);
  if (!/^[A-Za-z0-9_.:/ -]*$/.test(message) || /mongodb|secret|token|cookie|password|authorization|@/i.test(message)) return fallback;
  return message || fallback;
}

const job = process.env.CI_JOB_ID;
if (!allowedJobs.has(job)) throw new Error('CI_JOB_ID is not an allowed quality-gate job');
const workflow = requiredProvenance('GITHUB_WORKFLOW', /^.{1,120}$/, 'a non-empty workflow name');
const runId = requiredProvenance('GITHUB_RUN_ID', /^[1-9][0-9]*$/, 'a positive numeric run id');
const commit = requiredProvenance('GITHUB_SHA', /^[0-9a-f]{40}$/, 'a 40-character commit SHA');
const ref = requiredProvenance('GITHUB_REF', /^.{1,255}$/, 'a non-empty ref');
const jobStatus = process.env.CI_JOB_STATUS || 'failure';
let status = jobStatus === 'success' ? 'passed' : (jobStatus === 'cancelled' ? 'cancelled' : 'failed');
let category = process.env.CI_FAILURE_CATEGORY || (status === 'passed' ? 'none' : 'job_not_started');
if (!failureCategories.has(category)) throw new Error('CI_FAILURE_CATEGORY is invalid');
let discovery = optionalStatus('CI_DISCOVERY', status === 'passed' ? 'passed' : 'not_run', ['passed', 'failed', 'not_run']);
let discoveredCount = count('CI_DISCOVERED_COUNT');
let executedCount = count('CI_EXECUTED_COUNT');
const forcedFallback = process.env.CI_FORCE_FALLBACK === 'true';
if (forcedFallback) {
  status = 'failed';
  category = 'job_not_started';
  discovery = 'not_run';
  discoveredCount = null;
  executedCount = null;
}
if (status === 'passed' && (discovery !== 'passed' || discoveredCount === null || discoveredCount < 1 || executedCount === null || executedCount < 1)) {
  status = 'failed';
  category = 'discovery_zero';
  discovery = discovery === 'passed' ? 'failed' : discovery;
}
if (status !== 'passed' && category === 'none') category = 'unknown';
let healthPath = process.env.CI_HEALTH_PATH || null;
if (healthPath !== null && healthPath !== '/healthz') throw new Error('CI_HEALTH_PATH must be /healthz or empty');
const httpStatusRaw = process.env.CI_HTTP_STATUS || '';
if (httpStatusRaw && !/^[0-9]+$/.test(httpStatusRaw)) throw new Error('CI_HTTP_STATUS is invalid');
let httpStatus = httpStatusRaw ? Number(httpStatusRaw) : null;
if (httpStatus !== null && (!Number.isInteger(httpStatus) || httpStatus < 100 || httpStatus > 599)) throw new Error('CI_HTTP_STATUS is invalid');
let readiness = optionalStatus('CI_SERVICE_READINESS', 'not_run', ['passed', 'failed', 'not_run', 'unknown']);
if (healthPath === null && readiness !== 'not_run') throw new Error('CI_SERVICE_READINESS requires CI_HEALTH_PATH');
if (status === 'passed' && healthPath !== null && (readiness !== 'passed' || httpStatus !== 200)) {
  status = 'failed';
  category = 'service_readiness';
}
const exitCodeRaw = process.env.CI_EXIT_CODE || '';
if (exitCodeRaw && !/^[0-9]+$/.test(exitCodeRaw)) throw new Error('CI_EXIT_CODE is invalid');
let exitCode = exitCodeRaw ? Number(exitCodeRaw) : (status === 'passed' ? 0 : null);
if (exitCode !== null && (!Number.isInteger(exitCode) || exitCode < 0)) throw new Error('CI_EXIT_CODE is invalid');
if (forcedFallback) {
  healthPath = null;
  readiness = 'not_run';
  httpStatus = null;
  exitCode = null;
}
const generated = new Date().toISOString();
const summary = {
  schema_version: 'leanote.ci.failure-summary.v1', workflow, job,
  run: { id: runId, attempt: runAttempt() },
  commit, ref, status,
  stage: forcedFallback ? 'job_not_started' : (process.env.CI_STAGE || (status === 'passed' ? 'complete' : 'job_not_started')),
  toolchain: {
    go: process.env.GO_VERSION || null, node: process.env.NODE_VERSION || null,
    npm: process.env.NPM_VERSION || null, mongo: process.env.MONGO_VERSION || null,
    playwright: process.env.PLAYWRIGHT_VERSION || null,
  },
  failure: { category, message: safeMessage(process.env.CI_FAILURE_MESSAGE, status === 'passed' ? '' : category), exit_code: exitCode },
  service: { health_path: healthPath, readiness, http_status: httpStatus, exit_code: exitCode },
  tests: { discovery, discovered_count: discoveredCount, executed_count: executedCount },
  page_paths: [], resource_paths: [], status_codes: httpStatus === null ? [] : [httpStatus], generated_at: generated,
};
await fs.mkdir('ci-summaries', { recursive: true });
await fs.writeFile(path.join('ci-summaries', `${job}.json`), `${JSON.stringify(summary, null, 2)}\n`);
