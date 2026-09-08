# 基础设施层：MongoDB 持久化边界 — 执行计划

- [ ] 盘点 `app/db` 与 service 的 driver 类型、查询形态、配置键和错误处理。
- [ ] 完成 client/collection/query/ObjectID/BSON 转换单一边界，迁移剩余直接 driver 调用，并接入 domain-contracts 的中立值类型。
- [ ] 补齐查询、排序、分页、投影、批量更新、upsert、not-found、duplicate key 和 timeout 测试。
- [ ] 在 MongoDB 7/8 fixture 上运行 service、Golden、USN 和所有权回归。
- [ ] 扫描无 mgo/Revel 配置依赖、运行 `go mod verify`，记录 MOD-001。
