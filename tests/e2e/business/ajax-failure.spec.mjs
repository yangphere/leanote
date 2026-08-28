// Controlled-failure regressions for the direct $.get/$.post call sites
// (R-jQ3 / AC-jQ7): each representative class gets one injected HTTP failure
// and the test asserts the OBSERVABLE failure branch — a visible alert, a
// dialog error message, or the loading overlay being dismissed — never a
// silent completion. These injections are read-only: no fixture data is
// created, so no cleanup is required.
import { test, expect } from '@playwright/test';
import { ensureE2EIdentity, confirmE2EIdentityFresh } from '../e2e-environment.mjs';

function captureDialogs(page) {
  const dialogs = [];
  page.on('dialog', async (dialog) => {
    dialogs.push(dialog.message());
    await dialog.dismiss();
  });
  return dialogs;
}

test('album getAlbums failure surfaces a visible alert', async ({ page }) => {
  const env = await ensureE2EIdentity();
  const dialogs = captureDialogs(page);
  await page.goto(new URL('/login', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
  await page.locator('input[type="password"]').first().fill(env.password);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 30_000 });
  await confirmE2EIdentityFresh();
  await page.route('**/album/getAlbums', (route) => route.fulfill({ status: 500, body: 'boom' }));
  // In production the album document runs in an iframe whose parent provides
  // GlobalConfigs; mirror that contract for the standalone document.
  await page.addInitScript(() => { window.GlobalConfigs = window.GlobalConfigs || { uploadImageSize: 100 }; });
  await page.goto(new URL('/album/index', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await expect.poll(() => dialogs, { timeout: 15_000 }).toContain('error');
});

test('leaui album-copy getImages failure surfaces a visible alert', async ({ page }) => {
  const env = await ensureE2EIdentity();
  const dialogs = captureDialogs(page);
  await confirmE2EIdentityFresh();
  await page.route('**/file/getImages*', (route) => route.fulfill({ status: 500, body: 'boom' }));
  await page.goto(new URL('/tinymce/plugins/leaui_image/index.html', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  // The leaui main.js copy re-renders images on album-select change.
  await page.evaluate(() => window.jQuery('#albumsForList').trigger('change'));
  await expect.poll(() => dialogs, { timeout: 15_000 }).toContain('error');
});

test('note search failure dismisses loading and alerts', async ({ page }) => {
  const env = await ensureE2EIdentity();
  const dialogs = captureDialogs(page);
  await page.goto(new URL('/login', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
  await page.locator('input[type="password"]').first().fill(env.password);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 30_000 });
  await page.goto(new URL('/note', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.locator('#searchNoteInput').waitFor({ state: 'visible', timeout: 30_000 });
  await confirmE2EIdentityFresh();
  await page.route('**/note/searchNote', (route) => route.fulfill({ status: 500, body: 'boom' }));
  await page.locator('#searchNoteInput').fill('anything');
  await page.locator('#searchNoteInput').press('Enter');
  // The .fail branch must hide the loading overlays and alert.
  await expect.poll(() => dialogs, { timeout: 15_000 }).toContain('error!');
  await expect(page.locator('#searchNoteInput')).toBeVisible();
});

test('admin dialog openDialog failure renders an error message in the dialog', async ({ page }) => {
  const env = await ensureE2EIdentity();
  await page.goto(new URL('/login', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
  await page.locator('input[type="password"]').first().fill(env.password);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 30_000 });
  await page.goto(new URL('/admin/index', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await confirmE2EIdentityFresh();
  await page.route('**/nonexistent-dialog-target', (route) => route.fulfill({ status: 500, body: 'boom' }));
  await page.evaluate(() => openDialog({ url: '/nonexistent-dialog-target', title: 'e2e' }));
  // The .fail branch must put the error text into the dialog content.
  await expect.poll(async () => page.evaluate(() => document.querySelector('.aui_content')?.textContent || ''), { timeout: 15_000 }).toContain('error');
});

test('blog ajaxGet wrapper failure alerts instead of completing silently', async ({ page }) => {
  const env = await ensureE2EIdentity();
  const dialogs = captureDialogs(page);
  await page.goto(new URL('/blog', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  // ajaxGet lives in the blog post page script (blogCommonJsUrl), so enter a
  // real post from the fixture blog index.
  const postHref = await page.evaluate(() => {
    const link = document.querySelector('a[href*="/blog/post/"], a[href*="/blog/view/"]');
    return link ? link.getAttribute('href') : null;
  });
  expect(postHref, 'fixture blog must expose a post link').toBeTruthy();
  await page.goto(new URL(postHref, env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  expect(await page.evaluate(() => typeof window.ajaxGet), 'blog post page defines the wrapper').toBe('function');
  await confirmE2EIdentityFresh();
  await page.route('**/e2e-injected-failure', (route) => route.fulfill({ status: 500, body: 'boom' }));
  // Exercise the exact wrapper the blog front-end uses.
  await page.evaluate(() => ajaxGet('/e2e-injected-failure', {}, () => {}));
  await expect.poll(() => dialogs, { timeout: 15_000 }).toContain('error!');
});
