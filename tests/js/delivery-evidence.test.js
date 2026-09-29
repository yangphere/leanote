const assert = require('node:assert/strict');
const test = require('node:test');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const identity = { repository: 'yangphere/leanote', commit: 'a'.repeat(40), ref: 'refs/tags/v1.0.0', workflow: 'Release', run: { id: '123', attempt: 2 } };
const hash = (x) => crypto.createHash('sha256').update(JSON.stringify(x)).digest('hex');

async function fixture() {
  const api = await import('../../scripts/ci/delivery-evidence.mjs');
  const catalog = await api.buildDeliveryCatalog(process.cwd());
  const result = {
    schema_version: 'leanote.delivery-evidence.v1', ...identity, catalog_sha256: hash(catalog),
    runner: { executable_sha256: 'b'.repeat(64), arguments_sha256: 'c'.repeat(64) },
    started_at: '2026-09-29T01:00:00Z', finished_at: '2026-09-29T01:30:00Z',
    exit_code: 0, cleanup_status: 'passed',
    records: catalog.scenarios.map((s) => ({ id: s.id, status: 'passed', discovered: 2, executed: 2, passed: 2, failed: 0, skipped: 0, exit_code: 0, cleanup_status: 'passed', assertions: ['case.success', 'case.failure'], environment: 'isolated-test', fixture_sha256: 'd'.repeat(64) })),
  };
  return { api, catalog, result };
}
test('delivery catalog binds all nine owners and exactly 38 authoritative action IDs', async () => {
  const { catalog } = await fixture();
  assert.equal(new Set(catalog.sources.map((s) => s.owner)).size, 10);
  assert.equal(catalog.scenarios.filter((s) => s.id.startsWith('notes-action:')).length, 38);
  assert.equal(new Set(catalog.scenarios.map((s) => s.id)).size, catalog.scenarios.length);
  assert.equal(catalog.scenarios.filter((s) => s.id.startsWith('presentation-frontend:manual-')).length, 14);
  for (const topology of ['mongo7-standalone', 'mongo8-standalone', 'mongo8-replica-set']) assert(catalog.scenarios.some((s) => s.id === 'environment:' + topology));
  assert(catalog.scenarios.some((s) => s.id === 'compatibility:D-H6'));
  assert(catalog.scenarios.some((s) => s.id === 'compatibility:MOD-004'));
});
test('delivery validator rejects omissions, duplicate coverage, skipped and unverified cleanup', async () => {
  const { api, catalog, result } = await fixture();
  assert.doesNotThrow(() => api.validateDeliveryEvidence(result, catalog, identity));
  for (const mutate of [
    (x) => x.records.pop(),
    (x) => { x.records[1] = x.records[0]; },
    (x) => { x.records[0].skipped = 1; },
    (x) => { x.records[0].cleanup_status = 'unknown'; },
    (x) => { x.cleanup_status = 'failed'; },
    (x) => { x.records[0].executed = 0; },
    (x) => { x.records[0].exit_code = 1; },
  ]) {
    const bad = structuredClone(result); mutate(bad);
    assert.throws(() => api.validateDeliveryEvidence(bad, catalog, identity));
  }
});
test('delivery validator rejects self-consistent stale provenance and catalog drift', async () => {
  const { api, catalog, result } = await fixture();
  for (const change of [{ commit: 'e'.repeat(40) }, { repository: 'other/repo' }, { run: { id: '123', attempt: 3 } }, { catalog_sha256: 'f'.repeat(64) }]) {
    assert.throws(() => api.validateDeliveryEvidence({ ...result, ...change }, catalog, identity));
  }
  assert.throws(() => api.validateDeliveryEvidence({ ...result, raw_log: 'unexpected' }, catalog, identity));
});
test('delivery runner fails closed before command execution when protected config is missing', async () => {
  const { runDeliveryEvidence } = await import('../../scripts/ci/delivery-evidence.mjs');
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-delivery-'));
  try {
    await assert.rejects(() => runDeliveryEvidence({ root: process.cwd(), output: dir, env: {} }), /provenance|config/);
    assert.deepEqual(await fs.readdir(dir), []);
  } finally { await fs.rm(dir, { recursive: true, force: true }); }
});

test('catalog follows an archived delivery task but rejects ambiguous copies', async () => {
  const { api, catalog } = await fixture();
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-catalog-archive-'));
  try {
    for (const source of catalog.sources) {
      const destination = path.join(root, source.path);
      await fs.mkdir(path.dirname(destination), { recursive: true });
      await fs.copyFile(source.path, destination);
    }
    const live = path.join(root, '.trellis/tasks/09-08-delivery-verification');
    const archived = path.join(root, '.trellis/tasks/archive/2027-01/09-08-delivery-verification');
    await fs.mkdir(path.dirname(archived), { recursive: true });
    await fs.rename(live, archived);
    const moved = await api.buildDeliveryCatalog(root);
    assert.equal(moved.scenarios.length, catalog.scenarios.length);
    assert(moved.sources.some((s) => s.path.includes('archive/2027-01/09-08-delivery-verification')));
    await fs.mkdir(live);
    await assert.rejects(() => api.buildDeliveryCatalog(root), /uniquely/);
  } finally { await fs.rm(root, { recursive: true, force: true }); }
});

test('protected runner protocol executes a process and rejects nonzero, timeout and cleanup failures', async (t) => {
  // Synthetic child tests the transport contract ONLY; it is never a real delivery adapter.
  const { api, catalog } = await fixture();
  const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-delivery-process-'));
  t.after(() => fs.rm(temp, { recursive: true, force: true }));
  const root = path.join(temp, 'candidate');
  for (const source of catalog.sources) {
    const destination = path.join(root, source.path);
    await fs.mkdir(path.dirname(destination), { recursive: true });
    await fs.copyFile(source.path, destination);
  }
  const git = (args) => execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git(['init', '--quiet']); git(['add', '.']);
  git(['-c', 'user.name=Protocol Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '--quiet', '-m', 'isolated protocol fixture']);
  const script = path.join(temp, 'protocol-fixture.mjs');
  await fs.writeFile(script, [
    "import fs from 'node:fs';",
    "if (process.env.FIXTURE_MODE === 'timeout') await new Promise(resolve => setTimeout(resolve, 30000));",
    "if (process.env.FIXTURE_MODE === 'nonzero') { process.stderr.write('do-not-publish-raw-fixture-diagnostics'); process.exit(7); }",
    "const catalog = JSON.parse(fs.readFileSync(process.argv[process.argv.indexOf('--catalog') + 1]));",
    "const records = catalog.scenarios.map(s => ({id:s.id,status:'passed',discovered:1,executed:1,passed:1,failed:0,skipped:0,exit_code:0,cleanup_status:'passed',assertions:['fixture.contract'],environment:'synthetic-protocol-only',fixture_sha256:'f'.repeat(64)}));",
    "fs.writeFileSync(process.argv[process.argv.indexOf('--result') + 1], JSON.stringify({cleanup_status: process.env.FIXTURE_MODE === 'cleanup' ? 'failed' : 'passed', records}));",
  ].join(String.fromCharCode(10)));
  const configPath = path.join(temp, 'runner.json');
  const config = { executable: process.execPath, sha256: crypto.createHash('sha256').update(await fs.readFile(process.execPath)).digest('hex'), args: [script], timeout_ms: 10000 };
  const env = { ...process.env, GITHUB_REPOSITORY: identity.repository, GITHUB_SHA: git(['rev-parse', 'HEAD']), GITHUB_REF: identity.ref, GITHUB_WORKFLOW: identity.workflow, GITHUB_RUN_ID: identity.run.id, GITHUB_RUN_ATTEMPT: String(identity.run.attempt), LEANOTE_DELIVERY_RUNNER_CONFIG: configPath };
  for (const mode of ['success', 'nonzero', 'cleanup', 'timeout']) {
    await fs.writeFile(configPath, JSON.stringify({ ...config, timeout_ms: mode === 'timeout' ? 100 : 10000 }));
    const output = path.join(temp, mode), trace = {};
    const run = () => api.runDeliveryEvidence({ root, output, env: { ...env, FIXTURE_MODE: mode }, trace });
    if (mode === 'success') {
      await run();
      await api.validateDeliveryDirectory(output, root, api.deliveryIdentity(env));
      assert.equal(trace.cleanup_status, 'passed');
    } else {
      await assert.rejects(run, /execution|cleanup/);
      await assert.rejects(() => fs.access(output), { code: 'ENOENT' });
      assert.equal(trace.cleanup_status, mode === 'cleanup' ? 'failed' : 'unknown');
      if (mode === 'nonzero') assert.equal(trace.command_exit_code, 7);
      if (mode === 'timeout') assert.equal(trace.process_failure, 'timeout-or-output-limit');
      assert.doesNotMatch(JSON.stringify(trace), /do-not-publish/);
    }
    assert.equal(trace.workspace_cleanup_status, 'passed');
  }
});
