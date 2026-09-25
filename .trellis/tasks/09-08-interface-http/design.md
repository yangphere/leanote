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

## 3. Request pipeline

```text
Recover → Gzip → /healthz → OnRequest(CheckMongoSessionLost) → RouteTable.Match
  → Static | registry.Lookup → Session decode (旧/损坏 → 匿名) → Locale
  → Context{Params, Session, SessionReader/Writer, Principal policy}
  → BEFOREs → Handler → Session commit (先于响应头) → Result.Apply
```

- 方法（D-H5）：HEAD 映射 GET；identity 矩阵外方法在 registry 层返回 405 并带 `Allow`；显式 GET/POST 路由方法不符由 route table 返回 404；`*`/catch-all 不限方法。
- Commit 失败：失败结果原样返回，成功结果改写为 `session_commit` 失败 envelope（已有 `applySessionCommitFailure`）；P-07 注册路径改用 `registered_relogin_required`。

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

受控 wire 例外只有：D-H6（`GetImages`/`GetAlbums` 依赖失败 500 + legacy body）和上游已决的 Q-P5/Q-P9；每项都要更新 Golden 并在 evidence matrix 标注来源决策。

## 6. Rollback

先完成 harness/Golden 切换再删除 Revel；每个批次（implement.md B0–B9）单独提交，失败回滚该批提交。Revel 删除（B9）是最后一个批次，前置条件是 B8 在新入口上全部门禁通过；任何不完整迁移不引入静默双栈。
