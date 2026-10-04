# Bootstrap 5.3 升级（E-BS）— 执行计划

## Global Constraints

- 目标固定 `bootstrap` 5.3.8；只从 npm lockfile 和 manifest 生成资源，不使用 CDN、全局包或联网 fallback。
- 保持服务端渲染、现有引用 URL、URL/API、文案、信息架构、博客主题加载、跨 iframe 数据和存量笔记 HTML；不升级 TinyMCE core，不保留 Bootstrap 3 adapter，不做视觉重设计。
- 所有改动必须落在本任务的 Bootstrap 资源、模板、第一方调用/插件、内置博客主题、leaui_image UI、测试和文档边界内。生成的 `note.html` 只能由 `npm run build` 更新。

## Task 1: Freeze Inventory And Baseline

**Files:**
- Create: `.trellis/tasks/08-25-bootstrap-upgrade/research/bootstrap3-usage.md`
- Read: `app/views/`、`public/blog/themes/{default,elegant,nav_fixed}`、`public/js`、`public/admin`、`public/member`、`public/album`、`public/md`、`public/tinymce/plugins/{image,leaui_image}`
- Read: `scripts/build/manifest.mjs`、`public/js/main.js`、`app/controllers/BlogController.go`、D/E task artifacts

- [ ] Record exact version headers and SHA-256 for every Bootstrap core duplicate and iframe font: main 3.0.2, iframe 3.0.3, admin 3.2.0 and all min/theme copies; each row includes owner, provenance, runtime/non-runtime status, disposition and regression ID.
- [ ] Record every referenced CSS/JS URL and page/iframe, including BlogController-provided URLs, admin/member direct links, `note-dev`/generated `note`, album, PDF, `public/tinymce/plugins/image/dialog.htm` and `leaui_image/index.html` relative paths.
- [ ] Record every Bootstrap-owned data attribute/class/API call, every custom `data-toggle`/`data-target` exception, `BootstrapDialog`, hover dropdown, RequireJS alias, generated bundle input, CodeKit historical config and the existing JQMIGRATE URL allowlist with owner and planned disposition.
- [ ] Capture baseline Chromium component/business smoke and local desktop/narrow screenshots with sanitized data; no credentials, cookies, headers, page content or screenshots in CI artifacts.

## Task 2: Add The Single 5.3.8 Asset Contract

**Files:**
- Modify: `package.json`, `package-lock.json`, `scripts/build/manifest.mjs`, manifest/build contract tests and `tests/e2e/business/jquery-diagnostics.spec.mjs`
- Generate: `public/css/bootstrap.css`, `public/css/bootstrap-min.css`, `public/js/bootstrap.js`, `public/js/bootstrap-min.js`
- Rebuild: `public/js/dep.min.js`, `public/album/js/main.all.js`, other declared outputs as required by manifest
- Remove only after proof: old `bootstrap.min.css`, `bootstrap-theme*.css`, `bootstrap.min.js` and unreferenced duplicate directories/files

- [ ] Lock `bootstrap` 5.3.8 and use `bootstrap.bundle` (Popper included) as the JS input. Update output count, input existence, output uniqueness, Git tracking, URL mapping and zero-diff tests.
- [ ] Preserve all currently referenced canonical URLs with 5.3.8 bytes. Replace admin/member references to `/public/admin/css/bootstrap.3.2.0.min.css` and iframe relative `public/bootstrap3` references with canonical URLs.
- [ ] Make `dep` and `album` consume the npm Bootstrap input directly, with one Bootstrap core per document; prove no generated-output recursion and no Bootstrap 3 bytes in bundles.
- [ ] Verify `npm ls bootstrap`, lockfile provenance/license and static URL status before template migration.
- [ ] Remove `/js/bootstrap.js` and `/tinymce/plugins/leaui_image/public/bootstrap3/js/bootstrap.min.js` from the JQMIGRATE diagnostic allowlist; add a fail-closed assertion that any Bootstrap 3 URL, `public/bootstrap3` request or Bootstrap 3 signature is an owned-offender failure.

## Task 3: Migrate Markup, Styles And Shared APIs

**Files:**
- Modify: `app/views/` (including `note-dev.html`), `public/css`, `public/admin/css`, `public/member/css`, `public/album/css`, `public/js/common.js`, `public/js/plugins/{tips,history}.js`, `public/js/app`, `public/md/main-v2.js`, `public/album/js/main.js`, `public/tinymce/plugins/image/{dialog.htm,js/dialog.js}`, admin/member/blog inline scripts
- Modify: `public/js/main.js` and `public/js/bootstrap-hover-dropdown.js` as needed

- [ ] Apply the inventory mapping for Bootstrap `data-bs-*`, grid, float, responsive visibility, forms, input groups, buttons, panels/wells, navbar, close controls and icons. Preserve separately registered custom attributes.
- [ ] Replace all direct jQuery component calls with Bootstrap 5 native instances and preserve event/focus/backdrop/destroy semantics; add a single loading helper for all former `.button('loading'/'reset')` call sites.
- [ ] Replace `showDialogRemote`'s removed `remote` option with encoded request, success injection and visible failure cleanup; test both success and failure.
- [ ] Keep generated and source templates synchronized and remove only stale Bootstrap 3 selectors that are proven unused.

## Task 4: Migrate Built-In Blog And Third-Party Plugins

**Files:**
- Modify: `public/blog/themes/{default,elegant,nav_fixed}` and their shared JS/CSS
- Modify or replace: `public/blog/js/bootstrap-hover-dropdown.js`, `public/js/bootstrap-hover-dropdown.js`, `public/blog/js/bootstrap-dialog.min.js`, `public/js/bootstrap-dialog.min.js`
- Test: `public/js/app/blog/*.js`, `public/blog/js/share_comment.js`

- [ ] Migrate built-in theme navbar/collapse/dropdown markup and preserve `BootstrapDialog`/hover behavior used by comments, report, login prompt, QR code and share flows.
- [ ] Prefer an auditable Bootstrap 5 upstream for `BootstrapDialog`; otherwise replace the min-only runtime with a first-party equivalent and remove both duplicate old copies. Do not patch minified bytes without source/provenance.
- [ ] Preserve user-uploaded theme bytes and path/injection contract. Prove built-in themes have no resource 404 and unknown custom theme classes are not silently rewritten.

## Task 5: Migrate leaui_image

**Files:**
- Modify: `public/tinymce/plugins/leaui_image/index.html`, `public/tinymce/plugins/leaui_image/public/js/*.js`, `public/tinymce/plugins/leaui_image/public/css/*.css`
- Remove: `public/tinymce/plugins/leaui_image/public/bootstrap3/`

- [ ] Migrate tabs, alerts, forms, progress, close controls, layout and icons; use canonical shared Bootstrap/jQuery URLs and the actual `index.html` path.
- [ ] Exercise album creation/rename/delete, pagination, real upload callback, URL image validation, title/width/height/constrain editing, selected-image insertion and cleanup.
- [ ] Assert `top.LEAUI_DATAS`, `parent.GlobalConfigs`, `parent.getMsg`, `mdGetImgSrc` and cross-iframe insertion remain unchanged, with no second core loaded.

## Task 6: Verification And Rollback Gate

- [ ] Add `tests/e2e/business/bootstrap-components.spec.js` or `.mjs` under the existing `business` project; cover component events, custom attribute exceptions, all page families, blog themes, button loading/error, remote modal and iframe data flow.
- [ ] Run focused static contract tests (including forbidden old URLs and the tightened JQMIGRATE allowlist), `npm ci && npm run build && npm test`, component/business E2E, Golden/USN/page smoke and resource 404 checks.
- [ ] In the dirty implementation worktree, hash/snapshot all manifest outputs after the first build, rerun the build, and compare the second snapshot to the first; do not compare expected task changes directly to `HEAD`.
- [ ] In a clean CI checkout, run `git diff --exit-code` and `git status --porcelain --untracked-files=all` after `npm ci && npm run build`; both must be zero.
- [ ] Run controlled HTTP/network failures for each former direct component/AJAX path and assert visible error, loading restoration and no leftover backdrop or event duplication.
- [ ] Record real Chrome, Edge, Firefox and Safari current/previous-major smoke in `docs/modernization/browser-smoke/bootstrap-5.3.md` under the shared sanitized record contract; Chromium must remain the PR/push blocker.
- [ ] Before completion, verify no Bootstrap 3 core bytes/URL references remain except inventory-proofed non-runtime historical files, no TinyMCE core/URL/API/backend/DB changes occurred, and no generated output drift exists.

## Rollback Points

- Asset contract, shared component APIs, blog/plugin layer and leaui_image layer are separate review points. A failed equivalence must return to planning instead of adding a hidden adapter or fallback.
- Rollback restores the previous committed Bootstrap 3 resources/templates, then reruns build, zero-diff, resource and business gates. It does not alter database data, backend APIs or user-uploaded themes.
