# B-E5 规格审核与环境就绪度核实（2026-09-02）

## 结论（与此前各叶不同：存在必须交用户裁决的待确认事项）

B-E5 的**仓库侧前置已全部就绪**（B-E4 交付四 ID 套件 16 用例/4 文件、producer/validator/预检入口，双 CI 全绿），但**执行环境完全缺失**：仓库零自托管 runner、证据 workflow 从未运行、无候选指向的严格 tag、Safari 需 macOS 硬件且 Playwright 无法驱动真实 Safari。按 E design §4 失败状态机，这是 `blocked`（缺环境/缺浏览器/缺外部 runner），不是可绕过的实现细节。PRD 已按"环境就绪门禁"重写；**环境提供方式属用户决策，不得擅自假设**。

## Ready Selection Evidence

B-E4 已归档（`016cf5ae`），B-E5 `meta.depends_on=[browser-coverage]` 满足，是唯一 ready 叶（B-E6 依赖 B-E5）。"ready"指可进入规划/审核；**激活另受环境门禁约束**（见 PRD）。

## 环境就绪度核实（硬证据）

| # | 检查项 | 证据 | 判定 |
|---|---|---|---|
| 1 | 受保护 runner `[self-hosted, protected-browser-matrix]` | `gh api repos/yangphere/leanote/actions/runners` → `{"total_count":0,"runners":[]}` | **不存在**；workflow_dispatch 预检将永久排队 |
| 2 | 证据 workflow 运行史 | `gh run list --workflow="Protected browser release evidence"` → 无任何运行 | 从未执行过 |
| 3 | 严格候选 tag | `git ls-remote --tags origin`：20 个 tag 均为历史 `0.x`/`v0.2`（`2d107b7`，远古），无指向候选谱系的 `vX.Y.Z` | **不存在** |
| 4 | 真实浏览器可得性 | `docs/modernization/browser-smoke/jquery-3.7.md`（2026-08-28）：开发者 Windows 本机 Chrome 152/Edge 151 经 Playwright channel 真实执行 2/2 通过 | Chrome/Edge 有本机先例；**Safari 需 macOS 硬件**，且 Playwright 无 safari channel（仅 WebKit，契约明确不接受），真实 Safari 只能走 safaridriver/自研驱动 |
| 5 | `BROWSER_SMOKE_COMMAND_*` 受保护命令 | 仓库内不存在（workflow 注释明确"由 runner 提供"）；B-E4 已定义其必须满足的 marker 协议（producer 解析 fail-closed） | 协议已定，实现载体缺失 |

## 仓库侧就绪清单（B-E4 交付，已验证）

- 四 ID 套件：`--project=browser-smoke --list` = 16 用例/4 文件；`LEANOTE_SMOKE_BROWSER` 支持 chrome/msedge/firefox(chromium 族 channel 映射)。
- producer：marker 协议（版本/OS/三门禁 + 每 coverage 的 discovered/executed/entrypoints/iframes，标识符 `^[a-z0-9][a-z0-9._/-]{0,79}$`、entrypoints 非空）。
- validator：`--phase precheck --expected-commit <候选 SHA>`（tag 绑定、JCS 重算、跨 run 容忍）。
- 预检入口：workflow_dispatch（严格 tag、剥壳身份、无发布副作用、retention 7）。

## 三个旧浏览器文档的定位

`docs/modernization/browser-smoke/{jquery-3.7,bootstrap-5.3,tinymce-8}.md`：E design §5 已裁定它们**不再是发布真相**（历史/缺口台账）。jquery 文档含有效的执行方法学记录（本机 channel 模式）；bootstrap/tinymce 含 blocked/pending 标记。B-E5 执行后不更新这些文档为"真相"，只在 E 收口时作为历史台账引用。

## 待确认事项（必须用户裁决，影响面标注）

**Q-E5-1：受保护执行环境如何提供？** 三个选项（E 契约均允许，但后果不同）：

- **(a) 完整契约路径**：注册 macOS 自托管 runner（Safari 硬件）+ 装 4 产品 × 2 主版本 + 配置 `BROWSER_SMOKE_COMMAND_*`（含 Safari 的 safaridriver 方案）。影响：硬件/运维投入；一次配置，E/F/B-E6 全链解锁；证据等级最高（E design §3.3 原样满足）。
- **(b) 分步路径**：先注册 Windows/Linux runner 跑 Chrome/Edge/Firefox 六槽位，Safari 两槽位待 macOS 就绪。影响：矩阵不完整→E 的 AC-E6 仍 blocked（契约要求恰好 8 槽），但先把 6/8 的真实证据与 runner 命令调试落地。
- **(c) 暂缓**：B-E5 记 blocked（owner/retest 齐备），链路停在 B-E5，B-E6 的前置同样不满足（其 depends_on 含 B-E5）。影响：E/F 收口无限期挂起。

**Q-E5-2：tag 与版本号策略**。预检要求严格 `vX.Y.Z` 指向候选 SHA。现有 tag 均为非严格 `0.x`。需要用户定：首个严格版本号（如 `v1.0.0`？）及 tag 创建时机（预检 tag 是否最终发布 tag——Q-E1 已定：预检 tag 即候选绑定，E 归档后 F 终 release run 重新生成 artifact，同 tag 可复用）。

## 规格修复（PRD）

1. 新增 Environment Readiness Gate（Task 0）：Q-E5-1/Q-E5-2 获用户明确答复前，任务保持 `planning`，不运行 `task.py start`。
2. 原需求 R1-R5 保留（与 E §3.3/F 契约一致，经核对无冲突），补充执行序列与 runner 命令契约指针（design §2-3）。
3. AC-4 的 blocked 语义具体化：环境缺失时材料就绪即为本任务在当前阶段的"完成形态"，blocked 状态登记 owner/retest 并挂起，不算失败交付。

## 审核过程 provenance

- `gh api .../actions/runners`（total_count=0）、`gh run list --workflow`（无运行）、`git ls-remote --tags origin`（20 tag 全历史）。
- `docs/modernization/browser-smoke/` 三文档通读；E design §3.3/§4/§5、implement Task 4、evidence-matrix AC-E6；B-E4 归档材料（marker 协议/双相位/预检入口）。
- 未修改任何业务实现；未激活任务；环境提供方式未做任何假设。
