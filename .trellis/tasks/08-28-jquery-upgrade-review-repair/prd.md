# 修复 jQuery 升级复审问题

## Goal

在不改变 jQuery 3.7 升级既定 URL、API、页面行为和 test-mode 隔离契约的前提下，修复上一轮复审确认的 13 个缺陷，使第一方兼容性扫描、诊断 E2E、业务清理、harness 监督和报告门禁都 fail-closed。

## Scope and Requirements

1. 第一方兼容性：将 `public/js`、`public/md/main-v2.js`、admin/member/blog/album、插件和 `app/views/**/*.html` 内联脚本中的 jQuery shorthand（`.click/.focus/.change/.submit` 等）改为显式事件 API；补充静态契约覆盖无参 shorthand、Markdown bundle 和模板；为 Markdown 内嵌 `waitForImages 1.4.2` 的 `$.isFunction` 建立清单中的第三方所有权/精确豁免，不得修改上游字节。
2. 诊断与业务 E2E：相册/图片流程在注册 cleanup 后再解析和断言，确保任何中途失败均清理；图片 fallback 检查 HTTP 状态并在删除后复查列表；身份复核失败时 fail-closed，跳过破坏性 cleanup 并使测试失败；每个 AJAX failure route 注入前重新确认身份；跨导航持久收集 unhandled rejection。
3. Harness 与进程安全：服务进程在 readiness 等待前登记 supervisor；teardown 按当前 run token/摘要精确删除 marker；Windows Job Object 创建或配置失败必须阻止启动 child 并返回错误。
4. 报告与测试：修正 Playwright reporter `onBegin(config, suite)` 签名，更新单测以覆盖真实调用；Firefox 栈正则支持带端口 URL；清理 Trellis manifest 中的登记 warning，不引入秘密、fallback、生产 migrate 或第二 jQuery。

## Acceptance Criteria

- [ ] 静态契约对第一方 JS、Markdown 和所有模板内联脚本报告零未豁免 shorthand；`waitForImages` warning 仅按清单登记的第三方所有权豁免，未登记来源 fail-closed。
- [ ] diagnostics 相册/图片和 business 所有写入流程在断言、身份复核或请求失败时均执行 finally 清理；清理失败、身份复核失败或残留复查失败导致测试失败且不继续删除共享数据。
- [ ] harness 在 readiness、信号退出、teardown、marker 和 Windows 子进程监督失败时返回非零并完成可执行的剩余清理；只删除本次 run 的 marker。
- [ ] reporter 在真实 Playwright 生命周期中标记 active，摘要保持脱敏；跨导航 unhandled rejection、HTTP 状态和网络错误门禁仍有效。
- [ ] 通过针对性 Node/Go 测试、`npm test`、`npm run build` 双跑、`git diff HEAD --check`、`task.py validate 08-28-jquery-upgrade-review-repair`；真实 Mongo/Revel E2E 在环境可用时执行，否则明确记录为未验证，不伪造通过。

## Constraints and Out of Scope

- 不升级 Bootstrap/TinyMCE/jQuery 版本，不修改公开静态 URL、后端 API、数据库 schema 或视觉行为。
- 不手工编辑生成压缩文件；生成资源只由既有 build 管线产生。
- 不提交、推送、回滚或覆盖用户已有的其他工作区改动。

## Evidence Anchors

- 第一方 shorthand：`public/js/app/note.js`、`public/js/plugins/attachment_upload.js`、`public/js/plugins/editor_drop_paste.js`、`public/js/app/tag.js`、`public/md/main-v2.js`、`public/member/js/avatar.js`、`app/views/**/*.html`。
- 诊断/业务：`tests/e2e/business/jquery-diagnostics.spec.mjs`、`business-flows.spec.mjs`、`ajax-failure.spec.mjs`。
- Harness/reporter：`app/tests/harness/cmd/e2e/main.go`、`app/tests/harness/server.go`、`app/tests/harness/cmd/e2e/process_windows.go`、`tests/e2e/build/sanitized-summary-reporter.mjs`。
