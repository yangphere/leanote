# 前端构建链现代化（D）— 技术设计

## 1. 设计目标与边界

构建链只替换 Gulp 的生成机制，不能成为前端库升级或第二套运行时。`manifest.mjs` 定义全部输入、输出和变换；Node 构建模块消费 manifest，先完成 staging 验证再发布受跟踪产物。普通生产命令没有“尽力生成”分支，任何未声明输入、过期 marker 或不完整 i18n 都是可诊断失败。

```text
tracked source + messages + note-dev.html
                 |
          manifest validation
                 |
    JS / CSS / i18n / note generators
                 |
        complete staging tree + contracts
                 |
     atomic per-file publication to tracked paths
                 |
  npm test + CI diff/status + browser/G regression evidence
```

构建资源的浏览器证据由同一仓库提供一个窄入口：

```text
G-compatible Mongo/service harness + explicit env
                 |
 npm run test:e2e:build (Playwright 1.62.1, Chromium only)
                 |
  sanitized resource/page summary; local-only trace on failure
```

Playwright smoke 进程本身不负责启动 MongoDB 或 Revel 服务，也不写入业务数据；CI 的 `node-tests` job 在同一 job 中启动并等待 G-compatible Mongo/service harness，结束时无条件清理。该入口不承担 E 的完整业务 E2E；E 复用其配置、依赖和浏览器安装方式。

`public/tinymce` 的核心、插件和上游语言别名不流经此图。D 只生成其 7 个 locale 文件；TinyMCE 8 的重新打包由 E-TM 重新设计和验收。

## 2. Manifest Contract

`scripts/build/manifest.mjs` 导出一个不可变的 plain-data manifest。每条输入和输出必须是根相对 POSIX 路径，入口在加载时拒绝绝对路径、空路径、`..`、重复的 canonical output 和输出落在非允许目录的条目。

`index.mjs` 的第一项运行时检查是 Node major version：仅 24 通过，低于 24 或高于 24 都在读取任何输入前带检测版本失败。`package.json.engines` 是包管理器提示，不是这一验证的替代品。

| Kind | Canonical inputs, in order | Output |
| --- | --- | --- |
| dep JS | `public/js/jquery-1.9.0.min.js`, `public/js/jquery.ztree.all-3.5-min.js`, `public/js/jQuery-slimScroll-1.3.0/jquery.slimscroll-min.js`, `public/js/contextmenu/jquery.contextmenu-min.js`, `public/js/bootstrap-min.js`, `public/js/object_id.js` | `public/js/dep.min.js` |
| app JS | `public/js/common.js`, `public/js/app/note.js`, `public/js/app/page.js`, `public/js/app/tag.js`, `public/js/app/notebook.js`, `public/js/app/share.js` | `public/js/app.min.js` |
| plugin JS | `public/js/plugins/note_info.js`, `public/js/plugins/tips.js`, `public/js/plugins/history.js`, `public/js/plugins/attachment_upload.js`, `public/js/plugins/editor_drop_paste.js`, `public/js/plugins/main.js`, `public/js/plugins/libs-min/fileupload.js` | `public/js/plugins/main.min.js` |
| markdown JS | `public/js/require.js`, `public/md/main-v2.min.js` | `public/js/markdown-v2.min.js` |
| album JS | `public/js/jquery-1.9.0.min.js`, `public/js/bootstrap-min.js`, `public/js/plugins/libs-min/fileupload.js`, `public/js/jquery.pagination.js`, `public/album/js/main.js` | `public/album/js/main.all.js` |
| album CSS | `public/album/css/style.css` | `public/album/css/style-min.css` |
| shared CSS | `public/css/bootstrap.css`, `public/css/font-awesome-4.2.0/css/font-awesome.css`, `public/css/zTreeStyle/zTreeStyle.css`, `public/md/themes/default.css`, `public/js/contextmenu/css/contextmenu.css` | the five existing `*-min.css` paths in R-D2 |
| i18n | declared scan roots plus `messages/<locale>/{msg,member,markdown,album,blog,tinymce_editor}.conf` | 14 browser bundles + 7 locale TinyMCE files |
| note HTML | `app/views/note/note-dev.html` | `app/views/note/note.html` |

The table is the reviewable contract; the expanded paths and output list live only in manifest. `scripts/build/js.mjs`, `css.mjs`, `i18n.mjs`, `note-html.mjs` and `index.mjs` import it rather than defining another list. Test-only output redirection receives the same manifest and stages under its supplied root; it does not change input identity or omit an output.

The manifest also records the URL used by the existing routes/templates: ordinary JS and i18n use `/js/...`, the plugin bundle uses `/public/js/plugins/main.min.js`, album assets use `/public/album/...`, Bootstrap/Font Awesome/zTree CSS use `/css/...`, contextmenu CSS uses `/js/contextmenu/...`, the retained Markdown CSS uses `/public/md/themes/default-min.css`, and locale language files use `/tinymce/langs/...`. `app/views/note/note.html` is a rendered template verified through `/note`, not a 33rd static URL; the current template's `/public/md/themes/default.css` reference is intentionally not rewritten by D. The resource smoke must use these URLs rather than deriving a URL by simply removing the `public/` prefix.

## 3. JavaScript And CSS Generation

`dep.min.js` is the legacy exception: read inputs in order and concatenate without a minification pass, matching the old task's absence of `uglify`. For `app.min.js`, `plugins/main.min.js`, `markdown-v2.min.js` and `album/js/main.all.js`, transform/minify each input separately with the locally installed esbuild 0.28.2, then concatenate the transformed results in manifest order. No mode may enable bundling, ESM conversion, wrapper, banner, footer or source map. This preserves one global scope and the old production ordering; in particular, `app.min.js` remains `note -> page -> tag -> notebook -> share` and is not reordered to match the differing direct dev scripts in `note-dev.html`.

CSS is transformed only for minification. It emits no source map and must preserve URL tokens, selectors and file destinations. The implementation must not infer all `*.css` files: only the manifest five shared CSS outputs plus album CSS are owned. This prevents a build run from silently changing user themes or third-party assets.

Tests parse output JavaScript, verify the manifest sequence and required global/load anchors, verify CSS output destinations and assert that no undeclared file is published. They test a disposable output root so `npm test` cannot modify tracked production assets.

## 4. Template Transformation

`note-html.mjs` uses explicit marker-aware operations, never a broad file-wide substitute:

1. require exactly three complete `<!-- dev --> ... <!-- /dev -->` blocks and remove all three;
2. require exactly one each of `pro_dep_js`, `pro_app_js`, `pro_markdown_js`, `pro_tinymce_init_js` and replace each with the historical production text;
3. replace the single TinyMCE script URL and plugin main script URL in their script attributes only;
4. remove the sole `console.log(o);`; treat `console.trace(o);` as the legacy Gulp no-op it is in the current source, rather than requiring a nonexistent match;
5. reject remaining development/production markers, unexpected count, reordered production scripts, or any invalid output.

The generated file is UTF-8 without BOM and LF-terminated. Its fixture validates byte-stable repeat builds and a structural snapshot: all Revel expressions and every source segment outside the enumerated operations must remain semantically unchanged. Deterministic line-ending conversion and removal of trailing whitespace/space-before-tab are the only additional normalization, required so the generated tracked file passes `git diff --check`.

## 5. i18n Compatibility

The generator scans the legacy roots and only `.js`/`.html`, excluding all 33 D outputs and `manifest.i18n.derivedInputExclusions`. `public/md/main-v2.min.js` is a tracked legacy compression derivative consumed by the markdown bundle; its `getMsg(e)` is covered by canonical `public/md/main-v2.js` and must never be scanned as a third dynamic-call source. A static `getMsg` use means a quoted literal first argument (single or double quote, with at most one optional second data argument); a static inline `msg:` use means a quoted literal property value. Revel `{{msg ...}}` template expressions and other dynamic values are not client keys. Every unsupported/dynamic form fails unless it is one of the two explicitly registered canonical locations (`public/js/common.js:1158` and `public/md/main-v2.js:17417`), recording source path, line and column. The manifest records the exact derived-input exclusion list (including `public/md/main-v2.min.js`) and tests assert that excluded files are not visited. It reads the six owned `.conf` files per locale and writes deterministic, key-sorted `MSG` or `tinymce.addI18n` payloads. The browser `msg` namespace merge order is `msg`, `member`, `markdown`, `album` with later definitions winning; blog and TinyMCE language maps are independent.

Before implementation changes the generator, an immutable fixture is captured from tracked outputs at `tests/js/fixtures/build/i18n-contract.json`. It records the supported locale list, every static key with namespace/source/line, the registered dynamic-call locator, each locale's parsed `msg`, `blog` and TinyMCE map, and historical missing-key sets. Tests compare objects, not minified bytes, so deterministic sorting is permitted while translation values, key coverage and fallback behavior remain fixed.

The established dynamic calls have explicit source locators in the manifest/fixture. They are not guessed from all messages, and the build never compensates by emitting every message key. A new dynamic location or a newly missing static key fails with file, line, column, locale and key. Existing missing values are baseline data: omission from `MSG` intentionally preserves the browser's original-key fallback.

## 6. Publication And Failure Semantics

`index.mjs` validates the complete manifest and all sources before it permits writes. The Node-version guard is a pure function accepting a version string so 23/24/25 tests do not mutate read-only `process.versions`; the CLI performs that guard before importing manifest data or reading repository inputs. Generators write only beneath a newly created repository-local staging directory. They must finish output existence, uniqueness, syntax/structure and i18n/template contract checks before publication begins.

Publication uses a same-volume backup-and-rename transaction: move each existing declared output into a staging backup, rename the validated staged file into place, and on any failure remove newly published files and restore every backup before returning non-zero. A rollback failure is reported together with the original error. It never reports success, skips the file, emits an empty bundle or takes a stale output as evidence. Tests inject input absence, duplicate output, path escape, parse failure, missing translation, new dynamic key, malformed note marker and publish failure to assert diagnostics, cleanup and restoration of the complete previous output set.

After a successful default build, every declared output must be Git tracked. CI checks both tracked-file content (`git diff --exit-code`) and untracked leakage (`git status --porcelain --untracked-files=all`). This is required because `git diff` alone cannot detect a newly created ignored/untracked output.

## 7. CI, Runtime Evidence And Rollback

The existing `node-tests` job becomes, in order: checkout, Node 24 and Go/Revel setup, MongoDB 5.0 fixture restore, Leanote service start/readiness, `npm ci`, `npm run build`, tracked/untracked drift gate, `npm test`, Chromium install, build smoke, and unconditional harness cleanup. It remains D's focused proof; F may later compose it with release gates but must preserve its behavior.

D's runtime proof starts the existing service with the G-compatible Mongo fixture and invokes `npm run test:e2e:build` after `npm ci` and the explicit local `npm exec -- playwright install chromium` step. The runner must provide `LEANOTE_BASE_URL`, `LEANOTE_E2E_EMAIL` and `LEANOTE_E2E_PASSWORD`; the test fails closed when any prerequisite is missing rather than starting a second service or choosing credentials. CI builds the Revel CLI from the repository module graph with `GOTOOLCHAIN=local` (never `go install ...@v1.0.3`) and terminates the service process group during unconditional cleanup. The `build-smoke` Chromium project logs in, visits `/note`, `/album/index`, `/blog`, `/admin/index` and `/member/index`, verifies markdown assets through `/note`, requests all 32 static manifest URLs without following redirects, and records the generated `note.html` check, HTTP 200 results, console/page errors, unhandled rejections and owned-resource 4xx/5xx. It performs no note, upload, theme or production-output mutation. Its only CI artifact is a sanitized summary containing tool versions, page URLs, resource status and error categories; it excludes page text/HTML, screenshots/video, request/response headers and bodies, cookies, auth tokens and storage state. Raw trace, HTML report and service logs remain local or are deleted after redaction. The report complements, but does not replace, G Golden/USN/page smoke or E's full business Playwright suite; E reuses this package/configuration and the `business` project for Chromium business paths, while Firefox/Safari release smoke remains an E/F gate.

## 8. Playwright Smoke Contract

`package.json` adds the exact dev dependency `@playwright/test` 1.62.1 and the script `test:e2e:build` invoking `playwright test --config=playwright.config.mjs --project=build-smoke`. `playwright.config.mjs` declares two named Chromium projects over one configuration: `build-smoke` uses `testDir: tests/e2e/build`, `testMatch: **/build-resource-smoke.spec.mjs`; `business` uses `testDir: tests/e2e/business` and `testMatch: **/*.spec.{js,mjs}` for E-owned tests. D supplies the project and selection contract but no business test files. E adds `test:e2e` as `playwright test --config=playwright.config.mjs --project=business`; it must not create a second config or dependency. Neither project configures a `webServer` fallback: the caller owns the G-compatible MongoDB 5.0 fixture and service lifecycle, and `LEANOTE_BASE_URL` is required.

The smoke test reads credentials only from `LEANOTE_E2E_EMAIL` and `LEANOTE_E2E_PASSWORD`; no credential is committed or defaulted. It performs the login/session step and read-only page navigation, derives the 32 static URLs from the manifest instead of copying another list, and treats only manifest-owned resources as network assertions. Every page attaches listeners for `console.error`, `pageerror` and unhandled promise rejection. A missing browser binary, missing environment, non-2xx owned resource, generated-template marker mismatch or service connection failure is a non-zero test result. It emits only an allowlisted summary (versions, URLs, status codes, resource paths and error categories); it never records storage state, cookies, authorization headers, request/response bodies, page HTML/text, screenshots or video.

The browser install command is explicit and local (`npm exec -- playwright install chromium`) after `npm ci`; it is not part of `npm run build` and is never satisfied by a global binary. CI may run this command in a job that has already started the G-compatible harness, but it uploads only the sanitized summary and a separately redacted service-health summary with a retention limit of 7 days. Raw Playwright trace/HTML report, screenshots/video, storage state and unredacted service logs are deleted and never published. F owns eventual cross-browser orchestration; D's workflow change only documents and validates this reusable contract.

Until all checks pass, the old file and dependencies stay available only as a comparison reference. Once passed, the change is one independently revertible task commit containing the lockfile, build scripts/tests, regenerated declared outputs, Gulp removal, CI adjustment and documentation updates. Reverting that commit restores the prior generated assets and Gulp source together; no partial hand-edited bundle rollback is valid.
