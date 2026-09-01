# 前端库现代化（E）— 协调验收计划

## Global Constraints

- 规格审核阶段在获得最新规划摘要的明确批准前不运行 `task.py start`；任务激活后不重复运行该命令，除非任务生命周期另行批准。
- 本任务不修改生产代码、测试、构建脚本、workflow 或生成资源；发现实现问题只更新验收材料并返回 owner。
- 所有证据绑定同一候选提交；发现数为零、失败、超时、清理失败或缺环境均不得降级。
- 规划审核阶段如需同步父任务 DAG/生命周期事实，只能对父 PRD 做最小说明性更新；如需同步 E↔F 证据契约，只能更新 F 的任务规格、研究、验收材料或 task notes，且不得修改实现、测试、workflow、生成资源或任务状态；任务激活后上述例外均失效。

## Task 0: Activation Gate

- [x] 用户已确认 Q-E1 采用等待 tag artifact。受保护 producer 先在严格 tag 指向候选 SHA 后生成仅供 E 验收的 `browser-release-matrix-v1` 预检 artifact；该运行不创建 Release/GHCR、不改变 F 任务状态，也不新增 Trellis 任务依赖。E 归档后，F 再在最终 release run 中重新生成正式 artifact。
- [x] 用户已通过“启动功能实现”批准最新 Goal/In Scope/Out of Scope/AC/风险摘要。
- [x] 激活前已核对 `08-25-frontend-libs` 为 `planning`，依赖 D 及三个子任务均有归档 `completed` 证据；激活后以 `in_progress` 为当前生命周期事实。
- [x] 激活前 `task.py validate` 已通过，随后由主流程运行 `python ./.trellis/scripts/task.py start 08-25-frontend-libs`；当前任务指针为该任务。

## Task 1: Freeze Candidate And Inventory

- [ ] 记录候选提交的 40 位 SHA、分支、工作树状态和对应 GitHub run URL/ID。
- [ ] 核对三个子任务归档路径、状态、实现提交和未闭合外部证据；不得只看 `[3/3 done]`。
- [ ] 运行本地依赖与测试发现：

```powershell
npm ls --all --json > (Join-Path $env:TEMP 'leanote-npm-tree.json')
& .\node_modules\.bin\playwright.cmd test --config=playwright.config.mjs --project=build-smoke --list
& .\node_modules\.bin\playwright.cmd test --config=playwright.config.mjs --project=business --list
& .\node_modules\.bin\playwright.cmd test --config=playwright.config.mjs --project=browser-smoke --list
```

- [ ] 解析 `$env:TEMP/leanote-npm-tree.json` 及 `package-lock.json` 的完整 `packages` 树，逐一列出 jQuery/Bootstrap/TinyMCE 的所有安装路径；每个库精确版本且解析实例数为 1。`npm ls --depth=0` 只能作为辅助输出，不能证明嵌套依赖唯一性。
- [ ] 将发现文件、数量、缺失 suite 和依赖树路径写入 `acceptance/evidence-matrix.md`；临时 `leanote-npm-tree.json` 必须在任务材料之外保存或在记录后删除。

## Task 2: Clean Build And Static Contracts

- [ ] 在候选提交的干净 Linux checkout 使用 Node 24.20.0 执行：

```bash
npm ci
npm run build
git diff --exit-code
test -z "$(git status --porcelain --untracked-files=all)"
npm test
```

- [ ] 执行可复现的双 checkout 协议：checkout A 在第一次 `npm ci && npm run build` 后，在 checkout 外保存 manifest 输出集合、每个输出的 SHA-256 与 POSIX mode（100755/100644）；确认 `git diff --exit-code` 和 `git status --porcelain --untracked-files=all` 均为空。随后从同一候选 SHA 创建全新的 checkout B，重新执行 `npm ci && npm run build`，生成同样快照并逐项比较集合、字节 hash、mode 和 tracked/untracked 结果；任一差异均阻断并记录路径、旧/新 hash 或 mode。快照只保存脱敏元数据，不把 `node_modules` 或临时文件写回仓库。
- [ ] 证明 manifest/bundle 中无 jquery-migrate、旧核心、第二 runtime、CDN 或未声明 fallback。
- [ ] 任一漂移记录精确路径、旧/新 mode 或 hash，并归还生成闭包 owner；E 不修复。

## Task 3: Test-mode Integration And Backend Contracts

- [ ] 先选择并记录唯一 Mongo 模式：
  - service-backed Go/Mongo：使用 CI service `mongodb://127.0.0.1:27017/leanote_test`（或 `LEANOTE_TEST_MONGO_URL` 指向同一数据库），先恢复一次 fixture，运行 `LEANOTE_REQUIRE_MONGO=1 LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/... -count=1 -timeout 30m`；不得调用 `NewMongoEnvironment.Up()`。
  - Chromium 自建 harness：仅在没有外部 service/URI 时运行下方 supervisor，由它独占 `leanote-test-mongo`；不得同时启动 CI Mongo service。数据库必须为 `leanote_test`。
- [ ] 若同时发现 service 与 supervisor、URI 指向其他数据库、或任一 helper 试图绑定第二个固定 `27017`，在应用/测试启动前失败并记录为 blocked。
- [ ] 在 Chromium 自建 harness 模式中串行运行：

```bash
go run ./app/tests/harness/cmd/e2e -- sh -c 'npm run test:e2e:build && npm run test:e2e'
```

- [ ] Chromium 先执行 `build-smoke`（必须发现并执行 `build-resource-smoke.spec.mjs`），再执行 `business`；business 必须发现五个 spec 文件且全部通过。身份、权限、console/page/unhandled-rejection、owned-resource 和 cleanup 门禁均通过。
- [ ] 核对候选提交的 E allowlist job：`node-build`、`chromium-e2e`、`mongo-8_0`。`go-1_26_7`/`go-1_27_0`、`package-smoke`、`container-smoke`、`summary` 分别作为外部/下游证据记录 owner、run/job URL 和复验条件；失败 job 保留失败，不用本地通过覆盖。

## Task 4: Real-browser Evidence

- [ ] 对 Chrome、Edge、Firefox、Safari 的 current/previous stable 共 8 个产品/slot 运行真实产品。
- [ ] 每个 slot 按固定顺序执行并记录恰好四个稳定 coverage ID `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`，产生脱敏摘要（每个 ID 的 discovered/executed 数、入口/iframe 集合、结果）并记录摘要 SHA-256；摘要必须同时绑定同一不可变 run/attempt，且入口/iframe 只能是小写、无换行的稳定标识符；Safari 不接受 WebKit 替代。
- [ ] 按 Q-E1 等待模式校验受保护 producer 的 `browser-release-matrix-v1` 预检 artifact：恰好 8 行，tag commit/ref/run/attempt 与矩阵 SHA-256 完整，且 tag commit 必须等于候选 SHA；预检运行不得创建 Release/GHCR。E 归档后，F 最终 release run 必须重新生成并校验正式 artifact。两阶段均要求版本主号相邻、全部 gate passed、四个稳定 coverage ID 和每行 `coverage_summary_sha256`。
- [ ] 校验 provenance 的 `coverage_summaries` 恰好包含 8 个槽位、每槽四个稳定 ID 的正整数 discovered/executed（且 `executed_count <= discovered_count`）、入口/iframe 集合与 `passed` 结果；以去掉 digest 字段的 `{browser_product, release_slot, items}` 对象为输入，按 RFC 8785 JCS（键名排序、数组固定顺序、数字规范化、UTF-8、无空白和无尾随换行）重算 digest，并核对矩阵行、commit 和 run/attempt。当前固定通用 coverage 或旧 provenance 不足时保持阻断并交还 F owner。

## Task 5: Consolidate And Reconcile

- [ ] 更新 `acceptance/evidence-matrix.md`，每个 AC 有唯一证据或明确 blocker/owner/retest；状态枚举和聚合规则按矩阵文件定义执行。
- [ ] 记录 F 在 E 前归档以及 F notes 仍声称阻断的状态冲突；父任务收口不得声称 DAG 顺序已被证明。
- [ ] 只有 AC-E1 至 AC-E9 全部闭合且 Q-E1 所选模式的条件满足时才报告 `eligible_for_completion`；未闭合时只能报告 `blocked`。
- [ ] 未闭合时报告 `blocked`；不 commit、不 archive、不 push，除非用户另行授权对应阶段。

## Current Known Blockers

- `B-E1`：HEAD CI node-build 因 TinyMCE 插件生成文件 mode 从 100755 变 100644 而失败。
- `B-E2`：Mongo job 已提供 27017 服务，但 harness 测试再次启动固定端口容器，Golden/USN/所有权/note-save 失败。
- `B-E3`：Chromium business 19/22；`leaui_image` mock 触发 `editor.on is not a function`，主业务/编辑器流程随后超时且清理失败。
- `B-E4`：`browser-smoke` 未发现 Bootstrap component suite；release artifact 的通用 coverage 与具体测试摘要未绑定。
- `B-E5`：Bootstrap 与 TinyMCE 实浏览器记录未执行，jQuery 缺 Safari、各产品前一主版本及最终提交复跑。
- `B-E6`：F 已归档但当前 CI 的 package/container 门禁也失败，且 F metadata notes 与 completed 状态矛盾。

## Completion Gate

- [ ] 规划材料收敛、Q-E1 已确认且获得后续激活批准。
- [ ] AC-E1 至 AC-E9 在同一候选提交闭合，无 missing/failed/skipped/placeholder。
- [ ] 所有证据脱敏且 provenance 可复核；没有业务实现混入 E 的 diff。
