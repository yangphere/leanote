# 应用层：管理与运维业务 — 技术设计

## Boundaries

Config/Upgrade/Email service 负责管理业务决策与副作用；admin HTTP adapter 负责权限前置、绑定和渲染；生产启动配置只消费 `interface-http` 的 production-config boundary，管理员可变设置仍由 ConfigService 统一读写。

## Invariants

管理员权限不可绕过；空/default secret、Mongo 配置冲突和邮件/升级失败必须 fail closed；凭据只来自运行时环境并且日志脱敏。
- 外部可配置的备份/PDF executable 通过 allowlist 和 argv seam 执行，禁止 `/bin/sh -c`；数据库凭据通过受控 stdin、工具支持的 credential helper 或 mode `0600` 的短生命周期凭据文件传递。数据库凭据以及 PDF callback secret/token 均不得进入 argv、环境快照、日志或错误输出；callback 凭据的传递方式和清理失败也必须由回归测试覆盖。

## Rollback

按配置、升级、邮件和 admin controller 子边界回滚；不改变管理数据 Schema 或自动部署策略。
