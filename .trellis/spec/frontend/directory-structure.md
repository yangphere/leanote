# Directory Structure

> How frontend code is organized in this project.

---

## Overview

Legacy jQuery/Bootstrap frontend (no bundler runtime, no framework) with a Node 24 manifest-driven build. jQuery 3.7.1 + Bootstrap 5.3.8 + self-hosted TinyMCE 8.8.2 are consumed from `node_modules` and published to the old public URLs.

## Directory Layout

| Path | Role | Real examples |
|------|------|---------------|
| `public/js/` | Page scripts and app modules (loaded by views) | `public/js/common.js` (shared helpers + editor content plumbing), `public/js/app/note.js` (Note service/controller), `public/js/plugins/` |
| `public/js/*-source.js` | Editable sources for generated bundles (`app.min.js` etc.); never edit the generated counterpart | `public/js/tinymce-config-source.js`, `public/js/editor-state-source.js` |
| `public/css/` | Stylesheets incl. generated minified variants | `public/css/bootstrap.css` (generated from npm Bootstrap) |
| `public/tinymce/plugins/leaui_*/`, `leanote_*` | First-party TinyMCE plugins; `plugin.js` is source, `plugin.min.js` generated; per-plugin `public/js/main.js` + `index.html` are tracked sources (see `leaui_image`) | `public/tinymce/plugins/leaui_image/plugin.js` |
| `messages/<locale>/` | i18n `.conf` sources consumed by the build into `public/js/i18n/*.js` | `messages/zh-cn/msg.conf` |
| `app/views/note/note-dev.html` | Editor page source; `note.html` is generated from it + manifest | — |
| `scripts/build/` | Manifest-driven build chain | `manifest.mjs` (single source of outputs), `index.mjs` (staging→publish pipeline) |
| `tests/js/` | Node contract tests (`node --test`, CJS with dynamic ESM imports) | `build-pipeline.test.js`, `release-contract.test.js` |
| `tests/e2e/` | Playwright suites: `build/` (build-smoke), `business/` (business + browser-smoke 4-ID suites) | `tests/e2e/business/leaui-image-iframe.spec.mjs` |

## Feature Organization

- A page feature = view template + `public/js/app/<page>.js` module + shared `common.js` helpers; page state lives in window-level namespace objects (`Note`, `Notebook`, ...), not modules.
- First-party editor plugins keep everything inside their plugin directory (`plugin.js`, `index.html` dialog, `public/js/main.js`, `public/css/style.css`) and communicate cross-iframe via `window.parent.postMessage({mceAction: ...})` — see `leaui_image`.
- Adding a generated asset: declare it in `scripts/build/manifest.mjs` (output + inputs + url) — the build fails closed on undeclared outputs; never hand-edit what the manifest produces.

## Naming Conventions

- Generated bundles keep their legacy public URLs (`/js/jquery-1.9.0.min.js`, `/js/app.min.js`) — URL compatibility is a hard contract.
- Editable sources use `-source` suffix (`tinymce-config-source.js`); i18n keys live in `messages/<locale>/*.conf`.
- E2e spec files map 1:1 to the four stable browser-smoke coverage IDs (`business-flows`, `editor-flows`, `bootstrap-components`, `leaui-image-iframe`).
