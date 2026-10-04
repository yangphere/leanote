import fs from 'node:fs/promises';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { validateReleaseArtifact } from './validate-release-artifact.mjs';
import { validateBrowserArtifact } from './validate-browser-artifact.mjs';
import { validateDeliveryDirectory, deliveryIdentity } from './ci/delivery-evidence.mjs';
import { assertReleaseTarget, preflightRelease, recoverRelease, createOrdinaryRelease } from './release-control.mjs';
import { createReleaseRemote } from './release-remote.mjs';
import { sourceFromEnvironment, verifyOriginalExecution } from './release-source.mjs';
import { ReleaseError, describeReleaseError } from './release-error.mjs';

export async function runReleaseCommand(mode, env = process.env, trace = {}) {
  trace.stage = 'identity';
  if (!['source-check', 'preflight', 'create', 'recover', 'reconcile'].includes(mode)) throw new ReleaseError('unknown release mode');
  const recovery = mode === 'recover' || mode === 'source-check';
  const target = { repository: env.GITHUB_REPOSITORY, tag: env.RELEASE_TAG, commit: recovery ? env.RECOVERY_COMMIT : env.GITHUB_SHA };
  assertReleaseTarget(target);
  trace.stage = 'authorization';
  const approval = env.RELEASE_APPROVED_IDENTITY;
  if (approval !== target.repository + '@' + target.tag + ':' + target.commit) throw new ReleaseError('specific release authorization missing or mismatched');
  trace.stage = 'execution-provenance';
  const current = deliveryIdentity(env);
  const source = recovery ? sourceFromEnvironment(env) : null;
  const connection = { identity: target, token: env.GH_TOKEN, actor: env.GITHUB_ACTOR, assets: [] };
  const lookup = createReleaseRemote(connection);
  trace.stage = 'source-execution';
  const original = source ? await verifyOriginalExecution({ identity: target, source, remote: lookup }) : null;
  if (mode === 'source-check') return { schema_version: 'leanote.release-result.v1', mode, status: 'source-verified', target, source: original, execution: current };
  const expected = original || current;
  trace.stage = 'candidate-checkout';
  if (expected.commit !== target.commit || expected.ref !== 'refs/tags/' + target.tag) throw new ReleaseError('release target does not match verified execution');
  const candidateRoot = path.resolve(env.RELEASE_CANDIDATE_ROOT || '.');
  const candidate = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: candidateRoot, encoding: 'utf8' }).trim();
  if (candidate !== target.commit) throw new ReleaseError('release candidate checkout mismatch');
  if (execFileSync('git', ['status', '--porcelain', '--untracked-files=no'], { cwd: candidateRoot, encoding: 'utf8' }).trim()) throw new ReleaseError('release candidate has tracked modifications');
  const inputs = path.resolve(env.RELEASE_INPUTS_DIR || 'dist');
  const browser = path.resolve(env.RELEASE_BROWSER_DIR || 'test-results');
  const delivery = path.resolve(env.RELEASE_DELIVERY_DIR || 'delivery-results');
  trace.stage = 'release-inputs';
  const manifest = await validateReleaseArtifact({ directory: inputs, candidateRoot, expected: { ...expected, tag: target.tag } });
  trace.stage = 'browser-evidence';
  await validateBrowserArtifact({ root: browser, phase: 'final' }, expected);
  trace.stage = 'delivery-evidence';
  await validateDeliveryDirectory(delivery, candidateRoot, expected);
  const identity = { ...target, imageDigest: manifest.image_digest };
  const assets = [];
  for (const entry of manifest.files.filter((x) => ['tarball', 'checksum', 'metadata'].includes(x.kind))) {
    const assetPath = path.join(inputs, entry.path);
    const stat = await fs.lstat(assetPath);
    if (!stat.isFile() || stat.isSymbolicLink()) throw new ReleaseError('original release asset is not a regular file');
    assets.push({ name: entry.path, path: assetPath, size: stat.size, sha256: entry.sha256 });
  }
  if (assets.length !== 3) throw new ReleaseError('release asset set incomplete');
  const remote = createReleaseRemote({ ...connection, identity, assets });
  const create = remote.create;
  remote.create = async () => {
    trace.stage = 'release-write-and-readback';
    trace.writeAttempted = true;
    return create();
  };
  trace.stage = 'remote-reconciliation';
  let result;
  if (mode === 'reconcile') {
    const state = await remote.inspect();
    if (state.commit !== identity.commit || (state.imageDigest !== null && state.imageDigest !== identity.imageDigest)) throw new ReleaseError('remote candidate identity conflict');
    if (state.release !== null) {
      if (state.imageDigest === null) throw new ReleaseError('release exists but original image is missing');
      await remote.verifyRelease(state.release);
      result = { status: 'already-complete', release_id: state.release.id };
    } else {
      result = { status: state.imageDigest === null ? 'confirmed-absent' : 'image-present-release-absent' };
    }
  } else if (mode === 'preflight') {
    await preflightRelease({ identity, approval, remote });
    result = { status: 'preflight-passed' };
  } else if (mode === 'recover') {
    result = await recoverRelease({ identity, approval, remote, verifySource: async () => {
      const verified = await verifyOriginalExecution({ identity, source, remote });
      if (JSON.stringify(verified) !== JSON.stringify(original)) throw new ReleaseError('source execution changed during recovery');
    } });
  } else {
    result = await createOrdinaryRelease({ identity, approval, remote });
  }
  return { schema_version: 'leanote.release-result.v1', mode, ...result, target: identity, source: original, execution: current };
}

export async function executeReleaseCommand(mode, env = process.env) {
  const trace = { stage: 'identity', writeAttempted: false };
  try { return await runReleaseCommand(mode, env, trace); }
  catch (error) {
    const select = (value, pattern) => typeof value === 'string' && pattern.test(value) ? value : null;
    const numeric = (value) => select(value, /^[1-9][0-9]{0,19}$/);
    const commit = (value) => select(value, /^[0-9a-f]{40}$/);
    const recovery = mode === 'recover' || mode === 'source-check';
    return {
      schema_version: 'leanote.release-result.v1', mode: ['recover', 'source-check', 'create', 'preflight', 'reconcile'].includes(mode) ? mode : null,
      status: trace.writeAttempted ? 'publication-unconfirmed' : 'blocked',
      failure: { stage: trace.stage, ...describeReleaseError(error) },
      write_attempted: trace.writeAttempted,
      target: { repository: select(env.GITHUB_REPOSITORY, /^[A-Za-z0-9_.-]+[/][A-Za-z0-9_.-]+$/), tag: select(env.RELEASE_TAG, /^v[0-9]+[.][0-9]+[.][0-9]+$/), commit: commit(recovery ? env.RECOVERY_COMMIT : env.GITHUB_SHA) },
      source: recovery ? { run: { id: numeric(env.SOURCE_RUN_ID), attempt: numeric(env.SOURCE_ATTEMPT) }, artifacts: { inputs: numeric(env.SOURCE_INPUTS_ARTIFACT), browser: numeric(env.SOURCE_BROWSER_ARTIFACT), delivery: numeric(env.SOURCE_DELIVERY_ARTIFACT) } } : null,
      execution: { commit: commit(env.GITHUB_SHA), run: { id: numeric(env.GITHUB_RUN_ID), attempt: numeric(env.GITHUB_RUN_ATTEMPT) } },
    };
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const record = await executeReleaseCommand(process.argv[2]);
  if (record.failure) process.exitCode = 1;
  const output = process.env.RELEASE_RESULT_PATH;
  if (output) { await fs.mkdir(path.dirname(path.resolve(output)), { recursive: true }); await fs.writeFile(output, JSON.stringify(record)); }
  process.stdout.write(JSON.stringify(record) + String.fromCharCode(10));
}
