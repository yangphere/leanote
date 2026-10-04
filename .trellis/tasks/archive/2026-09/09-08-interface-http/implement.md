# 接口适配层：标准库 HTTP 与路由 — 执行计划

每个批次独立提交、独立运行门禁、独立回滚（D-H1：不拆子任务）。D-H1～D-H7 已于 2026-09-25 确认，冻结结论见 `research/spec-audit-2026-09-25.md` §6。2026-09-26 复核新增 D-H8～D-H10（同日确认全部采用推荐）；E-1 旧生成链已在 B9 删除；2026-09-27 Docker/Mongo 8 fixture 已恢复，真实 replay 已执行；执行顺序 B2.0 → B1.5 → B2，见 `research/spec-audit-2026-09-26.md` §4；Revel 全局消费点的批次归属见 design §7。

## B0 盘点与对账骨架

- [x] 生成 `research/action-inventory.md`：`conf/routes` 95 条（显式/Static/catch-all）与当前代码导出的 242 个 `revel.Result` controller 方法，逐项标注 routable/非路由辅助、owner application 任务、当前 method、BEFORE/`commonUrl`、响应类型、Golden 覆盖。
- [x] 读取已归档交接矩阵：identity `acceptance/evidence-matrix.md`、persistence、notes `research/action-inventory.md`、content `research/action-inventory.md`、publishing/admin `research/action-contract-inventory.md`、domain `research/input-contracts.json`。
- [x] 增加 registry ↔ inventory 对账测试（当前迁移缺口按设计先失败，随批次转绿）；增加“导出未注册方法不可达”负例。

## B1 runtime 与 production-config

- [x] 生产 App 装配 `LocaleResolver`、SessionReader/SessionWriter；补 27 个模板函数名集冻结测试与 `ViewArgs` 框架键。
- [x] 抽取只读 `ProductionConfig`（Addr、ShutdownTimeout、DatabaseName、`ConfiguredDatabaseIdentity`、`CredentialProviderRef`）并提供 admin consumer fixture；admin 生产动作的实际消费留到 B6。
- [x] D-H4：解析 `content.private.data`/`content.private.quarantine`/`content.public.data`/`content.public.quarantine`/`content.temporary`/`admin.backup.root`，调用 content validator，补齐稳定错误码；`/upload/*` 与 `/public/upload/*` 改从 public data root 提供；删除 `initializeContentRuntime` 兼容映射。
- [ ] D-H4：补生产进程级 exit 78 负例，保留当前 in-process 配置错误测试作为前置证据。
- [x] D-H4：同步 `Dockerfile`（卷与 `/var/lib/leanote/tmp`）与 `docs/modernization/cicd-delivery.md`（新键、三卷布局、tarball 绝对根目录创建和旧 `/app/files`/`/app/public/upload` 一次性迁移步骤）；确认 `sh/package.sh` 只打包应用前缀，不嵌入 `/var/lib/leanote`。
- [x] D-H5：identity 方法矩阵 405 与 `Allow` 头；其他路由保持观察值的 route negative。

## B1.5 test run mode 前移（D-H8 已定；在 B2.0 之后执行）

- [x] `cmd/leanote -runMode test`：只读仓库 `conf/app.conf` `[test]`，强制 `leanote_test`，允许 localhost；与 prod 互斥负例（D-H2 的 test 部分）。
- [x] harness 改为 native-only：直接构建并启动 `cmd/leanote -runMode test`，只 replay registry 已注册 action 的 Golden 子集；不再生成或启动旧运行时入口。
- [x] 2026-09-27 曾在 Docker Desktop Linux daemon 可用时完成 Mongo 8 fixture replay；2026-09-28 在本轮工作树代码上重新完成 focused multipart replay、完整 harness 和真实 listener smoke。旧 E-1 生成链已删除，replica-set、kill/restart 和 failpoint 仍属于下游交付证据。

## B2.0 应用配置 seam（B2 前置）

- [x] `ConfigService` 的 `adminUsername`/`site.url` 改由第一方只读配置提供（prod 来自已校验 `/etc/leanote/app.conf`；dev/test 在 B1.5/B8 接入），移除 `InitGlobalConfigsWithError` 对框架全局配置的依赖。
  - 注记（2026-09-26）：seam 为 `service.AppConfigSource`（`String(key) (string, bool)`）+ `service.SetAppConfigSource`；`cmd/leanote` 在 registry 装配前注入已校验 `*httpserver.Config`。原启动钩子的 site.url 域名推导改为 `SetAppConfigSource` 内的 `applySiteURLDomain`（行为含端口怪癖原样保留，且可重复调用）。备份根和 `db.*` 读取已改走 `RuntimeConfig`，剩余真实 Mongo/进程证据仍由 B8/交付层验证。
- [x] D-H9：快照未加载前新入口未就绪（healthz 与非静态请求 503 `{"status":"not_ready"}`，静态照常，后台退避重试，成功后自动就绪）；principal policy 区分“快照未加载”与“未配置 demo/admin”。
  - 注记（2026-09-26）：`httpserver.App.Ready` 通用门控（未就绪时不运行 `OnRequest`/`HealthCheck`/session decode/action；未匹配路由同样 503）；`cmd/leanote/readiness.go` 的 `startupReadiness` 在 DB 未连通时重复 `db.InitWithError`（可重入：先 reset collection 全局、失败时 client 置 nil），连通后只重试 `InitGlobalConfigsWithError`，1s 起指数退避、上限 30s，随关停 context 取消；outbox worker 改为就绪后在同一后台 goroutine 启动（不再在空快照上投递）。policy 未加载时返回 `service.ErrGlobalConfigNotLoaded`（非 `ErrDemoConfiguration`）。
- [x] 真实进程启动证据：`startBaselineServer` 在 Mongo 8 fixture 恢复后启动 `cmd/leanote -runMode test`，`/healthz` 就绪，随后完成 API/网页回放；配置 seam 的 in-process 证据仍覆盖 admin role、`openRegister`、`siteUrl` 与 demo 策略。
- [ ] 真实进程在启动时 Mongo 不可达、healthz/非静态 503 后自动恢复的过程级证据仍未执行；该项保留给 delivery-verification。

## B2 identity/API 身份批

- [x] `httpserver.Params` 补 `Has`/`Bool`（Atob）/`Strings`/嵌套键/严格整数与 ObjectID 读取；严格字段按 PRD R2 的边界并在 inventory 标注来源。
- [x] `ApiUser` 五个 action 按 U-05 矩阵注册（`Info` GET，其余 POST）；显式 `userId` 不一致返回 `forbidden`/`not_authenticated`；`_token/_userId` 写回时序；显式 invalid token 不 fallback。
- [x] `ApiUser.UpdateLogo`：multipart 校验与 `UpdateAvatar` 失败映射；D-H10 存储经 content resolver/publish 写入 public data root，wire 不变，不新增 `Files` 行，失败回收文件。
- [x] 主站 `Auth`、`Captcha`、`Index`（含 `Suggestion` 可选 `submissionId` 原样传递）、`User`；补齐 web session principal、登录/注册轮换与注销清理、验证码 PNG、模板 ViewArgs、未登录普通/XHR 分支，并加入 native HTTP contract。Mongo 8 真实 replay 已覆盖 API Auth/User 与 web 登录页面；P-01/P-06/P-07/U-04 的跨任务完整矩阵仍由 identity 验收复核。

## B3 notes/content 批

- [x] 主站 `Note`、`Notebook`、`Tag`、`NoteContentHistory`；notes `OperationId`/`ExpectedUsn` binder 与 result mapper；`/note/deleteTrash` alias 保留。基础 action 已接入 native registry，`Notebook` 的父级/拖拽参数和 `Tags` 两种形态均保留。
- [x] 主站 `File`、`Attach`、`Album`；api `ApiNote`/`ApiNotebook`/`ApiTag` 余量/`ApiFile`。ApiNote `AddNote`/`UpdateNote` 均接入 `Files[...]` + `FileDatas[...]` 的稳定资产流程；2026-09-28 native harness focused replay 与两轮完整 harness 均通过。
- [x] D-H6：`GetImages`/`GetAlbums` 依赖失败 500 + legacy body 形状；当前有 native contract，Golden 失败 variant 仍受 Mongo/新入口 replay 阻塞。
- [x] D-H7：`CopyHttpImage` 保持现有 wire；在 `docs/modernization-backlog.md` 新增 MOD 条目。
- [x] `ApiFile.GetImage/GetAttach/GetAllAttachs` 按 U-05 GET-only 注册（405 + `Allow`）。
- [x] `AttachService` 的文件读取改走 content resolver；主站 adapter 的开发模式改为 run mode 注入。

## B4 publishing 批

- [x] 主站 `Blog`（含 8 处 JSONP、Q-P5 callback 过滤、Q-P9 `submissionId` 必填）、`Share`、`Preview` 已接入标准库 registry；native adapter 保留博客 JSON/JSONP、分享和 Preview 的模板/错误边界。
- [x] 主题写入路径已接入 `ProductionConfig.ContentRoots.PublicUpload.Data`，不再由新 adapter 写入应用树路径。
- [x] 博客读取侧改以 `notes.IsBlog` 为事实来源，内容查询按 `_id + UserId` 兼容旧 `note_contents` 缺少 `IsBlog` 的数据；`Distinct` 修复搜索 checked 读取。Mongo 8 fixture 已验证博客页面 `/blog` 返回 200，且内容缺字段时仍可读。
- [ ] 显式用户主题的 content resolver 页面级 smoke 仍未执行；`/preview` 无主题 session 的 404 已由真实页面 smoke 覆盖，不能替代用户主题页面证据。

## B5 member 批

- [x] `member/*` 全部 action 已按 `MemberX.Y` 前缀注册，并统一复用 web session principal 与 LoginRequired BEFORE；主题图片上传和主题导入已执行实际写入，导入大小上限为 10 MiB。
  - 注记：显式用户主题页面级 smoke 与真实 member multipart replay 仍未执行，保持为未验收证据，不把旧的“明确拒绝桩”描述留在当前状态。

## B6 admin 批

- [x] `admin/*` 全部 action 已注册，统一经过 admin principal，并由生产入口注入同一份 `ProductionConfig` consumer。
- [x] 新 `AdminData.Download` adapter 使用 `ProductionConfig.BackupRoot` 与 `DatabaseName`，并在归档前执行 containment、稳定路径与大小校验；旧 controller 路径已删除。
- [x] `ConfigService` 的备份/恢复路径与数据库连接读取已改走 `RuntimeConfig`/typed database provider（对应原 `ConfigService.go` 备份与 `db.*` 消费点），不得只改 controller；adapter contract 已覆盖 typed consumer，备份真实回放仍未执行。

## B7 汇合

- [x] 提取 `needValidateAPI` 与 `needValidateWhitelist` 的重复白名单 helper；registry 对账测试已全绿（249 action，`missing=0, extra=0`）。
- [x] 各批业务调用链中的旧全局配置、应用根路径和日志依赖已收敛到第一方 seam；保留的历史语义注释和测试名称不构成运行时依赖。

## B8 入口与 harness（D-H2、D-H3）

- [x] D-H2：`cmd/leanote` 已增加 dev/test run mode，读取仓库 `conf/app.conf` `[dev]`/`[test]`，test 强制 `leanote_test`；仓库 conf 补 content/backup 相对默认值并覆盖互斥负例。
- [x] harness 已提供 `cmd/leanote -runMode test` 目标；2026-09-27 记录了 Mongo 8 fixture 下的 Golden/USN/权限/admin/member/页面 smoke 全量 replay，2026-09-28 在当前代码上重新通过 standalone harness 及 `LEANOTE_HTTP_INTEGRATION=1` 真实 listener smoke。replica-set、kill/restart、failpoint 和容器交付验证仍不属于本批。
- [x] D-H3：`sh/run.sh` 已改为 `go run ./cmd/leanote -runMode dev`（无 watch）；CI summary 已统一使用 `native-entrypoint` 分类。

## B9 旧运行时删除与残留清扫

- [x] 删除 `app/init.go` 旧链、`app/lea/route`、`app/cmd/`、旧 controller 签名与 `revel.Result`；运行 `go mod tidy`，`go.mod`/`go.sum` 不再包含 `github.com/revel/*`。
- [x] `conf/app.conf(-default)` 移除 `module.testrunner` 等旧专用键；第一方解析器对部署文件中的未知键保持忽略。
- [ ] 页面级主题读取、真实进程退出码和下游交付材料继续由对应验收任务完成；当前任务不提交、归档或推送。

## Continuation evidence (2026-09-27)

本节是当前环境的最终证据；上方 2026-09-26 的 E-2 blocked 记录保留为历史审计轨迹，不覆盖本节已执行结果。

- 分页适配器 `pageParam`/`apiPage` 将缺省、非法、零值和负值统一为第 1 页，避免负数 Mongo skip 被旧错误忽略路径转成空响应；对应 controller/API 回归测试通过。
- `go test ./app/controllers ./app/controllers/api -count=1`：passed。
- `gofmt -l app cmd`：无输出；`go test ./app/controllers ./app/controllers/api ./app/controllers/member ./app/controllers/admin ./app/httpserver ./cmd/leanote -count=1`、`go test ./app/service -count=1`：passed。
- `go test ./... -run '^$' -count=1`、`go build ./...`、`go vet ./...`：passed。
- `go mod tidy`：passed；AC-H11 规定的 `rg -n -i 'revel' app cmd go.mod go.sum sh conf scripts .github Dockerfile` 当前零命中。
- `npm test`：132 项，131 passed、1 skipped、0 failed；在该 2026-09-27 checkpoint，`python ./.trellis/scripts/task.py validate 09-08-interface-http`：passed（17 implement、13 check）；`git diff --check`：passed（仅换行转换提示）。
- `docker version --format '{{.Server.Version}} {{.Server.Os}}/{{.Server.Arch}}'`：`29.8.0 linux/amd64`；harness 自行启动 `mongo:8.0`、恢复 `leanote_test` fixture，并在测试后清理容器。
- `go test -p 1 ./app/tests/... -count=1 -timeout 30m`：109 pass、0 fail、3 skip；skip 为旧 `TestAuth`、缺少已审阅 PDF golden、默认关闭的 real-server smoke。真实 `TestGoldenAPIActions`、`TestGoldenWebOwnershipControllers`、`TestWebAdminMemberAndControllerSmoke`、USN mutation/boundary 和 multipart presence 均通过。
- `$env:LEANOTE_HTTP_INTEGRATION='1'; go test -p 1 ./app/tests/... -count=1 -timeout 30m`：110 pass、0 fail、2 skip；`TestServerServesLoginOverRealHTTP` 通过，真实 listener `GET /login` 返回 200。
- `app/tests/harness/server_test.go` 的 real-server smoke 复用 `startBaselineServer`，先恢复 Mongo 再等待 readiness；`gofmt -l app cmd` 无输出。显式用户主题页面、PDF golden、生产 exit-78、浏览器、容器卷/非 root/重启、replica-set/failpoint/跨进程证据仍保持 `unrun`/`blocked`。

## Continuation evidence (2026-09-28)

- Docker Desktop Linux engine 已恢复：`29.8.0 linux/amd64`；harness 每次基线启动前恢复 `leanote_test`，测试结束清理 `leanote-test-mongo`。
- `TestNativeAPINoteAddMultipartReplay` 与 `TestClientMultipartAddNoteCarriesMetadataAndFilePartsTogether` 以当前工作树代码通过；前者覆盖真实 multipart 字段/文件、`AttachNum=1`、Mongo 内容链接替换和附件下载回读。测试使用每轮新的 NoteId，并按 harness 的 `OID_TOKEN` 规范化断言，避免跨测试内容 manifest 与重置 Mongo 不一致。
- `go test -p 1 ./app/tests/... -count=1 -timeout 30m`：passed（`app/tests`、`app/tests/harness`、harness cmd packages）。
- `$env:LEANOTE_HTTP_INTEGRATION='1'; go test -p 1 ./app/tests/... -count=1 -timeout 30m`：passed，额外真实 listener `GET /login` smoke 通过。
- 仍未执行且交由下游：Mongo 7/replica-set、kill/restart、failpoint、浏览器、容器卷/非 root/restart、PDF、生产 exit-78、跨进程未就绪恢复、显式用户主题页面和备份真实回放。

## Validation

```bash
python ./.trellis/scripts/task.py validate 09-08-interface-http
go build ./...
go vet ./...
go test ./app/httpserver ./app/controllers/... ./cmd/leanote -count=1
go test ./...                                   # 需要 MongoDB
LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/... # 需要 MongoDB 8 standalone
npm test
rg -n 'github\.com/revel|revel\.' app cmd go.mod go.sum sh conf scripts .github Dockerfile
git diff --check
```

真实 HTTP/Mongo 证据写入 `acceptance/evidence-matrix.md`，记录命令、发现/执行数量、commit；缺环境记 `blocked`。

## Review gates

- 每批结束运行 `trellis-check`，并更新 evidence matrix 对应行。
- B8 结束前禁止删除任何 Revel 依赖；B9 结束前禁止宣称 AC-H11 通过。
