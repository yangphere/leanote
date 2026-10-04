import { ReleaseError } from './release-error.mjs';
import { qualityJobs } from './ci/quality-contract.mjs';

export const requiredReleaseJobs = Object.freeze([
  'validate', ...qualityJobs.map((job) => 'quality-gate / ' + job),
  'quality-gate / summary', 'quality-gate / release-inputs',
  'browser-evidence / browser-evidence', 'delivery-evidence / delivery-evidence',
]);
const artifactNames = { inputs: 'leanote-release-inputs-v1', browser: 'browser-release-matrix-v1', delivery: 'leanote-delivery-evidence-v1' };
function check(value, message) { if (!value) throw new ReleaseError(message); }

export function sourceFromEnvironment(env) {
  const attempt = Number(env.SOURCE_ATTEMPT);
  const source = { run: env.SOURCE_RUN_ID, attempt, artifacts: { inputs: env.SOURCE_INPUTS_ARTIFACT, browser: env.SOURCE_BROWSER_ARTIFACT, delivery: env.SOURCE_DELIVERY_ARTIFACT } };
  check(/^[1-9][0-9]*$/.test(source.run || '') && /^[1-9][0-9]*$/.test(env.SOURCE_ATTEMPT || '') && Number.isSafeInteger(attempt), 'source run/attempt invalid');
  check(Object.values(source.artifacts).every((v) => /^[1-9][0-9]*$/.test(v || '')) && new Set(Object.values(source.artifacts)).size === 3, 'source artifact IDs invalid');
  return source;
}

export async function verifyOriginalExecution({ identity, source, remote }) {
  check(/^[1-9][0-9]*$/.test(source.run || '') && Number.isSafeInteger(source.attempt) && source.attempt > 0, 'source execution identity invalid');
  check(Object.keys(source.artifacts).sort().join(',') === 'browser,delivery,inputs' && Object.values(source.artifacts).every((id) => /^[1-9][0-9]*$/.test(id || '')) && new Set(Object.values(source.artifacts)).size === 3, 'source artifact identity invalid');
  const repo = await remote.repository();
  const prefix = '/actions/runs/' + source.run + '/attempts/' + source.attempt;
  const run = await remote.github(prefix);
  const workflowPath = typeof run.path === 'string' ? run.path.split('@')[0] : '';
  check(String(run.id) === source.run && run.run_attempt === source.attempt && run.status === 'completed' && run.event === 'push' && ['.github/workflows/release.yml', identity.repository + '/.github/workflows/release.yml'].includes(workflowPath) && run.head_sha === identity.commit && run.repository?.id === repo.id && run.head_repository?.id === repo.id && run.repository?.full_name === identity.repository && run.head_repository?.full_name === identity.repository && typeof run.name === 'string' && run.name.length > 0 && Number.isSafeInteger(run.workflow_id) && run.workflow_id > 0, 'source workflow or attempt provenance mismatch');
  const workflow = await remote.github('/actions/workflows/' + run.workflow_id);
  check(workflow.id === run.workflow_id && workflow.path === '.github/workflows/release.yml' && workflow.state === 'active', 'source workflow identity missing or revoked');
  const jobs = [];
  for (let page = 1; page <= 10; page += 1) {
    const value = await remote.github(prefix + '/jobs?per_page=100&page=' + page);
    check(Array.isArray(value.jobs) && Number.isSafeInteger(value.total_count) && value.total_count >= 0 && value.total_count <= 1000, 'source job listing invalid');
    jobs.push(...value.jobs);
    if (jobs.length === value.total_count) break;
    check(value.jobs.length > 0 && page < 10 && jobs.length < value.total_count, 'source job listing incomplete');
  }
  for (const name of requiredReleaseJobs) {
    const matches = jobs.filter((job) => job.name === name);
    // The attempt-specific endpoint is authoritative; the job schema makes
    // run_attempt optional. A present conflicting value is still rejected.
    check(matches.length === 1 && matches[0].status === 'completed' && matches[0].conclusion === 'success' && String(matches[0].run_id) === source.run && (matches[0].run_attempt === undefined || matches[0].run_attempt === source.attempt) && matches[0].head_sha === identity.commit, 'source required gate failed or missing: ' + name);
  }
  const artifacts = [];
  for (const [kind, id] of Object.entries(source.artifacts)) {
    const artifact = await remote.github('/actions/artifacts/' + id);
    check(String(artifact.id) === id && artifact.name === artifactNames[kind] && artifact.expired === false && String(artifact.workflow_run?.id) === source.run && artifact.workflow_run?.head_sha === identity.commit && artifact.workflow_run?.repository_id === repo.id && artifact.workflow_run?.head_repository_id === repo.id, 'source artifact missing, expired or provenance mismatch');
    artifacts.push({ kind, id, name: artifact.name });
  }
  // The original run may be failed because publishing failed after these gates.
  return { repository: identity.repository, commit: identity.commit, ref: 'refs/tags/' + identity.tag, workflow: run.name, run: { id: source.run, attempt: source.attempt }, artifacts };
}
