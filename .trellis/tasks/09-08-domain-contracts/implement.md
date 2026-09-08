# 领域层：模型与跨层契约 — 执行计划

- [ ] 盘点 `app/info` 全部模型及其 API/BSON 使用方。
- [ ] 抽取领域使用的中立 ObjectID、时间和日志接口，移除 `app/info` 对携带 Mongo/Revel 的 `app/lea` 公共实现的编译依赖。
- [ ] 补齐模型、API envelope、ObjectID、USN、所有权和 HTML 语义测试。
- [ ] 为每条契约记录输入、输出、动态字段归一化和失败边界。
- [ ] 运行目标 Go 测试、Golden/USN 回归和依赖方向扫描；扫描必须确认 `go list -deps ./app/info` 不含 Revel 或 Mongo driver。
- [ ] 通过评审后再允许应用层任务激活。

验证：`go test ./app/info ./app/tests/...`、`go vet ./app/info`、`git diff --check`。
