# 接口适配层：标准库 HTTP 与路由 — 技术设计

## Boundaries

`app/httpserver` 提供 request/params、route table、registry、middleware、session、templates、response 和 server；controller adapter 只把 HTTP 映射到 application service。

## Migration shape

`conf/routes` → 显式 route table → 受限 controller/action registry → typed context → application service → Result writer。catch-all 只查 registry，拒绝反射。

## Invariants

公开 URL/API、method、参数、状态码、JSON/JSONP、Session、i18n、gzip、静态资源和模板行为保持；Web cookie 可一次性失效，API token 保持。`app/httpserver` 的 production-config seam 统一负责运行时来源、secret/Mongo 校验、稳定错误码和 bind/dial/healthz 时序；其他任务只消费其契约。生产 secret/config fail closed。

## Rollback

先完成 harness/Golden 切换再删除 Revel；任何不完整迁移回滚该 HTTP 适配提交，不引入静默双栈。
