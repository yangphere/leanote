# B-E5 执行真实四浏览器 current/previous 矩阵

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E5　优先级：P1　执行序号：5　责任 owner：browser evidence
基线分支：`dev`　复验基线：`17efa981` 谱系（B-E1..B-E4 均已归档）
执行序列与 runner 命令契约见 `design.md`，环境证据见 `research/spec-audit-2026-09-02.md`。

## Goal

在真实 Chrome、Edge、Firefox、Safari 的当前与前一主版本执行完整四项 coverage，生成八槽位唯一、不可变、脱敏且与运行 provenance 严格绑定的浏览器证据，经 tag 预检入口交付 E 验收。

## Environment Readiness Gate（Task 0，先于一切执行）

以下两项获用户明确答复前，本任务保持 `planning`，不激活、不运行矩阵：

- **Q-E5-1 执行环境**：(a) 完整契约路径（macOS 自托管 runner + 4 产品×2 主版本 + `BROWSER_SMOKE_COMMAND_*`，含 Safari 的 safaridriver 方案）；(b) 分步路径（先非-Safari 六槽位，Safari 待 macOS）；(c) 暂缓（blocked 登记）。证据：仓库 runner `total_count=0`、证据 workflow 零运行、Playwright 无真实 Safari channel。
- **Q-E5-2 tag 策略**：首个严格 `vX.Y.Z` 版本号与创建时机（预检 tag 指向候选 SHA；E 归档后 F 终 release run 复用同 tag 重新生成 artifact——Q-E1 已定）。

环境缺失时的交付形态：材料就绪 + blocked 登记（owner/retest），不视为失败交付（对齐 E design §4 与本任务 AC-4）。

## Dependencies And Order

- 前置 B-E4 已归档：合规 suite/producer/validator/预检入口齐备（16 用例/4 文件、marker 协议、双相位 validator）。
- 本任务完成后才可进入 B-E6 的最终 package/container/release 收口。

## Requirements

1. 矩阵固定为四个真实产品 × current/previous 相邻主版本；Safari 只接受真实 Safari（macOS + safaridriver 或等价真实驱动），不接受 WebKit、UA 伪装或替代产品。
2. 每槽位按固定顺序执行 `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe` 四项 coverage（即 B-E4 的 browser-smoke 四套件），覆盖身份预检、错误/资源门禁、owned-resource、写入清理与编辑器语义；`BROWSER_SMOKE_COMMAND_*` 必须满足 design §3 的 marker 契约（fail-closed）。
3. 执行序列按 design §2：候选 SHA → 用户建严格 tag → workflow_dispatch 预检 → 下载两文件 artifact → `validate-browser-artifact.mjs --phase precheck --expected-commit <候选>` → 交付 E；全程无 Release/GHCR/F 状态副作用。
4. 每条记录绑定不可变 commit/ref、producer workflow、run/attempt、完整版本、gate 全 `passed` 与 `coverage_summary_sha256`；矩阵恰好 8 条且产品/slot 唯一。
5. 脱敏与保留：artifact 只含契约 allowlist 字段，不含凭据/Cookie/正文/trace/截图/视频/原始日志；保留 ≤7 天；JCS 重算必须通过；失败记录原始保留，重跑生成新 attempt，不覆盖、不忽略退出码或清理失败。

## Acceptance Criteria

- [ ] （环境就绪后）Chrome、Edge、Firefox、Safari current/previous 共 8 槽位真实产品执行，四项 coverage 完整通过。
- [ ] `browser-release-matrix-v1` 预检恰好 8 条唯一记录；tag commit/ref/run/attempt 与 coverage summaries 一致且 tag commit 等于候选 SHA；预检无发布副作用（workflow 结构断言已由 B-E4 锁定）。
- [ ] 每槽位 discovered/executed 正整数且 executed≤discovered；JCS 重算 digest 与矩阵行一致（validator precheck 相位全绿）。
- [x] blocked 形态交付（2026-09-02）：用户裁决跳过 Safari 后，本机真实产品三 current 槽位（Chrome 152/Edge 152/Firefox 153）四套件 16/16 全绿（工程证据台账，jQuery 文档先例）；Safari×2 用户跳过、previous×3 缺旧版二进制，均登记 owner/retest（acceptance/engineering-evidence.md）；未伪造、未以部分槽位冒充 8 槽证据，E AC-E6 保持 blocked。

## Out Of Scope

- 不修改浏览器业务实现、测试套件、producer/validator/workflow（B-E4/原 owner 负责；发现缺陷只登记返回）。
- 不把候选/预检矩阵冒充最终 tag 发布 artifact；不更新 `docs/modernization/browser-smoke/*.md` 为发布真相（仅历史台账，E 收口引用）。

## Handoff And Retest

将八槽位矩阵、预检 run ID 与 validator 输出交给 E（AC-E6）与 B-E6；任一槽位重跑生成新 attempt，不覆盖原失败证据。
