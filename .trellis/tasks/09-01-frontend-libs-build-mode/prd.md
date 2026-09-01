# B-E1 修复前端构建产物 mode 漂移

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E1　优先级：P0　执行序号：1　责任 owner：frontend-build-chain / TinyMCE 生成闭包
基线分支：`dev`

## Goal

修复候选提交中 Node 构建生成文件的 POSIX mode 漂移，使同一输入在干净 Linux checkout 中可重复生成完全相同的受跟踪资源。该门禁是后续 E2E、真实浏览器和发布验收的基础，不以修改验收表或忽略 mode 差异关闭。

## Confirmed Defect

- CI run `33477561244` / attempt 1 的 [node-build job `99759909194`](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759909194) 因 `git diff --exit-code` 失败：20 个上游 TinyMCE 官方插件文件（advlist、autolink、charmap、directionality、link、lists、searchreplace、table、visualblocks、visualchars 的 `plugin.js` + `plugin.min.js`）由 `100755` 漂移为 `100644`。
- 根因与逐文件证据链见 `research/spec-audit-2026-09-01.md`：上游插件输出经 staging 写入并发布后为 `100644`，而 git 索引仍按历史 vendored 状态跟踪 `100755`；另有 `public/tinymce/plugins/leaui_mindmap/plugin.js|.min.js` 2 个文件同样跟踪 `100755`，仅因恒等复制保留源 mode 而未在本次 CI 漂移。
- 生成资源必须继续由 `scripts/build/manifest.mjs` 与 `app/views/note/note-dev.html` 产生，禁止手工修补 bundle。

## Mode Contract

- 全部 `BUILD_OUTPUTS`（manifest 声明的每一个输出）的规范 POSIX mode 为 **`100644`**（无可执行位）。这些文件是 HTTP 静态资源，无任何消费者依赖 exec 位；Windows 开发机上构建也无法产出 exec 位，反向修复（让构建产出 755）不可实施。
- VCS 侧：以 `git update-index --chmod=-x` 将当前跟踪为 `100755` 的全部 22 个 manifest 输出规范化为 `100644`；不使用工作树 `chmod`（Windows 不可用）。
- 构建侧：构建对每个输出**显式**固定 `0644`，且不依赖进程 umask、源文件 mode（含 node_modules tarball 与 checkout 现状）；可行机制与依据见研究文档 D3。

## Dependencies And Order

- 无前置子任务；本任务完成后才进入 B-E2 的 harness 收口与 B-E3 的稳定 E2E 复验。
- 失败时责任返回构建链或生成闭包 owner，不由 E 协调任务绕过。

## Requirements

1. **mode 契约落地**：按 Mode Contract 完成索引规范化与构建侧 0644 固定；修复只涉及生成/发布路径与对应索引 mode 位，不改任何输出的内容字节。
2. **A/B 双 checkout 协议**（对齐父任务 implement.md Task 2）：在 Node `24.20.0`、`npm ci`（禁用 `npm install`）的两个全新 Linux checkout 中分别执行 `npm ci && npm run build`；第一次构建后在 checkout 外保存输出集合、每文件 SHA-256 与 POSIX mode 快照，第二次从同一修复提交 SHA 重建并逐项比较集合、字节 hash、mode、`git diff --exit-code` 与 `git status --porcelain --untracked-files=all`。记录 OS、umask、Node/npm 精确版本；快照不含 `node_modules` 与敏感数据。
3. **既有契约保持**：构建必须保留既有公开 URL、文件路径和服务端模板契约；不得引入第二运行时、CDN fallback 或静默忽略 `git diff`。
4. **回归用例**（AGENTS.md 测试准则）：在 `tests/js/build-pipeline.test.js`（或同层新文件）增加聚焦回归用例，锁定 mode 契约——全部声明输出的规范 mode 为非可执行，且构建实现包含显式固定机制；POSIX 文件系统断言在 win32 上必须跳过或改写为跨平台等价断言，不得破坏本地 `npm test`。
5. **Windows 兼容**：索引级 mode 变更与构建内 chmod 对 Windows 开发机构建均为安全操作；修复后本地（win32）`npm run build && npm test` 行为不变。
6. **provenance**：记录修复提交 40 位 SHA、分支、工作树状态、本地 A/B 环境参数和 CI run/job URL；下游 B-E2..B-E6 以该修复提交为新基线。

## Acceptance Criteria

- [ ] A/B 两次构建的输出集合、每个文件字节 hash 和 mode 全部一致，且两份快照中每个输出 mode 均为 `100644`。
- [ ] 两个 checkout 的 `git diff --exit-code` 与 `git status --porcelain --untracked-files=all` 均为空。
- [ ] 修复提交相对修复前 HEAD：全部受跟踪 bundle 的内容 SHA-256 不变，仅允许上述 22 个文件的索引 mode 位由 `100755` 变为 `100644`；无手工修改的 bundle 内容。
- [ ] 修复提交的 CI `node-build` job 全绿（含 `npm test`），失败时保留原始 owner、run/job 和复验命令。
- [ ] 回归用例存在、被 `npm test` 发现并执行、在 Linux CI 与 Windows 本地均通过。
- [ ] 无 jquery-migrate、旧 runtime 或未声明产物进入生产资源（`git diff` 与 manifest 输出集合共同证明）。

## Out Of Scope

- 不处理 Mongo、Chromium、真实浏览器或 package/container 门禁（B-E2..B-E6 各自负责）。
- 不由本任务修改 E 验收矩阵、候选 SHA 基线、F workflow、quality-gate 门禁语义或父任务状态；E 的 evidence matrix 重置由 E 在收到本任务证据后自行完成。
- 不改动 22 个清单之外任何文件的 mode（含非 manifest 输出的 `sh/*.sh`、脚本等可执行文件）。

## Handoff And Retest

完成后向 B-E2～B-E6 提供修复提交 SHA、可复用的构建快照与 CI job 链接；下游全部以修复提交为新基线复验。任一 hash/mode 漂移均回到本任务 owner 修复后重新执行完整协议。
