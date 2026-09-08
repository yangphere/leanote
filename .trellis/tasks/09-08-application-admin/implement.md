# 应用层：管理与运维业务 — 执行计划

- [ ] 盘点 Config/Upgrade/Email service、admin controller、配置来源和日志调用。
- [ ] 收敛管理员权限、配置来源、升级/邮件错误和审计输出。
- [ ] 将 mongodump/mongorestore/PDF executable 配置收敛到 allowlist/绝对路径校验，使用 `exec.CommandContext` argv 和超时，删除 shell 拼接。
- [ ] 将 mongodump/mongorestore 凭据改为受控 stdin、credential helper 或 mode `0600` 临时凭据文件；PDF callback secret/token 使用同等受控传递方式，确保数据库凭据和 callback 凭据均不出现在 argv、环境快照、日志或错误输出，并在进程参数、环境快照、错误输出和清理失败场景加入回归。
- [ ] 补齐 admin 认证、用户/数据/设置/升级/邮件主要流程及失败测试。
- [ ] 补齐 shell 元字符、空格/引号、失败/超时及敏感日志回归；验证 ConfigService 对 production-config contract 的消费映射。
- [ ] 运行管理 service/adapter contract、Golden、敏感信息扫描和 `git diff --check`；production-config bind/dial/healthz、退出码 78 和管理端页面 smoke 由 interface/delivery 任务执行。
