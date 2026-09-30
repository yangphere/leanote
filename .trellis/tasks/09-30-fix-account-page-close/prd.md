# 修复账户设置页关闭按钮无响应

## 目标

让通过普通浏览器请求打开的独立账户设置页与登录页保持一致的卡片布局和视觉风格，并在点击“关闭”后直接返回登录页；通过 AJAX 加载的账户设置模态框继续由 Bootstrap 处理关闭。

## 背景与已确认事实

- `User.Account` 根据 `X-Requested-With` 在 AJAX 请求时渲染 `user/account.html`，普通请求时渲染 `user/account_page.html`。
- 独立页面的 `accountInfoDialog` 只有 `.modal-dialog`/`.modal-content` 结构，没有 Bootstrap `.modal` 容器；其关闭按钮仍只声明 `data-bs-dismiss="modal"`，因此没有可供 Bootstrap 关闭的模态实例。
- AJAX 片段由现有 `showDialogRemote` 放入模态容器，关闭行为属于现有 Bootstrap 模态契约，不应被独立页面修复破坏。
- 当前工作区已有 `README.md`、`app/controllers/httpserver_main.go` 的修改和未跟踪的 `app/views/user/account_page.html`；本任务只处理关闭行为，不回滚或重写这些既有改动。

## 需求

1. 独立账户页应复用登录页的绿色背景、500px 白色卡片、灰色标题栏、表单内边距、按钮和响应式宽度节奏。
2. 独立账户页点击页头关闭按钮或页脚“关闭”按钮时，应直接导航到 `/login`。
3. AJAX 模态片段中的两个关闭按钮保持 Bootstrap `data-bs-dismiss` 关闭，不导航到 `/login`。
4. 关闭处理应绑定到独立页面的明确边界，避免向共享账户片段或全局添加重复逻辑。

## 验收标准

- [x] 普通请求打开 `/user/account` 后，页面视觉结构与登录页完全一致（白色卡片、浅灰标题栏、表单控件比例、虚线分割白色页脚、外部 `#boxFooter` 版权链接），解决中间内容区穿透呈绿色的样式缺陷。
- [x] 独立页点击页头关闭按钮后到达 `/login`。
- [x] 独立页点击页脚“关闭”按钮后到达 `/login`。
- [x] AJAX 请求获取账户片段后，点击任一关闭按钮仍由 Bootstrap 关闭模态，不导航到 `/login`。
- [x] 运行针对性静态/前端测试与必要的 Go 检查；无法运行真实浏览器或 MongoDB 验收时，明确记录为未运行。

## 验证证据

- `npm test`: 209 tests, 208 passed, 1 skipped, 0 failed。
- `go test ./app/controllers -run TestNativeUserActionUsesWebAuthBoundary -count=1`: passed。
- `go test ./app/httpserver -run TestAccountPageTemplateStructure -v`: passed。
- `go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote`: passed。
- `go vet ./...`: passed。
- `git diff --check`: passed。
- 真实浏览器交互、MongoDB 和 HTTP 集成验收：未运行。

## 范围外

- 不改变账户设置表单、用户/邮箱/密码更新接口或模态加载路由。
- 不重写现有 README、控制器迁移改动或生成的前端资源。

## 未决问题

无。用户已明确指定登录页视觉风格和 `/login` 跳转目标。
