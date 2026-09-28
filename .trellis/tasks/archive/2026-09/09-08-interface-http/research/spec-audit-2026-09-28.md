# `interface-http` 当前状态复核（2026-09-28）

## 依据

本次复核以当前工作树代码、Git diff、可执行测试结果为准；`docs/CODEX_HANDOFF.md` 只作为待办线索。当前分支为 `dev`，任务仍为 `in_progress`，工作区未提交。

## 已收口的本轮缺口

- `app/controllers/api/httpserver_notes.go` 的 `ApiNote.AddNote` 已恢复完整 multipart 创建链：普通字段与 `Files[...]` 先绑定，`FileDatas[...]` 经过 `AttachService` 有界读取和校验，创建 receipt 冻结资产身份，预笔记资产发布后提交 note/content，内容链接替换为服务端文件 ID，成功后 finalize；note 未提交时只清理本次尝试的 request-owned 资产。
- 新增 `app/controllers/api/httpserver_notes_content_test.go`，覆盖图片/附件链接替换和相似本地 ID 不误替换。
- 新增 `app/tests/harness/api_note_add_multipart_test.go`，真实请求同时发送普通字段、`Files[0][...]` 元数据和 `FileDatas[local-attachment]` 文件，并验证响应字段、Mongo 内容链接和附件下载回读。

## 当前验证

- `gofmt` 已运行于本轮 Go 改动。
- `go test ./app/controllers/api -run 'TestRewriteAPINoteContentLinks|TestAPINoteCreateOperation' -count=1`：通过。
- `go test ./app/controllers/api -count=1`：通过。
- 请求边界修正：`Params.FormFile` 先执行 `ensureForm`，避免上传 action 直接先取文件时绕过请求体上限和普通 multipart 字段解析；`TestParamsFormFileParsesBoundedFormWhenCalledFirst` 通过。
- 复核后 `ApiNote.AddNote` 将成功上传的附件计入 `Note.AttachNum`，只保留 `HasBody=false` 的已有文件引用，并将最终资产及冻结摘要传给 note service；native replay 增加持久化 `AttachNum=1` 断言。
- `go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote -count=1`：通过；`go build ./...`、`go vet ./...`、`go test ./... -run '^$' -count=1` 均 exit 0。
- `npm test`：132 项，131 passed、1 skipped、0 failed；`python ./.trellis/scripts/task.py validate 09-08-interface-http`：通过（18 implement、14 check）；`git diff --check` exit 0。
- Docker Desktop Linux engine 已恢复（`29.8.0 linux/amd64`）；harness 自行启动并清理 `mongo:8.0`/`leanote-test-mongo`。
- `go test ./app/tests/harness -run 'TestNativeAPINoteAddMultipartReplay|TestClientMultipartAddNoteCarriesMetadataAndFilePartsTogether' -count=1 -timeout=90s`：通过；真实 replay 覆盖 multipart 字段/文件、`AttachNum=1`、Mongo 内容链接替换和附件下载回读。测试使用每轮新的 NoteId，并按 harness `OID_TOKEN` 规范化响应断言。
- `go test -p 1 ./app/tests/... -count=1 -timeout 30m`：通过；`app/tests`、`app/tests/harness` 与 harness cmd packages 均通过。
- `$env:LEANOTE_HTTP_INTEGRATION='1'; go test -p 1 ./app/tests/... -count=1 -timeout 30m`：通过；真实 `cmd/leanote -runMode test` listener 的 `GET /login` smoke 通过。

## 规格同步结论

- PRD 不再把 Blog 读取或 member 主题上传/导入写成当前未迁移桩；显式用户主题页面级 smoke、备份真实回放和下游交付证据仍保持未验收。
- `design.md` 已记录 AddNote/UpdateNote multipart 的绑定、receipt、预笔记、链接替换、reconcile/finalize 和失败清理边界。
- `acceptance/evidence-matrix.md` 已把 2026-09-28 当前工作树下的 Mongo 8 standalone replay、multipart focused replay 和真实 listener smoke 记录为当前证据；U-05 GET→405 Golden 的来源仍为归档 identity PRD §U-05，设计文档指定 `api/user/getSyncState_get.json`。
- `app/tests/README.md` 与 `docs/modernization/cicd-delivery.md` 已包含 native `cmd/leanote` 工作流，本轮无需添加重复说明。

## 未完成证据

本轮未提交或归档；浏览器、replica-set/failpoint、进程恢复、生产 exit-78、容器卷与非 root/restart、PDF、显式用户主题页面、备份真实回放和跨进程未就绪恢复仍需在具备相应环境时执行。
