import fs from 'node:fs/promises';
import path from 'node:path';
import { sanitizeSummary } from '../../tests/e2e/build/sanitized-summary-reporter.mjs';

const reportDir = path.resolve('test-results');
const summaryPath = path.join(reportDir, 'build-smoke-summary.json');
const healthPath = path.join(reportDir, 'service-health-summary.json');
const [stage = 'ci-unknown', category = 'runner-error', exitCodeValue = ''] = process.argv.slice(2);
const allowedCategories = new Set([
  'revel-cli', 'mongo-fixture', 'service-start', 'service-readiness', 'service-exit', 'cleanup', 'runner-error',
]);
const safeCategory = allowedCategories.has(category) ? category : 'runner-error';

let summary;
try {
  summary = JSON.parse(await fs.readFile(summaryPath, 'utf8'));
} catch {
  summary = {
    tool: { node: process.version, playwright: '1.62.1' },
    stage: 'ci-harness-starting',
    service: { baseUrl: 'http://127.0.0.1:9000', readiness: 'unknown', status: null, exitCode: null },
    auth: { finalUrl: null, authenticated: false },
    pages: [],
    resources: [],
    errors: [],
  };
}
if (stage === 'service-ready') {
  summary = sanitizeSummary({
    ...summary,
    stage: 'service-readiness',
    service: { ...(summary.service ?? {}), readiness: 'ready', status: 200, exitCode: null },
  });
} else if (stage === 'service-stopped') {
  const parsedExitCode = /^-?\d+$/.test(exitCodeValue) ? Number.parseInt(exitCodeValue, 10) : null;
  summary = sanitizeSummary({
    ...summary,
    service: { ...(summary.service ?? {}), readiness: 'stopped', status: null, exitCode: parsedExitCode },
  });
} else {
  summary = sanitizeSummary({
    ...summary,
    stage: stage.startsWith('ci-') ? `${stage}:failed` : 'failed',
    errors: [...(Array.isArray(summary.errors) ? summary.errors : []), `ci:${safeCategory}`],
  });
}
await fs.mkdir(reportDir, { recursive: true });
await fs.writeFile(summaryPath, `${JSON.stringify(summary, null, 2)}\n`, 'utf8');
await fs.writeFile(healthPath, `${JSON.stringify({ tool: summary.tool, stage: summary.stage, service: summary.service }, null, 2)}\n`, 'utf8');
