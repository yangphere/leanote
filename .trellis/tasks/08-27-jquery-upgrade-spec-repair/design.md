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

## 6. Round-2 Clarifications（2026-08-27 复核新增）

- **身份路由的可观察契约**：第 2 节「production/dev 路由必须不存在」与上游「非 test mode 或非 loopback 一律 404」按外部行为等价执行——在 test mode 且 loopback 之外，该端点必须无差别返回 fail-closed 404，且不泄露任何差异信号（注册方式、运行模式、数据库状态均不可区分）。实现方可选择条件注册或在 handler 内守卫，二者均满足契约。
- **marker 有效期阈值（RD-1，已确认）**：上游原仅表述「未过期」。2026-08-27 用户确认阈值为 `createdAt` + 2 小时（CI `node-tests` 35 分钟超时的宽裕上界），并已补写上游 PRD R-jQ3、design §3 与 implement 红灯清单。token 单次随机匹配仍是主控制，阈值只是过期分支回归的确定性依据。
- **member 流程执行账号（RD-5，已确认）**：上游只轮换 fixture admin（`admin@leanote.com`）；fixture 另有非 admin 账号 `demo@leanote.com`。2026-08-27 用户确认采用选项 (a)：member 区流程由同一已轮换 admin 账号执行，并在写入前验证其页面访问；不轮换、不使用 `demo` 账号。已补写上游 PRD R-jQ3 与 design §3。
