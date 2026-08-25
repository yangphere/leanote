# Go 工具链与通用依赖现代化（A）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

把项目最低 Go 版本提升到仍受官方支持的 1.26，并在 Go 1.26 与 1.27 上建立一致的构建、静态检查和测试门禁；同时升级不属于 Revel、MongoDB 专项任务的直接依赖，消除现代 `go vet` 已暴露的问题而不改变业务行为。

## Dependencies

- 必须在 `08-25-regression-baseline` 完成后开始；所有行为修正都要有 Golden 与 USN 基线保护。
- 本任务不升级 Revel 或 MongoDB 驱动，这两类依赖分别由 C-a 与 B 负责。

## Confirmed Facts

- `go.mod` 当前声明 `go 1.15`。
- 当前代码在本机 Go 1.27.0 下执行 `go build ./app/...` 成功。
- `go vet ./app/...` 当前失败，包含无效 struct tag、unreachable code、unkeyed literal、self-assignment、格式化调用错误和 signal channel 用法错误。
- 版本政策与本机证据统一记录在父任务 `research/external-facts.md`。

## Requirements

### R-A1 Go 版本政策

- `go.mod` 声明 `go 1.26`，不得写成浮动版本。
- CI 分别使用 Go 1.26 与 1.27 执行相同的构建、vet 和测试命令。
- 生产代码不得依赖 Go 1.27 新语法或仅 1.27 提供的标准库 API。

### R-A2 依赖升级边界

- 逐个升级非 Revel、非 MongoDB 的直接依赖，并在每次升级后运行针对性验证。
- 仅在 `go mod why -m` 和源码搜索都证明未使用时删除依赖。
- 不用 `go get -u ./...` 做不可归因的全量升级；传递依赖由已评审的直接依赖与 `go mod tidy` 收敛。

### R-A3 Vet 清零

- 修正 `go vet ./app/...` 的全部当前错误，不通过禁用 analyzer、添加宽泛忽略或降低检查范围获得绿灯。
- struct tag 修复必须通过 BSON/JSON 回归证明字段名和 `omitempty` 行为不变。
- unreachable code、self-assignment 与格式化调用修复不得顺带重构相邻业务逻辑。

### R-A4 行为与数据兼容

- `/api/*`、USN、用户所有权查询、MongoDB collection/字段名与模板输出保持不变。
- 依赖升级出现行为差异时，必须定位到具体依赖；不能通过更新 Golden 吸收无法解释的变化。

## Acceptance Criteria

- [ ] `go.mod` 为 `go 1.26`，且不包含本任务越权升级的 Revel/MongoDB 版本。
- [ ] Go 1.26 与 1.27 下 `go build ./app/...` 均通过。
- [ ] Go 1.26 与 1.27 下 `go vet ./app/...` 零错误，仓库内没有新增 `//nolint` 或 analyzer 关闭配置。
- [ ] MongoDB 8.0 fixture 环境中 `go test ./app/tests/...` 与 Golden/USN 回归通过。
- [ ] `npm test` 继续 10/10 通过。
- [ ] `go mod tidy` 后 `go.mod`、`go.sum` 可重复，`go mod verify` 通过。
- [ ] BSON/JSON 代表性模型测试覆盖 `app/info/Api.go`、`NoteInfo.go`、`UserInfo.go` 的字段名与省略语义。
- [ ] 父任务 Golden 回放无未解释差异。

## Out of Scope

- Revel 1.0→1.1 或迁出 Revel。
- `mgo.v2`→官方 MongoDB 驱动。
- 业务重构、新功能、Schema 或 API 契约变化。
- 为了“更现代”引入新的框架、日志库或配置系统。
