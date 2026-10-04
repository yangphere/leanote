const assert = require('node:assert/strict');
const test = require('node:test');
const identity = { repository: 'yangphere/leanote', tag: 'v1.0.0', commit: 'a'.repeat(40), imageDigest: 'sha256:' + 'b'.repeat(64) };
const approval = identity.repository + '@' + identity.tag + ':' + identity.commit;
function remoteFixture() {
  const calls = [];
  let release = null;
  const remote = {
    async inspect() { calls.push('inspect'); return { commit: identity.commit, imageDigest: identity.imageDigest, release }; },
    async verifyRelease(value) { calls.push('verify'); if (!value.complete) throw new Error('partial release'); },
    async create() { calls.push('create'); release = { id: 1, complete: true }; },
  };
  return { calls, remote, set: (value) => { release = value; } };
}
test('recovery validates original inputs before any remote write and creates only missing release', async () => {
  const { recoverRelease } = await import('../../scripts/release-control.mjs');
  const x = remoteFixture();
  const result = await recoverRelease({ identity, approval, remote: x.remote, verifySource: async () => x.calls.push('source') });
  assert.equal(result.status, 'created-complete');
  assert.equal(x.calls[0], 'source');
  assert.equal(x.calls.filter((c) => c === 'create').length, 1);
  assert.equal(x.calls.at(-1), 'verify');
});
test('recovery does not create with absent authorization or failed source gates', async () => {
  const { recoverRelease } = await import('../../scripts/release-control.mjs');
  for (const options of [{ approval: '' }, { verifySource: async () => { throw new Error('source gate failed'); } }]) {
    const x = remoteFixture();
    await assert.rejects(() => recoverRelease({ identity, approval, remote: x.remote, verifySource: async () => {}, ...options }));
    assert(!x.calls.includes('create'));
  }
});
test('recovery confirms complete existing release without writes and rejects partial existing release', async () => {
  const { recoverRelease } = await import('../../scripts/release-control.mjs');
  for (const complete of [true, false]) {
    const x = remoteFixture(); x.set({ id: 1, complete });
    const run = () => recoverRelease({ identity, approval, remote: x.remote, verifySource: async () => {} });
    if (complete) assert.equal((await run()).status, 'already-complete');
    else await assert.rejects(run, /partial/);
    assert(!x.calls.includes('create'));
  }
});
test('unknown queries and missing or conflicting image and tag never cause create', async () => {
  const { recoverRelease } = await import('../../scripts/release-control.mjs');
  for (const state of [null, { commit: 'c'.repeat(40), imageDigest: identity.imageDigest, release: null }, { commit: identity.commit, imageDigest: null, release: null }]) {
    const x = remoteFixture();
    x.remote.inspect = async () => { if (!state) throw new Error('remote unknown'); return state; };
    await assert.rejects(() => recoverRelease({ identity, approval, remote: x.remote, verifySource: async () => {} }));
    assert(!x.calls.includes('create'));
  }
});
test('create response loss is reconciled by reads without repeating the mutation', async () => {
  const { recoverRelease } = await import('../../scripts/release-control.mjs');
  const x = remoteFixture();
  x.remote.create = async () => { x.calls.push('create'); x.set({ id: 1, complete: true }); throw new Error('response lost'); };
  const result = await recoverRelease({ identity, approval, remote: x.remote, verifySource: async () => {} });
  assert.equal(result.status, 'already-complete');
  assert.equal(x.calls.filter((c) => c === 'create').length, 1);
});
test('a competing create found by the locked recheck produces a read-only no-op', async () => {
  const { recoverRelease } = await import('../../scripts/release-control.mjs');
  const x = remoteFixture(); let reads = 0;
  const inspect = x.remote.inspect;
  x.remote.inspect = async () => { if (++reads === 2) x.set({ id: 1, complete: true }); return inspect(); };
  assert.equal((await recoverRelease({ identity, approval, remote: x.remote, verifySource: async () => {} })).status, 'already-complete');
  assert(!x.calls.includes('create'));
});
test('ordinary publish rejects either existing remote object and never switches to recovery', async () => {
  const { preflightRelease } = await import('../../scripts/release-control.mjs');
  const x = remoteFixture();
  await assert.rejects(() => preflightRelease({ identity, approval, remote: x.remote }), /exists/);
  assert(!x.calls.includes('create'));
});
