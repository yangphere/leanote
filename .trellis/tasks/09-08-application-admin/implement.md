# 应用层：管理与运维业务 — 执行计划

- [ ] 盘点 Config/Upgrade/Email service、admin controller、配置来源和日志调用。
- [ ] 收敛管理员权限、配置来源、升级/邮件错误和审计输出。
- [ ] 接收 `application-notes` 的 Suggestion/feedback handoff：冻结 anonymous policy、`Addr`/`Suggestion` missing/blank/长度/HTML、重复 identity 和旧 wire response；先写持久化/outbox RED 测试。
- [ ] 将 feedback durable success 与 email transport 分离，改用可续跑 outbox；移除请求路径无结果 goroutine，验证 transport 失败不回滚已持久化 feedback。
- [ ] 接收 `application-publishing` 的评论通知合同：先与其确认稳定 comment ID/recipient、共同提交可领取闸门、comment 事件结构、安全邮件模板与日志最小化，再扩充 outbox kind/worker/投递；验证失败 retry/dead、不重复入队、SMTP 失败不撤销已确认评论，且现有账号事件与本任务规划的 feedback 投递不退化。
- [ ] 按 publishing `design.md` 共享状态表实现 comment/recipient 稳定身份、outbox 元数据中的取消/交接时间/原因及 CAS 版本；先写 `unconfirmed` 不可领、pending/retry/claimed/handoff_pending 取消、取消赢时 worker 不调用 SMTP、交接闸门赢时不得报告已取消、闸门后进程崩溃/SMTP 未知进入 `handoff_unknown` 且不自动重发的负例。按 `_id/status/version/lease/CancelRequested` 原子迁移，补 `(Kind, CommentId, Status)` 定位与去重索引；测试状态读回、过期 lease、确定拒绝 retry/dead、人工对账及旧账号/feedback 事件兼容。完成双方 AC-PB4-NOTIFY/AC-PB4-DELETE 集成证据前不得标通过；真实 Mongo/SMTP 故障注入交 delivery。
- [ ] 将 mongodump/mongorestore 配置收敛到 allowlist/绝对路径校验，使用 `exec.CommandContext` argv 和超时，删除 shell 拼接；PDF executable 只生成并交接经过校验的 descriptor，不在 admin 启动 renderer。
- [ ] 将 mongodump/mongorestore 凭据改为受控 stdin、credential helper 或 mode `0600` 临时凭据文件；确保数据库凭据和 legacy PDF callback 凭据均不出现在 descriptor、argv、环境快照、日志或错误输出。PDF renderer 的参数、sandbox、清理失败和真实 artifact 回归由 `application-content` 负责。
- [ ] 补齐 admin 认证、用户/数据/设置/升级/邮件主要流程及失败测试。
- [ ] 补齐 shell 元字符、空格/引号、失败/超时及敏感日志回归；验证 ConfigService 对 production-config contract 的消费映射。
- [ ] 运行管理 service/adapter contract、Golden、敏感信息扫描和 `git diff --check`；production-config bind/dial/healthz、退出码 78 和管理端页面 smoke 由 interface/delivery 任务执行。
