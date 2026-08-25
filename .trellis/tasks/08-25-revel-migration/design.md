# 迁出 Revel 到标准库 HTTP（C-b）— 技术设计

## 1. 新边界

建议新增 `app/httpserver/`，把框架职责集中而不让 `net/http` 细节散入 service：

- `server.go`：依赖组装、监听、优雅关停。
- `routes.go`：显式路由注册、静态资源和优先级。
- `registry.go`：受限 controller/action 注册表与 catch-all 分派。
- `request.go`：path/query/form/file 参数绑定。
- `response.go`：JSON、JSONP、text、template、binary、file 与 attachment 响应。
- `middleware.go`：恢复、日志、Mongo 健康、i18n、鉴权链。
- `session.go`：新 Cookie session 编解码与属性策略。
- `config.go`：兼容 `app.conf` section、插值与类型读取。
- `templates.go`：模板加载与 31 个 TemplateFuncs。
- `cmd/leanote/main.go`：纯 Go 可执行入口。

controller 仍负责 HTTP 编排，service 仍负责业务；service 不依赖 `net/http`。

## 2. 路由算法

1. 注册静态文件与 `conf/routes` 的显式规则，保持文件顺序和优先级。
2. 未命中时按前缀解析 controller/action。
3. controller/action 必须存在于编译期注册表；注册项同时声明方法、参数绑定器、鉴权策略。
4. 调用后只接受本项目 `Result` 类型，统一交给 response writer。

注册表可以由 Go 源码显式维护或由 `go generate` 产生并提交，但运行构建不得依赖生成步骤；不能使用“反射所有导出方法”的开放式路由。

## 3. Controller 迁移

定义第一方 `Context`、`Result` 与 `BaseController`，吸收原 `revel.Controller` 的 Session、Params、ViewArgs、Message 与 Render* 能力。先让 controller 编译于兼容层，再按响应类型清除残余 Revel 类型。API 包当前按值嵌入 BaseController 的语义必须保留并有回归测试。

## 4. Session

新 Cookie 使用 `app.secret` 做认证，解析失败视为匿名并记录安全级别日志，不尝试旧 Revel 解码。登录后写入与当前相同的 UserId/Email/Username/Theme 等键。API token 仍由 `SessionService` 的 MongoDB collection 管理，不迁入 Cookie。

prod 启动前校验 secret；dev/test 可使用明确的测试密钥。`Secure` 不从反向代理头猜测，只服从配置。

## 5. 配置与模板

复用已存在的 `github.com/robfig/config` 或等价小型兼容层解析 ini；先以测试钉住 section 覆盖、字符串插值、bool/duration。模板继续使用 `html/template`，启动时建立应用模板集；博客主题仍由 `app/lea/blog/Template.go` 独立克隆。

## 6. 错误与关停

- 参数绑定错误返回明确 4xx；未注册路由 404；panic 记录堆栈并返回 500。
- 响应写入后不能再改状态；统一 result writer 负责这一不变量。
- SIGTERM 触发 `http.Server.Shutdown`，停止接收新请求并等待受限时间；超时后返回非零错误。

## 7. 回滚

C-b 在独立分支完成，不保留双 HTTP 栈 feature flag。合并前需通过全部契约与主题测试；Schema 未变，回滚不涉及数据。一次性 Web 重登录是已接受部署影响。
