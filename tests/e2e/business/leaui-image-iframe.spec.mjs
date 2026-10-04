import { test, expect } from '@playwright/test';
import { ensureE2EIdentity } from '../e2e-environment.mjs';

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
    window.__editorListeners = {};
    const editor = {
      selection: { getNode: () => null },
      getContent: () => window.__inserted.join(''),
      dom: {
        get: (id) => document.getElementById(id),
        setAttrib: (element, name, value) => {
          if (!element) return;
          if (value == null) element.removeAttribute(name); else element.setAttribute(name, value);
        },
      },
      insertContent: (html) => window.__inserted.push(html),
      // The real TinyMCE 8 Editor exposes the Observable event API; the plugin
      // legitimately depends on it (factory-time dragstart guard, onSetup
      // subscriptions), so the shell implements the same boundary.
      on: (names, handler) => {
        for (const name of String(names).split(' ')) {
          (window.__editorListeners[name] = window.__editorListeners[name] || []).push(handler);
        }
      },
      off: (names, handler) => {
        for (const name of String(names).split(' ')) {
          window.__editorListeners[name] = (window.__editorListeners[name] || []).filter((registered) => registered !== handler);
        }
      },
      ui: { registry: {
        addButton: (_name, config) => { window.__button = config; },
        addMenuItem: () => {},
      } },
      windowManager: {
        openUrl: (config) => {
          window.__dialogConfig = config;
          return {};
        },
      },
    };
    window.__pluginFactory(editor);
    window.__button.onAction();
  }, seededImageSrc);
  expect(await page.evaluate(() => Object.keys(window.__editorListeners))).toEqual(['dragstart']);
  expect(await page.evaluate(() => {
    const teardown = window.__button.onSetup({ setEnabled() {} });
    const subscribed = ['NodeChange', 'ModeChange'].every((name) => (window.__editorListeners[name] || []).length === 1);
    teardown();
    const unsubscribed = ['NodeChange', 'ModeChange'].every((name) => (window.__editorListeners[name] || []).length === 0);
    return { subscribed, unsubscribed };
  }), 'onSetup subscribes through the event API and its teardown unsubscribes').toEqual({ subscribed: true, unsubscribed: true });
  // openAlbum legitimately resets window.LEAUI_DATAS to the editor's current
  // selection before the dialog posts its message, so the seeded source must
  // travel through the test scope, not through the window variable.
  await page.evaluate((src) => {
    window.__dialogConfig.onMessage({ close: () => { window.__closed = true; } }, {
      mceAction: 'insertImage',
      images: [{ src, width: '120', height: '60', title: 'seeded image' }],
    });
  }, seededImageSrc);
  await expect.poll(() => page.evaluate(() => window.__inserted.length)).toBeGreaterThan(0);
  expect(await page.evaluate(() => ({ closed: window.__closed, inserted: window.__inserted[0] }))).toEqual({
    closed: true,
    inserted: expect.stringContaining(seededImageSrc),
  });
});
