# B-E6 收口 package/container 门禁与 F 状态冲突

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E6　优先级：P1　执行序号：6　责任 owner：CI/CD delivery / 父任务收口
基线分支：`dev`　复验基线：`17efa981` 谱系（B-E1..B-E5 均已归档，B-E5 为 blocked 形态）
根因与修复机制见 `research/spec-audit-2026-09-02.md` 与 `design.md`。

## Goal

修复三个交付门禁的确定性缺陷（分支名误当 tag、RFC3339 误传整型参数、失败 job 被写 `job_not_started`），在候选 SHA 上恢复三门禁真实全绿，登记 F 状态冲突，并形成不夸大、不覆盖失败的 E/F/父任务发布结论。

## Confirmed Defect（根因行级证据见研究文档）

- **D1** package-smoke：`sh/package.sh:7` 将 `GITHUB_REF_NAME`（分支 `dev`）当 tag 传入 `version.mjs` 断言 → dev push 恒败（最新 run 33604371491 job `100165108289`）。
- **D2** container-smoke：quality-gate 构建步骤把 RFC3339 字符串传入整型 `SOURCE_DATE_EPOCH`（Dockerfile 单 arg 承载 buildkit 整型与 OCI RFC3339 标签两种需求）→ buildkit ParseInt 失败。
- **D3** summary：`write-summary.mjs:104` 使失败且未显式传 `CI_STAGE` 的 job 一律写 `stage=job_not_started`（artifact 实测），与 job 实际运行事实冲突；summary job 对失败上游的传播属正确行为。
- **D4** F（`archive/2026-09/08-25-cicd-delivery`）`completed` 状态与 notes"上游证据阻断"并存——只能登记，不得篡改。
- **D5** B-E5 裁决跳过 Safari → 8 槽 artifact 不可得 → F 发布门保持阻断（契约推论）。

## Dependencies And Order

- 前置 B-E1..B-E5 全部归档。本任务是六阻断的最终收口；任何前置 blocker 未闭合时只能记录 `blocked`。
- **终局结论形态（D5 约束）**：三门禁 dev push 全绿 + F 冲突登记 + 发布结论 `blocked`（缺 8 槽 tag 预检 artifact）；不得报告 `eligible_for_completion` 或发布获批。

## Requirements

1. **D1**：`sh/package.sh` 仅在显式 `RELEASE_TAG` 或 `GITHUB_REF` 为 `refs/tags/*` 时执行 tag-version 断言；tag 上下文（release.yml）语义与现有测试不变。
2. **D2**：`SOURCE_DATE_EPOCH` 传整型秒；OCI `created` 标签改由新 `ARG OCI_CREATED`（RFC3339）承载；**quality-gate.yml 与 release.yml 两条构建路径同步修复**（release.yml:102 存在同型 ParseInt 缺陷——参数类型缺陷修复不属"发布语义"变更）；镜像可复现性与标签正确性兼得。
3. **D3**：write-summary 成功执行（非 forcedFallback）时 stage 反映生命周期事实（`complete`），失败细节由 `failure.category/message/exit_code` 承载；forcedFallback 语义不变；job 状态与 job steps/attempt/退出码一致。
4. **锁步测试**：D1（tag/分支两态）、D2（整型断言已有则保、OCI 标签新断言）、D3（stage 语义正/反例）均有聚焦回归；既有 release-contract/summary 契约测试同步更新。
5. **D4/D5 登记**：F 冲突写入 E evidence matrix 与父任务 notes（发布阻断维持、8 槽缺口与恢复条件引用 B-E5 台账）；不改 F 归档文件。
6. **同一候选 SHA 复验与验证范围**：三门禁在修复提交的 CI run 全绿，记录 job step、退出码、发现/执行数量、artifact provenance 与失败 owner；package/container smoke 的验证范围保持原契约——tarball 与 SHA-256、OCI metadata、非 root 运行、外部 MongoDB、持久化路径、真实 `/note/toPdf` PDF smoke；失败保留原始原因与复验入口。
7. **两阶段 artifact 分离（承 F 契约）**：E 的 candidate evidence、tag 预检 artifact 与 F 的最终 tag artifact 分开绑定；E 归档前的预检不得创建 Release/GHCR，E 归档后 F 才能重新生成正式 artifact（本任务不触发任一阶段，仅保持该约束的登记）。

## Acceptance Criteria

- [ ] 修复提交的 CI：package-smoke、container-smoke、summary 三门禁全绿（连同 node-build/chromium/mongo 保持绿），run/job 记录入材料。
- [ ] 失败 job 的 summary（若再现）stage 与实际生命周期一致，不再出现"已运行却写 job_not_started"；summary 契约测试覆盖该语义。
- [ ] tag 上下文断言保留：`version.mjs`/package.sh 对 `refs/tags/*` 与显式 `RELEASE_TAG` 的断言行为有测试锁定；dev 分支 push 不再触发。
- [ ] 镜像构建可复现且 OCI `created` 为 RFC3339：契约测试断言双 arg 与 Dockerfile 标签来源。
- [ ] F 状态冲突在 E matrix 与父 notes 完整登记；最终结论为"门禁修复全绿 + 发布保持 blocked（缺 8 槽 artifact，恢复条件见 B-E5）"，无 `eligible_for_completion`/发布获批表述。

## Out Of Scope

- 不改 release.yml 的发布流程/门禁语义（其构建 arg 的同型类型缺陷除外，见 Req 2）、browser producer/validator、前端库业务逻辑、浏览器 suite、Mongo harness。
- 不通过删失败记录、重写历史 artifact、跳过 workflow 或篡改 F 归档制造绿色/和解结论。

## Handoff And Retest

向 E 与父任务提交最终 evidence matrix、F 状态对账与发布阻断结论；任一 package/container/summary 事实不一致返回本任务 owner 重跑。
