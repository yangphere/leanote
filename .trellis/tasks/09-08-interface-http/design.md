# 接口适配层：标准库 HTTP 与路由 — 技术设计

## Boundaries

`app/httpserver` 提供 request/params、route table、registry、middleware、session、templates、response 和 server；controller adapter 只把 HTTP 映射到 application service。

## Migration shape

`conf/routes` → 显式 route table → 受限 controller/action registry → typed context → application service → Result writer。catch-all 只查 registry，拒绝反射。

## Invariants

公开 URL/API、method、参数、状态码、JSON/JSONP、Session、i18n、gzip、静态资源和模板行为保持；匿名期 `_ID` 稳定但认证成功时必须轮换并失效旧 fallback 映射；Web cookie 可一次性失效，API token 保持。P-06 清理失败不得返回伪成功，P-07 注册 Cookie 提交失败不得设置认证 Cookie 或跳转受保护页面；API token 仅从 query/form 读取，任何未来 header 扩展的冲突规则需另立契约。`app/httpserver` 的 production-config seam 统一负责运行时来源、secret/Mongo/typed ContentRoots 校验、稳定错误码和 bind/dial/healthz 时序；ContentRoots 校验消费 content 定义的 paired data/quarantine same-filesystem、non-public、no-overlap contract，不复制 logical path resolver。其他任务只消费其契约。生产 secret/config fail closed。CSRF 与通用限流保持现状，不在本任务隐式新增。

## Rollback

先完成 harness/Golden 切换再删除 Revel；任何不完整迁移回滚该 HTTP 适配提交，不引入静默双栈。
