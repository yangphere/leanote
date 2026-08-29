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

- 等价重建 Revel filter 链（app/init.go 现序）：panic 恢复、URL 前缀改写（`lea/route` RouterFilter，必须保持链首业务位）、静态资源、参数解析、session、i18n（`lea/i18n` I18nFilter）、interceptor（AuthInterceptor 等 BEFORE 钩子，commonUrl 白名单随其迁移）、gzip 压缩（CompressFilter 等价，Accept-Encoding 协商行为属可观察契约）与 action 分派。
- 静态资源交付收编 module.static 默认行为与显式路由：`/public`、`/js`、`/images`、`/img`、`/css`、`/fonts`、`/tinymce`、`/upload` 的路径解析与 Content-Type 断言入测试。
- FlashFilter 与 ValidationFilter **显式删除**：生产代码零业务使用（`revel.Validation` 仅被 app/cmd 的 AST 分析器引用，随 app/cmd 一并删除），删除属规格化决策而非遗漏。
- 错误页等价：`errors/404.html`、`500.html`、`500-blog.html` 及 dev 变体（`*-dev.html`）按 run-mode 与博客域选择性渲染；panic 记录堆栈返回 500。
- `OnAppStart` 共 5 处（app/init.go、controllers/init.go、member/init.go、lea/i18n、service/ConfigService）按原注册顺序执行，并提供受超时约束的优雅关停（`http.shutdownTimeoutMs`，默认 30000）。
- 新入口为普通 Go main；`go build` 不再先生成 `routes.go`/`main.go`，`sh/run.sh` 不再依赖 Revel CLI。

### R-Cb3 Session 与安全

- 接受部署后 Web 用户重新登录一次，不读取 Revel Cookie 格式；API token 与 MongoDB session 记录保持有效。
- 继续支持 `cookie.prefix/domain/secure` 与 `session.expires` 配置。
- 新 Cookie 默认 `HttpOnly=true`、`SameSite=Lax`；`Secure` 由配置控制。
- prod 启动时若 `app.secret` 为空或仍是仓库公开默认值，必须明确失败并给出修复信息。
- 项目当前无 CSRF 机制（`rg -i csrf app conf` 零命中）；迁移不引入也不删除任何 CSRF 行为。

### R-Cb4 配置、模板和博客

- `conf/app.conf` 文件格式、run-mode section（[dev]/[prod]/[test]）、插值与现有键保持兼容。
- `app/init.go` 现有 **27 个活跃** TemplateFuncs（29 处注册语句中 2 处块注释弃用）逐个移植，名集与行为以契约测试冻结；含依赖 `revel.Message`/locale 的 msg/leaMsg/blogTags 系列；`messages/<locale>` i18n 查找与 `CurrentLocale` view-arg 键保持等价。
- Revel 模板目录解析、`note-dev.html`/`note.html` 使用方式保持不变。
- `app/lea/blog/Template.go` 的内置主题和用户上传主题继续使用 `html/template`，Preview 错误路径保持可用。

### R-Cb5 移除 Revel

- 先建第一方 seam 并迁移调用方，再删依赖：**config seam**（`revel.Config` 34 处——app/db 8 处（db.url/host/port/dbname/连接与操作超时等，B 的验收键）、service/ConfigService 13、controllers（Note 4、ApiNote 2 等）、lea（i18n 3、Email/blog/html2image 各 1）、admin 1）；**logger seam**（`lea/Debug` 是全站日志门面，直连 `revel.AppLog`）；**BasePath seam**（`revel.BasePath` 45 处：ThemeService 8、FileService 5、html2image 4 处文本（1 注释）、其余为 controllers/service 的文件系统与 URL 路径拼接）。seam 行为与 Revel 现值一致（BasePath 默认应用根，键名与默认值见 design）。
- 生产代码、`go.mod`、脚本不再依赖 `github.com/revel/*`；`rg 'github.com/revel|revel\.' app go.mod sh conf` 零命中。
- 删除 `app/cmd/`（vendored Revel CLI fork：revel.go、build.go、harness/、parser2/、gen_tmp.sh、flags_contract_test.go；已核实无 app/cmd 之外的 import 方）与只为 Revel 生成存在的代码。

## Acceptance Criteria

- [ ] `go build` 直接生成可运行服务器，无 `routes.go`/`main.go` 预生成步骤。
- [ ] `rg 'github.com/revel|revel\.' app go.mod sh conf` 零命中，`app/cmd/` 已删除；app/db 与 lea 的 config/logger/BasePath seam 生效（B 的连接/超时配置键与日志输出不变）。
- [ ] `conf/routes` 所有显式路由（含 `*` 方法路由与 `/_test/e2e/identity`）及三类 catch-all 的正/负路由测试通过；未注册 action 返回 404，不可调用任意方法。
- [ ] 全量 Golden、USN、所有权和页面 smoke 通过——且这些门禁运行在**移植后的 harness** 上：`app/tests/harness/server.go` 以新入口构建并启动服务器，CI node-tests job 不再构建 Revel CLI。
- [ ] Web 旧 Cookie 被拒绝并可重新登录；API token 在升级前后保持有效。
- [ ] prod 对空/默认 secret 启动失败；Cookie 属性测试覆盖 HttpOnly、SameSite、Secure 配置和 3 小时过期。
- [ ] 内置三主题与一个上传主题渲染正常，Preview 显示可定位模板错误；27 个活跃 TemplateFuncs 的名集与行为以契约测试冻结。
- [ ] 静态资源八个前缀（含 `/upload`）的路径解析、Content-Type 断言通过。
- [ ] SIGTERM 触发优雅关停：停止接收新请求、进行中请求在 `http.shutdownTimeoutMs`（默认 30000）内完成并退出、端口释放（测试以超时上界断言）。
- [ ] `sh/run.sh` 与 `sh/package.sh` 不再调用 Revel CLI；仓库文档（AGENTS.md/CLAUDE.md）的构建说明同步。

## Out of Scope

- SPA、前后端分离、模板引擎替换或 URL 重设计。
- 兼容旧 Revel Cookie 编码。
- 把所有 service 改为显式 `context.Context`；见 MOD-001。
- MongoDB/前端库升级或新业务功能。
