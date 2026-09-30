# 清理未使用的视图模板、Go 包与本地残留

## 目标

在 `public/` 清理之后，继续删除项目里其它不再使用的文件：没有任何渲染入口的视图模板及其专属静态资源、未被导入的 Go 包、路径早已失效的 IE 兼容脚本，以及未受 git 管理的本地残留目录。

## 背景与已确认事实

- 视图只通过 `RenderTemplate` 的字面量名字、`app/controllers/admin/httpserver.go` 的模板白名单（`/admin/t?t=`）、以及 `admin/setting/<action>.html` 这类有限拼接来渲染；博客页走主题目录，不走 `app/views`。
- `app/httpserver/templates_test.go` 断言 `errors/404.html` 与 `errors/500.html` 存在，二者保留。
- PDF 导出由 `service.ContentPDF`（`app/application/content/pdf.go`）直接生成，不渲染 `file/pdf.html`。
- `app/views/admin/top.html` 与 `app/views/member/top.html` 引用 `/public/admin/js/ie/*.js`，实际文件在 `public/admin/js/` 下，该路径一直是 404。
- `app/views` 是 i18n 扫描根，删除含文案键的视图需要同步 `tests/js/fixtures/build/i18n-contract.json`。

## 需求

1. 删除无渲染入口的视图：`admin/button.html`、`errors/500-blog.html`、`errors/500-dev.html`、`errors/404-dev.html`、`file/image.html`、`file/pdf.html`、`home/modal.html`、`member/user/add_account.html`、`note/histories.html`、`share/note_notebook_share_user_infos.html`。
2. 删除只被 `file/pdf.html` 引用的静态资源：`public/libs/md2html/md2html_for_export.js`、`public/css/pdf.css`、`public/css/pdf.less`、`public/images/blog/tag.png`、`public/images/logo/leanote_icon_blue.png`。
3. 删除未被导入的 Go 包 `app/lea/netutil/`。
4. 删除 `public/admin/js/` 下的 `excanvas.js`、`html5shiv.js`、`respond.min.js`，并移除两个 `top.html` 中对应的 IE 条件注释。
5. 同步 i18n 契约夹具；若有文案键因此不再被任何源码使用，逐个确认后接受生成物变化。
6. 清理未受管的本地残留：`none`、`git_repo/`、`dist/`、`ci-summaries/`，以及空目录 `app/cmd/`、`app/lea/binder`、`app/lea/blog`、`app/lea/html2image`。

## 验收标准

- [x] 需求 1–4 的文件已从工作区和索引移除。
- [ ] `gofmt -l app cmd` 无输出（未满足：列出 3 个本任务未改动的既有文件，见验证证据）。
- [x] `GOTOOLCHAIN=local go build ./...`、`go vet ./...` 通过。
- [x] `go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote` 通过。
- [x] `npm run build` 成功，生成物差异仅限于被确认的 i18n 键变化；`npm test` 无新增失败。
- [x] `git diff --check` 通过。
- [x] 真实服务端验收未执行时明确记录为未运行。

## 验证证据

- 删除 19 个受管文件（10 个视图、5 个 PDF 视图专属资源、`app/lea/netutil/NetUtil.go`、3 个 IE 兼容脚本），两个 `top.html` 各去掉 5 行 IE 条件注释。
- i18n 夹具删除 8 条指向 `member/user/add_account.html` 的位置记录，没有文案键被丢弃；`npm run build` 成功且生成物零漂移。
- `npm test`：209 tests，208 passed，1 skipped，0 failed。
- `GOTOOLCHAIN=local go build ./...`、`go vet ./...` 通过；`go test -count=1 ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote` 通过。
- `gofmt -l app cmd` 列出 `app/controllers/httpserver_inventory_test.go`、`app/tests/harness/server.go`、`app/tests/harness/server_test.go`，三者本任务未改动，属于既有状态。
- `git diff --check`：通过。
- 真实服务端（MongoDB + 登录）下的管理后台、会员中心页面验收：未运行。

## 实施中的偏离

- `.content-public-quarantine/` 与 `.content-temporary/` 未删除：它们是内容服务的运行时根目录（见 `app/service/content_runtime_test.go`），不是残留。

## 范围外

- 不删除 `errors/404.html`、`errors/500.html`，不动 `app/Index.html`、`notebook/Index.html`、`admin/blog/page.html`、`admin/email/page.html` 等未能确认的模板。
- 不动 `files/`、`.leanote-data/`、`.content-private-quarantine/` 等本地运行数据，也不动其它 AI 工具的本地配置目录。
- 不删除迁移期文档、`config.codekit`、`.less` 源文件。
