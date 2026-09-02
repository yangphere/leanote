import { test, expect } from '@playwright/test';
import { ensureE2EIdentity, confirmE2EIdentityFresh } from '../e2e-environment.mjs';

function newObjectId() {
  const hex = '0123456789abcdef';
  let id = '';
  for (let i = 0; i < 24; i += 1) id += hex[Math.floor(Math.random() * hex.length)];
  return id;
}

async function login(page, env) {
  await page.goto(new URL('/login', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await page.locator('input[type="email"], input[name="email"], input[name="Email"]').first().fill(env.email);
  await page.locator('input[type="password"]').first().fill(env.password);
  await page.locator('#loginBtn, button[type="submit"], input[type="submit"]').first().click();
  await page.waitForURL((url) => !['/login', '/home/login'].includes(url.pathname), { timeout: 30_000 });
}

async function createNote(request, baseUrl, notebookId, noteId, title, content) {
  await confirmE2EIdentityFresh();
  const response = await request.post(new URL('/note/updateNoteOrContent', baseUrl).href, {
    form: {
      IsNew: 'true',
      NoteId: noteId,
      NotebookId: notebookId,
      Title: title,
      Content: content,
    },
  });
  expect(response.status(), 'new note transport status').toBeLessThan(400);
  const body = await response.json();
  expect(body?.Ok, 'new note business result').toBe(true);
  expect(body?.Item?.NoteId, 'new note id').toBe(noteId);
}

async function deleteNote(request, baseUrl, noteId) {
  await confirmE2EIdentityFresh();
  mark('cleanup-identity-fresh');
  const removed = await request.post(new URL('/note/deleteNote', baseUrl).href, {
    form: { 'noteIds[]': noteId, isShared: 'false' },
  });
  mark('cleanup-delete-post');
  expect(removed.status(), 'delete note transport status').toBeLessThan(400);
  expect(await removed.json(), 'delete note result').toBe(true);
  const purge = await request.get(new URL(`/note/deleteTrash?noteId=${noteId}`, baseUrl).href);
  expect(purge.status(), 'purge note transport status').toBeLessThan(400);
  const purgeBody = await purge.json();
  expect(purgeBody?.Ok ?? purgeBody, 'purge note result').toBeTruthy();
}

async function waitForEditor(page, id) {
  await expect.poll(
    () => page.evaluate((editorId) => Boolean(window.tinymce?.get?.(editorId)?.initialized), id),
    { timeout: 30_000 },
  ).toBe(true);
  return page.evaluate((editorId) => {
    const editor = window.tinymce.get(editorId);
    return {
      inline: editor.inline,
      license: editor.options.get('license_key'),
      language: editor.options.get('language'),
      plugins: editor.options.get('plugins'),
    };
  }, id);
}

// Diagnostic budget markers (B-E3 Req 4 协议): visibility only, no assertions.
function mark(label) {
  console.log(`[editor-flow-budget] ${label} +${Date.now() - globalThis.__editorFlowStart}ms`);
}
function startBudget() {
  globalThis.__editorFlowStart = Date.now();
}

function saveRequest(request) {
  return new URL(request.url()).pathname === '/note/updateNoteOrContent' && request.method() === 'POST';
}

test('note editor keeps load baseline, title-only saves, content revisions, undo and readonly gate', async ({ page }) => {
  test.setTimeout(180_000);
  startBudget();
  const pageErrors = [];
  page.on('pageerror', (error) => pageErrors.push(String(error && error.message || error)));
  const env = await ensureE2EIdentity();
  mark('identity');
  await login(page, env);
  mark('login');
  await page.goto(new URL('/note', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  const notebook = page.locator('#curNotebookForNewNote');
  await expect(notebook).toHaveAttribute('notebookId', /.+/, { timeout: 30_000 });
  const notebookId = await notebook.getAttribute('notebookId');
  const noteId = newObjectId();
  const initialContent = '<p>editor-flow baseline</p>';
  const title = `editor-flow-${Date.now()}`;
  let created = false;

  try {
    mark('notebook-ready');
    await createNote(page.request, env.baseUrl, notebookId, noteId, title, initialContent);
    created = true;
    mark('createNote');
    await page.goto(new URL(`/note?noteId=${noteId}`, env.baseUrl).href, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('#editorContent')).toBeVisible({ timeout: 30_000 });
    const profile = await waitForEditor(page, 'editorContent');
    mark('waitForEditor');
    expect(profile.inline, 'note editor uses inline mode').toBe(true);
    expect(profile.license, 'note editor uses GPL license').toBe('gpl');
    expect(profile.plugins, 'note editor includes first-party plugins').toEqual(expect.arrayContaining([
      'leaui_image', 'leaui_mindmap', 'leanote_nav', 'leanote_code',
    ]));
    expect(await page.evaluate(() => ({ dirty: window.LeanoteEditorSession.isDirty(), revision: window.LeanoteEditorSession.snapshot().contentRevision })))
      .toEqual({ dirty: false, revision: 0 });

    mark('pre-editBtn');
    await page.locator('#editBtn').click({ timeout: 30_000 });
    mark('post-editBtn');
    await page.locator('#noteTitle').fill(`${title} title-only`, { timeout: 30_000 });
    mark('post-title-fill');
    const titleRequestPromise = page.waitForRequest(saveRequest);
    const titleResponsePromise = page.waitForResponse((response) => new URL(response.url()).pathname === '/note/updateNoteOrContent' && response.request().method() === 'POST');
    mark('pre-saveBtn');
    await page.locator('#saveBtn').click({ timeout: 30_000 });
    mark('post-saveBtn');
    const [titleRequest, titleResponse] = await Promise.all([titleRequestPromise, titleResponsePromise]);
    expect((await titleResponse.json()).Ok, 'title-only save succeeds').toBe(true);
    expect(new URLSearchParams(titleRequest.postData() || '').has('Content'), 'title-only save omits Content').toBe(false);
    expect(await page.evaluate(() => window.LeanoteEditorSession.isDirty()), 'title-only save keeps content clean').toBe(false);
    mark('title-save-flow');

    const editor = page.locator('#editorContent');
    await editor.click();
    await editor.press('End');
    await editor.pressSequentially(' user-edit');
    await expect.poll(() => page.evaluate(() => window.LeanoteEditorSession.snapshot().contentRevision), { timeout: 15_000 }).toBeGreaterThan(0);
    mark('type-revision-poll');
    const contentRequestPromise = page.waitForRequest(saveRequest);
    const contentResponsePromise = page.waitForResponse((response) => new URL(response.url()).pathname === '/note/updateNoteOrContent' && response.request().method() === 'POST');
    await page.locator('#saveBtn').click();
    const [contentRequest, contentResponse] = await Promise.all([contentRequestPromise, contentResponsePromise]);
    const submittedContent = new URLSearchParams(contentRequest.postData() || '').get('Content');
    expect(submittedContent, 'content save sends serialized HTML').toBeTruthy();
    expect((await contentResponse.json()).Ok, 'content save succeeds').toBe(true);
    await expect.poll(() => page.evaluate(() => window.LeanoteEditorSession.isDirty()), { timeout: 15_000 }).toBe(false);
    expect(await page.evaluate(() => window.LeanoteEditorSession.snapshot().persistedContent)).toBe(submittedContent);
    mark('content-save-flow');

    await page.evaluate(() => window.tinymce.get('editorContent').undoManager.undo());
    await expect.poll(() => page.evaluate(() => window.LeanoteEditorSession.isDirty()), { timeout: 15_000 }).toBe(true);
    await page.evaluate(() => window.tinymce.get('editorContent').undoManager.redo());
    await expect.poll(() => page.evaluate(() => window.LeanoteEditorSession.isDirty()), { timeout: 15_000 }).toBe(false);
    mark('undo-redo');

    await page.locator('#editBtn').click();
    const beforeReadonly = await page.evaluate(() => window.LeanoteEditorSession.snapshot());
    await page.evaluate(() => {
      document.querySelector('#editorContent').dispatchEvent(new InputEvent('input', { bubbles: true, data: 'blocked' }));
    });
    await page.waitForTimeout(200);
    expect(await page.evaluate(() => window.LeanoteEditorSession.snapshot())).toEqual(beforeReadonly);
    mark('readonly-gate');
  } finally {
    const maskState = await page.evaluate(() => ({
      maskZ: document.getElementById('noteMaskForLoading') ? document.getElementById('noteMaskForLoading').style.zIndex : 'missing',
      contentAjax: Boolean(window.Note && Note.contentAjax),
      curNoteId: window.Note && Note.curNoteId,
    })).catch((error) => `unavailable: ${error}`);
    console.log(`[editor-flow-budget] mask-state ${JSON.stringify(maskState)} page-errors ${JSON.stringify(pageErrors)}`);
    mark('finally-enter');
    if (created) await deleteNote(page.request, env.baseUrl, noteId);
    mark('cleanup-done');
  }
});

test('member single and abstract entries initialize the shared TinyMCE 8 profile', async ({ page }) => {
  test.setTimeout(120_000);
  const env = await ensureE2EIdentity();
  await login(page, env);

  await page.goto(new URL('/member/blog/addOrUpdateSingle', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await expect(page.locator('#content1')).toBeVisible({ timeout: 30_000 });
  const singleProfile = await waitForEditor(page, 'content1');
  expect(singleProfile.inline, 'single editor uses inline mode').toBe(true);
  expect(singleProfile.license, 'single editor uses GPL license').toBe('gpl');
  expect(singleProfile.plugins, 'single editor excludes retired fullpage plugin').not.toContain('fullpage');

  await page.goto(new URL('/member/blog/index', env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  const abstractLink = page.locator('a[href*="/member/blog/updateBlogAbstract"]').first();
  await expect(abstractLink).toHaveCount(1, { timeout: 30_000 });
  const abstractHref = await abstractLink.getAttribute('href');
  await page.goto(new URL(abstractHref, env.baseUrl).href, { waitUntil: 'domcontentloaded' });
  await expect(page.locator('#content1')).toBeVisible({ timeout: 30_000 });
  const abstractProfile = await waitForEditor(page, 'content1');
  expect(abstractProfile.inline, 'abstract editor uses inline mode').toBe(true);
  expect(abstractProfile.license, 'abstract editor uses GPL license').toBe('gpl');
  expect(abstractProfile.plugins, 'abstract editor excludes retired fullpage plugin').not.toContain('fullpage');
});
