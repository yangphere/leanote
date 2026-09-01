# B-E4 建立四项浏览器 coverage 与 release artifact 契约

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E4　优先级：P1　执行序号：4　责任 owner：Bootstrap / E2E / release producer-validator

## Goal

补齐 `browser-smoke` 的 Bootstrap 组件套件，并把每个浏览器槽位的四项语义 coverage 与候选、tag 预检、正式发布 artifact 绑定为同一套可验证契约。

## Confirmed Defect

- 当前 `browser-smoke` 仅发现 `business-flows`、`editor-flows` 两个文件，共 6 个用例，缺少 Bootstrap suite。
- 现有 producer/validator 仍使用通用 `scope`，没有四项 coverage summary、JCS digest 和严格 run/attempt 绑定；当前 release 流程也没有独立的“仅预检、不发布”路径。

## Dependencies And Order

- 前置：B-E1、B-E2、B-E3。
- 本任务完成后才允许 B-E5 执行真实八槽位矩阵；本任务不伪造任何浏览器结果。

## Requirements

1. `browser-smoke` 必须发现并执行稳定 ID：`business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`，并对 Bootstrap modal/tab/dropdown/tooltip/alert、BootstrapDialog、远程 modal 和主题加载建立可重复套件。
2. 每个槽位 summary 固定字段 `id`、`discovered_count`、`executed_count`、`entrypoints`、`iframes`、`result`；计数为正整数且 executed 不大于 discovered，结果只能为 `passed`。
3. 对去除 digest 的 `{browser_product, release_slot, items}` 输入使用 RFC 8785 JCS 计算小写 SHA-256；validator 必须重算并校验 coverage 顺序、槽位、commit、run/attempt 和矩阵原始字节 digest。
4. 等待模式的 tag 预检只能上传两文件、不得创建 Release/GHCR 或修改 F 状态；E 归档后 F 最终 release run 必须重新生成正式 artifact。候选与 tag commit 必须分列且解析为同一 SHA。
5. artifact 只保存脱敏 allowlist 字段，保留期不超过 7 天；旧通用 `scope` 或缺 digest 载荷一律拒绝。

## Acceptance Criteria

- [ ] Playwright list/执行证据证明四个 coverage ID 均被发现并执行，且没有缩小发现范围。
- [ ] validator 拒绝重复槽位、非相邻版本、错误 commit/ref/run/attempt、非 JCS digest、敏感字段和 tag/候选身份混用。
- [ ] 受保护 tag 预检和最终发布 artifact 的生命周期、两文件 allowlist、禁止发布副作用均有自动化验证。
- [ ] 产物 schema、producer、validator 与 E/F PRD/研究材料一致，并提供可复验命令和 owner/retest 入口。

## Out Of Scope

- 不执行真实四浏览器八槽位（B-E5）。
- 不修复 TinyMCE 运行时或 Mongo harness（B-E2/B-E3）。

## Handoff And Retest

向 B-E5 提供稳定 suite、schema 和摘要生成器；任何 coverage 缺失或 artifact 交叉校验失败都保持 blocker，不得用历史矩阵替代。
