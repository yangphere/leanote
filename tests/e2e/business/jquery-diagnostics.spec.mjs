// Diagnostics E2E (AC-jQ7, R-jQ4): run real pages against jQuery 3.7.1 with
// jquery-migrate 3.6.0 injected immediately after the core in the same
// document. The gate is contract-aligned (PRD R-jQ4 as amended 2026-08-28):
//   1. warnings attributable to first-party scripts must be zero;
//   2. third-party warnings are excused ONLY through the inventory §4.1
//      ownership table, and every registered exclusion category must
//      actually be exercised during the run;
//   3. every console warning must pair one-to-one with a stack-attributed
//      record, and unattributable warnings fail (fail closed).
// Both the dep bundle and the album bundle are rebuilt locally with the
// migrate injection, and every line range maps back to its input file,
// giving file-level attribution even inside minified bundles. The album
// section also performs a real upload so fileupload's add/submit/done
// callbacks run under migrate. The migrate bytes come from the locally
// locked package and are served through Playwright routes only; nothing is
// written back to the repo or the service static tree.
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { MANIFEST } from '../../../scripts/build/manifest.mjs';
import { ensureE2EIdentity, confirmE2EIdentityFresh } from '../e2e-environment.mjs';

const repoRoot = process.cwd();
const readRepo = (relative) => fs.readFileSync(path.join(repoRoot, relative));

const jQueryCore = fs.readFileSync(path.join(repoRoot, 'node_modules/jquery/dist/jquery.min.js'));
const migrate = fs.readFileSync(path.join(repoRoot, 'node_modules/jquery-migrate/dist/jquery-migrate.js'));

function diagnosticCore() {
  // Migrate must execute immediately after the core and before any plugin.
  // Disable only Migrate's message de-duplication in this temporary diagnostic
  // document. The package bytes stay verbatim, while identical warnings from
  // different owners (e.g. fileupload and waitForImages) remain attributable
  // one-by-one instead of the first stack hiding later sources.
  return Buffer.concat([
    jQueryCore,
    Buffer.from('\n'),
    migrate,
    Buffer.from('\nwindow.jQuery.migrateDeduplicateWarnings = false;\n'),
  ]);
}

// Rebuild a bundle input list with migrate injected after its jQuery input
// and record each input's line range inside the served buffer so a stack
// frame can be attributed to its exact source file.
function diagnosticBundle(bundleName) {
  const entry = MANIFEST.js.find((candidate) => candidate.name === bundleName);
  if (!entry) throw new Error(`manifest has no ${bundleName} entry`);
  const parts = [];
  const lineRanges = [];
  let line = 1;
  for (const input of entry.inputs) {
    const body = input === 'node_modules/jquery/dist/jquery.min.js'
      ? diagnosticCore()
      : readRepo(input);
    const lineCount = body.toString('utf8').split('\n').length;
    lineRanges.push({ input, startLine: line, endLine: line + lineCount - 1 });
    parts.push(body, Buffer.from('\n'));
    line += lineCount;
  }
  return { buffer: Buffer.concat(parts), lineRanges };
}

// Input paths inside the diagnostic bundles that are third-party libraries
// with a documented ownership decision (inventory §4.1). `contextmenu` is
// regenerated from its fixed first-party source and therefore not listed.
const DEP_LIB_INPUTS = new Set([
  'public/js/jquery.ztree.all-3.5-min.js',
  'public/js/jQuery-slimScroll-1.3.0/jquery.slimscroll-min.js',
  'public/js/bootstrap-min.js',
]);
const ALBUM_LIB_INPUTS = new Set([
  'public/js/bootstrap-min.js',
]);

// Verbatim upstream npm dist files (byte-sync enforced by
// tests/js/jquery-asset-contract.test.js). R-jQ5 forbids patching upstream
// bytes, so their warnings are recorded instead of fixed here.
const VERBATIM_UPSTREAM_URLS = new Set([
  '/tinymce/plugins/leaui_image/public/js/jquery.ui.widget.js',
  '/tinymce/plugins/leaui_image/public/js/jquery.iframe-transport.js',
  '/tinymce/plugins/leaui_image/public/js/jquery.fileupload.js',
]);
const VERBATIM_UPSTREAM_INPUTS = new Set([
  'node_modules/blueimp-file-upload/js/vendor/jquery.ui.widget.js',
  'node_modules/blueimp-file-upload/js/jquery.iframe-transport.js',
  'node_modules/blueimp-file-upload/js/jquery.fileupload.js',
]);
// Signatures that statically provably originate inside the verbatim upstream
// files concatenated into a bundle that also carries first-party code.
const UPSTREAM_SIGNATURES = [
  'jQuery.isArray is deprecated',
  'jQuery.isFunction() is deprecated',
];
const UPSTREAM_BUNDLE_URLS = new Set([
  '/public/js/plugins/main.min.js',
  '/public/album/js/main.all.js',
]);
// The readable Markdown source embeds waitForImages 1.4.2. The production
// URL is a generated bundle, so diagnostics serve a temporary bundle and map
// its input range back to this exact derived input. Static contracts ensure
// the only jQuery.isFunction call in that input belongs to waitForImages.
const MARKDOWN_WAIT_FOR_IMAGES_INPUT = 'public/md/main-v2.min.js';
const MARKDOWN_WAIT_FOR_IMAGES_SIGNATURE = 'jQuery.isFunction() is deprecated';

// Library assets owned by other tasks (inventory §4.1 ownership table).
const LIB_URL_PATHNAMES = new Set([
  '/tinymce/tinymce.full.min.js', // E-TM owned
  '/tinymce/tinymce.js', // dev core only served on dev pages
  '/js/bootstrap.js', // login page bootstrap 3 (E-BS owned)
  '/tinymce/plugins/leaui_image/public/bootstrap3/js/bootstrap.min.js', // E-BS owned
  '/public/admin/js/artDialog/jquery.artDialog.js', // vendored artDialog (admin 区验收处理)
]);

// Exclusion categories the visited pages are EXPECTED to exercise on every
// run; a missing category fails the diagnostic. `album-lib` stays registered
// in the classification map above (any hit is excused and recorded) but is
// not expected: the album page's load path has not been observed to trigger
// bootstrap-min warnings yet — if it ever does, the hit is still exempted.
const EXPECTED_EXCLUSION_CATEGORIES = new Set([
  'dep-lib', 'verbatim-input', 'verbatim-url', 'lib-url', 'upstream-signature',
  'markdown-waitforimages',
]);

async function collectMigrationEvidence(page) {
  // Evidence persists in sessionStorage: it survives same-tab navigation, so
  // every warning in every document is recorded exactly once and the final
  // tally matches the console stream one-to-one.
  await page.addInitScript(() => {
    const KEY = 'leanote-migrate-evidence';
    const load = () => {
      try { return JSON.parse(sessionStorage.getItem(KEY)) || { warnings: [] }; } catch { return { warnings: [] }; }
    };
    const save = (data) => {
      try { sessionStorage.setItem(KEY, JSON.stringify(data)); } catch {}
    };
    const originalWarn = console.warn;
    console.warn = function (message) {
      try {
        if (typeof message === 'string' && message.indexOf('JQMIGRATE') === 0) {
          const data = load();
          data.warnings.push({ message, stack: String(new Error().stack || '') });
          save(data);
        }
      } catch {}
      return originalWarn.apply(console, arguments);
    };
  });
}

// Parses Chrome named (`at fn (url:line:col)`), Chrome anonymous
// (`at url:line:col`) and Firefox (`fn@url:line:col`) frames. Query strings
// are stripped so bundle lookups match; inline page-script frames (the page
// path itself) are kept and count as first-party.
function frameLocations(stack) {
  const locations = [];
  const push = (rawPathname, line) => {
    const pathname = rawPathname.split('?')[0];
    locations.push({ pathname, line: line ? Number(line) : null });
  };
  for (const match of stack.matchAll(/\((?:https?:\/\/[^)]*?)(\/[^):]+?)(?::(\d+))?(?::\d+)?\)/g)) {
    push(match[1], match[2]);
  }
  for (const match of stack.matchAll(/at (https?:\/\/[^\s(]+):(\d+):(\d+)(?!\))/g)) {
    push(match[1].replace(/^https?:\/\/[^/]+/, ''), match[2]);
  }
  // Firefox frames are `fn@url:line:column`; capture the pathname (not the
  // function name) and allow a host with an optional `:port`.
  for (const match of stack.matchAll(/(?:[^@\s]+)?@https?:\/\/[^/\s]+(\/[^:\s]+):(\d+)(?::\d+)?/g)) {
    push(match[1], match[2]);
  }
  return locations.filter((frame) => frame.pathname.startsWith('/'));
}

// The offender is the topmost stack frame that is not the jQuery core or
// migrate itself. Returns null only when every frame resolved to a core
// asset — the caller treats null as unattributable and fails.
function offenderOf(stack, bundleRanges) {
  for (const frame of frameLocations(stack)) {
    const ranges = bundleRanges.get(frame.pathname);
    if (ranges) {
      const range = ranges.find((candidate) => frame.line >= candidate.startLine && frame.line <= candidate.endLine);
      if (!range) continue;
      if (range.input === 'node_modules/jquery/dist/jquery.min.js') continue; // core + migrate
      return { kind: 'bundle-input', id: range.input };
    }
    if (frame.pathname === '/js/jquery-1.9.0.min.js') continue; // core + migrate
    return { kind: 'url', id: frame.pathname };
  }
  return null;
}

function classify(warnings, bundleRanges) {
  const ownOffenders = new Set();
  const excludedByCategory = new Map();
  const exclude = (category, warning, id) => {
    if (!excludedByCategory.has(category)) excludedByCategory.set(category, []);
    excludedByCategory.get(category).push(`${warning.message} <- ${id}`);
  };
  for (const warning of warnings) {
    const offender = offenderOf(warning.stack, bundleRanges);
    if (!offender) {
      ownOffenders.add(`${warning.message}\n    <- <unattributable> full stack:\n${warning.stack}`);
      continue;
    }
    const id = offender.id;
    if (offender.kind === 'bundle-input' && DEP_LIB_INPUTS.has(id)) {
      exclude('dep-lib', warning, id);
      continue;
    }
    if (offender.kind === 'bundle-input' && ALBUM_LIB_INPUTS.has(id)) {
      exclude('album-lib', warning, id);
      continue;
    }
    if (offender.kind === 'bundle-input' && VERBATIM_UPSTREAM_INPUTS.has(id)) {
      exclude('verbatim-input', warning, id);
      continue;
    }
    if (
      offender.kind === 'bundle-input'
      && id === MARKDOWN_WAIT_FOR_IMAGES_INPUT
      && warning.message.includes(MARKDOWN_WAIT_FOR_IMAGES_SIGNATURE)
    ) {
      exclude('markdown-waitforimages', warning, id);
      continue;
    }
    if (offender.kind === 'url' && LIB_URL_PATHNAMES.has(id)) {
      exclude('lib-url', warning, id);
      continue;
    }
    if (offender.kind === 'url' && VERBATIM_UPSTREAM_URLS.has(id)) {
      exclude('verbatim-url', warning, id);
      continue;
    }
    if (
      offender.kind === 'url' && UPSTREAM_BUNDLE_URLS.has(id)
      && UPSTREAM_SIGNATURES.some((signature) => warning.message.includes(signature))
    ) {
      exclude('upstream-signature', warning, id);
      continue;
    }
    ownOffenders.add(`${warning.message}\n    <- ${id}`);
  }
  return { ownOffenders, excludedByCategory };
}

async function checkedJson(request, url, label) {
  const response = await request.get(url);
  if (response.status() >= 400) throw new Error(`${label}: HTTP ${response.status()}`);
  try {
    return await response.json();
  } catch {
    throw new Error(`${label}: response is not JSON`);
  }
}

test('zero first-party JQMIGRATE warnings with migrate 3.6.0 injected after the core', async ({ page }) => {
  test.setTimeout(300_000);
  const env = await ensureE2EIdentity();
  const request = page.request;

  // Fresh confirmation before any route injection.
  await confirmE2EIdentityFresh();

  const coreBody = diagnosticCore();
  const dep = diagnosticBundle('dep');
  const album = diagnosticBundle('album');
  const markdown = diagnosticBundle('markdown');
  await confirmE2EIdentityFresh();
  await page.route('**/js/jquery-1.9.0.min.js', (route) => route.fulfill({ body: coreBody }));
  await confirmE2EIdentityFresh();
  await page.route('**/js/dep.min.js', (route) => route.fulfill({ body: dep.buffer }));
  await confirmE2EIdentityFresh();
  await page.route('**/album/js/main.all.js?*', (route) => route.fulfill({ body: album.buffer }));
  await confirmE2EIdentityFresh();
  await page.route('**/album/js/main.all.js', (route) => route.fulfill({ body: album.buffer }));
  await confirmE2EIdentityFresh();
  await page.route('**/js/markdown-v2.min.js', (route) => route.fulfill({ body: markdown.buffer }));
  const bundleRanges = new Map([
    ['/js/dep.min.js', dep.lineRanges],
    ['/public/album/js/main.all.js', album.lineRanges],
    ['/js/markdown-v2.min.js', markdown.lineRanges],
  ]);

  const warnings = [];
  page.on('console', (message) => {
    if ((message.type() === 'warning' || message.type() === 'error' || message.type() === 'trace') && /JQMIGRATE/.test(message.text())) {
      warnings.push(message.text());
    }
  });
  await collectMigrationEvidence(page);

  // The migrate bundle must be live in every document under test.
  await page.addInitScript(() => {
    window.__leanoteMigrateCheck = () => Boolean(window.jQuery && window.jQuery.migrateVersion);
  });



  // Login page loads the public runtime URL directly.
  await page.goto(new URL('/login', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  expect(await page.evaluate(() => window.__leanoteMigrateCheck()), 'migrate live on /login').toBe(true);

  await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
  await page.locator('input[type="password"]').first().fill(env.password);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 30_000 });

  // The note page loads jQuery inside dep.min.js; the rebuilt bundle injects
  // migrate right after the core, before ztree/bootstrap/plugins execute.
  // The markdown bundle and the markdown/tinymce editor initialization run
  // in the same document, covering the markdown editor's load path.
  await page.goto(new URL('/note', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  expect(await page.evaluate(() => window.__leanoteMigrateCheck()), 'migrate live on /note').toBe(true);
  await page.locator('#searchNoteInput').waitFor({ state: 'visible', timeout: 30_000 });
  await page.evaluate(() => window.jQuery('#setTheme').trigger('click'));
  await expect(page.locator('#setThemeDialog')).toBeVisible({ timeout: 15_000 });
  await page.locator('#setThemeDialog .close, #setThemeDialog [data-dismiss="modal"]').first().click();

  // Album page: jQuery lives inside main.all.js; the rebuilt album bundle
  // injects migrate before bootstrap/fileupload/pagination/album main.js.
  // Register cleanup before the write and execute it from an outer finally so
  // malformed responses or later assertions cannot leak fixtures.
  const uploadCleanupSteps = [];
  const cleanupFailures = [];
  const albumName = `e2e-diag-album-${Date.now()}`;
  const imageName = `e2e-diag-${Date.now()}.png`;
  let albumId = null;
  let imageFileId = null;
  uploadCleanupSteps.push(['deleteAlbum', async () => {
    let id = albumId;
    if (!id) {
      const albums = await checkedJson(request, new URL('/album/getAlbums', env.baseUrl).href, 'diagnostic getAlbums cleanup');
      const list = Array.isArray(albums) ? albums : (albums?.List ?? albums?.Albums ?? []);
      id = list.find((item) => item?.Name === albumName || item?.Title === albumName)?.AlbumId || null;
    }
    if (!id) return;
    const remove = await checkedJson(request, new URL(`/album/deleteAlbum?albumId=${id}`, env.baseUrl).href, 'diagnostic deleteAlbum');
    if (remove?.Ok !== true) throw new Error('diagnostic deleteAlbum: Ok was not true');
    const after = await checkedJson(request, new URL('/album/getAlbums', env.baseUrl).href, 'diagnostic getAlbums after delete');
    expect(JSON.stringify(after), 'diagnostic album must not remain listed').not.toContain(id);
  }]);
  let primaryError = null;
  try {
    await confirmE2EIdentityFresh();
    const addAlbum = await request.get(new URL(`/album/addAlbum?name=${encodeURIComponent(albumName)}`, env.baseUrl).href);
    expect(addAlbum.status(), 'diagnostic addAlbum transport status').toBeLessThan(400);
    albumId = (await addAlbum.json())?.AlbumId;
    expect(albumId, 'diagnostic album id').toBeTruthy();
    await page.addInitScript(() => {
      window.GlobalConfigs = window.GlobalConfigs || { uploadImageSize: 100 };
    });
    const albumsDone = page.waitForResponse((response) => new URL(response.url()).pathname === '/album/getAlbums', { timeout: 30_000 });
    await page.goto(new URL('/album/index', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
    expect(await page.evaluate(() => window.__leanoteMigrateCheck()), 'migrate live on /album/index').toBe(true);
    const albumsPayload = await (await albumsDone).json();
    expect(JSON.stringify(albumsPayload), 'getAlbums must return the diagnostic album').toContain(albumId);
    await expect(page.locator(`#albumsForList option[value="${albumId}"]`)).toBeAttached({ timeout: 30_000 });
    await page.locator('#albumsForList').selectOption(albumId);
    await page.locator('#goAddImageBtn').click();
    await expect(page.locator('#albumsForUpload')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('#albumsForUpload')).toHaveValue(albumId, { timeout: 15_000 });
    uploadCleanupSteps.push(['deleteImage', async () => {
      let id = imageFileId;
      if (!id) {
        const images = await checkedJson(request, new URL(`/file/getImages?albumId=${albumId}&page=1`, env.baseUrl).href, 'diagnostic getImages cleanup');
        const entry = (images?.List ?? []).find((item) => item?.Title === imageName || item?.Name === imageName);
        id = entry?.FileId || null;
      }
      if (!id) return;
      const remove = await checkedJson(request, new URL(`/file/deleteImage?fileId=${id}`, env.baseUrl).href, 'diagnostic deleteImage');
      if (remove?.Ok !== true) throw new Error('diagnostic deleteImage: Ok was not true');
      const after = await checkedJson(request, new URL(`/file/getImages?albumId=${albumId}&page=1`, env.baseUrl).href, 'diagnostic getImages after delete');
      expect(JSON.stringify(after), 'diagnostic image must not remain listed').not.toContain(id);
    }]);
    await confirmE2EIdentityFresh();
    const imageDone = page.waitForResponse((response) => new URL(response.url()).pathname === '/file/uploadImageLeaui', { timeout: 60_000 });
    await page.locator('#upload input[name="file"]').first().setInputFiles({
      name: imageName,
      mimeType: 'image/png',
      buffer: Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    });
    const imageResponse = await imageDone;
    expect(imageResponse.status(), 'diagnostic image upload status').toBe(200);
    const imageBody = await imageResponse.json();
    expect(imageBody.Ok, 'diagnostic image upload payload ok').toBeTruthy();
    imageFileId = imageBody.Id;
    expect(imageFileId, 'diagnostic uploaded image id').toBeTruthy();

    for (const route of ['/blog', '/admin/index', '/member/index']) {
      await page.goto(new URL(route, env.baseUrl).href, { waitUntil: 'domcontentloaded' });
      expect(await page.evaluate(() => window.__leanoteMigrateCheck()), `migrate live on ${route}`).toBe(true);
    }
    await page.goto(new URL('/tinymce/plugins/leaui_image/index.html', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
    expect(await page.evaluate(() => window.__leanoteMigrateCheck()), 'migrate live in the leaui_image document').toBe(true);

    const evidenceRaw = await page.evaluate(() => sessionStorage.getItem('leanote-migrate-evidence'));
    const pairedWarnings = evidenceRaw ? (JSON.parse(evidenceRaw).warnings ?? []) : [];
    expect(pairedWarnings.length, 'migrate must have observed the documented third-party warnings').toBeGreaterThan(0);
    expect(pairedWarnings.length, 'every console warning must have a stack-attributed record').toBe(warnings.length);
    const { ownOffenders, excludedByCategory } = classify(pairedWarnings, bundleRanges);
    const ownOffenderList = [...ownOffenders];
    const exercisedCategories = [...excludedByCategory.keys()];
    const missingCategories = [...EXPECTED_EXCLUSION_CATEGORIES].filter((category) => !exercisedCategories.includes(category));
    expect(missingCategories, `registered exclusion categories must all be exercised; missing: ${missingCategories.join(', ')}; exercised: ${exercisedCategories.join(', ')}`).toEqual([]);
    expect(ownOffenderList, `first-party scripts must not trigger JQMIGRATE warnings:\n${ownOffenderList.join('\n')}`).toEqual([]);
  } catch (error) {
    primaryError = error;
  } finally {
    for (const [label, step] of uploadCleanupSteps.reverse()) {
      try {
        await step();
      } catch (error) {
        cleanupFailures.push(`${label}: ${error?.message || error}`);
      }
    }
  }
  if (primaryError) throw primaryError;
  expect(cleanupFailures, 'diagnostic upload cleanup must not leak data').toEqual([]);
});
