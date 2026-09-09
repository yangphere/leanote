# 应用层：笔记工作区与同步 — 执行计划

- [ ] 盘点 Note/Notebook/Tag/Trash/History/Suggestion service、controller、API 和 Golden/USN 入口。
- [ ] 抽取单一所有权与 USN mutation/sync 规则，删除重复 controller 逻辑；按 D-01 修复 notebook/tag delete 的新 USN tombstone/sync，并保留旧差异对照断言。
- [ ] 补齐 CRUD、分页、冲突、删除/恢复、空集合和跨用户拒绝测试。
- [ ] 补齐未编辑零写入、编辑保存、失败/partial write 和 revision 回归；按 D-06 验证同事务提交、失败不消耗 USN，以及无事务时显式补偿/idempotency 与 `partial_write`。
- [ ] 在 MongoDB 7/8 兼容边界上运行 Golden/USN/权限验证。

验证：`go test ./app/service ./app/controllers/api ./app/tests/harness`（需 Mongo）、`git diff --check`。
