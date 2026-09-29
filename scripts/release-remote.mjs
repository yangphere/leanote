import { ReleaseError } from './release-error.mjs';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';

const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const sha = /^[0-9a-f]{40}$/;
const digestPattern = /^sha256:[0-9a-f]{64}$/;
function check(value, message) { if (!value) throw new ReleaseError(message); }

async function bytes(response, limit) {
  const chunks = []; let total = 0;
  for await (const part of response.body) {
    total += part.length;
    if (total > limit) throw new ReleaseError('remote response exceeds budget');
    chunks.push(part);
  }
  return Buffer.concat(chunks);
}

export function createReleaseRemote({ identity, token, actor, assets, fetchImpl = fetch }) {
  check(typeof token === 'string' && token.length > 0 && typeof actor === 'string' && actor.length > 0, 'release credentials missing');
  check(new RegExp('^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$').test(identity.repository), 'invalid release repository');
  const repo = identity.repository;
  const image = repo.toLowerCase();
  const apiRoot = 'https://api.github.com/repos/' + repo;
  const githubHeaders = { Authorization: 'Bearer ' + token, Accept: 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'leanote-release-verifier' };
  async function request(url, options = {}) {
    try { return await fetchImpl(url, { ...options, signal: AbortSignal.timeout(300000) }); }
    catch { throw new ReleaseError('remote query or write outcome unknown'); }
  }
  async function github(endpoint, { method = 'GET', body, absent = false } = {}) {
    const response = await request(apiRoot + endpoint, { method, headers: { ...githubHeaders, 'Content-Type': 'application/json' }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    if (absent && response.status === 404) {
      const value = JSON.parse(await bytes(response, 65536));
      check(value.message === 'Not Found', 'remote absence response unknown');
      return null;
    }
    check(response.ok, 'GitHub operation failed with status ' + response.status);
    return JSON.parse(await bytes(response, 4 * 1024 * 1024));
  }
  async function repository() {
    const value = await github('');
    check(value.full_name?.toLowerCase() === repo.toLowerCase() && value.permissions?.pull === true && value.permissions?.push === true, 'repository identity or release permission unconfirmed');
    return value;
  }
  async function tagCommit() {
    const value = await github('/git/ref/tags/' + encodeURIComponent(identity.tag));
    let object = value.object;
    for (let depth = 0; depth < 8 && object?.type === 'tag'; depth += 1) {
      check(sha.test(object.sha), 'remote annotated tag invalid');
      object = (await github('/git/tags/' + object.sha)).object;
    }
    check(object?.type === 'commit' && sha.test(object.sha), 'remote tag peel unconfirmed');
    return object.sha;
  }
  async function imageDigest() {
    const tokenResponse = await request('https://ghcr.io/token?service=ghcr.io&scope=' + encodeURIComponent('repository:' + image + ':pull'), { headers: { Authorization: 'Basic ' + Buffer.from(actor + ':' + token).toString('base64') } });
    check(tokenResponse.ok, 'GHCR read authorization unconfirmed');
    const auth = JSON.parse(await bytes(tokenResponse, 65536));
    check(typeof auth.token === 'string' && auth.token.length > 0, 'GHCR authorization token missing');
    const headers = { Authorization: 'Bearer ' + auth.token, Accept: 'application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json' };
    const registry = await request('https://ghcr.io/v2/', { headers });
    check(registry.ok, 'GHCR read permission unconfirmed');
    const response = await request('https://ghcr.io/v2/' + image + '/manifests/' + encodeURIComponent(identity.tag), { headers });
    if (response.status === 404) {
      const value = JSON.parse(await bytes(response, 65536));
      check(Array.isArray(value.errors) && value.errors.length > 0 && value.errors.every((e) => e.code === 'MANIFEST_UNKNOWN'), 'GHCR absence response unknown');
      // A registry-wide login does not prove access to this exact package.
      // Require an authenticated package listing before trusting tag absence.
      const listing = await request('https://ghcr.io/v2/' + image + '/tags/list?n=1', { headers });
      check(listing.ok, 'GHCR target package read permission unconfirmed');
      const packageTags = JSON.parse(await bytes(listing, 65536));
      check(packageTags.name === image && Array.isArray(packageTags.tags) && packageTags.tags.every((tag) => typeof tag === 'string') && !packageTags.tags.includes(identity.tag), 'GHCR target package identity or tag absence unconfirmed');
      return null;
    }
    check(response.ok, 'GHCR image query failed with status ' + response.status);
    const manifestBytes = await bytes(response, 4 * 1024 * 1024);
    const actualDigest = 'sha256:' + hash(manifestBytes);
    check(response.headers.get('docker-content-digest') === actualDigest, 'GHCR digest read-back mismatch');
    const manifest = JSON.parse(manifestBytes);
    check(digestPattern.test(manifest.config?.digest), 'GHCR image config missing');
    const configResponse = await request('https://ghcr.io/v2/' + image + '/blobs/' + manifest.config.digest, { headers });
    check(configResponse.ok, 'GHCR image metadata unavailable');
    const configBytes = await bytes(configResponse, 4 * 1024 * 1024);
    check('sha256:' + hash(configBytes) === manifest.config.digest, 'GHCR config digest mismatch');
    const config = JSON.parse(configBytes);
    check(config.os === 'linux' && config.architecture === 'amd64' && config.config?.Labels?.['org.opencontainers.image.revision'] === identity.commit && config.config?.Labels?.['org.opencontainers.image.version'] === identity.tag.slice(1), 'GHCR image candidate metadata mismatch');
    return actualDigest;
  }
  async function verifyRelease(release) {
    check(Number.isSafeInteger(release.id) && release.id > 0 && release.tag_name === identity.tag && release.draft === false && release.prerelease === false, 'release is not the expected formal release');
    const uploaded = await github('/releases/' + release.id + '/assets?per_page=100');
    check(Array.isArray(uploaded) && uploaded.length === assets.length && new Set(uploaded.map((a) => a.name)).size === assets.length, 'release assets incomplete or conflicting');
    for (const expected of assets) {
      const actual = uploaded.find((a) => a.name === expected.name);
      check(actual && actual.state === 'uploaded' && actual.size === expected.size && Number.isSafeInteger(actual.id) && actual.id > 0, 'release asset identity mismatch');
      const response = await request(apiRoot + '/releases/assets/' + actual.id, { headers: { ...githubHeaders, Accept: 'application/octet-stream' } });
      check(response.ok, 'release asset bytes unavailable');
      const hasher = crypto.createHash('sha256'); let size = 0;
      for await (const chunk of response.body) {
        size += chunk.length; check(size <= expected.size, 'release asset exceeds expected size'); hasher.update(chunk);
      }
      check(size === expected.size && hasher.digest('hex') === expected.sha256, 'release asset byte hash mismatch');
    }
  }
  return {
    github, repository,
    async inspect() {
      await repository();
      return { commit: await tagCommit(), imageDigest: await imageDigest(), release: await github('/releases/tags/' + encodeURIComponent(identity.tag), { absent: true }) };
    },
    verifyRelease,
    async create() {
      const release = await github('/releases', { method: 'POST', body: { tag_name: identity.tag, name: identity.tag, draft: false, prerelease: false, generate_release_notes: true } });
      check(Number.isSafeInteger(release.id) && release.id > 0, 'release creation identity unknown');
      for (const asset of assets) {
        const stat = await fs.lstat(asset.path);
        check(stat.isFile() && !stat.isSymbolicLink() && stat.size === asset.size && path.basename(asset.path) === asset.name, 'original asset changed before upload');
        const stream = createReadStream(asset.path);
        try {
          const response = await request('https://uploads.github.com/repos/' + repo + '/releases/' + release.id + '/assets?name=' + encodeURIComponent(asset.name), { method: 'POST', headers: { ...githubHeaders, 'Content-Type': 'application/octet-stream', 'Content-Length': String(asset.size) }, body: stream, duplex: 'half' });
          check(response.status === 201, 'release asset upload outcome unconfirmed');
          await bytes(response, 65536);
        } finally { stream.destroy(); }
      }
    },
  };
}
