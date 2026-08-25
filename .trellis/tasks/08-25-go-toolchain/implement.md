# Go 工具链与通用依赖现代化（A）— 执行计划

> 实现前读取本任务 PRD、设计、`CONTEXT.md`、ADR-0001 和父任务 `research/external-facts.md`。

## Global Constraints

- 最低 Go 版本为 1.26；CI 同时验证 1.26 与 1.27。
- 不升级 `github.com/revel/*` 或 `gopkg.in/mgo.v2`。
- 不允许通过更新 Golden、关闭 vet analyzer 或改变 BSON/JSON 字段来消除失败。

### Task 1：建立工具链失败快照

**Files:** `go.mod`、`go.sum`、`app/info/*.go`、`app/service/*.go`、`app/controllers/*.go`、`app/lea/*.go`

- [ ] 保存 `go env GOVERSION`、`go list -m -u -json all` 和 `go vet ./app/...` 输出到本任务检查记录。
- [ ] 在 MongoDB 8.0 fixture 环境回放 G 的 Golden 与 USN 测试，确认修改前基线为绿。
- [ ] 运行 `npm test`，确认前端基线仍为 10 个测试通过。

### Task 2：先为 struct tag 行为增加回归测试

**Files:**
- Create: `app/tests/model_serialization_test.go`
- Read/Modify: `app/info/Api.go`、`app/info/NoteInfo.go`、`app/info/UserInfo.go`

- [ ] 写表驱动测试，分别序列化代表性 API、Note、User 值，断言字段名、ObjectId 形态、零值字段保留/省略规则。
- [ ] 运行 `go test ./app/tests/ -run TestModelSerialization -v`，确认测试在修 tag 前能暴露无效 tag 与期望之间的差异或明确记录当前序列化结果。
- [ ] 将全部无效 tag 改为合法的 `bson:"Field"` / `json:"Field,omitempty"` 形式，字段名按 Golden 与现有数据库字段确定，不从 Go 字段名猜测。
- [ ] 重跑序列化测试和 Golden，要求输出契约不变。

### Task 3：升级 Go directive 与通用直接依赖

**Files:** `go.mod`、`go.sum`

- [ ] 把 `go` directive 改为 `1.26`，不添加要求 1.27 的 `toolchain` directive。
- [ ] 按候选表逐个升级非 Revel/MongoDB 直接依赖；每次只改一个直接依赖并运行 `go build ./app/...` 与相关测试。
- [ ] 对每个拟删除模块分别执行 `go mod why -m` 并用源码搜索验证无引用；只有两者都证明未使用才删除，命令记录使用实际模块路径。
- [ ] 运行 `go mod tidy` 与 `go mod verify`，检查没有意外引入新的框架或第二套日志/配置实现。

### Task 4：按类别清零 vet

**Files:** 设计 §3 表中的所有具体文件。

- [ ] 把 unkeyed literal 改为具名字段，并运行拥有该类型的包测试。
- [ ] 删除 unreachable code 和 self-assignment，只保留原本可达的控制流。
- [ ] 修正 `BaseController.NotFound` 的格式化调用，使返回文本与现有行为一致。
- [ ] 把 `app/cmd/harness/harness.go` 的 signal channel 改为容量 1，验证 SIGTERM 关停路径。
- [ ] 运行 `gofmt` 后执行 `go vet ./app/...`，要求零输出、exit 0。

### Task 5：双版本与契约验证

**Files:** `.github/workflows/*.yml`（由 G 创建的 Go job，仅扩展 Go 版本矩阵）

- [ ] 在 Go 1.26 与 1.27 分别运行 `go build ./app/...`、`go vet ./app/...`、`go test ./app/tests/...`。
- [ ] 回放全部 Golden/USN 测试；任何差异必须定位并修复，禁止刷新基线。
- [ ] 运行 `npm test` 和 `git diff --check`。
- [ ] 复核 diff 中不存在 Revel/MongoDB 专项升级、Schema 改动或无关格式化。

## Rollback Points

- 依赖升级按单模块回滚；Go directive 可单独回滚。
- struct tag 修复只有在序列化与 Golden 同时通过时保留。
- 任一行为差异无法解释时停止，不把后续 B/C-a 的工作提前并入。
