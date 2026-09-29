const assert = require('node:assert/strict');
const test = require('node:test');
const { createHash } = require('node:crypto');
const identity = { repository: 'yangphere/leanote', tag: 'v1.0.0', commit: 'a'.repeat(40), imageDigest: 'sha256:' + 'b'.repeat(64) };
const response = (body, status = 200) => new Response(JSON.stringify(body), { status });
function fakeFetch(overrides = {}) {
  return async (url) => {
    if (overrides[url]) return overrides[url]();
    if (url === 'https://api.github.com/repos/yangphere/leanote') return response({ id: 42, full_name: identity.repository, permissions: { pull: true, push: true } });
    if (url.includes('/git/ref/tags/')) return response({ object: { type: 'commit', sha: identity.commit } });
    if (url.startsWith('https://ghcr.io/token?')) return response({ token: 'test-bearer' });
    if (url === 'https://ghcr.io/v2/') return response({});
    if (url.endsWith('/tags/list?n=1')) return response({ name: 'yangphere/leanote', tags: ['v0.9.0'] });
    if (url.includes('/manifests/')) return response({ errors: [{ code: 'MANIFEST_UNKNOWN' }] }, 404);
    if (url.includes('/releases/tags/')) return response({ message: 'Not Found' }, 404);
    throw new Error('unexpected test request');
  };
}
test('remote absence requires authenticated structured responses, not error text', async () => {
  const { createReleaseRemote } = await import('../../scripts/release-remote.mjs');
  const make = (fetchImpl) => createReleaseRemote({ identity, token: 'test', actor: 'actor', assets: [], fetchImpl });
  assert.deepEqual(await make(fakeFetch()).inspect(), { commit: identity.commit, imageDigest: null, release: null });
  for (const status of [401, 403, 429, 500]) {
    const fetchImpl = fakeFetch({ 'https://api.github.com/repos/yangphere/leanote/releases/tags/v1.0.0': () => response({ message: 'Not Found' }, status) });
    await assert.rejects(() => make(fetchImpl).inspect(), /status/);
  }
  await assert.rejects(() => make(fakeFetch({ 'https://ghcr.io/v2/yangphere/leanote/manifests/v1.0.0': () => response({ errors: [{ code: 'DENIED' }] }, 404) })).inspect(), /unknown/);
  await assert.rejects(() => make(fakeFetch({ 'https://ghcr.io/v2/yangphere/leanote/manifests/v1.0.0': () => response({ errors: [{ code: 'NAME_UNKNOWN' }] }, 404) })).inspect(), /unknown/);
  await assert.rejects(() => make(fakeFetch({ 'https://ghcr.io/v2/yangphere/leanote/tags/list?n=1': () => response({ errors: [{ code: 'DENIED' }] }, 403) })).inspect(), /permission/);
  await assert.rejects(() => make(fakeFetch({ 'https://api.github.com/repos/yangphere/leanote': () => response({ full_name: 'other/repo', permissions: { pull: true, push: true } }) })).inspect(), /identity/);
});
test('formal release verification hashes actual downloaded bytes', async () => {
  const { createReleaseRemote } = await import('../../scripts/release-remote.mjs');
  const data = Buffer.from('approved artifact');
  const assets = [{ name: 'artifact', size: data.length, sha256: createHash('sha256').update(data).digest('hex') }];
  let body = data;
  const remote = createReleaseRemote({ identity, token: 'test', actor: 'actor', assets, fetchImpl: async (url) => url.endsWith('/assets?per_page=100') ? response([{ id: 2, name: 'artifact', state: 'uploaded', size: data.length }]) : new Response(body) });
  const release = { id: 1, tag_name: identity.tag, draft: false, prerelease: false };
  await remote.verifyRelease(release);
  body = Buffer.from('wrong artifact!!!');
  await assert.rejects(() => remote.verifyRelease(release), /size|hash/);
  await assert.rejects(() => remote.verifyRelease({ ...release, draft: true }), /formal/);
});
test('original execution verifies attempt-specific gates and immutable artifact identities', async () => {
  const { verifyOriginalExecution, requiredReleaseJobs } = await import('../../scripts/release-source.mjs');
  const source = { run: '123', attempt: 2, artifacts: { inputs: '1', browser: '2', delivery: '3' } };
  const names = ['leanote-release-inputs-v1', 'browser-release-matrix-v1', 'leanote-delivery-evidence-v1'];
  const run = { id: 123, workflow_id: 99, run_attempt: 2, event: 'push', path: '.github/workflows/release.yml', name: 'Release', status: 'completed', head_sha: identity.commit, repository: { id: 42, full_name: identity.repository }, head_repository: { id: 42, full_name: identity.repository } };
  const workflow = { id: 99, path: '.github/workflows/release.yml', state: 'active' };
  const jobs = requiredReleaseJobs.map((name) => ({ name, status: 'completed', conclusion: 'success', run_id: 123, run_attempt: 2, head_sha: identity.commit }));
  const remote = { repository: async () => run.repository, github: async (endpoint) => endpoint.includes('/workflows/') ? workflow : endpoint.includes('/jobs?') ? { jobs, total_count: jobs.length } : endpoint.includes('/artifacts/') ? { id: Number(endpoint.split('/').at(-1)), name: names[Number(endpoint.split('/').at(-1)) - 1], expired: false, workflow_run: { id: 123, head_sha: identity.commit, repository_id: 42, head_repository_id: 42 } } : run };
  assert.equal((await verifyOriginalExecution({ identity, source, remote })).run.attempt, 2);
  run.path = identity.repository + '/.github/workflows/release.yml@refs/tags/' + identity.tag;
  delete jobs[0].run_attempt;
  assert.equal((await verifyOriginalExecution({ identity, source, remote })).run.attempt, 2);
  jobs[0].run_attempt = 3;
  await assert.rejects(() => verifyOriginalExecution({ identity, source, remote }), /gate/);
  jobs[0].run_attempt = 2;
  workflow.state = 'disabled_manually';
  await assert.rejects(() => verifyOriginalExecution({ identity, source, remote }), /revoked/);
  workflow.state = 'active';
  jobs[0].conclusion = 'skipped';
  await assert.rejects(() => verifyOriginalExecution({ identity, source, remote }), /gate/);
  jobs[0].conclusion = 'success'; run.run_attempt = 3;
  await assert.rejects(() => verifyOriginalExecution({ identity, source, remote }), /attempt|source/);
});
