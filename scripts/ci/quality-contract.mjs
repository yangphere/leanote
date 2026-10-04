export const qualityJobs = Object.freeze([
  'go-1_26_7', 'go-1_27_0', 'mongo-8_0', 'node-build',
  'chromium-e2e', 'package-smoke', 'container-smoke',
]);

export function executionIdentity(env) {
  const patterns = {
    GITHUB_SHA: /^(?!0{40}$)[0-9a-f]{40}$/,
    GITHUB_REF: /^(?!unknown$).{1,255}$/,
    GITHUB_WORKFLOW: /^(?!unknown$).{1,120}$/,
    GITHUB_RUN_ID: /^[1-9][0-9]*$/,
    GITHUB_RUN_ATTEMPT: /^[1-9][0-9]*$/,
  };
  for (const [name, pattern] of Object.entries(patterns)) {
    if (typeof env[name] !== 'string' || !pattern.test(env[name])) throw new Error('trusted execution provenance missing or invalid: ' + name);
  }
  const attempt = Number(env.GITHUB_RUN_ATTEMPT);
  if (!Number.isSafeInteger(attempt)) throw new Error('trusted execution attempt invalid');
  return { commit: env.GITHUB_SHA, ref: env.GITHUB_REF, workflow: env.GITHUB_WORKFLOW, run: { id: env.GITHUB_RUN_ID, attempt } };
}

export function assertExecutionIdentity(record, expected) {
  if (record.commit !== expected.commit || record.ref !== expected.ref || record.workflow !== expected.workflow ||
      record.run.id !== expected.run.id || record.run.attempt !== expected.run.attempt) {
    throw new Error('quality-gate summary does not match trusted execution provenance');
  }
}
