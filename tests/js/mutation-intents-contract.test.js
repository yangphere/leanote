const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');

function harness(crypto = webcrypto) {
  const window = {};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../../public/js/mutation-intents.js'), 'utf8'), { window });
  const requests = [];
  const manager = window.LeanoteMutationIntents.create({
    crypto, serialize: JSON.stringify, changed() {},
    send(url, body, success, failure) { requests.push({ url, body, success, failure }); },
  });
  return { manager, requests };
}
function save(extra = {}) {
  return { path: '/note/updateNoteOrContent', kind: 'save', noteIds: ['n1'],
    payload: { NoteId: 'n1', ExpectedUsn: 7, Title: 'before' }, ...extra };
}

test('unknown retry preserves the exact payload and identity, then rotates for a new intent', () => {
  const { manager, requests } = harness();
  let applied = 0;
  const command = save({ success() { applied++; } });
  const intent = manager.start(command);
  assert.match(intent.payload.OperationId, /^[a-f0-9]{32}$/);
  command.payload.Title = 'after';
  requests[0].failure({ status: 0 });
  assert.equal(intent.state, 'unknown');
  assert.equal(manager.start(save()), null);
  manager.retryUnknown();
  assert.equal(requests[1].body, requests[0].body);
  assert.equal(JSON.parse(requests[1].body).Title, 'before');
  requests[0].success({ Ok: true, Usn: 8 });
  assert.equal(applied, 0, 'a stale attempt must not settle the retry');
  requests[1].success({ Ok: true, Usn: 8 });
  requests[1].success({ Ok: true, Usn: 8 });
  assert.equal(applied, 1);
  assert.equal(manager.hasUnresolved(), false);
  assert.notEqual(manager.start(save()).payload.OperationId, intent.payload.OperationId);
});

test('pending saves block every new mutation for the same note', () => {
  const { manager, requests } = harness();
  manager.start(save());
  assert.equal(manager.start(save()), null);
  assert.equal(manager.start({ path: '/note/deleteNote', kind: 'boolean', noteIds: ['n1'], payload: { noteIds: ['n1'] } }), null);
  assert.equal(requests.length, 1);
  assert.equal(manager.hasUnresolved(), true);
});

test('save needs an authoritative revision and cannot confirm a malformed success', () => {
  for (const revision of [undefined, '7', -1, 1.2, NaN]) {
    const { manager, requests } = harness();
    assert.throws(() => manager.start(save({ payload: { NoteId: 'n1', ExpectedUsn: revision } })), /revision/);
    assert.equal(requests.length, 0);
  }
  for (const ret of [{ Ok: true }, { Ok: true, Usn: '8' }, { Ok: true, Usn: 6 }]) {
    const { manager, requests } = harness();
    let applied = false;
    const intent = manager.start(save({ success() { applied = true; } }));
    requests[0].success(ret);
    assert.equal(intent.state, 'unknown');
    assert.equal(applied, false);
  }
});

test('explicit conflict is retained for reconciliation and never auto-retried', () => {
  const { manager, requests } = harness();
  const intent = manager.start(save());
  requests[0].success({ Ok: false, Msg: 'conflict' });
  assert.equal(intent.state, 'rejected');
  manager.retryUnknown();
  assert.equal(requests.length, 1);
  assert.equal(manager.start(save()), null);
});

test('batch retry retains order, destination, owner and action; only boolean true commits', () => {
  for (const ret of [false, { Ok: false, Msg: 'storage' }, 'true', true]) {
    const { manager, requests } = harness();
    let applied = 0;
    const payload = { noteIds: ['n2', 'n1'], notebookId: 'book', fromUserId: 'owner' };
    const intent = manager.start({ path: '/note/moveNote', kind: 'boolean', noteIds: payload.noteIds, payload, success() { applied++; } });
    payload.noteIds.reverse();
    payload.notebookId = 'changed';
    requests[0].failure({ status: 0 });
    manager.retryUnknown();
    assert.equal(requests[1].body, requests[0].body);
    assert.equal(requests[1].url, '/note/moveNote');
    assert.deepEqual(JSON.parse(requests[1].body).noteIds, ['n2', 'n1']);
    requests[1].success(ret);
    assert.equal(applied, ret === true ? 1 : 0);
    assert.equal(intent.state, ret === true ? 'confirmed' : ret === 'true' ? 'unknown' : 'rejected');
  }
});

test('copy confirms only a complete note list and applies it once', () => {
  const { manager, requests } = harness();
  let applied = 0;
  manager.start({ path: '/note/copySharedNote', kind: 'copy', noteIds: ['n1'], payload: { noteIds: ['n1'], fromUserId: 'owner' }, success() { applied++; } });
  requests[0].success({ Ok: true, Item: null });
  manager.retryUnknown();
  requests[1].success({ Ok: true, Item: [{ NoteId: 'copy-1' }] });
  requests[1].success({ Ok: true, Item: [{ NoteId: 'copy-1' }] });
  assert.equal(applied, 1);
});

test('missing cryptographic randomness fails visibly before any request', () => {
  const { manager, requests } = harness({});
  assert.throws(() => manager.start(save()), /random/);
  assert.equal(requests.length, 0);
});

test('a fresh page has no restored request or automatic replay', () => {
  const first = harness();
  first.manager.start(save());
  first.requests[0].failure({ status: 0 });
  const reloaded = harness();
  reloaded.manager.retryUnknown();
  assert.equal(reloaded.requests.length, 0);
  assert.equal(reloaded.manager.hasUnresolved(), false);
});
