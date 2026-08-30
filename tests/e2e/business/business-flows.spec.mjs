import { test, expect } from '@playwright/test';
import { ensureE2EIdentity, confirmE2EIdentityFresh, assertIdentityResponse } from '../e2e-environment.mjs';
import { writeBusinessSummary } from './business-summary.mjs';

const summary = {
  stage: 'prerequisite-check',
  service: { readiness: 'unknown', status: null },
  auth: { finalUrl: null, authenticated: false },
  pages: [],
  resources: [],
  errors: [],
};

function newObjectId() {
  const hex = '0123456789abcdef';
  let id = '';
  for (let i = 0; i < 24; i += 1) id += hex[Math.floor(Math.random() * 16)];
  return id;
}

// Revel web actions answer HTTP 200 even when the business operation failed
// (Ok:false + message in the JSON body), so cleanups must assert the payload,
// never just the transport status.
async function getOkTrue(request, url, label) {
  const response = await request.get(url);
  expect(response.status(), `${label} transport status`).toBeLessThan(400);
  const body = await response.json().catch(() => {
    throw new Error(`${label}: response is not JSON`);
  });
  expect(body, `${label} payload`).toBeTruthy();
  if (typeof body === 'boolean') {
    expect(body, `${label} result`).toBe(true);
    return body;
  }
  expect(body.Ok, `${label} business Ok`).toBe(true);
  return body;
}

// Every verification request asserts the transport status first: a 500 JSON
// body that happens not to contain an id must never read as "cleaned up".
async function getJson(request, url, label) {
  const response = await request.get(url);
  expect(response.status(), `${label} transport status`).toBeLessThan(400);
  return response.json();
}

// Combines independent destructive steps so one failure can never skip the
// remaining deletions; every failure is reported together.
function sequentialSteps(...steps) {
  return async () => {
    const failures = [];
    for (const [label, step] of steps) {
      try {
        await step();
      } catch (error) {
        failures.push(`${label}: ${error?.message || error}`);
      }
    }
    if (failures.length) throw new Error(failures.join(' | '));
  };
}

test('identity preflight fails closed on a run token mismatch', async ({ request }) => {
  const env = await ensureE2EIdentity();
  const response = await request.get(new URL('/_test/e2e/identity', env.baseUrl).href);
  expect(response.status(), 'identity endpoint status inside an isolated harness run').toBe(200);
  const body = await response.json();
  expect(body.database).toBe('leanote_test');
  expect(() => assertIdentityResponse(body, `tampered-${env.runToken}`)).toThrow(/run token mismatch/);
  expect(() => assertIdentityResponse({ database: 'leanote', runToken: env.runToken }, env.runToken)).toThrow(/leanote_test/);
});

test('wrong credentials are rejected by /login', async ({ page }) => {
  const env = await ensureE2EIdentity();
  await page.goto(new URL('/login', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
  await page.locator('input[type="password"]').first().fill(`${env.password}-wrong`);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  // Rejected credentials must keep the user on the login screen.
  await page.waitForURL((url) => /\/login(?:\/|$)/.test(url.pathname) || url.pathname === '/home/login', { timeout: 15_000 });
  expect(new URL(page.url()).pathname, 'wrong password must not authenticate').toMatch(/login/i);
});

test('leaui_image preserves the real parent iframe boundary and TinyMCE insertion callback', async ({ page }) => {
  test.setTimeout(120_000);
  const env = await ensureE2EIdentity();
  const seededImageSrc = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==';

  // This is an actual same-origin parent document with the application iframe
  // URL. The parent owns the values that the production plugin exposes across
  // frames; navigating first establishes the service origin before replacing
  // the body with the controlled parent shell.
  await page.goto(new URL('/_test/e2e/identity', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.evaluate(({ baseUrl, src }) => {
    window.UrlPrefix = baseUrl;
    window.LEAUI_DATAS = [{ src, width: '120', height: '60', title: 'seeded image', constrain: 1 }];
    window.GlobalConfigs = { uploadImageSize: 100 };
    window.getMsg = (key) => key;
    document.body.innerHTML = `<iframe id="contract-frame" src="${new URL('/tinymce/plugins/leaui_image/index.html', baseUrl).href}" frameborder="0"></iframe>`;
  }, { baseUrl: env.baseUrl, src: seededImageSrc });
  const frame = page.frameLocator('#contract-frame');
  await expect(frame.locator('#preview img')).toHaveAttribute('src', seededImageSrc, { timeout: 30_000 });
  await expect(frame.locator('#preview img')).toHaveAttribute('data-width', '120');
  await expect(frame.locator('#preview img')).toHaveAttribute('data-height', '60');
  await expect(frame.locator('#preview img')).toHaveAttribute('data-title', 'seeded image');
  expect(await frame.locator('body').evaluate((body) => ({
    jquery: body.ownerDocument.defaultView.jQuery?.fn?.jquery,
    fileupload: Boolean(body.ownerDocument.defaultView.jQuery?.fn?.fileupload),
    parentData: body.ownerDocument.defaultView.top.LEAUI_DATAS?.[0]?.title,
    parentLimit: body.ownerDocument.defaultView.parent.GlobalConfigs?.uploadImageSize,
  }))).toEqual({ jquery: expect.stringMatching(/^3\./), fileupload: true, parentData: 'seeded image', parentLimit: 100 });

  // Load the real first-party plugin into a controlled TinyMCE shell. The
  // shell implements only the public editor methods used by this plugin, so
  // the callback traverses the same iframe and calls editor.insertContent.
  await page.evaluate(() => {
    window.__pluginFactory = null;
    window.tinymce = { PluginManager: { add: (_name, factory) => { window.__pluginFactory = factory; } } };
  });
  await page.addScriptTag({ url: new URL('/tinymce/plugins/leaui_image/plugin.js', env.baseUrl).href });
  await page.evaluate((src) => {
    window.__inserted = [];
    window.__closed = false;
    const editor = {
      selection: { getContent: () => `<img src="${src}" width="120" height="60" title="seeded image" />` },
      dom: {
        getAttrib: (element, name) => element.getAttribute(name) || '',
        createHTML: (tag, attrs) => {
          const element = document.createElement(tag);
          Object.entries(attrs).forEach(([name, value]) => element.setAttribute(name, value));
          return element.outerHTML;
        },
        get: (id) => document.getElementById(id),
        setAttrib: (element, name, value) => {
          if (!element) return;
          if (value == null) element.removeAttribute(name); else element.setAttribute(name, value);
        },
      },
      insertContent: (html) => window.__inserted.push(html),
      addButton: (_name, config) => { window.__button = config; },
      addMenuItem: () => {},
      windowManager: {
        open: (config) => {
          window.__dialogConfig = config;
          const holder = document.createElement('div');
          holder.innerHTML = config.html;
          document.body.append(...holder.childNodes);
          return {};
        },
      },
    };
    window.__pluginFactory(editor);
    window.__button.onclick();
  }, seededImageSrc);
  await expect(page.locator('#leauiIfr')).toHaveCount(1, { timeout: 30_000 });
  await expect(page.frameLocator('#leauiIfr').locator('#preview img')).toHaveAttribute('src', seededImageSrc, { timeout: 30_000 });
  await page.evaluate(() => {
    const context = { parent: () => ({ parent: () => ({ close: () => { window.__closed = true; } }) }) };
    window.__dialogConfig.buttons[1].onclick.call(context, {});
  });
  await expect.poll(() => page.evaluate(() => window.__inserted.length)).toBeGreaterThan(0);
  expect(await page.evaluate(() => ({ closed: window.__closed, inserted: window.__inserted[0] }))).toEqual({
    closed: true,
    inserted: expect.stringContaining('__mcenew0'),
  });
});

test('business flows: login, permission gates, note list/search, note+tag write, notebook, dialog, uploads, album, blog, admin, member, leaui iframe', async ({ page }) => {
  test.setTimeout(300_000);
  const request = page.request;
  const errors = [];
  const ownedResourcePaths = new Set([
    '/js/dep.min.js', '/js/app.min.js', '/js/markdown-v2.min.js',
    '/public/js/plugins/main.min.js', '/public/album/js/main.all.js',
    '/js/jquery-1.9.0.min.js',
  ]);
  const cleanups = [];
  let cleanupFailures = 0;
  let bodyError = null;
  let baseUrl = null;
  let env = null;

  page.on('console', (message) => {
    if (message.type() === 'error') {
      // Firefox reports font-pipeline failures ("downloadable font: download
      // failed") as console errors even though the font files are served
      // correctly and the page renders; this is pre-existing engine noise,
      // not an application script error. Everything else fails the gate.
      if (/downloadable font:/i.test(message.text())) return;
      // Echo the text to the test log for diagnosis; only the allowlisted
      // code below reaches the sanitized summary artifact.
      console.warn(`[business-e2e] console.error on ${page.url()}: ${message.text()}`);
      errors.push('runner:console-error');
    }
  });
  page.on('pageerror', () => errors.push('runner:page-error'));
  page.on('requestfailed', (failed) => {
    if (ownedResourcePaths.has(new URL(failed.url()).pathname)) errors.push('runner:request-failed');
  });
  page.on('response', (response) => {
    if (ownedResourcePaths.has(new URL(response.url()).pathname) && response.status() >= 400) errors.push('runner:http-error');
  });
  // Keep rejection evidence in sessionStorage so it survives every
  // same-tab navigation. A window-scoped flag is reset with each document
  // and therefore only reports the final page.
  await page.addInitScript(() => {
    const key = 'leanote-unhandled-rejections';
    window.addEventListener('unhandledrejection', (event) => {
      try {
        const current = JSON.parse(sessionStorage.getItem(key) || '[]');
        current.push(String(event?.reason?.message || event?.reason || 'unhandled rejection'));
        sessionStorage.setItem(key, JSON.stringify(current));
      } catch {}
    });
  });

  try {
    // --- preflight + authentication (inside the summary-guarded try) ---
    summary.stage = 'prerequisite-check';
    env = await ensureE2EIdentity();
    baseUrl = env.baseUrl;

    summary.stage = 'authentication';
    await page.goto(new URL('/login', baseUrl).href, { waitUntil: 'domcontentloaded' });
    await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
    await page.locator('input[type="password"]').first().fill(env.password);
    await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
    await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 30_000 });
    const authUrl = new URL(page.url());
    summary.auth.finalUrl = `${authUrl.origin}${authUrl.pathname}`;
    summary.auth.authenticated = authUrl.origin === new URL(baseUrl).origin;
    expect(summary.auth.authenticated, 'authenticated final URL').toBe(true);

    // --- permission gates BEFORE the first write: the rotated admin account
    // must be able to reach the admin and member areas it will clean up in ---
    for (const route of ['/admin/index', '/member/index']) {
      const response = await page.goto(new URL(route, baseUrl).href, { waitUntil: 'domcontentloaded' });
      const status = response?.status() ?? 0;
      expect(status, `${route} permission gate`).toBeGreaterThanOrEqual(200);
      expect(status, `${route} permission gate`).toBeLessThan(400);
      expect(new URL(page.url()).pathname, `${route} must not redirect to login`).not.toMatch(/\/login(?:\/|$)/);
    }

    // --- fresh identity confirmation: no mutation may happen before the
    // live service re-proves marker, token and database identity ---
    await confirmE2EIdentityFresh();

    // --- notebook write (cleanup registered immediately after creation) ---
    const notebookId = newObjectId();
    const notebookTitle = `e2e-notebook-${Date.now()}`;
    const addNotebook = await request.get(new URL(`/notebook/addNotebook?notebookId=${notebookId}&title=${encodeURIComponent(notebookTitle)}`, baseUrl).href);
    expect(addNotebook.ok(), 'addNotebook status').toBe(true);
    const notebookBody = await addNotebook.json();
    expect(notebookBody?.NotebookId, 'addNotebook returned the requested notebook').toBe(notebookId);
    cleanups.push(sequentialSteps(
      ['deleteNotebook', async () => {
        // DeleteNotebook is a soft delete (IsDeleted tombstone) when the
        // notebook is empty; assert the business Ok AND that the tombstone
        // no longer appears in the active notebook list.
        await getOkTrue(request, new URL(`/notebook/deleteNotebook?notebookId=${notebookId}`, baseUrl).href, 'deleteNotebook');
        const notebooks = await getJson(request, new URL('/notebook/getNotebooks', baseUrl).href, 'getNotebooks');
        expect(JSON.stringify(notebooks)).not.toContain(notebookId);
      }],
    ));

    // --- note + tag write through the real updateNoteOrContent path ---
    await confirmE2EIdentityFresh();
    const noteId = newObjectId();
    const tagName = `e2e-tag-${Date.now()}`;
    const noteTitle = `e2e note ${Date.now()}`;
    const addNote = await request.post(new URL('/note/updateNoteOrContent', baseUrl).href, {
      // Production saves notes through ajaxPost (form-encoded), which is what
      // Revel's struct binding expects; mirror it exactly.
      form: {
        IsNew: 'true',
        NoteId: noteId,
        NotebookId: notebookId,
        Title: noteTitle,
        Tags: tagName,
        Content: '<p>leanote business e2e note</p>',
      },
    });
    expect(addNote.ok(), 'addNote status').toBe(true);
    const noteBody = await addNote.json();
    expect(noteBody?.NoteId, 'addNote returned the requested note').toBe(noteId);
    expect(noteBody?.Title, 'addNote kept the title').toBe(noteTitle);
    expect(JSON.stringify(noteBody?.Tags ?? []), 'addNote stored the tag').toContain(tagName);
    // Register the destructive cleanup right away so a later assertion
    // failure can never leave the note behind.
    cleanups.push(sequentialSteps(
      ['deleteNote', async () => {
        // deleteNote always answers HTTP 200 with `true`, even when the
        // trash service failed, so every step is verified by observation:
        // 1. the body must be literally `true`;
        // 2. the note must appear in the trash;
        // 3. deleteTrash purges it (business Ok);
        // 4. the trash no longer lists it.
        const removed = await request.post(new URL('/note/deleteNote', baseUrl).href, {
          // Revel binds []string only from the jQuery-style bracket key.
          form: { 'noteIds[]': noteId, isShared: 'false' },
        });
        expect(removed.status(), 'deleteNote transport status').toBeLessThan(400);
        expect(await removed.json(), 'deleteNote result').toBe(true);
        const trashed = await getJson(request, new URL('/note/listTrashNotes', baseUrl).href, 'listTrashNotes');
        expect(JSON.stringify(trashed), 'note must be in the trash before purge').toContain(noteId);
        await getOkTrue(request, new URL(`/note/deleteTrash?noteId=${noteId}`, baseUrl).href, 'deleteTrash');
        const trashAfter = await getJson(request, new URL('/note/listTrashNotes', baseUrl).href, 'listTrashNotes');
        expect(JSON.stringify(trashAfter), 'note must not linger in trash').not.toContain(noteId);
      }],
      ['deleteTag', async () => {
        // Tag.DeleteTag always answers Ok:true, so verify the active tag
        // list rendered into /note no longer contains the tag.
        await getOkTrue(request, new URL(`/tag/deleteTag?tag=${encodeURIComponent(tagName)}`, baseUrl).href, 'deleteTag');
        const notePage = await request.get(new URL('/note', baseUrl).href);
        expect(notePage.status(), 'note page transport status').toBeLessThan(400);
        expect(await notePage.text(), 'deleted tag must vanish from the rendered tag list').not.toContain(tagName);
        const untagged = await getJson(request, new URL(`/note/searchNoteByTags?tags[]=${encodeURIComponent(tagName)}`, baseUrl).href, 'searchNoteByTags');
        expect(JSON.stringify(untagged), 'tag search must not resolve deleted note').not.toContain(noteId);
      }],
    ));

    // --- note page: list render, tag data, search ---
    // The tag nav DOM is transient (selecting a note re-renders it from that
    // note's own tags), so the durable contract is the server-rendered tag
    // list in the /note document itself.
    summary.stage = 'page-checks';
    const notePageResponse = await page.goto(new URL('/note', baseUrl).href, { waitUntil: 'domcontentloaded' });
    expect(notePageResponse?.status() ?? 0, '/note status').toBeLessThan(400);
    expect(await notePageResponse.text(), 'rendered /note must carry the fresh tag').toContain(tagName);
    await page.locator('#searchNoteInput').waitFor({ state: 'visible', timeout: 30_000 });
    await expect(page.locator('#notebook [notebookId]').first()).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('#noteItemList li').first()).toBeVisible({ timeout: 30_000 });
    const searchDone = page.waitForResponse((response) => new URL(response.url()).pathname === '/note/searchNote', { timeout: 30_000 });
    await page.locator('#searchNoteInput').fill(noteTitle.replace('e2e note ', ''));
    await page.locator('#searchNoteInput').press('Enter');
    const searchResponse = await searchDone;
    expect(searchResponse.status(), 'note search status').toBe(200);
    summary.pages.push({ url: '/note', status: searchResponse.status(), finalPath: '/note', authenticated: true });

    // --- dialog (theme settings modal; #setTheme lives inside a collapsed dropdown) ---
    await page.evaluate(() => window.jQuery('#setTheme').trigger('click'));
    await expect(page.locator('#setThemeDialog')).toBeVisible({ timeout: 15_000 });
    await page.locator('#setThemeDialog .btn-close, #setThemeDialog [data-bs-dismiss="modal"]').first().click();
    await expect(page.locator('#setThemeDialog')).toBeHidden({ timeout: 15_000 });

    // --- real attachment upload through the plugins bundle upload stack ---
    // The cleanup is registered BEFORE the upload; if the response cannot be
    // parsed, the fallback locates the attachment by its unique file name.
    const attachStamp = Date.now();
    const attachName = `e2e-attach-${attachStamp}.txt`;
    const attachNoteId = await page.evaluate(() => (window.Note && window.Note.curNoteId) || '');
    expect(attachNoteId, 'note page has a selected note for the upload').toBeTruthy();
    let attachId = null;
    cleanups.push(sequentialSteps(
      ['deleteAttach', async () => {
        let id = attachId;
        if (!id) {
          const list = await getJson(request, new URL(`/attach/GetAttachs?noteId=${attachNoteId}`, baseUrl).href, 'GetAttachs');
          const entry = (Array.isArray(list) ? list : []).find((item) => item?.Name === attachName || item?.Title === attachName);
          id = entry?.AttachId || null;
        }
        if (!id) return; // nothing was stored — cleanup is satisfied
        // Ok:true also proves the on-disk file was removed (AttachService
        // only reports success when os.Remove succeeded).
        await getOkTrue(request, new URL(`/attach/DeleteAttach?attachId=${id}`, baseUrl).href, 'DeleteAttach');
        const list = await getJson(request, new URL(`/attach/GetAttachs?noteId=${attachNoteId}`, baseUrl).href, 'GetAttachs');
        expect(JSON.stringify(list), 'attachment must not be listed after cleanup').not.toContain(id);
      }],
    ));
    await confirmE2EIdentityFresh();
    const attachDone = page.waitForResponse((response) => new URL(response.url()).pathname === '/attach/UploadAttach', { timeout: 60_000 });
    await page.locator('#uploadAttach input[type="file"]').setInputFiles({
      name: attachName,
      mimeType: 'text/plain',
      buffer: Buffer.from('leanote business e2e attachment'),
    });
    const attachResponse = await attachDone;
    expect(attachResponse.status(), 'attachment upload status').toBe(200);
    const attachBody = await attachResponse.json();
    expect(attachBody.Ok, 'attachment upload payload ok').toBeTruthy();
    attachId = attachBody.Item?.AttachId || null;
    expect(attachId, 'attachment id').toBeTruthy();

    // --- album write + real image upload through the album bundle stack ---
    await confirmE2EIdentityFresh();
    const albumName = `e2e-album-${attachStamp}`;
    const addAlbum = await request.get(new URL(`/album/addAlbum?name=${encodeURIComponent(albumName)}`, baseUrl).href);
    expect(addAlbum.ok(), 'addAlbum status').toBe(true);
    const albumId = (await addAlbum.json())?.AlbumId;
    expect(albumId, 'album id').toBeTruthy();
    const imageName = `e2e-image-${attachStamp}.png`;
    let imageFileId = null;
    cleanups.push(sequentialSteps(
      ['deleteImage', async () => {
        let id = imageFileId;
        if (!id) {
          const images = await getJson(request, new URL(`/file/getImages?albumId=${albumId}&page=1`, baseUrl).href, 'getImages');
          const entry = (images?.List ?? []).find((item) => item?.Title === imageName || item?.Name === imageName);
          id = entry?.FileId || null;
        }
        if (!id) return;
        // Ok:true also proves the on-disk image file was removed.
        await getOkTrue(request, new URL(`/file/deleteImage?fileId=${id}`, baseUrl).href, 'deleteImage');
        const after = await getJson(request, new URL(`/file/getImages?albumId=${albumId}&page=1`, baseUrl).href, 'getImages');
        expect(JSON.stringify(after), 'image must not be listed after cleanup').not.toContain(id);
      }],
      ['deleteAlbum', async () => {
        await getOkTrue(request, new URL(`/album/deleteAlbum?albumId=${albumId}`, baseUrl).href, 'deleteAlbum');
      }],
    ));

    // In production the album document always runs inside the note page
    // iframe; the parent supplies GlobalConfigs (album main.js reads
    // parent.GlobalConfigs.uploadImageSize). Seed the same contract for the
    // standalone document so the upload add-handler behaves as in the real
    // iframe context.
    await page.addInitScript(() => {
      window.GlobalConfigs = window.GlobalConfigs || { uploadImageSize: 100 };
    });
    await page.goto(new URL('/album/index', baseUrl).href, { waitUntil: 'domcontentloaded' });
    await expect(page.locator(`#albumsForList option[value="${albumId}"]`)).toBeAttached({ timeout: 30_000 });
    // Real user path: pick the album in the visible list, then jump to the
    // upload tab (its shown.bs.tab handler copies the selection).
    await page.locator('#albumsForList').selectOption(albumId);
    await page.locator('#goAddImageBtn').click();
    await expect(page.locator('#albumsForUpload')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('#albumsForUpload')).toHaveValue(albumId, { timeout: 15_000 });
    await confirmE2EIdentityFresh();
    const imageDone = page.waitForResponse((response) => new URL(response.url()).pathname === '/file/uploadImageLeaui', { timeout: 60_000 });
    await page.locator('#upload input[name="file"]').first().setInputFiles({
      name: imageName,
      mimeType: 'image/png',
      buffer: Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    });
    const imageResponse = await imageDone;
    expect(imageResponse.status(), 'album image upload status').toBe(200);
    const imageBody = await imageResponse.json();
    expect(imageBody.Ok, 'album image upload payload ok').toBeTruthy();
    imageFileId = imageBody.Id || null;
    expect(imageFileId, 'uploaded image id').toBeTruthy();

    // --- blog page (same rotated admin account) ---
    const blogResponse = await page.goto(new URL('/blog', baseUrl).href, { waitUntil: 'domcontentloaded' });
    const blogStatus = blogResponse?.status() ?? 0;
    summary.pages.push({ url: '/blog', status: blogStatus, finalPath: new URL(page.url()).pathname, authenticated: true });
    expect(blogStatus, '/blog status').toBeGreaterThanOrEqual(200);
    expect(blogStatus, '/blog status').toBeLessThan(400);
    expect(new URL(page.url()).pathname, '/blog must not redirect to login').not.toMatch(/\/login(?:\/|$)/);

    // --- leaui_image iframe document (private fileupload copies + shared runtime URL) ---
    // Seed the same top-level selection contract that TinyMCE's plugin uses
    // when opening the iframe, then verify the iframe preserves image
    // metadata and exposes the selected source back to the parent.
    const seededImageSrc = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==';
    await page.addInitScript(({ src }) => {
      window.LEAUI_DATAS = [{ src, width: '120', height: '60', title: 'seeded image', constrain: 1 }];
      window.GlobalConfigs = window.GlobalConfigs || { uploadImageSize: 100 };
      window.getMsg = window.getMsg || ((key) => key);
    }, { src: seededImageSrc });
    await page.goto(new URL('/tinymce/plugins/leaui_image/index.html', baseUrl).href, { waitUntil: 'domcontentloaded' });
    const leaui = await page.evaluate(() => ({
      jquery: window.jQuery && window.jQuery.fn && window.jQuery.fn.jquery,
      fileupload: Boolean(window.jQuery && window.jQuery.fn && window.jQuery.fn.fileupload),
    }));
    expect(leaui.jquery, 'leaui_image shared runtime jQuery major').toMatch(/^3\./);
    expect(leaui.fileupload, 'leaui_image fileupload registered').toBe(true);
    await expect(page.locator('#preview img')).toHaveAttribute('src', seededImageSrc);
    await expect(page.locator('#preview img')).toHaveAttribute('data-width', '120');
    await expect(page.locator('#preview img')).toHaveAttribute('data-height', '60');
    await expect(page.locator('#preview img')).toHaveAttribute('data-title', 'seeded image');
    expect(await page.evaluate(() => mdGetImgSrc()), 'leaui_image selected source crosses the iframe boundary').toBe(seededImageSrc);
    await page.locator('#preview li').first().click();
    await expect(page.locator('#attrTitle')).toBeEnabled();
    await expect(page.locator('#attrTitle')).toHaveValue('seeded image');
    await page.locator('#attrTitle').fill('edited image');
    await page.locator('#attrTitle').press('End');
    await expect(page.locator('#preview img')).toHaveAttribute('data-title', 'edited image');
    await page.locator('#attrWidth').fill('240');
    await page.locator('#attrWidth').press('End');
    await expect(page.locator('#preview img')).toHaveAttribute('data-width', '240');
    await page.locator('#attrConstrain').check();
    await page.locator('#attrHeight').fill('120');
    await page.locator('#attrHeight').press('End');
    await expect(page.locator('#preview img')).toHaveAttribute('data-height', '120');
    await page.locator('#myTab a[href="#url"]').click();
    const urlImage = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';
    await page.locator('#imageUrl').fill(urlImage);
    await page.locator('#addImageUrlBtn').click();
    await expect.poll(() => page.locator('#preview img').evaluateAll((images, src) => images.filter((image) => image.getAttribute('src') === src).length, urlImage)).toBe(1);
    summary.pages.push({ url: '/tinymce/plugins/leaui_image/index.html', status: 200, finalPath: '/tinymce/plugins/leaui_image/index.html', authenticated: false });
    // The standalone document must not request missing resources (a missing
    // UrlPrefix used to produce undefined/... 404s).
    const leauiFailures = [];
    page.on('response', (response) => {
      if (response.status() >= 400) leauiFailures.push(`${response.status()} ${response.url()}`);
    });
    await page.reload({ waitUntil: 'networkidle' });
    // networkidle means every request has already settled — a 404 that fires
    // late cannot escape the assertion the way it could with a fixed wait.
    await page.waitForLoadState('networkidle', { timeout: 15_000 });
    expect(leauiFailures, 'leaui_image document must not request missing resources').toEqual([]);

    // --- write-gate re-check: identity is re-requested and revalidated
    // against the live service before any further writes ---
    await confirmE2EIdentityFresh();

    const rejectionCount = await page.evaluate(() => {
      try { return JSON.parse(sessionStorage.getItem('leanote-unhandled-rejections') || '[]').length; } catch { return 1; }
    });
    if (rejectionCount > 0) errors.push('runner:unhandled-rejection');
    expect(errors, 'browser/network errors').toEqual([]);
    summary.stage = 'complete';
  } catch (error) {
    bodyError = error;
    summary.errors.push('runner:runner-error');
  } finally {
    // Revalidate identity before issuing any destructive request. If the
    // marker/token/database proof is stale, fail closed: do not delete shared
    // data under an unproven identity, and make the test fail explicitly.
    let identityFresh = false;
    try {
      await confirmE2EIdentityFresh();
      identityFresh = true;
    } catch {
      summary.errors.push('runner:identity-fresh-failed');
    }
    if (!identityFresh) {
      bodyError = bodyError || new Error('E2E identity freshness proof failed; destructive cleanup skipped');
    } else {
      for (const cleanup of cleanups.reverse()) {
        try {
          await cleanup();
        } catch (cleanupError) {
          cleanupFailures += 1;
          // Messages are test-authored strings (no credentials, no user data);
          // they are needed to diagnose which cleanup step regressed.
          console.warn(`[business-e2e] cleanup failed: ${cleanupError?.message || cleanupError}`);
          summary.errors.push('runner:cleanup-failed');
        }
      }
    }
    if (summary.stage !== 'complete') summary.stage = `${summary.stage.replace(':failed', '')}:failed`;
    // A cleanup failure must never leave the summary marked complete.
    if (cleanupFailures > 0 && !summary.stage.endsWith(':failed')) {
      summary.stage = `${summary.stage}:failed`;
    }
    await writeBusinessSummary(summary);
    if (bodyError) throw bodyError;
    expect(cleanupFailures, 'cleanup failures leave no fixture data behind').toBe(0);
  }
});
