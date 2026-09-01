# B-E5 执行真实四浏览器 current/previous 矩阵

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E5　优先级：P1　执行序号：5　责任 owner：browser evidence

## Goal

在真实 Chrome、Edge、Firefox、Safari 的当前与前一主版本执行完整四项 coverage，生成八槽位唯一、不可变、脱敏且与运行 provenance 严格绑定的浏览器证据。

## Dependencies And Order

- 前置：B-E4；没有合规 suite、producer 和 validator 时不得开始矩阵运行。
- 该任务完成后才可进入 B-E6 的最终 package/container/release 收口。

## Requirements

1. 支持矩阵固定为四个真实产品 × current/previous 两个相邻主版本；Safari 只接受真实 Safari，不接受 WebKit、UA 伪装或其他浏览器替代。
2. 每个槽位按固定顺序执行 `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`，覆盖身份预检、错误/资源门禁、owned-resource 4xx/5xx、写入清理和编辑器语义。
3. 每条记录必须绑定不可变 commit/ref、producer workflow、run/attempt、版本、gate 全部 `passed` 和 `coverage_summary_sha256`；矩阵恰好 8 条且键唯一。
4. provenance 的 coverage summaries 必须脱敏，不含密码、Cookie、token、页面正文、用户数据、trace、截图、视频或原始日志；artifact 保留不超过 7 天。
5. 严格校验 JCS digest、记录原始失败，不覆盖失败 artifact，不忽略退出码或清理失败。

## Acceptance Criteria

- [ ] Chrome、Edge、Firefox、Safari current/previous 共 8 个槽位均使用真实产品并完整通过四项 coverage。
- [ ] `browser-release-matrix-v1` 预检恰好 8 条唯一记录，tag commit/ref/run/attempt 与 coverage summaries 一致，且 tag commit 等于候选 SHA；预检无 Release/GHCR 副作用。
- [ ] 每个槽位 discovered/executed 均为正整数且 executed 不超过 discovered，JCS 重算 digest 与矩阵行一致。
- [ ] 证据只包含脱敏摘要和 provenance；缺浏览器、缺外部 runner 或任一失败时状态为 `blocked`，并保留 owner/retest。

## Out Of Scope

- 不修改浏览器业务实现、测试套件或 workflow producer（由 B-E4/原 owner 负责）。
- 不把候选矩阵冒充最终 tag 发布 artifact。

## Handoff And Retest

将八槽位矩阵和预检运行 ID 交给 B-E6；任一槽位重跑都生成新 attempt，不覆盖原失败证据。
