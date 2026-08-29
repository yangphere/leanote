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
- [ ] 实现第一方 Result 与统一 writer，保证错误不被写成 200。
- [ ] 按原 `OnAppStart` 顺序（5 处注册点）组装依赖，增加受限 Shutdown（`http.shutdownTimeoutMs`，默认 30000）。
- [ ] 运行 config/response/server 测试直到通过。

### Task 3：路由、参数与中间件

**Files:**
- Create: `app/httpserver/routes.go`、`registry.go`、`request.go`、`middleware.go`、`routes_test.go`
- Modify: `conf/routes`（只在需要标明兼容解析规则时修改，不改 URL）
- Read/Port: `app/lea/route/Route.go`（RouterFilter 前缀改写）、`app/lea/binder/binder.go`（MSSBinder/leanoteStructBinder/ObjectID binder）、`app/lea/i18n/`、四个 controller 包的 `init.go`（BEFORE 拦截器清单）

- [ ] 按 conf/routes 顺序注册显式路由（含 `*` 方法路由与 `/_test/e2e/identity`）和静态路径。
- [ ] 写路由表测试覆盖显式优先、三类 catch-all、静态文件、路径参数、未注册 controller/action 404。
- [ ] 建立静态 action 注册表，声明 HTTP 方法、绑定参数、controller 构造器与 BEFORE 钩子（活跃 25 处 InterceptFunc 逐条对照）。
- [ ] 移植前缀改写、鉴权（commonUrl 白名单随 AuthInterceptor）、i18n、恢复、gzip（CompressFilter 等价）与日志链。
- [ ] 运行路由负例，证明任意导出方法无法通过 URL 调用。

### Task 4：Session、BaseController 与 Render API

**Files:**
- Create: `app/httpserver/session.go`、`templates.go`
- Modify: `app/controllers/BaseController.go`、`app/controllers/api/ApiBaseController.go`
- Modify: controller action files under `app/controllers/`、`app/controllers/api/`、`app/controllers/admin/`、`app/controllers/member/`

- [ ] 实现新 Cookie session，保留业务键（UserId/Email/Username/UsernameRaw/Verified/Theme/themeId/NotebookWidth/NoteListWidth/LeftIsMin/Logo）和配置项，不实现 Revel decoder。
- [ ] 把 Session/Params/ViewArgs/Message/Render* 收敛到第一方 BaseController/Result；Render* 清单含 RenderTemplate/RenderJSON/**RenderJSONP**（BlogController 8 处）/RenderText/binary/file/attachment。
- [ ] 保持 API BaseController（`ApiBaseContrller`，按值嵌入 `controllers.BaseController`）行为并添加专门回归测试。
- [ ] 分响应类型迁移 controller，每完成一类就运行对应 Golden，而不是最后一次性验证。

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
