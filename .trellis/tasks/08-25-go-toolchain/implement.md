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
- [x] 用 MongoDB 5.0 fixture 连续两次 replay G 的 Golden/USN，确认目标测试数量非零、结果一致、Golden 文件 hash 不变。
  （2026-08-26 实证：Docker `mongo:5.0` + 本地 Go 1.26.7 作 `LEANOTE_TEST_GO`，两次 `go test -p 1 ./app/tests/...` 均 exit 0；
  61 个测试函数、golden 132 个文件两次运行前后 SHA256 全部一致；顺带验证 bootstrap 后 x/tools v0.49.0 在 1.26.7 下生成不再 panic。）
- [x] 运行 `go clean -cache` 后再记录 `go vet ./app/...` 完整快照；缓存态输出不可作为基线（实测可少报至 31 条）。同时记录 `git status`、`go env GOVERSION GOTOOLCHAIN GOFLAGS`、`go list -m -u` 和 `go mod why -m`；vet 差异先回到规格审查。
  （2026-08-26 复核：从未修改的 `HEAD` 归档、分别清缓存后，`research/vet-baseline-go1.26.7.txt` 与
  `research/vet-baseline-go1.27.0.txt` 各 237 条、各自 exit 1；两版原始诊断顺序不同，但排序规范化后的行集合完全一致。
  分类为 205 条 invalid struct tag、21 条
  unkeyed literal、6 条 unreachable、3 条 self-assignment、1 条 printf misuse、1 条 signal channel。原先 36 条
  快照已替换为完整基线；`research/env-snapshot.txt`、`research/go-list-m-u.txt`、`research/go-mod-why.txt` 已存档，
  全部直接依赖均有真实 import 路径，无可删除项。）
- [x] 核对官方稳定补丁仍为 1.26.7/1.27.0；如版本政策或目标依赖发生实质漂移，更新规划并重新审批。
  （2026-08-26 核对 go.dev/dl JSON：stable 最新为 go1.27.0 与 go1.26.7，与规划一致，无漂移。）
- [x] 确认 PRD/design/implement 不含未决产品或范围问题；`robfig/config` 按已批准的 `MOD-003` 延期决策执行。
  （2026-08-26 关键词扫描 TODO/待定/开放问题/未决/TBD 零命中（仅本门禁条目自身）。）

## Phase 1：先建立 DB-independent 契约测试

**Create/Modify:** `app/info/*_test.go`、必要的 `app/info/testdata/`、`app/lea/*_test.go`、`app/lea/i18n/*_test.go`、`app/cmd/**/*_test.go`

- [x] 在旧 tag 上建立 18 文件/205 字段的 BSON tag 清单与 BSON/JSON 行为快照，覆盖所有 `omitempty`、ObjectId、时间、nil/空集合和嵌套结构。
  （2026-08-26 trellis-check 实证：`legacyTagInventory` 与 HEAD 原始 tag 程序化比对，207 个 raw-tag 字段减去死代码 `TagNote` 的 2 个 = 205 行，
  key/omitempty 零错配、零虚构；行为快照覆盖零值/BSON 往返/ObjectId hex/`json:"-"` 排除与全部 35 个注册类型。）
- [x] 证明模型测试不连接 Mongo，且 `go test ./app/info/... -run <target> -count=1 -v` 实际发现目标用例。
  （2026-08-26：grep 无 mgo.Dial/Session、不 import app/db；`go test ./app/info/... -count=1 -v` 5/5 顶层 PASS 含 205 子用例矩阵。）
- [x] 为 `SubStringHTML`、bcrypt、i18n 解析、CLI 参数和 `go/packages` 源码生成补聚焦回归。
  （`app/lea/util_substring_html_test.go` 13 子用例、`crypto_bcrypt_test.go` 冻结 hash 契约、`app/lea/i18n/i18n_contract_test.go` 7 locale、
  `app/cmd/flags_contract_test.go`、`app/tests/harness/generate_contract_test.go` 真实生成+二进制构建。）
- [x] 运行新测试并记录修改前结果；测试不得依赖待修改实现生成期望值。
  （期望锚定 HEAD 旧 tag 而非修改后实现：trellis-check 以 `git show HEAD:` 独立复核 205/205 一致，无同义反复。）

## Phase 2：等价修复 struct tag

**Modify:** 设计第 4 节清单中的 18 个 `app/info/*.go`

- [x] 逐字段把 legacy raw BSON tag 原值移入 `bson:"..."`；不自动新增/改变 JSON tag。
  （2026-08-26 trellis-check：18 文件 token 级 diff 仅 tag 变化，205 处新增、0 处改动/删除既有 tag；`json:"-"`（Pwd、ImageSize 等）逐字节不变。）
- [x] 运行 `gofmt`、模型 BSON/JSON 契约、G Golden/USN；205 条 tag 警告归零且序列化/HTTP 输出不变。
  （2026-08-26：gofmt 缺失由 trellis-check 补跑修复（8 文件，含 WIP 引入的 5 个）；契约测试全绿；
  Golden replay 两次 + checker 复跑各一次均绿，132 golden 文件 SHA256 零 diff；205 条 tag 类归零后，当前 vet 剩余 36 条，后续类别见 Phase 4。）
- [x] 复核所有已有合法 `json:"-"`、ObjectId、字段名和 `omitempty` 行为未被改写。
  （trellis-check 独立 HEAD 审计：无字段改名、无类型变化、无逻辑变化；历史 tag 拼写 `ToCommendId`/`CommendId` 按契约保真保留。）

## Phase 3：建立 Go 1.26 模块基线

**Modify:** `go.mod`、`go.sum`

> 顺序约束：goquery/x/crypto/x/tools 目标版本要求主模块 `go >= 1.25`；而 G 夹具的每次 replay 都要用 `LEANOTE_TEST_GO` 指定的 Go 1.20.14 运行 legacy 源码生成（`buildServerBinary` → `app/cmd` parser2），README 已记录 Go 1.26/1.27 在旧 x/tools 下生成 panic、Go 1.20.14 又读不了 `go 1.26` 主模块。因此第 1 步必须是原子 bootstrap 批次，顺序不可调换。

- [x] **Bootstrap（原子批次，整体回滚单元）**：同批修改 go directive 为 `1.26`（确认无 `toolchain` directive）并把 `golang.org/x/tools` 升至 v0.49.0；立即用 Go 1.26.7 与 1.27.0 各执行一次真实生成入口（`app/cmd` build 子命令，即 harness `buildServerBinary` 的等效流程）确认不再 panic、生成 routes/tmp 后二进制可构建，然后运行 G replay。
  （2026-08-26：go.mod `go 1.26`、无 toolchain directive、x/tools v0.49.0；真实生成入口在 Go 1.26.7（replay ×3 含 checker 复跑）与 Go 1.27.0
  （TestGenerateLegacyEntrypointAndBinary 4.7s）均无 panic 且二进制可构建；G replay 全绿。）
- [x] 记录允许/禁止模块版本快照；Revel/Mongo 模块版本在整个阶段必须不变。
  （`research/go-list-m-u.txt` 与 `research/go-mod-why.txt` 为快照与调用方证据；go.sum 传递升级均可归因于已批准的 x/tools+x/crypto；
  Revel/mgo 版本零变化。）
- [x] 对允许模块按以下顺序逐个升级（bootstrap 之后），每项执行调用方测试、`go build ./app/...`、G replay 和模块 diff 审查：
  1. `github.com/PuerkitoBio/goquery` v1.6.1 -> v1.12.0；（2026-08-26：SubStringHTML 13 子用例+basics 全绿，build exit 0，
     Go 1.26.7 replay exit 0 且 132 golden 文件 SHA256 零 diff；传递 cascadia v1.1.0→v1.3.3 经 `go mod graph` 归因于
     goquery@v1.12.0 自身要求。）
  2. `github.com/jessevdk/go-flags` v1.4.0 -> v1.6.1；（2026-08-26：app/cmd flags 契约 3 测试全绿，
     TestGenerateLegacyEntrypointAndBinary 真实生成+二进制构建 4.21s PASS（Go 1.26.7），build exit 0，replay exit 0 且 golden 零 diff；
     零传递升级——go-flags 要求 x/sys ≥0.21.0 已被解析的 v0.47.0 覆盖。）
  3. `golang.org/x/crypto` -> v0.55.0；（2026-08-26 已完成：bcrypt 冻结 hash 契约全绿，go.sum 传递变化已归因审查。）
  4. `github.com/robfig/config` 保持当前版本，运行消息解析契约并核对 `MOD-003` 链接；（2026-08-26：
     `go test ./app/lea/i18n -count=1 -v` TestMessageContract 全绿，覆盖 de-de/en-us/es-co/fr-fr/pt-pt/zh-cn/zh-hk 七个 locale；
     `go list -m -u` 确认上游无可升级版本；MOD-003 延期决策记录于 `docs/modernization-backlog.md:23-31`。）
  5. `github.com/agtorre/gocolorize` 保持 v1.0.0 并记录无更新。（2026-08-26：`go list -m -u github.com/agtorre/gocolorize`
     无 `[vX.Y.Z]` 升级提示，上游自 v1.0.0 后无发布。）
- [x] 对任何拟删除模块同时执行 `go mod why -m` 与源码/生成入口搜索；有实际路径则保留。
  （Phase 0 已存档 `research/go-mod-why.txt`：全部直接依赖均有真实 import 路径，无可删除项；本阶段升级未产生新的删除候选。）
- [x] 运行 `go mod tidy` 两次并确认第二次零 diff，随后 `go mod verify`；审查所有传递变化的来源、GoVersion 和许可证。
  （2026-08-26：tidy 第二次运行 go.mod/go.sum 均零 diff；`go mod verify` "all modules verified" exit 0；
  升级模块 GoVersion 均 ≤1.25（goquery 1.25.0、cascadia 1.16、go-flags 1.20、x/crypto 1.25.0、x/tools 1.25.0）且许可证无变化；
  最终 diff 全量归因见 `research/module-upgrade-log.md` Transaction 4——Revel/mgo 版本值零变化。）

## Phase 4：按类别清零 vet

**Modify:** `app/info`、`app/cmd`、`app/controllers`、`app/lea`、`app/service` 中实现前快照点名的文件

- [x] 将 21 个 unkeyed literal 改为具名字段；显式核对项目模型、10 个 `bson.RegEx` 的 Pattern/Options 与 `revel.PlaintextErrorResult` 的错误值。
  （2026-08-26：mgo `bson/bson.go:428-431` 确认 Pattern/Options 字段序，10 个 RegEx 调用点逐一核对位置实参；
  PlaintextErrorResult 唯一字段 Error；BlogItem/BlogStat/ArchiveMonth/NoteAndContent/NoteAndContentSep/ShareNoteWithPerm
  按 app/info 定义逐字段映射（NoteAndContentSep 字段名 NoteInfo/NoteContentInfo 与变量名不同，已按类型核对）；
  `git diff` 确认仅 21 个预期 hunk；replay exit 0 且 golden 零 diff。）
- [x] 删除 6 个不可达语句（`app/lea/captcha/Captcha.go:388`、`app/cmd/harness/build.go:100/240`、`app/cmd/build.go:96`、`app/lea/File.go:147`、`app/controllers/AuthController.go:100`）和 3 个 self-assignment（`app/service/NoteService.go:972`、`app/controllers/member/MemberBlogController.go:383/483`），只保留原可达控制流与输出。
  （2026-08-26：harness/build.go 的 :100/:240 为同一 return 后死区的两个不可达 CFG 块首语句，整段删除原 99–241 并移除仅被死区使用的 import path/runtime/time；
  cmd/build.go 同理删除死区并将仅被死区消费的 `app` 改为 `_`（同一具名 err）；其余三处单语句删除。
  self-assignment 三处均核实单一绑定无 shadowing（filename 于 :373/:474 单次声明）。全量 replay exit 0、golden 零 diff。）
- [x] 先锁定 `E404`（`app/controllers/BaseController.go:161-163`）当前状态码/正文（包含 Revel 对 `("", nil)` 的实际格式化结果），再改成 vet-clean 表达并要求输出不变。
  （2026-08-26：Revel v1.0.0 controller.go——NotFound 仅在 len(objs)>0 时 Sprintf ⇒ `("", nil)` 实际 Description=""，
  改为 `c.NotFound("")` 输入 RenderError 的 Error{Title:"Not Found",Description:""} 与 status 404 完全一致；
  golden 中 `_notFound/_invalid` 钉的是 /api JSON 信封（status 200），不经 E404 HTML 管线；
  真实覆盖为 smoke 的 `/preview` 404+HTML 断言：改写前（Batch D replay）与改写后（Batch C 及最终门禁 replay）均 PASS。
  Route.go 两处以常量格式 `%s` + 消息作实参实现逐字节等价（objs 为空时 Sprintf 从不执行）；NoteController 两处 `前缀+Sprintf("%v",err)` ≡ `Sprintf("前缀 %v",err)`。）
- [x] 将 `app/cmd/harness/harness.go:333` 的 signal channel 容量改为 1，保持现有 `os.Interrupt`/`os.Kill` 订阅集合，验证 interrupt 后的 kill/退出路径；不新增 SIGTERM 行为。
  （2026-08-26：容量 1、订阅集合与 `<-ch`→Kill→Exit(1) 流程零改动；`go vet ./app/cmd/...` exit 0，harness 单测 ok。）
- [x] 每完成一个类别运行拥有该行为的聚焦测试与 G replay；最终两个 Go 版本的 `go vet ./app/...` 均须零输出、exit 0。
  （2026-08-26：A/B/C 各自全量 replay exit 0 且 golden 零 diff，B2/D 由过滤套件 replay + 最终门禁覆盖；
  最终门禁 `go clean -cache` 后系统 go1.27.0 与 C:\Users\rog\sdk\go1.26.7\bin\go.exe 双版本 `go vet ./app/...` 均零输出 exit 0；
  Golden 经 `git hash-object` 对 HEAD blob 132/132 一致。分批证据全文见 `research/vet-zero-proof.md`。）

## Phase 5：CI 与真实入口

**Modify:** G 已创建的 `.github/workflows/*.yml`、`.travis.yml`、`app/tests/README.md`、`app/tests/harness/server.go` 及其断言测试、`.trellis/spec/backend/quality-guidelines.md`

- [x] 把 G 的 Go job 扩展为固定 1.26.7/1.27.0 矩阵；两个版本运行相同 build、vet、DB-independent tests 和 MongoDB 5.0 replay。
  （2026-08-26：workflow `go-replay` 改为显式矩阵 ['1.26.7','1.27.0']，fail-fast false，两值执行相同 build/vet/DB-independent 单测/Mongo 5.0 replay 步骤；
  两版本本地等效验证——`go build ./app/...` 与 `go vet ./app/...` 双版本零输出 exit 0，全量 replay 分别在 go1.26.7（LEANOTE_TEST_GO 覆盖）与 go1.27.0（缺省 PATH）各跑一次均绿；
  真实 PR 矩阵运行待下次 push 取证。）
- [ ] 迁移手工 `record-export-pdf` job：`setup-go` 从 1.20.14 改为固定 1.26.7，确认 wkhtmltopdf 安装、Mongo fixture 与 `LEANOTE_GOLDEN=record` 流程在新工具链下成立。record 流程验证必须在隔离 checkout 或临时副本中执行（workflow_dispatch 实跑优先），禁止在本工作区直接运行 record；验证前记录 Golden hash，验证后工作区 Golden 文件必须零 diff，record artifact 不回填仓库。
  （2026-08-26：workflow diff 已完成——setup-go '1.20.14'→'1.26.7'，wkhtmltopdf 0.12.6.1 安装+sha256 校验、Mongo 5.0 fixture、`LEANOTE_GOLDEN=record` 流程与 artifact 上传保持不变；
  **pending-real-CI**：workflow_dispatch 实跑需 push 权限，本地按规格禁止 record（工作区 replay-only），故"新工具链下成立"的实证待真实 dispatch 运行回填；
  本地侧 Golden 防护已复核：全部验证后 `git status --short app/tests/golden` 为空。）
- [x] 迁移本地生成契约：`app/tests/README.md` 删除"必须安装 Go 1.20.14"的 panic 规避指南；`app/tests/harness/server.go:154` 的错误提示与 `server_test.go` 断言同步——`LEANOTE_TEST_GO` 降级为可选覆盖（缺省 PATH 中的 go）。缺省工具链必须 fail-closed：启动前校验版本 ≥1.26.7（低于即显式失败并给出安装指引），生成子进程设置 `GOTOOLCHAIN=local` 禁止自动下载；旧版本拒绝用桩测试锁定，1.26.7/1.27.0 通过用真实工具链锁定。
  （2026-08-26：README 重写为新基线语义；server.go 以 `minGeneratorVersion=1.26.7` 实现 goBinary fail-closed 解析（缺失/过旧/不可读版本均在生成前显式失败并给出 LEANOTE_TEST_GO 指引），goCommand 统一注入 GOTOOLCHAIN=local 且剔除继承值；
  server_test.go 用 `go build` 编译的跨平台桩二进制（非 shell shim）锁定：旧版 go1.25.9 拒绝、恰为 1.26.7 接受、devel 输出拒绝、显式覆盖逐字透传、GOTOOLCHAIN=local 强制钉入；
  TestGoBinaryResolvesRealSystemToolchain 用系统 go1.27.0 实跑通过（无跳过）；`go test -count=1 ./app/tests/harness` 全绿 41.69s。）
- [x] 更新 `.trellis/spec/backend/quality-guidelines.md` 的 HTTP Baseline 契约：生成器不再绑定 Go 1.20.14，改为新基线语义。
  （2026-08-26：Signatures 行改为"缺省 PATH go ≥1.26.7 fail-closed，LEANOTE_TEST_GO 可选覆盖"；Error Matrix 中"Missing LEANOTE_TEST_GO → explicit failure"改为"missing/older/unreadable 默认 go 在生成前显式失败"；Code Review Checklist 段先前已迁移，全文无 1.20.14 残留。）
- [x] 清零校验：`rg -n '1\.20\.14' .github app/tests .trellis/spec/backend/quality-guidelines.md` 零命中（`.trellis/tasks/**` 的历史与规格说明不在清零范围）。
  （2026-08-26：rg exit 1 = 零命中。）
- [x] `.travis.yml` 对齐最低 Go 1.26（选择器限定 minor 或 patch：如 `1.26.x` 或 `1.26.7`；禁止 1.15 及以下和 `stable`/`latest`/`tip` 等跨 minor 滚动别名），并通过主模块依赖图构建 Revel CLI；不在 A 删除或重构 Travis。
  （2026-08-26 复核：`go: 1.15`→`go: 1.26.x`；安装步骤以 `GOTOOLCHAIN=local go build -o "$HOME/gopath/bin/revel" github.com/revel/cmd/revel` 构建，
  再用 `go version -m` 断言实际二进制携带 `golang.org/x/tools v0.49.0`；`sh/run.sh` 与 `sh/package.sh` 继续从同一 PATH 调用该二进制，
  不再使用隔离模块图的 `go install ...@v1.0.3`。）
- [x] 在受控 Linux 环境运行 `app/cmd` 源码生成、生成后二进制 build、`revel run` 真实 HTTP smoke 和 `revel package`；检查 tarball 存在、非空、可解包。
  （2026-08-26：golang:1.26.7-bookworm + `--network container:leanote-test-mongo` 共享网络命名空间满足 127.0.0.1:27017，全程 GOTOOLCHAIN=local；
  A：app/cmd 真实生成入口 GENERATION OK 无 panic，生成 routes.go/tmp 后二进制构建 OK（21.9MB）；
  B：canonical sh/run.sh smoke 经模块图构建的 revel CLI（revel/cmd v1.0.3 源码 + x/tools v0.49.0）/login→200、/→200、/blog→200、/demo→302，服务端 request log 确认真实路由；
  canonical sh/package.sh 产出 leanote.tar.gz 27,446,759 字节、2303 条目、可解包；
  **如实记录的上游缺口**：`go install ...@v1.0.3` 构建的 stock CLI 因 revel 模块图冻结的 x/tools v0.0.0-20200219 在 Go ≥1.26 type-check 必然 panic（证据 research/linux-stock-cli-panic-go1.26.7.log），属 Revel 上游依赖问题，A 规格禁止升级 Revel ⇒ 建议随 C-a 处理；
  全文证据 research/linux-entrypoint-proof.md 与 linux-entrypoint-run3-all-pass.log；容器 exit 0；工作区零残留（无 app/tmp、app/routes、sh/leanote.tar.gz）。）
- [x] Node 24 运行 `npm test`，确认发现并通过 10 个测试。
  （2026-08-26：Node v24.19.0，npm test 发现并通过 10/10、fail 0、skip 0。）
- [ ] CI 或本地任何跳过必须是规格允许的非阻断平台项；A-AC0 至 A-AC9 不允许以跳过满足。
  （2026-08-26 复核：默认 harness 实跑明确跳过 `TestGoldenExportPdf`（缺少 reviewed
  `app/tests/golden/api/note_exportPdf.json`，或缺少 `wkhtmltopdf`），并跳过
  `TestServerServesLoginOverRealHTTP`（仅在 `LEANOTE_HTTP_INTEGRATION=1` 时运行）；两项均保留 fail-closed 的
  skip 条件。Linux canonical `revel run`/`package` smoke 已在受控环境单独通过，但本地默认套件没有执行 HTTP smoke；
  `record-export-pdf` 的 `workflow_dispatch` 真实取证仍 pending，不能记为“无任何测试跳过”或完成 A-AC7。）

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
