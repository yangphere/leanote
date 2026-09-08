# 应用层：身份认证与会话 — PRD

## Goal

收敛用户身份、密码、Session、Token、登录注册和认证授权业务，令 Web 与 `/api/*` 共享同一业务规则。

## Scope

`app/service/AuthService.go`、`UserService.go`、`PwdService.go`、`SessionService.go`、`TokenService.go`，以及 Auth/User controller、API Auth/User 和相关 session/auth interceptor。

## Requirements

- 保留密码校验、注册开关、登录失败、Token 生命周期和 API token 数据语义。
- Web session 可在 HTTP 迁移后一次性重新登录；API token 必须跨版本保持有效。
- 认证前置和用户所有权不得只由 controller 判断；服务边界必须返回稳定错误/状态。
- 清理跨层对 Revel session/config 的直接依赖，改由接口适配层注入。

## Acceptance criteria

- [ ] 登录、注册、登出、改密、改名、头像和 Token 失效有服务级回归。
- [ ] API invalid/none token、Web 匿名/已登录和权限失败的 service/adapter contract fixture 与 Golden 一致；真实 HTTP 路由回归由 interface/delivery 任务验收。
- [ ] 密码不出现在日志、artifact 或错误响应；Session cookie 安全属性由 HTTP 层统一生成。
- [ ] 服务不直接写 HTTP response，也不复制其他应用领域的认证规则。
- [ ] 领域契约任务完成后，本任务的 Go/Golden/权限验证通过；不把 interface-http 的 live smoke 作为本任务 ready 条件。

## Out of scope

不改变用户模型、密码算法、公开 API 字段或新增 OAuth/SSO；完整 HTTP 路由切换由 `interface-http` 负责。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
