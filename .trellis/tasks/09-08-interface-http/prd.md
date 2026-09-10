# 接口适配层：标准库 HTTP 与路由 — PRD

## Goal

完成从 Revel 到标准库 `net/http`/`ServeMux` 的全量接口迁移，保留服务端渲染、公开 URL/API、Session、鉴权、模板和静态资源行为。

## Requirements

- 根据 `conf/routes` 建立完整显式路由和受限 catch-all registry，禁止任意反射调用。
- 迁移全部主站、API、admin、member controller actions、参数绑定、结果转换和 BEFORE 钩子。
- Session、`ViewArgs`、i18n、模板函数、gzip、恢复、日志和生产 secret/cookie 安全默认值统一由 HTTP 层提供。
- HTTP 层的 production-config seam 是生产启动配置的唯一事实来源；它公开稳定的错误码、来源校验和 bind/dial/healthz 时序供 application-admin 与 delivery 消费。
- 将 Golden/USN harness、开发启动、打包入口迁移到 `cmd/leanote`。
- 完成后清除 module/import/runtime/app/cmd/文档中的 Revel 生产依赖，不保留双运行时 fallback。
- 消费 `application-identity` 与 `infrastructure-persistence` 已确认的 principal、SessionWriter、`Session["_ID"]`、API fallback、token TTL 和错误 seam，不在 HTTP 层复制认证或持久化规则。
- 对 identity action 使用固定方法矩阵；未列方法返回 405。显式 `userId` 只能与 principal 一致；显式 invalid/expired token 不得回退到 Web session。
- 完成 `ApiUser` first-party registry 注册、`_token/_userId` Cookie 回写顺序和 Set/Delete/Commit/Encode 失败传播，并用真实 HTTP replay 验证。

## Acceptance criteria

- [ ] `conf/routes` 全部公开 action 与静态前缀均能到达正确 registry handler。
- [ ] 未注册 controller/action 返回明确 404/405，任意导出方法不能通过 URL 调用。
- [ ] Web/API 参数、Session、JSON/JSONP/text/template/file/redirect 响应和错误状态与 Golden 一致。
- [ ] API token 保持有效，Web 旧 cookie 按约匿名并可重新登录；cookie 安全属性测试通过。
- [ ] identity action matrix 的方法、405、principal `userId`、`Session["_ID"]` fallback、显式 invalid token fail-closed 和 SessionWriter 写失败均有 contract/replay 证据。
- [ ] `ApiUser` first-party actions 已注册并可达；API Auth/User envelope、Content-Type、字段顺序和 Golden 保持兼容。
- [ ] production-config 在 bind/dial/healthz 前完成来源、secret/Mongo 校验，失败返回稳定错误码并以退出码 78 fail closed；有效配置的 healthz 语义与 delivery contract 一致。
- [ ] harness、`sh/run.sh`、`sh/package.sh`、CI 和文档不再依赖 Revel；`rg 'github.com/revel|revel\.' app cmd go.mod sh conf` 对生产范围零命中。

## Out of scope

不改变 URL/API/Schema、不引入第二个 Web 框架、不重写业务服务、不在本任务加入新产品功能。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
