# 迁出 Revel 到标准库 HTTP（C-b）— 技术设计

## 1. 新边界

建议新增 `app/httpserver/`，把框架职责集中而不让 `net/http` 细节散入 service：

- `server.go`：依赖组装、监听、优雅关停。
- `routes.go`：显式路由注册、静态资源和优先级。
- `registry.go`：受限 controller/action 注册表与 catch-all 分派。
- `request.go`：path/query/form/file 参数绑定。
- `response.go`：JSON、JSONP、text、template、binary、file 与 attachment 响应。
  - FileResult 的 Content-Type 使用确定性扩展名映射（`fileContentTypes` + mime 回退 + application/octet-stream 兜底），规避 mime.TypeByExtension 在 Windows 上读注册表的机器差异；mp4 等条目为 `/upload` 上传可达类型。
- `middleware.go`：恢复、URL 改写、静态资源、日志、i18n、鉴权、gzip。
- `session.go`：新 Cookie session 编解码与属性策略。
- `config.go`：兼容 `app.conf` section、插值与类型读取。
- `templates.go`：模板加载与 30 个模板函数（27 个项目活跃注册 + 视图依赖的 Revel 内置 set/append/pad）。
- `cmd/leanote/main.go`：纯 Go 可执行入口。

controller 仍负责 HTTP 编排，service 仍负责业务；service 不依赖 `net/http`。

## 1.1 第一方 seam（删 Revel 的前置条件）

`rg` 证实 Revel 依赖不止 controller 层，先建三个 seam 并迁移调用方，才可能达成 R-Cb5 零命中：

- **config**：`revel.Config` 34 处——app/db 8 处（db.url/host/port/dbname/连接与操作超时等，B 的验收键）、service/ConfigService 13、controllers（Note 4、ApiNote 2 等）、lea（i18n 3、Email/blog/html2image 各 1）、admin 1。`httpserver/config.go` 暴露等价读取（String/Int/Bool/section 语义），app/db 改为依赖注入或该 seam。
- **logger**：`lea/Debug.go` 的 Log/Logf/LogW/LogJ 是全站唯一日志门面，直连 `revel.AppLog`。保留函数签名，后端替换为标准库 slog（或等价结构化 logger），输出格式无外部契约。
- **BasePath**：`revel.BasePath` 45 处（ThemeService/FileService/html2image 及 controllers/service 的文件系统与 URL 路径拼接）。配置键定名 `basePath`（默认 "."，即应用根，与 Revel 现值一致；已确认，不再在实现期更名）。

## 2. 路由算法

1. 注册静态文件与 `conf/routes` 的显式规则，保持文件顺序和优先级。
2. 未命中时按前缀解析 controller/action。
3. controller/action 必须存在于编译期注册表；注册项同时声明方法、参数绑定器、鉴权策略。
4. 调用后只接受本项目 `Result` 类型，统一交给 response writer。

注册表可以由 Go 源码显式维护或由 `go generate` 产生并提交，但运行构建不得依赖生成步骤；不能使用“反射所有导出方法”的开放式路由。

## 2.1 Filter 链映射（app/init.go 现序 → 新中间件）

| Revel 现链 | 处置 |
|---|---|
| PanicFilter | recover 中间件：记录堆栈、按 run-mode 渲染 errors/500(-dev).html 或 500-blog.html |
| route.RouterFilter | 改写中间件：保持链首业务位；内部 `revel.MainRouter.Route` 二次路由逻辑移植为显式前缀映射（/api、/member、commonUrl 白名单） |
| FilterConfiguringFilter | 并入注册表（per-action 钩子声明） |
| ParamsFilter | request.go 绑定（含 lea/binder 的 MSSBinder/leanoteStructBinder 与 ObjectID TypeBinder 等价物） |
| SessionFilter | session.go |
| FlashFilter | 删除：生产代码零使用（grep 证据） |
| ValidationFilter | 删除：`revel.Validation` 仅 app/cmd/parser2（AST 分析器）引用，随 app/cmd 删除 |
| i18n.I18nFilter | i18n 中间件（lea/i18n 移植，解析 locale 并注入 CurrentLocale view-arg） |
| InterceptorFilter | registry 的 BEFORE 钩子执行（**活跃 25 处** InterceptFunc 注册按 controller 迁入注册表声明，与 §2.2 一致） |
| CompressFilter | gzip 中间件：Accept-Encoding 协商行为等价（浏览器可见行为；golden harness 客户端不发 Accept-Encoding，故录制不受影响）；压缩级别等参数无外部契约，实现期自定 |
| ActionInvoker | registry 分派 + result writer |

`revel.Message(locale, tag)`（messages/<locale> 查找，模板 msg/leaMsg/blogTags 依赖）由 templates.go/i18n 提供等价实现；`CurrentLocale` view-arg 键名不变。

## 2.2 拦截器与参数绑定清单

- `AuthInterceptor`（controllers/init.go）以 BEFORE 注册于 Notebook/Note/Share/User/Album/File 等主站 controller；api/admin/member 四个 init.go 各有自己的 BEFORE 集合——**活跃注册 25 处**（27 处文本匹配中 controllers/init.go:144,152 已 `//` 行注释）逐条对照迁入注册表声明，缺一条即鉴权回归；commonUrl 白名单（controllers/init.go 与 admin/init.go）是 AuthInterceptor 的跳过名单，随其迁移。
- `needValidate(controller, method)` 白名单语义保留。
- 自定义参数绑定器：lea/binder 的 `MSSBinder`（map[string]string）与 `leanoteStructBinder`（`revel.TypeBinders` 注册，binder.go:148-155）在 request.go 等价重建；ObjectID 参数无绑定器——action 内经 `db.MustObjectIDFromHex` 转换（现状保持）。

## 3. Controller 迁移

定义第一方 `Context`、`Result` 与 `BaseController`，吸收原 `revel.Controller` 的 Session、Params、ViewArgs、Message 与 Render* 能力。先让 controller 编译于兼容层，再按响应类型清除残余 Revel 类型。API 包当前按值嵌入 BaseController 的语义必须保留并有回归测试。

## 4. Session

新 Cookie 使用 `app.secret` 做认证，解析失败视为匿名并记录安全级别日志，不尝试旧 Revel 解码。登录后写入与当前相同的业务键（controllers 实际清单：UserId、Email、Username、UsernameRaw、Verified、Theme、themeId、NotebookWidth、NoteListWidth、LeftIsMin、Logo；API 侧每个请求另写 `_token`/`_userId`，见 api/init.go:81,98）。API token 仍由 `SessionService` 的 MongoDB collection 管理，不迁入 Cookie。API 无 token 时的回退路径读取 `c.Session.ID()`——新 session 须提供等价的会话标识（cookie 值的稳定哈希即可）。旧配置键 `cookie.httponly` 不再读取：新 Cookie 恒 `HttpOnly=true`（R-Cb3）。

prod 启动前校验 secret（空或仓库公开默认值 `V85Zz…` 拒绝启动）；dev/test 可使用明确的测试密钥。`Secure` 不从反向代理头猜测，只服从配置。

## 4.1 测试门禁与 CI 移植（合并门禁所在，缺失即失守）

G 建立的 Golden/USN/权限/页面 smoke 全部经 `app/tests/harness/server.go` 启动被测服务器：现流程为 `buildServerBinary`（Revel app tmp 构建）+ `serverRunMode`（依赖 Revel CLI）+ `-importPath/-runMode` 子进程参数。C-b 必须：

- 把 `buildServerBinary`/启动参数移植为构建并运行 `cmd/leanote`（新 flag 契约，例如 `-conf/-port`，tests 与 CI 同步）；
- `cmd/e2e`（Playwright 前置）不改测试逻辑，仅跟随新启动方式；
- CI node-tests job 删除 "Build Revel CLI from the main module graph" 及 `ci-revel-cli` 摘要步骤，改为构建 `cmd/leanote`；
- 移植完成前不得删除任何 Revel 依赖（顺序约束：先移植门禁，再删框架）。

## 5. 配置与模板

复用已存在的 `github.com/robfig/config` 或等价小型兼容层解析 ini；先以测试钉住 section 覆盖、字符串插值、bool/duration。模板继续使用 `html/template`，启动时建立应用模板集；博客主题仍由 `app/lea/blog/Template.go` 独立克隆。

## 6. 错误与关停

- 参数绑定错误返回明确 4xx；未注册路由 404；panic 记录堆栈并返回 500。
- 响应写入后不能再改状态；统一 result writer 负责这一不变量。
- SIGTERM 触发 `http.Server.Shutdown`，停止接收新请求并等待 `http.shutdownTimeoutMs`（默认 30000，配置可调）；超时后返回非零错误。
- 实现期加固（无 Revel 对应物，防御性新增）：`ReadHeaderTimeout` 30s；SIGTERM 与 SIGINT 均触发关停；`Server.Run` 另接受显式 `stop` 通道供测试/编程式停止。

## 6.1 对 B 任务归档设计 §2 的更正（supersession）

B（archive/2026-08/08-25-mongo-driver-migration）design §2 冻结的映射
`Update → UpdateOne`、`Upsert → UpdateOne + SetUpsert(true)` 在实践中被驱动
v2.8.1 推翻：`UpdateOne` 强制 `ensureDollarKey`（"update document must
contain key beginning with '$'"），而 mgo 的 Update/Upsert 同时接受算子式
（`{"$set": …}`）与替换式（全量文档）两种形态——service 层有大量替换式
调用方（TokenService、BlogService 等）。修正：app/db 引入
`splitUpdateKind`（按文档首键是否以 `$` 开头嗅探），替换式走
`ReplaceOne`（Upsert 为 `ReplaceOne + SetUpsert`），算子式走 `UpdateOne`。
归档设计不可改；以本节为准。回归测试：
`app/db/mongo_compat_test.go` TestCompatReplacementUpdateAndUpsert。

## 7. 回滚

C-b 与 B 同策略，工作直落 `dev`（已确认；不另开独立分支）。不保留双 HTTP 栈 feature flag。合并/推送前需通过全部契约与主题测试；Schema 未变，回滚为 revert C-b 的提交序列，不涉及数据。一次性 Web 重登录是已接受部署影响。
