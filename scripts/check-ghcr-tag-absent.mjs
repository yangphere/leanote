import { pathToFileURL } from 'node:url';
import { assertReleaseTag, readProjectVersion } from './version.mjs';

const MAX_RESPONSE_BYTES = 64 * 1024;
const imagePattern = /^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$/;

function check(value, message) {
  if (!value) throw new Error(message);
}

async function request(fetchImpl, stage, url, options = {}) {
  try {
    return await fetchImpl(url, { ...options, redirect: 'error', signal: AbortSignal.timeout(300000) });
  } catch (error) {
    if (error?.name === 'TimeoutError' || error?.name === 'AbortError') {
      throw new Error(`GHCR ${stage} query timed out`);
    }
    throw new Error(`GHCR ${stage} network failure`);
  }
}

async function json(response, label) {
  const chunks = [];
  let total = 0;
  check(response.body, `GHCR ${label} response body missing`);
  let exceedsBudget = false;
  try {
    for await (const chunk of response.body) {
      total += chunk.length;
      if (total > MAX_RESPONSE_BYTES) {
        exceedsBudget = true;
        break;
      }
      chunks.push(chunk);
    }
  } catch {
    throw new Error(`GHCR ${label} response read failure`);
  }
  check(!exceedsBudget, `GHCR ${label} response exceeds budget`);
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  } catch {
    throw new Error(`GHCR ${label} response has invalid JSON`);
  }
}

function hasErrors(value, code) {
  return Array.isArray(value?.errors)
    && value.errors.length > 0
    && value.errors.every((error) => error?.code === code && typeof error.message === 'string' && error.message.length > 0);
}

function isBoundNameUnknown(value, image) {
  return hasErrors(value, 'NAME_UNKNOWN')
    && value.errors.every((error) => error.detail?.name === undefined || error.detail.name === image);
}

export async function checkGhcrTagAbsent({
  image,
  tag,
  actor,
  token,
  allowInitialPackageCreate = false,
  fetchImpl = fetch,
}) {
  check(imagePattern.test(image), 'invalid GHCR image path');
  assertReleaseTag(tag, readProjectVersion());
  check(typeof actor === 'string' && actor.length > 0 && typeof token === 'string' && token.length > 0, 'GHCR credentials missing');
  check(typeof allowInitialPackageCreate === 'boolean', 'invalid initial package creation policy');

  const scope = encodeURIComponent(`repository:${image}:pull,push`);
  const tokenResponse = await request(fetchImpl, 'authorization', `https://ghcr.io/token?service=ghcr.io&scope=${scope}`, {
    headers: { Authorization: `Basic ${Buffer.from(`${actor}:${token}`).toString('base64')}` },
  });
  check(tokenResponse.ok, `GHCR authorization failed with status ${tokenResponse.status}`);
  const authorization = await json(tokenResponse, 'authorization');
  check(typeof authorization.token === 'string' && authorization.token.length > 0, 'GHCR authorization token missing');

  const headers = {
    Authorization: `Bearer ${authorization.token}`,
    Accept: [
      'application/vnd.oci.image.manifest.v1+json',
      'application/vnd.oci.image.index.v1+json',
      'application/vnd.docker.distribution.manifest.v2+json',
      'application/vnd.docker.distribution.manifest.list.v2+json',
    ].join(', '),
  };
  const registry = await request(fetchImpl, 'registry', 'https://ghcr.io/v2/', { headers });
  check(registry.ok, `GHCR registry access failed with status ${registry.status}`);

  const base = `https://ghcr.io/v2/${image}`;
  const manifestResponse = await request(fetchImpl, 'manifest', `${base}/manifests/${encodeURIComponent(tag)}`, { headers });
  if (manifestResponse.ok) throw new Error(`GHCR tag already exists: ${image}:${tag}`);
  check(manifestResponse.status === 404, `GHCR manifest query failed with status ${manifestResponse.status}`);
  const manifestError = await json(manifestResponse, 'manifest absence');

  const tagsResponse = await request(fetchImpl, 'package listing', `${base}/tags/list?n=100`, { headers });
  if (hasErrors(manifestError, 'MANIFEST_UNKNOWN')) {
    check(tagsResponse.ok, `GHCR package listing failed with status ${tagsResponse.status}`);
    const listing = await json(tagsResponse, 'package listing');
    check(listing.name === image, 'GHCR package identity unconfirmed');
    check(Array.isArray(listing.tags) && listing.tags.every((value) => typeof value === 'string'), 'GHCR package tag listing invalid');
    check(!listing.tags.includes(tag), `GHCR tag already exists: ${image}:${tag}`);
    return { initialPackage: false };
  }

  check(isBoundNameUnknown(manifestError, image), 'GHCR manifest absence response unknown');
  check(tagsResponse.status === 404, `GHCR initial package listing failed with status ${tagsResponse.status}`);
  const listingError = await json(tagsResponse, 'initial package absence');
  check(isBoundNameUnknown(listingError, image), 'GHCR package absence response unknown');
  check(allowInitialPackageCreate, 'GHCR initial package creation is disabled');
  return { initialPackage: true };
}

async function main() {
  const result = await checkGhcrTagAbsent({
    image: process.env.GHCR_IMAGE,
    tag: process.env.RELEASE_TAG,
    actor: process.env.GITHUB_ACTOR,
    token: process.env.GH_TOKEN,
    allowInitialPackageCreate: process.env.ALLOW_INITIAL_PACKAGE_CREATE === 'true',
  });
  process.stdout.write(result.initialPackage
    ? 'GHCR package is absent; explicit first-package creation is authorized for this workflow\n'
    : 'GHCR package exists and the requested version tag is absent\n');
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}
