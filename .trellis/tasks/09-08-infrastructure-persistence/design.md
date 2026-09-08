# 基础设施层：MongoDB 持久化边界 — 技术设计

## Boundaries

`app/db` 是唯一 MongoDB driver 边界，内部拆分 client、collection、query、ObjectID/BSON 转换和错误映射；service 只依赖仓库语义，不暴露 driver 类型。领域层的中立 ObjectID/时间值在此处转换，禁止领域包反向依赖 driver。

## Data flow

应用 service → typed repository/query wrapper → MongoDB 7/8；wrapper 在每次操作内创建有界 timeout 并立即 cancel，返回领域可识别结果。

## Invariants

collection/字段/BSON 不变；所有权条件由查询构造器强制；排序/分页/更新/upsert/not-found/duplicate key 与旧行为一致；无数据迁移。

## Rollback

以 driver contract、MongoDB 7/8 Golden/USN 和 `go mod verify` 为边界回滚；MOD-001 单独规划。
