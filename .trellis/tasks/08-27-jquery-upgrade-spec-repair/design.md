# jQuery 3.7 规格审核整改 - 技术设计

## 1. Boundary

本任务只修复规划契约。实际的 test-only identity route、harness、Playwright helper、workflow 和浏览器 smoke 均由 `08-25-jquery-upgrade` 在获批并激活后实现；本任务不修改运行时、测试或 CI 行为。

## 2. Write-Capable E2E Identity Contract

business E2E 不再信任任意 `LEANOTE_BASE_URL`。仓库 harness 恢复 `leanote_test` 后生成单次随机 run token，在临时 `e2e_runs` 记录中保存，并以 Revel test mode 启动 loopback 服务。只有该模式的 `/_test/e2e/identity` 返回匹配的 token 和 `leanote_test`；production/dev 路由必须不存在。Playwright shared fixture 先比对身份，再登录、验证 admin/member 访问并允许写入。测试结束移除自身数据和 marker，harness 最后销毁容器。

该设计使误指向真实服务时在第一条写请求前失败：真实服务既不注册 test-only 路由，也没有本次 run 的 fixture marker。随机 token 不是认证凭据，仍必须从摘要、日志和 artifact 排除。

## 3. CI Gate Contract

现有 `node-tests` 的 build smoke 服务生命周期扩展为 test-mode harness。workflow 在同一生命周期内先执行只读 build smoke，再执行 business E2E，并为两者保留独立的 allowlisted 摘要。服务、Mongo 和临时 marker 仅在两条命令后清理；身份、业务、报告或 cleanup 任一失败均保留失败退出码。

## 4. Browser Evidence Contract

PR/push 使用 Chromium 阻断 E2E。发布前由真实 Chrome、Edge、Firefox、Safari 的当前及前一主版本执行 smoke；每条记录包含 commit、浏览器产品和完整版本、OS、覆盖范围、认证/错误门禁、结果，且不含敏感数据。此契约同时写入根/协调/交付任务，避免 Chromium 被误记为 Chrome 或 Edge 证据。

## 5. Rollback

本次只修改计划工件；任何技术选择若不能保持 test-only 路由、随机 marker、无泄露摘要和四浏览器证据，必须回到规划而不能以人工声明或生产 fallback 代替。
