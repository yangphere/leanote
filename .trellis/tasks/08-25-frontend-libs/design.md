# 前端库现代化（E）— 协调设计

## 1. 为什么拆成三个子任务

jQuery 是基础运行时，Bootstrap 影响模板与交互，TinyMCE 影响数据与第一方插件，三者风险和回滚方式不同。顺序固定为 jQuery → Bootstrap → TinyMCE，使每次失败只有一个主要变量。

## 2. 共享兼容矩阵

| 维度 | 不变量 | 证据 |
|---|---|---|
| 浏览器 | Chrome、Edge、Firefox、Safari 当前及前一主版本 | PR/push 的 D/E Chromium smoke/E2E；发布前真实四浏览器 smoke 的受跟踪脱敏记录 |
| 资源 | 源码唯一事实源，生成物继续入库 | `npm run build` 后零 diff |
| 后端 | API/USN/所有权不变 | G 的回归套件 |
| 页面 | URL、服务端模板、博客主题不变 | 页面与主题 smoke |
| 数据 | 未编辑不保存；编辑后 HTML 语义等价 | 存量 HTML fixture |

## 3. 集成方式

每个子任务在自己的分支完成并回放整套前端 smoke。后一个子任务只基于前一个已验收状态开始。`frontend-libs` 复用 D 的 `playwright.config.mjs`、Chromium 安装步骤和 G 兼容服务/账号环境：D 的 `test:e2e:build` 通过 `--project=build-smoke` 先验证生成资源与页面加载，E 的 `test:e2e` 通过 `--project=business` 从 `tests/e2e/business/**/*.spec.{js,mjs}` 发现完整业务流程。最终集成不再改库版本，只运行组合门禁和修正文档/生成资源清单中的不一致；若需要生产代码修复，应回到拥有该代码的子任务。

E 不创建第二套 Playwright 配置或锁文件。E 必须在 `package.json` 增加 `test:e2e`，其命令固定为 `playwright test --config=playwright.config.mjs --project=business`；完整业务测试放在 `tests/e2e/business/`，继承 D 的 Chromium project、test-mode fixture identity、`LEANOTE_BASE_URL`/run token 生命周期和脱敏报告约束。D 的 `build-smoke` 选择不应发现或执行 E 的业务用例。每个子任务在发布验收前补齐真实 Chrome、Edge、Firefox、Safari 当前及前一主版本的脱敏记录；Chromium 仅覆盖 PR/push 阻断路径。

## 4. 回滚

三个库升级分别形成独立可回滚单元。回滚 TinyMCE 不要求回滚 Bootstrap/jQuery；若 Bootstrap 回滚，则 TinyMCE 只有在其插件依赖 Bootstrap 适配时随之回滚。任何回滚后均重新构建并执行漂移门禁。
