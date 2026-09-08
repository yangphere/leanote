# 领域层：模型与跨层契约 — PRD

## Goal

把 `app/info`、现有 Golden/USN 和模型测试中的稳定业务语义整理成单一领域契约，为所有应用服务和适配层提供可执行边界。

## Requirements

- 冻结 API response envelope、字段名/类型、ObjectID 和时间表示。
- 冻结 BSON/JSON 标签、omitempty、空集合、not-found、duplicate key 和错误语义。
- 冻结 mutation → USN 递增 → sync 返回的配对关系及用户所有权查询条件。
- 冻结存量笔记未编辑零写入、编辑后允许的 HTML 规范化和 DOM 语义不变量。
- 测试从真实模型/Golden 输入读取，不创建第二套领域事实来源。
- 领域模型使用无框架的 ObjectID、时间和日志接口；MongoDB/Revel 的编码、驱动和日志适配必须留在基础设施或接口层。

## Acceptance criteria

- [ ] `app/info` 模型契约测试覆盖所有对外 API 和 Mongo 持久化字段。
- [ ] USN、所有权、冲突和删除边界有可重复测试与 Golden 对照。
- [ ] 笔记 HTML 只读/编辑保存语义有明确回归用例。
- [ ] `app/info` 及其直接依赖不依赖 Revel、HTTP writer、Mongo driver/collection 或前端代码；`go list -deps ./app/info` 的生产依赖扫描可复现通过。
- [ ] `go test ./app/info ./app/tests/...`（所需 fixture 可用时）和静态依赖检查通过。

## Out of scope

不迁移 controller/service、替换数据库驱动实现或改变公开 API/Schema；允许为去除领域包框架依赖抽取中立类型/接口，具体驱动转换由持久化任务实现。发现既有缺陷只登记，不在本任务修复。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
