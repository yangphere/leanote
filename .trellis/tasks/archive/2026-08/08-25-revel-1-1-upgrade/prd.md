# Revel 1.1 基线升级（C-a）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

把 Revel 组件族从 v1.0 一代整体升级到上游最后一代 —— `github.com/revel/revel` v1.1.0、`github.com/revel/cmd` v1.1.2、`github.com/revel/modules` v1.1.0 —— 在 URL、配置、session、模板、`/api/*` 与 MongoDB Schema 零变化的前提下，为 C-b 迁出 Revel 建立稳定、可验证的框架基线。本任务是严格的版本阶段：只升级版本与必要的编译适配，不迁移架构。

## Readiness And Dependencies

- `task.json.meta.depends_on` 仅含 `08-25-go-toolchain`（A）。A 当前 `in_progress`：`record-export-pdf` 的真实 `workflow_dispatch` 取证 pending、自身 Phase 6 全量验收未勾选。**C-a 的元数据 ready 以 A 完成并归档为前提**；2026-08-28 完成的本轮需求审计只修订规格，不激活、不实现。
- 若 A 收尾引起 Go 工具链或非 Revel 依赖版本变化/回滚，本文 Confirmed Facts 与 R-Ca1 的边界必须重新核实后才可启动。
- G 的 Golden、USN 与页面 smoke 必须可运行；本任务不得通过更新基线解释框架升级差异。
- 本任务的数据库验证固定使用 G/A 的 **MongoDB 5.0** fixture；MongoDB 7.0/8.0 验收属于 B，不在本任务出现，也不得宣称。

## Confirmed Facts

上游与模块图事实（2026-08-28 经 Go module proxy 与本仓库复核）：

- `github.com/revel/revel` 上游止于 **v1.1.0**（release note：修复 log 递归调用、支持 SIGTERM 优雅关停）；`github.com/revel/cmd` 上游 **retract 了 v1.1.0 与 v1.1.1**（failed releases），1.1 代唯一有效版本为 **v1.1.2**；`github.com/revel/modules` 存在 v1.1.0。
- `revel/cmd` v1.1.2 的 go.mod（`go 1.17`）钉住 `golang.org/x/tools v0.1.10`、`go-flags v1.4.0`。隔离模块图 `go install github.com/revel/cmd/revel@v1.1.2` 将沿用上游旧图；A 已实证 v1.0.3 隔离图（x/tools v0.0.0-20200219）在 Go ≥1.26 下 type-check 必然 panic（`08-25-go-toolchain/research/linux-stock-cli-panic-go1.26.7.log`）。v1.1.2 隔离图行为未实测，**不得作为本任务的验收路径**。
- 当前 go.mod：revel v1.0.0 / cmd v1.0.3 / modules v1.0.0、`go 1.26`。A 交付并拥有的非 Revel 直接依赖（`golang.org/x/tools v0.49.0`、`go-flags v1.6.1`、`x/crypto v0.55.0`、`goquery v1.12.0`、`robfig/config` 保持、`gocolorize` 保持）**C-a 不得改动**；Revel 图经 MVS 只允许向上覆盖，且必须逐条归因。
- `app/cmd` 不是自包含副本：`app/cmd/build.go`、`app/cmd/revel.go`、`app/cmd/harness/app.go` 直接 import `github.com/revel/cmd/{model,utils,logger}`，A 的 `app/cmd/flags_contract_test.go` import `cmd/model(+command)`。cmd 升版会直接流入本地生成器的编译与上游 `cmd/model` 内置生成模板，生成的 `app/tmp` 形状可能随版本变化，必须由 Golden/harness 承接验证。
- 生成链证据：`app/tmp/run/run.go` import `github.com/revel/modules/static/app/controllers`（来自上游模板）；根目录 `tools.go`（build tag `tools`，默认不编译）为 `conf/app.conf` 的 `module.static` 运行时解析钉住 `revel/modules/static`、`bradfitz/gomemcache`、`garyburd/redigo`、`patrickmn/go-cache`。
- Revel v1.1.0 runtime 的 go.mod 改用 `gomodule/redigo v1.8.8`、新增 `google/uuid v1.3.0`，并抬升 fsnotify/go-colorable/go-stack/go-isatty 与 gomemcache 伪版本；其 `x/net` 旧要求被主模块已有 v0.58.0 覆盖。升级后 `garyburd/redigo`（tools.go 钉住）与 `gomodule/redigo` 将共存于模块图；精确增删清单以实现期 tidy diff 为准并逐条归因。
- `.travis.yml` 在 A 已对齐（`go 1.26.x`、主模块图构建 CLI、`go version -m` 断言 x/tools v0.49.0）且属非阻断遗留入口（最终删除/替换由 F 负责）。C-a 不修改该文件；cmd 升版后同一构建自动产出 v1.1.2 CLI，x/tools 断言不受影响（版本不变）。
- `.github/workflows/regression-baseline.yml` 的 `go-replay` 已是固定 1.26.7/1.27.0 双版本阻断矩阵（A 交付）；C-a 的 PR 必须保持其绿色，作为框架升级后的真实 CI 取证通道。
- 代码结构基线（2026-08-28 复核，R-Ca2 的对照基准）：四套 `commonUrl` 白名单（`app/controllers`、`app/controllers/admin`、`app/controllers/api`、`app/controllers/member` 各 `init.go`）；25 处激活的 `revel.InterceptFunc(AuthInterceptor, revel.BEFORE, ...)` 注册（controllers 8、api 6、member 4、admin 7），另有 2 处历史注释行原样存在；27 个唯一 `revel.TemplateFuncs` 名对应 27 处激活赋值（`blogTags` 与 `gt` 各另有 1 处位于块注释内的死代码，不得因此计数）；5 处 `revel.OnAppStart`（`app/init.go:421`、`app/controllers/init.go:155`、`app/controllers/member/init.go:133`、`app/lea/i18n/i18n.go:175`、`app/service/ConfigService.go:472`）。
- `conf/app.conf` 存在 `[dev]`/`[prod]`/`[test]` section；`sh/package.sh` 的 `--run-mode=prod` 与 test-mode harness 的 run mode 解析（Revel 在 app.conf 无同名 section 时 Fatal）均有既存支撑。

## Requirements

### R-Ca1 版本三件套与模块边界

- 精确固定 `github.com/revel/revel v1.1.0`、`github.com/revel/cmd v1.1.2`、`github.com/revel/modules v1.1.0`，形成单一 v1.1 代集合；不得混用 v1.0/v1.1 代（如 runtime v1.1.0 + modules v1.0.0）。若实现期证明某组件无法在 v1.1 代编译且无最小适配解，停止并回到规划，不得静默选旧版。
- Revel 族间接依赖的预期终态：`revel/config` 因 cmd v1.1.2 图经 MVS 抬升到 v1.1.0；`revel/log15 v2.11.20+incompatible` 与 `revel/pathtree`（既有伪版本）不变；实际以 tidy 后 `go list -m` 为准记录。
- 非 Revel 直接依赖版本零变化（A 拥有）；`tools.go` 的 build tag 与既有四个钉住（gomemcache、garyburd/redigo、go-cache、revel-modules/static）不得删除。允许且仅允许一类改动：**生成链必要适配的新增钉住**——生成主文件 `app/tmp/run/run.go` 硬编码空白导入 `revel/revel/cache`，v1.1 的 cache 实现改用 `gomodule/redigo`，该直接依赖必须经 tools.go 钉住才能在 tidy 剪枝下存活（C-a 已实际新增此一行，归类为 Revel 图换代的生成链直接依赖，见 research/module-diff-log.md）。新增钉住以外的任何 tools.go 改动回到规划。新增仅被 Revel 图引入、无业务调用的模块必须以 `// indirect` 呈现并归因到具体 Revel 组件。
- 不使用 `replace`/`exclude`/vendor 改写 Revel 图；任何额外版本变化必须先回到规划。

### R-Ca2 行为不变量（对照 Confirmed Facts 的结构基线）

- `conf/routes`、`conf/app.conf`、`RouterFilter`、四套 `commonUrl` 白名单、25 处激活鉴权 interceptor 注册（含 2 处注释行原样保留）、27 个唯一 TemplateFuncs、5 处 `OnAppStart` 的注册内容与相对顺序保持不变；允许且仅允许编译错误直接指向的最小适配。
- 启动顺序仍为 `db.Init` → 邮件/验证初始化 → services → controller service aliases → global config → API service；`results.pretty=false` 的 test 模式继续作为字节级契约环境。
- `/api/*`、USN、认证、所有权查询、模板输出、静态资源与错误语义保持；Golden/USN 零差异，不刷新基线。

### R-Ca3 CLI 构建策略与三条执行路径

- 官方 CLI 的 canonical 构建方式是**主模块图构建**：`go build -o <path> github.com/revel/cmd/revel`（与 A 的 `.travis.yml` 及 Linux smoke 同法），构建后以 `go version -m` 断言二进制携带 `github.com/revel/cmd v1.1.2` 与 `golang.org/x/tools v0.49.0`。
- 隔离图 `go install ...@v1.1.2` 最多作为实现期诊断记录（结果写入 `research/`），不得作为任何验收路径；若发现主模块图构建不可用而隔离图可用，停止并回到规划。
- 三条路径全部用新版本验证并通过：

  | 路径 | 入口 | 验证 |
  |---|---|---|
  | 测试二进制 | `app/cmd` 生成器（A 契约：默认 go ≥1.26.7 fail-closed、`GOTOOLCHAIN=local` 强制）+ `go build .../app/tmp` | G harness 的 MongoDB 5.0 Golden/USN replay，Go 1.26.7 与 1.27.0 双版本 |
  | 开发 | `revel run -a .` / `sh/run.sh`（PATH 中模块图构建的 v1.1.2 CLI） | 真实 HTTP smoke：`/`、`/login`、`/note`、`/blog`、`/demo` |
  | 生产包 | `revel package --run-mode=prod` / `sh/package.sh` | tarball 解包、启动、`/login` 与 `/api/auth/login` |

- `sh/run.sh`、`sh/package.sh` 内容零改动（它们只调用 PATH 中的 `revel`）；若必须改动即回到规划。

### R-Ca4 SIGTERM 优雅关停

- v1.1.0 的 SIGTERM 优雅关停属本任务入口验收（A 已显式移交：A 仅改 signal channel 容量，不新增 SIGTERM 语义）。Revel 在未覆盖配置时通过 `Config.IntDefault("app.cancel.timeout", 60)` 使用 60 秒默认超时；在不修改生产配置的受控 Linux 环境（与 A Phase 5 同类容器化环境）启动开发或测试路径进程，确认端口监听后发送 SIGTERM，断言进程在 effective `app.cancel.timeout` 内退出、监听端口释放、退出码与关闭日志符合上游实现。记录配置是否覆盖、effective 超时（默认应为 60 秒）、实际退出耗时、退出码、端口释放和关闭日志。Windows 不定义 SIGTERM 语义，不作为该项验收环境。

### R-Ca5 范围纪律

- 本任务只做版本升级与必要的编译适配；不引入 C-b 的 `net/http` 代码、显式路由或 session 重写。
- `app/cmd` 不与上游 v1.1.2 重新同步（不重植上游模板/命令集）；只允许编译错误直接指向的最小适配，且 A 的生成契约（`minGeneratorVersion=1.26.7` fail-closed、`GOTOOLCHAIN=local` 注入、桩测试锁定）保持绿。
- 不迁移 MongoDB 驱动、不跑 7.0/8.0 假验证、不改 `.travis.yml` 与 workflow 语义、不动 Cookie 安全默认值（HttpOnly/SameSite/Secure 收紧属 C-b）。

### R-Ca6 会话 Cookie 字节兼容（新增）

- C-a 仍由 Revel 处理 session；父任务的"一次性重新登录"只授权给 C-b。必须证明 v1.0 基线签发的会话 Cookie 在 v1.1 下仍被接受，或以模块缓存中 v1.0.0 与 v1.1.0 session 序列化源码 diff 证明格式零变化，并记录证据。
- 若实证 v1.1.0 改变 Cookie 字节格式：不得静默接受提前强制重登，停止升级并回到规划，由用户裁决。

### R-Ca7 模块 diff 归因与 tidy 纪律（新增）

- `go mod tidy` 连续两次零 diff、`go mod verify` 通过；go.mod/go.sum 的每一行变化必须归因到具体来源（Revel 组件图要求 / MVS 抬升 / tidy 重排），写入 `research/module-diff-log.md`；无未知模块、无第二套框架/日志/配置实现、无法归因即停止。

## Acceptance Criteria

- [ ] **AC-Ca1** `go list -m` 显示 revel v1.1.0、cmd v1.1.2、modules v1.1.0 的单一版本集合，无 v1.0 代残留；Revel 族间接依赖终态与模块 diff 日志一致；非 Revel 直接依赖零变化；`tools.go` 既有四个钉住未删，唯一改动为 R-Ca1 允许的生成链必要适配（新增 gomodule/redigo 钉住）。
- [ ] **AC-Ca2** Go 1.26.7 与 1.27.0 双版本：`go build ./app/...`、`go vet ./app/...` 零输出 exit 0；`go test ./app/tests/...`（MongoDB 5.0 fixture replay）目标测试数非零、Golden/USN 全绿且 Golden 文件 hash 零变化。
- [ ] **AC-Ca3** `app/cmd` 生成 `app/routes/routes.go` 与 `app/tmp/main.go` 后二进制可构建并经 harness 启动；A 的 flags/generate/server 契约测试保持通过；生成文件不入库、工作区零残留。
- [ ] **AC-Ca4** 主模块图构建的 v1.1.2 CLI（`go version -m` 断言通过）完成 `revel run -a .` 真实页面 smoke 与 `sh/package.sh` 生产 tarball（解包、启动、`/login`、`/api/auth/login`）验证；`sh/*.sh` 未改动。
- [ ] **AC-Ca5** Linux 容器内 SIGTERM 优雅关停取证通过；未覆盖 `app.cancel.timeout` 时 effective 默认值为 60 秒，并已记录配置状态、实际退出耗时、退出码、端口释放与关闭日志。
- [ ] **AC-Ca6** 会话 Cookie 兼容证据已记录（源码 diff 或 v1.0 Cookie 实测），无提前强制重登。
- [ ] **AC-Ca7** `go mod tidy` 连续两次零 diff、`go mod verify` 通过；模块 diff 逐条归因，无 replace/exclude、无未知模块、无静默降级。
- [ ] **AC-Ca8** 结构基线复核：`conf/routes`、`conf/app.conf`、`app/init.go`、四套 controller `init.go` 的 diff 仅含编译必需适配、无语义改动；`npm test` 通过且 `npm run build` 后 `git diff --exit-code`（前端生成物不受后端升级影响）。
- [ ] **AC-Ca9** C-a 的 PR/push 上 regression-baseline workflow 双版本 `go-replay` 真实运行绿色；`gofmt`、`git diff --check` 通过。
- [ ] **AC-Ca10** diff 不包含路由重写、controller 架构变化、session 格式重写、模板行为重写、Cookie 安全默认值变化或 C-b 架构代码。

## 待确认事项（启动前必须消除）

1. **A 的收尾状态**：`record-export-pdf` 真实 `workflow_dispatch` 取证与 A Phase 6 验收未完成，A 未归档——本任务不得在其之前激活。影响：当前为"规格就绪、启动被 A 阻塞"状态；若 A 收尾改变工具链/依赖事实，本文档需重核。
2. **SIGTERM 验收证据**：上游实现通过 `Config.IntDefault("app.cancel.timeout", 60)` 将未覆盖配置的默认超时固定为 60 秒；实现期仍须在受控 Linux 环境记录 effective 值、实际退出耗时、退出码、端口释放与关闭日志，不修改生产配置。影响：缺少这些运行证据时 AC-Ca5 不可判定。
3. **Cookie 兼容性实证结果**：规格已钉"不得提前强制重登"，但 v1.1.0 是否改变字节格式未知。若实测不兼容，需用户在"回退 v1.0"与"接受一次性重登提前"之间裁决（父任务语义倾向前者）。
4. **`app/cmd` 与 cmd v1.1.2 的编译兼容度**：上游 model/utils/logger 重构幅度未知；若最小适配无法编译或 A 生成契约被破坏，按回滚条款回退并记录到 C-b 设计，不得深改 `app/cmd`。

## Out of Scope

- 迁出 Revel、显式化全部路由或删除 `app/cmd`；`app/cmd` 与上游 v1.1.2 模板/命令集的重新同步。
- MongoDB 驱动迁移与 MongoDB 7.0/8.0 验证。
- Cookie 安全策略变化（HttpOnly/SameSite/Secure 收紧只在 C-b 新会话实现中发生）。
- 非 Revel 依赖升级、`tools.go` 清理、`.travis.yml` 删除/重构（属 F）、workflow 语义修改。
- 修复 Golden 已钉住的历史业务缺陷；发现后单独登记，不在本任务顺手修改。
