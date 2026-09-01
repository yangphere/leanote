import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { readProjectVersion, assertReleaseTag } from './version.mjs';

const sha256 = (value) => crypto.createHash('sha256').update(value).digest('hex');

function required(env, name) {
  const value = env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function parseAttempt(value) {
  if (!/^[0-9]+$/.test(value)) throw new Error('GITHUB_RUN_ATTEMPT must be a positive integer');
  const number = Number(value);
  if (!Number.isSafeInteger(number) || number < 1) throw new Error('GITHUB_RUN_ATTEMPT must be a positive integer');
  return number;
}

function buildSwitch(env, name) {
  const value = env[name] || 'disabled';
  if (value !== 'enabled' && value !== 'disabled') throw new Error(`${name} must be enabled or disabled`);
  return value;
}

async function writeJson(file, value) {
  await fs.writeFile(file, `${JSON.stringify(value, null, 2)}\n`);
}

export async function buildReleaseInputs({ root = process.cwd(), outDir = path.join(root, 'dist'), env = process.env } = {}) {
  const version = readProjectVersion(root);
  const tag = required(env, 'RELEASE_TAG');
  assertReleaseTag(tag, version);
  const commit = required(env, 'GIT_COMMIT');
  if (!/^[0-9a-f]{40}$/.test(commit) || /^0{40}$/.test(commit)) throw new Error('GIT_COMMIT must be the tag checkout SHA');
  const ref = env.GITHUB_REF || `refs/tags/${tag}`;
  if (ref !== `refs/tags/${tag}`) throw new Error('GITHUB_REF must be the release tag ref');
  const workflow = required(env, 'GITHUB_WORKFLOW');
  const runId = required(env, 'GITHUB_RUN_ID');
  if (!/^[1-9][0-9]*$/.test(runId)) throw new Error('GITHUB_RUN_ID must be a positive integer');
  const attempt = parseAttempt(required(env, 'GITHUB_RUN_ATTEMPT'));
  const epochRaw = required(env, 'SOURCE_DATE_EPOCH');
  if (!/^[0-9]+$/.test(epochRaw)) throw new Error('SOURCE_DATE_EPOCH must be a non-negative integer');
  const epoch = Number(epochRaw);
  if (!Number.isSafeInteger(epoch) || epoch < 0) throw new Error('SOURCE_DATE_EPOCH must be a non-negative integer');
  const imageDigest = required(env, 'IMAGE_DIGEST');
  if (!/^sha256:[0-9a-f]{64}$/.test(imageDigest)) throw new Error('IMAGE_DIGEST must be the verified local image digest');
  const baseImageDigest = required(env, 'BASE_IMAGE_DIGEST');
  if (!/^sha256:[0-9a-f]{64}$/.test(baseImageDigest)) throw new Error('BASE_IMAGE_DIGEST must be immutable');

  const tarballName = `leanote-v${version}-linux-amd64.tar.gz`;
  const tarballPath = path.join(outDir, tarballName);
  const tarballHash = sha256(await fs.readFile(tarballPath));
  const checksumName = `${tarballName}.sha256`;
  const checksumPath = path.join(outDir, checksumName);
  await fs.writeFile(checksumPath, `${tarballHash}  ${tarballName}\n`);

  const imageInputs = {
    schema_version: 'leanote.image-build-inputs.v1', version, commit,
    source_date_epoch: epoch, platform: 'linux/amd64', base_image_digest: baseImageDigest,
    provenance: buildSwitch(env, 'PROVENANCE'), attestation: buildSwitch(env, 'ATTESTATION'), sbom: buildSwitch(env, 'SBOM'),
  };
  const imageInputsName = 'image-build-inputs.json';
  const imageInputsPath = path.join(outDir, imageInputsName);
  await writeJson(imageInputsPath, imageInputs);
  const imageInputsHash = sha256(await fs.readFile(imageInputsPath));

  const metadata = {
    schema_version: 'leanote.build-metadata.v1', version, commit,
    source_date_epoch: epoch, platform: 'linux/amd64', tarball_sha256: tarballHash, image_digest: imageDigest,
  };
  const metadataName = 'build-metadata.json';
  const metadataPath = path.join(outDir, metadataName);
  await writeJson(metadataPath, metadata);
  const metadataHash = sha256(await fs.readFile(metadataPath));

  const releaseInputs = {
    schema_version: 'leanote.release-inputs.v1',
    artifact_name: 'leanote-release-inputs-v1', workflow,
    run: { id: runId, attempt }, ref, commit, version, source_date_epoch: epoch, platform: 'linux/amd64',
    image_digest: imageDigest, base_image_digest: baseImageDigest,
    provenance: imageInputs.provenance, attestation: imageInputs.attestation, sbom: imageInputs.sbom,
    files: [
      { path: tarballName, kind: 'tarball', sha256: tarballHash },
      { path: checksumName, kind: 'checksum', sha256: sha256(await fs.readFile(checksumPath)) },
      { path: metadataName, kind: 'metadata', sha256: metadataHash },
      { path: imageInputsName, kind: 'image_build_inputs', sha256: imageInputsHash },
    ],
    image_build_inputs_sha256: imageInputsHash,
  };
  await writeJson(path.join(outDir, 'release-inputs.json'), releaseInputs);
  return { version, tag, releaseInputs };
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  buildReleaseInputs().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
