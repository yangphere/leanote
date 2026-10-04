# Component Guidelines

> How UI components are built in this project (jQuery + Bootstrap 5, no framework).

---

## Overview

"Components" are jQuery plugins/page modules plus Bootstrap 5 widgets. BootstrapDialog is the modal system. TinyMCE 8 is the editor component; first-party plugins register through `tinymce.PluginManager.add`.

## Component Structure

- **Page module**: an object on a window namespace with lifecycle as event bindings — e.g. `Note.changeNote`, `Note.renderNote` in `public/js/app/note.js`. New pages follow the same shape (module object + jQuery `$(...)` binding block).
- **BootstrapDialog usage** (see `tests/e2e/business/bootstrap-components.spec.mjs` for the asserted contract): `BootstrapDialog.show({ title, message, buttons: [{label, action(dialog){...}}] })`; the dialog instance passes to dynamic title/message callbacks; close via `dialog.close()`; reopened dialogs keep working after a normal close. Remote modals encode params in the URL and clean stale state on success and failure — no `remote` option (removed in Bootstrap 3→5 migration).
- **TinyMCE plugin** (`public/tinymce/plugins/<name>/plugin.js`): register via `tinymce.PluginManager.add('name', function(editor){...})`, declare buttons/menu items through `editor.ui.registry`, wire state via `editor.on('NodeChange ModeChange', ...)` in `onSetup` with a teardown return, and keep dialog exchange on `editor.windowManager.openUrl` + `postMessage mceAction`. See `leaui_image/plugin.js` as the reference.
- **Tabs/dropdowns/tooltips**: standard Bootstrap 5 data-api (`data-bs-toggle`) — programmatic switching uses `bootstrap.Tab.getOrCreateInstance(el).show()` (see `leaui_image/index.html` `showBootstrapTab`).

## Composition

- Cross-iframe composition only through `postMessage` payloads with stable action names; the parent owns the data seeding, the iframe owns collection/UI.
- Shared behaviors (hover dropdown delays, theme loading) live in the blog theme bundle and built-in themes — assertions in the browser suites are the contract; don't fork behavior per theme.

## Accessibility & UX rules (as enforced by tests)

- Failure visibility is mandatory: AJAX failures alert; dialogs surface errors (`leaui_image` shows `notificationManager` error toasts); resource 4xx/5xx on app-owned paths fail the e2e gates.
- `leaui_image` iframe must not request missing resources (standalone UrlPrefix handling) — networkidle assertion in the business suite.
