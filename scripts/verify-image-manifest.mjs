import { createHash } from 'node:crypto';
import { readFile, stat } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const digestPattern = /^sha256:[0-9a-f]{64}$/;
const manifestMediaTypes = new Set([
  'application/vnd.oci.image.manifest.v1+json',
  'application/vnd.docker.distribution.manifest.v2+json',
]);
const maxManifestBytes = 1024 * 1024;

function requireDigest(value, name) {
  if (!digestPattern.test(value || '')) {
    throw new Error(`${name} must be a sha256 digest`);
  }
}

export function verifyImageManifest({
  manifestBytes,
  expectedManifestDigest,
  expectedConfigDigest,
}) {
  requireDigest(expectedManifestDigest, 'expected manifest digest');
  if (expectedConfigDigest !== undefined) {
    requireDigest(expectedConfigDigest, 'expected config digest');
  }

  const actualManifestDigest = `sha256:${createHash('sha256').update(manifestBytes).digest('hex')}`;
  if (actualManifestDigest !== expectedManifestDigest) {
    throw new Error(`manifest digest mismatch: expected ${expectedManifestDigest}, got ${actualManifestDigest}`);
  }

  let manifest;
  try {
    manifest = JSON.parse(manifestBytes.toString('utf8'));
  } catch (error) {
    throw new Error(`manifest is not valid JSON: ${error.message}`);
  }
  if (manifest?.schemaVersion !== 2 || !manifestMediaTypes.has(manifest?.mediaType)) {
    throw new Error('manifest must be a supported single-platform schema 2 image manifest');
  }

  const configDigest = manifest?.config?.digest;
  requireDigest(configDigest, 'manifest config digest');
  if (expectedConfigDigest !== undefined && configDigest !== expectedConfigDigest) {
    throw new Error(`config digest mismatch: expected ${expectedConfigDigest}, got ${configDigest}`);
  }

  return { manifestDigest: actualManifestDigest, configDigest };
}

export async function verifyImageManifestFile({
  manifestPath,
  expectedManifestDigest,
  expectedConfigDigest,
}) {
  const info = await stat(manifestPath);
  if (!info.isFile() || info.size === 0 || info.size > maxManifestBytes) {
    throw new Error(`manifest file size must be between 1 and ${maxManifestBytes} bytes`);
  }
  return verifyImageManifest({
    manifestBytes: await readFile(manifestPath),
    expectedManifestDigest,
    expectedConfigDigest,
  });
}

async function main() {
  const result = await verifyImageManifestFile({
    manifestPath: process.env.IMAGE_MANIFEST_PATH,
    expectedManifestDigest: process.env.EXPECTED_MANIFEST_DIGEST,
    expectedConfigDigest: process.env.EXPECTED_CONFIG_DIGEST || undefined,
  });
  process.stdout.write(`${result.configDigest}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(`image manifest verification failed: ${error.message}`);
    process.exitCode = 1;
  });
}
