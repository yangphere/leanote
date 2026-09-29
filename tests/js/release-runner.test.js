const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const test = require('node:test');
const hash = (value) => createHash('sha256').update(value).digest('hex');
const json = (value, status = 200) => new Response(JSON.stringify(value), { status });

async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-recovery-contract-'));
  const candidate = path.join(root, 'candidate');
  await fs.mkdir(candidate);
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const { buildDeliveryCatalog } = await import('../../scripts/ci/delivery-evidence.mjs');
  const { requiredReleaseJobs } = await import('../../scripts/release-source.mjs');
  const originalCatalog = await buildDeliveryCatalog(process.cwd());
  for (const source of originalCatalog.sources) {
    const dest = path.join(candidate, source.path);
    await fs.mkdir(path.dirname(dest), { recursive: true });
    await fs.copyFile(source.path, dest);
  }
  await fs.writeFile(path.join(candidate, 'package.json'), JSON.stringify({ version: '2.3.4' }));
  await fs.writeFile(path.join(candidate, 'package-lock.json'), JSON.stringify({ packages: { '': { version: '2.3.4' } } }));
  const git = (args) => execFileSync('git', args, { cwd: candidate, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git(['init', '--quiet']); git(['add', '.']);
  git(['-c', 'user.name=Contract Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '--quiet', '-m', 'isolated release fixture']);
  const commit = git(['rev-parse', 'HEAD']);
  const tag = 'v2.3.4';
  const config = Buffer.from(JSON.stringify({ os: 'linux', architecture: 'amd64', config: { Labels: { 'org.opencontainers.image.revision': commit, 'org.opencontainers.image.version': '2.3.4' } } }));
  const manifest = Buffer.from(JSON.stringify({ schemaVersion: 2, config: { digest: 'sha256:' + hash(config) } }));
  const imageDigest = 'sha256:' + hash(manifest);
  const inputs = path.join(root, 'dist'), browser = path.join(root, 'browser'), delivery = path.join(root, 'delivery');
  for (const dir of [inputs, browser, delivery]) await fs.mkdir(dir);
  await fs.writeFile(path.join(inputs, 'leanote-' + tag + '-linux-amd64.tar.gz'), 'original immutable tarball bytes');
  const { buildReleaseInputs } = await import('../../scripts/release-metadata.mjs');
  await buildReleaseInputs({ root: candidate, outDir: inputs, env: { RELEASE_TAG: tag, GIT_COMMIT: commit, GITHUB_REF: 'refs/tags/' + tag, GITHUB_WORKFLOW: 'Release', GITHUB_RUN_ID: '100', GITHUB_RUN_ATTEMPT: '2', SOURCE_DATE_EPOCH: git(['show', '-s', '--format=%ct']), IMAGE_DIGEST: imageDigest, BASE_IMAGE_DIGEST: 'sha256:' + 'c'.repeat(64), PROVENANCE: 'disabled', ATTESTATION: 'disabled', SBOM: 'disabled' } });
  const { jcsSha256 } = await import('../../scripts/jcs.mjs');
  const coverage = ['business-flows', 'editor-flows', 'bootstrap-components', 'leaui-image-iframe'];
  const records = [], summaries = [];
  for (const product of ['chrome', 'edge', 'firefox', 'safari']) {
    for (const slot of ['current_major', 'previous_major']) {
      const value = { browser_product: product, release_slot: slot, items: coverage.map((id) => ({ id, discovered_count: 1, executed_count: 1, entrypoints: ['note'], iframes: id === 'leaui-image-iframe' ? ['tinymce/plugins/leaui_image/index.html'] : [], result: 'passed' })) };
      const digest = jcsSha256(value);
      summaries.push({ ...value, coverage_summary_sha256: digest });
      records.push({ commit, browser_product: product, release_slot: slot, browser_version: slot === 'current_major' ? '123.4.5' : '122.4.5', os: 'linux', environment: 'real-browser', coverage, coverage_summary_sha256: digest, auth_gate: 'passed', error_gate: 'passed', resource_gate: 'passed', executed_at: '2026-09-29T01:00:00Z', result: 'passed' });
    }
  }
  const matrix = JSON.stringify({ schema_version: 'leanote.browser-smoke.release-matrix.v1', commit, records });
  const provenance = { schema_version: 'leanote.browser-smoke.release-matrix-provenance.v1', matrix_sha256: hash(matrix), commit, ref: 'refs/tags/' + tag, producer_workflow: 'Protected browser release evidence', release_run: { id: '100', attempt: 2 }, coverage_summaries: summaries };
  await fs.writeFile(path.join(browser, 'release-matrix.json'), matrix);
  await fs.writeFile(path.join(browser, 'provenance.json'), JSON.stringify(provenance));
  const catalog = await buildDeliveryCatalog(candidate);
  const evidence = { schema_version: 'leanote.delivery-evidence.v1', repository: 'yangphere/leanote', commit, ref: 'refs/tags/' + tag, workflow: 'Release', run: { id: '100', attempt: 2 }, catalog_sha256: hash(JSON.stringify(catalog)), runner: { executable_sha256: 'd'.repeat(64), arguments_sha256: 'e'.repeat(64) }, started_at: '2026-09-29T01:00:00Z', finished_at: '2026-09-29T01:30:00Z', exit_code: 0, cleanup_status: 'passed', records: catalog.scenarios.map((s) => ({ id: s.id, status: 'passed', discovered: 1, executed: 1, passed: 1, failed: 0, skipped: 0, exit_code: 0, cleanup_status: 'passed', assertions: ['fixture.contract'], environment: 'fixture-only', fixture_sha256: 'f'.repeat(64) })) };
  await fs.writeFile(path.join(delivery, 'catalog.json'), JSON.stringify(catalog));
  await fs.writeFile(path.join(delivery, 'delivery-evidence.json'), JSON.stringify(evidence));
  const env = { GITHUB_REPOSITORY: 'yangphere/leanote', RELEASE_TAG: tag, RECOVERY_COMMIT: commit, GITHUB_SHA: 'b'.repeat(40), GITHUB_REF: 'refs/heads/dev', GITHUB_WORKFLOW: 'Recover Release', GITHUB_RUN_ID: '200', GITHUB_RUN_ATTEMPT: '1', SOURCE_RUN_ID: '100', SOURCE_ATTEMPT: '2', SOURCE_INPUTS_ARTIFACT: '1', SOURCE_BROWSER_ARTIFACT: '2', SOURCE_DELIVERY_ARTIFACT: '3', RELEASE_APPROVED_IDENTITY: 'yangphere/leanote@' + tag + ':' + commit, RELEASE_CANDIDATE_ROOT: candidate, RELEASE_INPUTS_DIR: inputs, RELEASE_BROWSER_DIR: browser, RELEASE_DELIVERY_DIR: delivery, GH_TOKEN: 'fixture-token', GITHUB_ACTOR: 'fixture-actor' };
  const state = { writes: 0, release: null, uploaded: [], expired: false, corrupt: false, malformed: false };
  const fetchImpl = async (raw, options = {}) => {
    const url = new URL(raw);
    if (state.malformed) return new Response('secret-token-not-json');
    if (url.hostname === 'ghcr.io') {
      if (url.pathname === '/token') return json({ token: 'fixture-bearer' });
      if (url.pathname === '/v2/') return json({});
      if (url.pathname.includes('/manifests/')) return new Response(manifest, { headers: { 'docker-content-digest': imageDigest } });
      if (url.pathname.includes('/blobs/')) return new Response(config);
    }
    const endpoint = url.pathname.replace('/repos/yangphere/leanote', '');
    if (!endpoint) return json({ id: 42, full_name: env.GITHUB_REPOSITORY, permissions: { pull: true, push: true } });
    if (endpoint.includes('/git/ref/tags/')) return json({ object: { type: 'commit', sha: commit } });
    if (endpoint === '/actions/workflows/99') return json({ id: 99, path: '.github/workflows/release.yml', state: 'active' });
    if (endpoint === '/actions/runs/100/attempts/2/jobs') return json({ total_count: requiredReleaseJobs.length, jobs: requiredReleaseJobs.map((name) => ({ name, run_id: 100, status: 'completed', conclusion: 'success', head_sha: commit })) });
    if (endpoint === '/actions/runs/100/attempts/2') return json({ id: 100, run_attempt: 2, workflow_id: 99, event: 'push', path: 'yangphere/leanote/.github/workflows/release.yml@refs/tags/' + tag, name: 'Release', status: 'completed', conclusion: 'failure', head_sha: commit, repository: { id: 42, full_name: env.GITHUB_REPOSITORY }, head_repository: { id: 42, full_name: env.GITHUB_REPOSITORY } });
    if (endpoint.startsWith('/actions/artifacts/')) {
      const id = Number(endpoint.split('/').at(-1));
      return json({ id, name: ['leanote-release-inputs-v1', 'browser-release-matrix-v1', 'leanote-delivery-evidence-v1'][id - 1], expired: state.expired, workflow_run: { id: 100, head_sha: commit, repository_id: 42, head_repository_id: 42 } });
    }
    if (endpoint.startsWith('/releases/tags/')) return state.release ? json(state.release) : json({ message: 'Not Found' }, 404);
    if (endpoint === '/releases' && options.method === 'POST') {
      state.writes += 1; state.release = { id: 500, tag_name: tag, draft: false, prerelease: false }; return json(state.release, 201);
    }
    if (endpoint === '/releases/500/assets') {
      if (options.method !== 'POST') return json(state.uploaded.map(({ bytes, ...entry }) => entry));
      state.writes += 1; const chunks = [];
      for await (const chunk of options.body) chunks.push(chunk);
      const bytes = Buffer.concat(chunks);
      state.uploaded.push({ id: state.uploaded.length + 600, name: url.searchParams.get('name'), state: 'uploaded', size: bytes.length, bytes });
      return json({}, 201);
    }
    if (endpoint.startsWith('/releases/assets/')) {
      const asset = state.uploaded.find((a) => a.id === Number(endpoint.split('/').at(-1)));
      return new Response(state.corrupt ? Buffer.alloc(asset.size) : asset.bytes);
    }
    throw Error('unexpected fixture request ' + url.pathname);
  };
  const saved = globalThis.fetch; globalThis.fetch = fetchImpl; t.after(() => { globalThis.fetch = saved; });
  return { env, state, browser, provenance };
}

test('recovery validates original version and attempt, creates once and then reads only', async (t) => {
  const { env, state } = await fixture(t);
  const { executeReleaseCommand } = await import('../../scripts/release-runner.mjs');
  const result = await executeReleaseCommand('recover', env);
  assert.equal(result.status, 'created-complete', JSON.stringify(result));
  assert.equal(state.writes, 4);
  assert.equal(result.target.tag, 'v2.3.4');
  assert.equal(result.source.run.id, '100');
  assert.equal(result.execution.run.id, '200');
  assert.equal((await executeReleaseCommand('recover', env)).status, 'already-complete');
  assert.equal(state.writes, 4);
});

test('cross-attempt browser artifact and expired original artifact cannot write', async (t) => {
  const { env, state, browser, provenance } = await fixture(t);
  const { executeReleaseCommand } = await import('../../scripts/release-runner.mjs');
  provenance.release_run.attempt = 1;
  await fs.writeFile(path.join(browser, 'provenance.json'), JSON.stringify(provenance));
  const stale = await executeReleaseCommand('recover', env);
  assert.equal(stale.failure.stage, 'browser-evidence');
  assert.equal(state.writes, 0);
  state.expired = true;
  const expired = await executeReleaseCommand('recover', env);
  assert.equal(expired.failure.stage, 'source-execution');
  assert.equal(state.writes, 0);
});

test('asset read-back corruption keeps publication unconfirmed without a second create', async (t) => {
  const { env, state } = await fixture(t); state.corrupt = true;
  const { executeReleaseCommand } = await import('../../scripts/release-runner.mjs');
  const result = await executeReleaseCommand('recover', env);
  assert.equal(result.status, 'publication-unconfirmed');
  assert.match(result.failure.reason, /hash mismatch/);
  assert.equal(state.writes, 4);
  await executeReleaseCommand('recover', env);
  assert.equal(state.writes, 4);
});

test('malformed remote JSON reports the stage and never exposes response bytes', async (t) => {
  const { env, state } = await fixture(t); state.malformed = true;
  const { executeReleaseCommand } = await import('../../scripts/release-runner.mjs');
  const result = await executeReleaseCommand('recover', env);
  assert.equal(result.failure.category, 'invalid-json');
  assert.equal(result.failure.stage, 'source-execution');
  assert.doesNotMatch(JSON.stringify(result), /secret-token-not-json/);
  assert.equal(state.writes, 0);
});

test('checksum line-ending tampering fails before creating any remote object', async (t) => {
  const { env, state } = await fixture(t);
  const checksumPath = path.join(env.RELEASE_INPUTS_DIR, 'leanote-' + env.RELEASE_TAG + '-linux-amd64.tar.gz.sha256');
  const original = await fs.readFile(checksumPath);
  await fs.writeFile(checksumPath, Buffer.concat([original.subarray(0, original.length - 1), Buffer.from([13, 10])]));
  const { executeReleaseCommand } = await import('../../scripts/release-runner.mjs');
  const result = await executeReleaseCommand('recover', env);
  assert.equal(result.failure.stage, 'release-inputs');
  assert.equal(state.writes, 0);
});
