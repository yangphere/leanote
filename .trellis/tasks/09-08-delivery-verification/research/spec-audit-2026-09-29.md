# 交付任务规格审核 — 2026-09-29

## 1. 选择、授权与基线

- 已选择 `09-08-delivery-verification`：P1、原 planning、children=[]、layer=delivery；父任务 `09-08-business-layer-architecture` 仅协调。当前未归档任务只有父任务与本叶，按领域→应用→基础设施/接口→呈现/交付顺序，本叶是唯一 ready 叶。
- `task.json.meta.depends_on` 是就绪依据。9 个依赖均在 `../archive/2026-09/` 下 status=completed：domain-contracts（09-09）、infrastructure-persistence（09-10）、application-notes（09-15）、application-content（09-18）、application-identity（09-24）、application-publishing/admin（09-25）、interface-http/presentation-frontend（09-28）；日期均为 2026 年。依赖图不由父子关系替代。
- 先向用户报告选择和依赖，再修复 activation 前 context 校验的 7 个旧路径。`task.py validate` 通过后 `task.py start` 成功，状态 in_progress，当前会话指向本任务；没有创建新任务。
- 审核起点 HEAD=`3b963562`、branch=dev、工作树干净。本轮用户仅授权激活和规格审核；只编辑本叶规划/研究/验收/context 和激活元数据，没有业务实现、CI、测试脚本或生成资源变更。
- 使用 trellis-start 核对流程；参考 trellis-brainstorm 的需求/证据收敛方法，用户明确要求激活既有任务优先于通用新任务创建/激活顺序，不据此扩展功能编码授权。

## 2. 发现与处置

| ID / 重要性 | 已证实的问题与依据 | 本轮规格修复 / 后续 owner |
| --- | --- | --- |
| F01 / 阻断激活 | implement/check 共 7 条 identity/interface/content 路径仍指向未归档目录，validate 直接失败 | 更新为实际 archive 路径，重跑通过后激活 |
| F02 / 高 | 原 delivery 矩阵仅 identity/persistence/interface/content/汇总 5 行；publishing/admin/domain/notes/presentation 的细分 AC 无完整接收位置 | 扩充 13 个交付条目映射九轨；保留上游唯一合同与历史未运行状态 |
| F03 / 高 | 原 plan 要求前置 partial/unknown 一律先 blocked，却又让 delivery 负责补齐这些证据；容易形成循环门禁 | 区分缺合同/provider 与缺执行证据；后者是本任务待办，不禁止开始补齐 |
| F04 / 高 | notes 历史 21/38 与后续 native harness 包通过不能证明当前合并候选 38/38；上游 inventory §9、interface 矩阵 §2026-09-28、presentation closeout | 新建 38 个 action ID 索引，mapped 21/missing 17 仅历史；当前全部 unrun，明确逐请求/DB/receipt 断言 |
| F05 / 高 | precheck 与 final 混写为所有证据同一 run；`browser-release-evidence.yml:4-14,39-50` 有独立 dispatch，`validate-browser-artifact.mjs:55-65` 明确两种校验 | 分开候选预检/最终发布；final 仍同一 release run/attempt，独立预检不可移用 |
| F06 / 高 | `.github/workflows/quality-gate.yml:262-280,321-335` 的正向 package/container PDF URL smoke 不等于 production exit78/paired volume/restart 或恶意 PDF/zero-outbound 实证 | PRD R7、E-D06/E-D11 与计划 Phase2 明确缺口；delivery 补工具/runner，provider 缺陷回 content/interface |
| F07 / 高 | `scripts/browser-release-evidence.mjs:170-199` 执行受保护 shell 命令并解析 version/passed marker；schema 通过本身不能证明真实 binary/version 或完整业务覆盖 | 保留现 protected runner 边界，要求可审计命令版本/真实产品来源；禁伪 marker/WebKit 代 Safari；额外业务清单不被四 smoke 替代 |
| F08 / 高 / 规格已决、实现待办 | `.github/workflows/release.yml:73-94,104-119` 先拒重复再 push image，再 create Release；后者失败后普通重跑被既有 image 阻断 | 2026-09-29 用户确认 Q-DV1，允许核验原身份和制品后仅补建缺失 Release；普通 duplicate/final guard 不变，不自动删/覆盖；代码尚未修改 |
| F09 / 高 | `scripts/ci/validate-summaries.mjs:70-85` 核对 provenance/status；allowlist 校验不能证明错误诊断及完整授权 URL 不泄漏 | R8 与 evidence 规则要求输出脱敏/合成 fixture；不把静态 schema 通过当泄漏验证，原始日志不发布 |
| F10 / 中 | 旧 research 写“代码与 PRD 冲突以代码为准”，会把实现缺口倒变需求 | 改为代码证明现状、已批准合同定义目标；冲突必须记录并归属，不自动放宽 |
| F11 / 中 | task 生命周期、环境 blocked、历史 pass、candidate pass 与发布完成混用；旧 plan 说只重跑失败 stage，忽略 run_attempt 变化 | 区分证据状态与 task 状态；final 所消费 artifact 必须符合本次 run/attempt，不能复用前次 job 输出绕过 validator |

## 3. 全面性审核

| 维度 | 审核结论与落点 |
| --- | --- |
| 目标/范围 | 交付实证+版本制品，无生产部署/ARM64/新业务功能；本轮只审核；PRD Scope/R9/R10 |
| 业务规则 | 保留九轨批准的权限/USN/receipt/token/分享/评论/备份合同，不因历史文档未更新推断实现状态；R3～R6 |
| 输入输出 | full SHA、strict tag/version、fixture/环境来源、真实 run/attempt、固定 artifact/allowlist/digest；R1/design §3 |
| 交互流程 | Web 未知重试/换代/冲突、move 权威重读、copy/delete 队列出口、零编辑零写入；R6+上游未勾选手工清单 |
| 边界/异常 | 多拓扑、部分写/未知结果、kill/restart、failpoint、文件同卷/路径、PDF 网络/本地文件、超时/取消/清理；R4/R7 |
| 数据约束 | 发布标识/digest、isolated DB、历史索引冲突不自动删、comment submissionId 例外、隐私脱敏；R1/R3/R5/R8 |
| 兼容 | 原 URL/API/JSON/HTML、Golden baseline-target、token 过渡、无 CSRF 与同源主题风险、旧卷复制与无 runtime fallback；R3/R5～R7 |
| 依赖/上下游 | 9 个已归档依赖可激活；未齐实证由 delivery 收集，业务缺陷回原 owner；无新任务、无反向循环前置；R2/R10 |
| 验收 | AC-DV1～10 对应 E-D01～13；38-action 索引+八槽四固定 coverage+扩展业务实证+最终远端读回；未知不记 passed |
| 可实施性 | 已有 reusable workflow/validator/harness 可复用，production/PDF/fault/SMTP 等扩展门禁和已确认 Q-DV1 恢复需后续实现接线；真实 runner/环境仍未验证 |

## 4. 决策与执行条件

Q-DV1 已于 2026-09-29 由用户答复“允许”：允许核验后受控补建缺失 Release。PRD R9、AC-DV9/10、design §5、计划和恢复矩阵已同步；当前已识别产品决策无剩余待决项。恢复必须验证原门禁、原 run/attempt 与制品、远端 tag/commit/image digest，只在 Release 明确不存在时创建；完整既有结果只读确认，不修改不完整既有 Release，不重推镜像、不覆盖或移动 tag。原制品来源缺失或过期不可重建替代。

本次确认只解决恢复策略，不扩大此前“审核阶段只改规格材料”的范围，也不是对具体远端版本发布的执行指令。当前 task 仍 in_progress，代码实现和所有真实验收仍未执行。

其他不需要重新决定的事项：ADR-0004 已确认版本制品发布且无自动生产部署；真实八槽/PDF 零外联/敏感输出禁止是既有要求，不降低为 mock 或 marker-only。上游已有明确 decision 不重复询问。

后续执行前仍要核验：受保护 runner 的受审命令/真实八槽安装与主版本来源、隔离 Mongo 7/8 各拓扑、SMTP、Linux filesystem/PDF 与网络观测能力、GitHub/GHCR 授权/权限和候选 tag。它们是尚未验证的执行条件，不猜测本机可用性、不要求在审核阶段启动。当前任务无发布授权。

## 5. 证据与验证范围

- 代码链路核查为只读 context_explorer + 主流程定向复核；语义搜索因仓库没有 jbcontext index 无结果，随后使用定向读取。没有为调研创建 index 或修改业务文件。
- 所有历史 runtime 证据均明确来自上游矩阵，不冒充本轮重跑。原 notes 旧 Revel 回放不能替代当前 native harness。
- 本轮只验证 task contexts、JSON/路径/ID 覆盖、Markdown 引用和 git diff 范围/空白；不运行 Go/Node 产品测试、服务、数据库、浏览器、容器或发布。最终具体结果在 acceptance 矩阵记录。
- task start 提示 branch=base_branch=dev 的未来 PR/archive 限制；记录为后续流程前置，本轮没有切分支、commit/archive/journal/push。
