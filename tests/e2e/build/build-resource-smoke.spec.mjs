import { test, expect } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import { MANIFEST } from '../../../scripts/build/manifest.mjs';
import { sanitizeSummary } from './sanitized-summary-reporter.mjs';

const baseUrl = process.env.LEANOTE_BASE_URL;
const email = process.env.LEANOTE_E2E_EMAIL;
const password = process.env.LEANOTE_E2E_PASSWORD;
const pages = ['/note', '/album/index', '/blog', '/admin/index', '/member/index'];
const ownedResourcePaths = new Set([...MANIFEST.js, ...MANIFEST.css, ...MANIFEST.i18n].map((entry) => entry.url));
function sanitizeBaseUrl(value) {
  try { const url = new URL(value); url.username = ''; url.password = ''; url.search = ''; url.hash = ''; return url.href.replace(/\/$/, ''); } catch { return '<invalid>'; }
}
const summary = {
  tool: { node: process.version, playwright: '1.62.1' },
  stage: 'initializing',
  service: { baseUrl: baseUrl ? sanitizeBaseUrl(baseUrl) : '<unset>', readiness: 'unknown', status: null, exitCode: null },
  auth: { finalUrl: null, authenticated: false },
  pages: [], resources: [], errors: [],
};

async function writeSummary() {
  const reportDir = path.resolve('test-results');
  const safe = sanitizeSummary(summary);
  await fs.mkdir(reportDir, { recursive: true });
  await fs.writeFile(path.join(reportDir, 'build-smoke-summary.json'), `${JSON.stringify(safe, null, 2)}\n`, 'utf8');
  await fs.writeFile(path.join(reportDir, 'service-health-summary.json'), `${JSON.stringify({ tool: safe.tool, stage: safe.stage, service: safe.service }, null, 2)}\n`, 'utf8');
}

test.beforeAll(async () => {
  summary.stage = 'prerequisite-check';
  await writeSummary();
  for (const [name, value] of [['LEANOTE_BASE_URL', baseUrl], ['LEANOTE_E2E_EMAIL', email], ['LEANOTE_E2E_PASSWORD', password]]) {
    if (!value) {
      summary.stage = 'prerequisite-check:failed';
      summary.errors.push(`missing-${name}`);
      await writeSummary();
      throw new Error(`${name} is required for build smoke`);
    }
  }
});

test('generated resources and read-only pages are healthy', async ({ page, request }) => {
  const readiness = await request.get(baseUrl).catch(() => null);
  const errors = [];
  try {
    summary.stage = 'service-readiness';
    summary.service.status = readiness?.status() ?? null;
    summary.service.readiness = readiness && readiness.status() < 500 ? 'reachable' : 'unreachable';
    expect(readiness, 'service readiness').not.toBeNull();
    page.on('console', (message) => { if (message.type() === 'error') errors.push('console.error'); });
  page.on('pageerror', () => errors.push('pageerror'));
  page.on('response', (response) => {
    const pathname = new URL(response.url()).pathname;
    if (ownedResourcePaths.has(pathname) && response.status() >= 400) errors.push(`http-${response.status()}`);
  });
  page.on('requestfailed', (requestInfo) => {
    if (ownedResourcePaths.has(new URL(requestInfo.url()).pathname)) errors.push('requestfailed');
  });
  await page.addInitScript(() => {
    window.addEventListener('unhandledrejection', () => { window.__leanoteUnhandledRejection = true; });
  });
  summary.stage = 'authentication';
  await page.goto(new URL('/login', baseUrl).href, { waitUntil: 'domcontentloaded' });
  const emailInput = page.locator('input[type="email"], input[name="email"], input[name="Email"]').first();
  const passwordInput = page.locator('input[type="password"]').first();
  expect(await emailInput.count(), 'login email field').toBeGreaterThan(0);
  expect(await passwordInput.count(), 'login password field').toBeGreaterThan(0);
  await emailInput.fill(email);
  await passwordInput.fill(password);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 15_000 });
    const finalAuthUrl = new URL(page.url());
    summary.auth.finalUrl = `${finalAuthUrl.origin}${finalAuthUrl.pathname}`;
    summary.auth.authenticated = finalAuthUrl.origin === new URL(baseUrl).origin && !/\/login(?:\/|$)/.test(finalAuthUrl.pathname);
    expect(summary.auth.authenticated, 'authenticated final URL').toBe(true);
  summary.stage = 'page-checks';
  for (const route of pages) {
    const response = await page.goto(new URL(route, baseUrl).href, { waitUntil: 'domcontentloaded' });
    const status = response?.status() ?? 0;
    const finalPageUrl = new URL(page.url());
    const authenticatedPage = finalPageUrl.origin === new URL(baseUrl).origin && !/\/login(?:\/|$)/.test(finalPageUrl.pathname);
    summary.pages.push({ url: route, status, finalPath: finalPageUrl.pathname, authenticated: authenticatedPage });
    expect(status, `${route} status`).toBeGreaterThanOrEqual(200);
    expect(status, `${route} status`).toBeLessThan(400);
    expect(authenticatedPage, `${route} authentication`).toBe(true);
    if (route === '/note') {
      const scriptSources = await page.locator('script[src]').evaluateAll((elements) => elements.map((element) => element.getAttribute('src')));
      const requiredOrder = ['/js/dep.min.js', '/tinymce/tinymce.full.min.js', '/js/app.min.js', '/js/markdown-v2.min.js', '/public/js/plugins/main.min.js'];
      let previous = -1;
      for (const expected of requiredOrder) {
        const current = scriptSources.indexOf(expected);
        expect(current, `missing production script ${expected}`).toBeGreaterThan(-1);
        expect(current, `production script order ${expected}`).toBeGreaterThan(previous);
        previous = current;
      }
    }
    if (await page.evaluate(() => Boolean(window.__leanoteUnhandledRejection))) errors.push('unhandledrejection');
  }
  summary.stage = 'resource-checks';
  const outputs = [...MANIFEST.js, ...MANIFEST.css, ...MANIFEST.i18n];
  for (const entry of outputs) {
    const response = await request.get(new URL(entry.url, baseUrl).href, { maxRedirects: 0 });
    const status = response.status();
    summary.resources.push({ path: entry.url, status });
    expect(status, entry.url).toBe(200);
  }
  summary.stage = 'template-check';
  const note = await request.get(new URL('/note', baseUrl).href);
  expect(await note.text()).not.toMatch(/<!-- dev -->|<!-- pro_/);
    expect(errors, 'browser errors').toEqual([]);
    summary.stage = 'complete';
  } catch (error) {
    summary.errors.push(error?.name || 'test-failure');
    throw error;
  } finally {
    summary.errors.push(...errors);
    summary.errors = [...new Set(summary.errors)];
    if (summary.stage !== 'complete' && !summary.stage.endsWith(':failed')) summary.stage = `${summary.stage}:failed`;
    await writeSummary();
  }
});
