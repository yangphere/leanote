# jQuery 3.7 规格审核整改

## Goal

补齐 `08-25-jquery-upgrade` 在写入型 E2E 隔离、PR 合并门禁和浏览器兼容证据方面的规格缺口，使后续实现可以在不修改真实数据且不遗漏必经质量门的条件下开展。

## Confirmed Facts

- 审核开始时，`08-25-jquery-upgrade` 的 business E2E 会覆盖上传、相册、博客、admin/member 等写入路径，但其规格仅以 `LEANOTE_BASE_URL`、账号和密码描述环境前置条件。
- 审核开始时，`regression-baseline.yml` 在独立 MongoDB fixture 和 Leanote 服务上运行 build smoke，却没有执行 `npm run test:e2e` business project。
- 审核开始时，根任务与 `08-25-frontend-libs` 要求 Chrome、Edge、Firefox、Safari 的当前及前一主版本支持，而 jQuery 子任务仅定义 Firefox、Safari 和 Playwright Chromium 的证据。
- `conf/app.conf [test]` 和 `app/tests/harness` 已把服务限制为 `leanote_test` 与 loopback，但当前 browser smoke 没有身份端点或每次运行的 fixture marker，不能把该事实传递给独立的 Playwright 进程。

## Requirements

- **R-SR1 Environment identity:** 写入型 business E2E 在登录、创建或删除任何业务数据前，必须以可自动验证、fail-closed 的方式证明目标服务是指定的隔离 fixture，并验证测试账号具备所覆盖的 admin/member 权限。仅存在 URL、账号、密码或人工约定不能构成证明。
- **R-SR2 Required business gate:** `npm run test:e2e` 必须成为该任务 PR/push 的实际阻断检查，与 build smoke 使用同一隔离服务生命周期、凭据注入、脱敏报告和无条件清理契约；失败不能由手工声明替代。
- **R-SR3 Browser matrix evidence:** Chrome、Edge、Firefox、Safari 的当前及前一主版本均须有可审计 smoke 记录。Chromium 自动化可继续作为 PR 阻断 E2E，但不能代替真实 Chrome/Edge 的版本化证据。

## Acceptance Criteria

- [ ] 上游 jQuery PRD、design、implement 和任务元数据明确 R-SR1 的身份信号、验证时点、失败行为、账号角色与测试数据清理边界。
- [ ] 上游规格明确 business E2E 的 CI 所有者、工作流步骤、环境变量、服务/fixture 生命周期、失败 artifact 和 cleanup 要求；相关文件纳入任务范围。
- [ ] 上游规格与父/根任务对四类浏览器、版本范围、执行环境、记录字段和失败处置使用同一条无歧义契约。
- [ ] 本整改任务的规格变更经结构校验和全量差异复审，无业务实现代码、工作流实现或任务激活。

## Out Of Scope

- 修改 Leanote 应用、Playwright 测试、CI workflow、MongoDB fixture 或浏览器运行环境。
- 激活 `08-25-jquery-upgrade` 或执行 jQuery 升级功能开发。

## Open Questions

无。现有 test-mode harness 足以作为启动基础；缺失的 browser-visible 身份信号已明确为后续 jQuery 任务新增的 test-only contract。
