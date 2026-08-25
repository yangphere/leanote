# ADR-0002：在兼容边界后替换 mgo 与 Revel

- **状态**：Accepted
- **日期**：2026-08-25

## Context

`mgo.v2` 无法对受支持的 MongoDB 执行 CRUD，Revel 上游也已停滞。与此同时，数据库类型与查询遍布应用，Revel controller、session、配置、渲染与动态路由构成现有行为的一部分。

## Decision

- MongoDB 支持范围为 7.0–8.0，CI 与本地基线固定 8.0。
- 使用 `go.mongodb.org/mongo-driver/v2`，在 `app/db` 内建立兼容边界，保留现有 collection 全局名称和常用查询形状；collection、字段名、BSON 标签和 ObjectId JSON 契约不变。
- 本轮不把 `context.Context` 贯穿所有 controller/service 方法。兼容层为每次操作设置显式、可配置的超时；全链路上下文传播记录为 `MOD-001`。
- 先把 Revel 升至最后版本 1.1，再迁到 Go 标准库 `net/http` 与 `ServeMux`。
- 显式路由直接注册；历史 catch-all 只允许从静态 controller/action 注册表分派，禁止任意反射调用。
- C-b 上线允许 Web 用户重新登录一次，不复刻 Revel Cookie 编码；API token 和 MongoDB session 语义不变。
- 生产模式拒绝空密钥和仓库公开默认密钥；新 Cookie 默认 `HttpOnly=true`、`SameSite=Lax`，`Secure` 继续由配置控制。

## Consequences

- 数据和 HTTP 迁移分别拥有明确收敛点，减少业务层改动。
- 现有 Web Cookie 在 C-b 部署后失效一次，这是明确接受的兼容代价。
- 请求取消暂时不能自然传播到数据库；有界超时防止无限阻塞，完整改造由后续独立任务承担。
- 所有权查询和外部 API 契约必须由 Golden、USN 与双用户权限用例持续验证。

