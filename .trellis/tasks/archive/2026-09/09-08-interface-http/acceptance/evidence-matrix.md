# interface-http 验收证据矩阵

决策 D-H1～D-H7 已确认（见 `research/spec-audit-2026-09-25.md` §6），矩阵中不再有决策类 blocked。

状态取值：`unrun`（未执行）、`blocked`（前置决策或环境缺失）、`partial`、`passed`。每条 `passed` 必须写明命令、发现/执行数量与 commit；mock、controller 直调或历史 artifact 不得记为真实 HTTP 证据。当前工作树尚未提交，因此本轮新增的真实回放证据先保持 `partial`，待提交后再按同一命令和 commit 复核晋级。

| AC | 内容 | 证据类型 | 前置 | 状态 |
|----|------|----------|------|------|
| AC-H1 | 全部公开 action/静态前缀可达；registry ↔ inventory 对账 | route-table + 对账测试 | B0 inventory | partial |
| AC-H2 | 未注册 404、导出未注册不可达、identity 405、其他方法策略 | route negative | B1 | partial |
| AC-H3 | 响应/状态/Golden；binder 负例 | contract + Golden replay（Mongo 8 standalone） | B2–B6 | partial |
| AC-H4 | API token 保持、旧 Web cookie 匿名、cookie 安全属性 | session contract + replay | B1 | partial |
| AC-H5 | identity 方法/principal/`_ID`/P-01/P-03/P-06/P-07/U-04 | contract + 真实 HTTP replay | B2 | partial |
| AC-H6 | `ApiUser` 可达、API Auth/User Golden（identity AC-I5） | Golden live replay | B2 | partial |
| AC-H7 | production-config 校验、ContentRoots/BackupRoot、exit 78、healthz、admin consumer | config contract + 进程级 exit code 测试 | B1 | partial |
| AC-H8 | LocaleResolver、27 模板函数、主题渲染/Preview | contract + 页面 smoke | B1、B4 | partial |
| AC-H9 | D-H6 wire、notes OperationId/ExpectedUsn、publishing submissionId/callback | 真实 HTTP regression | B3、B4 | partial |
| AC-H10 | harness 在 `cmd/leanote -runMode test` 全量运行；run mode 互斥；SIGTERM | harness 全量 + 负例 | B8 | partial |
| AC-H11 | first-party 代码/配置无旧运行时命中、go.mod 无旧依赖、`app/cmd` 删除 | scoped rg + `go mod tidy` diff | B8/B9 | partial |
| AC-H12 | build/vet/test/npm/diff-check/validate | 质量门 | 全部批次 | partial |

## B0 evidence (2026-09-25)

- `research/action-inventory.md` records the code-derived baseline: 95 routes (83 action, 9 static, 3 catch-all) and 242 exported controller methods returning `revel.Result`; each row includes routability, owner, method, current BEFORE/commonUrl facts, response shape, and Golden status.
- `go test ./app/httpserver -count=1`: passed, including route parsing and the negative that an exported `BaseController` helper is not reachable through the catch-all.
- `go vet ./app/httpserver`: passed.
- `go test ./app/controllers -run '^TestRegistryMatchesB0Inventory$' -count=1`: passed with `missing=0, extra=0`; the marker contains 249 routable action names (including route aliases) and the first-party registry contains the same set. This proves registry reconciliation only; no real HTTP/Mongo replay was run.

## B1 focused evidence (2026-09-26)

- `go test ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/controllers/admin -count=1`: passed. Covers locale precedence, 27 template function names, `ViewArgs`, ContentRoots/BackupRoot validation and error-code mapping, both `public` and `public/upload` static roots, admin typed consumer fixture, and identity 405/`Allow` registration. Existing session evidence is `TestAppWritesSessionCookieBeforeActionResponse`, `TestAppUsesInjectedSessionWriter`, `TestAppRejectsSessionCommitFailure`, and `TestAppPreservesActionFailureWhenSessionCommitAlsoFails`.
- `go vet ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/controllers/admin`: passed.
- `go build ./...`: passed.
- Git Bash `sh sh/package.sh`: passed; tar listing contains `bin/leanote` and no `var/` tree, so absolute `/var/lib/leanote` roots are created by deployment rather than embedded under an `/app` extraction prefix.
- `gofmt -l` over all B1 Go files: no output; `python ./.trellis/scripts/task.py validate 09-08-interface-http`: passed (`implement.jsonl` 16 entries, `check.jsonl` 12 entries); `git diff --check`: passed.
- `go test ./... -count=1`: historical partial evidence. The registry reconciliation passed, while Mongo-backed tests and the former generated harness were environment-blocked; the generated harness has since been replaced by the native entrypoint.
- Real listener requests, Mongo/Golden replay, production exit-78 process checks, container volume/non-root/restart checks, browser smoke, and full harness migration remain `unrun`; no focused in-process result is promoted to those gates.

## B2 前复核（2026-09-26）

- 历史 B1 记录：旧 harness 曾在服务启动前扫描被 git 忽略的嵌套 worktree，生成错误入口并失败；该生成链已在 B9 删除，当前 native harness 不再走这条路径。该历史结果不代表当前实现回归。
- 本机 `127.0.0.1:27017` 拒绝连接（E-2）；所有需要 Mongo 的证据保持 `blocked`。
- 历史 B1 缺口：新入口最初没有配置 seam，导致全局配置快照未加载；B2.0 已补齐 in-process seam 和未就绪门控，真实进程证据仍待 E-2。
- D-H8～D-H10 已于 2026-09-26 确认采用推荐：AC-H3/H5/H6/H9 的真实 HTTP 证据从 B2 起按批在 `cmd/leanote -runMode test` 上 replay（B1.5 交付后）；AC-H7 增加 D-H9 未就绪负例；AC-H5/H6 增加 D-H10 `UpdateLogo` wire 不变证据。

## B2.0 focused evidence (2026-09-26)

范围：应用配置 seam（PRD R4“应用配置 seam”、design §7 首行）与 D-H9 未就绪门控；B1.5 与 B2 已继续交付，真实 Mongo replay 仍受 E-2 阻塞。

- 实现：`service.AppConfigSource`/`SetAppConfigSource`（`adminUsername` 缺省 `admin`，`site.url` 缺省空，均非 secret，prod 键校验规则未改）；`InitGlobalConfigsWithError` 不再读取框架全局配置；`ConfigService.GlobalSnapshotLoaded()` 与 `service.ErrGlobalConfigNotLoaded`；`httpserver.App.Ready` 门控；`cmd/leanote/readiness.go` 后台退避重试（含 Mongo 重连）；`api.PrincipalPolicyFromConfig` 未加载时 fail closed。旧入口链已在 B9 删除。
- `gofmt -l`（11 个改动/新增 Go 文件）：无输出。
- `go build ./...`：passed。
- `go vet ./app/httpserver ./cmd/leanote ./app/controllers/... ./app/service`：passed。
- `go test ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/controllers/admin ./app/service -count=1`：passed。新增用例：
  - `app/service/config_seam_test.go`：`TestInitGlobalConfigsUsesInjectedAppConfigSource`（`revel.Config=nil` 下 seam 的 `adminUsername`/`site.url` 生效，admin 角色 ID、`openRegister`、`siteUrl`、demo 策略、site.url 域名推导）、`...DefaultsAdminUsernameAndKeepsEmptySiteURL`、`...FailsClosedWithoutAppConfigSource`、`...FailureKeepsSnapshotUnloaded`、`...WithoutDatabaseReportsDependencies`、`TestSetAppConfigSourceDerivesSiteDomainIdempotently`。
  - `app/httpserver/readiness_test.go`：未就绪时 healthz/显式路由/catch-all/未匹配路由/非 GET healthz 均 503 `{"status":"not_ready"}\n`、无 Set-Cookie、不运行 action/`OnRequest`/`HealthCheck`，静态照常；就绪后恢复 `HealthCheck` 语义与正常分派；gzip 协商下 healthz 仍为固定行；未设 `Ready` 时分派不变。
  - `cmd/leanote/readiness_test.go`：重连→加载→退避（1s/2s/4s 封顶）、DB 已连通不重连、关停取消、就绪门控接入 App、`*httpserver.Config` 满足 seam（`[prod]` 覆盖 DEFAULT、行内注释剥离）、`main.go`/`readiness.go` 零 `revel.Config` 引用且 seam 注入先于 registry 装配。check 复核补充：`TestNotReadyServesPublicDataRootStatics`（真实 `conf/routes` + 生产 `staticHandlerWithContent`，未就绪时 `/upload/*`、`/public/upload/*` 从 public data root、`/js/*`、`/public/js/*` 从应用树照常服务，HEAD 静态 200；HEAD `/healthz`、`/login`、`POST /api/auth/login`、`POST /upload/...` 均 503 且不运行 `OnRequest`/`HealthCheck`）；`TestStartupReadinessLogsOncePerPendingStage`（重试日志每个未就绪阶段只输出一条稳定提示，不含错误原文）。
  - `app/controllers/api/demo_policy_test.go`：`TestPrincipalPolicyFromConfigFailsClosedWhenSnapshotNotLoaded`（未加载返回 `ErrGlobalConfigNotLoaded`、principal 为匿名）；既有两条 policy 用例改为显式标记快照已加载。`firstPartyAPIApp`（Mongo 依赖）夹具同样显式标记，保持其 pre-D-H9 语义。
  - 因无 Mongo 跳过（环境 E-2，均为既有用例）：api 包 `TestApiAuthLoginRoundTrip`、`TestApiAuthLogoutClearsToken`、`TestApiAuthLogoutWithoutTokenIsIdempotent`、`TestApiTagLifecycle`；service 包 18 条 Mongo 集成用例（notes/notebook receipt、blog/comment/theme 等）。
- `go test -race ./app/httpserver ./cmd/leanote -run 'Readiness|Health|EntryNever|ProductionConfigSatisfies|WaitWith' -count=1`：passed。
- `go test ./app/controllers/... -count=1`：native controller packages pass, including `TestRegistryMatchesB0Inventory` with `missing=0, extra=0`; Mongo-backed action tests remain environment-blocked when the local service is unavailable.
- `git diff --check`：passed（仅 CRLF 提示）。
- `rg 'revel\.(BasePath|Config)' app/service/ConfigService.go`：快照路径（原 53-61、964）零命中；剩余命中为备份根与 `db.*`（B6 归属）及说明注释。
- **blocked（E-2）**：真实进程 + Mongo 8 下“新入口启动后快照成功加载”、“启动时 Mongo 不可达 → healthz/非静态 503、恢复后自动就绪”的证据未执行；AC-H7 维持 `partial`。
- 行为变化（有意）：新入口的 outbox worker 改为快照就绪后启动（此前 DB 就绪即启动，即使快照加载失败）；关停超时错误文本改为 `readiness/outbox worker shutdown timed out`。

## B1.5/B2 focused evidence (2026-09-26)

- 实现：`cmd/leanote` 现在按 `-runMode` 选择 `dev`/`test`/`prod`；dev/test 读取仓库 `conf/app.conf`，test 强制 `leanote_test`，拒绝 canonical production config；local runtime 将相对 content/backup 根解析为绝对路径并复用 content validator。`app/tests/harness` 直接构建并启动 `cmd/leanote -runMode test`，不再生成旧运行时入口。
- 实现：`httpserver.Params` 增加 `Has`、Atob 兼容 `Bool`、`Strings`、`NestedString`、`StrictInt`、`StrictObjectID`；API registry 注册 `ApiUser.Info`（GET）与四个 POST action，显式 `userId` 不一致分别映射 `forbidden`/`not_authenticated`；`UpdateLogo` 校验 multipart/大小/扩展名，经 content publish 写入 public upload 根，`UpdateAvatar` 失败时执行 content cleanup。
- `go test ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/service -count=1`：passed；新增 `Params`、local runtime、ApiUser method-matrix 覆盖。
- `go vet ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/service ./app/tests/harness`：passed；`go build ./...`：passed。
- `go test ./app/tests/harness -count=1`：全量 replay 仍需 Mongo/fixture；native build/toolchain/contract 测试不再依赖旧生成器。当前本机 Mongo 不可用，真实 listener 未启动。
- Mongo 8 standalone is unavailable on `127.0.0.1:27017` (E-2); real `cmd/leanote -runMode test` listener, Golden/USN/identity replay, and `UpdateLogo` storage read-back remain `blocked`, so AC-H3/H5/H6/H10 stay open.

## B2 web adapter focused evidence (2026-09-26)

- 实现：`RegisterMainHTTP` 注册 `Auth` 十个显式 action、`Captcha.Get`、`Index.Default/Index/Suggestion` 与 `User` 十个 action；`webSessionBefore` 将 `_SESSION` 的 `UserId` 提升为统一 principal，`requireWebAuthentication` 保留普通请求 302 与 XHR `NOTLOGIN` 分支。
- 实现：登录/注册/体验登录复用已有 Auth/Session service，成功时轮换匿名 `_ID` 并提交 web session；注销清理 token/session；验证码先持久化 captcha 再写 PNG；Suggestion 原样传递可选 `submissionId`；User 更新 action 复用 service、session writer、模板/i18n ViewArgs。
- `go test ./app/controllers -run 'Test(RegisterMainHTTP|Native)' -count=1`：passed（4 个 B2 native contract 用例：注册/方法矩阵、登录 storage 失败不计数、验证码先持久化、User 未登录普通/XHR 分支）。
- `go test ./app/lea/i18n ./app/controllers -run 'Test(GetDefaultLang|RegisterMainHTTP|Native|FirstParty|Auth|Captcha)' -count=1`：passed；覆盖 plain-Go 默认语言 seam。
- `go vet ./app/controllers ./app/controllers/api ./app/httpserver ./app/service ./cmd/leanote`：passed；`go build ./...`：passed；`git diff --check`：passed（仅换行风格提示）。
- **blocked（E-2）**：Mongo 8 不可用，故 Auth/User/Suggestion 的真实 HTTP、Cookie replay、Suggestion durable receipt、Golden 子集和 `cmd/leanote -runMode test` listener 未执行；AC-H3/H5/H6/H10 仍保持 `unrun`/`blocked`，不能由上述 in-process contract 晋级。

## B3 focused evidence (2026-09-26)

- 实现：主站 Note/Notebook/Tag/NoteContentHistory、File/Attach/Album，以及 ApiNote/ApiNotebook/ApiFile adapters 接入 first-party registry；保留 `Note.ExportPDF`/`Note.ExportPdf` route alias，`ApiFile.GetImage/GetAttach/GetAllAttachs` 固定 GET-only。Notebook 父级、拖拽 JSON、`Tags`/`Tags[]`、`ExpectedUsn` 和 API note 基础新增/更新均在 HTTP 边界绑定；`ApiNote.AddNote`/`UpdateNote` 均接入 multipart `Files[...]` + `FileDatas[...]` 的稳定资产流程，真实 replay 由当前 continuation 单独记录。
- D-H6：`File.GetImages`/`Album.GetAlbums` 依赖失败返回 HTTP 500，并保持空 `info.Page`/`[]info.Album` body；`Attach.GetAttachs` 仍返回 legacy `Re{Msg:"error"}` 200 envelope。`CopyHttpImage` 从配置读取 `uploadImageSize` 并保留 `Id` 字段；新增安全 `Content-Disposition` filename 清理和下载读取错误传播。
- `go test ./app/httpserver ./app/controllers/api ./app/controllers -run 'Test(RegisterNotesHTTP|RegisterHTTP|BindAPINote|BindWebNote|FirstParty|Native|DownloadDisposition)' -count=1`：passed。
- `go vet ./app/httpserver ./app/controllers ./app/controllers/api ./app/service ./cmd/leanote`：passed；`go build ./...`：passed；`gofmt -l`（本批改动 Go 文件）：无输出；`git diff --check`：passed（仅 CRLF 提示）。
- `go test ./app/controllers ./app/controllers/api ./app/httpserver ./cmd/leanote -count=1`：native packages pass, including registry reconciliation (`missing=0, extra=0`); Mongo-backed API tests are blocked by E-2 when the local service is unavailable.
- 历史 `go test ./... -count=1`：registry 缺口已关闭，但旧 harness 的 E-1 生成链和 Mongo 8 不可用共同阻塞了 replay；当前 continuation 已移除 E-1 生成链，剩余真实 listener/Golden/USN/身份 replay 仍受 E-2 阻塞。

## B4 publishing focused evidence (2026-09-26)

- `Blog`、`Share`、`Preview` actions 已在 `app/controllers/httpserver_publishing.go` 注册；JSONP callback 校验拒绝不安全 callback，`Blog.CommentPost` 在 service 写入前强制 32 位小写十六进制 `submissionId`。
- `ThemeService` 用户主题写入通过注入的 public upload content root 解析。博客读取侧兼容已在当前工作树实现；显式用户主题页面级 smoke 仍不提升为 live evidence。
- `go test ./app/controllers -run 'TestPublishing|TestRegisterPublishingHTTP' -count=1`、`go vet ./app/controllers ./app/httpserver`：passed。真实 publishing HTTP、主题渲染和 Mongo receipt replay 受 E-2 阻塞。

## B5 member focused evidence (2026-09-26)

- `member/*` actions 已按 `MemberX.Y` 注册，复用 web session principal 和匿名 redirect/XHR `NOTLOGIN` 行为；`ExportTheme` 使用标准库文件响应。
- 主题图片上传和主题导入已恢复实际写入；真实 member 页面和 multipart replay 尚未执行。
- `go test ./app/controllers/member -count=1`、`go vet ./app/controllers/member`：passed。

## B6 admin focused evidence (2026-09-26)

- inventory 中的 admin actions 已全部注册并经过 admin principal；生产入口向 adapter 注入已校验 `ProductionConfig`，`AdminData.Download` 使用 typed backup/database 值并执行 containment、稳定路径和大小校验。
- `ConfigService` 的备份/恢复和数据库连接消费已由 `RuntimeConfig`/typed database provider 提供，当前 scoped 代码不再读取旧 `revel.Config`/`BasePath`；`go test ./app/controllers/admin -count=1`、`go vet ./app/controllers/admin`：passed。备份真实 replay 仍待环境，未把 focused 结果升级为生产证据。

## B7/B8 focused evidence (2026-09-26)

- API whitelist helper 已由两个 API 校验路径共享；registry reconciliation 对 249 个 marker action 报告 `missing=0, extra=0`。
- `cmd/leanote` 支持 dev/test/prod，test 强制 `leanote_test`；`sh/run.sh` 使用 `go run ./cmd/leanote -runMode dev`；harness 和 CI summary 已包含 native entrypoint 目标。
- `go build ./...`、`go vet ./...`、`python ./.trellis/scripts/task.py validate 09-08-interface-http`、`git diff --check`：passed。`npm test` 完成 132 个测试，其中 131 个通过、1 个跳过、0 个失败。
- E-2 阻塞 Mongo-backed native replay；生产进程 exit-78、浏览器和容器证据仍为 unrun/blocked。B9 的依赖、配置键、`app/cmd` 和 scoped zero-hit 清扫已在后续 continuation evidence 记录。

## Continuation evidence (2026-09-27)

本节是当前环境的最终证据；上方 2026-09-26 的 E-2 blocked 记录保留为历史审计轨迹，不覆盖本节已执行结果。

- 分页修复：`pageParam`/`apiPage` 对缺省、非法、零值和负值 page 返回第 1 页；`go test ./app/controllers ./app/controllers/api -count=1` passed。该修复避免负数 Mongo skip 被旧错误忽略路径转成空响应，Golden replay 仍需 Mongo。
- Native harness/test fixes：`httpserver_main_test.go` 的 native fakes 与 API multipart presence boundary 已补齐；`app/tests/harness` 只构建 `cmd/leanote`，不再扫描嵌套 worktree 生成旧入口。
- `go mod tidy` passed；`go.mod`/`go.sum` 已删除旧运行时依赖，`conf/app.conf` 与 `conf/app.conf-default` 已删除 `module.testrunner`。
- Scoped cleanup command `rg -n -i "revel" app cmd go.mod go.sum sh conf scripts .github Dockerfile`：零命中；`app/httpserver` 中保留的是行为兼容实现，已改为不含旧运行时名称的说明。历史文档、AGENTS/CLAUDE、`tests/e2e` 和 `.trellis` 材料不在 AC-H11 scoped command 中。
- `gofmt -l app cmd`：无输出；`go test ./app/controllers ./app/controllers/api ./app/controllers/member ./app/controllers/admin ./app/httpserver ./cmd/leanote -count=1` 与 `go test ./app/service -count=1`：passed。
- `go test ./... -run '^$' -count=1`、`go build ./...`、`go vet ./...`：passed；`npm test`：132 项，131 passed、1 skipped、0 failed；在该 2026-09-27 checkpoint，`python ./.trellis/scripts/task.py validate 09-08-interface-http`：passed（17 implement、13 check）；`git diff --check`：passed（仅换行转换提示）。
- Docker 真实环境：`docker version --format '{{.Server.Version}} {{.Server.Os}}/{{.Server.Arch}}'` 返回 `29.8.0 linux/amd64`；harness 自行启动 `mongo:8.0`（`leanote-test-mongo`）、恢复 `leanote_test` fixture，测试结束后容器清理。
- `go test -p 1 ./app/tests/... -count=1 -timeout 30m`：Mongo 8 fixture 下通过；JSON 事件统计为 109 pass、0 fail、3 skip（旧 `TestAuth` 无外部服务、缺少已审阅 PDF golden、默认关闭的 real-server smoke）。该命令覆盖真实 `TestGoldenAPIActions`、`TestGoldenWebOwnershipControllers`、`TestWebAdminMemberAndControllerSmoke`、USN mutation/boundary 和 API multipart presence。
- `$env:LEANOTE_HTTP_INTEGRATION='1'; go test -p 1 ./app/tests/... -count=1 -timeout 30m`：通过；在修复 harness 生命周期后为 110 pass、0 fail、2 skip，额外执行 `TestServerServesLoginOverRealHTTP`，真实 `cmd/leanote -runMode test` listener 的 `GET /login` 返回 200。
- 修复：`app/tests/harness/server_test.go` 的 real-server smoke 复用 `startBaselineServer`，先恢复 Mongo fixture 再等待 readiness；不改变应用未就绪时的 503 门控。`gofmt -l app cmd`：无输出。
- 真实回放补充：博客 fixture 的 `notes.IsBlog=true` 而对应 `note_contents` 缺少 `IsBlog` 时，`/blog` 仍返回 200；无主题 Preview 真实请求仍返回 404。显式用户主题页面、PDF golden、生产 exit-78、浏览器、容器卷/非 root/重启、replica-set/failpoint/跨进程证据继续 `unrun`/`blocked`。

## Current continuation evidence (2026-09-28)

- Docker Desktop Linux engine is available: `docker version --format '{{.Server.Version}} {{.Server.Os}}/{{.Server.Arch}}'` returned `29.8.0 linux/amd64`. The harness started `mongo:8.0` as `leanote-test-mongo`, restored the `leanote_test` fixture for each baseline server, and cleaned the container after each test.
- 代码依据：`ApiNote.AddNote` 已恢复 multipart 创建流程：先绑定普通字段和 `Files[...]` 元数据，再以 `FileDatas[...]` 读取并校验资产，冻结创建 receipt，发布预笔记资产，提交 note/content，替换内容中的本地文件链接，并在 note 成功后 finalize；失败时只清理本次尝试的 request-owned 预笔记资产。`ApiNote.UpdateNote` 继续使用同一 content/asset seam。
- 复核修正：成功上传的附件现在写入 `Note.AttachNum`；清理准备阶段不再改写或误删 `HasBody=false` 的已有文件引用；提交给 notes service 的资产清单只包含最终保留的文件并带回冻结 `ContentSHA256`。
- 新增单测：`go test ./app/controllers/api -run 'TestRewriteAPINoteContentLinks|TestAPINoteCreateOperation' -count=1`：passed；完整 `go test ./app/controllers/api -count=1`：passed。
- 新增 native harness 用例：`TestNativeAPINoteAddMultipartReplay` 同时发送普通字段、`Files[0][...]` 元数据和 `FileDatas[local-attachment]` 文件，并检查响应字段、Mongo 内容链接、`AttachNum=1` 及附件下载回读。`go test ./app/tests/harness -run 'TestNativeAPINoteAddMultipartReplay|TestClientMultipartAddNoteCarriesMetadataAndFilePartsTogether' -count=1 -timeout=90s`：passed（两个用例）。测试断言遵循 harness 的 `OID_TOKEN` 响应规范化，并为每轮生成新的 NoteId，避免每测试重置 Mongo 而持久化 content manifest 造成跨存储污染。
- HTTP 请求边界复核修正：`Params.FormFile` 先调用统一的 `ensureForm`，因此直接先取文件的上传 action 也会应用请求体上限并解析普通 multipart 字段；新增 `TestParamsFormFileParsesBoundedFormWhenCalledFirst`。
- 本轮门禁：`go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote -count=1`、`go build ./...`、`go vet ./...`、`go test ./... -run '^$' -count=1` 均通过；`npm test` 为 132 项（131 passed、1 skipped、0 failed）；`task.py validate 09-08-interface-http` 通过（18 implement、14 check）；`git diff --check` exit 0（仅换行转换提示）。
- `go test -p 1 ./app/tests/... -count=1 -timeout 30m`：passed；`app/tests`、`app/tests/harness`（79.363s）及 harness cmd packages 均通过，真实 Mongo 8 fixture、Golden/USN/权限/admin/member smoke 和 multipart replay 均执行。
- `$env:LEANOTE_HTTP_INTEGRATION='1'; go test -p 1 ./app/tests/... -count=1 -timeout 30m`：passed；`app/tests/harness`（83.216s）及 harness cmd packages 均通过，并额外执行真实 `cmd/leanote -runMode test` listener 的 `GET /login` smoke。
- `docs/modernization/cicd-delivery.md` 与 `app/tests/README.md` 已检查，当前已描述 native `cmd/leanote` 工作流；无需追加重复 README 文案。
- 仍未执行：Mongo 7 standalone、Mongo 8 replica set、kill/restart、failpoint、真实浏览器、容器 paired volume/non-root/restart、PDF golden、生产 exit-78、跨进程未就绪恢复、显式用户主题页面和备份真实回放；这些证据不能由本轮 standalone harness 晋级。

## 下游移交（本任务不关闭）

- Mongo 7 standalone、Mongo 8 replica set、kill/restart、failpoint、真实浏览器 8 槽、容器 paired volume/non-root/restart、PDF、tarball/GHCR → `delivery-verification`。
- 前端 `submissionId`/`OperationId` 生成与复用、`registered_relogin_required` 展示 → `presentation-frontend`。
