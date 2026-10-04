# E 组合验收证据矩阵

更新时间：2026-09-02（终局收口）
候选提交谱系：`fcc979bb…`（E 激活时）→ `7aaa2927`（B-E1..B-E6 修复链终点）；终局全绿证据绑定 [run 33637319776](https://github.com/yangphere/leanote/actions/runs/33637319776)（`7ccb4d6c`）
任务状态：**BLOCKED（终局）——唯一剩余阻断为 AC-E6 的 8 槽真实浏览器 artifact（B-E5 用户裁决跳过 Safari）**

本文件只记录脱敏证据。除明确标为 `passed-local` 的本地辅助结果外，所有通过结论都必须绑定候选提交和可复核的命令或 run/job URL。父任务进度、子任务归档状态和本地局部通过不能覆盖候选提交的 `failed`、`blocked` 或 `missing`。

## 状态枚举与聚合

- `passed`：候选提交的必需证据完整，命令/运行、发现数、执行数、结果和 provenance 均可复核。
- `failed`：已执行且观察到失败、超时、清理失败或错误门禁；必须保留原始失败类别，不能降级。
- `blocked`：前置环境/契约冲突或未决决策使门禁不能合法执行；不是成功或跳过。
- `partial`：有部分证据，但缺少 AC 要求的字段、覆盖或同一提交绑定，不能关闭门禁。
- `passed-local`：本地辅助检查通过，但不具备候选提交 CI、真实服务或真实浏览器的证明力。
- `missing`：要求的运行或记录尚不存在；不得当作 `not_run` 的成功等价物。

AC 聚合规则：同一 AC 的所有行必须为 `passed` 才能聚合为 `passed`；任一 `failed`、`blocked` 或 `missing` 聚合为 `blocked`；仅含 `partial`/`passed-local` 的 AC 聚合为 `partial`，而 `partial` 在任务级一律映射为 `blocked`，不能作为完成状态。最终任务只有 AC-E1..E9 全部 `passed` 且 Q-E1 所选模式满足时才可为 `eligible_for_completion`，否则只能为 `blocked`。

AC-E6 的两行是同一个“所选 Q-E1 模式”门禁，而不是同时要求候选和 tag artifact；用户选择模式后，只验收该模式对应的 producer/validator。Q-E1 未确认前两行保持当前的 `missing`/`blocked`，任务不得激活。

本轮执行记录见 `research/execution-evidence-2026-09-01.md`。所有本地命令在候选 SHA、Node
`v24.20.0`/npm `11.19.0` 上完成，且业务树（排除 `.trellis/tasks`）为空；没有启动 Mongo、E2E harness
或真实浏览器。`--list` 只计 discovery，不能冒充用例执行。

## 逐行证据

## 浏览器证据载体

| 模式 | 受保护 artifact | 固定载荷 | 身份绑定 | 允许作为 E 完成证据 |
|---|---|---|---|---|
| 候选预发布（Q-E1 不等待 tag） | `candidate-browser-matrix-v1` | `candidate-matrix.json` + `candidate-provenance.json` | 候选 SHA、非 tag ref、`candidate_run:{id,attempt}`（id 为非零十进制字符串，attempt 为正整数） | 是；不等同于发布 tag artifact；保留不超过 7 天 |
| tag 预检（Q-E1 等待模式，供 E 验收） | `browser-release-matrix-v1` | `release-matrix.json` + `provenance.json` | 严格 `vX.Y.Z` tag commit/ref、producer run/attempt；tag commit 必须等于候选 SHA | 是；仅供 E 验收，不创建 Release/GHCR、不改变 F 任务状态；保留不超过 7 天 |
| 正式发布（E 归档后） | `browser-release-matrix-v1` | `release-matrix.json` + `provenance.json` | 严格 `vX.Y.Z` tag commit/ref、最终 release run/attempt | 仅供 F 发布；须由最终 release run 重新生成并通过 F contract 的 coverage digest/schema；保留不超过 7 天 |

两种载荷均按固定顺序使用四个稳定 coverage ID、每槽位 `coverage_summary_sha256` 和 provenance 内嵌
`coverage_summaries`。摘要输入是去掉 digest 字段的 `{browser_product, release_slot, items}` 对象，
并以 RFC 8785 JCS（无空白、无尾随换行）计算 SHA-256；字段约束和 validator 规则以 E PRD/design 及 F
`research/release-matrix-contract.md` 为唯一来源；本表只保存脱敏结果，不保存浏览器原始 artifact。

| AC | Gate | Candidate commit | Command or run/job URL | Discovery / execution | Result / evidence | Status | Owner | Retest / close condition |
|---|---|---|---|---:|---|---|---|---|
| AC-E1 | ready selection + dependency | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `task.json`; archived D/child `task.json`; `git rev-parse HEAD`; `git status --porcelain`; `task.py validate` | N/A (selection/metadata gate) | E 已从选中时的 planning 激活为 in_progress；`meta.depends_on=[08-25-frontend-build-chain]`，D 与三个 child 均有 archived completed 记录。E 本轮本地记录和 CI run 33477561244 都绑定该 SHA；排除任务材料后业务树为空 | passed | E coordinator | 关闭；后续新增 E-owned 运行仍必须记录同一候选 SHA。等待模式的 tag commit 单列于 AC-E6 |
| AC-E2 | dependency/runtime uniqueness | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `npm ls --all --json`; `package-lock.json` `packages` 树；`MANIFEST` import/static scans | N/A (dependency/manifest gate; no test suite) | jquery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 各 1 安装路径；Migrate 3.6.0 仅 1 dev 路径且 manifest inputs=0。manifest 无 CDN；兼容 jquery URL 的 87533 bytes SHA-256 与 locked input 相同。详情见 execution research | passed | D / E-jQ / E-BS / E-TM | 已闭合：B-E1 修复 mode 漂移后 node-build 多轮全绿（干净 Linux checkout + 零漂移 + 121→130 契约测试含 migrate 排除/URL 兼容断言）；终局 run 33637319776 node-build success |
| AC-E3 | Node build and zero drift | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [node-build job](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759909194) | N/A (build/drift gate; npm test not reached) | TinyMCE 生成文件 mode 由 100755 漂移为 100644，`git diff --exit-code` failed | passed | D / E-TM 生成闭包（已闭合→B-E1） | B-E1 归档：`c903007` mode 契约（chmod 0644 + 22 文件索引规范化 + POSIX 回归用例）；Route A 双 checkout 等价验证；此后所有 run node-build 零漂移，含终局 33637319776 |
| AC-E3 | Node contracts (local aid) | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `npm test` | 120/120 | 本地测试通过，但不是候选提交 CI 或构建零漂移证据 | passed | E | 候选谱系 node-build 全绿（npm test 121→130 用例随修复链增长；win32 POSIX 用例按设计跳过）；本地辅助已由 CI 证据覆盖 |
| AC-E4 | Playwright business discovery | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `playwright test --config=playwright.config.mjs --project=business --list` | 22 tests / 5 files；包含 ajax-failure、bootstrap-components、business-flows、editor-flows、jquery-diagnostics | 2026-09-01 本地 `--list` 以 0 退出；仅证明五套件发现，未执行用例主体。`browser-smoke` 覆盖另列于 AC-E6 | passed | E / E2E | 已闭合（B-E3/B-E4）：chromium-e2e 全绿 [run 33637319776]；business 22 用例/6 文件（B-E4 迁移后）、build-smoke 先行 1/1、清理摘要完整 |
| AC-E4 | Playwright build-smoke discovery | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `playwright test --config=playwright.config.mjs --project=build-smoke --list` | 1 test / 1 file | 2026-09-01 本地 `--list` 以 0 退出，指定 `build-resource-smoke.spec.mjs` 已发现；未取得候选提交级执行/清理结果 | passed | D / E2E | 已闭合：build-smoke 1/1 先行执行（chromium-e2e job 串行协议，多轮全绿）；资源门禁与 cleanup 断言在 suite 内 |
| AC-E4 | Chromium E2E | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [chromium-e2e job](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759999476) | 22 discovered / 19 passed before failure（job log）；failure summary 的计数为 N/A，且错误标为 job_not_started | 3 failed；`leaui_image` 出现 `editor.on is not a function`，随后业务/编辑器超时并清理失败。下载的 summary 与已启动 job 冲突，不能覆盖 job 事实 | passed | E-TM / E2E（已闭合→B-E3） | B-E3 归档：四层修复（shell 事件 API、md 条件类、潜伏缺陷、undoManager 竞态）后 chromium-e2e 持续全绿（33589413738 起），终局 33637319776 success；failure summary stage 语义由 B-E6 修复（artifact 实测 passed/complete） |
| AC-E5 | Go 1.26/1.27 external evidence | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [quality-gate run](https://github.com/yangphere/leanote/actions/runs/33477561244) | N/A (external job summaries not captured) | 两个 Go job 成功；属于 E 消费的外部证据，不是 E-owned allowlist，尚缺逐 job summary 计数 | passed | Backend / G | go-1_26_7 与 go-1_27_0 终局 run 33637319776 均 success（外部证据按 owner 记录） |
| AC-E5 | Mongo/Golden/USN/ownership/note-save | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [mongo-8_0 job](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759909276) | N/A (test blocked before discovery by port conflict) | CI service 已占用 27017，`NewMongoEnvironment.Up()` 又启动固定端口容器，套件失败 | passed | G / harness（已闭合→B-E2） | B-E2 归档：三态判定（REQUIRE=1⇒service-backed 零 docker；未设⇒自建；supervisor 恒自建+端口预检）；mongo-8_0 从 33576516744 起持续全绿，终局 33637319776 success |
| AC-E6 | real browser 8-slot execution (selected Q-E1 tag-precheck mode) | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `browser-release-matrix-v1` 受保护 tag-precheck producer；[CI run 33477561244](https://github.com/yangphere/leanote/actions/runs/33477561244) artifact API | N/A（没有 strict tag、受保护 runner 或 discovery/execution counts） | 本地及 `origin` 均没有 tag 指向候选；候选 push run 只含 8 个 `ci-summary-*` artifact、没有 browser matrix。不得用 Playwright Chromium/WebKit 代替真实四产品两主版本 | blocked | F / protected browser owner（B-E5 blocked 形态交付） | B-E5 归档：用户裁决跳过 Safari（2026-09-02"跳过macOS验证"）；三 current 槽（Chrome 152/Edge 152/Firefox 153）四套件 16/16 工程证据全绿（acceptance/engineering-evidence.md）；8 槽 artifact 因 producer 硬性要求含 Safari 无法产出。恢复条件：macOS/Safari 环境或契约修订授权 |
| AC-E6 | semantic coverage + provenance (selected Q-E1 mode) | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `scripts/browser-release-evidence.mjs`; `scripts/validate-browser-artifact.mjs` static contract scan | N/A（provenance gate；无有效 artifact） | 当前 producer 写入通用 `build-smoke/auth-gate/error-gate/resource-gate` 并允许 1--40 项；matrix 无 `coverage_summary_sha256`，provenance 无 `coverage_summaries`，validator 仍只接受旧六字段。未满足固定四 ID/JCS 摘要契约 | passed | F / release-evidence owner（已闭合→B-E4） | B-E4 归档：producer 四 ID+每行 coverage_summary_sha256+八槽 coverage_summaries+marker 协议；validator JCS 重算+final/precheck 双相位；契约测试 26/26（node-build 承载，终局全绿）。实际 artifact 生成待 AC-E6 第一行环境恢复 |
| AC-E6 | tag-precheck lifecycle isolation | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `.github/workflows/release.yml`; `.github/workflows/browser-release-evidence.yml` static flow scan | N/A（workflow lifecycle gate） | browser workflow 仅 `workflow_call`；release tag flow 在 quality-gate 后产 artifact，随即由 `publish` 消费并可 GHCR push/Create Release。不存在“仅预检、不发布、不改 F 状态”的独立受保护入口 | passed | F / release-evidence owner（已闭合→B-E4） | B-E4 归档：browser workflow 增 workflow_dispatch 仅预检入口（严格 tag+剥壳+无发布步骤），结构断言测试锁定；两文件 allowlist、retention 7 保持 |
| AC-E7 | HTML/API/USN/ownership/schema/URL/theme | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | `npm run test:e2e`; `LEANOTE_GOLDEN=replay ... go test ...`（候选重跑入口） | N/A (aggregate compatibility gate; child counts tracked above) | Node 局部测试通过；Chromium 与 Mongo 失败；尚未证明未编辑零写入、失败 revision、API/USN/Schema、用户上传主题字节/路径不变 | blocked | E-TM / G / Backend | AC-E4/E5 已全绿（终局 run 33637319776：chromium-e2e 22/22 含未编辑零写入/revision/undo/只读语义；mongo-8_0 含 Golden/USN/所有权/note-save；node-build 130 契约测试含 jQuery/Bootstrap/TinyMCE 归档契约映射）；**本 AC 的聚仍因 AC-E6 真实浏览器层缺失而 blocked**——四 coverage 中 bootstrap-components/leaui-image-iframe 的真实浏览器执行未发生 |
| AC-E8 | E quality-gate allowlist | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [node-build](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759909194)、[chromium-e2e](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759999476)、[mongo-8_0](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99759909276) | N/A（job-status gate；run 33477561244/attempt 1 的 3 allowlist jobs） | node-build、chromium-e2e、mongo-8_0 均为 failed；E allowlist 未全绿。下载 failure summaries 把实际已启动的失败 job 写成 job_not_started，不能降低或掩盖失败 | passed | E coordinator（已闭合→B-E1..B-E6 修复链） | 终局 [run 33637319776](https://github.com/yangphere/leanote/actions/runs/33637319776)：node-build、chromium-e2e、mongo-8_0 全部 success；summary stage 语义修复后 artifact 实测 passed/complete/11-11 |
| AC-E8 | F/parent downstream jobs (external) | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [package-smoke](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99760156757)、[container-smoke](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99760157037) | N/A (job-status gate; package/container) | package 因 `dev` 被当 release tag 失败；container 因 RFC3339 值传给 `SOURCE_DATE_EPOCH` 失败；不属于 E-owned allowlist，但阻断 F/父任务发布 | passed | F / parent（已闭合→B-E6） | B-E6：D1 tag 判定、D2 整型 epoch（三处）、ERE 非捕获组真根因等七层修复；终局 run 33637319776 package-smoke、container-smoke 均 success |
| AC-E8 | summary (external) | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | [summary job](https://github.com/yangphere/leanote/actions/runs/33477561244/job/99762299589) | N/A（job-status gate；summary 预期聚合 7 jobs，但失败 summary 的计数不可用） | summary job 在 Validate every quality summary 步骤失败；下载 artifact 也写 job_not_started，和 job step 冲突。package/container 仍保留为 F/父任务失败，不成为 E 通过证据 | passed | F / parent / quality-summary owner（已闭合→B-E6） | B-E6：write-summary stage 语义修复（失败≠job_not_started）；终局 run 33637319776 summary success |
| AC-E9 | evidence completeness | `fcc979bb9f0fe35d1771b00665017e470e2182d4` | 本文件逐行命令/run URL；`research/execution-evidence-2026-09-01.md` | N/A（metadata completeness gate） | 已记录本轮候选/版本/完整依赖路径/3 project discovery/CI run attempt/artifact 名称与静态 producer 缺口。当前仍含 failed/blocked/missing/partial 行；没有敏感数据 | partial | E coordinator | 终局：AC-E1..E5、E8 全 passed；AC-E6 第一行（8 槽执行）blocked 且不可在本环境闭合；AC-E7 因 AC-E6 连锁 blocked。所有行已填入最终 run/证据/owner/retest；脱敏检查通过。**任务终局=blocked**（唯一根因：8 槽真实浏览器 artifact 缺失） |

## Downstream reconciliation

- F（`08-25-cicd-delivery`）已归档 completed，但其 notes 仍声称上游真实证据阻断；这不是 E 的通过证据。F 的 tag-bound artifact 与 E 候选提交证据必须分别记录。
- 同一 run 的 package/container 失败仅作为 F/父任务下游状态保留；E 的 allowlist 不包含这两个 job，但父任务不得据此声称整体质量门通过。

## Q-E1 conditional gate

- **等待 tag artifact 模式（当前选择）**：受保护 producer 先在严格 `vX.Y.Z` tag 指向候选 SHA 后生成仅供 E 验收的 `browser-release-matrix-v1` 预检 artifact；该运行不创建 Release/GHCR、不改变 F 任务状态，也不是 Trellis 任务依赖。E 必须校验候选 SHA 与 tag commit 分列且相等；E 归档后，F 在最终 release run 中重新生成正式 artifact。
- **不等待 tag artifact 模式（推荐）**：E 只在 `candidate-browser-matrix-v1` 的真实四浏览器两版预发布证据、语义 coverage 摘要及 AC-E1..E9 其余门禁闭合后完成；F artifact 仍是独立发布阻断门，缺失它不得宣称发布获批。

### 终局结论（2026-09-02）

**BLOCKED（终局）**。AC-E1..E5、AC-E8 全部 passed（终局全绿 [run 33637319776](https://github.com/yangphere/leanote/actions/runs/33637319776)，八 quality-gate job 首次同轮 success）；AC-E6 的 producer/validator/预检入口（第二、三行）已由 B-E4 交付并通过，但**第一行（8 槽真实浏览器执行）因 B-E5 用户裁决跳过 Safari 而不可闭合**——producer 契约硬性要求恰好 8 槽含 Safari。AC-E7 随 AC-E6 连锁 blocked。

唯一恢复路径：①macOS/Safari 受保护环境 + 严格 `vX.Y.Z` tag → 预检 artifact → 8 槽全绿；或②用户授权契约修订（缩槽或 Safari 替代规则）。两条路径均不要求重开 B-E1..B-E4/B-E6 的任何修复。

## Evidence hygiene

允许：候选/运行 commit SHA、run/job ID/URL、工具完整版本、OS、稳定 coverage ID、发现/执行数量、结果、脱敏错误类别、owner 与复验命令。
禁止：密码、token、Cookie、认证头、storage state、页面正文、用户数据、截图、视频、trace 和未脱敏服务日志。
