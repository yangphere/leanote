import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { verifyImageManifest } from '../../../../../../../scripts/verify-image-manifest.mjs';

const root = path.dirname(fileURLToPath(import.meta.url));
const evidence = path.join(root, 'registry-evidence');
const bytes = (name) => fs.readFileSync(path.join(evidence, name));
const json = (name) => JSON.parse(bytes(name));
const digest = (data) => `sha256:${createHash('sha256').update(data).digest('hex')}`;
const manifests = {};

for (const version of ['2.0.1', '2.0.2', '1.9.9']) {
  const metadata = json(`build-${version}.json`);
  manifests[version] = verifyImageManifest({
    manifestBytes: bytes(`version-${version}.json`),
    expectedManifestDigest: metadata['containerimage.digest'],
    expectedConfigDigest: metadata['containerimage.config.digest'],
  });
}
const decisions = [];
for (const [label, candidate, hasLatest, promote] of [
  ['initialize', '2.0.1', false, true],
  ['advance', '2.0.2', true, true],
  ['equal', '2.0.2', true, false],
  ['older', '1.9.9', true, false],
]) {
  assert.deepEqual(json(`decision-${label}.json`), { candidate, hasLatest, promote });
  const finalVersion = label === 'initialize' ? '2.0.1' : '2.0.2';
  assert.deepEqual(bytes(`latest-${label}-after.json`), bytes(`version-${finalVersion}.json`));
  const config = json(`latest-config-${label}-after.json`);
  assert.equal(config.config.Labels['org.opencontainers.image.version'], finalVersion);
  assert.equal(config.config.Labels['org.opencontainers.image.source'], 'https://github.com/yangphere/leanote');
  assert.equal(config.os, 'linux');
  assert.equal(config.architecture, 'amd64');
  if (!promote) assert.deepEqual(bytes(`latest-${label}-before.json`), bytes(`latest-${label}-after.json`));
  decisions.push({ label, candidate, hasLatest, promote, latestVersion: finalVersion, latestDigest: digest(bytes(`latest-${label}-after.json`)) });
}
assert.notEqual(manifests['2.0.1'].manifestDigest, manifests['2.0.2'].manifestDigest);
const production = {};
for (const tag of ['2.0.1', 'latest']) {
  assert.deepEqual(bytes(`production-${tag}-before.json`), bytes(`production-${tag}-after.json`));
  production[tag] = digest(bytes(`production-${tag}-after.json`));
}
assert.match(bytes('bootstrap-default.txt').toString(), /GHCR initial package creation is disabled/);
const pending = JSON.parse(fs.readFileSync(path.join(root, 'queue-snapshot-pending.json')));
assert.equal(pending.find((run) => run.id === 37184573081).status, 'in_progress');
for (const runId of [37184614263, 37184617541]) assert.equal(pending.find((run) => run.id === runId).status, 'pending');
const runs = JSON.parse(fs.readFileSync(path.join(root, 'queue-snapshot-after-promotion.json')));
for (const run of runs) { assert.equal(run.status, 'completed'); assert.equal(run.conclusion, 'success'); }
const queue = [];
let previousFinished = 0;
for (const runId of [37184573081, 37184614263, 37184617541]) {
  const jobs = JSON.parse(fs.readFileSync(path.join(root, `jobs-${runId}.json`)));
  const hold = jobs.find((job) => job.name === 'hold');
  assert.equal(hold.conclusion, 'success');
  assert.ok(Date.parse(hold.started_at) > previousFinished);
  const finished = jobs.reduce((latest, job) => Math.max(latest, Date.parse(job.completed_at)), 0);
  queue.push({ runId, startedAt: hold.started_at, finishedAt: new Date(finished).toISOString() });
  previousFinished = finished;
}
const summary = { queue, decisions, manifests, production, bootstrapDefaultRejected: true };
fs.writeFileSync(path.join(root, 'verified-summary.json'), `${JSON.stringify(summary, null, 2)}\n`);
console.log(JSON.stringify(summary, null, 2));
