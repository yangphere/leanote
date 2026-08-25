# 迁出 Revel 到标准库 HTTP（C-b）— 执行计划

## Global Constraints

- 仅用 Go 1.26 标准库 HTTP 栈；不引入第二套路由框架。
- URL、API、USN、配置文件和模板主题保持兼容。
- 旧 Web Cookie 不兼容；API token 必须兼容。

### Task 1：先写框架边界契约测试

**Files:**
- Create: `app/httpserver/config_test.go`、`routes_test.go`、`session_test.go`、`response_test.go`
- Reuse: G 的 HTTP Golden、smoke 与双用户权限用例

- [ ] 写路由表测试覆盖显式优先、三类 catch-all、静态文件、路径参数、未注册 controller/action 404。
- [ ] 写 app.conf 测试覆盖 run-mode section、插值、bool、duration 和缺失必需配置。
- [ ] 写 session 测试覆盖新 Cookie、旧/损坏 Cookie 拒绝、HttpOnly、SameSite、Secure 与过期。
- [ ] 写 response 测试覆盖 JSON/JSONP/text/template/binary/file/attachment，先确认新包缺失导致测试失败。

### Task 2：建立配置、结果和服务器骨架

**Files:**
- Create: `app/httpserver/config.go`、`response.go`、`server.go`
- Create: `cmd/leanote/main.go`
- Modify: `app/init.go`

- [ ] 实现配置兼容读取和 prod secret 验证；测试密钥只存在 test 配置。
- [ ] 实现第一方 Result 与统一 writer，保证错误不被写成 200。
- [ ] 按原 `OnAppStart` 顺序组装依赖，增加受限 Shutdown。
- [ ] 运行 config/response/server 测试直到通过。

### Task 3：路由、参数与中间件

**Files:**
- Create: `app/httpserver/routes.go`、`registry.go`、`request.go`、`middleware.go`
- Modify: `conf/routes`（只在需要标明兼容解析规则时修改，不改 URL）
- Read/Port: `app/lea/route/Route.go`、`app/lea/i18n/`、四个 controller 包的 `init.go`

- [ ] 按 conf/routes 顺序注册显式路由和静态路径。
- [ ] 建立静态 action 注册表，声明 HTTP 方法、绑定参数和 controller 构造器。
- [ ] 移植前缀改写、commonUrl 白名单、鉴权、i18n、恢复与日志链。
- [ ] 运行路由负例，证明任意导出方法无法通过 URL 调用。

### Task 4：Session、BaseController 与 Render API

**Files:**
- Create: `app/httpserver/session.go`、`templates.go`
- Modify: `app/controllers/BaseController.go`、`app/controllers/api/ApiBaseController.go`
- Modify: controller action files under `app/controllers/`、`app/controllers/api/`、`app/controllers/admin/`、`app/controllers/member/`

- [ ] 实现新 Cookie session，保留业务键和配置项，不实现 Revel decoder。
- [ ] 把 Session/Params/ViewArgs/Message/Render* 收敛到第一方 BaseController/Result。
- [ ] 保持 API BaseController 按值嵌入行为并添加专门回归测试。
- [ ] 分响应类型迁移 controller，每完成一类就运行对应 Golden，而不是最后一次性验证。

### Task 5：模板、博客和静态资源

**Files:** `app/init.go`、`app/lea/blog/Template.go`、`app/views/`、`public/`

- [ ] 注册现有 31 个 TemplateFuncs 并加载应用模板。
- [ ] 验证内置 default/elegant/nav_fixed 与一个上传主题，保持 Preview 错误展示。
- [ ] 验证 `/public`、`/js`、`/images`、`/img`、`/css`、`/fonts`、`/tinymce`、`/upload` 的路径与内容类型。

### Task 6：切换构建并删除 Revel

**Files:**
- Modify: `go.mod`、`go.sum`、`sh/run.sh`、`sh/package.sh`、`CLAUDE.md`
- Delete: `app/cmd/`

- [ ] 把开发启动改为构建/运行 `cmd/leanote`，生产打包不调用 Revel CLI。
- [ ] 删除 `github.com/revel/*` 依赖和 `app/cmd/`，执行 `go mod tidy`。
- [ ] 用源码搜索清理生产代码中所有 Revel 标识；只允许历史文档明确标注旧架构。

### Task 7：完整契约与关停验证

- [ ] 运行 `go build ./...`、`go vet ./...`、`go test ./app/tests/...`、`npm test`。
- [ ] 回放全部 API/web Golden、USN、权限、admin/member 与页面 smoke。
- [ ] 验证旧 Web Cookie 变匿名、新登录成功、升级前 API token 仍可用。
- [ ] 验证 prod 默认/空 secret 启动失败，合法 secret 启动成功。
- [ ] 发送 SIGTERM，确认停止接收新请求、进行中请求在截止时间内完成、端口释放。
- [ ] 运行 `git diff --check` 并复核没有前端库或 Schema 改动。

## Rollback Point

只有在完整契约门禁全绿后合并。回滚为整体回退 C-b 分支；不保留双栈或静默回落到 Revel。
