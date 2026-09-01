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

## Task 4: A/B 双 checkout 协议（Linux，Node 24.20.0，`npm ci`）

- [ ] checkout A：`npm ci && npm run build`；在 checkout 外保存输出集合、每文件 SHA-256、POSIX mode 快照；`git diff --exit-code` 与 `git status --porcelain --untracked-files=all` 均为空。
- [ ] 从同一修复提交新建 checkout B 重复构建并生成第二份快照，逐项比较：集合、hash、mode 全等且均为 `100644`。
- [ ] 记录 OS、umask、Node/npm 精确版本；快照不含 `node_modules` 与敏感数据。
- [ ] 推送后在修复提交上确认 CI `node-build` job 全绿（含 `npm test` 与零漂移）。

## Task 5: Provenance 与交接

- [ ] 在任务材料中记录修复提交 40 位 SHA（已记录 `c903007`，待补 push 后的 CI run/job URL）、A/B 环境参数、CI run/job URL。
- [ ] 通知 E：AC-E3 的 retest 输入已就绪；B-E2..B-E6 以修复提交为新基线，evidence matrix 重置由 E 执行。

## Completion Gate

- [ ] PRD 全部 AC 勾选；无手工 bundle 内容修改；22 文件清单之外无任何 mode 变化。
