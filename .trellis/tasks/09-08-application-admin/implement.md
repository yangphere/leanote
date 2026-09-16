# 应用层：管理与运维业务 — 执行计划

- [ ] 盘点 Config/Upgrade/Email service、admin controller、配置来源和日志调用。
- [ ] 收敛管理员权限、配置来源、升级/邮件错误和审计输出。
- [ ] 接收 `application-notes` 的 Suggestion/feedback handoff：冻结 anonymous policy、`Addr`/`Suggestion` missing/blank/长度/HTML、重复 identity 和旧 wire response；先写持久化/outbox RED 测试。
- [ ] 将 feedback durable success 与 email transport 分离，改用可续跑 outbox；移除请求路径无结果 goroutine，验证 transport 失败不回滚已持久化 feedback。
- [ ] 将 mongodump/mongorestore 配置收敛到 allowlist/绝对路径校验，使用 `exec.CommandContext` argv 和超时，删除 shell 拼接；PDF executable 只生成并交接经过校验的 descriptor，不在 admin 启动 renderer。
- [ ] 将 mongodump/mongorestore 凭据改为受控 stdin、credential helper 或 mode `0600` 临时凭据文件；确保数据库凭据和 legacy PDF callback 凭据均不出现在 descriptor、argv、环境快照、日志或错误输出。PDF renderer 的参数、sandbox、清理失败和真实 artifact 回归由 `application-content` 负责。
- [ ] 补齐 admin 认证、用户/数据/设置/升级/邮件主要流程及失败测试。
- [ ] 补齐 shell 元字符、空格/引号、失败/超时及敏感日志回归；验证 ConfigService 对 production-config contract 的消费映射。
- [ ] 运行管理 service/adapter contract、Golden、敏感信息扫描和 `git diff --check`；production-config bind/dial/healthz、退出码 78 和管理端页面 smoke 由 interface/delivery 任务执行。
