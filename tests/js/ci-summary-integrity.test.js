const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const test = require('node:test');

const root = process.cwd();
const jobs = ['go-1_26_7', 'go-1_27_0', 'mongo-8_0', 'node-build', 'chromium-e2e', 'package-smoke', 'container-smoke'];
const identity = { GITHUB_SHA: 'a'.repeat(40), GITHUB_REF: 'refs/heads/dev', GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '123', GITHUB_RUN_ATTEMPT: '2' };
function record(job) {
  return {
    schema_version: 'leanote.ci.failure-summary.v1', workflow: identity.GITHUB_WORKFLOW, job,
    run: { id: '123', attempt: 2 }, commit: identity.GITHUB_SHA, ref: identity.GITHUB_REF,
    status: 'passed', stage: 'complete', toolchain: { go: null, node: null, npm: null, mongo: null, playwright: null },
    failure: { category: 'none', message: '', exit_code: 0 },
    service: { health_path: null, readiness: 'not_run', http_status: null, exit_code: 0 },
    tests: { discovery: 'passed', discovered_count: 2, executed_count: 2 },
    page_paths: [], resource_paths: [], status_codes: [], generated_at: '2026-09-29T00:00:00Z',
  };
}
async function summaries(change, env = {}) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-ci-integrity-'));
  try {
    for (const job of jobs) {
      const value = record(job);
      change(value);
      await fs.writeFile(path.join(dir, job + '.json'), JSON.stringify(value));
    }
    return spawnSync(process.execPath, [path.join(root, 'scripts/ci/validate-summaries.mjs'), dir], {
      cwd: root, env: { ...process.env, ...identity, ...env }, encoding: 'utf8', timeout: 10000,
    });
  } finally { await fs.rm(dir, { recursive: true, force: true }); }
}
test('quality summaries bind even internally consistent records to current execution', async () => {
  for (const field of ['GITHUB_SHA', 'GITHUB_REF', 'GITHUB_WORKFLOW', 'GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT']) {
    const other = field === 'GITHUB_SHA' ? 'b'.repeat(40) : field === 'GITHUB_RUN_ATTEMPT' ? '3' : field === 'GITHUB_RUN_ID' ? '124' : 'other';
    const result = await summaries(() => {}, { [field]: other });
    assert.notEqual(result.status, 0, field);
    assert.match(result.stderr, /execution|provenance/);
  }
});
test('quality summaries reject passed records with unknown or nonzero exit', async () => {
  for (const exit of [null, 1]) {
    const result = await summaries((value) => { value.failure.exit_code = exit; });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /exit/);
  }
});
test('quality summaries reject execution counts exceeding discovery', async () => {
  const result = await summaries((value) => { value.tests.executed_count = 3; });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /count|discovery/);
});
test('quality summaries require trusted execution identity', async () => {
  const result = await summaries(() => {}, { GITHUB_RUN_ID: '' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /execution|provenance/);
});
test('valid quality summaries pass with matching execution', async () => {
  const result = await summaries(() => {});
  assert.equal(result.status, 0, result.stderr);
});
test('summary writer does not report success with a failed command exit', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-ci-writer-'));
  try {
    const result = spawnSync(process.execPath, [path.join(root, 'scripts/ci/write-summary.mjs')], { cwd: dir, encoding: 'utf8', timeout: 10000, env: {
      ...process.env, ...identity, CI_JOB_ID: 'node-build', CI_JOB_STATUS: 'success', CI_DISCOVERY: 'passed',
      CI_DISCOVERED_COUNT: '2', CI_EXECUTED_COUNT: '2', CI_EXIT_CODE: '7', CI_FORCE_FALLBACK: '', CI_FAILURE_CATEGORY: '',
    } });
    assert.equal(result.status, 0, result.stderr);
    const value = JSON.parse(await fs.readFile(path.join(dir, 'ci-summaries/node-build.json'), 'utf8'));
    assert.equal(value.status, 'failed');
    assert.equal(value.failure.exit_code, 7);
    assert.notEqual(value.failure.category, 'none');
  } finally { await fs.rm(dir, { recursive: true, force: true }); }
});

test('summary writer needs an observed zero exit instead of deriving it from job success', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-ci-exit-'));
  try {
    for (const exit of ['', '0']) {
      const result = spawnSync(process.execPath, [path.join(root, 'scripts/ci/write-summary.mjs')], { cwd: dir, encoding: 'utf8', timeout: 10000, env: {
        ...process.env, ...identity, CI_JOB_ID: 'node-build', CI_JOB_STATUS: 'success', CI_DISCOVERY: 'passed', CI_DISCOVERED_COUNT: '2', CI_EXECUTED_COUNT: '2', CI_EXIT_CODE: exit, CI_FORCE_FALLBACK: '', CI_FAILURE_CATEGORY: '',
      } });
      assert.equal(result.status, 0, result.stderr);
      const value = JSON.parse(await fs.readFile(path.join(dir, 'ci-summaries/node-build.json'), 'utf8'));
      assert.equal(value.status, exit === '0' ? 'passed' : 'failed');
      assert.equal(value.failure.exit_code, exit === '0' ? 0 : null);
    }
  } finally { await fs.rm(dir, { recursive: true, force: true }); }
});
