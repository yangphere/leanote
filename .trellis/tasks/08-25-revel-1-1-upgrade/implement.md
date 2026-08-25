# Revel 1.1 基线升级（C-a）— 执行计划

## Global Constraints

- 仅升级 Revel runtime/CLI 及其必要关联依赖。
- URL、配置、Cookie、模板、API/USN 契约保持不变。
- 不写 C-b 的 `net/http` 代码，不删除 `app/cmd`。

### Task 1：冻结三条路径的基线

**Files:** `go.mod`、`go.sum`、`app/cmd/`、`conf/routes`、`conf/app.conf`、`sh/run.sh`、`sh/package.sh`

- [ ] 在当前 v1.0 上生成测试二进制并回放 G 的 Golden。
- [ ] 启动 `revel run -a .`，验证 `/`、`/login`、`/note`、`/blog`、`/demo`。
- [ ] 运行现有 `sh/package.sh`，记录 tarball 文件清单与启动命令。

### Task 2：升级 runtime 与 CLI

**Files:** `go.mod`、`go.sum`

- [ ] 把 `github.com/revel/revel` 改为 v1.1.0，把 `github.com/revel/cmd` 改为 v1.1.2。
- [ ] 运行 `go mod tidy` 与 `go mod verify`，检查关联 Revel 模块只有一个版本集合。
- [ ] 运行 `go build ./app/...`；只对明确编译错误做最小适配。

### Task 3：验证生成、开发与打包

**Files:** `app/cmd/**`、`app/routes/routes.go`（生成、gitignored）、`app/tmp/main.go`（生成、gitignored）

- [ ] 运行 `app/cmd` 生成流程，构建 `app/tmp` 二进制并通过 harness 启动。
- [ ] 使用 v1.1.2 CLI 启动开发服务器，完成页面与 API smoke。
- [ ] 使用 v1.1.2 CLI 生成生产 tarball，解包到临时目录并启动验证。
- [ ] 向测试进程发送 SIGTERM，断言进程在超时内退出且端口释放。

### Task 4：完整回归与范围复核

- [ ] 运行 `go build ./app/...`、`go vet ./app/...`、`go test ./app/tests/...`、`npm test`。
- [ ] 回放全部 Golden 与 USN 成对用例，要求零差异。
- [ ] 对比 `conf/routes`、`conf/app.conf`、`app/init.go` 和 controller `init.go`，确认无语义改动。
- [ ] `git diff --check` 通过，diff 不包含 C-b 架构代码。

## Rollback Point

回滚 `go.mod`、`go.sum` 与必要编译适配即可恢复 v1.0；生成文件不入库，不参与回滚。
