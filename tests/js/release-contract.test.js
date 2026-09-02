const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const path = require('node:path');
const test = require('node:test');

test('release metadata exposes the five-file handoff builder', async () => {
  const { buildReleaseInputs } = await import('../../scripts/release-metadata.mjs');
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-release-contract-'));
  try {
    await fs.writeFile(path.join(root, 'package.json'), JSON.stringify({ version: '1.2.3' }));
    await fs.writeFile(path.join(root, 'package-lock.json'), JSON.stringify({ packages: { '': { version: '1.2.3' } } }));
    const outDir = path.join(root, 'dist');
    await fs.mkdir(outDir);
    await fs.writeFile(path.join(outDir, 'leanote-v1.2.3-linux-amd64.tar.gz'), 'tarball');
    await buildReleaseInputs({ root, outDir, env: {
      RELEASE_TAG: 'v1.2.3', GIT_COMMIT: 'a'.repeat(40), GITHUB_REF: 'refs/tags/v1.2.3',
      GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1', SOURCE_DATE_EPOCH: '100',
      IMAGE_DIGEST: `sha256:${'b'.repeat(64)}`, BASE_IMAGE_DIGEST: `sha256:${'c'.repeat(64)}`,
      PROVENANCE: 'disabled', ATTESTATION: 'disabled', SBOM: 'disabled',
    } });
    const names = (await fs.readdir(outDir)).sort();
    assert.deepEqual(names, ['build-metadata.json', 'image-build-inputs.json', 'leanote-v1.2.3-linux-amd64.tar.gz', 'leanote-v1.2.3-linux-amd64.tar.gz.sha256', 'release-inputs.json']);
    const manifest = JSON.parse(await fs.readFile(path.join(outDir, 'release-inputs.json')));
    assert.equal(manifest.files.length, 4);
    assert.equal(manifest.files.find((entry) => entry.kind === 'tarball').sha256, crypto.createHash('sha256').update('tarball').digest('hex'));
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('release artifact validation rejects unknown metadata schema versions', async () => {
  const { buildReleaseInputs } = await import('../../scripts/release-metadata.mjs');
  const { execFileSync } = require('node:child_process');
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-release-validator-'));
  const commit = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  const epoch = execFileSync('git', ['show', '-s', '--format=%ct', commit], { encoding: 'utf8' }).trim();
  try {
    await fs.writeFile(path.join(root, 'leanote-v1.0.0-linux-amd64.tar.gz'), 'tarball');
    await buildReleaseInputs({ root: process.cwd(), outDir: root, env: {
      RELEASE_TAG: 'v1.0.0', GIT_COMMIT: commit, GITHUB_REF: 'refs/tags/v1.0.0',
      GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1', SOURCE_DATE_EPOCH: epoch,
      IMAGE_DIGEST: `sha256:${'b'.repeat(64)}`, BASE_IMAGE_DIGEST: `sha256:${'c'.repeat(64)}`,
      PROVENANCE: 'disabled', ATTESTATION: 'disabled', SBOM: 'disabled',
    } });
    const metadataPath = path.join(root, 'build-metadata.json');
    const metadata = JSON.parse(await fs.readFile(metadataPath, 'utf8'));
    metadata.schema_version = 'unknown.schema.v9';
    await fs.writeFile(metadataPath, `${JSON.stringify(metadata, null, 2)}\n`);
    const manifestPath = path.join(root, 'release-inputs.json');
    const manifest = JSON.parse(await fs.readFile(manifestPath, 'utf8'));
    const metadataEntry = manifest.files.find((entry) => entry.kind === 'metadata');
    metadataEntry.sha256 = crypto.createHash('sha256').update(await fs.readFile(metadataPath)).digest('hex');
    await fs.writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
    assert.throws(() => execFileSync(process.execPath, ['scripts/validate-release-artifact.mjs', root], {
      // Pin the run provenance to the fixture values so CI-injected
      // GITHUB_RUN_ID/GITHUB_RUN_ATTEMPT cannot trip the replay guard
      // before the schema path under test is reached.
      cwd: process.cwd(), env: { ...process.env, RELEASE_TAG: 'v1.0.0', GIT_COMMIT: commit, GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1' },
      stdio: 'pipe',
    }), /schema version|metadata mismatch/i);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('release metadata rejects a non-numeric source date epoch', async () => {
  const { buildReleaseInputs } = await import('../../scripts/release-metadata.mjs');
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-release-epoch-'));
  try {
    await fs.writeFile(path.join(root, 'package.json'), JSON.stringify({ version: '1.2.3' }));
    await fs.writeFile(path.join(root, 'package-lock.json'), JSON.stringify({ packages: { '': { version: '1.2.3' } } }));
    const outDir = path.join(root, 'dist');
    await fs.mkdir(outDir);
    await fs.writeFile(path.join(outDir, 'leanote-v1.2.3-linux-amd64.tar.gz'), 'tarball');
    await assert.rejects(() => buildReleaseInputs({ root, outDir, env: {
      RELEASE_TAG: 'v1.2.3', GIT_COMMIT: 'a'.repeat(40), GITHUB_REF: 'refs/tags/v1.2.3',
      GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1', SOURCE_DATE_EPOCH: '100garbage',
      IMAGE_DIGEST: `sha256:${'b'.repeat(64)}`, BASE_IMAGE_DIGEST: `sha256:${'c'.repeat(64)}`,
      PROVENANCE: 'disabled', ATTESTATION: 'disabled', SBOM: 'disabled',
    } }), /SOURCE_DATE_EPOCH/);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('release artifact validation binds build metadata to the tarball bytes', async () => {
  const { buildReleaseInputs } = await import('../../scripts/release-metadata.mjs');
  const { execFileSync } = require('node:child_process');
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-release-metadata-hash-'));
  const commit = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  const epoch = execFileSync('git', ['show', '-s', '--format=%ct', commit], { encoding: 'utf8' }).trim();
  try {
    await fs.writeFile(path.join(root, 'leanote-v1.0.0-linux-amd64.tar.gz'), 'tarball');
    await buildReleaseInputs({ root: process.cwd(), outDir: root, env: {
      RELEASE_TAG: 'v1.0.0', GIT_COMMIT: commit, GITHUB_REF: 'refs/tags/v1.0.0',
      GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1', SOURCE_DATE_EPOCH: epoch,
      IMAGE_DIGEST: `sha256:${'b'.repeat(64)}`, BASE_IMAGE_DIGEST: `sha256:${'c'.repeat(64)}`,
      PROVENANCE: 'disabled', ATTESTATION: 'disabled', SBOM: 'disabled',
    } });
    const metadataPath = path.join(root, 'build-metadata.json');
    const metadata = JSON.parse(await fs.readFile(metadataPath, 'utf8'));
    metadata.tarball_sha256 = '0'.repeat(64);
    await fs.writeFile(metadataPath, `${JSON.stringify(metadata, null, 2)}\n`);
    const manifestPath = path.join(root, 'release-inputs.json');
    const manifest = JSON.parse(await fs.readFile(manifestPath, 'utf8'));
    manifest.files.find((entry) => entry.kind === 'metadata').sha256 = crypto.createHash('sha256').update(await fs.readFile(metadataPath)).digest('hex');
    await fs.writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
    assert.throws(() => execFileSync(process.execPath, ['scripts/validate-release-artifact.mjs', root], {
      // Same fixture-pinned provenance as the schema rejection test above.
      cwd: process.cwd(), env: { ...process.env, RELEASE_TAG: 'v1.0.0', GIT_COMMIT: commit, GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1' }, stdio: 'pipe',
    }), /build metadata tarball hash mismatch/);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

const coverageIds = ['business-flows', 'editor-flows', 'bootstrap-components', 'leaui-image-iframe'];

function coverageSummaryFixture(browser_product, release_slot) {
  const items = coverageIds.map((id, index) => ({
    id,
    discovered_count: 3,
    executed_count: index === 3 ? 1 : 3,
    entrypoints: ['note'],
    iframes: id === 'leaui-image-iframe' ? ['tinymce/plugins/leaui_image/index.html'] : [],
    result: 'passed',
  }));
  // Digest is filled by the caller via jcsSha256 once imported.
  return { browser_product, release_slot, items };
}

async function buildBrowserArtifactFixture(commit) {
  const { jcsSha256 } = await import('../../scripts/jcs.mjs');
  const summaries = ['chrome', 'edge', 'firefox', 'safari'].flatMap((browser_product) => ['current_major', 'previous_major'].map((release_slot) => {
    const summary = coverageSummaryFixture(browser_product, release_slot);
    summary.coverage_summary_sha256 = jcsSha256({ browser_product, release_slot, items: summary.items });
    return summary;
  }));
  const digestFor = (browser_product, release_slot) => summaries
    .find((summary) => summary.browser_product === browser_product && summary.release_slot === release_slot).coverage_summary_sha256;
  const records = ['chrome', 'edge', 'firefox', 'safari'].flatMap((browser_product) => ['current_major', 'previous_major'].map((release_slot) => ({
    commit, browser_product, release_slot, browser_version: release_slot === 'current_major' ? '123.4.5' : '122.4.5', os: 'linux', environment: 'real-browser',
    coverage: [...coverageIds], coverage_summary_sha256: digestFor(browser_product, release_slot),
    auth_gate: 'passed', error_gate: 'passed', resource_gate: 'passed',
    executed_at: '2026-08-31T12:00:00Z', result: 'passed',
  })));
  return {
    matrix: { schema_version: 'leanote.browser-smoke.release-matrix.v1', commit, records },
    coverage_summaries: summaries,
  };
}

test('browser release evidence requires the canonical eight-record matrix', async () => {
  const { validateBrowserMatrix, crossValidateBrowserEvidence } = await import('../../scripts/browser-release-evidence.mjs');
  const commit = 'd'.repeat(40);
  const { matrix, coverage_summaries: summaries } = await buildBrowserArtifactFixture(commit);
  const provenance = {
    schema_version: 'leanote.browser-smoke.release-matrix-provenance.v1',
    matrix_sha256: 'a'.repeat(64), commit, ref: 'refs/tags/v1.2.3',
    producer_workflow: 'Protected browser release evidence',
    release_run: { id: '12', attempt: 1 },
    coverage_summaries: summaries,
  };
  const records = matrix.records;
  assert.equal(validateBrowserMatrix(matrix, commit).records.length, 8);
  assert.doesNotThrow(() => crossValidateBrowserEvidence(matrix, provenance));
  assert.throws(() => validateBrowserMatrix({ ...matrix, records: records.slice(1) }, commit), /exactly eight/);
  const nonAdjacent = records.map((row) => row.browser_product === 'chrome' && row.release_slot === 'previous_major'
    ? { ...row, browser_version: '121.4.5' }
    : row);
  assert.throws(() => validateBrowserMatrix({ ...matrix, records: nonAdjacent }, commit), /not adjacent/);
  const invalidDate = records.map((row, index) => index === 0 ? { ...row, executed_at: '2026-02-30T12:00:00Z' } : row);
  assert.throws(() => validateBrowserMatrix({ ...matrix, records: invalidDate }, commit), /RFC3339|executed_at/);
  // The legacy generic scope and any coverage reordering are rejected.
  const legacyScope = records.map((row, index) => index === 0 ? { ...row, coverage: ['build-smoke'] } : row);
  assert.throws(() => validateBrowserMatrix({ ...matrix, records: legacyScope }, commit), /four stable coverage ids/);
  const reordered = records.map((row, index) => index === 0 ? { ...row, coverage: [...coverageIds].reverse() } : row);
  assert.throws(() => validateBrowserMatrix({ ...matrix, records: reordered }, commit), /four stable coverage ids/);
});

test('coverage summaries enforce slot uniqueness, item rules and JCS digests', async () => {
  const { crossValidateBrowserEvidence } = await import('../../scripts/browser-release-evidence.mjs');
  const commit = 'e'.repeat(40);
  const { matrix, coverage_summaries: bound } = await buildBrowserArtifactFixture(commit);
  const baseProvenance = (summaries) => ({
    schema_version: 'leanote.browser-smoke.release-matrix-provenance.v1',
    matrix_sha256: 'a'.repeat(64), commit, ref: 'refs/tags/v1.2.3',
    producer_workflow: 'Protected browser release evidence',
    release_run: { id: '12', attempt: 1 },
    coverage_summaries: summaries,
  });

  // The fixture's summaries are pre-bound to the matrix rows by digest.
  assert.doesNotThrow(() => crossValidateBrowserEvidence(matrix, baseProvenance(bound)));

  const missingSlot = bound.slice(0, 7);
  assert.throws(() => crossValidateBrowserEvidence(matrix, baseProvenance(missingSlot)), /eight slots/);
  const duplicateSlot = [...bound, bound[0]];
  assert.throws(() => crossValidateBrowserEvidence(matrix, baseProvenance(duplicateSlot)), /eight slots|duplicate/);
  const reorderedItems = bound.map((summary, index) => index === 0
    ? { ...summary, items: [...summary.items].reverse() }
    : summary);
  assert.throws(() => crossValidateBrowserEvidence(matrix, baseProvenance(reorderedItems)), /fixed order/);
  const emptyEntrypoints = bound.map((summary, index) => index === 0
    ? { ...summary, items: summary.items.map((item) => ({ ...item, entrypoints: [] })) }
    : summary);
  assert.throws(() => crossValidateBrowserEvidence(matrix, baseProvenance(emptyEntrypoints)), /entrypoints/);
  const executedBeyondDiscovered = bound.map((summary, index) => index === 0
    ? { ...summary, items: summary.items.map((item) => ({ ...item, executed_count: item.discovered_count + 1 })) }
    : summary);
  assert.throws(() => crossValidateBrowserEvidence(matrix, baseProvenance(executedBeyondDiscovered)), /counts are invalid/);
  // A tampered digest (not matching the recomputed JCS value) is rejected.
  const tamperedDigest = bound.map((summary, index) => index === 0 ? { ...summary, coverage_summary_sha256: 'b'.repeat(64) } : summary);
  assert.throws(() => crossValidateBrowserEvidence(matrix, baseProvenance(tamperedDigest)), /digest mismatch/);
  // The legacy six-field provenance (no coverage_summaries) is rejected.
  const { coverage_summaries, ...legacyProvenance } = baseProvenance(bound);
  assert.ok(legacyProvenance);
  assert.throws(() => crossValidateBrowserEvidence(matrix, legacyProvenance), /unknown or missing fields/);
});

test('container smoke grants the canonical config group and verifies PDF output', async () => {
  const script = await fs.readFile(path.join(process.cwd(), 'scripts/container-smoke.sh'), 'utf8');
  assert.match(script, /chmod 0777 "\$FILES_DIR" "\$UPLOAD_DIR"/);
  assert.match(script, /--user 10001:10001 --group-add "\$\(id -g\)"/);
  assert.doesNotMatch(script, /entrypoint \/bin\/chown|MSYS_NO_PATHCONV=1/);
  assert.match(script, /%PDF-/);
});

test('package smoke verifies reproducible archive output', async () => {
  const script = await fs.readFile(path.join(process.cwd(), 'scripts/package-smoke.sh'), 'utf8');
  assert.match(script, /sha256sum/);
  assert.match(script, /SOURCE_DATE_EPOCH/);
  assert.match(script, /PACKAGE_SMOKE_PDF_URL/);
  assert.match(script, /real \/note\/toPdf route/);
  assert.match(script, /curl[\s\S]*PDF/);
  assert.doesNotMatch(script, /wkhtmltopdf --quiet about:blank/);
});

test('container smoke renders the application PDF route instead of a blank page', async () => {
  const script = await fs.readFile(path.join(process.cwd(), 'scripts/container-smoke.sh'), 'utf8');
  assert.match(script, /CONTAINER_SMOKE_PDF_URL/);
  assert.match(script, /real \/note\/toPdf route/);
  assert.match(script, /curl[\s\S]*PDF/);
  assert.doesNotMatch(script, /wkhtmltopdf --quiet about:blank/);
});

test('quality gate fallback summaries preserve GitHub provenance', async () => {
  const workflow = await fs.readFile(path.join(process.cwd(), '.github/workflows/quality-gate.yml'), 'utf8');
  assert.doesNotMatch(workflow, /GITHUB_WORKFLOW:-unknown/);
  assert.doesNotMatch(workflow, /GITHUB_REF:-unknown/);
  assert.doesNotMatch(workflow, /GITHUB_SHA:-0{40}/);
  assert.match(workflow, /CI_FORCE_FALLBACK/);
});

test('smoke scripts carry the executable bit for direct workflow invocation', async () => {
  const { execFileSync } = require('node:child_process');
  const modes = execFileSync('git', ['ls-files', '-s', 'scripts/package-smoke.sh', 'scripts/container-smoke.sh'], { encoding: 'utf8' })
    .split('\n').filter(Boolean).map((line) => line.split(' ')[0]);
  assert.equal(modes.length, 2);
  assert.ok(modes.every((mode) => mode === '100755'), `smoke scripts must be 100755, got ${modes.join(',')}`);
});

test('package tag assertion only applies to real tag contexts', async () => {
  const script = await fs.readFile(path.join(process.cwd(), 'sh/package.sh'), 'utf8');
  // The tag must come from an explicit RELEASE_TAG or a refs/tags/* GITHUB_REF;
  // branch pushes also set GITHUB_REF_NAME (e.g. "dev") and must never be
  // treated as release tags.
  assert.match(script, /case "\$\{GITHUB_REF:-\}" in/);
  assert.match(script, /refs\/tags\/\*\) TAG=/);
  assert.doesNotMatch(script, /RELEASE_TAG:-\$\{GITHUB_REF_NAME/);
});

test('container builds pass an integer epoch and a separate RFC3339 OCI label', async () => {
  const gate = await fs.readFile(path.join(process.cwd(), '.github/workflows/quality-gate.yml'), 'utf8');
  const release = await fs.readFile(path.join(process.cwd(), '.github/workflows/release.yml'), 'utf8');
  const dockerfile = await fs.readFile(path.join(process.cwd(), 'Dockerfile'), 'utf8');
  for (const [name, workflow] of [['quality-gate', gate], ['release', release]]) {
    assert.match(workflow, /--build-arg SOURCE_DATE_EPOCH="\$epoch" --build-arg OCI_CREATED="\$created"/, `${name} build args`);
    assert.doesNotMatch(workflow, /--build-arg SOURCE_DATE_EPOCH="\$created"/, `${name} must not feed RFC3339 into the epoch arg`);
  }
  assert.match(dockerfile, /ARG OCI_CREATED=/);
  assert.match(dockerfile, /org\.opencontainers\.image\.created="\$OCI_CREATED"/);
  assert.doesNotMatch(dockerfile, /org\.opencontainers\.image\.created="\$SOURCE_DATE_EPOCH"/);
});

test('summary writer records failed jobs with a complete lifecycle stage', async () => {
  const { execFile } = require('node:child_process');
  const script = path.join(process.cwd(), 'scripts/ci/write-summary.mjs');
  const run = (cwd, env) => new Promise((resolve) => {
    execFile(process.execPath, [script], { cwd, env }, (error, stdout, stderr) => resolve({ error, stdout, stderr }));
  });
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-summary-stage-'));
  const base = {
    CI_JOB_ID: 'node-build', CI_JOB_STATUS: 'failure',
    GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '123', GITHUB_RUN_ATTEMPT: '1',
    GITHUB_SHA: 'a'.repeat(40), GITHUB_REF: 'refs/heads/dev',
  };
  try {
    // A job that ran and failed reaches this writer: its lifecycle stage is
    // complete even though the job failed; failure details live in the block.
    const failed = await run(root, { ...process.env, ...base });
    assert.equal(failed.error, null, failed.stderr);
    const failedSummary = JSON.parse(await fs.readFile(path.join(root, 'ci-summaries/node-build.json'), 'utf8'));
    assert.equal(failedSummary.status, 'failed');
    assert.equal(failedSummary.stage, 'complete');

    // The writer's own fallback path is the only legitimate job_not_started.
    const fallback = await run(root, { ...process.env, ...base, CI_FORCE_FALLBACK: 'true' });
    assert.equal(fallback.error, null, fallback.stderr);
    const fallbackSummary = JSON.parse(await fs.readFile(path.join(root, 'ci-summaries/node-build.json'), 'utf8'));
    assert.equal(fallbackSummary.stage, 'job_not_started');

    // An explicit CI_STAGE still wins.
    const staged = await run(root, { ...process.env, ...base, CI_STAGE: 'package-built' });
    assert.equal(staged.error, null, staged.stderr);
    const stagedSummary = JSON.parse(await fs.readFile(path.join(root, 'ci-summaries/node-build.json'), 'utf8'));
    assert.equal(stagedSummary.stage, 'package-built');
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('summary writer rejects missing provenance and preserves valid fallback context', async () => {
  const { execFile } = require('node:child_process');
  const script = path.join(process.cwd(), 'scripts/ci/write-summary.mjs');
  const run = (cwd, env) => new Promise((resolve) => {
    execFile(process.execPath, [script], { cwd, env }, (error, stdout, stderr) => resolve({ error, stdout, stderr }));
  });
  const missingEnv = { ...process.env, CI_JOB_ID: 'go-1_26_7', CI_FORCE_FALLBACK: 'true' };
  delete missingEnv.GITHUB_WORKFLOW;
  delete missingEnv.GITHUB_RUN_ID;
  delete missingEnv.GITHUB_RUN_ATTEMPT;
  delete missingEnv.GITHUB_SHA;
  delete missingEnv.GITHUB_REF;
  const missing = await run(process.cwd(), missingEnv);
  assert.notEqual(missing.error, null);
  assert.match(missing.stderr, /GITHUB_WORKFLOW|GITHUB_RUN_ID|GITHUB_SHA|GITHUB_REF/);

  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-summary-provenance-'));
  try {
    const validEnv = {
      ...process.env,
      CI_JOB_ID: 'go-1_26_7', CI_FORCE_FALLBACK: 'true', CI_JOB_STATUS: 'failure',
      GITHUB_WORKFLOW: 'Quality gate', GITHUB_RUN_ID: '123', GITHUB_RUN_ATTEMPT: '2',
      GITHUB_SHA: 'a'.repeat(40), GITHUB_REF: 'refs/heads/dev',
    };
    const valid = await run(root, validEnv);
    assert.equal(valid.error, null, valid.stderr);
    const summary = JSON.parse(await fs.readFile(path.join(root, 'ci-summaries/go-1_26_7.json'), 'utf8'));
    assert.equal(summary.workflow, 'Quality gate');
    assert.deepEqual(summary.run, { id: '123', attempt: 2 });
    assert.equal(summary.commit, 'a'.repeat(40));
    assert.equal(summary.ref, 'refs/heads/dev');
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('quality gate pins Jammy PDF tooling and installs mongorestore explicitly', async () => {
  const workflow = await fs.readFile(path.join(process.cwd(), '.github/workflows/quality-gate.yml'), 'utf8');
  assert.match(workflow, /wkhtmltopdf=0\.12\.6-2(?:\s|$)/);
  assert.doesNotMatch(workflow, /wkhtmltopdf=0\.12\.6-2\+b1/);
  assert.match(workflow, /mongodb-database-tools-ubuntu2204-x86_64-\$\{version\}/);
  assert.match(workflow, /\/usr\/local\/bin\/mongorestore/);
});

test('release validation peels annotated tags and rechecks the remote ref', async () => {
  const workflow = await fs.readFile(path.join(process.cwd(), '.github/workflows/release.yml'), 'utf8');
  assert.match(workflow, /refs\/tags\/\$\{GITHUB_REF_NAME\}\^\{\}/);
  assert.match(workflow, /git fetch --force origin "refs\/tags\/\$\{TAG\}:refs\/remotes\/origin\/tags\/\$\{TAG\}"/);
  assert.match(workflow, /refs\/remotes\/origin\/tags\/\$\{GITHUB_REF_NAME\}\^\{\}/);
});

test('protected browser workflow executes commands instead of importing a matrix file', async () => {
  const workflow = await fs.readFile(path.join(process.cwd(), '.github/workflows/browser-release-evidence.yml'), 'utf8');
  assert.doesNotMatch(workflow, /BROWSER_MATRIX_SOURCE/);
  assert.match(workflow, /Execute protected real-browser matrix/);
  const script = await fs.readFile(path.join(process.cwd(), 'scripts/browser-release-evidence.mjs'), 'utf8');
  assert.match(script, /BROWSER_SMOKE_COMMAND_/);
  assert.match(script, /execAsync\(command/);
});

test('browser precheck entry is isolated from any publishing side effects', async () => {
  const workflow = await fs.readFile(path.join(process.cwd(), '.github/workflows/browser-release-evidence.yml'), 'utf8');
  assert.match(workflow, /workflow_dispatch:/);
  // The dispatch input is a strict version tag and the identity is resolved by
  // peeling the tag in-repo, never from caller-controlled strings beyond the
  // validated tag itself.
  assert.match(workflow, /tag:/);
  assert.match(workflow, /\^\{\}/);
  assert.match(workflow, /grep -Eq '\^v\(0\|\[1-9\]\[0-9\]\*\)/);
  // Two-file artifact with bounded retention; read-only permissions; and no
  // publish/release/registry steps anywhere in the workflow.
  assert.match(workflow, /retention-days: 7/);
  assert.match(workflow, /contents: read/);
  assert.doesNotMatch(workflow, /ghcr|create-release|release\/action|publish:/i);
  assert.match(workflow, /test-results\/release-matrix\.json/);
  assert.match(workflow, /test-results\/provenance\.json/);
});

test('runtime image exposes the PDF binary at the application contract path', async () => {
  const dockerfile = await fs.readFile(path.join(process.cwd(), 'Dockerfile'), 'utf8');
  assert.match(dockerfile, /wkhtmltopdf=0\.12\.6-2\+b1/);
  assert.match(dockerfile, /ln -s \/usr\/bin\/wkhtmltopdf \/usr\/local\/bin\/wkhtmltopdf/);
  assert.match(dockerfile, /COPY conf\/routes \/app\/conf\/routes/);
});

test('package layout carries the runtime route table', async () => {
  const script = await fs.readFile(path.join(process.cwd(), 'sh/package.sh'), 'utf8');
  assert.match(script, /conf\/routes/);
});

test('summary validator rejects an unverified service readiness pass', async () => {
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-summary-contract-'));
  const commit = 'e'.repeat(40);
  const jobs = ['go-1_26_7', 'go-1_27_0', 'mongo-8_0', 'node-build', 'chromium-e2e', 'package-smoke', 'container-smoke'];
  const summary = (job, service = { health_path: null, readiness: 'not_run', http_status: null, exit_code: 0 }) => ({
    schema_version: 'leanote.ci.failure-summary.v1', workflow: 'Quality gate', job,
    run: { id: '10', attempt: 1 }, commit, ref: 'refs/heads/dev', status: 'passed', stage: 'complete',
    toolchain: { go: null, node: null, npm: null, mongo: null, playwright: null },
    failure: { category: 'none', message: '', exit_code: 0 }, service,
    tests: { discovery: 'passed', discovered_count: 1, executed_count: 1 }, page_paths: [], resource_paths: [],
    status_codes: service.http_status === null ? [] : [service.http_status], generated_at: '2026-08-31T12:00:00Z',
  });
  try {
    for (const job of jobs) await fs.writeFile(path.join(root, `${job}.json`), JSON.stringify(summary(job)));
    await fs.writeFile(path.join(root, 'package-smoke.json'), JSON.stringify(summary('package-smoke', {
      health_path: '/healthz', readiness: 'unknown', http_status: 200, exit_code: 0,
    })));
    const result = await import('node:child_process').then(({ execFile }) => new Promise((resolve) => {
      execFile(process.execPath, ['scripts/ci/validate-summaries.mjs', root], { cwd: process.cwd() }, (error, stdout, stderr) => resolve({ error, stdout, stderr }));
    }));
    assert.notEqual(result.error, null);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('summary validator rejects placeholder provenance', async () => {
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-summary-placeholder-'));
  const jobs = ['go-1_26_7', 'go-1_27_0', 'mongo-8_0', 'node-build', 'chromium-e2e', 'package-smoke', 'container-smoke'];
  const record = (job, provenance) => ({
    schema_version: 'leanote.ci.failure-summary.v1', workflow: provenance.workflow,
    job, run: { id: provenance.runId, attempt: 1 }, commit: provenance.commit, ref: provenance.ref,
    status: 'failed', stage: 'job_not_started',
    toolchain: { go: null, node: null, npm: null, mongo: null, playwright: null },
    failure: { category: 'job_not_started', message: 'job_not_started', exit_code: null },
    service: { health_path: null, readiness: 'not_run', http_status: null, exit_code: null },
    tests: { discovery: 'not_run', discovered_count: null, executed_count: null },
    page_paths: [], resource_paths: [], status_codes: [], generated_at: '2026-08-31T12:00:00Z',
  });
  try {
    for (const [index, job] of jobs.entries()) {
      await fs.writeFile(path.join(root, `${job}.json`), JSON.stringify(record(job, {
        workflow: index === 0 ? 'unknown' : 'Quality gate', runId: index === 0 ? '0' : '10',
        commit: index === 0 ? '0'.repeat(40) : 'a'.repeat(40), ref: index === 0 ? 'unknown' : 'refs/heads/dev',
      })));
    }
    const result = await import('node:child_process').then(({ execFile }) => new Promise((resolve) => {
      execFile(process.execPath, ['scripts/ci/validate-summaries.mjs', root], { cwd: process.cwd() }, (error, stdout, stderr) => resolve({ error, stdout, stderr }));
    }));
    assert.notEqual(result.error, null);
    assert.match(result.stderr, /workflow invalid|run provenance invalid|commit invalid|ref invalid/);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('browser evidence provenance names the protected producer workflow and carries coverage summaries', async () => {
  const { buildBrowserEvidence } = await import('../../scripts/browser-release-evidence.mjs');
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-browser-contract-'));
  const commit = 'f'.repeat(40);
  const source = path.join(root, 'source.json');
  const summariesPath = path.join(root, 'summaries.json');
  try {
    const { matrix, coverage_summaries } = await buildBrowserArtifactFixture(commit);
    await fs.writeFile(source, JSON.stringify(matrix));
    await fs.writeFile(summariesPath, JSON.stringify(coverage_summaries));
    const { provenance } = await buildBrowserEvidence({ source, summaries: summariesPath, output: path.join(root, 'out'), env: {
      RELEASE_COMMIT: commit, RELEASE_REF: 'refs/tags/v1.2.3', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1', GITHUB_WORKFLOW: 'Release',
    } });
    assert.equal(provenance.producer_workflow, 'Protected browser release evidence');
    assert.equal(provenance.coverage_summaries.length, 8);
    for (const summary of provenance.coverage_summaries) {
      assert.equal(summary.items.length, 4);
    }
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('browser evidence rejects a prefixed run attempt', async () => {
  const { buildBrowserEvidence } = await import('../../scripts/browser-release-evidence.mjs');
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-browser-attempt-'));
  const commit = '1'.repeat(40);
  const source = path.join(root, 'source.json');
  const summariesPath = path.join(root, 'summaries.json');
  try {
    const { matrix, coverage_summaries } = await buildBrowserArtifactFixture(commit);
    await fs.writeFile(source, JSON.stringify(matrix));
    await fs.writeFile(summariesPath, JSON.stringify(coverage_summaries));
    await assert.rejects(() => buildBrowserEvidence({ source, summaries: summariesPath, output: path.join(root, 'out'), env: {
      RELEASE_COMMIT: commit, RELEASE_REF: 'refs/tags/v1.2.3', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1garbage',
    } }), /attempt/);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('browser artifact validator enforces the final and precheck phases', async () => {
  const root = await fs.mkdtemp(path.join(process.cwd(), 'tmp-browser-phase-'));
  const { buildBrowserEvidence } = await import('../../scripts/browser-release-evidence.mjs');
  const commit = '2'.repeat(40);
  const source = path.join(root, 'source.json');
  const summariesPath = path.join(root, 'summaries.json');
  const runValidator = (args, env) => import('node:child_process').then(({ execFile }) => new Promise((resolve) => {
    execFile(process.execPath, ['scripts/validate-browser-artifact.mjs', ...args], { cwd: process.cwd(), env }, (error, stdout, stderr) => resolve({ error, stdout, stderr }));
  }));
  try {
    const { matrix, coverage_summaries } = await buildBrowserArtifactFixture(commit);
    await fs.writeFile(source, JSON.stringify(matrix));
    await fs.writeFile(summariesPath, JSON.stringify(coverage_summaries));
    const output = path.join(root, 'out');
    await buildBrowserEvidence({ source, summaries: summariesPath, output, env: {
      RELEASE_COMMIT: commit, RELEASE_REF: 'refs/tags/v1.2.3', GITHUB_RUN_ID: '999', GITHUB_RUN_ATTEMPT: '1',
    } });
    const baseEnv = { ...process.env, GITHUB_REF: 'refs/tags/v1.2.3', GITHUB_RUN_ID: '999', GITHUB_RUN_ATTEMPT: '1', GIT_COMMIT: commit };

    // Final phase: producer run must equal the validating run.
    const finalOk = await runValidator([output], { ...baseEnv });
    assert.equal(finalOk.error, null, finalOk.stderr);
    const finalMismatch = await runValidator([output], { ...baseEnv, GITHUB_RUN_ID: '1000' });
    assert.notEqual(finalMismatch.error, null);
    assert.match(finalMismatch.stderr, /provenance mismatch/);

    // Precheck phase: identity binds to the candidate commit, not the
    // validating process's run; a foreign run id is therefore acceptable.
    const precheckOk = await runValidator([output, '--phase', 'precheck', '--expected-commit', commit], { ...baseEnv, GITHUB_RUN_ID: '555' });
    assert.equal(precheckOk.error, null, precheckOk.stderr);
    assert.match(precheckOk.stdout, /precheck phase/);
    const precheckWrongCommit = await runValidator([output, '--phase', 'precheck', '--expected-commit', '3'.repeat(40)], baseEnv);
    assert.notEqual(precheckWrongCommit.error, null);
    // --expected-commit is meaningless in the final phase and must be refused.
    const finalWithExpected = await runValidator([output, '--expected-commit', commit], baseEnv);
    assert.notEqual(finalWithExpected.error, null);
    assert.match(finalWithExpected.stderr, /only valid in precheck/);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});
