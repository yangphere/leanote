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

- [ ] 上游 jQuery PRD、design、implement 和任务元数据明确 R-SR1 的身份信号、验证时点、失败行为、账号角色与测试数据清理边界。「任务元数据明确」按以下可核验清单判定（路径覆盖本身不是充分条件，须与显式契约引用共同成立）：canonical 共享 helper `tests/e2e/e2e-environment.mjs`、两个 E2E 入口（`tests/e2e/build/build-resource-smoke.spec.mjs` 与 `tests/e2e/business/jquery-upgrade.spec.mjs`）、`.github/workflows/regression-baseline.yml`、`app/tests/harness`、`conf/app.conf`、`playwright.config.mjs` 与 browser-smoke record `docs/modernization/browser-smoke/jquery-3.7.md` 均出现在 `task.json.relatedFiles`；已存在的入口与配置文件（build smoke、sanitized reporter、playwright 配置）登记进两个 JSONL manifest，实现期才创建的 helper 与 business spec 在创建时同步登记（JSONL 校验硬性检查文件存在性，不得提前登记不存在的路径）；`task.json` notes 显式引用 R-jQ3/R-jQ6 契约。
- [ ] 上游规格明确 business E2E 的 CI 所有者、工作流步骤、环境变量、服务/fixture 生命周期、失败 artifact 和 cleanup 要求；相关文件纳入任务范围。
- [ ] 上游规格与父/根任务对四类浏览器、版本范围、执行环境、记录字段和失败处置使用同一条无歧义契约。
- [ ] 本整改任务的规格变更经结构校验和全量差异复审，无业务实现代码、工作流实现或任务激活。

## Out Of Scope

- 修改 Leanote 应用、Playwright 测试、CI workflow、MongoDB fixture 或浏览器运行环境。
- 激活 `08-25-jquery-upgrade` 或执行 jQuery 升级功能开发。
- 替 `08-25-frontend-libs` 修订其自身已过时的表述（见 RD-4）；该滞后不阻塞本任务，归属协调任务自有范围。

## Round-2 Audit Status（2026-08-27 复核）

提交 `6c299d0` 在创建本任务的同一次提交中已把三条需求对应的主要契约写入上游工件。逐条复核结果：

- **R-SR1 大部分已满足**：身份信号、验证时点、fail-closed 失败行为、fixture admin 角色与清理边界已在 `.trellis/tasks/08-25-jquery-upgrade/prd.md` R-jQ3、design 第 3 节、implement 第 1 步红灯清单中定义；`task.json.relatedFiles` 已覆盖 harness、`conf/app.conf`、workflow 与 Playwright 文件。member 执行账号契约残留另见 RD-5。
- **R-SR2 大部分已满足**：workflow 所有权、共享 harness 生命周期、fork PR 凭据路径与脱敏 artifact 契约已在 R-jQ6 与上游 design 第 5 节定义（详见上游工件，此处不复述条款）。
- **R-SR3 已满足**：R-jQ7/AC-jQ9 与根 PRD R-E、`08-25-frontend-libs` prd R-E2/R-E3、`08-25-cicd-delivery` prd R-F9/design 使用同一条契约：Chromium 阻断 PR/push，四款真实浏览器当前及前一主版本做发布前 smoke，Chromium 不充当 Chrome/Edge；记录字段实质对齐（R-jQ7 为超集，额外要求执行日期与身份验证结果）。
- 本轮新核对通过的事实：workflow 目前仍为 dev mode `revel run` + Mongo 5.0 fixture 且只跑 build smoke（Confirmed Fact 2 现仍成立）；`app/tests/README.md` 的 `env up` 可恢复 `leanote_test`；`conf/app.conf [test]` 绑定 loopback 与 `leanote_test`。

### Residual Findings（本轮发现的剩余缺口）

| ID | 发现 | 影响 | 处置 |
|----|------|------|------|
| RD-1 | 上游仅要求 marker「未过期」，未定义任何具体有效期阈值，过期回归无法写成确定性断言 | 阻塞 R-jQ3 过期分支的可验收实现 | 已确认阈值 `createdAt` + 2 小时（2026-08-27 用户采纳建议），已补写上游 PRD R-jQ3、design §3 与 implement 红灯清单 |
| RD-2 | 本任务 design 说「production/dev 路由必须不存在」，上游却规定非 test/非 loopback 一律 404 | 表述冲突，可能被误读为两种互斥实现 | 已在本任务 design 第 6 节消歧为可观察契约等价 |
| RD-3 | AC-1 中「任务元数据明确…」缺少判定标准 | 验收时对 `task.json` 是否达标可产生分歧 | 判定标准并入下方验收标准 AC-1 |
| RD-4 | `08-25-frontend-libs/design.md` 仍要求「E 必须在 package.json 增加 test:e2e」，但该脚本与 business project 已由 D 预置 | 协调任务文档轻微滞后，非阻塞 | 已确认延后至 `08-25-frontend-libs` 启动时处理（2026-08-27）；修改权归该任务 |
| RD-5 | 上游仅轮换 fixture admin（`admin@leanote.com`）密码；member 区流程由哪个账号执行、fixture 已有的非 admin 账号（`demo@leanote.com`）是否纳入轮换均未约定 | R-SR1「覆盖 admin/member 权限」的 member 侧证明不完整 | 已确认选项 (a)（2026-08-27 用户采纳建议）：member 流程复用已轮换 admin 账号；已补写上游 PRD R-jQ3 与 design §3 |

## Open Questions

无。原三项已于 2026-08-27 经用户确认采纳建议：RD-1 阈值 `createdAt` + 2 小时、RD-5 选项 (a)（member 流程复用已轮换 admin 账号）均已回填上游；RD-4 确认延后至 `08-25-frontend-libs` 启动时处理。
