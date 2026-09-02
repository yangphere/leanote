# B-E4 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`）；design/implement 按 workflow.md 复杂任务护栏就位。
- [x] B-E3 已归档（126f1cb7）；用户已批准激活（"归档并批准激活"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 实现阶段允许修改 design §1 所列六文件；不执行真实四浏览器、不改受保护 runner 环境、不动 E 验收矩阵。
- business 用例内容与数量（22）零变化，仅迁移文件归属；不伪造任何浏览器执行结果。
- 提交在 `dev`，每提交聚焦单一目的。

## Task 1: 套件迁移与发现

- [ ] 新建 `leaui-image-iframe.spec.mjs`，迁移 business-flows:84 的独立 leaui 契约用例（design §2；:187 内嵌段不动）。
- [ ] browser-smoke testMatch 扩四文件；`--list` 证明四 ID 文件发现、business 6 文件/22 用例。
- [ ] 本地 supervisor 串行协议复跑 business 全绿（迁移无回归）。

## Task 2: JCS 规范化器与单测

- [ ] 实现 JCS 最小规范化器（design §4）+ RFC 8785 向量子集与契约载荷正/反例单测。

## Task 3: producer 升级

- [ ] marker 协议扩展解析（design §3，合成 stdout 单测）。
- [ ] 矩阵四 ID 固定顺序 + 每行 `coverage_summary_sha256`；provenance 八槽位 `coverage_summaries`；原始字节 matrix_sha256 不变语义。

## Task 4: validator 双相位升级

- [ ] schema 升级（coverage_summaries 必含、旧六字段拒绝、通用 scope 拒绝）。
- [ ] JCS 重算与交叉校验；`--phase final|precheck` 与 `--expected-commit`（design §5）。

## Task 5: 预检 workflow 入口

- [ ] `workflow_dispatch` + 严格 tag 断言 + 剥壳解析 + 无发布步骤（design §6）。

## Task 6: 锁步契约测试

- [ ] 升级 release-contract.test.js 四用例；新增 JCS/结构/相位断言（PRD Req 6）。

## Task 7: 本地与 CI 验证

- [ ] `npm test` 全绿；本地 supervisor business 22/22。
- [ ] 候选 CI `node-build`（承载契约测试）通过；记录 run/job 与 `--list` 计数。

## Task 8: Provenance 与交接

- [ ] PRD AC 勾选并附 provenance；通知 E：AC-E6 producer/validator 行与预检入口 retest 输入就绪，"5 files"措辞待 E 同步；B-E5 以四套件+marker 协议为输入；归档需用户确认。

## Completion Gate

- [ ] PRD 全部 AC 勾选；与 F release-matrix-contract 差异清单为空；无浏览器结果伪造。
