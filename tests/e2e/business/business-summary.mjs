import fs from 'node:fs/promises';
import path from 'node:path';
import { sanitizeSummary } from '../build/sanitized-summary-reporter.mjs';

const reportDir = path.resolve('test-results');
const businessPath = path.join(reportDir, 'business-e2e-summary.json');
const healthPath = path.join(reportDir, 'service-health-summary.json');

const initial = () => ({
  tool: { node: process.version, playwright: '1.62.1' },
  stage: 'prerequisite-check',
  service: {
    baseUrl: process.env.LEANOTE_BASE_URL ? String(process.env.LEANOTE_BASE_URL).replace(/\/$/, '') : '<unset>',
    readiness: 'unknown',
    status: null,
    exitCode: null,
  },
  auth: { finalUrl: null, authenticated: false },
  pages: [],
  resources: [],
  errors: [],
});

export async function writeBusinessSummary(summary) {
  const base = initial();
  const safe = sanitizeSummary({
    ...base,
    ...summary,
    service: { ...base.service, ...(summary.service ?? {}) },
  });
  await fs.mkdir(reportDir, { recursive: true });
  await fs.writeFile(businessPath, `${JSON.stringify(safe, null, 2)}\n`, 'utf8');
  await fs.writeFile(healthPath, `${JSON.stringify({ tool: safe.tool, stage: safe.stage, service: safe.service }, null, 2)}\n`, 'utf8');
  return safe;
}

export function classifyPageError(error) {
  if (error?.name === 'TimeoutError') return 'runner:timeout';
  return 'runner:runner-error';
}
