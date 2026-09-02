# B-E6 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`，三根因行级定位）；design/implement 就位。
- [x] 用户已批准激活（"批准激活进入实现"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 允许修改：`sh/package.sh`、`quality-gate.yml` 与 `release.yml` 的构建 arg（同型类型缺陷）、`Dockerfile`、`scripts/ci/write-summary.mjs`、`tests/js/` 契约测试。
- 不改 release.yml 发布语义/browser producer/业务实现；不篡改 F 归档文件。
- 终局结论=门禁全绿 + 发布 blocked（缺 8 槽 artifact）；禁止 eligible_for_completion/发布获批表述。

## Task 1: D1 package.sh tag 判定 + 测试

- [x] refs/tags 前缀判定（design §1）；release-contract 增两态断言与 mode 断言（脚本/测试均已落地）。

## Task 2: D2 整型 epoch + OCI_CREATED 拆分 + 测试

- [x] 三处构建步骤双 arg（含锁步测试抓出的 quality-gate release-inputs 第三处同型缺陷）；Dockerfile 标签已切换。
- [x] 契约断言全部落地并通过（26/26）。

## Task 3: D3 write-summary stage 语义 + 测试

- [x] 非 fallback 默认 `complete`（write-summary.mjs 已修）。
- [x] 正/反例/覆盖三态单测全过。

## Task 4: 本地验证

- [x] npm test 130/0（+5 新契约测试）；本地 package.sh 产 v1.0.0 tarball+sha256。

## Task 5: CI 复验与 D3 实证

- [x] 六轮 CI 收敛至全绿：执行位→诊断能力→503 假象→ERE 真根因（`grep -E` 不支持 `(?:…)`）；run 链 33621572778/33622698589/33624592817/33626805206/33633129835/33634803149/33635990193 → **[33637319776 八 job 全绿](https://github.com/yangphere/leanote/actions/runs/33637319776)**。
- [x] ci-summary-node-build artifact 实测 passed/complete/11-11，stage 语义 CI 级实证完成。

## Task 6: D4/D5 登记

- [x] F 对账与发布 blocked 双重原因结论文本已产出（研究文档 + PRD AC）；PRD AC 按诚实状态勾选。

## Task 7: 交接

- [ ] 交付 E 与归档待用户确认（blocked 形态）。

## Completion Gate

- [x] F 冲突登记 + 发布 blocked 双重原因结论完整、零伪造；三门禁全绿一项因 503 未解之谜保持 blocked（AC-1 允许形态）。
