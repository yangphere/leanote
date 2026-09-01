# B-E6 收口 package/container 门禁与 F 状态冲突

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E6　优先级：P1　执行序号：6　责任 owner：CI/CD delivery / 父任务收口

## Goal

在前置构建、测试和真实浏览器证据闭合后，重新验证 package/container 交付门禁，纠正 F 归档状态与阻断 notes 的事实冲突，并形成不夸大、不覆盖失败的 E/F/父任务发布结论。

## Confirmed Defect

- CI `33477561244` / attempt `1` 的 package/container 门禁失败；summary 还把实际执行过的 job 错写为 `job_not_started`。
- F 已归档为 `completed`，但其 notes 仍声称上游真实证据阻断；该冲突不能被“已归档”或父任务进度计数覆盖。

## Dependencies And Order

- 前置：B-E1、B-E2、B-E4、B-E5；B-E3 已由 B-E4 的执行前置间接覆盖。
- 这是六个阻断中的最终收口任务；任何前置 blocker 未闭合时只能记录 `blocked`，不得宣称发布获批。

## Requirements

1. 在同一候选 SHA 上重新运行 package/container 及 summary 门禁，记录真实 job step、退出码、发现/执行数量、artifact provenance 和失败 owner；禁止以 summary 占位状态替代 job 事实。
2. 校验 tarball、SHA-256、OCI metadata、非 root、外部 Mongo、持久化路径和真实 PDF smoke；失败必须保留原始原因并提供复验入口。
3. 对 F 的 `completed` 状态与阻断 notes 做事实 reconciliation：只有通过合法任务生命周期命令、重新验收和明确授权后才能重开/更新；不得直接篡改状态以消除冲突。
4. E 的 candidate evidence、tag 预检 artifact 和 F 的最终 tag artifact 必须分开绑定；E 归档前的预检不得创建 Release/GHCR，E 归档后 F 才能重新生成正式 artifact。

## Acceptance Criteria

- [ ] package/container/summary 门禁要么在候选 SHA 上真实通过，要么明确保持 `failed`/`blocked`，并列出 owner、run/job 和复验命令。
- [ ] summary 的 job 状态与实际 job steps、attempt 和退出码一致，不再出现错误的 `job_not_started` 结论。
- [ ] F 的状态、notes、正式 artifact 生命周期和父任务 DAG 叙述一致；任何无法合法修正的冲突仍作为发布阻断。
- [ ] 只有 B-E1～B-E5 全部证据满足且 E/F 两阶段 artifact 各自校验后，才允许报告 `eligible_for_completion` 或发布获批。

## Out Of Scope

- 不在本任务内修改前端库业务逻辑、浏览器 suite 或 Mongo harness。
- 不通过删除失败记录、重写历史 artifact、跳过 workflow 或 push 来制造绿色结论。

## Handoff And Retest

完成后向 E 和父任务提交最终 evidence matrix、F 状态对账和 release provenance；任一 package/container 或状态事实不一致都返回本任务 owner 重跑。
