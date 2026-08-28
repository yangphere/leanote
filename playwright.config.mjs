import { defineConfig } from '@playwright/test';

// The release smoke (R-jQ7) re-runs the full business flows on real browser
// binaries via LEANOTE_SMOKE_BROWSER=chrome|msedge (real vendor channels) or
// firefox (Playwright Firefox). Chromium is the fallback for local sanity.
const smokeBrowser = process.env.LEANOTE_SMOKE_BROWSER || 'chromium';

export default defineConfig({
  timeout: 30_000,
  fullyParallel: false,
  // All suites share the same isolated leanote_test fixture; cross-file
  // parallelism would interleave writes from different tests.
  workers: 1,
  reporter: [['line'], ['./tests/e2e/build/sanitized-summary-reporter.mjs']],
  use: { headless: true, trace: 'off', video: 'off', screenshot: 'off' },
  projects: [
    { name: 'build-smoke', testDir: 'tests/e2e/build', testMatch: '**/build-resource-smoke.spec.mjs', use: { browserName: 'chromium' }, outputDir: 'test-results/build-smoke' },
    { name: 'business', testDir: 'tests/e2e/business', testMatch: '**/*.spec.{js,mjs}', use: { browserName: 'chromium' }, outputDir: 'test-results/business' },
    {
      name: 'browser-smoke',
      testDir: 'tests/e2e/business',
      testMatch: '**/business-flows.spec.mjs',
      outputDir: 'test-results/browser-smoke',
      use: smokeBrowser === 'firefox'
        ? { browserName: 'firefox' }
        : { browserName: 'chromium', channel: smokeBrowser },
    },
  ],
});
