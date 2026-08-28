import fs from 'node:fs/promises';
import path from 'node:path';

const reportDir = path.resolve('test-results');
const summaryPath = path.join(reportDir, 'build-smoke-summary.json');
const healthPath = path.join(reportDir, 'service-health-summary.json');
const STAGES = new Set([
  'runner-initializing', 'runner-started', 'runner-error', 'ci-harness-starting',
  'ci-revel-cli', 'ci-mongo-fixture', 'ci-service-start', 'ci-service-readiness', 'ci-service-exit', 'ci-cleanup',
  'prerequisite-check', 'service-readiness', 'authentication', 'page-checks',
  'resource-checks', 'template-check', 'complete', 'failed',
]);
const READINESS = new Set(['unknown', 'reachable', 'unreachable', 'ready', 'stopped', 'failed']);
const ERROR_CATEGORIES = new Set([
  'runner:timeout', 'runner:browser-init', 'runner:fixture-init', 'runner:runner-error',
  'runner:console-error', 'runner:page-error', 'runner:unhandled-rejection',
  'runner:request-failed', 'runner:http-error', 'runner:missing-env', 'runner:unknown',
  'runner:identity-preflight', 'runner:identity-fresh-failed', 'runner:cleanup-failed',
  'ci:revel-cli', 'ci:mongo-fixture', 'ci:service-start', 'ci:service-readiness',
  'ci:service-exit', 'ci:cleanup', 'ci:runner-error',
]);

function sanitizeBaseUrl(value) {
  try {
    const url = new URL(value);
    url.username = '';
    url.password = '';
    url.search = '';
    url.hash = '';
    return url.href.replace(/\/$/, '');
  } catch {
    return '<invalid>';
  }
}

function sanitizeUrl(value) {
  if (typeof value !== 'string' || !value) return null;
  try {
    const url = new URL(value, 'http://leanote.invalid');
    return `${url.pathname || '/'}${url.search ? '' : ''}`;
  } catch {
    return null;
  }
}

function normalizeStage(value) {
  if (typeof value !== 'string') return 'failed';
  const base = value.endsWith(':failed') ? value.slice(0, -7) : value;
  if (STAGES.has(value)) return value;
  if (STAGES.has(base)) return `${base}:failed`;
  return 'failed';
}

function normalizeError(value) {
  if (typeof value !== 'string') return 'runner:unknown';
  if (ERROR_CATEGORIES.has(value)) return value;
  if (value === 'console.error') return 'runner:console-error';
  if (value === 'pageerror') return 'runner:page-error';
  if (value === 'unhandledrejection') return 'runner:unhandled-rejection';
  if (value === 'requestfailed') return 'runner:request-failed';
  if (/^http-\d{3}$/.test(value)) return 'runner:http-error';
  if (/^missing-LEANOTE_/.test(value)) return 'runner:missing-env';
  return 'runner:unknown';
}

export function sanitizeSummary(input = {}) {
  const source = input && typeof input === 'object' ? input : {};
  const tool = source.tool && typeof source.tool === 'object' ? source.tool : {};
  const service = source.service && typeof source.service === 'object' ? source.service : {};
  const auth = source.auth && typeof source.auth === 'object' ? source.auth : {};
  const pages = Array.isArray(source.pages) ? source.pages : [];
  const resources = Array.isArray(source.resources) ? source.resources : [];
  const errors = Array.isArray(source.errors) ? source.errors : [];
  const status = (value) => Number.isInteger(value) ? value : null;
  return {
    tool: {
      node: typeof tool.node === 'string' && /^v\d+\.\d+\.\d+$/.test(tool.node) ? tool.node : process.version,
      playwright: '1.62.1',
    },
    stage: normalizeStage(source.stage),
    service: {
      baseUrl: sanitizeBaseUrl(service.baseUrl),
      readiness: READINESS.has(service.readiness) ? service.readiness : 'unknown',
      status: status(service.status),
      exitCode: status(service.exitCode),
    },
    auth: {
      finalUrl: sanitizeUrl(auth.finalUrl),
      authenticated: auth.authenticated === true,
    },
    pages: pages.map((item) => ({ url: sanitizeUrl(item?.url), status: status(item?.status) })).filter((item) => item.url),
    resources: resources.map((item) => ({ path: sanitizeUrl(item?.path), status: status(item?.status) })).filter((item) => item.path),
    errors: [...new Set(errors.map(normalizeError))],
  };
}

function initialSummary() {
  return {
    tool: { node: process.version, playwright: '1.62.1' },
    stage: 'runner-initializing',
    service: {
      baseUrl: process.env.LEANOTE_BASE_URL ? sanitizeBaseUrl(process.env.LEANOTE_BASE_URL) : '<unset>',
      readiness: 'unknown',
      status: null,
      exitCode: null,
    },
    auth: { finalUrl: null, authenticated: false },
    pages: [],
    resources: [],
    errors: [],
  };
}

async function readSummary() {
  try {
    return sanitizeSummary(JSON.parse(await fs.readFile(summaryPath, 'utf8')));
  } catch {
    return sanitizeSummary(initialSummary());
  }
}

async function writeSummary(summary) {
  summary = sanitizeSummary(summary);
  await fs.mkdir(reportDir, { recursive: true });
  await fs.writeFile(summaryPath, `${JSON.stringify(summary, null, 2)}\n`, 'utf8');
  await fs.writeFile(healthPath, `${JSON.stringify({ tool: summary.tool, stage: summary.stage, service: summary.service }, null, 2)}\n`, 'utf8');
}

function errorCategory(error) {
  if (error?.name === 'TimeoutError') return 'timeout';
  if (error?.message && /browser|executable/i.test(error.message)) return 'browser-init';
  if (error?.message && /fixture|worker|beforeAll|beforeEach/i.test(error.message)) return 'fixture-init';
  return 'runner-error';
}

export default class SanitizedSummaryReporter {
  constructor() {
    this.writeQueue = Promise.resolve();
    this.active = false;
  }

  // The reporter is scoped to the build-smoke project so a business-project
  // invocation sharing this config never clobbers the build summary files.
  isActive(suite) {
    return (suite?.suites ?? []).some((projectSuite) => projectSuite.project()?.name === 'build-smoke');
  }

  enqueue(update) {
    this.writeQueue = this.writeQueue.then(async () => {
      const summary = await readSummary();
      await update(summary);
      await writeSummary(summary);
    });
    return this.writeQueue;
  }

  // Playwright invokes reporters with (config, suite). Keep the config
  // parameter explicit so project scoping is evaluated against the suite
  // rather than the config object.
  async onBegin(_config, suite) {
    if (!this.isActive(suite)) return;
    this.active = true;
    await this.enqueue((summary) => {
      const service = summary.service;
      Object.assign(summary, initialSummary(), { stage: 'runner-started' });
      if (service) summary.service = service;
    });
  }

  async onError(error) {
    if (!this.active) return;
    await this.enqueue((summary) => {
      const category = errorCategory(error);
      summary.errors = [...new Set([...(summary.errors ?? []), `runner:${category}`])];
      if (summary.stage === 'runner-initializing' || summary.stage === 'runner-started') summary.stage = 'runner-error';
    });
  }

  async onEnd(result) {
    if (!this.active) return;
    await this.writeQueue;
    await this.enqueue((summary) => {
      if (result?.status === 'passed' && summary.stage === 'complete') summary.stage = 'complete';
      else if (summary.stage !== 'complete' && !summary.stage.endsWith(':failed')) summary.stage = `${summary.stage}:failed`;
    });
    await this.writeQueue;
  }

  async onExit() {
    if (!this.active) return;
    await this.writeQueue;
  }
}
