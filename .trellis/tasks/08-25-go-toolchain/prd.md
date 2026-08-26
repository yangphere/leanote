# Go 工具链与通用依赖现代化（A）- PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在不改变 Leanote 业务、外部 API、USN、MongoDB Schema、服务端模板和发布物语义的前提下，把源码最低版本提升到 Go 1.26，以 Go 1.26/1.27 双版本建立构建、静态检查和测试门禁；同时对本任务拥有的直接依赖作可归因升级，并清零现代 `go vet ./app/...` 已确认的 237 条问题。

用户价值是让后续 Revel、MongoDB 和前端迁移建立在仍受支持、能持续检查且行为已被锁定的 Go 基线上，而不是把多个迁移故障混在一起排查。

## Readiness And Dependencies

### 轨道选择

- 本任务是后端轨道首个叶任务，`task.json.meta.depends_on` 仅包含 `08-25-regression-baseline`。
- `08-25-regression-baseline` 的任务元数据已标记 `completed` 并归档，因此本任务按 Trellis 元数据属于 ready。
- 同时 ready 的 `08-25-frontend-build-chain` 位于父规划的前端轨道（Phase 2B）；本任务位于后端轨道（Phase 2A），按轨道顺序优先。

### 实质启动门禁（2026-08-26 已解除）

仓库现状复核结果：

- **已满足**：`.github/workflows/regression-baseline.yml`、`app/tests/golden/{api,usn,web}` 与 `app/tests/harness/` 均已实现并提交（commit `2dc85af`）；`app/tests/config_test.go` 等调试脚本与包级 `init()` 已按 R-G4 清理，`auth_test.go` 在无 Mongo 时跳过而非 panic。
- **已满足**：G-AC8 已由 draft PR [#3](https://github.com/yangphere/leanote/pull/3) 的真实
  `pull_request` 运行 [32871393901](https://github.com/yangphere/leanote/actions/runs/32871393901)
  完成取证（head `0fce6e7b933166412142ad8b109edcdef414163a`）。`go-replay` 在 Ubuntu 22.04
  恢复 MongoDB 5.0 fixture 后通过 `TestAuth` 与 Golden/USN replay；`node-tests` 使用 Node 24.19.0，
  `npm test` 发现并通过 10/10 个测试、失败 0 个；证据已回填 G 归档。

本任务现在同时满足**元数据 ready**与**实质可启动 ready**。本次处置选择原裁决的 push 取证路径：

1. 仅把已提交 HEAD 推到临时分支 `codex/g-ac8-evidence`，未推送 `dev`，也未包含工作区规划改动；
2. 创建指向 `master` 的 draft PR #3，实际触发 Regression baseline workflow；
3. PR 事件的 `go-replay` 与 `node-tests` 均成功，`record-export-pdf` 按事件边界跳过；
4. 2026-08-26 用户明确要求同时解决 G-AC8 与 A 启动问题，构成证据回填后的 `task.py start` 批准。

G-AC8 硬阻塞已解除；A 仍不得补造简化 Golden、跳过集成测试或把失败解释为已知环境限制。

## Confirmed Facts

- `go.mod` 当前声明 `go 1.15`；本机 `go1.27.0 windows/amd64` 下 `go build ./app/...` 可通过。
- Go 官方发布源在 2026-08-25 显示最新稳定补丁为 Go 1.26.7 与 1.27.0；采用的源码最低版本仍写作 `go 1.26`。
- 当前 `go vet ./app/...` 实测 exit 1，共 237 条：205 条非法 struct tag、21 条 unkeyed literal、6 条 unreachable、3 条 self-assignment、1 条格式化调用错误、1 条无缓冲 signal channel 错误。
- **快照必须在 `go clean -cache` 之后生成**：2026-08-25 审核实测，陈旧构建缓存会使 vet 静默少报（缓存态仅 31 条、`./app/info/` 零输出；清缓存后恢复完整 237 条）。以未清缓存的输出为基线会把 205 条 tag 误判为已修复。
- 205 条非法 tag 分布在 `app/info` 的 18 个文件。旧 mgo 在 tag 不含冒号时会把整段原始 tag 当作 BSON tag，并识别其中 `omitempty`；`encoding/json` 不会把这种原始 tag 当作 JSON tag。
- `.github/workflows/regression-baseline.yml` 已随 G 交付（含 replay job、Node job与手工
  `record-export-pdf` job），并已在 PR #3 的真实 `pull_request` 运行 32871393901 中通过 replay/Node
  两个阻断 job；现有 `.travis.yml` 固定 Go 1.15、执行浮动的 `go get -u github.com/revel/cmd/revel`，
  且不运行 build/vet/Go tests。
- `app/tests` 已无包级 `init()`（G 的 R-G4 清理后 `auth_test.go` 自行探测 Mongo，无 Mongo 时跳过、`LEANOTE_REQUIRE_MONGO=1` 时失败）；该包仍是集成测试与 harness 所在地，DB-independent 模型契约单测仍放在 `app/info`，理由是包职责边界而非 init 副作用。
- 旧 `mgo.v2` 只使用 legacy opcode，无法在 MongoDB 5.1+ 执行 CRUD；在 B 完成官方驱动迁移前，A 的数据库/Golden 验证必须继续使用 G 的 MongoDB 5.0 测试环境，不能要求 MongoDB 7/8。
- 当前可归因的非 Revel、非 Mongo 直接依赖共有 6 个；版本与调用边界见 R-A2。
- G 的 replay 夹具在每次运行时经 `LEANOTE_TEST_GO` 指定的独立 Go 1.20.14 执行 legacy Revel 源码生成（`app/tests/harness/server.go` 的 `buildServerBinary` → `app/cmd` parser2）；`app/tests/README.md` 记录 Go 1.26/1.27 在旧 x/tools 类型检查器下生成 panic。该契约连同 workflow 双 job、README、server 提示/测试和 backend quality spec 中的 Go 1.20.14 引用，全部属于本任务迁移范围。

## Inputs And Outputs

### Inputs

- 已提交的 G Golden/USN/smoke、MongoDB 5.0 fixture 与 workflow 定义，以及已回填 G 归档的
  G-AC8 PR 运行证据（PR #3 / run 32871393901）；
- `go.mod`/`go.sum` 当前模块图；
- `go clean -cache` 后重新生成的 `go vet ./app/...` 完整快照（当前为 237 条）；
- `CONTEXT.md` 的 API、USN、所有权查询、BSON 字段不变量；
- 父任务 `research/external-facts.md` 与 ADR-0001。

### Outputs

- `go.mod`/`go.sum` 的 Go 1.26 基线和已批准直接依赖版本；
- DB-independent 的 BSON/JSON 模型契约测试；
- 只为清零既有 vet 问题而做的聚焦源码修复；
- G 的 Go workflow 扩展为 1.26.7/1.27.0 双版本矩阵，并将遗留 Travis 配置从 Go 1.15 对齐到最低 1.26；
- 每个依赖升级、vet 类别、Golden/USN、源码生成、启动/打包入口的可审计验证记录。

## Requirements

### R-A0 前置门禁不可伪造

- G 的实现资产与真实验收记录必须存在；归档状态本身不算证据。
- A 只消费 G 的 replay 基线，不录制、刷新或弱化 Golden。
- A 的数据库测试固定 MongoDB 5.0；MongoDB 7.0/8.0 验收属于 B。
- 无 Mongo、Docker、网络或 CI 权限时必须明确失败/阻塞，不得静默跳过后宣称任务通过。

### R-A1 Go 版本政策

- `go.mod` 精确声明 `go 1.26`，不添加 `toolchain` directive，不依赖 `GOTOOLCHAIN=auto` 隐式选择编译器。
- 阻断 CI 分别固定 Go 1.26.7 与 1.27.0，并对两个版本运行相同的 build、vet 和 Go tests；未来补丁更新必须通过可审查的 workflow diff，不使用 `stable`、`latest` 或未锁定的 `1.x`。
- 生产代码和测试不得使用 Go 1.27 独有语法或仅在 1.27 提供的标准库 API。
- `.travis.yml` 不在本任务删除或重构，但必须停止使用 Go 1.15 和浮动 `go get -u`，Go 版本选择器显式限定到 minor 或 patch 且不低于 1.26（如 `1.26.x` 或 `1.26.7`），仅禁止 `stable`/`latest`/`tip` 等跨 minor 滚动别名。Travis 属非阻断遗留入口，不受 R-A1 阻断 CI 的补丁级锁定约束；最终删除/替换由 F 负责。

### R-A2 直接依赖所有权与目标

版本候选以 2026-08-25 的 `go list -m -u` 为证据，实施不得自行漂移到其他版本：

| 模块 | 当前版本 | A 的处置 | 主要调用边界 | 必须验证 |
|---|---|---|---|---|
| `github.com/PuerkitoBio/goquery` | v1.6.1 | 升至 v1.12.0 | `app/lea/Util.go` 的 `SubStringHTML` | 截断、多字节字符、不完整标签和 HTML 补全输出 |
| `github.com/agtorre/gocolorize` | v1.0.0 | 保持；上游无更新，C-b 删除 `app/cmd` 时一并消失 | `app/cmd/revel.go` | CLI 帮助/错误路径和源码生成不退化 |
| `github.com/jessevdk/go-flags` | v1.4.0 | 升至 v1.6.1 | `app/cmd/revel.go` | CLI 参数解析与源码生成 |
| `github.com/robfig/config` | 2014 pseudo-version | 保持当前版本；替换解析器延期为 `MOD-003` | `app/lea/i18n` 消息文件解析 | 7 个 locale 的 section、key、插值和缺失键行为 |
| `golang.org/x/crypto` | 2020 pseudo-version | 升至 v0.55.0 | `app/lea/crypto.go` 的 bcrypt | 既有 hash 校验、错误密码、生成 hash 可回验 |
| `golang.org/x/tools` | 2020 pseudo-version | 升至 v0.49.0 | `app/cmd/parser2` 的 `go/packages` | routes/tmp 源码生成和生成后二进制构建 |

- `github.com/revel/cmd`、`github.com/revel/modules`、`github.com/revel/revel` 及其他 `github.com/revel/*` 版本由 C-a 拥有；`gopkg.in/mgo.v2` 由 B 拥有，A 不改变这些模块版本。
- 升级顺序由硬约束决定：goquery v1.12.0、x/crypto v0.55.0、x/tools v0.49.0 要求主模块 `go >= 1.25`；而 go directive 升到 1.26 后旧 x/tools 生成即 panic、Go 1.20.14 生成器又无法读取 `go 1.26` 主模块。因此 go directive 与 x/tools 必须作为同一原子 bootstrap 批次先行落地并验证源码生成，其余依赖在其后逐个升级；该批次是独立回滚单元。R-A2 表格定义各模块处置，不定义先后顺序。
- A 可以把旧 `bson.RegEx` 或 Revel result literal 改成具名字段以消除 vet，但只能保持相同字段值和控制流；这不授权升级或重写对应框架/驱动。
- 每个允许升级的直接依赖单独修改并验证。共享传递依赖只可因该直接依赖的已解析模块图而变化，必须能说明因果。
- `robfig/config` 在 A 中保持当前版本；A 仍建立消息解析契约，为 `MOD-003` 提供替换前基线，但不引入第二解析器或兼容 fallback。
- 不运行 `go get -u ./...`。拟删除模块必须同时满足 `go mod why -m` 无业务路径、源码/生成入口无引用、针对性入口仍通过；当前 6 个模块均有实际调用方，不能仅凭 `// indirect` 或陈旧注释删除。
- 目标模块若要求高于 Go 1.26、引入另一套框架/配置/日志实现、改变许可证或无法通过调用方验证，必须停止该模块升级并回到规划，不得静默选旧版或增加兼容 fallback。

### R-A3 Vet 零错误及归属

- 在 `go clean -cache` 后重新生成的完整快照为准，清零 `go vet ./app/...` 的所有输出；若数量/类别与 237 条基线不同（缓存污染或上游 analyzer 行为变化均可导致），先解释差异并更新规划证据。
- 205 条 struct tag 按 R-A4 处理。
- 21 条 unkeyed literal 全部改为具名字段，包括本项目模型、`mgo/bson.RegEx` 和 `revel.PlaintextErrorResult`；字段值、正则 pattern/options、错误文本和 Apply 行为不得变化。
- 6 条 unreachable 只删除不可达语句；3 条 self-assignment 只删除无效赋值。不得借机改变相邻业务分支。
- `BaseController.NotFound` 的格式化修复必须保持客户端实际可见的状态码与响应文本；当前 `E404` 位于 `app/controllers/BaseController.go:161-163` 并传入 `("", nil)`，Revel 会执行 `fmt.Sprintf`，不能假定额外参数被无声忽略，必须以 G replay/聚焦测试锁定实际正文。（2026-08-26 复核确认该位置与调用形态不变。）
- signal channel 只改为容量 1（2026-08-26 复核定位：`app/cmd/harness/harness.go:333` 的 `make(chan os.Signal)`，`:334` 订阅 `os.Interrupt`、`os.Kill`），保持当前 signal 订阅集合与中断后的 kill 流程，不在 A 新增 SIGTERM 语义；SIGTERM 属于 C-a 的 Revel 1.1 入口验收。
- 不通过 analyzer 关闭、`//nolint`、缩小包范围、构建标签排除、吞错或宽泛 fallback 获得绿灯。

### R-A4 Struct Tag 与数据契约

- 对 18 个 `app/info` 文件中的 205 个无效原始 tag 建立完整清单；旧值 `X` 必须等价迁移为 `bson:"X"`，旧值 `X,omitempty` 必须等价迁移为 `bson:"X,omitempty"`。
- 无效原始 tag 当前对 `encoding/json` 等价于没有 JSON tag，因此不得因为 BSON 修复而新增 `json` 名称、`omitempty` 或 `-`。已有合法 `json` tag（例如密码字段的 `json:"-"`）保持不变。
- BSON 字段名、`omitempty`、ObjectId、时间、nil/空集合和嵌套结构行为保持；MongoDB collection 与 Schema 不变，不执行数据迁移。
- JSON 字段名、零值是否出现、ObjectId 小写 24 位 hex、字段类型及 Golden 可见顺序保持。
- 纯序列化测试放在 `app/info` 的 DB-independent 测试文件/测试数据中，不放入会执行 Mongo 初始化的 `app/tests` 包；BSON 与 JSON 分别断言，并覆盖所有被改 tag 的类型和每个 `omitempty` 边界。
- 模型契约先在旧 tag 上通过并记录当前有效行为，再修改 tag；修改后同一断言保持通过。不得从 Go 字段名或“原本可能想表达什么”猜测存储契约。

### R-A5 行为、入口与错误处理

- `/api/*`、USN、用户所有权查询、MongoDB collection/字段、模板输出和生成路由保持不变。
- 依赖升级或 vet 修复出现 Golden/USN/模型契约差异时，必须定位到具体改动并修复或回滚；不能刷新 Golden 吸收差异。
- `app/cmd` 源码生成、生成后二进制构建、`revel run` 开发启动和 `revel package` 生产打包必须仍可执行；命令失败必须保留原始错误和非零退出码。
- 依赖下载、checksum、`go mod tidy` 或 `go mod verify` 失败即失败，不使用 vendored 临时副本、跳过校验或手工伪造 `go.sum`。
- `go mod tidy` 后逐项审查模块 diff：Revel/Mongo 直接版本零变化，没有未知模块、第二套框架/日志/配置实现或无法解释的传递升级。
- 仓库中所有指向 Go 1.20.14 的可执行契约由本任务迁移到新基线：`.github/workflows/regression-baseline.yml` 的 replay job 与手工 `record-export-pdf` job、`app/tests/README.md`、`app/tests/harness/server.go` 错误提示及 `server_test.go` 断言、`.trellis/spec/backend/quality-guidelines.md` 契约。迁移后 `LEANOTE_TEST_GO` 为可选覆盖，但必须 fail-closed：缺省解析 PATH 中的 go 时，harness 在启动前校验版本 ≥1.26.7，低于下限显式失败；生成/构建子进程强制 `GOTOOLCHAIN=local` 禁止自动下载工具链；用桩测试锁定旧版本拒绝、用真实 1.26.7/1.27.0 锁定通过。
- `.trellis/tasks/**`（含本任务自身未归档的 PRD/design/implement）中的 1.20.14 属历史与规格说明，不在清零范围；零残留门禁限定为可执行契约路径：`rg -n '1\.20\.14' .github app/tests .trellis/spec/backend/quality-guidelines.md` 必须零命中。

## Acceptance Criteria

- [ ] **A-AC0** G 的 Golden/USN/harness、MongoDB 5.0 fixture 均存在，且 G-AC8 证据已按"实质启动门禁"完成处置（真实 workflow 运行链接，或用户批准的本地 replay 替代证据已记录进 G 归档）；A 的实现提交基于该证据，而不是仅基于 archived status。
- [ ] **A-AC1** `go.mod` 声明 `go 1.26` 且无 `toolchain` directive；阻断 CI 固定 Go 1.26.7/1.27.0，并在两个版本执行相同 build/vet/Go tests。
- [ ] **A-AC2** R-A2 每个模块都有 current/target/调用方/验证结果；`robfig/config` 保持当前版本并链接 `MOD-003`，Revel/Mongo 模块版本不变，模块图不存在无法解释的变化。
- [ ] **A-AC3** `go vet ./app/...` 在 Go 1.26.7 与 1.27.0 均零输出、exit 0，且没有 analyzer 抑制或范围缩减。
- [ ] **A-AC4** 205 个 legacy BSON tag 全部等价命名化；DB-independent 测试覆盖 BSON/JSON 名称、零值、ObjectId、时间、nil/空集合和全部 `omitempty` 边界，HTTP Golden 无变化。
- [ ] **A-AC5** MongoDB 5.0 fixture 中，两个 Go 版本分别以默认 replay 运行 `go test ./app/tests/... -count=1`，目标测试数量非零且 Golden/USN 全绿；未运行 MongoDB 8.0 假验证。
- [ ] **A-AC6** `SubStringHTML`、bcrypt、i18n、CLI 参数、`go/packages` 源码生成均有针对性回归，且源码生成在 Go 1.26.7 与 1.27.0 下均无 panic；源码生成后二进制可构建。
- [ ] **A-AC7** `revel run` 真实 HTTP smoke 与 `revel package` tarball 验证通过，错误路径保持非零退出；`.travis.yml` 不再请求 Go 1.15 或浮动 `go get -u`；R-A5 所列全部 Go 1.20.14 契约（workflow 双 job、README、harness 提示/测试、backend quality spec）完成迁移，且 `rg -n '1\.20\.14' .github app/tests .trellis/spec/backend/quality-guidelines.md` 零命中。
- [ ] **A-AC8** `go mod tidy` 后连续运行零 diff，`go mod verify` 通过，`go.mod`/`go.sum` 可重复且许可证边界已复核。
- [ ] **A-AC9** Node 24 下 `npm test` 仍发现并通过 10 个测试；最终 `gofmt`、`git diff --check` 与聚焦 diff 复核通过。

## Out Of Scope

- Revel 1.0 -> 1.1、迁出 Revel、删除 `.travis.yml` 或重建最终 CI/CD。
- `mgo.v2` -> 官方 MongoDB 驱动、MongoDB 7/8 运行验证、Schema/数据迁移。
- API、USN、认证、所有权查询、模板、前端资源或业务功能变化。
- 替换 `robfig/config` 或引入新消息配置解析器；该结构性工作登记为 `MOD-003`。
- 用新框架、日志库、配置系统或兼容层包装依赖升级问题。
- 修复 Golden 已钉住的历史业务缺陷；发现后单独登记，不在 A 顺手修改。

## Risks And Deferred Items

- G 的实现资产与 G-AC8 真实 PR 运行证据均已交付，原归档流程冲突已通过证据回填闭合；
  若 PR #3 或 run 32871393901 后续不可核验，A 必须重新停止并恢复此门禁。
- 陈旧构建缓存会使 `go vet` 少报（2026-08-25 实测 31 vs 237）；所有 vet 快照与清零验证都必须在 `go clean -cache` 后执行。
- `app/cmd` 是修改过的 Revel cmd 副本，`x/tools`/`go-flags` 升级可能破坏源码生成；必须用真实生成入口验证，不能只靠 `go build ./app/...`。
- go directive 升到 1.26 与旧 x/tools 生成 panic、Go 1.20.14 生成器失效三者互相锁死；不按 bootstrap 批次处理会让 Phase 3 中途没有可用工具链（已按 R-A2 排序约束消除）。
- 手工 `record-export-pdf` job 与本地 replay 文档仍绑定 Go 1.20.14；只迁移阻断矩阵会出现在 CI 变绿的同时破坏 PDF 补录和本地入口的假阳性，已列为 R-A5/A-AC7 强制范围。
- `robfig/config` 没有可直接升级的新版本且位于运行时 i18n 路径；用户已决定 A 保持现状并用七种语言契约保护，替换工作延期为 `MOD-003`。
