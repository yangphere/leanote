# 基础设施层：MongoDB 持久化边界 — PRD

## Goal

把 MongoDB 驱动、查询/更新、ObjectID、BSON、超时和错误映射收敛到 `app/db` 的单一基础设施边界，让应用服务不再感知具体 driver。

## Requirements

- 使用官方 `mongo-driver/v2`，支持 MongoDB 7.0–8.0，保持 collection、字段名、BSON 类型和排序分页语义。
- 明确表达 Find/Sort/Skip/Limit/One/All/Count/Insert/Update/UpdateAll/Upsert/Remove/Select 等现有用法。
- not-found、duplicate key、空集合、更新结果和 ObjectID 表现与领域契约一致。
- 负责领域中立 ObjectID/时间值与 Mongo BSON 的单向转换；领域包不得反向引入 driver。
- 每次 DB 操作使用显式有界 timeout；全链路 request context 传播另记 MOD-001。
- 业务 service 不直接持有 driver client/collection/query 类型。

## Acceptance criteria

- [ ] `app/db` contract tests 在 MongoDB 7/8 通过，且无数据迁移。
- [ ] 全量 service 查询可通过兼容边界，静态扫描无直接 driver collection 使用。
- [ ] 领域值类型与 Mongo BSON 转换集中在 `app/db`，`app/info` 依赖扫描不出现 Mongo driver。
- [ ] 用户所有权条件、排序、分页、upsert 和批量更新有回归。
- [ ] timeout、连接失败、not-found 和 duplicate key 错误可定位且不吞错。
- [ ] `go mod verify`、目标 Go 版本构建和 Golden/USN 通过。

## Out of scope

不修改 MongoDB Schema、不把 `context.Context` 贯穿 controller/service、不替换消息配置解析器。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
