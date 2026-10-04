# 接口适配层：标准库 HTTP 与路由 — 技术设计

## 1. Boundaries

`app/httpserver` 提供 request/params、route table、registry、middleware、session、templates、response、production-config 和 server；controller adapter 只把 HTTP 映射到 application service，不持有 Mongo collection/client，不复制 USN、权限、receipt 或 quarantine 状态机。

| 关注点 | owner | 本任务职责 |
|--------|-------|------------|
| 认证/授权决策、token/session 规则 | application-identity（已归档） | principal 注入、Cookie 时序、错误映射、replay |
| workspace/USN/receipt | application-notes | binder + result category mapper |
| 文件根校验、path resolver、quarantine | application-content | 从配置构造 typed `ContentRoots` 并调用 validator |
| 评论身份与 receipt | application-publishing | `submissionId` 必填绑定、callback 过滤 |
| 备份/升级/设置 | application-admin | 提供只读 `ProductionConfig` producer |
| driver/事务/超时 | infrastructure-persistence | 仅通过 service/db seam 使用 |

## 2. Migration shape

`conf/routes` → 显式 route table → 受限 controller/action registry → typed `Context` → application service → Result writer。catch-all 只查 registry，拒绝反射。

每个 controller 包提供 `RegisterHTTP(*httpserver.Registry, deps)`，以静态表声明 `Controller.Action`、BEFORE 钩子与 handler；`cmd/leanote` 按原 `OnAppStart` 顺序装配（db → email → vd → service → admin/member service → global config → api service → registry）。registry 在启动时与 inventory 对账：inventory 中 routable 但未注册，或注册了 inventory 外的名字，启动测试失败。

双实现冻结规则（2026-09-26）：B2–B8 期间旧 Revel controller 仍供 harness 使用，新 handler 另行实现（如 `ApiAuthServer`）。action 一旦注册进 registry，新 handler 即为唯一事实来源，对应 Revel 版本冻结到 B9 删除为止；只有当新 handler 修复的缺陷也会让旧 harness 基线失真时，才允许同步修改旧版本，并须在该批 evidence 中注明。超出参数绑定/结果映射的判断（如 E2E identity 的 `evaluateE2eIdentity`）应提取为无 HTTP 依赖的共享 core，两边共用，不得复制。

## 3. Request pipeline

```text
Recover → Gzip → /healthz → OnRequest(CheckMongoSessionLost) → RouteTable.Match
  → Static | registry.Lookup → Session decode (旧/损坏 → 匿名) → Locale
  → Context{Params, Session, SessionReader/Writer, Principal policy}
  → BEFOREs → Handler → Session commit (先于响应头) → Result.Apply
```

- 方法（D-H5）：HEAD 映射 GET；identity 矩阵外方法在 registry 层返回 405 并带 `Allow`；显式 GET/POST 路由方法不符由 route table 返回 404；`*`/catch-all 不限方法。
- Commit 失败：失败结果原样返回，成功结果改写为 `session_commit` 失败 envelope（已有 `applySessionCommitFailure`）；P-07 注册路径改用 `registered_relogin_required`。

`ApiNote.AddNote`/`UpdateNote` 的 multipart 数据流固定为：`Params.ensureForm` 先解析普通字段和 `Files[...]` 元数据，随后通过 `FormFile("FileDatas[<LocalFileId>]")` 读取有界文件；创建路径以 owner + note + 请求摘要冻结 operation receipt，发布预笔记资产后提交 note/content，再由 content asset port reconcile 并 finalize 预笔记 manifest。内容中的本地图片/附件 URL 在提交前替换为服务端 `FileId`；创建失败只清理本次 receipt 绑定的 request-owned 资产，note 已提交但 projection/finalize 失败则保留 receipt 供重试。HTTP 层只编排 service seam，不直接操作 Mongo、文件根或 quarantine。

## 4. Run modes and configuration

| mode | 配置来源 | Mongo 约束 | 可达的测试端点 |
|------|----------|------------|----------------|
| prod | 仅 `/etc/leanote/app.conf`（0440）+ `MONGODB_URL`/`LEANOTE_APP_SECRET` | 非 localhost、db≠`leanote_test`、路径 db = `db.dbname` | 无 |
| dev | 仓库 `conf/app.conf` `[dev]`（D-H2） | 允许 localhost | 无 |
| test | 仓库 `conf/app.conf` `[test]`（D-H2） | db 必须为 `leanote_test`，否则 exit 78 | `/_test/e2e/identity`（loopback） |

三条路径由 `-runMode` 先行分派，互不回退；prod 校验失败 exit 78，绝不尝试 dev/test 来源。

`ProductionConfig`（只读）：

```go
type ProductionConfig struct {
    Addr              string
    ShutdownTimeout   time.Duration
    DatabaseName      string
    DatabaseIdentity  service.ConfiguredDatabaseIdentity // canonical 字段 + digest，无密码
    Credential        service.CredentialProviderRef      // {ProviderKind:"env", OpaqueHandle:"MONGODB_URL", Owner:"interface-http"}
    ContentRoots      service.ContentRoots                // content validator 的 canonical 结果
    BackupRoot        string                              // 非公开、canonical（D-H4）
}
```

D-H4 配置键 → 字段：`content.private.data`/`content.private.quarantine` → `PrivateFiles`，`content.public.data`/`content.public.quarantine` → `PublicUpload`，`content.temporary` → `Temporary`，`admin.backup.root` → `BackupRoot`；`ServedRoots` 由入口固定为 `public` 静态树与 public data root。prod 值必须为绝对路径；dev/test 允许仓库相对值，由入口按仓库根解析后走同一 validator。容器参考布局：`/var/lib/leanote/private/{files,quarantine}`、`/var/lib/leanote/public/{upload,quarantine}`、`/var/lib/leanote/backup` 三卷 + 镜像内 `/var/lib/leanote/tmp`。

静态分派：`/upload/*` 与 `/public/upload/*` 由 public data root 的 `http.FileServer` 提供（`/public/*` 的 handler 对 `upload/` 子路径转交该 handler），其余 `/public/*` 仍来自应用 `public` 树。

校验时序：文件结构 → 键/来源冲突 → 环境值 → secret/Mongo URL → ContentRoots/BackupRoot → bind → dial（失败只令 healthz 503，不退出）。ContentRoots/BackupRoot 错误码使用 `CONFIG_CONTENT_ROOT_{MISSING|RELATIVE|UNWRITABLE|CROSS_DEVICE|PUBLIC|OVERLAP}`，`Key` 为 D-H4 配置键名。

## 5. Invariants

公开 URL/API、method（按 D-H5 保持观察值）、参数、状态码、JSON/JSONP、Session、i18n、gzip、静态资源和模板行为保持；匿名期 `_ID` 稳定但认证成功时必须轮换并失效旧 fallback 映射；Web cookie 可一次性失效，API token 保持。P-06 清理失败不得返回伪成功，P-07 注册 Cookie 提交失败不得设置认证 Cookie 或跳转受保护页面；API token 仅从 query/form 读取，任何未来 header 扩展的冲突规则需另立契约。生产 secret/config fail closed。CSRF 与通用限流保持现状，不在本任务隐式新增。

受控 wire 例外包括：D-H6（`GetImages`/`GetAlbums` 依赖失败 500 + legacy body）、上游已决的 Q-P5/Q-P9，以及 identity 任务 U-05 方法矩阵带来的 GET→POST 约束。U-05 的来源是归档 identity `prd.md` §U-05；允许方法之外必须返回 405 + `Allow`，并保留至少一条 GET→405 Golden 负例（当前为 `api/user/getSyncState_get.json`）。每项都要更新 Golden 并在 evidence matrix 标注来源决策。

## 6. Rollback

先完成 harness/Golden 切换再删除 Revel；每个批次（implement.md B0–B9）单独提交，失败回滚该批提交。Revel 删除（B9）是最后一个批次，前置条件是 B8 在新入口上全部门禁通过；任何不完整迁移不引入静默双栈。

## 7. Revel 全局消费点 → 批次（2026-09-26 复核）

新入口中 `revel.Config == nil`、`revel.BasePath` 指向应用树；下列消费点必须在所属批次改走第一方 config/`ContentRoots`/logger seam，否则该批 action 在 prod 下行为错误。行号以 `5c665ba8` 为准。

| 消费点 | 当前依赖 | 目标 seam | 批次 |
|--------|----------|-----------|------|
| `service/ConfigService.go:53-61,964`（`InitGlobalConfigsWithError`、`adminUsername`、`site.url`） | `revel.Config` | 第一方只读应用配置（PRD R4“应用配置 seam”）；快照不可用策略按 D-H9 | B2.0（B2 前置） |
| `controllers/api/ApiUserController.go:150`（API 头像写入） | `revel.BasePath` | content resolver + `ContentRoots.PublicUpload`（D-H10） | B2 |
| `service/AttachService.go:637,812,825,945,965`（附件存在性/读取/打包） | `revel.BasePath` | content resolver（`files/` → private data，`public/upload/` → public data） | B3 |
| `controllers/NoteController.go:152`（`mode.dev`） | `revel.Config` | run mode 注入（prod=false，dev/test 按 section） | B3 |
| `controllers/BlogController.go:116,168`、`lea/blog/Template.go:104,115`（主题渲染读取、`results.chunked`、`DevMode`） | `revel.Config`/`BasePath`/`DevMode` | 用户主题基路径经 content resolver；`results.chunked` 固定为 false（Revel 专用键，B9 移除）；run mode 注入 | B4 |
| `service/ThemeService.go:112,124,1183`（主题写入） | `revel.BasePath` | `ContentRoots.PublicUpload.Data` | B4 |
| `controllers/admin/AdminData.go`、`service/ConfigService.go:551,657-664,726-738,794-813,876`（备份根、db.* 连接信息） | `revel.Config`/`BasePath` | `ProductionConfig.BackupRoot`/`DatabaseName`/`DatabaseIdentity`/`CredentialProviderRef` | B6 |
| `lea/i18n/i18n.go:42,77,249,296`（默认语言、messages 目录、locale cookie 名） | `revel.Config`/`BasePath`/`CookiePrefix` | B1 的 `LocaleResolverFromConfig` + `i18n.LoadMessages(dir)`；Revel 分支随 B9 删除 | B7 |
| `lea/Debug.go`（`AppLog`） | `revel.AppLog` | 第一方 logger | B7 |
| `db/Mgo.go:77-89`、`db/mongo_client.go:28-39`（dev 连接、超时键） | `revel.Config` | dev/test run mode 的配置读取 | B8 |
| `app/init.go`、`TestE2eController.go`、`app/tests/harness` | Revel 运行时 | 删除或切换到新入口 | B8/B9 |
