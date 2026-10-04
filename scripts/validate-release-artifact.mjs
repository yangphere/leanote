import { ReleaseError } from './release-error.mjs';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { readProjectVersion, assertReleaseTag } from './version.mjs';

const sha256 = (data) => crypto.createHash('sha256').update(data).digest('hex');
export async function validateReleaseArtifact({ directory = 'dist', candidateRoot = process.cwd(), expected = null } = {}) {
  const context = expected || { tag: process.env.RELEASE_TAG, commit: process.env.GIT_COMMIT || process.env.GITHUB_SHA, workflow: process.env.GITHUB_WORKFLOW, run: { id: process.env.GITHUB_RUN_ID, attempt: process.env.GITHUB_RUN_ATTEMPT }, epoch: process.env.SOURCE_DATE_EPOCH };
  const root = path.resolve(directory);
  const tag = context.tag;
  const version = readProjectVersion(candidateRoot);
  assertReleaseTag(tag, version);
  const commit = context.commit;
  if (!/^[0-9a-f]{40}$/.test(commit || '') || /^0{40}$/.test(commit)) throw new ReleaseError('release artifact commit is invalid');
  const expectedNames = new Set([
    'release-inputs.json', `leanote-v${version}-linux-amd64.tar.gz`,
    `leanote-v${version}-linux-amd64.tar.gz.sha256`, 'build-metadata.json', 'image-build-inputs.json',
  ]);
  const assertKeys = (value, expected, label) => {
    const actual = Object.keys(value).sort();
    const required = [...expected].sort();
    if (actual.length !== required.length || actual.some((key, index) => key !== required[index])) throw new ReleaseError(`${label} schema mismatch`);
  };
  const parseNonNegativeInteger = (value, label) => {
    if (!/^[0-9]+$/.test(value)) throw new ReleaseError(`${label} must be a non-negative integer`);
    const parsed = Number(value);
    if (!Number.isSafeInteger(parsed) || parsed < 0) throw new ReleaseError(`${label} must be a non-negative integer`);
    return parsed;
  };
  const names = (await fs.readdir(root)).sort();
  if (names.length !== expectedNames.size || names.some((name) => !expectedNames.has(name))) throw new ReleaseError('release artifact file allowlist mismatch');
  for (const name of names) {
    const stat = await fs.lstat(path.join(root, name));
    if (!stat.isFile() || stat.isSymbolicLink()) throw new ReleaseError('release artifact must contain only regular files');
  }
  const releaseInputs = JSON.parse(await fs.readFile(path.join(root, 'release-inputs.json'), 'utf8'));
  assertKeys(releaseInputs, new Set(['schema_version', 'artifact_name', 'workflow', 'run', 'ref', 'commit', 'version', 'source_date_epoch', 'platform', 'image_digest', 'base_image_digest', 'provenance', 'attestation', 'sbom', 'files', 'image_build_inputs_sha256']), 'release inputs');
  assertKeys(releaseInputs.run, new Set(['id', 'attempt']), 'release inputs run');
  if (releaseInputs.schema_version !== 'leanote.release-inputs.v1' || releaseInputs.artifact_name !== 'leanote-release-inputs-v1' || releaseInputs.version !== version || releaseInputs.commit !== commit || releaseInputs.platform !== 'linux/amd64') throw new ReleaseError('release inputs identity mismatch');
  if (typeof releaseInputs.workflow !== 'string' || releaseInputs.workflow.length < 1 || releaseInputs.workflow.length > 120 || releaseInputs.workflow === 'unknown' || releaseInputs.ref !== `refs/tags/${tag}` || !/^[1-9][0-9]*$/.test(releaseInputs.run?.id || '') || !Number.isSafeInteger(releaseInputs.run?.attempt) || releaseInputs.run.attempt < 1) throw new ReleaseError('release inputs provenance schema mismatch');
  if (context.run?.id && releaseInputs.run.id !== context.run.id) throw new ReleaseError('release inputs run mismatch');
  if (context.run?.attempt && releaseInputs.run.attempt !== parseNonNegativeInteger(String(context.run.attempt), 'expected attempt')) throw new ReleaseError('release inputs attempt mismatch');
  if (context.workflow && releaseInputs.workflow !== context.workflow) throw new ReleaseError('release inputs workflow mismatch');
  if (!Number.isSafeInteger(releaseInputs.source_date_epoch) || releaseInputs.source_date_epoch < 0) throw new ReleaseError('release inputs source date is invalid');
  let gitEpoch;
  try {
    gitEpoch = parseNonNegativeInteger(execFileSync('git', ['show', '-s', '--format=%ct', commit], { cwd: candidateRoot, encoding: 'utf8' }).trim(), 'git commit timestamp');
  } catch {
    throw new ReleaseError('release checkout timestamp is unavailable');
  }
  if (!Number.isSafeInteger(gitEpoch) || gitEpoch < 0 || releaseInputs.source_date_epoch !== gitEpoch) throw new ReleaseError('release inputs source date mismatch');
  if (context.epoch && parseNonNegativeInteger(String(context.epoch), 'SOURCE_DATE_EPOCH') !== gitEpoch) throw new ReleaseError('SOURCE_DATE_EPOCH does not match tag commit');
  const tarballName = `leanote-v${version}-linux-amd64.tar.gz`;
  const checksumName = `${tarballName}.sha256`;
  const metadata = JSON.parse(await fs.readFile(path.join(root, 'build-metadata.json'), 'utf8'));
  const imageInputs = JSON.parse(await fs.readFile(path.join(root, 'image-build-inputs.json'), 'utf8'));
  assertKeys(metadata, new Set(['schema_version', 'version', 'commit', 'source_date_epoch', 'platform', 'tarball_sha256', 'image_digest']), 'build metadata');
  assertKeys(imageInputs, new Set(['schema_version', 'version', 'commit', 'source_date_epoch', 'platform', 'base_image_digest', 'provenance', 'attestation', 'sbom']), 'image inputs');
  if (metadata.schema_version !== 'leanote.build-metadata.v1') throw new ReleaseError('build metadata schema version mismatch');
  if (imageInputs.schema_version !== 'leanote.image-build-inputs.v1') throw new ReleaseError('image inputs schema version mismatch');
  for (const value of [metadata, imageInputs]) {
    if (value.version !== version || value.commit !== commit || value.source_date_epoch !== releaseInputs.source_date_epoch || value.platform !== 'linux/amd64') throw new ReleaseError('release metadata mismatch');
  }
  if (!/^sha256:[0-9a-f]{64}$/.test(releaseInputs.image_digest) || !/^sha256:[0-9a-f]{64}$/.test(releaseInputs.base_image_digest) || !['enabled', 'disabled'].includes(releaseInputs.provenance) || !['enabled', 'disabled'].includes(releaseInputs.attestation) || !['enabled', 'disabled'].includes(releaseInputs.sbom)) throw new ReleaseError('release image input schema mismatch');
  if (metadata.image_digest !== releaseInputs.image_digest || imageInputs.base_image_digest !== releaseInputs.base_image_digest || imageInputs.provenance !== releaseInputs.provenance || imageInputs.attestation !== releaseInputs.attestation || imageInputs.sbom !== releaseInputs.sbom) throw new ReleaseError('release image input mismatch');
  const tarballHash = sha256(await fs.readFile(path.join(root, tarballName)));
  if (metadata.tarball_sha256 !== tarballHash) throw new ReleaseError('build metadata tarball hash mismatch');
  const checksumBytes = await fs.readFile(path.join(root, checksumName));
  const checksum = checksumBytes.toString('utf8').replace(/\r\n/g, '\n');
  if (checksum !== `${tarballHash}  ${tarballName}\n`) throw new ReleaseError('tarball checksum mismatch');
  if (!Array.isArray(releaseInputs.files) || releaseInputs.files.length !== 4 || releaseInputs.files.some((entry) => !entry || Object.keys(entry).sort().join(',') !== 'kind,path,sha256' || typeof entry.path !== 'string' || entry.path.length < 1 || entry.path.length > 180 || !/^[0-9a-f]{64}$/.test(entry.sha256))) throw new ReleaseError('release manifest entries are invalid');
  const byKind = Object.fromEntries(releaseInputs.files.map((entry) => [entry.kind, entry]));
  if (Object.keys(byKind).sort().join(',') !== 'checksum,image_build_inputs,metadata,tarball' || byKind.tarball.path !== tarballName || byKind.checksum.path !== checksumName || byKind.metadata.path !== 'build-metadata.json' || byKind.image_build_inputs.path !== 'image-build-inputs.json') throw new ReleaseError('release manifest allowlist mismatch');
  if (byKind.tarball.sha256 !== tarballHash || byKind.checksum.sha256 !== sha256(checksumBytes) || byKind.metadata.sha256 !== sha256(await fs.readFile(path.join(root, 'build-metadata.json'))) || byKind.image_build_inputs.sha256 !== sha256(await fs.readFile(path.join(root, 'image-build-inputs.json'))) || releaseInputs.image_build_inputs_sha256 !== byKind.image_build_inputs.sha256) throw new ReleaseError('release manifest hash mismatch');
  return releaseInputs;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await validateReleaseArtifact({ directory: process.argv[2] || 'dist' });
  process.stdout.write('release artifact validated' + String.fromCharCode(10));
}
