# 应用层：管理与运维业务 — 技术设计

## Boundaries

Config/Upgrade/Email service 负责管理业务决策与副作用；admin HTTP adapter 负责权限前置、绑定和渲染；生产启动配置只消费 `interface-http` 的 production-config boundary，管理员可变设置仍由 ConfigService 统一读写。

评论通知 outbox 由 admin 的邮件/投递边界实现 comment 事件编码、投递及取消状态；publishing 持有评论 ID、合法收件人、可确认的评论/计数/意图提交及删除权限。状态与迁移按 publishing `design.md`“评论通知取消与 transport 交接”：`unconfirmed` 不可领，`pending/retry → claimed → handoff_pending → handed_off → sent`；闸门前可 CAS `cancelled`，确定失败转 `retry/dead`，交接结果不明转 `handoff_unknown` 并禁止自动重发。稳定 `(CommentId, RecipientId)` 去重、`(Kind, CommentId, Status)` 定位、`_id/status/version/lease/CancelRequested` CAS 与版本递增必须跨 worker 一致。状态元数据不在 payload；`TransportHandedOffAt` 为调用 SMTP 前的保守不可撤回闸门而非 SMTP 接受证明，CAS 失败的 worker 不得使用已领取的正文继续发送。删除与交接的先后按持久 CAS 裁决，未知时读回/人工对账，不称 exactly-once。四项合同已由用户于 2026-09-24 确认，现有五态 outbox/`DeliverOutbox` 尚未实现评论能力；历史账号/feedback 事件不得被新 comment 状态机误处理。

## Invariants

管理员权限不可绕过；空/default secret、Mongo 配置冲突和邮件/升级失败必须 fail closed；凭据只来自运行时环境并且日志脱敏。
- 外部可配置的备份 executable 通过 allowlist 和 argv seam 执行，禁止 `/bin/sh -c`；PDF executable 在本任务只经过 allowlist/绝对路径/regular-file 校验并输出 descriptor，实际进程、argv、sandbox、timeout、输入输出和 cleanup 由 `application-content` 的 renderer 唯一执行。数据库凭据通过受控 stdin、工具支持的 credential helper 或 mode `0600` 的短生命周期凭据文件传递。数据库凭据以及 legacy PDF callback secret/token 均不得进入 descriptor、argv、环境快照、日志或错误输出；PDF 传递和清理失败由 content contract 覆盖。

## Rollback

按配置、升级、邮件和 admin controller 子边界回滚；评论事件/取消状态如需扩充存储合同，先核对历史事件兼容、索引及回滚，不得静默损坏现有账号 outbox 或本任务规划中的 feedback outbox；不自动部署生产。
