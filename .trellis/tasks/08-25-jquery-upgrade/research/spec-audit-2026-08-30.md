# jQuery 3.7 规格审核记录（2026-08-30）

## 结论

本轮选中的执行叶为 `08-25-jquery-upgrade`：后端轨道已完成到 `08-25-revel-migration`，前端构建链 `08-25-frontend-build-chain` 已归档；其唯一直接依赖已完成，且 jQuery 位于 Bootstrap → TinyMCE 之前。该任务已通过 `task.py start` 恢复任务指针；本轮不修改业务实现。

## 证据核对

- 版本和生成物：`package.json` 锁定 `jquery@3.7.1`、`jquery-migrate@3.6.0`、`blueimp-file-upload@10.32.0`；`scripts/build/manifest.mjs` 声明 `jquery-runtime` 与 34 项输出；`tests/js/jquery-asset-contract.test.js` 验证唯一 runtime、旧 URL 保留和生产无 migrate。
- 隔离身份：`app/controllers/TestE2eController.go` 将模式、loopback、`leanote_test`、唯一 marker、token 摘要和有效期作为 fail-closed 条件；`TestE2eController_test.go` 已覆盖缺失、重复、过期、未来时间和摘要错误。
- E2E/CI：`tests/e2e/e2e-environment.mjs` 在登录前预检并支持写入前刷新确认；`business-flows.spec.mjs` 注册后置清理并复查资源；`.github/workflows/regression-baseline.yml` 在同一 harness 中依次执行 build smoke 与 business E2E。
- 外部证据：`docs/modernization/browser-smoke/jquery-3.7.md` 已记录 Windows 上的 Chrome/Edge/Firefox 当前版本，但真实 Safari、前一主版本和最终合并 SHA 仍明确列为发布验收阻断项；不将缺失环境证据假定为通过。

## 本轮规格修订

1. 将实现已有的 marker 未来时间边界（最多允许 1 分钟时钟偏差，超过即 503）写入 PRD、技术设计、执行计划和 AC-jQ6，避免只存在于代码/测试而无法按规格验收。
2. 将继承自根任务的最小只读 `GITHUB_TOKEN` 权限写入 R-jQ6、AC-jQ10、设计和执行计划；当前 workflow 尚未声明该权限，后续实现/复核必须显式补齐或记录阻断。

## 审核问题修复（2026-08-30）

1. 修正执行计划的数据库基线：现行 test-mode harness、回归 workflow 与父级 CI/CD 规格均以 MongoDB 8.0 为基线；MongoDB 5.0 已从 `implement.md` 删除，并标明其他版本只能作为额外兼容性验证。
2. 修正 PRD 的任务状态叙述：`08-27-jquery-upgrade-spec-repair` 已完成归档，当前 ready/执行叶是 `08-25-jquery-upgrade`，与 `task.json.meta.depends_on` 保持一致。
3. 补齐未来时间边界的可验收规格：`createdAt = validationNow + 60s` 必须返回 200，`createdAt = validationNow + 60s + 1ns`（实现精度不足时可用 `+1s`）必须返回 503。由于本轮禁止修改测试实现，精确边界回归仍是后续编码阶段必须新增并运行的测试，不宣称当前测试已覆盖该等价/最小超界值。
4. 修正 PRD 的生成物与输入事实：`scripts/build/manifest.mjs` 当前声明 34 项输出；`jquery-runtime`、`dep.min.js` 和 `album/js/main.all.js` 均从 `node_modules/jquery/dist/jquery.min.js` 读取 jQuery 3.7.1，`public/js/jquery-1.9.0.min.js` 仅是保留公开 URL 对应的生成输出，不再作为这些 bundle 的输入。该修订与 R-jQ1、R-jQ2、AC-jQ2 和 AC-jQ3 对齐。

## 未改变的边界

- `08-25-frontend-libs` 中已记录的 RD-4（`test:e2e` 脚本表述滞后）仍由协调父任务在其启动时处理，本轮不越权修改父任务工件。
- Safari 与前一主版本 smoke 仍是外部环境依赖，不伪造记录、不降低 AC-jQ9。
- TinyMCE 内核、Bootstrap 迁移、后端 API/USN、数据库 schema 和生产 bundle 均未修改。
- 规格审核当时 CI workflow 尚未显式声明 `permissions: contents: read`；实现阶段已补齐该最小只读权限，并在本轮复核中重新检查。

## 验证

- `python ./.trellis/scripts/task.py validate 08-25-jquery-upgrade`：通过（implement 14 条、check 13 条）。
- `npm test`：63/63 通过；规格审核阶段的 `npm run build` 退出码 0，且当时未产生生成物漂移。实现阶段随后更新了 Markdown 生成物，当前工作树还包含这些已授权的实现与生成资源改动。
- `git diff --check`：通过（文档修订前后均检查）。

## 实现复核补充（2026-08-30）

- 已将 `public/md/main-v2.js` 中剩余的第一方 jQuery focus 简写改为显式 `.trigger('focus')`，并用锁定的 esbuild 重新生成 `public/md/main-v2.min.js`；随后 `npm run build` 再生 `public/js/markdown-v2.min.js`。
- 本地复核通过：`npm test` 63/63、`go test ./app/controllers ./app/db ./app/tests/harness/...`、`task.py validate 08-25-jquery-upgrade`、`git diff --check` 和 Node 语法检查均通过。
- 规格审核阶段未运行真实 Docker/MongoDB/Revel/Playwright E2E；该阶段不以缺失环境证据宣称通过。实现阶段的真实 E2E 证据应以同一任务的最新实现记录和复核输出为准。
- 真实 Safari 与前一主版本浏览器 smoke 仍是 AC-jQ9 的外部发布阻断项；本地环境未伪造其通过证据。
