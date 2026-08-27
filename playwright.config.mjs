import { defineConfig } from '@playwright/test';

export default defineConfig({
  timeout: 30_000,
  fullyParallel: false,
  reporter: [['line'], ['./tests/e2e/build/sanitized-summary-reporter.mjs']],
  use: { headless: true, trace: 'off', video: 'off', screenshot: 'off' },
  projects: [
    { name: 'build-smoke', testDir: 'tests/e2e/build', testMatch: '**/build-resource-smoke.spec.mjs', use: { browserName: 'chromium' } },
    { name: 'business', testDir: 'tests/e2e/business', testMatch: '**/*.spec.{js,mjs}', use: { browserName: 'chromium' } },
  ],
});
