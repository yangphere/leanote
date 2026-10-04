# B-E3 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`，CI+本地双源证据）；design/implement 按 workflow.md 复杂任务护栏就位。
- [x] 用户已批准激活（"批准激活"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 实现阶段允许修改 `tests/e2e/business/business-flows.spec.mjs`、`public/tinymce/plugins/leaui_image/index.html`（及可选 Node 契约测试）；生产 `plugin.js` 零改动。
- 不放宽任何既有断言/超时；不忽略退出码；清理失败单独报告。
- 复验基线 `d37e3b8` 谱系；提交在 `dev` 上，每提交聚焦单一目的。

## Task 1: shell 事件 API（失败 1）

- [x] shell 补 `on/off`（空格分隔事件名、订阅存储、退订语义）。
- [x] 断言：工厂执行后 `dragstart` 已订阅；`onSetup` 订阅/退订闭环（design §2）。
- [x] 本地与 CI 该 spec 全部通过。

## Task 2: 对话框条件类（失败 2）

- [x] `index.html` 恢复 `md=1` 条件类（design §3）；`#previewAttrs` 非 md 可见。
- [x] 回归断言（design §3）：非 md `#attrTitle` 可见可填；`index.html?md=1` 下 body 类为 `md` 且 `#previewAttrs` 隐藏。

## Task 3: 本地全量复验

- [x] 本地三轮全绿（最终 41.1s）；实现期另修复两个暴露的潜伏缺陷（测试读重置变量 / main.js data: URL 分支，见研究文档补录）。

## Task 4: CI 复验与失败 3 协议

- [x] [run 33589413738](https://github.com/yangphere/leanote/actions/runs/33589413738/job/100120419936) 全绿（1/1 + 22/22，38.9s）。
- [x] editor-flows 首轮仍败（21/22）→ 三轮诊断轮定位 undoManager 竞态根因 → 守卫修复后全绿；全程未放宽超时。

## Task 5: Provenance 与交接

- [x] PRD AC 全勾并附 provenance；修复提交 `42a9025e`（主体）+ `7f0a0ba2`（根因守卫）；诊断轮 `46caf2aa`/`29bb285d`/`aece2691` 的脚手架已剥离、过程留档研究文档。
- [ ] 通知 E：AC-E4 的 chromium 行 retest 输入就绪；B-E4 以本任务输出为前提；归档需用户确认。

## Completion Gate

- [x] PRD 全部 AC 勾选；生产 plugin.js 零改动；22 用例既有断言与超时原样（仅新增断言与两处修复）。
