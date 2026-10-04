import { pathToFileURL } from 'node:url';
import { qualityJobs } from './ci/quality-contract.mjs';
import { assertImageTagFormat } from './version.mjs';

const shaPattern = /^(?!0{40}$)[0-9a-f]{40}$/;
const numericPattern = /^[1-9][0-9]*$/;
const requiredJobs = Object.freeze([
  'validate',
  ...qualityJobs.map((job) => `quality-gate / ${job}`),
  'quality-gate / summary',
]);

function check(value, message) {
  if (!value) throw new Error(message);
}

async function readJSON(response) {
  check(response.body, 'GitHub response body missing');
  const chunks = [];
  let total = 0;
  try {
    for await (const chunk of response.body) {
      total += chunk.length;
      check(total <= 4 * 1024 * 1024, 'GitHub response exceeds budget');
      chunks.push(chunk);
    }
  } catch (error) {
    if (error?.message === 'GitHub response exceeds budget') throw error;
    throw new Error('GitHub response read failure');
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  } catch {
    throw new Error('GitHub response has invalid JSON');
  }
}

export async function verifyImageSourceRun({ repository, tag, commit, runId, runAttempt, token, fetchImpl = fetch }) {
  check(repository === 'yangphere/leanote', 'source repository identity invalid');
  assertImageTagFormat(tag);
  check(shaPattern.test(commit), 'source candidate commit invalid');
  check(numericPattern.test(runId), 'source run id invalid');
  check(numericPattern.test(runAttempt), 'source run attempt invalid');
  check(typeof token === 'string' && token.length > 0, 'GitHub source verification token missing');

  const attempt = Number(runAttempt);
  check(Number.isSafeInteger(attempt), 'source run attempt invalid');
  const apiRoot = `https://api.github.com/repos/${repository}`;
  const headers = {
    Authorization: `Bearer ${token}`,
    Accept: 'application/vnd.github+json',
    'X-GitHub-Api-Version': '2022-11-28',
    'User-Agent': 'leanote-image-source-verifier',
  };
  const github = async (endpoint) => {
    let response;
    try {
      response = await fetchImpl(`${apiRoot}${endpoint}`, {
        headers,
        redirect: 'error',
        signal: AbortSignal.timeout(300000),
      });
    } catch {
      throw new Error('GitHub source verification network failure');
    }
    check(response.ok, `GitHub source verification failed with status ${response.status}`);
    return readJSON(response);
  };

  const repo = await github('');
  check(Number.isSafeInteger(repo.id) && repo.id > 0 && repo.full_name === repository, 'source repository identity unconfirmed');
  const prefix = `/actions/runs/${runId}/attempts/${runAttempt}`;
  const latestRun = await github(`/actions/runs/${runId}`);
  check(latestRun.run_attempt === attempt, 'source run attempt is no longer the latest artifact-bearing attempt');
  const run = await github(prefix);
  const workflowPath = typeof run.path === 'string' ? run.path.split('@')[0] : '';
  check(
    String(run.id) === runId
      && run.run_attempt === attempt
      && run.event === 'push'
      && run.status === 'completed'
      && run.conclusion === 'failure'
      && run.name === 'Docker image'
      && ['.github/workflows/docker-image.yml', `${repository}/.github/workflows/docker-image.yml`].includes(workflowPath)
      && run.head_sha === commit
      && run.head_branch === tag
      && run.repository?.id === repo.id
      && run.head_repository?.id === repo.id
      && run.repository?.full_name === repository
      && run.head_repository?.full_name === repository
      && Number.isSafeInteger(run.workflow_id)
      && run.workflow_id > 0,
    'source Docker image workflow provenance mismatch',
  );

  const workflow = await github(`/actions/workflows/${run.workflow_id}`);
  check(workflow.id === run.workflow_id && workflow.path === '.github/workflows/docker-image.yml' && workflow.state === 'active', 'source Docker image workflow identity missing or revoked');

  const jobs = [];
  for (let page = 1; page <= 10; page += 1) {
    const value = await github(`${prefix}/jobs?per_page=100&page=${page}`);
    check(Array.isArray(value.jobs) && Number.isSafeInteger(value.total_count) && value.total_count >= 0 && value.total_count <= 1000, 'source job listing invalid');
    jobs.push(...value.jobs);
    if (jobs.length === value.total_count) break;
    check(value.jobs.length > 0 && page < 10 && jobs.length < value.total_count, 'source job listing incomplete');
  }
  for (const name of requiredJobs) {
    const matches = jobs.filter((job) => job.name === name);
    check(matches.length === 1 && matches[0].status === 'completed' && matches[0].conclusion === 'success' && String(matches[0].run_id) === runId && (matches[0].run_attempt === undefined || matches[0].run_attempt === attempt) && matches[0].head_sha === commit, `source required gate failed or missing: ${name}`);
  }
  const publish = jobs.filter((job) => job.name === 'publish');
  check(publish.length === 1 && publish[0].status === 'completed' && publish[0].conclusion === 'failure' && String(publish[0].run_id) === runId && publish[0].head_sha === commit && Array.isArray(publish[0].steps), 'source publish failure identity missing');
  for (const [name, conclusion] of [
    ['Build the immutable candidate', 'success'],
    ['Smoke the exact candidate', 'success'],
    ['Push once and verify registry manifest digest', 'failure'],
  ]) {
    const steps = publish[0].steps.filter((step) => step.name === name);
    check(steps.length === 1 && steps[0].status === 'completed' && steps[0].conclusion === conclusion, `source publish step mismatch: ${name}`);
  }
  return { repository, tag, commit, run: { id: runId, attempt } };
}

async function main() {
  const result = await verifyImageSourceRun({
    repository: process.env.GITHUB_REPOSITORY,
    tag: process.env.RELEASE_TAG,
    commit: process.env.CANDIDATE_SHA,
    runId: process.env.SOURCE_RUN_ID,
    runAttempt: process.env.SOURCE_RUN_ATTEMPT,
    token: process.env.GH_TOKEN,
  });
  process.stdout.write(`verified Docker image source run ${result.run.id} attempt ${result.run.attempt}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}
