# 技术设计

## 边界与不变量

本修复任务只改变复审指出的错误分支和扫描覆盖，不改变既有 jQuery 3.7 运行时资产、公开 URL/API、服务端模板结构或 test-mode 身份协议。任何写入型 E2E 都必须在本次 run 的身份 marker 仍有效时执行，失败时 fail-closed。

## 方案

- 前端：用 `.on(...)`/显式事件处理器替换第一方 shorthand；把模板内联脚本纳入同一静态扫描；对 Markdown 中的第三方 `waitForImages` 仅登记归属和实际命中豁免，保持逐字节上游输入。
- E2E：将资源句柄和 cleanup 注册置于解析/断言之前；用状态检查和删除后列表复查证明删除成功；共享身份 helper 在 route 注入、写入和 finally 前提供新鲜确认；通过 Node 进程级收集器跨导航保存 rejection。
- Harness：启动后立即把 server 加入 supervisor，再等待 readiness；teardown 根据 token 摘要精确删除 marker 并聚合错误；Windows Job Object API 错误向上传递，禁止无监督 child。
- Reporter：遵循 Playwright `onBegin(config, suite)` 生命周期，在测试中以双参数调用验证 active 状态；栈解析允许 `host:port`。

## 回滚与风险

每组改动可独立回退；不触碰既有生成源之外的压缩产物。若静态扫描发现第三方 warning 无法建立精确归属，保留 fail-closed 失败并停止扩大豁免，不添加 shim 或静默过滤。
