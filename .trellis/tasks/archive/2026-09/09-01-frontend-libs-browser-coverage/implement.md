# B-E4 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`）；design/implement 按 workflow.md 复杂任务护栏就位。
- [x] B-E3 已归档（126f1cb7）；用户已批准激活（"归档并批准激活"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 实现阶段允许修改 design §1 所列六文件；不执行真实四浏览器、不改受保护 runner 环境、不动 E 验收矩阵。
- business 用例内容与数量（22）零变化，仅迁移文件归属；不伪造任何浏览器执行结果。
- 提交在 `dev`，每提交聚焦单一目的。

## Task 1: 套件迁移与发现

- [x] 迁移完成；business 22/6、browser-smoke 16/4 实测。
- [x] testMatch 四文件；--list 计数如上。
- [x] 本地 supervisor 22/22（39.8s）。

## Task 2: JCS 规范化器与单测

- [x] scripts/jcs.mjs + 4 单测（排序/域拒绝/手算摘要/篡改敏感）。

## Task 3: producer 升级

- [x] parseCoverageMarkers 落地（fail-closed：缺 marker/计数/标识符非法即拒）。
- [x] producer 全面升级；rebuild 模式（source+summaries）保持 provenance 契约完整。

## Task 4: validator 双相位升级

- [x] crossValidateBrowserEvidence 导出共享校验。
- [x] 双相位 CLI 落地并有相位契约测试。

## Task 5: 预检 workflow 入口

- [x] 预检入口落地；结构断言测试锁定。

## Task 6: 锁步契约测试

- [x] 26/26（升级三用例 + 新增摘要/相位/预检结构三用例）。

## Task 7: 本地与 CI 验证

- [x] npm test 0 失败；本地 22/22。
- [x] [run 33601564498](https://github.com/yangphere/leanote/actions/runs/33601564498)：node-build + chromium-e2e + mongo-8_0 全部 success（`c8c7518e`）。

## Task 8: Provenance 与交接

- [x] PRD AC 全勾并附 provenance；实现提交 `c8c7518e`、材料 `206cc727`。E 同步项与 B-E5 输入已写入 Handoff；归档待用户确认（待归档时同步 E notes）。

## Completion Gate

- [x] 差异清单为空；未执行任何真实浏览器矩阵（B-E5 范围）。
