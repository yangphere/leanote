# 迁出 Revel 到标准库 HTTP（C-b）— 执行计划

## Global Constraints

- 仅用 Go 1.26 标准库 HTTP 栈；不引入第二套路由框架。
- URL、API、USN、配置文件和模板主题保持兼容。
- 旧 Web Cookie 不兼容；API token 必须兼容。

### Task 1：先写框架边界契约测试

**Files:**
- Create: `app/httpserver/config_test.go`、`session_test.go`、`session_codec_test.go`、`response_test.go`、`server_test.go`、`shutdown_test.go`（routes_test.go 随 Task 3 的路由实现补写）
- Reuse: G 的 HTTP Golden、smoke 与双用户权限用例

- [ ] 写 app.conf 测试覆盖 run-mode section、插值、bool、duration 和缺失必需配置。
- [ ] 写 session 测试覆盖新 Cookie、旧/损坏 Cookie 拒绝、HttpOnly、SameSite、Secure 与过期。
- [ ] 写 response 测试覆盖 JSON/JSONP/text/template/binary/file/attachment，先确认新包缺失导致测试失败。

### Task 2：建立配置、结果和服务器骨架

**Files:**
- Create: `app/httpserver/config.go`、`response.go`、`server.go`
- Create: `cmd/leanote/main.go`
- Modify: `app/init.go`、`app/db/Mgo.go`、`app/db/mongo_client.go`、`app/lea/Debug.go`

- [ ] 实现配置兼容读取（section/插值/类型）和 prod secret 验证；测试密钥只存在 test 配置。
- [ ] 真实 `conf/app.conf` 与 `conf/app.conf-default` 在 dev/prod/test 三种 run-mode 下全量解析成功并抽查关键键（http.port、db.dbname、app.secret、%(app.name)s 插值）——解析器为 fatal 语义，真实文件回归必须被测试先抓住。
- [ ] 建立 config/logger/BasePath seam 并迁移 app/db 与 lea/Debug 的 `revel.Config`/`revel.AppLog` 调用（B 的配置键与日志行为不变，db 测试守门）。
  - 进度注记（2026-08-29 审核）：本项按 design §1.2 以**容忍式 seam** 交付（lea/Debug nil 回退、app/db `revel.Config == nil` 分支、cmd/leanote 自持 URL 推导）；调用方全量迁移顺延至 Task 4 批次就地迁移 + Task 6 清扫，不视为本项未完成的欠账。
- [ ] 实现第一方 Result 与统一 writer，保证错误不被写成 200。
- [ ] 按原 `OnAppStart` 顺序（5 处注册点）组装依赖，增加受限 Shutdown（`http.shutdownTimeoutMs`，默认 30000）。
  - 进度注记：cmd/leanote 已接 service→controllers→api 三条 InitService 链、SIGTERM 关停，以及模板/i18n 装配（`TemplateSetRenderer(LoadTemplates(...))`、`i18n.LoadMessages(...)`、`i18n.DefaultLanguage`）。模板执行回归覆盖了按相对路径命名的模板集。
- [ ] 运行 config/response/server 测试直到通过。（已达成：53 测试 -race 全绿）

### Task 3：路由、参数与中间件

**Files:**
- Create: `app/httpserver/routes.go`、`registry.go`、`request.go`、`middleware.go`、`routes_test.go`
- Modify: `conf/routes`（只在需要标明兼容解析规则时修改，不改 URL）
- Read/Port: `app/lea/route/Route.go`（RouterFilter 前缀改写）、`app/lea/binder/binder.go`（MSSBinder/leanoteStructBinder/ObjectID binder）、`app/lea/i18n/`、四个 controller 包的 `init.go`（BEFORE 拦截器清单）

- [ ] 按 conf/routes 顺序注册显式路由（含 `*` 方法路由与 `/_test/e2e/identity`）和静态路径。
- [ ] 写路由表测试覆盖显式优先、三类 catch-all、静态文件、路径参数、未注册 controller/action 404。
- [ ] 建立静态 action 注册表，声明 HTTP 方法、绑定参数、controller 构造器与 BEFORE 钩子（活跃 25 处 InterceptFunc 逐条对照）。
- [ ] 移植前缀改写、鉴权（commonUrl 白名单随 AuthInterceptor）、i18n、恢复、gzip（CompressFilter 等价）与日志链。
  - 进度注记（2026-08-30 审核 reconcile）：路由表/registry/catch-all/负例与 Recover/Gzip/LoginRequired 骨架已交付（routes_test.go 存在，api 批次经其跑通）；**本任务仍未关闭**——余量为参数绑定等价物（MSSBinder/leanoteStructBinder/[]键归并，详见 §2.2 线格式）与 i18n 中间件接线，两项随"主站批次前置框架项"落地后本任务方可勾完。
- [ ] 运行路由负例，证明任意导出方法无法通过 URL 调用。

### Task 4：Session、BaseController 与 Render API

**Files:**
- Create: `app/httpserver/session.go`、`templates.go`
- Modify: `app/controllers/BaseController.go`、`app/controllers/api/ApiBaseController.go`
- Modify: controller action files under `app/controllers/`、`app/controllers/api/`、`app/controllers/admin/`、`app/controllers/member/`

- [ ] 实现新 Cookie session，保留业务键（UserId/Email/Username/UsernameRaw/Verified/Theme/themeId/NotebookWidth/NoteListWidth/LeftIsMin/Logo）和配置项，不实现 Revel decoder。
- [ ] 把 Session/Params/ViewArgs/Message/Render* 收敛到第一方 BaseController/Result；Render* 清单含 RenderTemplate/RenderJSON/**RenderJSONP**（BlogController 8 处）/RenderText/binary/file/attachment。
- [ ] 保持 API BaseController（`ApiBaseContrller`，按值嵌入 `controllers.BaseController`）行为并添加专门回归测试。
  - 进度注记：api 批次（ApiAuth/ApiTag + firstPartyAPIApp 回归 + 真实 Mongo 隔离库测试）已提交（25d4842/65c9054）；主站 BaseController 的收敛仍待主站批次完成。
  - 审核注记（2026-08-30）：已提交 api 批次的 `_token`/`_userId` session 回写当前被 cookie 写出顺序缺陷（design §4.1）静默丢弃——firstPartyAPIApp 回归走 token 参数路径故未暴露；该缺陷修复落地时须为回写补 Set-Cookie 断言。
- [ ] 分响应类型迁移 controller，每完成一类就运行对应 Golden，而不是最后一次性验证。

**余量清单（2026-08-29 审核盘点，按 Go 源文件计，`rg --glob '*.go' -l "github.com/revel" app` 现存 64 文件、其中 app/cmd 8 个归 Task 6 删除）：**

| 批次 | 控制器（含 Base 收敛） |
|---|---|
| 主站（14+1） | Auth（起始，最简模板渲染）→ Note、Notebook、User、Blog、Index、Tag、Share、File、Attach、NoteContentHistory、Captcha、Preview、**Album**（memory 旧清单漏记，已补）+ `BaseController.go` |
| api 余量（4+1） | ApiFile、ApiNote、ApiNotebook、ApiUser + `ApiBaseController.go` |
| admin（7+1） | Admin、AdminBlog、AdminData、AdminEmail、AdminSetting、AdminUpgrade、AdminUser + `AdminBaseController.go` |
| member（4+1） | MemberUser、MemberBlog、MemberGroup、MemberIndex + `MemberBaseController.go` |

**批次模式（已验证，照抄）：** `type XxxServer struct{}` + `func (s *XxxServer) Action(c *httpserver.Context) httpserver.Result` + `rs.Register("Xxx", "Action", beforeHooks, s.Action)`；`c.Session["_userId"]` 取 API 侧用户、`c.Session["UserId"]` 取 Web 侧用户；BEFORE 钩子 `httpserver.BeforeFunc`，返回 nil 继续。

**主站批次前置框架项（2026-08-30 审核新增，全部完成前 Auth 批次不得开工；依据见 design §4/§4.1/§3/§2.2）：**

- [ ] 修复 session 落 Cookie 顺序：handler 路径 `applySessionCookie` 须先于 `ApplyResult`（现状 Set-Cookie 在响应提交后被 net/http 丢弃，api 批次 `_token`/`_userId` 回写同受影响）；回归：登录成功响应含 Set-Cookie 且下一请求能解出 UserId，Logout 后会话匿名化。
- [ ] `Context.SessionID` 语义改为 `Session["_ID"]`（缺失时惰性 crypto 随机 hex 并经 SetSession 落 Cookie）；登录次数/验证码门控按其记账，CaptchaController 依赖不变。
- [ ] `Context` 增加 `ViewArgs` 并在 dispatch 注入框架键 RunMode/DevMode/session/currentLocale；RenderTemplate/NotFound 以 ViewArgs 渲染（NotFound 透传，不传 nil）。
- [ ] i18n LocaleResolver 接线进 cmd/leanote 的 App：cookie `i18n.cookie`（默认 `cookie.prefix+"_LANG"`）→ Accept-Language 首项 → ""，同时供 Context.Message 与 currentLocale 视图键。
- [ ] Params 补齐：Bool 等价于 `revel.Atob`（先 trim/lower，只有 `""`、`false`、`off`、`f`、`0`、`0.0` 为 false，其余任意值为 true）；`Params.Values`/`Params.Files` 保留原始键，令 `Has("Tags[0]")` 与 `Files[0][LocalFileId]` 等嵌套 binder 继续可用；[]string 切片视图中 `name[i]` 保留稀疏索引，索引大于 `params.max_index`（默认 4096）时忽略该参数并记录 binder 诊断，且不得按其分配切片；只有 `name[]` 在显式索引位置之后追加未索引值；再补 `Bind`（int/[]byte 文件）与 `Files` 文件头形态。
- [ ] 主站 BaseController 收敛：包装 `*httpserver.Context`，会话写全经 SetSession/DeleteSession；`ClearSession` 的 `"theme"` 小写怪癖原样保留。
- [ ] commonUrl 白名单字节级原样迁移（含死注册 Index/Oauth/Blog 条目与 `FindPasswword` 拼写，均不改名不清理）；LoginRequired 按 8 个被拦截 controller 的每个 action 挂 BEFORE。
- [ ] `RenderTemplateStr` 全仓零调用方，按 TemplateRenderer 等价移植（保留忽略错误的既有怪癖），不删除。

**批次伴随约束（审核新增）：**

- [ ] 每批次就地迁移该批所用 service 的 `revel.Config`/`revel.BasePath` 调用（对应 design §1.1 调用方清单：ConfigService 13、ThemeService 8、FileService 5、html2image 4、AttachService/AuthService 等），避免 Task 6 清扫堆积。
- [ ] 批次汇合时提取 `needValidateAPI`（api/httpserver.go）与 `needValidateWhitelist`（middleware.go）的字节级重复为单一白名单辅助。
- [x] 批次收尾在 cmd/leanote 完成 Task 2 注记的三项展示层启动装配（TemplateSetRenderer + i18n.LoadMessages + DefaultLanguage）。这不表示主站已可做真实页面验证：当前 `controllers.RegisterHTTP` 仅注册 `TestE2e`，主站 URL 仍返回 404；须先迁入主站 actions 及所需 dispatch 注入，才能验证页面渲染。

### Task 5：模板、博客和静态资源

**Files:** `app/init.go`（29 个 TemplateFuncs 注册）、`app/lea/blog/Template.go`、`app/lea/i18n/`（`revel.Message` 等价查找）、`app/views/`（含 errors/ 错误页）、`messages/`、`public/`

- [ ] 注册现有 27 个活跃 TemplateFuncs（29 处注册中 2 处块注释弃用），名集与行为以契约测试冻结；msg/leaMsg/blogTags 的 locale+messages 查找等价。
- [ ] 验证内置 default/elegant/nav_fixed 与一个上传主题，保持 Preview 错误展示。
- [ ] 验证 `/public`、`/js`、`/images`、`/img`、`/css`、`/fonts`、`/tinymce`、`/upload` 的路径与内容类型（module.static 默认行为收编）。

### Task 6：移植测试门禁、切换构建并删除 Revel

**Files:**
- Modify: `app/tests/harness/server.go`（启动移植到 `cmd/leanote`，新 flag 契约）、`app/tests/harness/cmd/e2e/main.go`、`.github/workflows/regression-baseline.yml`（删 Revel CLI 构建/ci-revel-cli 步骤）、`go.mod`、`go.sum`、`sh/run.sh`、`sh/package.sh`、`AGENTS.md`、`CLAUDE.md`
- Delete: `app/cmd/`

- [ ] 先移植 harness 启动（`buildServerBinary`/`serverRunMode` → 构建运行 `cmd/leanote`），确认 Golden/USN/权限门禁在新入口上运行——此步完成前不得删除任何 Revel 依赖。
- [ ] 清扫 Task 4 批次未覆盖的残余 Revel 调用方（design §1.2）：service/lea 层未随批迁移的 `revel.Config`/`revel.BasePath`/`revel.AppLog` 调用、`app/init.go` 旧 filter 链与 TemplateFuncs 注册、db 操作超时键接第一方 config（沿用 B 既有键名）。此步完成前不得进入删除步骤。
- [ ] 把开发启动改为构建/运行 `cmd/leanote`（`sh/run.sh`），生产打包不调用 Revel CLI（`sh/package.sh`）。
- [ ] 更新 CI node-tests job：删 "Build Revel CLI"/`ci-revel-cli` 步骤，改为构建 `cmd/leanote`。
- [ ] 删除 `github.com/revel/*` 依赖和 `app/cmd/`，执行 `go mod tidy`。
- [ ] 用源码搜索清理生产代码中所有 Revel 标识（`rg 'github.com/revel|revel\.' app go.mod sh conf` 零命中）；同步 AGENTS.md/CLAUDE.md 构建说明；只允许历史文档明确标注旧架构。

### Task 7：完整契约与关停验证

- [ ] 运行 `go build ./...`、`go vet ./...`、`go test ./app/tests/...`、`npm test`。
- [ ] 回放全部 API/web Golden、USN、权限、admin/member 与页面 smoke。
- [ ] 验证旧 Web Cookie 变匿名、新登录成功、升级前 API token 仍可用。
- [ ] 验证 prod 默认/空 secret 启动失败，合法 secret 启动成功。
- [ ] 发送 SIGTERM，确认停止接收新请求、进行中请求在 `http.shutdownTimeoutMs` 上界内完成、端口释放。
- [ ] 运行 `git diff --check` 并复核没有前端库或 Schema 改动。

## Rollback Point

只有在完整契约门禁全绿后合并。工作直落 `dev`；回滚为 revert C-b 的提交序列；不保留双栈或静默回落到 Revel。
