# 呈现层：前端构建与编辑器运行时 — 技术设计

## Boundaries

manifest 与 Node scripts 负责源码到生成资源；模板负责服务端呈现；jQuery/Bootstrap/TinyMCE 与第一方插件是运行时；业务 service/API 不在前端复制。

## Data flow

源码/语言/模板 → manifest staging → deterministic generated assets → HTTP static/template adapter → browser business flows。失败时原子回滚生成物。

## Invariants

Node 24、版本唯一、无 CDN/生产 migrate、生成资源零漂移；编辑器只读/编辑保存状态与 HTML 语义保持；资源错误可观测。`OperationId` 代表一次用户意图：unknown-result 重试复用，新意图换代；`ExpectedUsn` 来自当前已确认 revision，不从服务端错误响应猜测。

## Rollback

按 build chain、jQuery、Bootstrap、TinyMCE 和插件输出边界回滚；不手工编辑生成文件。
