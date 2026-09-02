# B-E3 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`，CI+本地双源证据）；design/implement 按 workflow.md 复杂任务护栏就位。
- [x] 用户已批准激活（"批准激活"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 实现阶段允许修改 `tests/e2e/business/business-flows.spec.mjs`、`public/tinymce/plugins/leaui_image/index.html`（及可选 Node 契约测试）；生产 `plugin.js` 零改动。
- 不放宽任何既有断言/超时；不忽略退出码；清理失败单独报告。
- 复验基线 `d37e3b8` 谱系；提交在 `dev` 上，每提交聚焦单一目的。

## Task 1: shell 事件 API（失败 1）

- [ ] shell 补 `on/off`（空格分隔事件名、订阅存储、退订语义）。
- [ ] 断言：工厂执行后 `dragstart` 已订阅；`onSetup` 订阅/退订闭环（design §2）。
- [ ] 本地单测该 spec 文件相关用例通过。

## Task 2: 对话框条件类（失败 2）

- [ ] `index.html` 恢复 `md=1` 条件类（design §3）；`#previewAttrs` 非 md 可见。
- [ ] 回归断言（design §3）：非 md `#attrTitle` 可见可填；`index.html?md=1` 下 body 类为 `md` 且 `#previewAttrs` 隐藏。

## Task 3: 本地全量复验

- [ ] `go run ./app/tests/harness/cmd/e2e -- sh -c 'npm run test:e2e:build && npm run test:e2e'` → build-smoke 1/1 + business 22/22，无 `editor.on`、无可见性超时、无清理失败。

## Task 4: CI 复验与失败 3 协议

- [ ] 提交推送，CI `chromium-e2e` job 全绿（22/22 + 清理摘要 + run/job 记录）。
- [ ] 若 editor-flows 仍败：按 PRD Req 4 采集证据、登记独立事实，不放宽超时。

## Task 5: Provenance 与交接

- [ ] PRD AC 勾选并附本地/CI provenance。
- [ ] 通知 E：AC-E4 的 chromium 行 retest 输入就绪；B-E4 以本任务输出为前提；归档需用户确认。

## Completion Gate

- [ ] PRD 全部 AC 勾选；生产 plugin.js 零改动；22 用例断言原样。
