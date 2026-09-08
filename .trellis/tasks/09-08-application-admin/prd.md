# 应用层：管理与运维业务 — PRD

## Goal

收敛管理端、配置、升级、邮件和运维审计业务，明确管理员权限、失败边界和敏感信息处理。

## Scope

`ConfigService.go`、`UpgradeService.go`、`EmailService.go`、admin controllers、管理模板和相关日志/配置适配。

## Requirements

- Admin 权限在应用服务和 HTTP 前置钩子均有明确边界，未授权请求不得执行副作用。
- 生产启动配置遵循 `interface-http` 的唯一 production-config seam，只接受显式来源，空/default secret 和不一致的 Mongo 配置必须 fail closed；本任务不复制另一套解析规则。
- `mongodump`、`mongorestore` 和 PDF 可执行文件只能来自显式 allowlist/经过校验的绝对路径，执行必须使用 argv 和有界超时，禁止把配置拼入 shell；MongoDB 用户名/密码及 PDF callback secret/token 不得进入 argv、命令日志或错误输出。
- 升级、邮件和数据维护操作记录可定位错误，不吞异常、不打印凭据。
- 管理页面与 API 的状态码、错误文本和重定向保持兼容。
- 管理业务设置通过 ConfigService 单一 seam；生产启动配置规则统一由 `interface-http` production-config seam 提供，后续消息解析器替换另记 MOD-003。

## Acceptance criteria

- [ ] Admin service/adapter 的登录、权限、用户/数据/设置/升级/邮件主要流程有回归；真实页面和启动 smoke 由 interface/delivery 任务验收。
- [ ] ConfigService 只消费 `interface-http` production-config contract，不重复解析来源、secret 或 Mongo 配置；consumer mapping fixture 通过。
- [ ] 邮件/升级失败可观测且不产生虚假的成功响应。
- [ ] 敏感值不出现在日志、summary、制品或测试 fixture。
- [ ] 管理服务和 adapter 的可执行文件 allowlist、argv 参数化、超时、错误和敏感日志回归通过；数据库凭据及 PDF callback secret/token 不出现在 argv、环境快照、日志或错误输出；管理页面 smoke 由 interface/delivery 任务验收。

## Out of scope

不增加管理功能、不存储生产凭据、不替换消息配置解析器、不自动部署生产。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
