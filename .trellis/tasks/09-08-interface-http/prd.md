# 接口适配层：标准库 HTTP 与路由 — PRD

## Goal

完成从 Revel 到标准库 `net/http` 的全量接口迁移，保留服务端渲染、公开 URL/API、Session、鉴权、模板和静态资源行为；`cmd/leanote` 成为 dev/test/prod 唯一入口，并交付唯一的 production-config seam。

## 现状基线（B9 清扫与 2026-09-27 continuation 之后，详见 `research/spec-audit-2026-09-26.md` §2 和 `acceptance/evidence-matrix.md`）

- B0 inventory：`conf/routes` 95 条（83 action、9 static、3 catch-all），242 个导出 `revel.Result` 方法；当前 first-party registry 与 249 条 routable action marker 对账通过（`missing=0, extra=0`）。
- B1–B8 已交付标准库 adapter、production-config/content root seam、identity/API、notes/content、publishing、member/admin registry、dev/test run mode、native harness 目标和 CI/run.sh 接线；focused contract/build/vet 证据已记录在 `acceptance/evidence-matrix.md`。
- 生产入口装配 `LocaleResolver`、SessionReader/Writer、`ViewArgs`；D-H4 六个键与 typed `ContentRoots`；`/upload/*`、`/public/upload/*` 从 public data root 提供；identity 405/`Allow` 与 D-H9 未就绪门控均有 focused contract。
- `cmd/leanote` 已支持 dev/test/prod，test 强制 `leanote_test`；harness 已改为构建 native entrypoint 目标，旧 E-1 生成链已删除。2026-09-27 曾记录 Mongo 8 fixture 下的 native replay 通过；本轮新增 multipart replay 尚未重新执行，当前 Docker Desktop daemon 不可用，故新增真实证据保持 blocked。
- 已知缺口：显式用户主题的 content resolver 页面级证据、备份真实回放及下游交付证据仍待验收；Blog 读取侧兼容和 member 主题上传/导入代码已在当前工作树恢复，不能再按“待清扫/明确拒绝”描述。AttachService/NoteController 的旧全局消费、ConfigService 的备份与数据库连接消费已迁移到第一方 seam；旧 Revel controller/runtime、`app/cmd`、`app/lea/route` 与依赖清扫已在 B9 完成。

## Requirements

### R1 路由与 registry

- 根据 `conf/routes` 建立完整显式路由和受限 catch-all registry，禁止任意反射调用；registry 是唯一可达性来源，未注册的 `Controller.Action` 一律不可达。
- 前缀改写保持：`/api/x/y` → `ApiX.Y`，`/member/x/y` → `MemberX.Y`，其余 `/x/y` → `X.Y`；`conf/routes` 中的 9 条 Static 前缀和路径参数语义不变。
- HTTP 方法策略（D-H5 已定）：identity action 按已决 U-05 固定方法矩阵，未列方法返回 405 + `Allow`；其他 action 保持 `conf/routes` 观察值——`*`/catch-all 接受任意方法，显式 GET/POST 路由方法不符沿用现状 404；HEAD 映射 GET。
- `WN-10 /note/deleteTrash` 等已登记的 deprecated alias 保留（本任务不改 URL）。

### R2 请求适配

- 迁移全部主站、API、admin、member controller actions、参数绑定、结果转换和 BEFORE 钩子（现有活跃 `InterceptFunc` 与 `commonUrl` 白名单逐条对照，白名单字节级保留）。
- 参数绑定保留 Revel 可观察语义（`Atob` 布尔、`Tags[0]`/`Files[0][LocalFileId]` 嵌套键、`Has` 判存在）；在此基础上按 domain 交接执行：required ID 非法 hex、整数溢出、未知枚举值在 adapter 显式失败，不以 zero/默认页码/成功 envelope 继续；`Tags` 字符串与数组两种形态的解析与 Golden 一致。
- 严格/宽松边界（2026-09-26 冻结）：只有上游合同明确点名的字段（domain `input-contracts.json` 的 `required_in`、identity/notes/publishing/admin 交接字段）按严格规则失败；其余参数保持 Revel 零值语义（缺失或不可解析 → 零值/默认值，与 Golden 一致）。每个严格字段在 `research/action-inventory.md` 对应 action 行标注来源合同；未标注的不得擅自收紧。`httpserver.Params` 必须提供 `Has`、`Bool`（Revel `Atob` 语义）、`Strings`、嵌套键读取与“严格整数/ObjectID”读取，现有 `Int` 的宽松语义只用于宽松字段。
- `submissionId` 两类规则不得混用：`Blog.CommentPost`（publishing Q-P9）必填，binder 在业务写入前拒绝缺失/非法值；`Index.Suggestion`（admin Q-A2）为可选兼容输入，登录请求携带时须是 32 位小写十六进制，匿名携带返回 `validation`，匿名不带仍按旧流程提交——这些判断由 `SuggestionService` 完成，binder 只原样传递，不做必填检查。
- 按已归档上游决策承接的请求约束（本任务只实现绑定与错误映射，不复制业务规则）：
  - publishing Q-P9：评论/回复 `CommentPost` 的 `submissionId` 必填，缺失/空/非 32 位小写十六进制在业务写入前失败；Q-P5：非法 JSONP callback 不原样输出。
  - notes KD-N6：`OperationId`、`ExpectedUsn` 的绑定与 notes typed result category → 既有 wire 的逐 action mapper。
  - identity：显式 `userId` 必须与 principal 一致；显式 invalid/expired token 不回退 Web session；API token 仅从 query/form 读取（P-03）。
- Session、`ViewArgs`、i18n、27 个活跃模板函数、gzip、恢复、日志和生产 secret/cookie 安全默认值统一由 HTTP 层提供；生产入口必须装配 `LocaleResolver`（cookie `i18n.cookie` → `Accept-Language` 首项 → 默认语言）、SessionReader/SessionWriter。
- content 委托的 wire 决策：
  - D-H6：`GetImages`/`GetAlbums` 依赖失败返回 500、`application/json; charset=utf-8`，body 保持 legacy 形状（空 Page / `[]`），不得再以 200 伪装成功；成功响应不变。
  - D-H7：`CopyHttpImage` 不新增参数，维持每次请求新导入；在 `docs/modernization-backlog.md` 新增 MOD 条目记录调用方 operation identity 缺口。

### R3 Session 与身份

- 消费 `application-identity` 与 `infrastructure-persistence` 已确认的 principal、SessionWriter、`Session["_ID"]`、API fallback、token TTL 和错误 seam，不在 HTTP 层复制认证或持久化规则。
- 完成 `ApiUser` first-party registry 注册、`_token/_userId` Cookie 在响应头提交前写回，以及 Set/Delete/Commit/Encode 失败传播。
- 承接 P-01～P-08：匿名期 `_ID` 稳定、登录成功轮换并失效旧 fallback；保持无 CSRF、query/form token、`_ID`+Captcha 基线；登出清理失败显式失败；注册 Cookie 提交失败返回 `registered_relogin_required`；API token 32 字节 CSPRNG/base64url、摘要存储及旧 24 位 token 过渡。HTTP 层只实现映射、Cookie/响应时序和 replay。
- admin/member/demo 使用同一 principal（U-04）：`demoUserId` 缺失/不一致 fail closed，admin 由 UserId 判定。principal policy 只能读取**已成功加载**的全局配置快照；快照未加载与“未配置 demo/admin”必须可区分，前者按 D-H9 未就绪处理，不得把空快照当作有效配置。
- identity U-05 方法矩阵（来源：归档 identity `prd.md` §U-05，逐字承接）：

  | action | 允许方法 | 批次 |
  |--------|----------|------|
  | `ApiAuth.Login`、`ApiAuth.Logout` | GET、POST | B1 已注册 |
  | `ApiAuth.Register` | POST | B1 已注册 |
  | `ApiUser.Info` | GET | B2 |
  | `ApiUser.UpdateUsername`、`UpdatePwd`、`UpdateLogo`、`GetSyncState` | POST | B2 |
  | `ApiFile.GetImage`、`GetAttach`、`GetAllAttachs` | GET | B3 |

  HEAD 按 GET 处理；矩阵外方法返回 405 + `Allow`。显式 `userId` 参数：缺失按 action 旧合同处理；非空且与 principal 不一致时，已认证返回 `Msg:"forbidden"`、未认证返回 `Msg:"not_authenticated"`（API envelope，HTTP 200），不得切换目标用户。
- `ApiUser.UpdateLogo`：先校验 multipart `file` 存在且非空（缺失/空 → `fileRequired`，不得 panic）；`UpdateAvatar` 落库失败返回 `storage`，且不得返回 `Logo` 成功。文件存储按 D-H10：wire 不变，经 content resolver/publish 写入 `ContentRoots.PublicUpload`，不新增 `Files` 行，`UpdateAvatar` 失败回收已发布文件。

### R4 production-config seam

- `app/httpserver` 的 production-config 是生产启动配置唯一事实来源；公开稳定错误码、来源校验和 bind/dial/healthz 时序，供 application-admin 与 delivery 消费。现有 `MONGODB_URL`/`LEANOTE_APP_SECRET`/`db.dbname` 规则、exit 78 与 `/healthz` 契约不变。
- 输出只读结构化 `ProductionConfig`，至少包含：typed `ContentRoots`、admin backup root、`ConfiguredDatabaseIdentity`（按 admin 设计的 canonical 字段与 digest 规则，不含密码）和 `CredentialProviderRef`（owner=interface，provider kind + opaque handle，不含凭据）。admin 不再自行解析环境、secret、Mongo URL、bind 或 healthz。
- `ContentRoots`：private/public data root、各自 non-public same-filesystem quarantine root、temporary root，必须在 bind/dial 前完成 absolute/canonical、可写、same-filesystem、non-public、no-overlap 校验（调用 content 提供的 validator，不复制 path resolver 或 quarantine 状态机）；失败以稳定错误码 + exit 78 fail closed。
- 配置键与布局（D-H4 已定）：`[prod]` 必填非敏感键 `content.private.data`、`content.private.quarantine`、`content.public.data`、`content.public.quarantine`、`content.temporary`、`admin.backup.root`，均为绝对路径。容器布局为 `/var/lib/leanote/private/{files,quarantine}`、`/var/lib/leanote/public/{upload,quarantine}`、`/var/lib/leanote/backup` 三个卷与镜像内非 root 可写的 `/var/lib/leanote/tmp`。旧 `/app/files`、`/app/public/upload` 卷由运维手册一次性迁移，运行时无 fallback、无自动迁移；`initializeContentRuntime` 的 application-base 兼容映射删除。
- 静态 `/upload/*` 与 `/public/upload/*` 两个前缀都从已验证的 `content.public.data` 提供，URL 不变；`ServedRoots` = `public` 静态树 + public data root，quarantine 不得位于其中。
- 存储逻辑路径映射（由 D-H4 推导）：`public/upload/<rel>` → `<content.public.data>/<rel>`，`files/<rel>` → `<content.private.data>/<rel>`，并且只能经 content 的单一 resolver 解析。任何 service/controller 不得再以 `revel.BasePath` 或应用树拼接数据路径。
- 应用配置 seam：同一份已校验配置还必须以第一方只读接口向 `ConfigService` 提供 `adminUsername`（缺省 `admin`）与 `site.url`，覆盖 prod/dev/test 三种 run mode，并在 registry 装配前完成；`ConfigService` 不再读取 `revel.Config`。这两个键不是 secret，不改变现有 prod 键校验规则。

### R5 入口、harness 与 Revel 清扫

- `cmd/leanote` 支持 dev/test/prod 三种 run mode（D-H2 已定）：dev/test 只读仓库 `conf/app.conf` 的 `[dev]`/`[test]` section，允许 localhost，content/backup 键可给仓库相对默认值并在启动时解析为绝对路径后走同一 validator；test 强制 `db.dbname=leanote_test`，否则 exit 78。硬约束：prod 路径不得读取仓库 `conf/app.conf`、不得接受 localhost 或 `leanote_test`；三条路径按 `-runMode` 先行分派、互不回退，负例测试覆盖。
- 将 Golden/USN harness、`sh/run.sh`、`sh/package.sh`、CI 迁移到 `cmd/leanote`；`sh/run.sh` 改为 `go run ./cmd/leanote -runMode dev`，不提供 watch/自动重建（D-H3 已定）；`/_test/e2e/identity` 仅 test 模式 + loopback 可达的语义不变。
- 先完成 harness/Golden 在新入口上运行，再删除 Revel：删除 `app/init.go` 旧 filter/TemplateFuncs 注册、`app/lea/route`、`app/cmd/`、`github.com/revel/*` 依赖；`conf/app.conf(-default)` 移除 Revel 专用键（`module.*`、`results.*` 等），第一方解析器对旧部署文件中遗留的这些键忽略且不报错；同步 `CLAUDE.md`、`AGENTS.md`、README/部署文档。不保留双运行时 fallback。

## Acceptance criteria

- [ ] AC-H1 路由：`conf/routes` 全部公开 action 与 9 个静态前缀均到达正确 registry handler；每个已注册 action 至少一条 route-table 测试，registry 注册数与 action 清单（research inventory）对账一致。
- [ ] AC-H2 负例：未注册 controller/action 返回 404；任意导出但未注册的方法（含 BaseController 辅助方法）不能通过 URL 调用；identity 矩阵外方法返回 405 + `Allow`；显式 GET/POST 路由方法不符返回 404；`*`/catch-all 任意方法可达（D-H5）。
- [ ] AC-H3 响应：Web/API 参数、Session、JSON/JSONP/text/template/file/attachment/redirect 响应和错误状态与 Golden 一致；binder 负例（非法 hex、整数溢出、缺失/重复字段、`Tags` 双形态、`submissionId` 缺失/非法）有 contract 测试且在业务写入前失败。
- [ ] AC-H4 兼容：API token 保持有效，Web 旧 Revel cookie 按约匿名并可重新登录；cookie HttpOnly/SameSite/Secure/过期测试通过。
- [ ] AC-H5 identity：方法矩阵、405、principal `userId`、`Session["_ID"]` fallback、显式 invalid token fail-closed、SessionWriter 写失败、P-01 轮换、P-06 logout cleanup failure、P-07 `registered_relogin_required`、P-03 query/form-only、U-04 admin/member/demo principal 均有 contract + 真实 HTTP replay 证据；无 CSRF 与仅 `_ID`+Captcha 风险已记录，不得伪造为安全能力。
- [ ] AC-H6 `ApiUser` first-party actions 已注册并可达；API Auth/User envelope、Content-Type、字段顺序与 Golden 一致（identity AC-I5 live replay）。
- [ ] AC-H7 production-config：bind/dial/healthz 前完成来源、secret/Mongo、typed `ContentRoots` 和 backup root 校验；缺失/相对/不可写/cross-device quarantine/static-reachable/overlap 各有稳定错误码并以 exit 78 fail closed；六个 content/backup 键缺失或非绝对路径同样 exit 78；`/upload/*` 与 `/public/upload/*` 从 public data root 提供且 quarantine 不可经 HTTP 读取；有效配置的 healthz 语义与 delivery contract 一致；admin 通过 consumer fixture 读取 `ProductionConfig`，无重复解析。`ConfigService` 的 `adminUsername`/`site.url` 来自第一方 seam，新入口启动后全局配置快照成功加载（真实进程 + Mongo 证据）；快照不可用时的行为按 D-H9 有负例测试；`rg 'revel\.(BasePath|Config)' app/service app/controllers` 中本批已迁移 action 的调用链零命中。
- [ ] AC-H8 i18n/模板：生产入口 LocaleResolver 生效（cookie、Accept-Language、默认三路测试）；27 个模板函数名集与行为冻结；内置 3 个博客主题与一个上传主题渲染、Preview 错误展示保持。
- [ ] AC-H9 content/notes/publishing wire：`GetImages`/`GetAlbums` 依赖失败 500 + legacy body 形状与成功不变的真实 HTTP regression，D-H7 backlog 条目已登记；notes `OperationId`/`ExpectedUsn` 与 result mapper、publishing `submissionId`/callback 在新 runtime 上 replay 通过。
- [ ] AC-H10 入口：harness 在 `cmd/leanote -runMode test` 上完整运行 Golden/USN/权限门禁；dev/test/prod 来源互斥负例（prod 不读仓库 conf、test 非 `leanote_test` exit 78、dev/test 不接受 canonical prod 文件作为回退）通过；`sh/run.sh` 以 `go run ./cmd/leanote -runMode dev` 启动；SIGTERM 受限 shutdown 通过。
- [ ] AC-H11 清扫：`rg -n 'github\.com/revel|revel\.' app cmd go.mod go.sum sh conf scripts .github Dockerfile` 零命中（含注释与测试）；`go mod tidy` 后 `go.mod` 无 `github.com/revel/*`；`app/cmd/` 与 `app/tmp` 生成链不存在；唯一允许例外是 `docs/` 与 `.trellis/tasks/archive/` 中明确标注为历史架构的文字。
- [ ] AC-H12 质量门：`go build ./...`、`go vet ./...`、`go test ./...`（Mongo 可用）、`npm test`、`git diff --check`、`task.py validate` 通过。

## 证据边界

- 本任务必须自行提供：in-process `httptest` contract 测试，以及 MongoDB 8 standalone + 真实 HTTP 进程的 Golden/USN/identity replay，记录命令、发现/执行数量与 commit。
- Mongo 7、replica set、kill/restart、failpoint、真实浏览器 8 槽、容器/PDF/tarball/GHCR 证据归 `delivery-verification`；本任务不得以 focused mock、controller 直调或历史 artifact 冒充这些证据。缺环境记 `blocked`/`unrun`。
- 发现既有业务缺陷时回开对应已归档 application 任务或单独建任务，不在本任务隐式修改业务语义。

## 决策状态

D-H1～D-H7 已于 2026-09-25 由用户确认全部采用推荐，冻结结论见 `research/spec-audit-2026-09-25.md` §6。D-H1：不拆子任务，按 B0–B9 批次独立提交与门禁。

2026-09-26 B2 前复核新增的 D-H8～D-H10 已于同日由用户确认全部采用推荐，冻结结论见 `research/spec-audit-2026-09-26.md` §6：

- D-H8：test run mode 与 harness 新入口目标前移为 B1.5，B2–B6 每批在新入口 replay 本批 Golden 子集；执行顺序 B2.0 → B1.5 → B2。
- D-H9：全局配置快照未加载前新入口未就绪：`/healthz` 与非静态请求返回 503 `{"status":"not_ready"}`，静态照常，后台退避重试，成功后自动就绪；不在空快照上执行 action。
- D-H10：`ApiUser.UpdateLogo` wire 不变（路径、扩展名白名单、5MB、`Logo` URL），改经 content resolver/publish 写入 public data root，不新增 `Files` 行，`UpdateAvatar` 失败回收文件并返回 `storage`。
- E-1（嵌套 worktree 污染旧生成器）已通过删除生成链消除；E-2（本机无 Mongo 8）仍为环境阻塞，相关真实 replay 证据保持 `blocked`。

实现中若发现新的不可推导需求，须先回写本 PRD 再实施。

## Out of scope

不改变 URL/API/Schema（D-H6 是已确认的受控 wire 例外，容器卷布局按 D-H4 改变属部署契约变更而非数据迁移）、不引入第二个 Web 框架、不重写业务服务、不新增 CSRF/限流/Bearer header、不提供 dev watch、不为 `CopyHttpImage` 新增参数、不在本任务加入新产品功能。
