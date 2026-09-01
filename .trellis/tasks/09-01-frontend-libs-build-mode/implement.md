# B-E1 实施计划

## Task 0: Activation Gate

- [x] 规格审核完成并通过独立 code-review（Standards 4 项 + Spec 2 项有效发现均已修复，范围合规）。
- [x] 用户已明确批准激活（"批准激活任务"，2026-09-01）；`task.py start` 以 `TRELLIS_CONTEXT_ID` 持久化当前任务指针，状态 `in_progress`。
- [x] branch=base_branch=`dev`（仓库归档先例，PR 目标 dev 需另行拉分支时由收尾流程处理）。

## Global Constraints

- 规格审核已完成（`research/spec-audit-2026-09-01.md`），任务已激活；本计划进入实现阶段，允许修改 `scripts/build/` 与 `tests/js/`，但禁止手工修补受跟踪 bundle 内容（AC 要求内容 SHA-256 不变）。
- 修复范围严格限于 Mode Contract：22 个索引 mode 位、构建侧输出 mode 固定、聚焦回归用例；不触碰清单外任何文件的 mode。
- 提交在 `dev` 上进行（branch=base_branch=`dev`，仓库归档先例）；每提交聚焦单一目的。

## Task 1: 构建侧 mode 固定

- [x] 在 `scripts/build/` 的 staging 写入路径上对每个输出显式固定 `0644`（写入后 chmod 或等效），确保不依赖 umask 与源文件 mode；机制依据见研究文档"mode 传播矩阵"。
- [x] 本地验证：Windows 上 `npm run build` 行为不变（chmod 对 win32 exec 位为 no-op）、`npm test` 全绿。

## Task 2: 回归用例

- [x] 在 `tests/js/build-pipeline.test.js`（或同层新文件）增加 mode 契约用例：全部 `BUILD_OUTPUTS` 声明为非可执行，且构建实现包含显式固定机制；POSIX 文件系统断言在 win32 跳过或改写为跨平台等价断言。

## Task 3: 索引规范化与提交

- [x] 对研究文档列出的 22 个文件执行 `git update-index --chmod=-x`；用 `git diff --cached` 确认仅 mode 位变化（`100755`→`100644`），内容字节不变。
- [x] 提交修复（构建侧 + 测试 + 索引 mode 位），保持每提交单一目的。

## Task 4: A/B 验证（Route A：CI 证据 + 用户批准的规格偏差，2026-09-01）

> 偏差记录：字面"双 checkout + checkout 外快照逐项比较"在本机不可实施（WSL 仅有 Windows 互操作 npm，无原生 Linux Node）；用户批准以 CI 证据等价替代（"采用推荐方案"）。等价性论证：checkout A ≡ CI 在全新 runner checkout 上构建（零 diff 证明发布树与索引逐字节一致，Linux `core.filemode=true` 下 mode 亦同）；checkout B ≡ POSIX 回归用例在独立临时副本树中的完整构建，在敌对 umask 0o077 与 0755 源文件下断言全部 164 个输出 mode==0644。两树均为同一 lockfile（npm ci）与钉死 esbuild 0.28.2 的确定性函数。

- [x] checkout A（[run 33519988846](https://github.com/yangphere/leanote/actions/runs/33519988846)，`c903007`）：`npm ci && npm run build` 后 `git diff --exit-code` ✓、`git status --porcelain` 空 ✓。
- [x] checkout B（[run 33522450969](https://github.com/yangphere/leanote/actions/runs/33522450969) node-build 内 POSIX 用例）：独立副本树全量构建，逐输出断言 mode==`0o644`，8.3s 执行通过。
- [x] 版本记录：Node 24.20.0（CI setup-node 钉死）、npm 11.x、umask 以 0o077 敌对值验证；无敏感数据。
- [x] 修复提交（`99abfab`，含 `c903007` 变更）的 [node-build job](https://github.com/yangphere/leanote/actions/runs/33522450969/job/99904830024) 全绿：零漂移 + npm test 121/121。

## Task 5: Provenance 与交接

- [x] 修复提交：`c9030072686335c57dbb2d4a383b240070d10218`（mode 契约：构建侧 chmod + 22 文件索引规范化 + POSIX 回归用例）；测试封闭化修复：`99abfab`（F 契约测试 CI 环境泄漏，见 `09-01-release-contract-hermetic-env`）。
- [x] CI 链路：[run 33519988846](https://github.com/yangphere/leanote/actions/runs/33519988846) 证明零漂移门禁通过（npm test 被 F 契约缺陷阻断，已由独立任务修复）；[run 33522450969 / job `99904830024`](https://github.com/yangphere/leanote/actions/runs/33522450969/job/99904830024) node-build 全绿（121/121）。
- [ ] 通知 E：AC-E3 的 retest 输入已就绪（候选基线重置为 `99abfab` 谱系）；B-E2..B-E6 以该提交为新基线，evidence matrix 重置由 E 执行；两任务归档均需用户确认。

## Completion Gate

- [x] PRD 全部 AC 勾选；无手工 bundle 内容修改；22 文件清单之外无任何 mode 变化（`c903007` diff 已核）。
