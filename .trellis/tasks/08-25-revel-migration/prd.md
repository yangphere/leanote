# 迁出 Revel 到标准库 HTTP（C-b）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在 MongoDB 驱动迁移稳定后，把 Leanote 从停滞的 Revel 框架迁到 Go 1.26 标准库 `net/http`/`ServeMux`，保留服务端渲染、URL、API、鉴权、配置、模板、博客主题和静态资源行为，并删除 Revel CLI 生成链与 `app/cmd`。

## Dependencies

- 依赖 `08-25-mongo-driver-migration` 完成；不得与 B 并行修改后端。
- G 的 Golden、USN、所有权与页面 smoke 是合并门禁。

## Requirements

### R-Cb1 HTTP 与路由

- 使用 `net/http` 与 `ServeMux`，不引入 Gin、Echo、chi 或第二套路由框架。
- `conf/routes` 的显式路由保持优先；历史 `/:controller/:action`、`/api/:controller/:action`、`/member/:controller/:action` 由静态 controller/action 注册表分派。
- 分派器只调用显式注册的 controller/action，禁止按任意输入反射调用导出方法。
- 参数绑定、上传、JSONP、文本、JSON、二进制、文件下载、附件头和模板结果保持兼容。

### R-Cb2 中间件与启动

- 等价重建静态资源、Mongo 健康检查、i18n、panic/错误、鉴权和日志中间件；错误必须显式返回并记录。
- `OnAppStart` 的初始化顺序保持不变，并提供受超时约束的优雅关停。
- 新入口为普通 Go main；`go build` 不再先生成 `routes.go`/`main.go`，`sh/run.sh` 不再依赖 Revel CLI。

### R-Cb3 Session 与安全

- 接受部署后 Web 用户重新登录一次，不读取 Revel Cookie 格式；API token 与 MongoDB session 记录保持有效。
- 继续支持 `cookie.prefix/domain/secure` 与 `session.expires` 配置。
- 新 Cookie 默认 `HttpOnly=true`、`SameSite=Lax`；`Secure` 由配置控制。
- prod 启动时若 `app.secret` 为空或仍是仓库公开默认值，必须明确失败并给出修复信息。

### R-Cb4 配置、模板和博客

- `conf/app.conf` 文件格式、run-mode section、插值与现有键保持兼容。
- 31 个 TemplateFuncs、Revel 模板目录解析、`note-dev.html`/`note.html` 使用方式保持不变。
- `app/lea/blog/Template.go` 的内置主题和用户上传主题继续使用 `html/template`，Preview 错误路径保持可用。

### R-Cb5 移除 Revel

- 生产代码、`go.mod`、脚本不再依赖 `github.com/revel/*`。
- 删除 `app/cmd/` 与只为 Revel 生成存在的代码；保留仍被业务使用的第一方工具。

## Acceptance Criteria

- [ ] `go build` 直接生成可运行服务器，无 `routes.go`/`main.go` 预生成步骤。
- [ ] `rg 'github.com/revel|revel\.' app go.mod sh conf` 无生产依赖命中，`app/cmd/` 已删除。
- [ ] `conf/routes` 所有显式路由及三类 catch-all 的正/负路由测试通过；未注册 action 返回 404，不可调用任意方法。
- [ ] 全量 Golden、USN、所有权和页面 smoke 通过。
- [ ] Web 旧 Cookie 被拒绝并可重新登录；API token 在升级前后保持有效。
- [ ] prod 对空/默认 secret 启动失败；Cookie 属性测试覆盖 HttpOnly、SameSite、Secure 配置和 3 小时过期。
- [ ] 内置三主题与一个上传主题渲染正常，Preview 显示可定位模板错误。
- [ ] SIGTERM 在限定时间内停止接收、完成进行中请求并退出。
- [ ] `sh/run.sh` 与 `sh/package.sh` 不再调用 Revel CLI。

## Out of Scope

- SPA、前后端分离、模板引擎替换或 URL 重设计。
- 兼容旧 Revel Cookie 编码。
- 把所有 service 改为显式 `context.Context`；见 MOD-001。
- MongoDB/前端库升级或新业务功能。
