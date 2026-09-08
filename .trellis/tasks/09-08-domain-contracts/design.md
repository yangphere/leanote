# 领域层：模型与跨层契约 — 技术设计

## Boundaries

以 `app/info`、`CONTEXT.md`、现有 Golden/USN 和模型契约测试为输入，输出可供 service、db、HTTP 和前端消费的稳定数据/行为契约。该层不导入 Revel、HTTP 或 Mongo driver。

## Dependency seam

- `app/info` 只依赖中立的领域值类型和接口；ObjectID/BSON、日志和配置的具体实现由 `app/db`、HTTP 或运行时适配层提供。
- 领域契约任务必须先抽取或替换 `app/lea` 中携带 Mongo/Revel 的公共类型，再运行依赖扫描；仅增加测试不能满足无框架依赖验收。
- 该 seam 不改变公开 JSON/BSON 字段或 ObjectID 的外部表示，转换和兼容回归由持久化任务共同验收。

## Contract groups

- API：envelope、字段类型、状态码、ObjectID/time 表示。
- Persistence：BSON 名称、omitempty、空值、排序和更新结果。
- Business：USN mutation/sync、所有权、冲突、删除、not-found。
- Editor：未编辑零写入与编辑后 HTML 语义等价。

## Rollback

只回滚领域契约测试和文档；发现旧行为差异时登记基线缺陷，不修改生产逻辑。
