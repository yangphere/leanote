import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const asset = (relative) => path.join(ROOT, relative);
const LEAUI_BODY = fs.readFileSync(asset('public/tinymce/plugins/leaui_image/index.html'), 'utf8')
  .match(/<body[^>]*>([\s\S]*?)<\/body>/i)?.[1]
  .replace(/<script\b[\s\S]*?<\/script>/gi, '') || '';

async function loadBootstrapFixture(page, { jquery = false, common = false } = {}) {
  await page.setContent(`
    <base href="http://leanote.test/">
    <main>
      <button id="openModal" type="button" class="btn btn-primary" data-bs-toggle="modal" data-bs-target="#componentModal">Open modal</button>
      <div class="modal fade" id="componentModal" tabindex="-1" aria-hidden="true">
        <div class="modal-dialog"><div class="modal-content">
          <div class="modal-header"><h5 class="modal-title">Components</h5><button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button></div>
          <div class="modal-body">Modal body</div>
        </div></div>
      </div>
      <ul class="nav nav-tabs" id="componentTabs" role="tablist">
        <li class="nav-item" role="presentation"><button class="nav-link active" id="home-tab" data-bs-toggle="tab" data-bs-target="#home-pane" type="button" role="tab">Home</button></li>
        <li class="nav-item" role="presentation"><button class="nav-link" id="profile-tab" data-bs-toggle="tab" data-bs-target="#profile-pane" type="button" role="tab">Profile</button></li>
      </ul>
      <div class="tab-content"><div class="tab-pane fade show active" id="home-pane" role="tabpanel">Home</div><div class="tab-pane fade" id="profile-pane" role="tabpanel">Profile</div></div>
      <div class="dropdown"><button id="dropdownButton" class="btn btn-secondary dropdown-toggle" data-bs-toggle="dropdown" type="button">Menu</button><ul class="dropdown-menu"><li><a class="dropdown-item" href="#item" onclick="event.preventDefault()">Item</a></li></ul></div>
      <button id="tooltipButton" type="button" class="btn btn-secondary" data-bs-toggle="tooltip" data-bs-title="Tooltip text">Tooltip</button>
      <div id="alert" class="alert alert-warning alert-dismissible fade show" role="alert">Warning<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button></div>
      <div class="modal fade" id="leanoteDialogRemote" tabindex="-1" aria-hidden="true"></div>
    </main>
  `);
  await page.addStyleTag({ path: asset('public/css/bootstrap.css') });
  if (jquery) await page.addScriptTag({ path: asset('node_modules/jquery/dist/jquery.min.js') });
  await page.addScriptTag({ path: asset('public/js/bootstrap.js') });
  if (jquery) await page.addScriptTag({ path: asset('public/js/bootstrap-dialog.js') });
  if (common) await page.addScriptTag({ path: asset('public/js/common.js') });
}

async function loadBlogThemeFixture(page, theme) {
  await page.setViewportSize({ width: 600, height: 800 });
  await page.setContent(`
    <base href="http://leanote.test/">
    <nav class="navbar navbar-expand-md navbar-default">
      <div class="container">
        <button id="themeToggler" type="button" class="navbar-toggler" data-bs-toggle="collapse" data-bs-target="#themeCollapse" aria-controls="themeCollapse" aria-label="Toggle navigation">
          <span class="navbar-toggler-icon"></span>
        </button>
        <div id="themeCollapse" class="navbar-collapse collapse">
          <ul class="nav navbar-nav">
            <li><a class="nav-link" href="#home">Home</a></li>
            <li class="dropdown">
              <a id="themeDropdown" class="dropdown-toggle nav-link" href="#menu" data-hover="dropdown" data-bs-toggle="dropdown" role="button">More</a>
              <ul id="themeMenu" class="dropdown-menu" role="menu"><li><a class="dropdown-item" href="#item">Item</a></li></ul>
            </li>
          </ul>
        </div>
      </div>
    </nav>
  `);
  await page.addStyleTag({ path: asset('public/css/bootstrap.css') });
  await page.addStyleTag({ path: asset(`public/blog/themes/${theme}/style.css`) });
  await page.addScriptTag({ path: asset('node_modules/jquery/dist/jquery.min.js') });
  await page.addScriptTag({ path: asset('public/js/bootstrap.js') });
  await page.addScriptTag({ path: asset('public/js/bootstrap-hover-dropdown.js') });
}

async function loadLeauiIframeFixture(page, src) {
  await page.setContent('<main><iframe id="leaui-frame" title="Image manager"></iframe></main>');
  await page.evaluate(({ imageSrc }) => {
    window.UrlPrefix = 'http://leanote.test';
    window.LEAUI_DATAS = [{ src: imageSrc, width: '120', height: '60', title: 'seeded image', constrain: 1 }];
    window.GlobalConfigs = { uploadImageSize: 100 };
    window.getMsg = (key) => `translated:${key}`;
    document.getElementById('leaui-frame').src = 'about:blank';
  }, { imageSrc: src });
  const handle = await page.locator('#leaui-frame').elementHandle();
  const frame = await handle.contentFrame();
  if (!frame) throw new Error('leaui_image iframe did not create a child frame');
  await frame.setContent(`<base href="http://leanote.test/">${LEAUI_BODY}`);
  await frame.addStyleTag({ path: asset('public/css/bootstrap.css') });
  await frame.addStyleTag({ path: asset('public/tinymce/plugins/leaui_image/public/css/style.css') });
  await frame.addScriptTag({ path: asset('node_modules/jquery/dist/jquery.min.js') });
  await frame.evaluate(() => {
    const get = (url, data, success) => {
      if (typeof data === 'function') success = data;
      const request = { fail: () => request };
      setTimeout(() => {
        if (url.includes('/album/getAlbums')) success?.([]);
        if (url.includes('/file/getImages')) success?.({ Count: 0, List: [] });
      }, 0);
      return request;
    };
    window.jQuery.get = get;
    window.G = { maxSelected: 1, perPageItems: 12, imageSrcPrefix: 'upload' };
  });
  await frame.addScriptTag({ path: asset('public/js/bootstrap.js') });
  await frame.addScriptTag({ path: asset('public/js/plugins/libs-min/jquery.ui.widget.js') });
  await frame.addScriptTag({ path: asset('public/js/plugins/libs-min/jquery.iframe-transport.js') });
  await frame.addScriptTag({ path: asset('public/js/plugins/libs-min/jquery.fileupload.js') });
  await frame.addScriptTag({ path: asset('public/js/jquery.pagination.js') });
  await frame.addScriptTag({ path: asset('public/tinymce/plugins/leaui_image/public/js/main.js') });
  return frame;
}

test('Bootstrap 5 modal, tab, dropdown, tooltip and alert interactions are observable', async ({ page }) => {
  await loadBootstrapFixture(page);

  await page.locator('#openModal').click();
  await expect(page.locator('#componentModal')).toBeVisible();
  await expect(page.locator('.modal-backdrop')).toHaveCount(1);
  await expect(page.locator('body')).toHaveClass(/modal-open/);
  await page.locator('#componentModal .btn-close').click();
  await expect(page.locator('#componentModal')).toBeHidden();
  await expect(page.locator('.modal-backdrop')).toHaveCount(0);

  await page.locator('#profile-tab').click();
  await expect(page.locator('#profile-tab')).toHaveClass(/active/);
  await expect(page.locator('#profile-pane')).toHaveClass(/show/);
  await expect(page.locator('#home-pane')).not.toHaveClass(/active/);

  await page.locator('#dropdownButton').click();
  await expect(page.locator('.dropdown-menu')).toHaveClass(/show/);
  await page.locator('.dropdown-item').click();
  await expect(page.locator('.dropdown-menu')).not.toHaveClass(/show/);

  await page.evaluate(() => bootstrap.Tooltip.getOrCreateInstance(document.getElementById('tooltipButton')));
  await page.locator('#tooltipButton').hover();
  await expect(page.locator('.tooltip')).toBeVisible();
  await page.locator('#alert .btn-close').click();
  await expect(page.locator('#alert')).toBeHidden();
});

test('BootstrapDialog preserves dialog argument and button receiver through a real click', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true });
  await page.evaluate(() => {
    window.__dialog = BootstrapDialog.show({
      title: 'Dialog',
      message: 'Dialog body',
      buttons: [{
        id: 'dialog-ok',
        label: 'OK',
        action: function (dialog) {
          window.__dialogAction = {
            sameDialog: dialog === window.__dialog,
            receiverIsButton: Boolean(this && this.jquery),
            receiverId: this && this.attr('id'),
          };
          dialog.close();
        },
      }],
    });
  });
  await expect(page.locator('#dialog-ok')).toBeVisible();
  await page.locator('#dialog-ok').click();
  await expect(page.locator('.modal.show')).toHaveCount(0);
  await expect(page.locator('.modal-backdrop')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => window.__dialogAction)).toEqual({
    sameDialog: true,
    receiverIsButton: true,
    receiverId: 'dialog-ok',
  });
});

test('BootstrapDialog preserves lifecycle callbacks and public mutators', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true });
  await page.evaluate(() => {
    window.__dialogEvents = [];
    window.__dialog = BootstrapDialog.show({
      title: 'Initial',
      message: 'Body',
      type: 'type-warning',
      size: 'size-large',
      cssClass: 'custom-dialog',
      onhide: (dialog) => window.__dialogEvents.push(`hide:${dialog.getTitle()}`),
      onhidden: (dialog) => window.__dialogEvents.push(`hidden:${dialog.getMessage()}`),
      buttons: [{ id: 'mutate-button', label: 'OK', action: (dialog) => dialog.close() }],
    });
    window.__dialog.setTitle('Updated').setMessage('Updated body').setType('type-success').setSize('size-large').setCssClass('custom-dialog');
    window.__dialog.setData('marker', 'ok');
  });
  await expect(page.locator('#mutate-button')).toBeVisible();
  expect(await page.evaluate(() => ({
    title: window.__dialog.getTitle(),
    message: window.__dialog.getMessage(),
    type: window.__dialog.getType(),
    size: window.__dialog.getSize(),
    cssClass: window.__dialog.getCssClass(),
    data: window.__dialog.getData('marker'),
    button: Boolean(window.__dialog.getButton('mutate-button')),
  }))).toEqual({ title: 'Updated', message: 'Updated body', type: 'type-success', size: 'size-large', cssClass: 'custom-dialog', data: 'ok', button: true });
  await page.evaluate(() => window.__dialog.close());
  await expect(page.locator('.modal.show')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => window.__dialogEvents)).toEqual(['hide:Updated', 'hidden:Updated body']);
});

test('BootstrapDialog close mutators update an already-open modal instance', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true });
  await page.evaluate(() => {
    window.__dialog = BootstrapDialog.show({ title: 'Mutable close policy', message: 'Body' });
    window.__dialog.setCloseByKeyboard(false).setCloseByBackdrop(false);
  });
  await expect(page.locator('.modal.show')).toHaveCount(1);
  await page.keyboard.press('Escape');
  await expect(page.locator('.modal.show')).toHaveCount(1);
  await page.evaluate(() => {
    const modal = document.querySelector('.modal.show');
    modal.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    modal.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  await expect(page.locator('.modal.show')).toHaveCount(1);
  await page.evaluate(() => window.__dialog.close());
  await expect(page.locator('.modal.show')).toHaveCount(0);
});

test('BootstrapDialog can be reopened after a normal close', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true });
  await page.evaluate(() => {
    window.__dialog = BootstrapDialog.show({ title: 'Reusable', message: 'Body' });
  });
  await expect(page.locator('.modal.show')).toHaveCount(1);
  await page.evaluate(() => window.__dialog.close());
  await expect(page.locator('.modal.show')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => window.__dialog.isRealized())).toBe(false);
  await page.evaluate(() => window.__dialog.show());
  await expect(page.locator('.modal.show')).toHaveCount(1);
  await page.evaluate(() => window.__dialog.close());
  await expect(page.locator('.modal.show')).toHaveCount(0);
});

test('BootstrapDialog passes its instance to dynamic title and message content', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true });
  await page.evaluate(() => {
    window.__dynamicContent = [];
    window.__dialog = BootstrapDialog.show({
      title(dialog) {
        window.__dynamicContent.push(dialog);
        return window.jQuery('<span id="dynamic-title"></span>').text(dialog.getId());
      },
      message(dialog) {
        window.__dynamicContent.push(dialog);
        return window.jQuery('<span id="dynamic-message"></span>').text(dialog.getDefaultText());
      },
    });
  });
  await expect(page.locator('#dynamic-title')).toHaveText(await page.evaluate(() => window.__dialog.getId()));
  await expect(page.locator('#dynamic-message')).toHaveText('Tips');
  expect(await page.evaluate(() => window.__dynamicContent.every((dialog) => dialog === window.__dialog))).toBe(true);
});

test('remote modal encodes parameters and cleans stale state on success and failure', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true, common: true });
  const successUrl = await page.evaluate(async () => {
    let requested;
    window.fetch = async (url) => {
      requested = String(url);
      return { ok: true, text: async () => '<div class="modal-dialog"><div class="modal-content"><div class="modal-body">Remote content</div></div></div>' };
    };
    await showDialogRemote('/remote?existing=keep', { term: 'a b', tags: ['x&y', 'z'] });
    return requested;
  });
  const parsed = new URL(successUrl);
  expect(parsed.pathname).toBe('/remote');
  expect(parsed.searchParams.get('existing')).toBe('keep');
  expect(parsed.searchParams.get('term')).toBe('a b');
  expect(parsed.searchParams.getAll('tags')).toEqual(['x&y', 'z']);
  await expect(page.locator('#leanoteDialogRemote')).toHaveClass(/show/);
  await expect(page.locator('#leanoteDialogRemote')).toContainText('Remote content');

  await page.evaluate(async () => {
    window.fetch = async () => ({ ok: false, status: 503, text: async () => 'unavailable' });
    await showDialogRemote('/remote', { term: 'failure' });
  });
  await expect(page.locator('#leanoteDialogRemote .alert-danger')).toBeVisible();
  await page.evaluate(() => hideDialogRemote());
  await expect(page.locator('#leanoteDialogRemote')).toBeHidden();
  await expect(page.locator('.modal-backdrop')).toHaveCount(0);
  await expect(page.locator('body')).not.toHaveClass(/modal-open/);
});

test('remote modal makes network rejection and empty responses visible without stale state', async ({ page }) => {
  await loadBootstrapFixture(page, { jquery: true, common: true });
  await page.evaluate(async () => {
    window.fetch = async () => { throw new Error('offline'); };
    await showDialogRemote('/remote', { term: 'network' });
  });
  await expect(page.locator('#leanoteDialogRemote .alert-danger')).toBeVisible();
  await expect(page.locator('#leanoteDialogRemote')).not.toHaveAttribute('aria-busy', 'true');
  await page.evaluate(() => hideDialogRemote());
  await expect(page.locator('.modal-backdrop')).toHaveCount(0);
  await expect(page.locator('body')).not.toHaveClass(/modal-open/);

  await page.evaluate(async () => {
    window.fetch = async () => ({ ok: true, text: async () => '  \n\t' });
    await showDialogRemote('/remote', { term: 'empty' });
  });
  await expect(page.locator('#leanoteDialogRemote .alert-danger')).toBeVisible();
  await page.evaluate(() => hideDialogRemote());
  await expect(page.locator('.modal-backdrop')).toHaveCount(0);
  await expect(page.locator('body')).not.toHaveClass(/modal-open/);
});

test('all built-in blog themes keep Bootstrap 5 collapse and hover dropdown behavior', async ({ page }) => {
  for (const theme of ['default', 'elegant', 'nav_fixed']) {
    await loadBlogThemeFixture(page, theme);
    await page.locator('#themeToggler').click();
    await expect(page.locator('#themeCollapse')).toHaveClass(/show/);
    await page.locator('#themeDropdown').hover();
    await expect(page.locator('#themeMenu')).toBeVisible({ timeout: 2_000 });
    await page.mouse.move(1, 1);
    await expect(page.locator('#themeMenu')).toBeHidden({ timeout: 2_000 });
  }
});

test('leaui_image preserves parent iframe data and exposes its selected source', async ({ page }) => {
  const seededImageSrc = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==';
  const frame = await loadLeauiIframeFixture(page, seededImageSrc);
  await expect(frame.locator('#preview img')).toHaveAttribute('src', seededImageSrc);
  await expect(frame.locator('#preview img')).toHaveAttribute('data-width', '120');
  await expect(frame.locator('#preview img')).toHaveAttribute('data-height', '60');
  await expect(frame.locator('#preview img')).toHaveAttribute('data-title', 'seeded image');
  expect(await frame.evaluate(() => ({
    sameParentData: top.LEAUI_DATAS[0].src === mdGetImgSrc(),
    parentConfig: parent.GlobalConfigs.uploadImageSize,
    parentMessage: parent.getMsg('Images'),
  }))).toEqual({ sameParentData: true, parentConfig: 100, parentMessage: 'translated:Images' });
  await frame.locator('#preview li').first().click();
  await expect(frame.locator('#attrTitle')).toBeEnabled();
  await frame.locator('#previewAttrs').evaluate((element) => { element.style.display = 'block'; });
  await frame.locator('#attrTitle').fill('edited image');
  await frame.locator('#attrTitle').press('End');
  await expect(frame.locator('#preview img')).toHaveAttribute('data-title', 'edited image');
  expect(await page.evaluate(() => document.getElementById('leaui-frame').contentWindow.mdGetImgSrc())).toBe(seededImageSrc);
});
