# Go 工具链与通用依赖现代化（A）- 执行计划

> 实现前读取本任务 PRD/设计、`CONTEXT.md`、`CLAUDE.md`、ADR-0001、父任务 `research/external-facts.md` 和已归档 G 的 PRD/设计/验收证据。

## Global Constraints

- G-AC8 已由 PR #3 的真实 `pull_request` workflow 运行 32871393901 完成取证并回填 G 归档，
  原硬阻塞已解除。
- 最终规格已通过复核；用户于 2026-08-26 明确要求同时解决 G-AC8 与 A 启动问题，完成本轮
  文档验证后可运行 `task.py start`。
- MongoDB 固定 5.0；不升级 Revel/Mongo 模块，不刷新 Golden，不关闭 vet analyzer。
- 每个依赖与 vet 类别独立验证、独立回滚；任何失败保留非零退出和原始诊断。

## Phase 0：启动前完整性门禁

- [x] 核对 `08-25-regression-baseline` 的实现提交（`2dc85af`）、`.github/workflows/regression-baseline.yml`、`app/tests/golden/`、`app/tests/harness/`；均已存在。
- [x] 处置 G-AC8：PR #3 的 `pull_request` 运行 32871393901 中 `go-replay` 与 `node-tests` 均成功，证据已回填 G 归档。
- [ ] 用 MongoDB 5.0 fixture 连续两次 replay G 的 Golden/USN，确认目标测试数量非零、结果一致、Golden 文件 hash 不变。
- [ ] 运行 `go clean -cache` 后再记录 `go vet ./app/...` 完整快照；缓存态输出不可作为基线（实测可少报至 31 条）。同时记录 `git status`、`go env GOVERSION GOTOOLCHAIN GOFLAGS`、`go list -m -u` 和 `go mod why -m`；vet 差异先回到规格审查。
- [ ] 核对官方稳定补丁仍为 1.26.7/1.27.0；如版本政策或目标依赖发生实质漂移，更新规划并重新审批。
- [ ] 确认 PRD/design/implement 不含未决产品或范围问题；`robfig/config` 按已批准的 `MOD-003` 延期决策执行。

## Phase 1：先建立 DB-independent 契约测试

**Create/Modify:** `app/info/*_test.go`、必要的 `app/info/testdata/`、`app/lea/*_test.go`、`app/lea/i18n/*_test.go`、`app/cmd/**/*_test.go`

- [ ] 在旧 tag 上建立 18 文件/205 字段的 BSON tag 清单与 BSON/JSON 行为快照，覆盖所有 `omitempty`、ObjectId、时间、nil/空集合和嵌套结构。
- [ ] 证明模型测试不连接 Mongo，且 `go test ./app/info/... -run <target> -count=1 -v` 实际发现目标用例。
- [ ] 为 `SubStringHTML`、bcrypt、i18n 解析、CLI 参数和 `go/packages` 源码生成补聚焦回归。
- [ ] 运行新测试并记录修改前结果；测试不得依赖待修改实现生成期望值。

## Phase 2：等价修复 struct tag

**Modify:** 设计第 4 节清单中的 18 个 `app/info/*.go`

- [ ] 逐字段把 legacy raw BSON tag 原值移入 `bson:"..."`；不自动新增/改变 JSON tag。
- [ ] 运行 `gofmt`、模型 BSON/JSON 契约、G Golden/USN；205 条 tag 警告归零且序列化/HTTP 输出不变。
- [ ] 复核所有已有合法 `json:"-"`、ObjectId、字段名和 `omitempty` 行为未被改写。

## Phase 3：建立 Go 1.26 模块基线

**Modify:** `go.mod`、`go.sum`

> 顺序约束：goquery/x/crypto/x/tools 目标版本要求主模块 `go >= 1.25`；而 G 夹具的每次 replay 都要用 `LEANOTE_TEST_GO` 指定的 Go 1.20.14 运行 legacy 源码生成（`buildServerBinary` → `app/cmd` parser2），README 已记录 Go 1.26/1.27 在旧 x/tools 下生成 panic、Go 1.20.14 又读不了 `go 1.26` 主模块。因此第 1 步必须是原子 bootstrap 批次，顺序不可调换。

- [ ] **Bootstrap（原子批次，整体回滚单元）**：同批修改 go directive 为 `1.26`（确认无 `toolchain` directive）并把 `golang.org/x/tools` 升至 v0.49.0；立即用 Go 1.26.7 与 1.27.0 各执行一次真实生成入口（`app/cmd` build 子命令，即 harness `buildServerBinary` 的等效流程）确认不再 panic、生成 routes/tmp 后二进制可构建，然后运行 G replay。
- [ ] 记录允许/禁止模块版本快照；Revel/Mongo 模块版本在整个阶段必须不变。
- [ ] 对允许模块按以下顺序逐个升级（bootstrap 之后），每项执行调用方测试、`go build ./app/...`、G replay 和模块 diff 审查：
  1. `github.com/PuerkitoBio/goquery` v1.6.1 -> v1.12.0；
  2. `github.com/jessevdk/go-flags` v1.4.0 -> v1.6.1；
  3. `golang.org/x/crypto` -> v0.55.0；
  4. `github.com/robfig/config` 保持当前版本，运行消息解析契约并核对 `MOD-003` 链接；
  5. `github.com/agtorre/gocolorize` 保持 v1.0.0 并记录无更新。
- [ ] 对任何拟删除模块同时执行 `go mod why -m` 与源码/生成入口搜索；有实际路径则保留。
- [ ] 运行 `go mod tidy` 两次并确认第二次零 diff，随后 `go mod verify`；审查所有传递变化的来源、GoVersion 和许可证。

## Phase 4：按类别清零 vet

**Modify:** `app/info`、`app/cmd`、`app/controllers`、`app/lea`、`app/service` 中实现前快照点名的文件

- [ ] 将 21 个 unkeyed literal 改为具名字段；显式核对项目模型、10 个 `bson.RegEx` 的 Pattern/Options 与 `revel.PlaintextErrorResult` 的错误值。
- [ ] 删除 6 个不可达语句（`app/lea/captcha/Captcha.go:388`、`app/cmd/harness/build.go:100/240`、`app/cmd/build.go:96`、`app/lea/File.go:147`、`app/controllers/AuthController.go:100`）和 3 个 self-assignment（`app/service/NoteService.go:972`、`app/controllers/member/MemberBlogController.go:383/483`），只保留原可达控制流与输出。
- [ ] 先锁定 `E404` 当前状态码/正文（包含 Revel 对 `("", nil)` 的实际格式化结果），再改成 vet-clean 表达并要求输出不变。
- [ ] 将 signal channel 容量改为 1，保持现有 signal 集合，验证 interrupt 后的 kill/退出路径；不新增 SIGTERM 行为。
- [ ] 每完成一个类别运行拥有该行为的聚焦测试与 G replay；最终两个 Go 版本的 `go vet ./app/...` 均须零输出、exit 0。

## Phase 5：CI 与真实入口

**Modify:** G 已创建的 `.github/workflows/*.yml`、`.travis.yml`、`app/tests/README.md`、`app/tests/harness/server.go` 及其断言测试、`.trellis/spec/backend/quality-guidelines.md`

- [ ] 把 G 的 Go job 扩展为固定 1.26.7/1.27.0 矩阵；两个版本运行相同 build、vet、DB-independent tests 和 MongoDB 5.0 replay。
- [ ] 迁移手工 `record-export-pdf` job：`setup-go` 从 1.20.14 改为固定 1.26.7，确认 wkhtmltopdf 安装、Mongo fixture 与 `LEANOTE_GOLDEN=record` 流程在新工具链下成立。record 流程验证必须在隔离 checkout 或临时副本中执行（workflow_dispatch 实跑优先），禁止在本工作区直接运行 record；验证前记录 Golden hash，验证后工作区 Golden 文件必须零 diff，record artifact 不回填仓库。
- [ ] 迁移本地生成契约：`app/tests/README.md` 删除"必须安装 Go 1.20.14"的 panic 规避指南；`app/tests/harness/server.go:154` 的错误提示与 `server_test.go` 断言同步——`LEANOTE_TEST_GO` 降级为可选覆盖（缺省 PATH 中的 go）。缺省工具链必须 fail-closed：启动前校验版本 ≥1.26.7（低于即显式失败并给出安装指引），生成子进程设置 `GOTOOLCHAIN=local` 禁止自动下载；旧版本拒绝用桩测试锁定，1.26.7/1.27.0 通过用真实工具链锁定。
- [ ] 更新 `.trellis/spec/backend/quality-guidelines.md` 的 HTTP Baseline 契约：生成器不再绑定 Go 1.20.14，改为新基线语义。
- [ ] 清零校验：`rg -n '1\.20\.14' .github app/tests .trellis/spec/backend/quality-guidelines.md` 零命中（`.trellis/tasks/**` 的历史与规格说明不在清零范围）。
- [ ] `.travis.yml` 对齐最低 Go 1.26 并移除浮动 `go get -u`；不在 A 删除或重构 Travis。
- [ ] 在受控 Linux 环境运行 `app/cmd` 源码生成、生成后二进制 build、`revel run` 真实 HTTP smoke 和 `revel package`；检查 tarball 存在、非空、可解包。
- [ ] Node 24 运行 `npm test`，确认发现并通过 10 个测试。
- [ ] CI 或本地任何跳过必须是规格允许的非阻断平台项；A-AC0 至 A-AC9 不允许以跳过满足。

## Phase 6：全量验收与复核

- [ ] Go 1.26.7 与 1.27.0：`go build ./app/...`、`go vet ./app/...`、全部 Go tests 结果一致。
- [ ] MongoDB 5.0：默认 replay 的 Golden/USN 连续两次通过，目标测试数非零，Golden hash 零变化。
- [ ] `go mod tidy` 第二次零 diff、`go mod verify`、`npm test`、`gofmt`、`git diff --check` 全部通过；`rg -n '1\.20\.14' .github app/tests .trellis/spec/backend/quality-guidelines.md` 零命中。
- [ ] 逐项核对 A-AC0 至 A-AC9；审查 diff 无 Revel/Mongo 版本升级、MongoDB 7/8 假验证、Schema/API 变化、隐藏 fallback、无关格式化或未解释模块漂移。
- [ ] 验证错误路径：依赖下载/checksum、源码生成、启动和打包失败均返回非零且诊断可定位。

## Rollback Points

- 模型契约测试与 tag 修复为独立批次；任何 BSON/JSON 差异整体回退 tag 批次。
- Bootstrap 批次（go directive + x/tools）互为前提、整体回滚；其后每个直接依赖分别回滚，不回退已验证且无关的前序模块。
- 每类 vet 修复分别回滚；不借回滚混入业务修改。
- CI/Travis/README/harness/spec 契约迁移只回滚本任务改动，不破坏 G 已验证的 replay 语义（replay 默认、record 显式、Mongo 5.0 fixture 不变）。
- 任一差异无法解释时停止 A；不提前并入 B、C-a、C-b 或 F。
