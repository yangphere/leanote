import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readProjectVersion, assertReleaseTag } from './version.mjs';

const sha256 = (data) => crypto.createHash('sha256').update(data).digest('hex');
const root = path.resolve(process.argv[2] || 'dist');
const tag = process.env.RELEASE_TAG;
const version = readProjectVersion();
assertReleaseTag(tag, version);
const commit = process.env.GIT_COMMIT || process.env.GITHUB_SHA;
if (!/^[0-9a-f]{40}$/.test(commit || '') || /^0{40}$/.test(commit)) throw new Error('release artifact commit is invalid');
const expectedNames = new Set([
  'release-inputs.json', `leanote-v${version}-linux-amd64.tar.gz`,
  `leanote-v${version}-linux-amd64.tar.gz.sha256`, 'build-metadata.json', 'image-build-inputs.json',
]);
const assertKeys = (value, expected, label) => {
  const actual = Object.keys(value).sort();
  const required = [...expected].sort();
  if (actual.length !== required.length || actual.some((key, index) => key !== required[index])) throw new Error(`${label} schema mismatch`);
};
const parseNonNegativeInteger = (value, label) => {
  if (!/^[0-9]+$/.test(value)) throw new Error(`${label} must be a non-negative integer`);
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 0) throw new Error(`${label} must be a non-negative integer`);
  return parsed;
};
const names = (await fs.readdir(root)).sort();
if (names.length !== expectedNames.size || names.some((name) => !expectedNames.has(name))) throw new Error('release artifact file allowlist mismatch');
const releaseInputs = JSON.parse(await fs.readFile(path.join(root, 'release-inputs.json'), 'utf8'));
assertKeys(releaseInputs, new Set(['schema_version', 'artifact_name', 'workflow', 'run', 'ref', 'commit', 'version', 'source_date_epoch', 'platform', 'image_digest', 'base_image_digest', 'provenance', 'attestation', 'sbom', 'files', 'image_build_inputs_sha256']), 'release inputs');
assertKeys(releaseInputs.run, new Set(['id', 'attempt']), 'release inputs run');
if (releaseInputs.schema_version !== 'leanote.release-inputs.v1' || releaseInputs.artifact_name !== 'leanote-release-inputs-v1' || releaseInputs.version !== version || releaseInputs.commit !== commit || releaseInputs.platform !== 'linux/amd64') throw new Error('release inputs identity mismatch');
if (typeof releaseInputs.workflow !== 'string' || releaseInputs.workflow.length < 1 || releaseInputs.workflow.length > 120 || releaseInputs.workflow === 'unknown' || releaseInputs.ref !== `refs/tags/${tag}` || !/^[1-9][0-9]*$/.test(releaseInputs.run?.id || '') || !Number.isSafeInteger(releaseInputs.run?.attempt) || releaseInputs.run.attempt < 1) throw new Error('release inputs provenance schema mismatch');
if (process.env.GITHUB_RUN_ID && releaseInputs.run.id !== process.env.GITHUB_RUN_ID) throw new Error('release inputs run mismatch');
if (process.env.GITHUB_RUN_ATTEMPT && releaseInputs.run.attempt !== parseNonNegativeInteger(process.env.GITHUB_RUN_ATTEMPT, 'GITHUB_RUN_ATTEMPT')) throw new Error('release inputs attempt mismatch');
if (!Number.isSafeInteger(releaseInputs.source_date_epoch) || releaseInputs.source_date_epoch < 0) throw new Error('release inputs source date is invalid');
let gitEpoch;
try {
  gitEpoch = parseNonNegativeInteger(execFileSync('git', ['show', '-s', '--format=%ct', commit], { cwd: path.dirname(root), encoding: 'utf8' }).trim(), 'git commit timestamp');
} catch {
  throw new Error('release checkout timestamp is unavailable');
}
if (!Number.isSafeInteger(gitEpoch) || gitEpoch < 0 || releaseInputs.source_date_epoch !== gitEpoch) throw new Error('release inputs source date mismatch');
if (process.env.SOURCE_DATE_EPOCH && parseNonNegativeInteger(process.env.SOURCE_DATE_EPOCH, 'SOURCE_DATE_EPOCH') !== gitEpoch) throw new Error('SOURCE_DATE_EPOCH does not match tag commit');
const tarballName = `leanote-v${version}-linux-amd64.tar.gz`;
const checksumName = `${tarballName}.sha256`;
const metadata = JSON.parse(await fs.readFile(path.join(root, 'build-metadata.json'), 'utf8'));
const imageInputs = JSON.parse(await fs.readFile(path.join(root, 'image-build-inputs.json'), 'utf8'));
assertKeys(metadata, new Set(['schema_version', 'version', 'commit', 'source_date_epoch', 'platform', 'tarball_sha256', 'image_digest']), 'build metadata');
assertKeys(imageInputs, new Set(['schema_version', 'version', 'commit', 'source_date_epoch', 'platform', 'base_image_digest', 'provenance', 'attestation', 'sbom']), 'image inputs');
if (metadata.schema_version !== 'leanote.build-metadata.v1') throw new Error('build metadata schema version mismatch');
if (imageInputs.schema_version !== 'leanote.image-build-inputs.v1') throw new Error('image inputs schema version mismatch');
for (const value of [metadata, imageInputs]) {
  if (value.version !== version || value.commit !== commit || value.source_date_epoch !== releaseInputs.source_date_epoch || value.platform !== 'linux/amd64') throw new Error('release metadata mismatch');
}
if (!/^sha256:[0-9a-f]{64}$/.test(releaseInputs.image_digest) || !/^sha256:[0-9a-f]{64}$/.test(releaseInputs.base_image_digest) || !['enabled', 'disabled'].includes(releaseInputs.provenance) || !['enabled', 'disabled'].includes(releaseInputs.attestation) || !['enabled', 'disabled'].includes(releaseInputs.sbom)) throw new Error('release image input schema mismatch');
if (metadata.image_digest !== releaseInputs.image_digest || imageInputs.base_image_digest !== releaseInputs.base_image_digest || imageInputs.provenance !== releaseInputs.provenance || imageInputs.attestation !== releaseInputs.attestation || imageInputs.sbom !== releaseInputs.sbom) throw new Error('release image input mismatch');
const tarballHash = sha256(await fs.readFile(path.join(root, tarballName)));
if (metadata.tarball_sha256 !== tarballHash) throw new Error('build metadata tarball hash mismatch');
const checksum = (await fs.readFile(path.join(root, checksumName), 'utf8')).replace(/\r\n/g, '\n');
if (checksum !== `${tarballHash}  ${tarballName}\n`) throw new Error('tarball checksum mismatch');
if (!Array.isArray(releaseInputs.files) || releaseInputs.files.length !== 4 || releaseInputs.files.some((entry) => !entry || Object.keys(entry).sort().join(',') !== 'kind,path,sha256' || typeof entry.path !== 'string' || entry.path.length < 1 || entry.path.length > 180 || !/^[0-9a-f]{64}$/.test(entry.sha256))) throw new Error('release manifest entries are invalid');
const byKind = Object.fromEntries(releaseInputs.files.map((entry) => [entry.kind, entry]));
if (Object.keys(byKind).sort().join(',') !== 'checksum,image_build_inputs,metadata,tarball' || byKind.tarball.path !== tarballName || byKind.checksum.path !== checksumName || byKind.metadata.path !== 'build-metadata.json' || byKind.image_build_inputs.path !== 'image-build-inputs.json') throw new Error('release manifest allowlist mismatch');
if (byKind.tarball.sha256 !== tarballHash || byKind.checksum.sha256 !== sha256(Buffer.from(checksum)) || byKind.metadata.sha256 !== sha256(await fs.readFile(path.join(root, 'build-metadata.json'))) || byKind.image_build_inputs.sha256 !== sha256(await fs.readFile(path.join(root, 'image-build-inputs.json'))) || releaseInputs.image_build_inputs_sha256 !== byKind.image_build_inputs.sha256) throw new Error('release manifest hash mismatch');
process.stdout.write('release artifact validated\n');
