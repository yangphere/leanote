# Revel 1.1 基线升级（C-a）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

把 Revel runtime 从 v1.0.0 升到上游最后版本 v1.1.0、Revel CLI 从 v1.0.3 升到 v1.1.2，在不改变 URL、配置、session、模板和 API 行为的前提下建立迁出 Revel 前的稳定基线。

## Dependencies

- 依赖 `08-25-go-toolchain` 完成。
- G 的 Golden、USN 与 smoke 必须可运行；本任务不得通过更新基线解释框架升级差异。

## Requirements

- **R-Ca1** `github.com/revel/revel` 固定 v1.1.0，`github.com/revel/cmd` 固定 v1.1.2，相关 `revel/modules` 等依赖取与其兼容的明确版本。
- **R-Ca2** `conf/routes`、`conf/app.conf`、`RouterFilter`、四套 `commonUrl` 白名单、27 处鉴权 interceptor、31 个 TemplateFuncs 与 `OnAppStart` 顺序保持不变。
- **R-Ca3** `app/cmd` 的本地生成流程、官方 `revel` CLI 开发运行与生产打包均能使用新版本。
- **R-Ca4** SIGTERM 能触发 v1.1 的优雅关停路径，进程在限定时间内退出且不留下监听端口。
- **R-Ca5** 本任务只做版本升级与必要的编译适配，不引入 C-b 的 `net/http` 代码。

## Acceptance Criteria

- [ ] `go list -m all` 显示 Revel v1.1.0、cmd v1.1.2，且无旧版本残留。
- [ ] `go build ./app/...`、`go vet ./app/...`、`go test ./app/tests/...` 全部通过。
- [ ] `app/cmd` 生成 `app/routes/routes.go` 与 `app/tmp/main.go` 后可构建并启动测试服务器。
- [ ] `revel run -a .` 能启动开发服务器；`revel package --run-mode=prod` 能生成 tarball。
- [ ] 全量 Golden、USN 与页面 smoke 无差异。
- [ ] SIGTERM 测试证明服务优雅退出。
- [ ] diff 不包含路由、controller 架构、session 格式或模板行为重写。

## Out of Scope

- 迁出 Revel、显式化全部路由或删除 `app/cmd`。
- MongoDB 驱动迁移。
- Cookie 安全策略变化；该变化只在 C-b 新会话实现中发生。
