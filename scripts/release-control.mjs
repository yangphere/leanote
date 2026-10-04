import { ReleaseError } from './release-error.mjs';
export function assertReleaseTarget(identity) {
  if (!identity || typeof identity.repository !== 'string' || !new RegExp('^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$').test(identity.repository) ||
      !/^v(0|[1-9][0-9]*)[.](0|[1-9][0-9]*)[.](0|[1-9][0-9]*)$/.test(identity.tag) ||
      !/^(?!0{40}$)[0-9a-f]{40}$/.test(identity.commit)) throw new ReleaseError('invalid release identity');
}

export function assertReleaseAuthorization(identity, approval) {
  assertReleaseTarget(identity);
  if (!/^sha256:[0-9a-f]{64}$/.test(identity.imageDigest)) throw new ReleaseError('invalid release image identity');
  if (approval !== identity.repository + '@' + identity.tag + ':' + identity.commit) throw new ReleaseError('specific release authorization is missing or mismatched');
}

function verifyState(state, identity, requireImage) {
  if (!state || !Object.hasOwn(state, 'imageDigest') || !Object.hasOwn(state, 'release')) throw new ReleaseError('remote state is unknown');
  if (state.commit !== identity.commit) throw new ReleaseError('remote tag identity conflict');
  if (state.imageDigest !== null && state.imageDigest !== identity.imageDigest) throw new ReleaseError('remote image identity conflict');
  if (requireImage && state.imageDigest === null) throw new ReleaseError('original image is missing; recovery cannot rebuild or push');
}

export async function preflightRelease({ identity, approval, remote }) {
  assertReleaseAuthorization(identity, approval);
  const state = await remote.inspect();
  verifyState(state, identity, false);
  if (state.imageDigest !== null || state.release !== null) throw new ReleaseError('release or image already exists');
  return state;
}

async function createAndReadBack(identity, remote) {
  let uncertain = false;
  try { await remote.create(); } catch { uncertain = true; }
  // A failed create may already have written remote state. Never retry it.
  const after = await remote.inspect();
  verifyState(after, identity, true);
  if (after.release === null) throw new ReleaseError('release creation outcome is unconfirmed; no automatic retry');
  await remote.verifyRelease(after.release);
  return { status: uncertain ? 'already-complete' : 'created-complete', release_id: after.release.id };
}

export async function recoverRelease({ identity, approval, remote, verifySource }) {
  assertReleaseAuthorization(identity, approval);
  await verifySource();
  const state = await remote.inspect();
  verifyState(state, identity, true);
  if (state.release !== null) {
    await remote.verifyRelease(state.release);
    return { status: 'already-complete', release_id: state.release.id };
  }
  // The workflow's repository/tag lock spans source verification through read-back.
  const before = await remote.inspect();
  verifyState(before, identity, true);
  if (before.release !== null) {
    await remote.verifyRelease(before.release);
    return { status: 'already-complete', release_id: before.release.id };
  }
  return createAndReadBack(identity, remote);
}

export async function createOrdinaryRelease({ identity, approval, remote }) {
  assertReleaseAuthorization(identity, approval);
  const state = await remote.inspect();
  verifyState(state, identity, true);
  if (state.release !== null) throw new ReleaseError('release already exists; ordinary publish cannot recover');
  return createAndReadBack(identity, remote);
}
