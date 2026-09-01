# B-E1 规格审核与根因核实（2026-09-01）

## 结论

选中的 ready 叶是 `09-01-frontend-libs-build-mode`（B-E1）。原 PRD 方向正确但存在 1 处关键歧义（未固定规范化方向）、多处范围/机制/验收缺口；已全部依据仓库证据修复，无需用户裁决的开放问题。审核只改本任务规格与研究材料，未触碰业务实现。

## Ready Selection Evidence

| 项 | 现场值 | 判定 |
|---|---|---|
| 轨道状态 | 根任务 `08-25-tech-stack-modernization` 的 8 个 child 中，仅 `08-25-frontend-libs`（E）为 `in_progress`，其余全部归档 `completed` | E 是唯一活动轨道 |
| E 的活动 child | B-E1..B-E6 六个阻断子任务均为 `planning` | 有效叶集合 |
| 依赖 | B-E1 `meta.depends_on=[]`；B-E2 依赖 B-E1；B-E3 依赖 B-E1+B-E2；B-E4 依赖前三者；B-E5 依赖 B-E4；B-E6 依赖全部 | B-E1 是唯一 ready 叶 |
| 优先级 | B-E1 为 P0 / sequence 1 | 轨道优先顺序第一 |

## 根因核实（代码级证据链）

CI 证据：run `33477561244` / attempt 1 / [node-build job `99759909194`](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759909194)，`git diff --exit-code` 步骤失败。

1. `scripts/build/manifest.mjs:129` —— `normalizeTrailingWhitespace` 的正则 `/^node_modules\/tinymce\/.+\.(?:css|js)$/i` 命中**全部**上游 TinyMCE js 输入，含 10 个官方插件的 `plugin.js`/`plugin.min.js` 共 20 个输出。
2. `scripts/build/js.mjs:78-80` —— normalize 分支用 `fs.writeFile` 写入 **staging** 目录；staging 是全新目录（`index.mjs:60` mkdtemp），文件按 `0666 & ~umask` 创建，CI umask 022 下为 `0644`。
3. `scripts/build/index.mjs:134-150` —— 发布采用 backup→rename：原 checkout 文件被移走，staged 文件 rename 到位，**最终 mode 完全由 staged 文件决定**，与 git 索引无关。
4. git 索引将这 20 个文件跟踪为 `100755`（历史 vendored 遗留），Linux `core.filemode=true` 下 `git diff` 报 `old mode 100755 / new mode 100644`。
5. 本地不可见的原因：开发机 Windows `core.filemode=false`，mode 差异被掩蔽——解释了为何漂移只在 CI 显形。

### 漂移清单（逐文件核实）

- **CI 实际漂移 20 个**（上游插件 ×2）：advlist、autolink、charmap、directionality、link、lists、searchreplace、table、visualblocks、visualchars 的 `plugin.js` + `plugin.min.js`。
- **潜在不一致 2 个**：`public/tinymce/plugins/leaui_mindmap/plugin.js|.min.js` 同样被跟踪为 `100755`，但走 `tinyMceFirstParty` 恒等 copy 条目（`manifest.mjs:95-99`，input==output），经 `fs.copyFile`（`js.mjs:82`）保留 checkout mode，故本次 CI 未漂移。该"通过"是侥幸：一旦构建改为强制固定输出 mode（本任务即将做的事），这 2 个文件会**开始**漂移。
- 其余全部 manifest 输出（`public/js`、`public/css`、`public/album`、`public/md/themes`、`app/views/note/note.html` 及 tinymce 核心/theme/icons/model/skin、其余第一方插件）均为 `100644`，已用 `git ls-files -s` 全量核实。
- 结论：**索引中跟踪为 755 的 manifest 输出恰为 22 个，全部应规范化为 644**。

### mode 传播矩阵（修复设计的依据）

| 输出类别 | 写入分支 | 输出 mode 来源 | umask 敏感 |
|---|---|---|---|
| 全部 `node_modules/tinymce/**` 输出（含 20 个漂移文件） | writeFile（normalize 分支） | `0666 & ~umask` | 是 |
| js/css/i18n/note-html 生成输出 | 各自 writeFile | `0666 & ~umask` | 是 |
| 第一方恒等 copy（leaui_* plugin、静态资源） | copyFile | 源文件（即 checkout 后的索引 mode） | 否 |

node_modules tarball 的 mode 不会传播到输出（全部上游 tinymce 输出走 writeFile）；copyFile 分支的源全部是仓库内受跟踪文件。

## Audit By Requirement Dimension

### 目标与方向歧义（关键缺陷 D1）

原 Requirement 1 写"预期 mode（100755 或 100644）及其来源"，未固定方向。字面执行者可能反向修复（让构建产出 755 以匹配索引）。依据以下证据，规范方向唯一可为 **100644**：

- 这些文件是 HTTP 静态资源，无任何消费者依赖 exec 位（Revel 静态服务、`sh/package.sh` 打 tar 均不需要）。
- Windows 开发机上 `fs.chmod` 无法置 exec 位，"构建产出 755" 在本地不可复现，直接违反 A/B 协议。
- CI 的 writeFile 分支已经产出 644；`BUILD_OUTPUTS` 共 164 个，反向修复等于让其余 142 个已正确为 644 的输出迁就 22 个历史遗留位，无合理性。
- 修复机制应含 VCS 侧：`git update-index --chmod=-x`（Windows 可用的索引级操作），而非 `chmod` 工作树文件。

已修复：PRD 新增"Mode Contract"节固定全部 `BUILD_OUTPUTS` 为 `100644`。

### 范围缺口（D2）

原 Confirmed Defect 只提 CI 观察到的漂移，未列清单、遗漏 leaui_mindmap 潜在 2 文件。已修复：清单入本研究文档，PRD 引用 22 文件计数与指针。

### 机制约束缺失（D3）

原规格未提 umask。`fs.writeFile` 的创建 mode 被 umask 掩蔽（例：umask 027 时 0644→0640），且 copyFile 分支依赖源文件 mode。若不显式固定，A/B 快照的"POSIX mode 一致"依赖运行环境巧合。已修复：PRD 要求构建对每个输出**显式**固定 0644，且不依赖进程 umask、源文件 mode 或 node_modules tarball（写入后 chmod 或等效手段；chmod 不受 umask 掩蔽）。

说明：仅靠索引规范化也能过 CI git 门禁（git 只跟踪 exec 位，0666 基数永不产生 exec 位），但不满足"声明 mode 契约"的可测试性，且 A/B 的 mode 逐项比较在跨环境时会假阳性。故构建侧强制固定为必要项而非可选。

### 回归用例缺失（D4）

`tests/js/build-pipeline.test.js`（570 行、36 个用例）覆盖闭包/回滚/symlink/i18n，**无任何 POSIX mode 断言**。AGENTS.md 要求每个修复附聚焦回归用例。已修复：PRD 新增回归用例要求（锁定 mode 契约声明与执行，POSIX 断言须跳过或改写 win32 等价物，保证跨平台安全）。

### 验收标准缺口（D5/D6）

- 原 AC4 "git diff 证明没有手工修改受跟踪 bundle" 有歧义：修复提交本身必然携带 22 个文件的索引 mode 变更。已改为：**内容 SHA-256 相对修复前 HEAD 不变，仅允许 mode 位 100755→100644**；修复提交后的干净 checkout 构建留下空 diff/status。
- 原 AC 只比"一致"不锚绝对值（两次都 755 也算"一致"）。已补：A/B 两份快照中每个输出 mode 均为 100644。

### 环境与 provenance（D7）

- 原 Req 2 未说明 A/B 执行环境。已明确：A/B 双 checkout 为工程证据（须记录 OS、umask、Node/npm 版本，`npm ci` 而非 `npm install`），最终验收绑定修复提交的 CI node-build job（证据等级高于本地）。
- Node 版本锚定 `24.20.0`（CI setup-node 钉死该版本；`engines >=24 <25` 兼容）。

### Windows 兼容（D9）

索引级 `--chmod=-x` 与构建内 chmod 在 Windows 均安全（后者对 exec 位为 no-op）；测试不得在 win32 断言文件系统 POSIX mode。已写入 PRD 兼容性要求。

### 元数据缺陷（D8）

`task.json` 的 `base_branch: "master"` 与实际集成分支矛盾：E 在 `dev` 上 in_progress，现代化根任务 base_branch=dev，近期提交均在 dev。已用 `set-base-branch` 改为 `dev`，防止实现者从 master 拉分支。

### 交接与基线（D10）

已明确：B-E2..B-E6 的复验基线是**修复提交 SHA**（非旧候选 `fcc979b…`）；E evidence matrix 的候选重置由 E 自行完成，不属本任务。

## 规格修复清单（对 PRD 的改动）

1. 新增 Mode Contract 节：全部 `BUILD_OUTPUTS` 规范 mode 为 `100644`；VCS 侧 `git update-index --chmod=-x` 规范化 22 个文件；构建侧显式固定输出 0644 且不依赖 umask/源 mode。
2. Confirmed Defect 更新：补 job URL、20+2 清单指针、根因一段（指向本研究文档）。
3. Requirements 重排为 R1-R7：mode 契约、索引规范化、构建确定化、A/B 协议（含 umask/版本记录）、URL/路径/模板契约保持、回归用例、provenance 记录。
4. Acceptance Criteria 重写：绝对 mode 断言、A/B 快照逐项一致、空 diff/status、内容 hash 不变仅 mode 位变化、CI node-build 全绿、回归用例存在且通过、无未声明产物。
5. Out Of Scope 补充：不改 E 验收矩阵/候选基线（E 自行重置）；不改 quality-gate 门禁语义。
6. Handoff 明确下游以修复提交 SHA 为新基线。
7. task.json base_branch master→dev。

## 无需用户确认的判断

规范方向 100644、22 文件清单、构建侧强制固定、回归用例要求均可由仓库证据唯一确定（见上文论证），未发现必须交用户裁决的产品/范围问题。

## 审核过程 provenance

- 2026-09-01 独立 code-review（Standards/Spec 双轴）后修复：研究文档用例数 24→36、"325 个文件"→164 输出中其余 142 个；task.json 补回行尾换行；jsonl 移除 workflow 代码文件条目；PRD 压缩重复根因链；新增 implement.md 补齐复杂任务启动护栏。审核后复核确认"根任务 8 个 child"表述正确（Spec 代理该项为误报）。

## Activation follow-up (2026-09-01)

用户以"批准激活任务"明确批准；`task.py start` 重运行以持久化当前任务指针（首次运行处于 degraded 模式未落指针），状态保持 `in_progress`。据此进入实现阶段；本审核记录中"未触碰业务实现"仅描述审核阶段。

- `git ls-files -s` 全量扫描 manifest 输出目录的索引 mode 分布。
- `gh run view 33477561244 --job 99759909194 --log` 提取全部 `diff --git` 行，逐文件核对漂移清单。
- 通读 `scripts/build/{manifest,js,index}.mjs`、`tests/js/build-pipeline.test.js`、`.github/workflows/quality-gate.yml`（node-build job）、E 的 `prd/design/implement`、`acceptance/evidence-matrix.md`、`research/spec-audit-2026-09-01.md`。
- 本地 `git config core.filemode` = false（Windows），佐证本地不可见性论证。
- 未修改任何业务实现、测试、workflow、生成资源；未动 E/父任务材料（E 已激活，规划审核例外失效）。
