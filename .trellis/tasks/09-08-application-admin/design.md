# 应用层：管理与运维业务 — 技术设计

## 1. 边界与依赖

```text
identity principal/demo policy ─┐
domain/info + persistence ──────┼─> admin application services
notes/content contracts ────────┤        │
publishing comment intent ──────┘        ├─> durable outbox / worker ──> SMTP
interface production-config ────────────┘
```

- `AdminBaseController`/HTTP adapter 只做 principal、输入 presence、旧响应映射和模板/二进制写出；不得持有 Mongo client、文件删除状态机或邮件 worker。
- Config/Upgrade/Feedback/Email service 负责业务决策、持久化边界和结果分类。Mongo/BSON/索引/事务能力来自 `infrastructure-persistence`；纯领域类型来自 `domain-contracts`。
- `application-identity` 提供 authenticated admin principal、demo 单一事实来源和账号邮件事件兼容；admin 不复制 session/token 解析。
- `application-notes` 仅交接 feedback 的 owner 边界；`application-content` 提供 PDF renderer/文件根 primitive；`application-publishing` 提供 comment 的合法收件人、共同提交和删除结果；`interface-http` 提供唯一 production-config producer。

## 2. 现有实现基线与需要收敛的差异

| 组件 | 当前事实 | 设计结论 |
| --- | --- | --- |
| `ConfigService` | 配置更新有逐键 preflight/read-back；Backup/Restore 使用 allowlist、argv、受控 stdin、root containment 和保护备份 | 继续补齐总量预算、同库身份摘要和真实工具/故障注入证据；失败不保留未确认 cache |
| `SuggestionService`/`Index.Suggestion` | 已同步持久化 feedback、receipt 和 outbox；公开 action 保留旧 `Re` | 继续保持 durable boundary；错误映射只返回稳定分类键，真实 Mongo/SMTP/HTTP 仍由验收矩阵覆盖 |
| `EmailService` | comment outbox renderer/SMTP context 已存在；批量 admin 邮件仍有 goroutine；旧账号 event 与 comment 共用 worker 入口 | 按 `Kind` 分离 typed event；admin broadcast/feedback 走 durable enqueue；旧 event 继续使用其既有 payload/状态 |
| `db.OutboxEvent` | 新 comment event 写入 `CommentId`/`RecipientId`/`EventVersion` typed metadata；历史事件仍可能只有 payload 和可选 key | typed 字段建立 partial unique/status 索引；旧事件只读兼容解码并校验 key 与 aggregate/recipient 一致；取消必须 CAS + read-back |
| `BlogService.Comment/DeleteComment` | 已有 unconfirmed→confirmed、取消和 workspace mutation；真实 Mongo/SMTP 竞态仍未完成 | publishing 保持业务 owner；admin 只验证 transport、状态机、索引、对账和旧 event 不退化 |
| admin templates/controllers | `GlobalStringConfigs` 会被直接放进 admin view；多键写入覆盖 `re.Ok`；AdminData 下载可 panic/路径未验证 | 统一 secret redaction、逐键错误聚合、严格 binary/archive 结果；不以旧模板行为掩盖敏感泄露 |

## 3. 配置与生产 seam

1. `ProductionConfig` 是 interface-http 输出的只读结构化输入。admin 只能读取其已验证字段，不从 `revel.Config`、环境变量或请求重新解析来源、secret、Mongo URL、bind 或 healthz。
2. `ConfigService` 的可变设置更新采用 `ConfigMutation`：先规范化并校验所有键，再以事务或明确的补偿/版本写入 Mongo，成功后严格读回并原子替换内存快照；失败保留旧快照并返回分类错误。
3. secret key（SMTP/password/demo/action token/Mongo credential）有统一 redaction policy；通用配置投影和错误只能返回 `configured=true`、长度/摘要等非可逆信息。空的 masked value 表示“不修改”。
4. PDF `ExecutableDescriptor` 只含经过 canonicalization/regular-file/allowlist 校验的路径、版本/摘要和 policy id，不含 callback secret、token、Mongo credential 或可执行参数。descriptor 交给 content，不在 admin 启动进程。

## 4. Backup/Restore 数据流

```text
admin principal
  -> parse registered backup id
  -> validate root + executable + budget
  -> build argv + credential channel
  -> CommandContext(timeout)
  -> verify exit/files/digest
  -> persist metadata + strict read-back
```

- backup metadata 保存 opaque id、canonical root-relative path、结构化 `ConfiguredDatabaseIdentity`（canonical `scheme/clusterHost/clusterPort/databaseName/authSource/tlsMode`，按 UTF-8 长度前缀拼接后 SHA-256 digest；不含密码）、created time、size/digest、状态和脱敏错误；不保存命令行或凭据。`ProductionConfig` 只通过 interface-http 提供已验证的 identity 与 typed `CredentialProviderRef`（provider kind + opaque handle，owner 为 interface/infrastructure，admin 仅消费），不重新解析 Mongo URL/secret。
- 默认 backup root 是应用数据目录下的非公开 `mongodb_backup`；root 本身不可由请求输入，最多保留 30 份或 20 GiB。保留清理按 confirmed 记录的 created time/opaque id 稳定排序，只删除未保护、未运行中的最旧记录，并在删除后逐项读回；无法确定状态即停止并报告 partial/unknown。单次 Download 限制为最多 100,000 个文件、源文件总计 10 GiB、生成归档 4 GiB；超限在创建可下载产物前失败。
- restore 先创建并确认保护备份（当前 configured database 的完整 metadata/manifest 快照，状态 `protected`，带 operation/lease identity），再读取目标 metadata；保护备份失败即停止。目标路径必须仍在 configured root 内并通过 regular-file/symlink/manifest 校验。
- restore 只允许 metadata 中的 configured database identity 与当前运行身份完全一致；createdTime 只解析为 opaque backup id，不参与路径拼接。
- Download 只消费已确认 metadata，遍历目录时拒绝 symlink、`..`、绝对名、目录逃逸、文件数/字节超限和中途消失；tar header 使用 canonical relative name，任何错误都关闭/删除临时产物并返回失败。
- retention GC 领取带 owner/lease 的 metadata CAS，跳过 `protected`、运行中和最近恢复目标；删除 metadata 与目录后逐项 read-back，孤儿目录或单个备份超限只标记 `orphan/over_budget` 并继续报告 partial，不扩大删除范围。Delete 使用同一 metadata/manifest 和 owner-scoped CAS；删除成功后读回确认目录和记录状态。重复删除返回可区分的 already-deleted/not-found，不把未知删除映射为成功。

## 5. Feedback 与 admin 邮件

### 5.1 Feedback 提交

1. adapter 保留 `/suggestion` 的旧 `Re`，把缺失字段和 presence 交给 service。`Addr` 为空允许；非空必须是单个 addr-spec、最多 254 字节且拒绝显示名、CR/LF 和控制字符。`Suggestion` 必须合法 UTF-8、trim 后非空、最多 2000 个 Unicode 码点和 8192 字节；trim 只用于校验，不改写保存正文。
2. 新客户端使用 128-bit 随机、32 位小写十六进制 `submissionId`；以 `(ActorId, SubmissionId, Kind=feedback)` 为主键。已登录请求的 `ActorId` 是 authenticated principal；匿名请求携带 `submissionId` 直接返回 `validation`，不得创建 receipt/outbox。匿名旧请求无 ID 仍保持旧 wire 行为并标记 `RetrySafe=false`，不参与唯一索引且不能按正文/时间猜测去重。receipt 冻结目标和正文 digest；终态 receipt 保留至少 30 天。
3. Feedback receipt 使用 BSON 版本化结构：`SchemaVersion:int32`、`Kind/ActorId/State/TargetDigest/BodyDigest/ErrorCategory/ReconciliationId:string`、可选 `SubmissionId:string`、`RetrySafe:bool`、`RecipientSnapshot:[]string`、`OutboxIds:[]string`、`CreatedAt/TerminalAt/ExpiresAt:date`。collection 为 `feedbackReceipts`，唯一索引 `feedback_receipt_actor_submission_kind_uq` 的 keys 为 `{ActorId:1,SubmissionId:1,Kind:1}`，partial filter 为 `{"Kind":"feedback","RetrySafe":true,"SubmissionId":{"$exists":true}}`；匿名无 ID 不进入该索引。`RecipientSnapshot` 在入队前冻结，后续配置变更不得改变已入队目标；terminal receipt 由受控 GC 在 `ExpiresAt <= now` 时删除并逐条 read-back。
4. Mongo transaction 可用时，feedback 文档、`feedbackReceipts` receipt 与 `Kind=feedback` outbox 同事务提交，并对三者 strict read-back；standalone 只能在 persistence 提供持久 receipt/补偿时执行，否则返回 `partial_write`，绝不报告 `Ok:true`。提交成功后 SMTP 只更新 outbox，不回滚反馈或 receipt。
5. feedback 邮件 body 在 HTML sink 统一 escape；`Addr` 不直接成为 SMTP `To`，默认不作为 `Reply-To`。`feedbackRecipients` 是独立内部收件人列表，最多 20 个，启动和更新时规范化、校验、去重；缺失或无效时在 feedback 写入前 fail closed。subject/to 拒绝 CR/LF。

### 5.2 Admin broadcast

旧请求缺失 `batchId` 时生成的 reconciliation ID 必须写入批次 receipt，并在旧 `Re.Id` 字段返回；该请求标记 `RetrySafe=false`。

批量收件人先规范化、去重、限量；新客户端必须提供 32 位小写十六进制 `batchId`，旧请求缺失时由服务端生成一次性 reconciliation ID、写入 receipt 并在旧 `Re.Id` 字段返回，标记 `RetrySafe=false`，不得宣称可重试。以 `(ActorId, BatchId, Kind=broadcast)` 建立批次 receipt，并以 `(BatchId, RecipientId)` 固定每条 event identity。相同 batch/body/recipient digest 重放原批次结果，改目标或正文返回冲突且零新增事件，再逐事件持久化 outbox；所有事件和 receipt 读回确认后保持 HTTP 200/旧 `Re` 外形，`Re.Ok=true` 只表示“已入队”。worker 同步 lease/CAS 发送，transport 明确拒绝进入 retry/dead，未知进入 handoff_unknown；旧 `EmailLog` 不写密码/token/完整 body。部分或未知持久化结果返回失败和可对账 ID，不启动请求 goroutine。

## 6. Comment outbox 交接

### 6.1 状态与权限

publishing 的 comment mutation 负责 comment/计数/recipient intent 的共同确认。admin 只接受已确认 event，并执行：

```text
unconfirmed --confirm--> pending
pending/retry --claim--> claimed --preflight--> handoff_pending
handoff_pending --persist gate--> handed_off --transport ok--> sent
handoff_pending/handed_off --reject--> retry | dead
pre-gate --cancel CAS--> cancelled
unknown transport --> handoff_unknown (no automatic retry)
```

每一步以 `_id + Kind + Status + Version + LeaseId + CancelRequested` CAS，版本递增；`TransportHandedOffAt` 是 SMTP 调用前的持久闸门，不是外部接受证明。取消与 worker 竞态必须以持久 CAS 读回裁决，不能依赖请求开始时间或内存快照。

### 6.2 身份与兼容

- 新 comment event 的 BSON typed metadata 至少包含 `CommentId:string`、`RecipientId:string`、稳定 `IdempotencyKey:string`、`EventVersion:int32` 和 aggregate owner/target digest；对 typed 字段建立 partial filter `{"Kind":"comment","CommentId":{"$exists":true},"RecipientId":{"$exists":true},"IdempotencyKey":{"$exists":true},"EventVersion":{"$exists":true}}` 的 `(Kind, CommentId, RecipientId)` 唯一约束及状态扫描索引，避免 legacy 空字段互相冲突。迁移/回写只允许带 `_id + Version + LeaseId` 的 CAS，读回必须确认 typed 字段和旧字段一致。
- 现有事件可能只有 `AggregateId` 和 payload `recipientId`。只允许严格、有界的兼容读取：两者均可验证且一致时继续，否则拒绝发送并进入 `handoff_unknown`/人工对账；启动时只做 preflight 统计，不静默回填或改写历史事件。迁移需 dry-run、分批 CAS、读回并保留旧字段，观察期后再评估移除兼容读取。
- 取消同一 comment 的多个 recipient event 时逐事件 CAS；若一个事件已 handoff/sent/unknown，结果必须报告“未完全取消”并保留对账列表，不能把之前已取消的部分回滚成假成功，也不能声称剩余事件安全。
- 账号激活/找回/邀请 event 使用原状态机和模板；comment 状态只由 `Kind=comment` 进入，feedback 使用独立 `Kind=feedback`。

## 7. 升级与错误语义

- checkpoint collection 为 `upgradeCheckpoints`，BSON 字段 `OperationId:string`、`StepKey:string`、`InputDigest:string`、`TargetScope:string`、`State:string`、`Version:int64`、`LeaseId:string`、`Fence:int64`、`ResultDigest:string`、`CreatedAt/UpdatedAt/LeaseUntil/CompletedAt:date`；唯一索引 `upgrade_checkpoint_operation_step_uq` keys `{OperationId:1,StepKey:1,TargetScope:1}`。每个升级入口先读 checkpoint，按 `OperationId + deterministic StepKey + InputDigest` 执行 idempotent steps；领取使用 expected-version CAS、短 lease 和单调 fencing token，第二个并发执行者只能读回同一 operation 结果。每步 verify/read-back 后才推进 checkpoint，推进后再次 read-back 校验 identity/digest。重复执行返回 already-applied 或成功读回，不重复写。
- 跨文档失败保存 operation/checkpoint 和脱敏 error category；恢复可继续执行未确认步骤，不能跳过失败或回拨 USN。controller 只映射 `Re`，不把 `UpgradeBlog` 当前无返回值当成成功证据。
- service 错误在边界只记录一次，包含 task/action、safe identity digest、category、retryable；不含 content、password、token、SMTP response 全文或命令行。

## 8. 回滚与发布

- 配置、backup、upgrade、feedback、comment transport 分别有 migration/rollback 点；索引 preflight、metadata schema 或 outbox 字段不兼容时停止就绪，不删除历史数据。
- 回滚不得取消已 handoff 的邮件、回拨 USN、恢复已删除的用户数据或清空可对账 receipt。comment/feedback 状态扩展先验证旧账号 event 解码和索引回滚。
- 不自动部署生产；真实 Mongo/SMTP、进程/文件系统 failpoint、HTTP、浏览器和 PDF artifact 只在 owner 验收通过后改变状态。

## 9. 已冻结合同

Q-A1～Q-A6 已由用户于 2026-09-25 全部采用推荐方案。实现可以按本设计进入编码，但 AC-A3/A5/A6/A7 仍须等待对应真实边界证据，不能以 task status、focused test 或静态检查代替。
