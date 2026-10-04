const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const test = require('node:test');

test('ordinary publish requires independent delivery evidence and explicit approval', async () => {
  const workflow = await fs.readFile('.github/workflows/release.yml', 'utf8');
  assert.ok(workflow.includes('needs: [validate, quality-gate, browser-evidence, delivery-evidence]'));
  assert.match(workflow, /uses: [.][/].github[/]workflows[/]delivery-evidence.yml/);
  assert.match(workflow, /environment: release/);
  assert.ok(workflow.includes('RELEASE_APPROVED_IDENTITY: ${{ vars.RELEASE_APPROVED_IDENTITY }}'));
  for (const mode of ['preflight', 'create']) assert.ok(workflow.includes('node scripts/release-runner.mjs ' + mode));
  assert.doesNotMatch(workflow, /gh release create|release_view=|inspect_status=/);
  assert.doesNotMatch(workflow, /dist[/]image-metadata.json/);
  assert.match(workflow, /name: leanote-delivery-evidence-v1/);
});

test('recovery shares the tag lock and consumes exact original artifact IDs', async () => {
  const release = await fs.readFile('.github/workflows/release.yml', 'utf8');
  const recovery = await fs.readFile('.github/workflows/release-recovery.yml', 'utf8');
  assert.ok(release.includes('group: release-${{ github.ref }}'));
  assert.ok(recovery.includes('group: release-refs/tags/${{ inputs.tag }}'));
  for (const workflow of [release, recovery]) assert.match(workflow, /cancel-in-progress: false/);
  assert.match(recovery, /environment: release/);
  assert.match(recovery, /packages: read/);
  assert.doesNotMatch(recovery, /docker (?:build|push)|gh release create|git push|release-metadata.mjs/);
  assert.ok(recovery.indexOf('release-runner.mjs source-check') < recovery.indexOf('artifact-ids:'));
  for (const kind of ['inputs', 'browser', 'delivery']) assert.ok(recovery.includes('artifact-ids: ${{ inputs.' + kind + '_artifact }}'));
  assert.ok(recovery.includes('run-id: ${{ inputs.source_run }}'));
  assert.ok(recovery.includes('ref: ${{ inputs.commit }}'));
  assert.match(recovery, /RELEASE_CANDIDATE_ROOT: candidate/);
  assert.match(recovery, /node scripts[/]release-runner.mjs recover/);
});

test('delivery gate executes protected runner and uploads only sanitized evidence', async () => {
  const workflow = await fs.readFile('.github/workflows/delivery-evidence.yml', 'utf8');
  assert.match(workflow, /workflow_call:/);
  assert.match(workflow, /group: protected-delivery-resources/);
  assert.match(workflow, /cancel-in-progress: false/);
  assert.ok(workflow.includes('runs-on: [self-hosted, protected-delivery]'));
  assert.match(workflow, /node scripts[/]ci[/]delivery-evidence.mjs/);
  assert.match(workflow, /delivery-results[/]catalog.json/);
  assert.match(workflow, /delivery-results[/]delivery-evidence.json/);
  assert.match(workflow, /if-no-files-found: error/);
  assert.doesNotMatch(workflow, /continue-on-error: true/);
});

test('release failure keeps source and target identities without raw response text', async () => {
  const { executeReleaseCommand } = await import('../../scripts/release-runner.mjs');
  const env = { GITHUB_REPOSITORY: 'yangphere/leanote', RELEASE_TAG: 'v1.0.0', RECOVERY_COMMIT: 'a'.repeat(40), GITHUB_SHA: 'b'.repeat(40), GITHUB_REF: 'refs/heads/dev', GITHUB_WORKFLOW: 'Recover Release', GITHUB_RUN_ID: '12', GITHUB_RUN_ATTEMPT: '1', SOURCE_RUN_ID: '10', SOURCE_ATTEMPT: '2', SOURCE_INPUTS_ARTIFACT: '1', SOURCE_BROWSER_ARTIFACT: '2', SOURCE_DELIVERY_ARTIFACT: '3' };
  const result = await executeReleaseCommand('recover', env);
  assert.equal(result.status, 'blocked');
  assert.equal(result.failure.stage, 'authorization');
  assert.equal(result.target.commit, env.RECOVERY_COMMIT);
  assert.equal(result.source.run.id, '10');
  assert.equal(result.source.run.attempt, '2');
  assert.equal(result.execution.commit, env.GITHUB_SHA);
  assert.equal(result.write_attempted, false);
  assert.doesNotMatch(JSON.stringify(result), /token|password/i);
});
