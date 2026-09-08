# 应用层：身份认证与会话 — 执行计划

- [ ] 以 `domain-contracts` 的 API/用户/所有权契约为输入盘点 Auth/User/Session/Token 调用方。
- [ ] 将认证决策收敛到 service，移除 controller 重复判断和 Revel session 依赖。
- [ ] 补齐登录、注册、登出、密码、用户资料、Web/API token 和权限失败测试。
- [ ] 定义供 HTTP adapter 消费的 cookie/session writer contract，并用 adapter contract fixture 确认旧 Web cookie 和 API token 行为；真实 HTTP smoke 由 interface/delivery 任务验收。
- [ ] 运行目标 Go、Golden、权限和敏感信息扫描。

验证：`go test ./app/service ./app/controllers/api ./app/controllers`、`go vet ./app/service`、`git diff --check`。
