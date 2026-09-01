# B-E1 修复前端构建产物 mode 漂移

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E1　优先级：P0　执行序号：1　责任 owner：frontend-build-chain / TinyMCE 生成闭包

## Goal

修复候选提交中 Node 构建生成文件的 POSIX mode 漂移，使同一输入在干净 Linux checkout 中可重复生成完全相同的受跟踪资源。该门禁是后续 E2E、真实浏览器和发布验收的基础，不以修改验收表或忽略 mode 差异关闭。

## Confirmed Defect

- CI `33477561244` / attempt `1` 的 `node-build` 因 TinyMCE 插件生成文件从 `100755` 漂移为 `100644` 失败。
- 生成资源必须继续由 `scripts/build/manifest.mjs` 与 `app/views/note/note-dev.html` 产生，禁止手工修补 bundle。

## Dependencies And Order

- 无前置子任务；本任务完成后才进入 B-E3 的稳定 E2E 复验。
- 失败时责任返回构建链或生成闭包 owner，不由 E 协调任务绕过。

## Requirements

1. 明确 manifest 声明的全部输出、预期 mode（`100755` 或 `100644`）及其来源；只修复导致漂移的生成/发布路径。
2. 在 Node `24.20.0` 的干净 Linux checkout A/B 中分别执行 `npm ci && npm run build`，比较输出集合、原始字节 SHA-256、POSIX mode、tracked diff 与 non-ignored untracked 状态。
3. 构建必须保留既有公开 URL、文件路径和服务端模板契约；不得引入第二运行时、CDN fallback 或静默忽略 `git diff`。
4. 记录候选 SHA、Node/npm 版本、命令、输出快照摘要和 CI run/job provenance；快照不得包含 `node_modules` 或敏感数据。

## Acceptance Criteria

- [ ] A/B 两次构建的输出集合、每个文件字节 hash 和 mode 全部一致。
- [ ] 两个 checkout 的 `git diff --exit-code` 与 `git status --porcelain --untracked-files=all` 均为空。
- [ ] `npm test` 及候选提交 `node-build` job 通过，失败时保留原始 owner、run/job 和复验命令。
- [ ] `git diff` 证明没有手工修改受跟踪 bundle，且无 jquery-migrate、旧 runtime 或未声明产物进入生产资源。

## Out Of Scope

- 不处理 Mongo、Chromium、真实浏览器或 package/container 门禁。
- 不由本任务修改 E 验收矩阵、F workflow 或父任务状态来掩盖失败。

## Handoff And Retest

完成后提供可复用的构建快照和 CI job 链接给 B-E2～B-E6；任一 hash/mode 漂移均回到本任务 owner 修复后重新执行完整协议。
