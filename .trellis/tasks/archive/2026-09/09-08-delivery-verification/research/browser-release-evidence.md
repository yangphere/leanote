# 浏览器发布证据来源图

本任务的浏览器证据契约由以下仓库入口共同定义，执行者必须直接读取并运行对应入口，不能只依据 PRD 摘要：

- `scripts/browser-release-evidence.mjs`：四浏览器产品、current/previous 两个槽位、四个 coverage ID、八条 matrix record、provenance 和 JCS digest 校验。
- `scripts/validate-browser-artifact.mjs`：发布 artifact 的 schema、commit/run/attempt 绑定和跨文件校验入口。
- `tests/js/release-contract.test.js`：matrix、coverage summary、provenance 和失败路径的 Node contract 回归。
- `.github/workflows/browser-release-evidence.yml`：受保护真实浏览器 workflow、artifact 生成和失败阻断条件。
- `.trellis/spec/frontend/type-safety.md` 与 `.trellis/spec/frontend/directory-structure.md`：validator/test 约定和四个稳定 coverage ID 的项目规范。

脚本、测试和 workflow 证明当前实际行为，已确认 PRD/design 决定验收目标；两者不一致时记录实现缺口或规格冲突并归属 owner，不能自动以较弱实现覆盖需求。缺失、未运行或 artifact 不完整均不通过。

## 2026-09-29 审核澄清

- 固定 coverage 顺序：`business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`；槽位为 `current_major`/`previous_major`。每槽真实浏览器完整版本、命令版本和产品来源需可追溯，schema/marker 不是实际执行的独立证明。
- `workflow_dispatch` 的 tag-precheck 只用于候选验收；final 由 Release 的 `workflow_call` 重新执行，artifact 必须绑定当前 release run/attempt。`validate-browser-artifact.mjs:55-65` 明确区分两者，禁止混用。
- Q-DV1 受控恢复保留原 final artifact 与原发布 run/attempt 的绑定，另记 recovery 执行来源；它不把原 artifact 伪装成新 final，也不消费 precheck 代替 final。具体目标合同见本任务 design §5；当前实现尚未增加 recovery 模式。
- `browser-release-matrix-v1` 仅两个文件：`release-matrix.json`、`provenance.json`。不要为补证据随意放入第三个文件或原始日志，schema 扩展须同步唯一 producer/validator。
- protected runner 的八条 `BROWSER_SMOKE_COMMAND_<PRODUCT>_<SLOT>` 必须运行真实产品，不能只 echo passed markers；环境缺失保持 blocked。本轮未启动任何 browser 或 runner。
- 四项 smoke 不关闭 publishing/admin/notes/presentation 的全部业务验收；完整交接与故障项见本任务 evidence matrix 和上游明细。
