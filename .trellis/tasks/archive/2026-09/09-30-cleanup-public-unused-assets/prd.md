# 清理 public 无用前端库与静态资源

## 目标

技术栈重构后（Go `net/http` 服务端 + Node 24 清单式构建，jQuery 3.7.1 / Bootstrap 5.3.8 / TinyMCE 8.8.2 由 `node_modules` 发布），`public/` 下残留了大量不再被任何视图、博客主题、Go 代码或构建清单引用的第三方库文件与静态资源。本任务删除这些文件，并同步受影响的契约夹具与测试，不改变任何仍在使用的公开 URL。

## 背景与已确认事实

- 静态资源入口只有：`app/views/**` 模板、`public/blog/themes/{default,elegant,nav_fixed}`、`app/controllers` 中硬编码的资源 URL、`scripts/build/manifest.mjs` 的输入与输出。
- MathJax 只通过 `libs/MathJax/MathJax.js?config=TeX-AMS_HTML` 加载（`public/md/main-v2.js` 与 `public/libs/md2html/md2html_for_export.js`）；配置为 `webFont: "TeX"`、`availableFonts: ["STIX","TeX"]`、`showFontMenu: false`，只存在 HTML-CSS 输出。
- ace 只被 `app/views/note/note*.html` 与 `app/views/member/blog/update_theme.html` 加载；主题固定为 `ace/theme/tomorrow`，没有加载任何扩展、键位或片段。代码块语言来自笔记内容里的 `brush:xxx`，可能是任意语言，因此 `mode-*.js` 必须保留。笔记页关闭 worker，主题编辑页只用 html / css / javascript。`ace.js` 会按需懒加载 `ext-searchbox`、`ext-error_marker`。
- Markdown 编辑器只使用 v2（`public/md/main-v2.js`、`main-v2.min.js`、生成的 `public/js/markdown-v2.min.js`）。
- `public/tinymce/` 几乎全部由构建清单登记，不在本次删除范围。
- `.less` 源文件与 `config.codekit` 被 `tests/js/` 契约测试直接读取，必须保留。
- `public/md`、`public/js`、`public/libs` 是 i18n 扫描根；`tests/js/fixtures/build/i18n-contract.json` 按 `namespace:key:path:line` 记录每个静态文案键，删除被扫描的文件后夹具必须同步。

## 需求

1. 删除 MathJax 多余文件：
   - `public/libs/MathJax/config/` 下除 `TeX-AMS_HTML.js` 外的全部文件（含 `local/`）。
   - `public/libs/MathJax/fonts/HTML-CSS/` 与 `public/libs/MathJax/jax/output/HTML-CSS/fonts/` 下的 `Asana-Math`、`Gyre-Pagella`、`Gyre-Termes`、`Latin-Modern`、`Neo-Euler`、`STIX-Web`。保留 `TeX` 与 `STIX`。
2. 删除 Markdown v1：`public/md/main.js`、`public/md/main.min.js`、`public/md/themes/default-v1.less`、`public/js/markdown.min.js`。
3. 删除 ace 未使用部分：`ace-old.js`、`snippets/`、`keybinding-*.js`、除 `theme-tomorrow.js` 外的 `theme-*.js`、除 `ext-searchbox.js` 与 `ext-error_marker.js` 外的 `ext-*.js`，以及 `worker-coffee.js`、`worker-json.js`、`worker-lua.js`、`worker-php.js`、`worker-xml.js`、`worker-xquery.js`。保留 `ace.js`、`ace-modify.txt`、全部 `mode-*.js`、`worker-html.js`、`worker-css.js`、`worker-javascript.js`。
4. 删除 `public/js` 遗留脚本：`main.js`、`other.js`、`jquery-cookie.js`、`jquery-cookie-min.js`、`jsrender.js`、`jquery.qrcode.min.js`、`google-code-prettify/lang-*.js`、`google-code-prettify/run_prettify.js`、`google-code-prettify/mine.css`、`jQuery-slimScroll-1.3.0/jquery.slimscroll.min.js`、`jQuery-slimScroll-1.3.0/slimScroll修改`。
5. 删除无引用的零散图片与样式（逐个复核无引用后删除）：`css/leanote-font.css`、`css/theme/css/font.css`、`images/home/{mobile,preview2,preview3}.png`、`images/leanote/leanote_alipay.jpg`、`images/loading-a-20-2.gif`、`images/loading-a-24.gif`、`images/logo/{leanote-blue,leanote-old,leanote,leanote_icon_blue-old}.png`、`images/slider/v2/mobile_tinymce.png`、`css/zTreeStyle/img/{left_menuForOutLook,line_conn,zTreeStandard}.gif`。
6. 同步契约：更新 `tests/js/fixtures/build/i18n-contract.json` 中指向已删除文件的键位置；更新 `tests/js/jquery-asset-contract.test.js` 中点名已删除文件的列表。
7. 被丢弃的 i18n 键必须逐个确认在剩余源码（含被排除扫描的 `main-v2.min.js`）中没有动态使用。

## 验收标准

- [x] 需求 1–5 列出的文件均已从工作区和索引中移除，保留项仍在。
- [x] `npm run build` 成功，且除 i18n 产物因键集合变化产生的预期差异外，没有其它生成物漂移。
- [x] `npm test` 全部通过（与清理前基线对比，无新增失败）。
- [x] `GOTOOLCHAIN=local go build ./...` 与 `go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote` 通过。
- [x] `git diff --check` 通过。
- [x] 对剩余源码再次做引用扫描，确认没有指向已删除文件的引用。
- [x] 真实浏览器验收（编辑器公式渲染、代码块、博客文章页）未执行时明确记录为未运行。

## 验证证据

- 删除 613 个受管文件，`public/` 从 47M 降到 25M，受管文件 1666 → 1053。
- `npm run build`：成功，生成物零漂移（`git status` 中除删除项外只有夹具和测试两处修改）。
- `npm test`：209 tests，208 passed，1 skipped，0 failed，与清理前基线一致。
- `GOTOOLCHAIN=local go build ./...`：通过；`go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote`：通过。
- `git diff --check` 与 `git diff --cached --check`：通过。
- 无头 Chromium 静态冒烟（临时脚本，仅托管 `public/`）：MathJax 以 `TeX-AMS_HTML` 渲染 2 个公式，字体为 TeX，ace 加载 tomorrow 主题与 javascript/html/css/python 模式、三个保留的 worker 及 `ext-searchbox`，0 个 404，0 个控制台错误。该冒烟最初发现 `config/TeX-AMS_HTML.js` 被误删，已恢复。
- 经真实服务端（MongoDB + 登录）的编辑器、博客文章页、PDF 导出验收：未运行。

## 实施中的偏离

- 保留 `public/js/app/blog/`：它被 `tests/js/jquery-asset-contract.test.js` 列为第一方源码，且删除会从 `blog.<locale>.js` 丢掉 18 个文案键，改变已发布的生成物。保留后 i18n 产物完全不变，夹具只删除了指向 Markdown v1 文件的 71 条位置记录。
- 保留 `leaui_mindmap` 下的两个 `.css.map`：`default.all.css` 仍带有指向它的 sourceMappingURL。

## 范围外

- 不调整 `public/tinymce/` 的清单登记内容，不改构建清单的输入输出。
- 不删除任何 `.less` 源文件、`config.codekit`、`ace` 的 `mode-*.js`、`libs/uml`、`libs/md2html`、`admin/js/artDialog`。
- 不修复管理后台视图中早已失效的 `/public/admin/js/ie/*.js` 引用。
- 不提交，由用户审阅后决定。

## 已确认的决策

- 删除范围：用户选择“安全档 + ace worker”。
- 残余风险：用户自行上传的旧博客主题若硬编码了 `/js/jsrender.js`、`/js/jquery-cookie.js`、`/js/jquery.qrcode.min.js` 这类根路径脚本，会得到 404；内置三套主题使用的是 `public/blog/js/` 下的副本，不受影响。
