# 应用层：管理与运维业务 — 执行计划

## 0. 开始前门禁

- [x] 已获得 Q-A1～Q-A6 的明确产品/兼容答复；答复已写入 PRD、design、研究和验收矩阵。进入编码前仍须完成本清单的上下游门禁，不编辑超出任务边界的 `app/`、`conf/`、`public/` 或业务测试。
- [x] 读取归档的 domain/identity/notes/content/publishing/persistence 材料和 `.trellis/spec/backend/*`；确认 manifest 使用归档路径，不引用已经移动的活动目录；同时把本任务 `prd.md`、`design.md`、`implement.md`、`task.json`、`research/` 和 `acceptance/` 纳入上下文 manifest，避免实现/复核遗漏冻结合同。
- [x] 固定现有 admin action inventory、`/suggestion` 公开路由、旧 `Re`/redirect/status、BSON key 和错误映射作为实现前 baseline。

## 1. 认证、配置和管理 adapter

- [ ] 建立 admin principal/service seam，覆盖 anonymous/member/demo/admin、principal storage error 和未授权零副作用；保持 `AdminBaseController` 只做适配。
- [ ] 盘点 Config/Upgrade/Email service、admin controller/template、配置来源、缓存和日志调用；为每个多键更新建立逐键错误聚合、旧快照保留和严格 Mongo read-back。
- [ ] 接入 interface-http production-config consumer fixture；删除 admin 对环境来源、secret/Mongo URL、bind/healthz 的重复解析。
- [ ] 对通用配置投影、demo/SMTP/Mongo secret、token、callback secret 和 EmailLog 建立统一 redaction 断言；空 masked input 不得清空旧 secret。

## 2. 备份、恢复、下载与 PDF descriptor

- [ ] 以 Q-A5 冻结的 root/budget/retention 建立 backup metadata/manifest：非公开 `mongodb_backup` root、最多 30 份或 20 GiB、按 confirmed/未保护/未运行稳定淘汰最旧副本；GC 使用 owner lease/CAS，保护备份为当前库完整 manifest 快照，孤儿目录/单个超限只报告 partial；Download 最多 100,000 文件/10 GiB 源文件/4 GiB 归档；校验 canonical `ConfiguredDatabaseIdentity` 字段和长度前缀 SHA-256 digest。
- [ ] 将 mongodump/mongorestore 改为 `exec.CommandContext` argv + bounded timeout；凭据只经受控 stdin、credential helper 或 `0600` 临时文件，命令、环境、错误和日志均不含 secret。
- [ ] Backup 只在 command exit、文件清单/digest、metadata write/read-back 全部确认后返回成功；restore 先确认保护备份，失败分类为 `partial_write`/`unknown_result` 并可重试。
- [ ] Delete/Download 使用 owner-scoped manifest、safe relative tar names、文件数/字节预算、逐文件错误和 cleanup；禁止任意 `RemoveAll`、路径穿越、symlink、panic 和空响应伪成功。
- [ ] PDF 只生成 typed descriptor/allow-policy 并交接 application-content；不在 admin 启动 renderer 或复制其 argv/sandbox/timeout/cleanup。

## 3. Upgrade service

- [ ] 为 `UpgradeBlog`、Beta2、Beta4 建立 `upgradeCheckpoints` collection、`(OperationId,StepKey,TargetScope)` 唯一键、带 `OperationId/确定性 StepKey/InputDigest` 的 checkpoint、expected-version CAS、lease/fencing token、稳定批次顺序和 step verify/read-back；并发/重复调用只读回同一 operation，不重复插入、不回拨 USN、不删除意外索引。
- [ ] 为查询、更新、索引、模板、config 写入和中途崩溃建立错误分类、checkpoint/resume、逐条统计和 strict read-back；controller 不把无返回值动作当作 success。
- [ ] 补齐 service/DB contract 后，再交由 delivery 运行真实跨版本/重启/故障注入；未运行证据保留 `unrun`。

## 4. Feedback 与 admin broadcast outbox

- [ ] 按已冻结 Q-A1～Q-A3 实现 `Addr`/`Suggestion` presence：`Addr` 单个 addr-spec、254 字节、无显示名/CR/LF/控制字符；`Suggestion` 合法 UTF-8、trim 后非空、最多 2000 码点/8192 字节；完成 HTML sink escaping 和旧 public `Re` 映射，`Addr` 不成为 SMTP `To`。
- [ ] 引入可选 32 位小写十六进制 `submissionId`/receipt；登录使用 principal ActorId，匿名携带 ID 直接 validation 拒绝，匿名无 ID 仍可提交且 `RetrySafe=false`；`feedbackReceipts` 使用 BSON typed schema、`feedback_receipt_actor_submission_kind_uq` partial unique index，receipt 保存 digest、冻结 recipient snapshot、outbox IDs、状态/过期时间并由 30 天 terminal GC 清理；相同摘要回放原结果，冲突零写入。
- [ ] 让 feedback 文档、`feedbackReceipts` receipt 与 `Kind=feedback` outbox 进入同一事务或 persistence compensation boundary，并逐项 enqueue/read-back 确认才 `Ok:true`，transport 失败不回滚 feedback。
- [ ] 将 admin 用户群发从请求 goroutine 改为 durable outbox；新请求必须带 32 位小写十六进制 `batchId`，旧无 ID 请求生成并通过旧 `Re.Id` 返回 reconciliation ID 且 `RetrySafe=false`；批次 `(ActorId,batchId,Kind=broadcast)` 去重，事件 `(batchId,recipientId)` 固定 identity；`feedbackRecipients` 最多 20 个并在写入前校验、去重、冻结 recipients、subject/body digest，worker 同步 lease/CAS、retry/dead、日志脱敏；HTTP 200/`Re.Ok=true` 只表示已入队。
- [ ] 保持 identity-owned activate/reset/invite event 的独立 payload、状态和回归；禁止 comment/feedback handler 误处理旧 event。

## 5. Comment transport 交接

- [ ] 先消费 publishing 已确认的 comment intent；不重新判定 comment permission、recipient 或评论/计数共同提交。
- [ ] 将新 comment event 的 `CommentId`/`RecipientId`/`IdempotencyKey`/`EventVersion` 置于 typed metadata 和带类型过滤的唯一索引；为旧 payload-only event 实现严格、有界、冲突即拒绝的兼容读取，异常进入 `handoff_unknown`/人工对账，不启动时静默回写。迁移必须经过 preflight、dry-run、分批 CAS 和读回。
- [ ] 验证 `unconfirmed → pending → claimed → handoff_pending → handed_off → sent`、明确拒绝 `retry/dead`、未知 `handoff_unknown`、取消 CAS、lease reclaim、非法迁移和 version/lease fencing；`TransportHandedOffAt` 不当作 SMTP 接受。
- [ ] 取消按 comment/recipient 逐事件 CAS + read-back；覆盖 pending/retry/已领取未交接、交接先赢、重复删除、停用失败、未知提交和人工对账。任何部分/未知结果不得伪造“全部取消”。
- [ ] 与 publishing 联验 AC-PB4-NOTIFY/DELETE/HANDOFF，再交由 delivery 执行真实 Mongo/SMTP 竞态；未完成双方证据不得标通过。

## 6. 验证、复核和证据

- [ ] 每个 service/adapter 变更配套聚焦 regression；真实 HTTP 通过 interface/delivery 的 server boundary，禁止直接调用 controller 代替。
- [ ] 运行针对性 Go tests（每批 timeout ≤60s）、`go vet`、`go build`、Golden/manifest checks、`git diff --check`；`LEANOTE_GOLDEN=replay` 只读。
- [ ] 运行时矩阵记录 Go/Mongo/SMTP/OS、fixture、命令、退出码、discovered/executed/pass/fail/skip、首个脱敏错误和 commit/run 摘要；focused tests 不能升级为真实 Mongo/HTTP/SMTP/browser/failpoint 通过。
- [ ] 完成实现、测试、规格和任务元数据四层复核；发现规格漂移先回到 Phase 1 更新 PRD/design/implement/research/acceptance，再继续编码。

## 7. 回滚点

- [ ] 配置 seam、backup metadata、升级 checkpoint、feedback receipt/outbox、comment metadata/index 分开迁移和回滚；索引 preflight 或旧 event 解码失败时 fail closed。
- [ ] 回滚不得删除历史 feedback/receipt、回拨 USN、恢复已删除数据、取消已 handoff 邮件或暴露任何 secret；保留人工对账所需的最小脱敏身份。

## 当前修复批次记录（2026-09-25）

- 已落地并有 focused regression：旧 comment payload 兼容（含 `IdempotencyKey` 一致性校验）、Suggestion 多行与 C0 控制字符边界、mongodump/mongorestore argv 与 stdin 密码通道、保护备份/恢复目标排除/新目录 cleanup、三类安全配置写入口与启动/更新预检、broadcast receipt 版本 CAS、升级 USN 单调重试、Suggestion 稳定错误分类、群发空行过滤和 Download 显式失败状态。
- 本批 focused 命令：`go test ./app/db ./app/service ./app/controllers`、`go vet ./app/db ./app/service ./app/controllers`、`go build ./app/...`、`git diff --check`、`python ./.trellis/scripts/task.py validate 09-08-application-admin`；真实 Mongo、SMTP、mongodump/mongorestore、HTTP、浏览器和故障注入仍保持 `unrun`/`partial`。
- 尚未关闭的实现门禁包括：升级 checkpoint 租约续期/逐步骤拆分、备份 metadata 完整 manifest/owner CAS、feedback/broadcast/comment 的 Mongo 并发与 transport 证据，以及 `CredentialProviderRef` 的 interface seam 接入。未完成项不得勾选为通过。
