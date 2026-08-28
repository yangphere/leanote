# Revel 1.1 基线升级（C-a）— 执行计划

> 实现前读取本任务 PRD/设计、`.trellis/spec/backend/quality-guidelines.md`、`CONTEXT.md`、ADR-0001、父任务 `research/external-facts.md`，以及 A 的归档记录（尤其 `research/linux-stock-cli-panic-go1.26.7.log`、`research/module-upgrade-log.md`、`research/linux-entrypoint-proof.md`）。

## Global Constraints

- 仅升级 Revel 三件套及其图内被动变化；非 Revel 直接依赖与 `tools.go` 零改动。
- URL、配置、Cookie、模板、API/USN 契约保持不变；Golden/USN 零 diff，不刷新基线。
- 数据库验证固定 MongoDB 5.0 fixture；不跑 MongoDB 7/8 假验证。
- 不写 C-b 的 `net/http` 代码，不删除/重植 `app/cmd`。
- CLI canonical 构建为主模块图；隔离图 `go install @v1.1.2` 只许诊断记录。
- 任何失败显式失败，保留非零退出码与原始诊断；不可解释差异即停止。

### Task 1：冻结三条路径的基线（A 交付树上）

**Files:** `go.mod`、`go.sum`、`app/cmd/`、`conf/routes`、`conf/app.conf`、`sh/run.sh`、`sh/package.sh`

- [x] 在当前 v1.0 + Go 1.26/1.27 上生成测试二进制并回放 G 的 Golden/USN，记录目标测试数与 Golden 文件 hash（升级后对照的基线）。
  （2026-08-28 同 HEAD：双版本 replay ×2 exit 0、68 测试函数、Golden 132 文件 SHA256 聚合 f6ec2ec036b91340bbf44c6387282825ac1a94de，见 research/baseline-freeze-2026-08-28.md。）
- [x] 用主模块图构建的 v1.0.3 CLI 启动 `revel run -a .`，验证 `/`、`/login`、`/note`、`/blog`、`/demo`，记录响应基线。
  （2026-08-28 容器实跑：200/200/302/200/302，另记录 SIGTERM 观察与 dev 代理 502 瞬态教训，见 research/linux-v1.0-baseline-run.log。）
- [x] 运行现有 `sh/package.sh`，记录 tarball 字节数、条目数与解包启动结果。
  （27,445,709 bytes / 2298 entries；规范启动命令 `-srcPath` 契约本次实跑确认，见同上日志。）
- [x] 记录升级前模块图快照（`go list -m all`、`go.mod`/`go.sum` hash）至 `research/`。
  （module-snapshot-v1.0.txt：go.mod sha1 68a37618…、go.sum sha1 907a4d32…。）

### Task 2：升级 runtime、CLI 与 modules

**Files:** `go.mod`、`go.sum`（必要时编译适配：编译错误直接指向的文件）

- [x] 把 `github.com/revel/revel` 改为 v1.1.0、`github.com/revel/cmd` 改为 v1.1.2、`github.com/revel/modules` 改为 v1.1.0；不添加 replace/exclude。
  （2026-08-28：go get 三模块到位；config v1.1.0 随 cmd 图 MVS 抬升，与预测一致。）
- [x] 运行 `go mod tidy` 两次（第二次零 diff）与 `go mod verify`；`go list -m` 确认 Revel 族单一版本集合（含 revel/config 预期抬升）；对照 PRD/design 模块图预期逐条归因，写入 `research/module-diff-log.md`。
  （tidy 双跑零 diff、verify 全部通过；归因日志含新增 gomodule/redigo/google/uuid 与 fasthttp/testify 簇抬升，A 拥有依赖零变化。）
- [x] 运行 `go build ./app/...` 与 `go vet ./app/...`（Go 1.26.7 与 1.27.0 双版本）；只对明确编译错误做最小适配，`git diff` 逐 hunk 复核无业务逻辑变化。
  （双版本 build/vet 零输出 exit 0；唯一必要适配为 tools.go 新增 `_ gomodule/redigo/redis` 钉住——生成主文件 `app/tmp/run/run.go` 硬编码空白导入 revel/cache，v1.1 cache 换用 gomodule 客户端而 tidy 剪枝导致测试服务二进制构建失败，修复沿 tools.go 既有钉住机制、未删既有钉住；app 业务代码零改动。）
- [x] A 的 `app/cmd` 契约测试（flags、generate、server fail-closed 桩）保持通过。
  （go test ./app/cmd/... ok；harness 契约测试含于双版本 replay 全绿。）

### Task 3：验证生成、开发与打包 + SIGTERM + Cookie 兼容

**Files:** `app/routes/routes.go`（生成、gitignored）、`app/tmp/`（生成、gitignored）、`research/`

- [x] 运行 `app/cmd` 生成流程，构建 `app/tmp` 二进制并经 harness 以 MongoDB 5.0 fixture 完成 Golden/USN replay（Go 1.26.7 与 1.27.0 各至少一次），Golden 文件 hash 与 Task 1 基线零差异。
  （2026-08-28：Revel v1.1 下双版本 replay 各一轮 exit 0，Golden 聚合 hash 与基线逐字节一致；首跑暴露并修复 tools.go 钉住缺口（见 Task 2）。）
- [x] 主模块图构建 v1.1.2 CLI 并以 `go version -m` 断言（revel/cmd v1.1.2、x/tools v0.49.0）；用其启动开发服务器完成页面 smoke（对照 Task 1 基线）。
  （容器实跑：断言命中；五页面 200/200/302/200/302 与 v1.0 基线逐项一致，见 research/linux-v1.1-run.log。）
- [x] 用 v1.1.2 CLI 执行 `sh/package.sh`，解包到临时目录并启动验证 `/login` 与 `/api/auth/login`；验证后工作区零残留（无 `app/tmp`、`app/routes`、`sh/leanote.tar.gz` 入库）。
  （27,453,086 bytes / 2298 entries（条目数与 v1.0 同 HEAD 基线一致）；规范 run.sh 启动后 /login 200、POST /api/auth/login 200 信封一致；全部产物在容器内，宿主工作树零残留。）
- [ ] Linux 容器内对服务进程发送 SIGTERM：不修改生产配置，在未覆盖 `app.cancel.timeout` 的环境确认 Revel effective 默认值为 60 秒；记录配置状态、实际退出耗时、退出码、端口释放与关闭日志。
- [x] Cookie 兼容取证：模块缓存中 diff v1.0.0/v1.1.0 session 序列化实现，或以 v1.0 基线签发 Cookie 在 v1.1 实测仍认证通过；证据写入 `research/`。
  （research/session-diff-v1.0-v1.1.txt：实质差异仅 uuid 库替换（同为随机 v4 hex，只影响新 ID 生成源）与注释句号；序列化/过期/Cookie 写入逻辑零变化 ⇒ v1.0 签发 Cookie 在 v1.1 仍有效，无提前强制重登。）
- [x] （诊断，可选）隔离图 `go install github.com/revel/cmd/revel@v1.1.2` 在 Go 1.26/1.27 下的结果记录入 `research/`；无论结果如何不作为验收路径。
  （Go 1.26.7 隔离图构建成功 exit 0——v1.1.2 的 x/tools v0.1.10 图不再触发 v1.0.3 时代 panic；canonical 构建策略不变，见 research/linux-v1.1-run.log。）

### Task 4：完整回归与范围复核

- [x] Go 1.26.7 与 1.27.0：`go build ./app/...`、`go vet ./app/...`、`go test ./app/tests/...`（MongoDB 5.0 replay）结果一致且全绿。
  （2026-08-28 双版本全部复验通过。）
- [x] 回放全部 Golden 与 USN 成对用例，要求零差异；`npm test` 通过且 `npm run build` 后 `git diff --exit-code`。
  （双版本 replay Golden 聚合 hash 与基线零差异；npm test 63/63；build 后零漂移。）
- [x] 对比 `conf/routes`、`conf/app.conf`、`app/init.go`、四套 controller `init.go`、`tools.go`、`sh/*.sh`、`.travis.yml`：确认无语义改动（结构基线计数见 PRD Confirmed Facts）。
  （git diff --name-only 仅 go.mod/go.sum/tools.go；tools.go 仅新增一行钉住 + 注释，业务与配置零改动。）
- [ ] C-a PR 的 regression-baseline workflow 双版本 `go-replay` 真实运行绿色（AC-Ca9 取证）。
  （**pending-push 取证**，沿用 A 的回填先例：本实现提交尚在本地 dev 未推送（发布前留给用户独立评审），
  与 A 期 08-27 push run 33041448799 / 08-28 dispatch run 33179138645 双版本 go-replay 绿同构的
  套件已在本机双版本全绿；下次 push 后以真实运行回填本项。）
- [x] `gofmt`、`git diff --check` 通过；逐项核对 AC-Ca1..AC-Ca10；diff 不含 C-b 架构代码。
  （tools.go blob gofmt 零 diff；AC 逐项核对见 implement 末尾验收清单。）

## Rollback Point

回滚 `go.mod`、`go.sum` 与必要编译适配即可恢复 A 交付的 v1.0 基线；生成文件不入库，不参与回滚。任一不可解释差异（Golden 差异、Cookie 字节不兼容、A 生成契约破坏、双版本编译失败）停止 C-a，回退并在 C-b 设计中记录，不改 Golden 强行接受。
