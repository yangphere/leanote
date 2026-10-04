const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const test = require('node:test');

const workflowPath = path.join(process.cwd(), '.github/workflows/docker-image.yml');

test('docker image workflow is tag-only, self-serialized, and minimally privileged', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /push:\s*\n\s+tags: \['\[0-9\]\+\.\[0-9\]\+\.\[0-9\]\+'\]/);
  assert.doesNotMatch(workflow, /tags: \['v/);
  assert.doesNotMatch(workflow, /workflow_dispatch:|branches:/);
  assert.match(workflow, /group: docker-image-\$\{\{ github\.ref \}\}/);
  assert.doesNotMatch(workflow, /group: release-\$\{\{ github\.ref \}\}/);
  assert.match(workflow, /permissions:\s*\n\s+contents: read/);
  assert.match(workflow, /publish:[\s\S]*?permissions:\s*\n\s+contents: read\s*\n\s+packages: write/);
  assert.doesNotMatch(workflow, /contents: write/);
  const externalActions = [...workflow.matchAll(/^\s*- uses: ([^\s]+)$/gm)]
    .map((match) => match[1])
    .filter((value) => !value.startsWith('./'));
  assert.ok(externalActions.length > 0);
  assert.ok(externalActions.every((value) => /@[0-9a-f]{40}$/.test(value)));
});

test('docker image workflow validates the canonical version and immutable main tag twice', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /node scripts\/check-version\.mjs --image-tag/);
  assert.match(workflow, /github\.event\.forced/);
  assert.ok((workflow.match(/refs\/remotes\/origin\/tags\/\$\{TAG\}\^\{\}/g) ?? []).length >= 1);
  assert.ok((workflow.match(/merge-base --is-ancestor/g) ?? []).length >= 2);
  assert.ok((workflow.match(/refs\/heads\/main:refs\/remotes\/origin\/main/g) ?? []).length >= 2);
  assert.ok((workflow.match(/refs\/remotes\/origin\/main/g) ?? []).length >= 4);
  assert.doesNotMatch(workflow, /refs\/(?:heads|remotes\/origin)\/master|master ancestry/);
  assert.match(workflow, /uses: \.\/\.github\/workflows\/quality-gate\.yml/);
});

test('ordinary CI follows dev and the default main branch', async () => {
  const workflow = await fs.readFile(path.join(process.cwd(), '.github/workflows/ci.yml'), 'utf8');
  assert.match(workflow, /push:\s*\n\s+branches: \[dev, main\]/);
  assert.doesNotMatch(workflow, /branches: \[[^\]]*master/);
  assert.match(workflow, /uses: \.\/\.github\/workflows\/quality-gate\.yml/);
});

test('image and protected release tags keep distinct strict version contracts', async () => {
  const { assertImageTag, assertReleaseTag } = await import('../../scripts/version.mjs');
  assert.doesNotThrow(() => assertImageTag('2.0.1', '2.0.1'));
  assert.throws(() => assertImageTag('v2.0.1', '2.0.1'), /image tag must match X\.Y\.Z/);
  assert.throws(() => assertImageTag('2.0.2', '2.0.1'), /does not match package version/);
  assert.doesNotThrow(() => assertReleaseTag('v2.0.1', '2.0.1'));
  assert.throws(() => assertReleaseTag('2.0.1', '2.0.1'), /release tag must match vX\.Y\.Z/);
});

test('docker image workflow smokes the exact deterministic candidate before the only push', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /docker\/setup-buildx-action@[0-9a-f]{40}/);
  assert.match(workflow, /docker buildx build --platform linux\/amd64 --load --metadata-file/);
  for (const arg of ['VERSION', 'REVISION', 'SOURCE_DATE_EPOCH', 'OCI_CREATED']) {
    assert.match(workflow, new RegExp(`--build-arg ${arg}=`));
  }
  assert.match(workflow, /--provenance=false --sbom=false/);
  assert.match(workflow, /\['containerimage\.digest'\]/);
  const smoke = workflow.indexOf('scripts/container-smoke.sh "$IMAGE"');
  const absence = workflow.indexOf('node scripts/check-ghcr-tag-absent.mjs');
  const finalSourceCheck = workflow.lastIndexOf('git fetch --force --no-tags origin');
  const pushes = [...workflow.matchAll(/docker push "\$IMAGE"/g)];
  assert.notEqual(smoke, -1);
  assert.ok(absence > smoke);
  assert.ok(finalSourceCheck > absence);
  assert.equal(pushes.length, 1);
  assert.ok(pushes[0].index > finalSourceCheck);
  assert.match(workflow, /imagetools inspect "\$IMAGE" --format '\{\{\.Manifest\.Digest\}\}'/);
  assert.doesNotMatch(workflow, /(?:^|[^A-Za-z])latest(?:[^A-Za-z]|$)/);
});

test('docker image workflow makes first-package creation explicit', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /ALLOW_INITIAL_PACKAGE_CREATE: 'true'/);
  assert.match(workflow, /IMAGE_REPOSITORY: ghcr\.io\/yangphere\/leanote/);
  assert.match(workflow, /GHCR_IMAGE: yangphere\/leanote/);
  assert.match(workflow, /IMAGE=\$\{IMAGE_REPOSITORY\}:\$\{RELEASE_TAG\}/);
  assert.doesNotMatch(workflow, /ghcr\.io\/yangphere\/leanote:\$\{\{ github\.ref_name \}\}/);
  assert.doesNotMatch(workflow, /RELEASE_TAG#v/);
});

const json = (status, value) => new Response(JSON.stringify(value), {
  status,
  headers: { 'content-type': 'application/json' },
});

const nameUnknown = (image) => ({
  errors: [{
    code: 'NAME_UNKNOWN',
    message: 'repository name not known to registry',
    ...(image === undefined ? {} : { detail: { name: image } }),
  }],
});

const manifestUnknown = { errors: [{ code: 'MANIFEST_UNKNOWN', message: 'manifest unknown' }] };

function registryFetch(routes) {
  const calls = [];
  return {
    calls,
    fetchImpl: async (url, requestOptions) => {
      calls.push({ url, requestOptions });
      if (url.startsWith('https://ghcr.io/token?')) return json(200, { token: 'registry-token' });
      if (url === 'https://ghcr.io/v2/') return new Response('', { status: 200 });
      const response = routes.get(url);
      if (!response) throw new Error(`unexpected URL ${url}`);
      return response;
    },
  };
}

const options = (fetchImpl, extra = {}) => ({
  image: 'yangphere/leanote',
  tag: '2.0.1',
  actor: 'release-bot',
  token: 'secret-token',
  allowInitialPackageCreate: false,
  fetchImpl,
  ...extra,
});

test('GHCR absence check accepts a missing tag only after package identity is listed', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const base = 'https://ghcr.io/v2/yangphere/leanote';
  const registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(404, manifestUnknown)],
    [`${base}/tags/list?n=100`, json(200, { name: 'yangphere/leanote', tags: ['v0.9.0'] })],
  ]));
  assert.deepEqual(await checkGhcrTagAbsent(options(registry.fetchImpl)), { initialPackage: false });
  assert.ok(registry.calls.every((call) => call.requestOptions.redirect === 'error'));
  const manifestCall = registry.calls.find((call) => call.url.endsWith('/manifests/2.0.1'));
  assert.ok(manifestCall);
  assert.ok(!registry.calls.some((call) => call.url.endsWith('/manifests/v2.0.1')));
  assert.match(manifestCall.requestOptions.headers.Accept, /application\/vnd\.oci\.image\.index\.v1\+json/);
  assert.match(manifestCall.requestOptions.headers.Accept, /application\/vnd\.docker\.distribution\.manifest\.list\.v2\+json/);
});

test('GHCR absence check permits explicit first-package creation only on two bound NAME_UNKNOWN replies', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const base = 'https://ghcr.io/v2/yangphere/leanote';
  const routes = () => new Map([
    [`${base}/manifests/2.0.1`, json(404, nameUnknown())],
    [`${base}/tags/list?n=100`, json(404, nameUnknown())],
  ]);
  let registry = registryFetch(routes());
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /initial package creation is disabled/);
  registry = registryFetch(routes());
  assert.deepEqual(await checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })), { initialPackage: true });

  registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(404, nameUnknown('another/package'))],
    [`${base}/tags/list?n=100`, json(404, nameUnknown('another/package'))],
  ]));
  await assert.rejects(
    () => checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })),
    /absence response unknown/,
  );
});

test('GHCR absence check blocks existing tags and uncertain remote states', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const base = 'https://ghcr.io/v2/yangphere/leanote';
  let registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(200, { schemaVersion: 2 })],
    [`${base}/tags/list?n=100`, json(200, { name: 'yangphere/leanote', tags: ['2.0.1'] })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /already exists/);

  registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, new Response('{', { status: 404 })],
    [`${base}/tags/list?n=100`, json(200, { name: 'yangphere/leanote', tags: [] })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /invalid JSON/);

  await assert.rejects(
    () => checkGhcrTagAbsent(options(async () => { throw new Error('network down'); })),
    /authorization network failure/,
  );
  await assert.rejects(
    () => checkGhcrTagAbsent(options(async () => { throw Object.assign(new Error('slow'), { name: 'TimeoutError' }); })),
    /authorization query timed out/,
  );
});

test('GHCR absence check rejects permission, rate-limit, listing, identity, and size failures', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const base = 'https://ghcr.io/v2/yangphere/leanote';
  const tokenFailure = async () => json(401, { errors: [{ code: 'UNAUTHORIZED', message: 'denied' }] });
  await assert.rejects(() => checkGhcrTagAbsent(options(tokenFailure)), /authorization failed with status 401/);

  let registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(403, { errors: [{ code: 'DENIED', message: 'denied' }] })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /manifest query failed with status 403/);

  registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(404, manifestUnknown)],
    [`${base}/tags/list?n=100`, json(429, { errors: [{ code: 'TOOMANYREQUESTS', message: 'limited' }] })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /package listing failed with status 429/);

  registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(404, manifestUnknown)],
    [`${base}/tags/list?n=100`, json(200, { name: 'another/package', tags: [] })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /package identity unconfirmed/);

  registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, new Response(JSON.stringify({ errors: [{ code: 'MANIFEST_UNKNOWN', message: 'x'.repeat(70 * 1024) }] }), { status: 404 })],
    [`${base}/tags/list?n=100`, json(200, { name: 'yangphere/leanote', tags: [] })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /response exceeds budget/);
});

test('GHCR first-package path rejects one-sided absence and missing credentials', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const base = 'https://ghcr.io/v2/yangphere/leanote';
  const registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(404, nameUnknown())],
    [`${base}/tags/list?n=100`, json(200, { name: 'yangphere/leanote', tags: [] })],
  ]));
  await assert.rejects(
    () => checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })),
    /initial package listing failed/,
  );
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl, { token: '' })), /credentials missing/);
});

test('GHCR absence check redacts response stream failures with the failing stage', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const brokenBody = {
    async *[Symbol.asyncIterator]() {
      throw new Error('socket details must not escape');
    },
  };
  const response = { ok: true, status: 200, body: brokenBody };
  await assert.rejects(
    () => checkGhcrTagAbsent(options(async () => response)),
    (error) => error.message === 'GHCR authorization response read failure',
  );
});
