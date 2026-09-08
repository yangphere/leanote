# 应用层：身份认证与会话 — 技术设计

## Boundaries

`AuthService`、`UserService`、`PwdService`、`SessionService`、`TokenService` 负责业务决策；HTTP adapter 负责 cookie、参数和 response。服务不依赖 controller 类型。

## Data flow

请求 → 参数/身份 adapter → auth/user service → db boundary → domain result → API/Web adapter。API token 仍存于既有 session collection；Web cookie 由 HTTP 层签名。

## Invariants

保留登录失败与注册开关、token 有效期、用户所有权、一次性重新登录和敏感信息脱敏。认证失败必须明确返回，不用匿名 fallback 掩盖错误。

## Rollback

按 Auth/User/Session/Token 子边界回滚；不回退领域契约和持久化 driver。
