import fs from 'node:fs/promises';
import path from 'node:path';

const expected = ['go-1_26_7', 'go-1_27_0', 'mongo-8_0', 'node-build', 'chromium-e2e', 'package-smoke', 'container-smoke'];
const statuses = new Set(['passed', 'failed', 'cancelled', 'not_run']);
const readinessStates = new Set(['passed', 'failed', 'not_run', 'unknown']);
const failureCategories = new Set([
  'none', 'job_not_started', 'checkout', 'setup', 'dependency', 'compile', 'lint', 'test',
  'discovery_zero', 'service_readiness', 'drift', 'package', 'container', 'pdf', 'version',
  'cleanup', 'schema', 'permission', 'timeout', 'unknown',
]);

const requiredKeys = (value, keys, label) => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object`);
  const actual = Object.keys(value).sort();
  const expectedKeys = [...keys].sort();
  if (actual.length !== expectedKeys.length || actual.some((key, index) => key !== expectedKeys[index])) {
    throw new Error(`${label} schema mismatch`);
  }
};
const isIntegerOrNull = (value, minimum = 0, maximum = Number.MAX_SAFE_INTEGER) =>
  value === null || (Number.isSafeInteger(value) && value >= minimum && value <= maximum);
const isDate = (value) => typeof value === 'string' && /Z$/.test(value) && !Number.isNaN(Date.parse(value));

function validateRecord(record, file) {
  requiredKeys(record, [
    'schema_version', 'workflow', 'job', 'run', 'commit', 'ref', 'status', 'stage', 'toolchain',
    'failure', 'service', 'tests', 'page_paths', 'resource_paths', 'status_codes', 'generated_at',
  ], file);
  if (record.schema_version !== 'leanote.ci.failure-summary.v1') throw new Error(`${file} schema version mismatch`);
  if (typeof record.workflow !== 'string' || record.workflow.length < 1 || record.workflow.length > 120) throw new Error(`${file} workflow invalid`);
  if (!expected.includes(record.job) || record.job !== path.basename(file, '.json')) throw new Error(`${file} job identity mismatch`);
  requiredKeys(record.run, ['id', 'attempt'], `${file}.run`);
  if (!/^[0-9]+$/.test(record.run.id) || record.run.id === '0' || !Number.isSafeInteger(record.run.attempt) || record.run.attempt < 1) throw new Error(`${file} run provenance invalid`);
  if (!/^[0-9a-f]{40}$/.test(record.commit) || /^0{40}$/.test(record.commit)) throw new Error(`${file} commit invalid`);
  if (typeof record.ref !== 'string' || record.ref.length < 1 || record.ref.length > 255 || record.ref === 'unknown') throw new Error(`${file} ref invalid`);
  if (record.workflow === 'unknown') throw new Error(`${file} workflow invalid`);
  if (!statuses.has(record.status)) throw new Error(`${file} status invalid`);
  if (typeof record.stage !== 'string' || record.stage.length < 1 || record.stage.length > 80) throw new Error(`${file} stage invalid`);

  requiredKeys(record.toolchain, ['go', 'node', 'npm', 'mongo', 'playwright'], `${file}.toolchain`);
  for (const [name, value] of Object.entries(record.toolchain)) {
    if (value !== null && (typeof value !== 'string' || value.length > (name === 'mongo' || name === 'playwright' ? 80 : 40))) {
      throw new Error(`${file}.toolchain.${name} invalid`);
    }
  }
  requiredKeys(record.failure, ['category', 'message', 'exit_code'], `${file}.failure`);
  if (!failureCategories.has(record.failure.category) || typeof record.failure.message !== 'string' || record.failure.message.length > 500 || !isIntegerOrNull(record.failure.exit_code)) {
    throw new Error(`${file}.failure invalid`);
  }
  requiredKeys(record.service, ['health_path', 'readiness', 'http_status', 'exit_code'], `${file}.service`);
  if (record.service.health_path !== null && record.service.health_path !== '/healthz') throw new Error(`${file} health path must be /healthz`);
  if (!readinessStates.has(record.service.readiness) || !isIntegerOrNull(record.service.http_status, 100, 599) || !isIntegerOrNull(record.service.exit_code)) throw new Error(`${file}.service invalid`);
  if (record.service.health_path === null && record.service.readiness !== 'not_run') throw new Error(`${file} service readiness without health path`);
  if (record.service.health_path !== null && record.service.readiness === 'passed' && record.service.http_status !== 200) throw new Error(`${file} readiness pass requires HTTP 200`);
  if (record.status === 'passed' && record.service.health_path !== null && record.service.readiness !== 'passed') throw new Error(`${file} claims an unverified readiness pass`);

  requiredKeys(record.tests, ['discovery', 'discovered_count', 'executed_count'], `${file}.tests`);
  if (!new Set(['passed', 'failed', 'not_run']).has(record.tests.discovery) || !isIntegerOrNull(record.tests.discovered_count) || !isIntegerOrNull(record.tests.executed_count)) throw new Error(`${file}.tests invalid`);
  if (record.status === 'passed' && (record.tests.discovery !== 'passed' || record.tests.discovered_count < 1 || record.tests.executed_count < 1)) throw new Error(`${file} claims an unverified test pass`);
  for (const [name, pattern] of [['page_paths', /^\/[A-Za-z0-9._~/?=&%-]{0,240}$/], ['resource_paths', /^\/[A-Za-z0-9._~/?=&%-]{0,240}$/]]) {
    if (!Array.isArray(record[name]) || record[name].length > 20 || record[name].some((value) => typeof value !== 'string' || !pattern.test(value))) throw new Error(`${file}.${name} invalid`);
  }
  if (!Array.isArray(record.status_codes) || record.status_codes.length > 20 || record.status_codes.some((value) => !Number.isSafeInteger(value) || value < 100 || value > 599)) throw new Error(`${file}.status_codes invalid`);
  if (!isDate(record.generated_at)) throw new Error(`${file}.generated_at invalid`);
  if (record.status === 'passed' && record.failure.category !== 'none') throw new Error(`${file} passed with a failure category`);
  if (record.status !== 'passed' && record.failure.category === 'none') throw new Error(`${file} non-pass has no failure category`);
}

const dir = process.argv[2] || 'ci-summaries';
const files = (await fs.readdir(dir)).filter((name) => name.endsWith('.json') && name !== 'summary.json').sort();
if (files.length !== expected.length) throw new Error('quality-gate summary count mismatch');
const records = [];
for (const file of files) {
  const record = JSON.parse(await fs.readFile(path.join(dir, file), 'utf8'));
  validateRecord(record, file);
  records.push(record);
}
const baseline = records[0];
for (const record of records) {
  if (record.commit !== baseline.commit || record.ref !== baseline.ref || record.workflow !== baseline.workflow || record.run.id !== baseline.run.id || record.run.attempt !== baseline.run.attempt) {
    throw new Error('quality-gate summary provenance is inconsistent');
  }
  if (record.status !== 'passed') throw new Error(`quality-gate job ${record.job} failed`);
}
process.stdout.write(`validated ${records.length} quality-gate summaries\n`);
