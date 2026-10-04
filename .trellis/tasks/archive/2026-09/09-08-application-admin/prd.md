# 应用层：管理与运维业务 — PRD

## 1. 目标

把管理端权限、全局配置、备份/恢复、升级、邮件投递和 Suggestion/feedback 收敛到一个可恢复、可观测的应用层边界。保留已有管理 URL、`info.Re`/JSON 外形和公开 `/suggestion` 兼容行为；任何数据库、文件、邮件或升级副作用失败都不能返回伪成功，也不能泄露凭据。

本任务已经按业务轨道选为下一个 ready 叶，但“依赖已完成”“任务已激活”“功能已实现”和“真实环境验收通过”是四个独立状态。当前 Mongo、HTTP、SMTP、浏览器和故障注入证据仍须按验收矩阵实际运行。

## 2. 当前基线与依赖

- 父任务 `09-08-business-layer-architecture/design.md` 的应用轨道是 `identity → notes → content → publishing → admin`。
- 直接消费的已归档契约为 `domain-contracts`、`infrastructure-persistence`、`application-identity`、`application-notes`、`application-content` 和 `application-publishing`；它们的任务状态均为 `completed`，但归档标签不替代跨层运行证据。
- `/suggestion` 由通配路由映射到 `Index.Suggestion`，当前主站拦截器将其列为公开 action；匿名请求会写入零值 `UserId`。此兼容基线暂保留，改为登录必需需另有产品决定。
- `SuggestionService`/`Index.Suggestion` 现已具备同步 feedback、receipt 和 outbox 持久边界；公开匿名入口与旧 `Re` 外形仍保留。真实 Mongo/SMTP/HTTP 证据仍按验收矩阵执行。
- `ConfigService.Backup/Restore` 现使用 allowlist、`exec.CommandContext` argv、受控 stdin、root containment、保护备份和数据库身份摘要；Download 失败返回显式错误状态。真实 mongodump/mongorestore、Mongo 故障注入和归档证据仍未运行。
- 评论 outbox 已在当前代码中存在：`app/db/outbox.go` 有 `unconfirmed`、`pending`、`retry`、`claimed`、`handoff_pending`、`handed_off`、`sent`、`cancelled`、`dead`、`handoff_unknown` 状态，`BlogService.Comment/DeleteComment` 已创建、确认和取消 comment 事件。旧规格中“comment outbox 尚未实现”的表述已过时；当前仍缺显式 comment/recipient 元数据索引、批量取消的完整读回/对账和真实 Mongo/SMTP 证据。
- `application-content` 独占 PDF renderer 的进程、argv、sandbox、timeout、输入输出和 cleanup；本任务只负责 admin 配置的绝对路径/regular-file/allow-policy 校验和 descriptor 交接。
- `interface-http` 独占 production-config 来源、secret/Mongo 校验、bind/dial/healthz 与退出码 78；本任务只消费该 seam，不复制解析器。

## 3. 范围与责任

| 领域 | 本任务拥有 | 明确交给其他任务 |
| --- | --- | --- |
| 管理权限 | admin service/adapter 的 principal 检查、未授权副作用拦截、错误映射 | identity 提供 principal/demo 事实来源；interface 提供 HTTP binder/route |
| 配置 | ConfigService 全局设置读写、缓存与持久化一致性、敏感字段脱敏、PDF descriptor/allow-policy | interface 的 production-config producer；MOD-003 消息解析器替换 |
| 数据维护 | mongodump/mongorestore、备份元数据、恢复/删除/下载边界及可恢复失败语义 | infrastructure 提供 Mongo/索引/事务边界；delivery 做真实 Mongo/文件系统故障注入 |
| 升级 | `UpgradeBlog`、Beta2、Beta4 的幂等 checkpoint、逐步失败和读回 | notes/domain 提供模型与 USN 契约；delivery 提供跨版本/真实数据库证据 |
| 反馈 | `/suggestion` 的匿名策略、输入校验、重复身份、持久成功和 feedback outbox | interface 保留 route/HTTP 外形；notes 只保留 handoff 说明 |
| 邮件 | admin 广播、feedback 邮件和 publishing comment transport 的 outbox 编码、worker、模板、重试/dead、取消读回与脱敏日志 | identity 的注册/找回/改邮箱业务规则；publishing 决定评论收件人、评论/计数/通知意图共同提交与删除权限 |

现有账号激活、找回密码、改邮箱和邀请邮件的身份规则不在本任务重新定义；本任务不得让 admin 的新状态机误处理这些历史 event，但要提供兼容回归。`conf/routes` 的完整标准库 HTTP 迁移、真实页面 smoke 和浏览器证据由 `interface-http`/`presentation-frontend`/`delivery-verification` 负责。

## 4. 需求与不变量

### ADM-AUTH — 权限与副作用顺序

1. 每个 admin action 在 service 和 HTTP adapter 都必须使用统一 authenticated admin principal；匿名、普通 member、demo 或 principal 解析/storage 失败均 fail closed。
2. 权限判定必须先于 Mongo 写入、文件创建/删除、升级步骤、outbox enqueue 和 SMTP 调用；未授权请求不得留下可观察副作用。
3. 保持当前 `Re`/重定向/状态码外形；transport 200 不能代替业务 `Ok`。错误分类至少区分 `not_authenticated`、`forbidden`、`validation`、`not_found`、`storage`、`partial_write`、`side_effect` 和 `unknown_result`。

### ADM-CONFIG — 配置单一事实来源

1. 管理可变设置只能经 ConfigService 一个 seam 读写；每个多键 action 必须记录逐键结果，任何键失败都返回失败并使内存缓存与 Mongo 读回一致，不能用连续赋值覆盖先前错误。
2. 初始化、读取和更新的 Mongo 错误不得变成空配置或默认成功；启动/读回失败必须保留可定位的脱敏错误。
3. production-config 只消费 `interface-http` 的已验证结构化 contract；admin 不读取环境变量、解析 secret/Mongo URL 或重新定义 bind/healthz。
4. SMTP 密码、demo 密码、Mongo 凭据、action token 和 callback secret 不得出现在 admin JSON/模板的通用配置投影、日志、错误、summary、artifact 或测试 fixture；管理页面若需显示设置，只能显示脱敏占位符，提交空占位符不得清空原值。

### ADM-DATA — 备份、恢复、删除与下载

1. `mongodump`、`mongorestore` 以及 PDF executable 只能来自配置的 allowlist；路径必须是绝对路径、regular file、可执行且经 canonicalization，拒绝相对路径、目录、符号链接逃逸、shell 元字符和 allowlist 外路径。
2. 执行必须使用 `exec.CommandContext` 的 argv 和有界 timeout，禁止 `/bin/sh -c`；数据库用户名/密码仅可经工具支持的受控 stdin、credential helper 或 `0600` 短生命周期文件传递，不进入 argv、环境快照、日志或错误文本。
3. 备份根目录由应用拥有且必须在该根目录内；`createdTime` 是不透明的已登记标识，恢复、删除、下载必须先从持久元数据解析并重新验证路径，不能拼接任意请求字符串或 `RemoveAll` 任意目录。metadata 必须绑定 canonical `ConfiguredDatabaseIdentity`（scheme/clusterHost/clusterPort/databaseName/authSource/tlsMode，长度前缀 UTF-8 后 SHA-256 digest），ProductionConfig 只提供 interface-owned typed `CredentialProviderRef`，admin 不重新解析 URL/secret。
4. Backup 的目录创建、命令退出、输出/错误、元数据写入和读回组成可观察结果；命令失败、超时、进程丢失或元数据写入未知时返回明确失败/`unknown_result`，不写入可用备份记录。Restore 必须先完成并确认自动保护备份，再执行目标恢复；任一步失败不得报告成功。
5. Delete/Download 必须拒绝路径越界、符号链接、缺失/部分文件和超出资源预算的归档；打包使用安全相对名、文件数/总字节上限、逐文件错误返回和 cleanup，不能 panic 或把空响应当成功。删除成功后再次删除是可观测幂等结果，读回失败不得假报已删除。保留清理用 owner lease/CAS，只按 confirmed metadata 的 created time/opaque id 稳定淘汰未保护、未运行中的最旧副本；保护备份是当前库完整 manifest 快照，孤儿目录/单个超限只报告 partial，不扩大删除范围。
6. PDF 在本任务只输出不含 secret/token 的 typed descriptor 和 allow-policy；renderer 的 argv、sandbox、timeout、artifact、local-file/网络限制和 cleanup 由 `application-content` 唯一验收。

### ADM-UPGRADE — 数据升级

1. `UpgradeBlog`、`UpgradeBetaToBeta2`、`UpgradeBeta3ToBeta4` 的版本标记和每一步必须可重试、幂等且绑定目标数据；`upgradeCheckpoints` collection 以 `(OperationId,StepKey,TargetScope)` 唯一，checkpoint 以 `InputDigest`、version、lease/fencing 和 result digest 记录，并通过 expected-version CAS 防止并发重复执行；重复调用不得重复插入、回拨 USN 或删除未预期索引。
2. 每条批处理记录成功/失败/跳过数量和第一失败原因；查询、更新、索引、模板或 config 写入错误不能被忽略。跨文档部分完成时返回 `partial_write`/`unknown_result` 并保留可恢复 checkpoint，不返回 `Ok:true`。
3. 升级只消费 domain/notes/persistence 已定义的模型和所有权条件，不在 controller 复制业务状态机；真实跨版本/重启证据由 delivery 记录。

### ADM-FEEDBACK — Suggestion/feedback

1. 保留 `/suggestion` 的现有 public route、`info.Re` 外形和匿名兼容基线；匿名时 `UserId` 为零值，登录时绑定 authenticated UserId，不能从请求字段伪造其他用户。
2. `Suggestion` 必须在写入前拒绝缺失/空白、非法 UTF-8、超过已确认的 rune/byte 上限和不允许的控制字符；允许 `LF (U+000A)`、`CR (U+000D)`、`TAB (U+0009)`，拒绝其余 C0 控制字符 `U+0000–U+001F` 以及 `DEL (U+007F)`。`Addr` 可为空，非空时必须是单个 addr-spec 且不超过 254 字节，并拒绝显示名、CR/LF 和控制字符。`Suggestion` 不超过 2000 个 Unicode 码点和 8192 字节。原始文本按 Unicode 保存，HTML 只在最终邮件/HTML 输出边界转义，不能把用户文本拼入模板或邮件 header。
3. 声明为 retry-safe 的新客户端提交必须有稳定重复身份；登录请求使用 principal `ActorId`，匿名携带 `submissionId` 直接 validation 拒绝，匿名旧请求无 ID 仍可提交且 `RetrySafe=false`，不建立 receipt identity、不参与唯一索引，也不按正文/时间猜测幂等。相同 actor、身份、目标和正文重试返回原结果且不重复写入/发信；同身份改正文/目标返回 conflict；不同 actor 不得互相读到结果。
4. Feedback receipt 必须写入 `feedbackReceipts` collection，字段含 BSON `SchemaVersion:int32`、`Kind/ActorId/SubmissionId/State/TargetDigest/BodyDigest/ErrorCategory/ReconciliationId:string`、`RetrySafe:bool`、`RecipientSnapshot:[]string`、`OutboxIds:[]string`、`CreatedAt/TerminalAt/ExpiresAt:date`；唯一索引 `feedback_receipt_actor_submission_kind_uq` 使用 `{ActorId:1,SubmissionId:1,Kind:1}`，partial filter 为 `{"Kind":"feedback","RetrySafe":true,"SubmissionId":{"$exists":true}}`。snapshot 不随配置变更，terminal receipt 至少 30 天后由 GC 删除并 read-back。
5. feedback 文档、`feedbackReceipts` receipt 与必要 outbox 事件必须在同一事务或持久补偿边界内提交，并逐项 strict read-back；只有三者均已确认写入才返回 `Ok:true`。反馈已提交后 SMTP transport 失败只影响 outbox 状态，不回滚反馈或 receipt；enqueue/读回失败返回 `side_effect`/`partial_write` 或 `unknown_result`，并提供可对账 receipt。请求路径不得启动无结果 goroutine。

### ADM-MAIL — 管理邮件与模板

Broadcast identity：新请求必须携带 32 位小写十六进制 `batchId`；旧无 ID 请求生成 reconciliation ID，在 receipt/旧 `Re.Id` 字段返回并标记 `RetrySafe=false`，不得宣称可重试。批次按 `(ActorId,batchId,Kind=broadcast)` 去重，事件按 `(batchId,recipientId)` 固定 identity；同 digest replay，改目标/正文冲突且零新增事件。

1. admin 对用户群发、feedback 和 comment 事件均先持久化 outbox，再由 worker 发送；返回值表示 enqueue/receipt 是否成功，不把 SMTP 已发送当作请求成功前提，也不在请求路径 fire-and-forget。
2. 收件人、主题和 header 拒绝 CR/LF、非法地址；规范化、去重后的群发收件人最多 1000 个，超过上限时整批拒绝，空白行先过滤，非空 CR/LF 项仍拒绝。新请求携带 32 位小写十六进制 `batchId`（旧请求使用 reconciliation ID 并标记 `RetrySafe=false`），批次 receipt 以 `(ActorId, batchId, Kind=broadcast)` 去重，单个 event 以 `(batchId, recipientId)` 固定 identity；同 digest 重放，改目标/正文冲突且零新增事件。模板解析/渲染失败、SMTP 明确拒绝、超时和结果未知分别进入可重试/dead/`handoff_unknown`，错误脱敏且不撤销已经确认的业务写入。
3. 现有 `activate-email`、`reset-password` 等 event 的 payload/状态兼容；comment/feedback 必须有明确 `Kind`、稳定 identity 和 typed payload，不能让新状态机把旧账号 event 当成 comment。
4. EmailLog 或审计记录只保留完成验收所需的脱敏 metadata；不得写入 SMTP 密码、token、完整授权 URL 或未转义的用户敏感正文。

### ADM-COMMENT — publishing/admin 交接

1. publishing 拥有 comment ID、合法 recipient、评论/计数/通知意图共同提交和删除权限；admin 不重新查询发布权限、不生成评论、不在失败时补写评论。
2. 当前状态机的 `unconfirmed` 只能由 publishing 确认后变为 `pending`；`pending/retry → claimed → handoff_pending → handed_off → sent`，取消闸门前允许 CAS 到 `cancelled`，明确 transport 拒绝进入 `retry/dead`，交接结果不明进入 `handoff_unknown` 且禁止自动重发。非法迁移必须拒绝并可读回。
3. 新事件必须持久化 typed `CommentId`/`RecipientId` 元数据和稳定唯一键；不能只把 recipient 放在不可查询的 payload 或仅依赖 Mongo 返回顺序。历史事件缺少元数据时只允许有界、可验证的兼容解码，不得静默重写或误发。
4. 删除先于持久发送闸门时，pending/retry/已领取但未交接事件全部停止；闸门或发送先赢不承诺召回。批量取消必须逐事件 CAS、读回并留下对账结果；任何失败/未知不得返回“已取消且不会发送”。`TransportHandedOffAt` 只表示 SMTP 调用前的持久闸门，不代表外部接受。
5. 当前代码已覆盖部分状态和内存/Mongo focused tests，但 comment identity/index、取消批量读回、真实 Mongo/SMTP 竞态和人工 `handoff_unknown` 对账仍是未完成验收，不得把现有单测标为本任务完成。

## 5. 交互、输入输出与失败流程

匿名 identity 规则：匿名请求携带 `submissionId` 直接返回 `validation`，不得写 receipt/outbox；匿名旧请求无 ID 仍可提交，`RetrySafe=false`，不参与唯一索引。

1. **管理 action**：HTTP adapter 绑定输入 → admin principal 检查 → service validation → durable mutation/outbox → strict read-back → 映射既有 `Re`/template/binary response。任何早期失败都不得创建文件、写 Mongo 或发送邮件。
2. **Suggestion**：请求带 `Addr`/`Suggestion`（旧客户端可能只带 `Suggestion`）和可选稳定提交身份；`Addr` 为空允许，非空必须是单个 addr-spec、最多 254 字节且拒绝 CR/LF/控制字符；`Suggestion` 必须是合法 UTF-8、去首尾空白后非空、最多 2000 个 Unicode 码点和 8192 字节。service 保留正文、只用 trim 做校验 → 在持久边界写 feedback + outbox + receipt → 返回原 `Re`。登录新客户端使用 32 位小写十六进制 `submissionId`；匿名携带 ID 直接 validation 拒绝，匿名旧请求无 identity 时可提交但标记 `RetrySafe=false`，不按正文/时间猜测去重。transport 只改变 outbox 状态。
3. **Backup/Restore**：admin principal → 已登记 backup 元数据解析 → 专用非公开 root/allowlist/预算校验 → 命令执行 → 退出/文件/元数据读回。默认 root 保留为应用数据目录下的 `mongodb_backup`；最多保留 30 份或 20 GiB，单次下载最多 100,000 个文件、源文件 10 GiB、生成归档 4 GiB。Restore 只允许同一 configured database identity，先自动保护当前库；保护失败即停止。Download 只读取已验证目录并安全生成 tar.gz，任何文件错误返回显式失败。
4. **Comment**：publishing 提供已确认事件 → admin worker lease/CAS → 模板安全编码 → handoff gate → SMTP → `sent/retry/dead/handoff_unknown`；删除流程与 worker 竞争时以持久 CAS 为唯一裁决。

## 6. 兼容性

- 保留 admin 现有 action 名称、通配路由、旧 `Re` 字段/错误外形和 `/suggestion` public 入口；`submissionId` 是兼容可选输入，未携带时不得宣称 retry-safe。群发和 feedback 请求在 durable enqueue/receipt 确认后保持既有 HTTP 200/`Re` 外形，`Re.Ok=true` 表示已入队，不表示 SMTP 已接受。
- 保留历史 `Config`/`Suggestion`/`EmailLog` BSON key；不批量改写旧文档，不用新索引删除或覆盖历史冲突，索引 preflight 失败必须阻止就绪并给出脱敏冲突数量。
- 旧账号邮件 event 继续由 identity/persistence 合同处理；admin comment/feedback event 的新增字段必须能读旧事件且不能扩大收件人或权限。
- 本任务不删除 Revel 入口；完整标准库 HTTP 迁移由 interface 任务拥有。真实页面、Mongo 拓扑、SMTP、浏览器、PDF 和发布证据只在对应 owner 的矩阵中计为通过。

## 7. 验收标准

- [ ] **AC-A1 权限与 wire**：逐 action 覆盖 admin controller/service 的身份矩阵、未授权零副作用、`Re`/重定向/状态码兼容；adapter 真实 HTTP smoke 由 interface/delivery 执行。
- [ ] **AC-A2 配置**：初始化/更新错误、逐键失败、缓存读回、secret 脱敏和 production-config consumer mapping 有 focused contract；不重复解析 interface seam。
- [ ] **AC-A3 数据维护**：backup/restore/delete/download 覆盖 allowlist、绝对 regular file、argv/timeout、凭据不泄露、root containment、symlink/traversal、预算、partial/unknown、保护备份和严格 read-back；PDF 仅验 descriptor，renderer 证据由 content 接收。
- [ ] **AC-A4 升级**：三类升级的 idempotency/checkpoint、重复调用、逐条错误、USN/索引/配置失败和 restart resume 有 service/DB contract；未运行的跨版本/真实 Mongo 证据保持 `unrun`。
- [ ] **AC-A5 feedback**：匿名/登录边界、非法 UTF-8/空白/控制字符/显式 rune+byte 上限、HTML/header escaping、稳定 identity/conflict、feedback 文档+receipt+outbox durable success、transport failure、retry/dead 和无 goroutine 均有 service/DB/HTTP contract。
- [ ] **AC-A6 邮件**：admin broadcast、feedback、comment 的 enqueue/lease/CAS/模板/CRLF/recipient/重试/dead/未知交接/日志脱敏覆盖；旧账号 event 独立回归，SMTP/Mongo 故障注入由 delivery 补充。
- [ ] **AC-A7 comment 交接**：所有状态合法/非法迁移、确认闸门、取消与领取竞态、批量读回、handoff_unknown 对账、metadata/index 和旧事件兼容均有双方证据；publishing 的 AC-PB4-NOTIFY/DELETE 未经双方证据不得标通过。
- [ ] **AC-A8 敏感信息与审计**：源码、日志、模板、summary、artifact、环境快照和 EmailLog 均无密码/token/完整授权 URL；失败只记录脱敏 category、event identity digest 和可重试线索。
- [ ] **AC-A9 证据完整性**：每项记录命令、fixture、Go/Mongo/SMTP/OS、discovered/executed/pass/fail/skip、退出码、commit/run 摘要；不能用 `go test`、`go build` 或 task status 代替真实 HTTP/Mongo/SMTP/browser/failpoint 证据。

## 8. 已确认决策及影响

用户于 2026-09-25 确认全部推荐方案，Q-A1～Q-A6 现作为实现和验收合同：

| 编号 | 已确认合同 | 主要影响 |
| --- | --- | --- |
| Q-A1 | `Addr` 是可选联系邮箱；空值允许；非空必须是单个 addr-spec，最多 254 字节，拒绝显示名、CR/LF 和控制字符。`Suggestion` 必须合法 UTF-8、trim 后非空，最多 2000 Unicode 码点和 8192 字节；只 trim 校验，不改写正文。 | 固定 BSON/HTTP 输入验证、拒绝码、HTML/header escaping 和 fixture 边界。 |
| Q-A2 | 登录新客户端使用 128-bit 随机、32 位小写十六进制 `submissionId`；匿名携带 ID 拒绝，唯一键为 `(ActorId, SubmissionId, Kind=feedback)` 仅适用于登录 receipt。同 identity 同摘要回放原 receipt，改目标/正文冲突且零写入。匿名无 ID 的旧请求仍可提交，但 `RetrySafe=false`，不猜测去重；终态 receipt 保留至少 30 天。 | 固定 route binder、receipt schema、唯一索引和重试语义。 |
| Q-A3 | 使用独立 `feedbackRecipients` 内部收件人列表，启动和更新时校验、去重并限制最多 20 个；`Addr` 不直接作为 SMTP `To`，默认不作为 `Reply-To`。收件人缺失/无效时在 feedback 写入前 fail closed。 | 固定配置 seam、投递安全和无收件人错误行为；只保存 feedback 的模式需另行显式配置。 |
| Q-A4 | feedback 与 admin broadcast 在 outbox/receipt durable 写入并读回确认后返回既有 HTTP 200/`Re.Ok=true`，语义为“已入队”；SMTP 由 worker 处理。部分/未知写入返回失败和可对账 ID，SMTP 失败不回滚已确认业务写入。 | 固定旧 wire、前端提示、EmailLog 状态和 worker 边界。 |
| Q-A5 | 默认使用应用数据目录下的非公开 `mongodb_backup` root；最多 30 份或 20 GiB。单次 Download 限制 100,000 文件、源文件 10 GiB、生成归档 4 GiB。Restore 仅允许同一 configured database identity，必须先确认保护备份。 | 固定 root containment、GC/容量、归档预算、恢复安全和真实 Mongo 验收。 |
| Q-A6 | 新 comment event 写入 typed `CommentId`/`RecipientId`/`IdempotencyKey`/`EventVersion` metadata，并建立带类型过滤的唯一索引。旧 payload-only event 只读兼容；缺失/冲突拒绝发送并进入 `handoff_unknown`/对账，不启动时静默回填。迁移采用 preflight、dry-run、分批 CAS、读回和保留旧字段的阶段方案。 | 固定 schema/index preflight、历史兼容、重复投递风险和可回滚迁移。 |

上述合同已解除规格决策阻塞，但真实 Mongo、HTTP、SMTP、浏览器和故障注入证据仍须按 AC-A1～A9 执行后才能标记通过。

## 9. 非目标

不新增管理能力、不自动部署生产、不保存明文生产凭据、不替换消息解析器、不重写旧评论/feedback 正文、不实现 PDF renderer、不迁移完整 HTTP runtime、不用静默 fallback 掩盖配置/数据库/SMTP 失败。
