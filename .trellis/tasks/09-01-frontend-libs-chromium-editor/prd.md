# B-E3 修复 Chromium 编辑器 iframe 失败与清理超时

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E3　优先级：P0　执行序号：3　责任 owner：TinyMCE / E2E

## Goal

修复 `leaui_image` iframe 中 `editor.on is not a function` 的真实兼容错误，并恢复因此超时的主业务、编辑器流程及测试清理，使 Chromium 门禁能够提供完整、可追溯的通过证据。

## Confirmed Defect

- CI `33477561244` / attempt `1` 的 Chromium business 仅 `19/22` 通过；`leaui_image` mock/iframe 触发 `editor.on is not a function`，随后出现超时和清理失败。
- 该问题不能以 mock 成功、放宽超时、忽略退出码或吞掉 console/page 错误关闭。

## Dependencies And Order

- 前置：B-E1、B-E2。
- 本任务完成后，B-E4 才能把四项 coverage 的缺口与编辑器运行时错误分离验证。

## Requirements

1. 追踪 `leaui_image` iframe 与 TinyMCE 8.8.2 的事件 API 边界，修复真实运行时兼容，而不是增加第二编辑器 runtime 或仅修测试 mock。
2. 覆盖主笔记和会员博客两个编辑器入口的加载、插入、更新、undo/redo、保存、重载、只读、失败恢复和跨 iframe 清理。
3. 保留未编辑零写入、失败不确认 revision、HTML 语义和现有公开 URL/API 契约。
4. 测试失败时仍执行清理并单独报告清理结果；console、page、resource、unhandled rejection 和身份/所有权门禁均必须可见。

## Acceptance Criteria

- [ ] `build-smoke` 先于 `business` 执行并全绿；business 五个 suite 共 `22` 个用例全部发现并执行。
- [ ] `leaui_image` iframe 的身份预检、上传/选择/跨 iframe 插入、错误和清理通过，不再出现 `editor.on` 错误、超时或残留数据。
- [ ] 未编辑笔记没有保存请求；真实编辑成功/失败的 revision 与 HTML 语义符合夹具契约。
- [ ] 候选 CI `chromium-e2e` job 通过，且运行记录包含候选 SHA、发现/执行数量、退出码和清理结果。

## Out Of Scope

- 不新增 Bootstrap coverage 或真实四浏览器矩阵（由 B-E4/B-E5 负责）。
- 不修改 Mongo service/harness 选择规则（由 B-E2 负责）。

## Handoff And Retest

将稳定的 Chromium 运行摘要交给 B-E4 作为 coverage producer 的输入；任何错误隐藏、清理失败或仅局部重跑都不能作为完成证据。
