const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const test = require('node:test');

const workflowPath = path.join(process.cwd(), '.github/workflows/docker-image.yml');

test('docker image workflow keeps numeric pushes and a controlled same-tag recovery serialized', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /push:\s*\n\s+tags: \['\[0-9\]\+\.\[0-9\]\+\.\[0-9\]\+'\]/);
  assert.doesNotMatch(workflow, /tags: \['v/);
  assert.match(workflow, /workflow_dispatch:/);
  for (const input of ['operation', 'tag', 'expected_commit', 'expected_registry_digest', 'expected_config_digest', 'source_run_id', 'source_run_attempt']) {
    assert.match(workflow, new RegExp(`\\n      ${input}:`));
  }
  assert.doesNotMatch(workflow, /branches:/);
  assert.match(workflow, /group: docker-image-latest/);
  assert.doesNotMatch(workflow, /group: release-\$\{\{ github\.ref \}\}/);
  assert.match(workflow, /permissions:\s*\n\s+actions: read\s*\n\s+contents: read/);
  assert.match(workflow, /publish:[\s\S]*?permissions:\s*\n\s+contents: read\s*\n\s+packages: write/);
  assert.doesNotMatch(workflow, /contents: write/);
  const externalActions = [...workflow.matchAll(/^\s*- uses: ([^\s]+)$/gm)]
    .map((match) => match[1])
    .filter((value) => !value.startsWith('./'));
  assert.ok(externalActions.length > 0);
  assert.ok(externalActions.every((value) => /@[0-9a-f]{40}$/.test(value)));
});

test('manual recovery binds main executor, original evidence, and immutable candidate inputs', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /test "\$GITHUB_REF" = refs\/heads\/main/);
  assert.match(workflow, /node scripts\/verify-image-source-run\.mjs/);
  assert.match(workflow, /run-id: \$\{\{ inputs\.source_run_id \}\}/);
  assert.match(workflow, /pattern: ci-summary-\*/);
  assert.match(workflow, /--include-summary --source-execution/);
  assert.match(workflow, /ref: '\$\{\{ needs\.validate\.outputs\.candidate_sha \}\}'.*path: candidate/);
  assert.match(workflow, /RELEASE_TAG: \$\{\{ needs\.validate\.outputs\.tag \}\}/);
  assert.match(workflow, /CANDIDATE_SHA: \$\{\{ needs\.validate\.outputs\.candidate_sha \}\}/);
  assert.match(workflow, /--build-arg REVISION="\$CANDIDATE_SHA"/);
  assert.match(workflow, /node "\$GITHUB_WORKSPACE\/executor\/scripts\/check-ghcr-tag-absent\.mjs"/);
});

test('docker image workflow validates the canonical version and immutable main tag twice', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /node "\$GITHUB_WORKSPACE\/executor\/scripts\/check-version\.mjs" --image-tag/);
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
  const { assertImageTag, assertImageTagFormat, assertReleaseTag } = await import('../../scripts/version.mjs');
  assert.doesNotThrow(() => assertImageTagFormat('2.0.1'));
  assert.throws(() => assertImageTagFormat('v2.0.1'), /image tag must match X\.Y\.Z/);
  assert.doesNotThrow(() => assertImageTag('2.0.1', '2.0.1'));
  assert.throws(() => assertImageTag('v2.0.1', '2.0.1'), /image tag must match X\.Y\.Z/);
  assert.throws(() => assertImageTag('2.0.2', '2.0.1'), /does not match package version/);
  assert.doesNotThrow(() => assertReleaseTag('v2.0.1', '2.0.1'));
  assert.throws(() => assertReleaseTag('2.0.1', '2.0.1'), /release tag must match vX\.Y\.Z/);
});

test('manual recovery verifier binds the latest explicit attempt, candidate gates, and failed publish step', async () => {
  const { verifyImageSourceRun } = await import('../../scripts/verify-image-source-run.mjs');
  const { qualityJobs } = await import('../../scripts/ci/quality-contract.mjs');
  const repository = 'yangphere/leanote';
  const tag = '2.0.1';
  const commit = 'd'.repeat(40);
  const runId = '37173559882';
  const runAttempt = '1';
  const repo = { id: 42, full_name: repository };
  const run = {
    id: Number(runId), run_attempt: 1, event: 'push', status: 'completed', conclusion: 'failure',
    name: 'Docker image', path: `${repository}/.github/workflows/docker-image.yml@refs/tags/${tag}`,
    head_sha: commit, head_branch: tag, workflow_id: 99,
    repository: repo, head_repository: repo,
  };
  const successful = ['validate', ...qualityJobs.map((job) => `quality-gate / ${job}`), 'quality-gate / summary']
    .map((name) => ({ name, status: 'completed', conclusion: 'success', run_id: Number(runId), run_attempt: 1, head_sha: commit }));
  const publish = {
    name: 'publish', status: 'completed', conclusion: 'failure', run_id: Number(runId), run_attempt: 1, head_sha: commit,
    steps: [
      { name: 'Build the immutable candidate', status: 'completed', conclusion: 'success' },
      { name: 'Smoke the exact candidate', status: 'completed', conclusion: 'success' },
      { name: 'Push once and verify registry manifest digest', status: 'completed', conclusion: 'failure' },
    ],
  };
  const route = (url) => {
    const endpoint = url.replace(`https://api.github.com/repos/${repository}`, '');
    if (endpoint === '') return repo;
    if (endpoint === `/actions/runs/${runId}` || endpoint === `/actions/runs/${runId}/attempts/1`) return run;
    if (endpoint === '/actions/workflows/99') return { id: 99, path: '.github/workflows/docker-image.yml', state: 'active' };
    if (endpoint.includes('/jobs?')) return { total_count: successful.length + 1, jobs: [...successful, publish] };
    throw new Error(`unexpected endpoint ${endpoint}`);
  };
  const fetchImpl = async (url) => json(200, route(url));
  const input = { repository, tag, commit, runId, runAttempt, token: 'token', fetchImpl };
  assert.deepEqual(await verifyImageSourceRun(input), { repository, tag, commit, run: { id: runId, attempt: 1 } });

  run.run_attempt = 2;
  await assert.rejects(() => verifyImageSourceRun(input), /latest artifact-bearing attempt/);
  run.run_attempt = 1;
  successful[1].conclusion = 'failure';
  await assert.rejects(() => verifyImageSourceRun(input), /required gate/);
  successful[1].conclusion = 'success';
  publish.steps[1].conclusion = 'failure';
  await assert.rejects(() => verifyImageSourceRun(input), /publish step mismatch/);
});

test('docker image workflow smokes one dual-exported candidate before byte-preserving version and latest publication', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /docker\/setup-buildx-action@[0-9a-f]{40}/);
  assert.match(workflow, /docker buildx build --platform linux\/amd64 --load --output type=oci,dest="\$RUNNER_TEMP\/candidate\.oci\.tar" --metadata-file/);
  for (const arg of ['VERSION', 'REVISION', 'SOURCE_DATE_EPOCH', 'OCI_CREATED']) {
    assert.match(workflow, new RegExp(`--build-arg ${arg}=`));
  }
  assert.match(workflow, /--provenance=false --sbom=false/);
  assert.match(workflow, /\['containerimage\.digest'\]/);
  assert.match(workflow, /\['containerimage\.config\.digest'\]/);
  const smoke = workflow.indexOf('scripts/container-smoke.sh "$IMAGE"');
  const absence = workflow.indexOf('check-ghcr-tag-absent.mjs');
  const finalSourceCheck = workflow.indexOf('git fetch --force --no-tags origin', absence);
  const configBinding = workflow.indexOf('test "$archive_config" = "$local_config"');
  const versionCopy = workflow.indexOf('oci-archive:/work/candidate.oci.tar docker://"$IMAGE"');
  const latestCopy = workflow.indexOf('oci-archive:/work/candidate.oci.tar docker://"$LATEST_IMAGE"');
  assert.notEqual(smoke, -1);
  assert.ok(configBinding < smoke);
  assert.ok(absence > smoke);
  assert.ok(finalSourceCheck > absence);
  assert.ok(versionCopy > finalSourceCheck);
  assert.ok(latestCopy > versionCopy);
  assert.doesNotMatch(workflow, /docker push/);
  assert.equal([...workflow.matchAll(/docker buildx build /g)].length, 1);
  assert.match(workflow, /skopeo copy --preserve-digests/);
  assert.match(workflow, /SKOPEO_IMAGE: quay\.io\/skopeo\/stable@sha256:[0-9a-f]{64}/);
  assert.match(workflow, /--mount "type=bind,source=\$HOME\/\.docker,target=\/root\/\.docker,readonly"/);
  assert.match(workflow, /--dest-authfile \/root\/\.docker\/config\.json/);
  assert.match(workflow, /inspect --authfile \/root\/\.docker\/config\.json --raw/);
  assert.doesNotMatch(workflow, /apt-get install[^\n]*skopeo/);
  assert.match(workflow, /LATEST_IMAGE=\$\{IMAGE_REPOSITORY\}:latest/);
});

test('latest-only recovery verifies and smokes the immutable version digest before promotion', async () => {
  const workflow = await fs.readFile(workflowPath, 'utf8');
  assert.match(workflow, /update_latest/);
  assert.match(workflow, /EXPECTED_REGISTRY_DIGEST/);
  assert.match(workflow, /EXPECTED_CONFIG_DIGEST/);
  assert.match(workflow, /docker pull "\$VERSION_SOURCE"/);
  const updateLatest = workflow.slice(workflow.indexOf('- name: Verify immutable version and update latest'));
  assert.match(updateLatest, /CONTAINER_SMOKE_PDF_URL: http:\/\/127\.0\.0\.1:9000\/note\/toPdf\?noteId=/);
  assert.match(workflow, /scripts\/container-smoke\.sh "\$VERSION_SOURCE"/);
  assert.match(workflow, /skopeo copy --preserve-digests --src-authfile \/root\/\.docker\/config\.json --dest-authfile \/root\/\.docker\/config\.json docker:\/\/"\$VERSION_SOURCE" docker:\/\/"\$LATEST_IMAGE"/);
  assert.doesNotMatch(updateLatest, /docker buildx build|oci-archive:|check-ghcr-tag-absent/);
  assert.equal([...updateLatest.matchAll(/skopeo copy /g)].length, 1);
  const sourceCheck = updateLatest.indexOf('git fetch --force --no-tags origin');
  const latestCopy = updateLatest.indexOf('skopeo copy --preserve-digests');
  assert.ok(sourceCheck >= 0 && latestCopy > sourceCheck);
});

test('image manifest verifier binds raw bytes, media type, and candidate config before publication', async () => {
  const { verifyImageManifest } = await import('../../scripts/verify-image-manifest.mjs');
  const manifest = Buffer.from(JSON.stringify({
    schemaVersion: 2,
    mediaType: 'application/vnd.oci.image.manifest.v1+json',
    config: { digest: `sha256:${'a'.repeat(64)}` },
    layers: [],
  }));
  const { createHash } = await import('node:crypto');
  const manifestDigest = `sha256:${createHash('sha256').update(manifest).digest('hex')}`;
  assert.deepEqual(
    verifyImageManifest({ manifestBytes: manifest, expectedManifestDigest: manifestDigest, expectedConfigDigest: `sha256:${'a'.repeat(64)}` }),
    { manifestDigest, configDigest: `sha256:${'a'.repeat(64)}` },
  );
  assert.throws(
    () => verifyImageManifest({ manifestBytes: manifest, expectedManifestDigest: manifestDigest, expectedConfigDigest: `sha256:${'b'.repeat(64)}` }),
    /config digest mismatch/,
  );
  assert.throws(
    () => verifyImageManifest({ manifestBytes: manifest, expectedManifestDigest: `sha256:${'0'.repeat(64)}`, expectedConfigDigest: `sha256:${'a'.repeat(64)}` }),
    /manifest digest mismatch/,
  );
  const index = Buffer.from(JSON.stringify({
    schemaVersion: 2,
    mediaType: 'application/vnd.oci.image.index.v1+json',
    manifests: [],
  }));
  const indexDigest = `sha256:${createHash('sha256').update(index).digest('hex')}`;
  assert.throws(
    () => verifyImageManifest({ manifestBytes: index, expectedManifestDigest: indexDigest }),
    /supported single-platform schema 2 image manifest/,
  );
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

test('GHCR absence check permits the observed first-package response pair only under explicit policy', async () => {
  const { checkGhcrTagAbsent } = await import('../../scripts/check-ghcr-tag-absent.mjs');
  const base = 'https://ghcr.io/v2/yangphere/leanote';
  const routes = (manifest = manifestUnknown, listing = nameUnknown()) => new Map([
    [`${base}/manifests/2.0.1`, json(404, manifest)],
    [`${base}/tags/list?n=100`, json(404, listing)],
  ]);
  let registry = registryFetch(routes());
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl)), /initial package creation is disabled/);
  registry = registryFetch(routes());
  assert.deepEqual(await checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })), { initialPackage: true });

  registry = registryFetch(routes(
    { errors: [{ code: 'MANIFEST_UNKNOWN', message: 'manifest unknown', detail: { name: 'another/package' } }] },
    nameUnknown(),
  ));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })), /manifest absence response unknown/);

  registry = registryFetch(routes(manifestUnknown, nameUnknown('another/package')));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })), /package absence response unknown/);

  registry = registryFetch(new Map([
    [`${base}/manifests/2.0.1`, json(404, manifestUnknown)],
    [`${base}/tags/list?n=100`, json(404, { message: 'Not Found' })],
  ]));
  await assert.rejects(() => checkGhcrTagAbsent(options(registry.fetchImpl, { allowInitialPackageCreate: true })), /package absence response unknown/);
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
