# B-E4 建立四项浏览器 coverage 与 release artifact 契约

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E4　优先级：P1　执行序号：4　责任 owner：Bootstrap / E2E / release producer-validator
基线分支：`dev`　复验基线：`7f0a0ba2` 谱系（B-E1/B-E2 归档、B-E3 完成待归档）
技术设计见 `design.md`，证据链见 `research/spec-audit-2026-09-02.md`；字段级唯一来源为 F 归档 `research/release-matrix-contract.md` 与 E design §3.3/§5。

## Goal

补齐 `browser-smoke` 的四项稳定 coverage 套件，并把 producer/validator/受保护 workflow 升级到四 ID、JCS 摘要、严格 provenance 的 v1 契约，为 B-E5 的真实八槽位矩阵与 E 的 tag 预检验收提供可验证基础。

## Confirmed Defect

- browser-smoke `--list` 实测仅 6 用例/2 文件（business-flows、editor-flows），缺 `bootstrap-components` 与 `leaui-image-iframe` 稳定 ID（playwright.config.mjs testMatch）。
- producer 硬编码通用 coverage（browser-release-evidence.mjs:110）且 matrix 校验接受 1-40 任意 scope（:69）；provenance 无 `coverage_summaries`（:134-139）；无 JCS 摘要。
- validator 的 provenance 键 allowlist 硬编码旧六字段（validate-browser-artifact.mjs:20）——**符合契约的 v1 载荷反被拒绝**；无摘要重算与双相位。
- `browser-release-evidence.yml` 仅 `workflow_call`，无"仅预检、不发布"的独立受控入口（E AC-E6 第 3 行 blocked 直接原因）；该 workflow 自身无发布副作用，发布在 release.yml。
- `tests/js/release-contract.test.js` 四用例（:118/:221/:298/:319）锁定旧契约，必须与实现同步升级。

## Dependencies And Order

- 前置：B-E1、B-E2 已归档；B-E3 已完成待归档——**本任务激活前需 B-E3 归档证据**。
- 完成后才允许 B-E5 执行真实八槽位矩阵；本任务不伪造任何浏览器结果，不执行真实四浏览器。

## Requirements

1. **四套件结构（文件-ID 一一对应）**：从 `business-flows.spec.mjs` **迁移**（不复制）其唯一的独立 leaui iframe 契约用例（:84）至新 `leaui-image-iframe.spec.mjs`（大业务流测试 :187 内嵌的 leaui 段不拆，见 design §2）；browser-smoke testMatch 扩为四文件；business 项目发现变为 6 文件/**22 用例数不变**；E 的 evidence-matrix AC-E4"22 tests/5 files"措辞与 E design §3.2"五个文件"枚举均由 E 收口阶段同步（登记为 E 侧 spec-sync 项）。
2. **槽位摘要契约**（producer 与受保护命令间的 marker 传输协议以 design §3 为准）：每槽位 `coverage_summaries` 恰好四项、固定顺序 `business-flows`/`editor-flows`/`bootstrap-components`/`leaui-image-iframe`；项字段 `id`/`discovered_count`/`executed_count`/`entrypoints`/`iframes`/`result`，计数为正整数且 executed≤discovered，`result` 只能 `passed`，`entrypoints` 非空（≥1）而 `iframes` 可为空，均为小写无换行稳定标识符（`^[a-z0-9][a-z0-9._/-]{0,79}$`，≤40 项）；矩阵每行携带对应 `coverage_summary_sha256`。
3. **JCS 摘要**：对去掉 digest 字段的 `{browser_product, release_slot, items}` 按 RFC 8785 计算小写 SHA-256；validator 必须重算并校验摘要、顺序、槽位、commit、run/attempt 与矩阵原始字节 digest（实现机制与向量锁定见 design §4）。
4. **validator 双相位**：正式相位（默认，release.yml 内）要求 producer run/attempt == 当前 run；预检相位（E 侧）校验 tag commit == 候选 SHA 与全部结构/摘要规则，不要求 run/attempt 等于校验者；旧六字段 provenance、通用 `scope`、缺 digest 载荷一律拒绝。
5. **受保护预检入口**：`browser-release-evidence.yml` 增加仅预检的 `workflow_dispatch` 入口；安全不变量为 `contents: read`、protected runner 标签、两文件 artifact、retention ≤7 天、无 Release/GHCR/F 状态副作用（身份解析与结构断言机制见 design §6）。E 归档后 F 最终 release run 以同一 producer 重新生成正式 artifact。
6. **锁步契约测试**：release-contract.test.js 四用例升级 + 新增：通用 scope 拒绝、coverage_summaries 结构正/反例、JCS 向量与载荷摘要、预检相位身份校验、预检 workflow 无发布步骤的结构断言。
7. **记录**：全部验证输出与 artifact 字段保持脱敏 allowlist；不新增第三文件。

## Acceptance Criteria

- [ ] browser-smoke `--list` 与执行证据证明四个 coverage ID 文件均被发现执行；business 发现 6 文件/22 用例（数量不缩水、用例不重复）。
- [ ] producer 产出的矩阵每行含固定顺序四 ID 与 `coverage_summary_sha256`，provenance 含恰好八槽位 `coverage_summaries`；validator（两相位）重算全部摘要并拒绝重复槽位、非相邻版本、错误 commit/ref/run/attempt、非 JCS digest、敏感字段与 tag/候选身份混用。
- [ ] 预检 workflow_dispatch 入口存在且结构断言证明：两文件 allowlist、retention ≤7 天、无 Release/GHCR/publish 步骤、身份取自剥壳 tag 而非输入。
- [ ] 锁步契约测试（含 JCS 向量）全部通过；`npm test` 全绿；候选 CI `node-build`（承载契约测试）通过。
- [ ] schema/producer/validator 与 E design §3.3、F release-matrix-contract 完全一致（差异清单为空），提供可复验命令与 owner/retest 入口。

## Out Of Scope

- 不执行真实四浏览器八槽位（B-E5）；不修改受保护 runner 环境自带命令（B-E5 执行期遵守扩展 marker 协议即可）。
- 不修复 package/container/summary（B-E6）；不改 E 验收矩阵（"5 files"措辞同步由 E 执行）。

## Handoff And Retest

向 B-E5 提供稳定四套件、schema、JCS 摘要生成器与 marker 协议；任何 coverage 缺失或 artifact 交叉校验失败都保持 blocker，不得用历史矩阵替代。
