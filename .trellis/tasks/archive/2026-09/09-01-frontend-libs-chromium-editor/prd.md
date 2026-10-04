# B-E3 修复 Chromium 编辑器 iframe 失败与清理超时

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E3　优先级：P0　执行序号：3　责任 owner：TinyMCE / E2E
基线分支：`dev`　复验基线：`d37e3b8` 谱系（B-E1/B-E2 已归档）
技术设计见 `design.md`，证据链见 `research/spec-audit-2026-09-02.md`。

## Goal

消除 Chromium business 的两处普遍失败（leaui_image 契约测试的 `editor.on is not a function`、大业务流的 `#attrTitle` 不可见超时），并按复验协议处置第三处 CI 级联失败，使 Chromium 门禁提供完整、可追溯的 22/22 通过证据。

## Confirmed Defect

- CI [run 33579336426 / chromium-e2e job `100090357472`](https://github.com/yangphere/leanote/actions/runs/33579336426/job/100090357472)：build-smoke 1/1；business 19 过 / 3 败。
- **失败 1**（`business-flows.spec.mjs:84`，本地复现）：受控 editor shell 未实现 TinyMCE 事件 API，插件工厂 `plugin.js:152` 的合法 `editor.on('dragstart')` 抛 `TypeError`。生产运行时无兼容问题（真实编辑器全量跑通，见研究文档）。
- **失败 2**（`business-flows.spec.mjs:162`，本地同点位 spec:470 复现）：`index.html` 硬编码 `<body class="md">` 命中 `style.css` 的 `.md #previewAttrs{display:none}`，富文本图片对话框属性面板被隐藏——真实生产回归（原设计为 URL `md=1` 条件类；markdown 编辑器走 `/album/index?md=1` 另一页面）。
- **失败 3**（`editor-flows.spec.mjs:69`，本地**通过**）：CI 主体跑到清理、`deleteNote` POST 处 180s 预算耗尽——判定为失败 2 的 CI 级联 + 慢机预算，非独立缺陷。
- CI 的 `[business-e2e] cleanup failed` 系列均为失败 2 超时后上下文销毁的次生产物。

## Dependencies And Order

- 前置 B-E1、B-E2 已归档；本任务在 `d37e3b8` 谱系上实施。
- 本任务完成后，B-E4 才能把四项 coverage 的缺口与编辑器运行时错误分离验证。

## Requirements

1. **shell 保真（失败 1）**：契约测试的受控 editor shell 补最小事件 API，使真实插件工厂的 `editor.on('dragstart')` 与按钮 `onSetup` 的订阅/退订路径可执行并被断言；不得改生产 `plugin.js` 去迁就 shell，也不得以 try/catch 吞掉工厂异常。
2. **对话框条件类（失败 2）**：恢复 `index.html` 的 `md` 条件类原设计（URL 含 `md=1` 才应用）；standalone 与富文本对话框显示 `#previewAttrs`，`/album/index?md=1` 的 markdown 页面行为不变。
3. **既有契约保持**：22 用例既有断言不放宽——未编辑零写入、失败不确认 revision、HTML 语义、公开 URL/API、身份/所有权/console/page 门禁原样；不得放宽超时、忽略退出码或跳过清理。
4. **失败 3 复验协议**：按 design.md §4 执行序处置；无论结果如何，不得以调大 timeout、忽略退出码或跳过清理关闭。
5. **聚焦回归**：为两处修复各附最小断言——shell 事件 API 的订阅/退订（机制形态见 design.md §2）；`#attrTitle` 非 md 上下文可见且 md 上下文仍隐藏（断言承载见 design.md §3）。
6. **记录**：候选 SHA、本地与 CI 的发现/执行数、失败清单、清理结果与 run/job provenance。

## Acceptance Criteria

- [x] 本地 supervisor 串行协议全绿：build-smoke + business 22/22（最终轮 41.1s）；无 `editor.on`、无可见性超时、无清理失败。
- [x] [run 33589413738 / job `100120419936`](https://github.com/yangphere/leanote/actions/runs/33589413738/job/100120419936)（`7f0a0ba2`）chromium-e2e success：build-smoke 1/1（3.2s）先行 + business 22/22（38.9s），清理摘要完整。E allowlist 三 job（node-build/chromium-e2e/mongo-8_0）首次同轮全绿。
- [x] shell `on/off` + dragstart 订阅与 onSetup 订阅/退订闭环断言；`#attrTitle` 非 md 可见 + `md=1` 下 body 类与隐藏双向断言；生产 `plugin.js` 零改动（diff 仅 common.js 守卫/index.html 条件类/main.js data: 分支与测试）。
- [x] 失败 3 经三轮诊断轮（`46caf2aa`/`29bb285d`/`aece2691`）定位真实根因——`common.js:414` undoManager 初始化竞态抛错致加载遮罩永不隐藏（审核的"CI 级联"假设机制有误，已在研究文档修正）；等行数守卫修复，未调任何超时，诊断脚手架已剥离；CI 全绿关闭。

## Out Of Scope

- 不新增 Bootstrap coverage 或真实四浏览器矩阵（B-E4/B-E5）。
- 不修改 Mongo service/harness 选择规则（B-E2 已收口）。
- 不改 `/album/index` 页面及其 `md=1` 行为；不动 22 用例之外的覆盖面。

## Handoff And Retest

将稳定的 Chromium 运行摘要（22/22 + 清理证据）交给 B-E4 作为 coverage producer 的输入；任何错误隐藏、清理失败或仅局部重跑都不能作为完成证据。
