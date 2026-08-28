# Revel 1.1 基线升级（C-a）— 技术设计

## 1. 策略

这是严格的依赖版本阶段：不建立兼容 shim，不改变 controller API。升级对象是 v1.1 代三件套（runtime v1.1.0、cmd v1.1.2、modules v1.1.0），所有必要适配限制在编译错误直接指向的位置。

与 A 的关键差异：`app/cmd` 直接 import `github.com/revel/cmd` 的 `model`/`utils`/`logger`，且生成模板来自上游 `cmd/model`——cmd 升版会流入本地生成器的编译与生成 `app/tmp` 的形状。因此"编译错误驱动最小适配 + Golden 承接行为验证"是本任务的核心控制回路；任何超出编译指向的重构都意味着回到规划。

## 2. 三条运行路径

| 路径 | 入口 | 验证 |
|---|---|---|
| 测试二进制 | `app/cmd` 生成器（go ≥1.26.7 fail-closed、`GOTOOLCHAIN=local`）+ `go build github.com/yangphere/leanote/app/tmp` | G 的 MongoDB 5.0 HTTP harness 与 Golden/USN，Go 1.26.7/1.27.0 双版本 |
| 开发 | `revel run -a .` / `sh/run.sh`（PATH 中主模块图构建的 v1.1.2 CLI） | 隔离端口真实请求 smoke（`/`、`/login`、`/note`、`/blog`、`/demo`） |
| 生产包 | `revel package --run-mode=prod` / `sh/package.sh` | 解包、启动、访问 `/login` 与 `/api/auth/login` |

CLI provenance：canonical 构建为主模块图 `go build -o ... github.com/revel/cmd/revel`，`go version -m` 断言 revel/cmd v1.1.2 + x/tools v0.49.0。隔离图 `go install @v1.1.2`（上游图 x/tools v0.1.10）存在 A 实证的同类 panic 风险（v1.0.3 图 + x/tools v0.0.0-20200219 在 Go ≥1.26 type-check 必然 panic），只许作诊断记录，不许作验收路径。

三条路径必须同时通过，避免只证明 runtime 可编译却遗漏生成器或打包器。

## 3. 模块图预期与边界

- 目标终态（MVS 推导，实现期以 tidy diff 验证）：revel v1.1.0、cmd v1.1.2、modules v1.1.0、revel/config v1.1.0（由 cmd v1.1.2 图抬升）、revel/log15 v2.11.20+incompatible 与 revel/pathtree 不变；`gomodule/redigo`、`google/uuid` 进入；fsnotify/go-colorable/go-stack/go-isatty/gomemcache 抬升；`garyburd/redigo`、`go-cache`、`gomemcache` 因 `tools.go` 钉住而保留；`x/net` 被主模块已有 v0.58.0 覆盖。
- `tools.go`（build tag `tools`）机制不变：`conf/app.conf` 的 `module.static` 依赖它把运行时解析的模块留在图内。
- 非 Revel 直接依赖（x/tools、go-flags、x/crypto、goquery、robfig/config、gocolorize）版本零变化；`x/tools v0.49.0` 由主模块 require 钉住，Revel 图的更低要求不构成变化。
- 归因规则：go.mod/go.sum 的每一行变化必须指向具体来源（Revel 组件图要求 / MVS 抬升 / tidy 重排），写入 `research/module-diff-log.md`；无法归因即停止。

## 4. 不变量

- 启动顺序仍为 `db.Init` → 邮件/验证初始化 → services → controller service aliases → global config → API service。
- `conf/routes` 优先级、catch-all 与 `RouterFilter` 改写保持原样；四套 `commonUrl`、25 处激活 interceptor 注册、27 个唯一 TemplateFuncs、5 处 OnAppStart 的注册内容与顺序不变（结构基线与验证计数见 PRD Confirmed Facts）。
- Session Cookie 仍由 Revel 处理且字节格式保持（v1.0 签发的 Cookie 在 v1.1 仍被接受）；一次性重新登录决策只适用于 C-b。验证方法：模块缓存中 v1.0.0 vs v1.1.0 session 序列化源码 diff，或保留 v1.0 基线 Cookie 实测认证仍通过。
- `results.pretty=false` 的测试模式继续作为字节级契约环境。

## 5. SIGTERM 优雅关停

v1.1.0 release note 声明支持 SIGTERM 优雅关停（A 已把该验收显式移交给本任务）。Revel 在未覆盖配置时通过 `Config.IntDefault("app.cancel.timeout", 60)` 使用 60 秒默认超时。验证在不修改生产配置的受控 Linux 容器（与 A Phase 5 同类环境）进行：启动开发或测试路径进程 → 确认端口已监听 → 发送 SIGTERM → 断言进程在 effective `app.cancel.timeout` 内退出、端口释放、退出码与关闭日志符合上游实现；记录配置是否覆盖、effective 超时（默认应为 60 秒）、实际退出耗时、退出码、端口释放与关闭日志。Windows 不参与本项验证。

## 6. 回滚

该阶段形成单一可回滚版本提交：回退 `go.mod`、`go.sum` 与必要编译适配即恢复 A 交付的 v1.0 基线；生成文件不入库，不参与回滚。若 v1.1 产生无法用必要兼容适配解决的行为变化（含 Cookie 字节格式变化、A 生成契约破坏、双版本编译失败），回退到 v1.0 并在 C-b 设计中记录，不通过改 Golden 强行接受。
