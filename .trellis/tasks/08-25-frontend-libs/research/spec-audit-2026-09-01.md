# E 协调收口规格审核（2026-09-01）

## 结论

依赖层面选中的 ready 叶是 `08-25-frontend-libs`，但原规格不能直接指导当前收口：它仍按“将要依次执行三个子任务”描述，浏览器证据来源与 F 的 tag artifact 冲突，且没有吸收当前 HEAD 的失败 CI。已将 PRD/design/implement 重写为同一提交的组合验收契约；规格审核阶段没有修改业务实现、测试、构建、workflow、生成资源或任务状态。

Q-E1 已确认采用等待 tag artifact 的两阶段路径；B-E1 至 B-E6 仍未闭合，只阻断完成结论，不替代实现前置审查。规格校验通过后可激活现有任务，但在这些证据闭合前不能声明可完成。

## Ready Selection Evidence

| 项 | 现场值 | 判定 |
|---|---|---|
| 目标 | `08-25-frontend-libs`，`status=planning`，parent=`08-25-tech-stack-modernization` | 未完成候选 |
| 子任务 | jQuery、Bootstrap、TinyMCE 三项均在 `archive/2026-08` 且 `status=completed` | 活动图上无未完成 child |
| `meta.depends_on` | `08-25-frontend-build-chain` | 唯一依赖 |
| 依赖证据 | `archive/2026-08/08-25-frontend-build-chain/task.json`，`status=completed` | dependency-ready |
| 其他活动任务 | `00-bootstrap-guidelines=in_progress`；现代化 root=`planning` | 不是新的 ready 叶 |

`children` 保留已归档历史，因此“有效叶”指活动任务图中没有未完成 child；不删除历史 child 关系。

## Reviewed Evidence

- 目标、父任务和三个归档子任务的 `task.json`、PRD、design、implement、research/context material。
- ADR-0003、`package.json`/lockfile、manifest、Playwright config、E2E/Node tests。
- `quality-gate.yml`、`release.yml`、`browser-release-evidence.yml` 与 F 的 PRD/design/implement、task notes、
  `release-matrix-contract.md`。
- 三份 `docs/modernization/browser-smoke/*.md`。
- GitHub Actions CI run `33477561244`，commit `fcc979bb9f0fe35d1771b00665017e470e2182d4`。

语义检索服务在本轮返回 404；随后使用已知任务/配置/测试入口和精确搜索完成核验。

## Audit By Requirement Dimension

### 目标、业务规则与范围

- 原文把已经归档的三个子任务写成未来步骤，且让 E “增加 `test:e2e`”，与当前 `package.json` 已存在脚本及“E 不改生产代码”冲突。
- 已改为组合验收目标，并明确 inputs、outputs、owner 与 fail-closed 结论。

### 输入输出与交互流程

- 原文没有定义候选提交、运行 provenance 或最终 evidence matrix schema。
- 已固定候选 SHA、干净 checkout、子任务/CI/browser artifact 输入，以及 `eligible_for_completion|blocked` 两种输出。
- 交互覆盖按 build → business/contract → real browsers 三层组织，并列出业务、编辑器、Bootstrap 组件与 iframe。

### 边界、异常与数据约束

- 补充零发现、缺环境、身份错误、资源/console/page 错误、超时、清理失败、artifact 跨 run/ref/attempt 和 mode 漂移的失败语义。
- 补充证据脱敏 allowlist，禁止认证材料、正文、用户数据及原始浏览器 artifact。
- 保留未编辑 HTML 字节、真实编辑语义、API/USN/所有权/Schema 不变量。

### 兼容性与依赖

- 保留旧公开静态 URL，但以 manifest 输入和实际字节判断版本；`jquery-migrate` 可在开发依赖存在、不得进入生产。
- F 已在 E 仍 planning 时归档，违反原 DAG 时序；F notes 同时仍声称上游证据阻断。E 不篡改历史，只要求父任务收口登记冲突。

### 验收完整性

- 原验收只要求命令“通过”，没有 exact commit、test discovery、CI run、mode/untracked、owner/retest 约束。
- 已将 AC-E1..E9 映射到 evidence matrix，并禁止 archived 状态替代当前运行。

## Current Local Evidence

| Probe | Result |
|---|---|
| `npm ls --all --json`（此前的 `--depth=0` 仅作顶层辅助） | 顶层精确版本符合规格；完整嵌套路径计数尚待候选 checkout 重验 |
| Playwright `build-smoke --list` | 1 test / 1 file |
| Playwright `business --list` | 22 tests / 5 files |
| Playwright `browser-smoke --list` | 6 tests / 2 files；缺 `bootstrap-components` |
| `npm test` | 120/120 passed，耗时约 121 秒 |
| 本地真实服务/Mongo/浏览器 | 本轮未运行，不宣称通过 |

## Current CI Evidence And Blockers

CI run：`https://github.com/yangphere/leanote/actions/runs/33477561244`。

| Blocker | Evidence | Owner / impact |
|---|---|---|
| B-E1 | node-build job `99759909194`：TinyMCE 官方插件生成文件由 mode 100755 变 100644，`git diff --exit-code` 失败 | D/E-TM 生成闭包；AC-E3 阻断 |
| B-E2 | mongo job `99759909276`：已有 service 占用 27017，harness 又启动 `leanote-test-mongo`，Golden/USN/所有权/note-save 失败 | G/harness；AC-E5 阻断 |
| B-E3 | chromium job `99759999476`：19/22；`leaui_image` 出现 `editor.on is not a function`，主业务和编辑器清理超时 | E-TM/E2E；AC-E4/7/8 阻断 |
| B-E4 | `browser-smoke` 只发现 business/editor；F producer 固定通用 coverage，严格 v1 validator 也尚未接受四个稳定 ID、`coverage_summary_sha256` 与 `coverage_summaries` | E-BS/F；AC-E6 阻断 |
| B-E5 | Bootstrap/TinyMCE 浏览器文档为待执行/blocked；jQuery 缺 Safari、前一主版本和最终 SHA 复跑 | 外部真实浏览器环境；AC-E6 阻断 |
| B-E6 | package job `99760156757` 以 `release tag ... dev` 失败；container job `99760157037` 把 RFC3339 时间当 epoch；F 已 archived 且 notes 仍写阻断 | F/父任务；下游整体门禁与生命周期冲突 |

Go 1.26/1.27 jobs 成功，但不能覆盖其他失败门禁。

## Specification Conflicts Resolved

1. **Tracked browser record vs tag artifact**：现有浏览器 Markdown 降为历史/缺口台账；正式发布证据以 F artifact 为准。
2. **Parent direct implementation**：删除 E 新增脚本/测试的职责，E 只验收；实现缺陷返回 owner。
3. **Child archive vs current truth**：明确 archived 不是通过凭据，必须验证当前提交。
4. **Diff scope**：同时检查 tracked、untracked、内容与 Linux mode。
5. **Browser semantics**：结构八行不足以证明页面/组件覆盖，必须绑定具体 suite/脱敏摘要。

## Resolved Decision

Q-E1 已由用户选择“等待 tag artifact”。为避免 E 等待 F、F 又依赖 E 的循环，采用受保护 tag 预检 artifact：预检只生成两文件 `browser-release-matrix-v1` 供 E 验收，不创建 Release/GHCR、不改变 F 任务状态；E 归档后，F 在最终 release run 中重新生成并校验正式 artifact。候选 SHA 与 tag commit 分列记录且必须相等。

无其他无法从仓库回答的产品范围问题。实现缺陷均为已确认 blocker，不是需要猜测的需求。

## Post-review specification repairs (2026-09-01)

针对本轮 diff-review 的 9 项问题，已只修改 E 的 PRD/design/implement、研究与验收材料，以及父任务和 F 规格材料的历史 DAG/发布契约说明；没有运行 `task.py start`，没有修改业务实现、测试实现、构建脚本、workflow 或生成资源。

1. Q-E1 已改为两种互斥、条件化的完成标准：等待 tag artifact 时先修正 E→F 无环依赖；不等待时 E 必须闭合候选 SHA 的真实浏览器预发布证据，F artifact 仅作为发布门。候选 SHA、tag commit 和 artifact 不再被要求同时出现在同一证据链。
2. 语义 coverage 已固定为 `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe` 四个稳定 ID（每条矩阵记录按固定顺序恰好四项）；每条记录还必须绑定 discovered/executed、入口/iframe、结果摘要的 digest 与 run/attempt provenance。F 当前通用 `scope` producer 因未满足该契约继续阻断。
3. Mongo 验收已拆分为 service-backed Go/Mongo（复用 CI `127.0.0.1:27017`，不调用 `NewMongoEnvironment.Up()`）和 Chromium 自建 harness（无外部 service 时独占容器）两种互斥模式；混用、错误数据库或第二个固定端口均 fail closed。
4. E quality-gate allowlist 已明确为 `node-build`、`chromium-e2e`、`mongo-8_0`；Go 两版本是外部证据，package/container/summary 属 F/父任务，并在矩阵中分别记录 owner/retest。
5. `acceptance/evidence-matrix.md` 已重写为逐行 schema，定义 `passed`、`failed`、`blocked`、`partial`、`passed-local`、`missing` 的含义和聚合规则；每行包含候选 commit、命令或 run/job URL、发现/执行数量、结果、owner 与复验条件。AC-E1 不再以不完整 provenance 标记 passed。
6. 依赖唯一性检查从 `npm ls --depth=0` 改为 `npm ls --all --json` 加 lockfile 完整 `packages` 树路径计数，明确顶层输出不能证明嵌套唯一性。
7. E 设计新增归档子任务契约映射：jQuery 第一方/第三方 Migrate warning 与 AJAX 失败可见性，Bootstrap remote modal、BootstrapDialog、hover-dropdown、`leaui_image` 和用户主题字节/路径，TinyMCE 安全默认值、paste/drop 单次插入、七 locale、revision/失败语义。
8. 父任务 PRD 已同步为“历史任务树 + 当前收口状态”表述：三个子任务和 F 的归档状态不再被描述为当前将要执行的 DAG 顺序；F 在 E planning 时归档及 notes 冲突必须显式登记。
9. 连续构建已写成双 checkout 协议：第一次构建在 checkout 外保存输出集合、SHA-256 和 POSIX mode，第二个同 SHA 的全新 checkout 重建并逐项比较，同时检查 tracked diff 和 non-ignored untracked 均为空。

本轮复审后追加的规格修复：

10. Q-E1 的“不等待 tag artifact”路径已固定为受保护的 `candidate-browser-matrix-v1` artifact，载荷为 `candidate-matrix.json` + `candidate-provenance.json`，明确 8 槽位、候选 SHA/ref/run/attempt、四个 coverage ID 和 validator 入口；不再把 AC 级验收表当作槽位载荷。
11. 语义摘要契约已固定：每槽位四个 summary 项、正整数 discovered/executed 且 `executed_count <= discovered_count`、受限稳定入口/iframe 标识、`passed` 结果及 RFC 8785 JCS SHA-256（摘要输入为去掉 digest 字段的 `{browser_product, release_slot, items}`，无空白/尾随换行）；F 保持两文件 allowlist，在 `provenance.json` 内嵌 `coverage_summaries`，矩阵行新增 `coverage_summary_sha256`，旧通用 scope 载荷无效。
12. 验收矩阵已补充候选/发布 artifact 载体表、build-smoke 和 summary 行，并将未进入 discovery 或非测试门禁的 `N/A` 原因与非 passed 状态写法固定；`partial` 在任务级明确映射为 `blocked`。
13. E ownership 增加规划审核例外：仅允许为同步父任务 DAG/生命周期事实修改父 PRD；任务激活后仍不得越权修改父任务或业务实现。
14. 复审发现并修正了候选与发布契约漂移：候选矩阵沿用发布记录字段但使用独立 schema/version 和候选 SHA/ref；F PRD/design/implement/task notes 已同步 coverage summary digest 要求；验收矩阵对未执行 protected runner 和未取得外部 job summary 的行改用带原因的 `N/A`，不再把零或缺失计数写成可执行证据。
15. AC-E6 的候选与 tag artifact 证据改为同一“所选 Q-E1 模式”门禁；未选模式不再被错误地作为 E 的额外必需 artifact，但 Q-E1 未确认前两行仍保持阻断。
16. 真实浏览器 smoke 与摘要契约统一为每个产品/slot 必须执行四项稳定 coverage（含 `leaui-image-iframe`），不再留下“至少三项”而使实现者误以为 iframe 可选；候选与发布 artifact 的保留期均固定不超过 7 天。
17. 候选 provenance 的 `candidate_run.id` 明确采用与 F `release_run.id` 相同的非零十进制字符串，`attempt` 明确为 JSON 正整数，消除候选与发布 schema 的类型歧义。
18. E 的规划审核 ownership 明确允许为保持 E↔F 证据契约同步 F 的任务规格、研究、验收材料和 task notes，但禁止修改 F 实现、测试、workflow、生成资源或任务状态；该例外在 E 激活后失效。
19. AC-E1 改为只约束 E 自有运行记录的候选 SHA；若等待 tag artifact，F 发布运行的 tag commit 按 AC-E6 独立记录，避免把两个不可相同的身份强行合并。
20. Q-E1 确认后将等待路径落为两阶段：tag 预检 artifact 可在 E 归档前生成但不得发布或改变 F 状态；F 的最终 release run 在 E 归档后重新生成正式 artifact，父任务 DAG 不增加反向任务边。

上述修复只解决规格可实施性与证据可审计性，不关闭 B-E1..B-E6 的实现/环境失败；Q-E1 已确认，现有任务可在启动门禁通过后激活，但完成结论仍保持 `blocked` 直至这些证据闭合。

## Activation follow-up (2026-09-01)

用户随后明确批准“启动功能实现”，主流程据此运行任务激活并将 `08-25-frontend-libs` 切换为
`in_progress`；`task.py current` 当前指向该任务。上述规格审核记录中的“未运行 `task.py start`”仅描述审核
阶段，不表示激活后仍处于 `planning`，也不构成任何实现或验收通过证据。
