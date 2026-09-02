# B-E6 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`，三根因行级定位）；design/implement 就位。
- [x] 用户已批准激活（"批准激活进入实现"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 允许修改：`sh/package.sh`、`quality-gate.yml` 与 `release.yml` 的构建 arg（同型类型缺陷）、`Dockerfile`、`scripts/ci/write-summary.mjs`、`tests/js/` 契约测试。
- 不改 release.yml 发布语义/browser producer/业务实现；不篡改 F 归档文件。
- 终局结论=门禁全绿 + 发布 blocked（缺 8 槽 artifact）；禁止 eligible_for_completion/发布获批表述。

## Task 1: D1 package.sh tag 判定 + 测试

- [ ] refs/tags 前缀判定（design §1）；release-contract 增两态断言。

## Task 2: D2 整型 epoch + OCI_CREATED 拆分 + 测试

- [ ] quality-gate.yml 与 release.yml 两处构建步骤双 arg；Dockerfile `ARG OCI_CREATED` 标签来源切换。
- [ ] 契约断言：SOURCE_DATE_EPOCH 不在标签行、双 arg 传递、标签引用 OCI_CREATED。

## Task 3: D3 write-summary stage 语义 + 测试

- [ ] 非 fallback 默认 `complete`（design §3）。
- [ ] 正/反例：失败+非 fallback→complete；CI_FORCE_FALLBACK→job_not_started；CI_STAGE 显式覆盖仍生效。

## Task 4: 本地验证

- [ ] `npm test` 全绿；本地 `sh sh/package.sh` 无 tag 上下文跑通（或记录限制）。

## Task 5: CI 复验与 D3 实证

- [ ] push 后三门禁 + 既有绿门禁全部 success；记录 run/job。
- [ ] 下载 package/container/summary 三 summary artifact，核对 stage/status 与 job 事实一致。

## Task 6: D4/D5 登记

- [ ] 产出 F 冲突对账与发布 blocked 结论文本（供 E matrix/父 notes 引用）；PRD AC 勾选。

## Task 7: 交接

- [ ] 交付 E（最终 matrix 输入）与父任务（发布结论）；归档需用户确认。

## Completion Gate

- [ ] 三门禁真实全绿 + summary 事实一致 + F 冲突登记 + 发布 blocked 结论完整；零伪造。
