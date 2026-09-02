# B-E4 规格审核与现状核实（2026-09-02）

## 结论

B-E4 原 PRD 方向与 E design §3.3/§5、F `release-matrix-contract.md`（字段唯一来源）三方一致，但存在 1 处跨契约张力（E 的"五个 suite"措辞 vs 四 ID 套件结构）、2 处需求粒度缺口（锁定旧行为的契约测试需同步升级；validator 双相位未定义）与若干证据精化。已修复 PRD 并补 design/implement；实现方向均可由仓库证据确定，无用户裁决项。审核只改任务规格与研究材料。

## Ready Selection Evidence

B-E1、B-E2 已归档；B-E3 已完成（CI chromium-e2e 全绿 run 33589413738）**待归档确认**。B-E4 `meta.depends_on=[build-mode, mongo-harness, chromium-editor]` 实质满足；按惯例 B-E4 激活前需 B-E3 归档证据落盘。

## 现状盘点（逐文件核实）

| 组件 | 现状 | 与契约差距 |
|---|---|---|
| `playwright.config.mjs` browser-smoke | testMatch 仅 business-flows + editor-flows；实测 `--list` 6 用例/2 文件 | 缺 bootstrap-components 与 leaui-image-iframe 两个稳定 ID 的发现 |
| `scripts/browser-release-evidence.mjs`（producer，149 行） | coverage 硬编码通用 `['build-smoke','auth-gate','error-gate','resource-gate']`（:110）；validateBrowserMatrix 接受 1-40 任意 scope（:69）；provenance 无 `coverage_summaries`（:134-139）；无 JCS | 四 ID 固定顺序、每行 `coverage_summary_sha256`、provenance 八槽位摘要、JCS 摘要计算全部缺失 |
| `scripts/validate-browser-artifact.mjs`（validator，22 行） | provenance 键 allowlist 硬编码六字段（:20），**携带 `coverage_summaries` 的 v1 载荷反被拒绝**；无摘要重算/交叉校验；run/attempt 只按当前 run 相等校验 | 需 schema 升级 + JCS 重算 + 预检/正式双相位 |
| `browser-release-evidence.yml`（36 行） | 仅 `workflow_call`；self-hosted protected runner；上传两文件 artifact、retention 7 天；**自身无发布副作用**（发布在 release.yml publish job） | 缺"仅预检"独立受控入口（workflow_dispatch + 严格 tag 解析），E AC-E6 第 3 行 blocked 的直接原因 |
| `tests/js/release-contract.test.js` | :118（canonical eight-record，锁通用 coverage）、:221（protected workflow 执行命令）、:298（producer workflow 命名）、:319（前缀 attempt 拒绝）四用例锁**旧**契约 | 必须与新契约同步升级，否则实现即测试红 |
| "五个 suite"约束 | quality-gate/测试**无**机械断言；仅 E evidence-matrix AC-E4 措辞（22 tests/5 files） | 张力点，见下 |

## 跨契约张力与裁定（D1）

E 的 AC-E4 措辞"business 五个 suite 共 22 用例"与 AC-E6 四稳定 ID 套件结构存在文件数张力：四 ID 需要 `leaui-image-iframe` 成为可独立发现的套件。裁定（依 E design §3.3 "每个产品/slot 逐项"与 B-E4 AC "list/执行证据证明四个 ID 均被发现"）：

- **文件-ID 一一对应**：新建 `tests/e2e/business/leaui-image-iframe.spec.mjs`（从 business-flows.spec.mjs **迁移**两个 leaui iframe 用例，非复制），browser-smoke testMatch 扩为四文件。
- business 项目发现变为 6 文件/22 用例（**用例数不变**，迁移不复制）；Chromium job 无文件数断言（已核实），AC 不破。
- E 的 evidence-matrix AC-E4"5 files"措辞与 E design §3.2"五个文件"枚举均属 E 自有材料，由 E 收口阶段同步——登记为 E 侧 spec-sync 项，不阻塞 B-E4。

## 需求精化（D2-D5）

- **D2 锁步测试**：四个契约测试必须与 producer/validator 同 PR 升级（含通用 scope 拒绝、JCS 摘要正/反例、coverage_summaries 结构、预检相位）；PRD 原文未提，已补为硬性要求。
- **D3 validator 双相位**：预检相位（E 侧运行）校验 tag commit == 候选 SHA、全部摘要/结构规则，**不**要求 run/attempt 等于校验者自身；正式相位（release.yml 内）保持 producer run/attempt == 当前 run。以显式相位参数区分，默认正式相位（fail-closed）。
- **D4 JCS 实现约束**：载荷域受限（ASCII 标识符 + 安全正整数），实现最小 RFC 8785 规范化器（键按 UTF-16 码元排序、JSON 字符串转义、整数规范表示、无空白/尾随换行、UTF-8），并以内置 RFC 8785 附录向量 + 契约载荷正/反例单测锁定；不引入新依赖。
- **D5 预检入口设计约束**：browser-release-evidence.yml 增加 `workflow_dispatch`（严格 `vX.Y.Z` tag 输入；checkout 解析剥壳 tag commit（`refs/tags/vX.Y.Z^{}`，同 release.yml 既有模式）；RELEASE_COMMIT 用解析 SHA 而非输入覆盖；permissions 仅 `contents: read`；runner 标签不变；只上传两文件、retention ≤7 天；无 Release/GHCR/F 状态副作用——该 workflow 本无发布 job，需以 workflow 结构断言锁定）。

## 无需用户确认的判断

文件-ID 对应（vs 前缀映射）、迁移而非复制、JCS 最小实现 + 向量锁定、预检走 workflow_dispatch、validator 相位参数——均可由 E/F 契约与现状证据唯一确定。受保护 runner 的 `BROWSER_SMOKE_COMMAND_*` 环境由 runner 自带（workflow 不注入），producer 只需扩展 marker 协议（每 coverage 的 discovered/executed/entrypoints/iframes），B-E5 执行时遵守。

### 评审后补录（2026-09-02 code-review 双轴）

- 计数修正：business-flows.spec.mjs 仅有一个独立 leaui 契约用例（:84），"两个用例"系笔误；:187 内嵌段不拆的边界在 PRD/design/implement 三处统一。
- 规则补齐：`entrypoints` 非空、`iframes` 可空（F schema minItems 1 / 仅 maxItems 40 的区分）写入 PRD Req 2 与 design §3。
- E 侧 spec-sync 项补全：除 evidence-matrix AC-E4"5 files"外，E design §3.2 的"五个文件"枚举同样受六文件影响，一并登记。
- PRD Req 3/5 收敛为行为级 + design §指针（去实现细节复述）；implement 末任务标题规范为 "Task 8"。

## 审核过程 provenance

- 通读 `browser-release-evidence.mjs`/`validate-browser-artifact.mjs`/`browser-release-evidence.yml` 全文；release.yml 相关流（validate→publish、剥壳 tag 断言来自 release-contract.test.js:210-214）。
- F 归档 `research/release-matrix-contract.md`（179 行）全文；E design §3.3/§5/§7。
- `--project=browser-smoke --list` 实测 6 用例/2 文件。
- `tests/js/release-contract.test.js` 浏览器相关四用例逐一定位；quality-gate chromium 步骤与测试树无五文件断言核实。
- 未修改任何业务实现、测试、workflow；未激活任务（待用户批准，且激活前需 B-E3 归档）。
