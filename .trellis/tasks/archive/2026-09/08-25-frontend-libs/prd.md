# 前端库现代化（E，协调收口）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

对已经归档的 jQuery 3.7.1、Bootstrap 5.3.8 与 TinyMCE 8.8.2 三个升级结果做一次**同一提交上的组合验收**，证明版本、生成资源、真实业务交互、编辑器数据、后端契约与浏览器支持矩阵能够共同成立，并如实输出“可完成”或“仍阻断”的结论。

本任务是协调与验收任务，不直接修改生产代码、测试实现、构建脚本或 CI workflow。若组合验收发现实现缺陷，必须回到原拥有者或在获得新的范围授权后处理；不得在本任务内静默热修。

## Confirmed Facts

- `meta.depends_on = ["08-25-frontend-build-chain"]`，该依赖已归档且为 `completed`。
- `08-25-jquery-upgrade`、`08-25-bootstrap-upgrade`、`08-25-tinymce-upgrade` 均已归档且为 `completed`；本任务是现代化活动任务图中唯一仍为 `planning` 的未完成子任务。
- 当前 `package.json`/已安装依赖精确解析为 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2、jquery-migrate 3.6.0、Playwright 1.62.1 与 esbuild 0.28.2。
- 当前 Playwright 发现 `build-smoke` 1 个、`business` 22 个、`browser-smoke` 6 个用例；`browser-smoke` 只包含 `business-flows` 与 `editor-flows`，不包含 Bootstrap 组件套件。
- 当前提交 `fcc979bb9f0fe35d1771b00665017e470e2182d4` 的 GitHub Actions CI run `33477561244` 失败。Go 1.26/1.27 job 通过，但 Node 构建漂移、Mongo 集成、Chromium E2E、package 与 container job 失败；因此归档状态不能作为组合验收通过的替代证据。
- F 的正式发布浏览器证据已改为 tag 后生成的 `browser-release-matrix-v1` artifact，不是源码中的受跟踪矩阵文件；现有三个 `docs/modernization/browser-smoke/*.md` 只是历史/缺口台账，当前均不足以证明完整四产品两主版本矩阵。

完整现场证据与缺口见 `research/spec-audit-2026-09-01.md`，逐门禁状态见 `acceptance/evidence-matrix.md`。

## Scope And Deliverables

### Inputs

- 一个明确的候选提交 SHA，以及该 SHA 的干净 checkout。
- 三个已归档子任务的 PRD、研究材料、实现提交与验收记录。
- `package.json`、`package-lock.json`、`scripts/build/manifest.mjs`、受跟踪生成资源与公开 URL。
- `playwright.config.mjs`、Node/Go 测试、test-mode harness、质量门 workflow 与运行结果。
- 真实浏览器矩阵及其 commit/ref/run/attempt/checksum provenance。

### Outputs

- 一份只包含脱敏证据、精确命令、发现数量、运行链接/ID、提交 SHA 与结果的组合验收矩阵。
- 对每个失败项给出根因类别、原拥有者、阻断范围和重新验收入口；失败不得被降级为 warning 或“已归档所以通过”。
- 明确的任务结论：`eligible_for_completion` 或 `blocked`。不得输出含糊的“基本通过”。

### In Scope

- 版本唯一性、生成物来源与公开 URL 兼容。
- Chromium PR/push 门禁、真实四浏览器两主版本发布门禁及证据 provenance。
- 登录、笔记/笔记本/标签、富文本/markdown、上传/相册、博客、admin/member、Bootstrap 组件、三个编辑器入口与 `leaui_image` iframe。
- 未编辑零写入、编辑后 HTML 语义、`/api/*`、USN、所有权和 MongoDB Schema 不变。
- 失败、清理、敏感数据、artifact 保留和跨提交证据拒绝规则。

## Requirements

### R-E1: Dependency And Evidence Truth

- ready 只由当前任务状态、活动叶条件和 `task.json.meta.depends_on` 对应任务的归档 `completed` 证据确定；父任务 `[n/n done]`、归档目录名或历史说明不能替代依赖核验。
- E 的完成结论及其 E-owned 证据必须绑定同一候选提交；若 Q-E1 选择等待 tag artifact，F 的发布 artifact 按 AC-E6 单列 tag commit，不改变 E 候选证据的绑定规则。子任务归档状态只证明历史生命周期，不证明当前 HEAD 仍通过。
- 当前 F 已在 E 仍为 `planning` 时归档，且 F 的 `task.json.notes` 仍声称上游证据阻断；该状态/叙述冲突必须在现代化父任务收口前如实登记，不得把它改写成“F 按 DAG 顺序完成”。

### R-E2: Single Runtime And Generated-asset Contract

- lockfile 中 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 必须各有且仅有一个解析版本。`jquery-migrate@3.6.0` 只允许作为开发诊断依赖，不能进入 manifest、生产 bundle 或运行页面。
- `scripts/build/manifest.mjs` 与 `app/views/note/note-dev.html` 是生成物事实来源；不得手工修补受跟踪 bundle。
- 保留现有公开 URL，包括文件名仍为 `/js/jquery-1.9.0.min.js` 的兼容 URL，但实际字节必须来自锁定的 jQuery 3.7.1；不得以旧文件名推断旧运行时。
- 干净 Linux checkout 的构建必须同时保持内容与文件 mode 稳定，且 tracked diff 与 non-ignored untracked 状态都为空。

### R-E3: Automated Integration Gate

- PR/push Chromium 门禁必须在同一 test-mode harness、`leanote_test` 数据库、随机 run token 与轮换 admin 凭据生命周期内先运行 `build-smoke`，再运行完整 `business` project。
- 测试发现与执行必须显式证明 `build-resource-smoke.spec.mjs` 以及 business 下的 `ajax-failure`、`bootstrap-components`、`business-flows`、`editor-flows`、`jquery-diagnostics` 均被发现并执行；发现数为零、文件漏发现或测试/清理失败均失败。
- Golden、USN、所有权和 note-save contract 必须复用 CI 已提供的 MongoDB 8.0 服务或一个明确的外部 URI；测试不得再次抢占固定 `27017` 启动平行容器。两种互斥执行模式必须在运行前明确记录：
  - **service-backed Go/Mongo 模式**：使用 CI service 的 `mongodb://127.0.0.1:27017/leanote_test`（或显式 `LEANOTE_TEST_MONGO_URL` 指向同一服务），先恢复一次 fixture，再运行 `LEANOTE_REQUIRE_MONGO=1` 的 Go 套件；不得调用 `NewMongoEnvironment.Up()`。
  - **Chromium 自建 harness 模式**：仅在没有外部 Mongo service 时由 `go run ./app/tests/harness/cmd/e2e` 创建并销毁 `leanote-test-mongo`；该模式不得同时声明 service URI。若同时检测到两种来源，或 URI 的数据库不是 `leanote_test`，必须在启动应用前 fail closed。
- E 的质量门 allowlist 固定为 `node-build`、`chromium-e2e` 与 `mongo-8_0`；`go-1_26_7`/`go-1_27_0` 是 E 消费的后端/工具链外部证据，`package-smoke`、`container-smoke` 与 `summary` 归 F/父任务，不得在 E 矩阵中冒充 E-owned job。allowlist 内任一 job 失败或缺失即阻断；外部 job 的失败仍须记录 owner/retest，并阻止父任务整体发布结论。本地 120/120 Node 测试不能替代任何提交级 CI、真实服务 E2E 或 Mongo 套件。

### R-E4: Real-browser Matrix And Coverage

- 支持 Chrome、Edge、Firefox、Safari 在执行时官方 stable 的当前主版本与紧邻前一主版本，不支持 IE；四产品均必须使用真实产品，Safari 只接受真实 Safari。
- 每个产品/slot 的 smoke 必须覆盖 `business-flows`、`editor-flows`、`bootstrap-components` 与 `leaui-image-iframe` 四项适用交互，包含身份预检、错误门禁、owned-resource 4xx/5xx 门禁和写入清理。jQuery Migrate 注入诊断只属于 Chromium 开发门禁，不要求进入发布生产浏览器。
- 若 Q-E1 选择“等待 tag artifact”，`browser-release-matrix-v1` 必须恰好含 8 条唯一记录，绑定严格 `vX.Y.Z` tag 的 commit/ref、producer workflow、当前 run/attempt 与矩阵 SHA-256，保留不超过 7 天；缺失、重复、跨 run/ref/attempt、非相邻主版本、占位版本或任一非 passed 字段均阻断。若选择“不等待”，必须生成受保护的 `candidate-browser-matrix-v1` artifact（不进入源码或本验收表），载荷固定为 `candidate-matrix.json` 与 `candidate-provenance.json`，同样恰好 8 条记录但绑定候选 SHA/ref，不得称为 tag artifact。两种 artifact 的保留期均不超过 7 天。
- 两种 artifact 的每条记录 `coverage` 必须按固定顺序恰好为稳定 ID `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`，并新增 64 位小写十六进制 `coverage_summary_sha256`。对应 provenance 必须携带同一 run/attempt 生成的脱敏 `coverage_summaries`：每个槽位恰好四个 summary 项，每项固定包含 `id`、`discovered_count`、`executed_count`、`entrypoints`、`iframes`、`result`；每项的 `discovered_count` 与 `executed_count` 均为正整数且 `executed_count <= discovered_count`，`result` 必须为 `passed`，数组按稳定顺序序列化。`entrypoints`/`iframes` 只能是小写、无换行的稳定标识符（不得写 URL、页面正文或认证信息）。摘要输入固定为去掉 digest 字段的 `{browser_product, release_slot, items}` 对象；使用 RFC 8785 JSON Canonicalization Scheme（JCS）按键排序、固定数组顺序、UTF-8、无空白和无尾随换行后计算 SHA-256。validator 必须重算并校验摘要输入、槽位、候选/tag commit 和 run/attempt 一致。摘要不得包含认证材料、页面正文、用户数据或原始 trace。
- `candidate-matrix.json` 采用与 F `release-matrix.json` 相同的记录字段、八槽位唯一键、版本主号相邻和 gate 规则，但 `schema_version` 固定为 `leanote.browser-smoke.candidate-matrix.v1`，顶层及每条记录的 `commit` 固定为同一候选 SHA（40 位小写且非全零）。`candidate-provenance.json` 的固定字段为 `schema_version=leanote.browser-smoke.candidate-provenance.v1`、`matrix_sha256`、`candidate_commit`、非 tag `ref`、`producer_workflow=Protected browser candidate evidence`、`candidate_run:{id,attempt}` 和 `coverage_summaries`；字段及嵌套对象均 `additionalProperties=false`，`ref` 必须是无换行的 `refs/...` 且不得以 `refs/tags/` 开头，`candidate_run.id` 采用与 F `release_run.id` 相同的非零十进制字符串（`^[1-9][0-9]*$`），`candidate_run.attempt` 为 JSON 正整数（`>=1`）。`matrix_sha256` 必须计算候选矩阵上传文件的原始字节，不能解析后重序列化。候选 validator 必须把这些字段与受保护 workflow 的实际 SHA/ref/run/attempt 交叉校验，并拒绝任何 `refs/tags/*` 或用户可变的 checkout 输入。F 的两文件发布 allowlist 保持不变，但其 `provenance.json` 必须增加同构的 `coverage_summaries` 字段、release 槽位绑定和上述摘要校验；旧的仅含通用 `scope` 或缺 digest 的 v1 载荷一律无效。F owner 更新契约、producer 与 validator 前，必须保持 AC-E6 阻断，不得把“8 条结构有效记录”表述为完整 E 验收。

### R-E5: Compatibility And Data Invariants

- 保留服务端渲染、模板组织、现有公开 URL、RequireJS/全局脚本契约、三套内置博客主题与用户上传主题的**原始字节和路径**；只允许验证主题加载/注入契约，不得自动重写用户上传主题。不引入 SPA、第二运行时或永久兼容层。
- `/api/*`、USN、认证、所有权、MongoDB collection/字段/BSON 类型和外部错误语义不因前端升级改变；不得用后端 fallback 掩盖前端错误。
- 未编辑笔记不得发出内容保存，数据库 HTML 字节不变；真实编辑后仅允许夹具逐项登记的非语义归一化。失败或部分写入不得确认 editor revision 或显示成功。

### R-E6: Failure, Cleanup And Sensitive-data Rules

- 环境变量缺失、身份 marker 缺失/重复/过期/摘要不符、错误数据库、认证失败、浏览器缺失、资源失败、console/page/unhandled-rejection 错误、测试超时或清理失败均 fail closed。
- 验收记录不得包含密码、Cookie、token、认证头、storage state、页面正文、用户数据、原始 trace、截图/视频或未脱敏服务日志。
- 不接受 mock 成功、手工伪造矩阵、重跑后覆盖失败 artifact、忽略退出码、缩小发现范围或把失败 job 记为“基础设施告警”。

### R-E7: Ownership Boundary

- 本任务只修改自身 PRD、design、implement、research、acceptance 与 context manifest；规划审核阶段可为同步父任务
  DAG/生命周期事实对父任务 PRD 做最小说明性更新，也可为保持 E↔F 证据契约一致同步 F 的任务规格、研究、验收材料和 task notes；
  两类同步均不得修改实现、测试、workflow、生成资源或任务状态，激活后例外失效。
- Node 构建 mode 漂移归构建/TinyMCE 生成闭包所有者；Bootstrap 实浏览器发现缺口归 Bootstrap/E2E 配置所有者；编辑器/iframe 失败归 TinyMCE 所有者；Mongo harness 端口冲突归 G/harness 所有者；release artifact 语义覆盖归 F 所有者。
- 在原拥有者已归档且没有合法修复载体时，必须先获得用户对“重开原任务或扩大当前任务实现范围”的明确决定，不得在协调任务内越权修改。

## Acceptance Criteria

- [ ] **AC-E1** 选择证明包含活动任务树、`children` 历史、`meta.depends_on` 与归档 `status=completed` 证据；候选提交 SHA 唯一且贯穿 E 自有的所有构建/测试运行记录。若 Q-E1 选择等待 tag artifact，F 的发布运行按 AC-E6 另行绑定 tag commit，不纳入候选 SHA 约束；候选 SHA 与 tag commit 必须分列。若 E 自有运行记录仍未绑定候选 SHA，状态必须为 `blocked`，不能因依赖已归档而记为通过。
- [ ] **AC-E2** `npm ls` 证明 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 唯一；生产 manifest/bundle 无 jquery-migrate、旧核心字节、第二 runtime 或 CDN fallback。
- [ ] **AC-E3** 干净 Node 24.20.0 checkout 执行 `npm ci && npm run build && npm test` 通过；构建后 tracked diff 和 non-ignored untracked 状态均为空，Linux 文件 mode 不漂移。
- [ ] **AC-E4** Playwright `build-smoke` 列表证明 `build-resource-smoke.spec.mjs` 被发现，`business` 列表证明五个 business 套件均被发现，`browser-smoke` 的适用列表另行记录；同一 harness 中 `npm run test:e2e:build` 与 `npm run test:e2e` 全绿，写入清理无残留，console/page/resource/认证门禁全绿。
- [ ] **AC-E5** Go 1.26/1.27 build/vet/unit、MongoDB 8.0 的 Golden/USN/所有权/note-save 套件均在候选提交成功，发现数大于零；Go/Mongo 运行明确属于 service-backed 模式或 Chromium 自建模式之一，且不启动冲突的第二个固定端口容器。
- [ ] **AC-E6** 真实四浏览器两主版本覆盖 `business-flows`、`editor-flows`、`bootstrap-components` 与 `leaui-image-iframe`；若 Q-E1 选择等待 tag artifact，则校验严格有效的 8 条 `browser-release-matrix-v1` 记录和带 `coverage_summaries` 的 provenance；若选择不等待，则校验 `candidate-browser-matrix-v1` 的 8 条记录、`candidate-provenance.json` 的固定字段及同一候选 SHA。两种模式都要求 coverage ID、`coverage_summary_sha256`、discovered/executed 摘要与每条记录存在可审计不可变绑定，不能把候选记录冒充 tag artifact。
- [ ] **AC-E7** 未编辑零写入、编辑保存/失败 revision、HTML 语义、`/api/*`、USN、所有权、Schema、URL 与主题兼容全部通过，没有后端兼容分支或隐藏 fallback。
- [ ] **AC-E8** 候选提交的 E quality-gate allowlist（`node-build`、`chromium-e2e`、`mongo-8_0`）全绿；`go-1_26_7`/`go-1_27_0`、`package-smoke`、`container-smoke` 与 `summary` 的外部/下游状态分别记录 owner 与复验入口，任何失败均保留原状态和根因，不以本地局部门禁或 archived 状态覆盖。
- [ ] **AC-E9** `acceptance/evidence-matrix.md` 每一行都有 commit、命令或 run/job URL、测试门禁的发现/执行数量；非测试选择/元数据门禁，或在 discovery 前被阻断、无法取得外部摘要的测试门禁，必须显式写 `N/A` 及原因，且状态只能为 `failed`/`blocked`/`partial`/`missing`，不得记为 `passed`。每行还必须有结果、失败 owner 与复验条件，且不含敏感数据。

## Out Of Scope

- 由协调任务直接修复业务代码、测试、构建脚本、workflow 或生成资源。
- jQuery 4、Bootstrap 6、TinyMCE 之外的新编辑器、SPA/前后端分离、视觉重设计、API/Schema 迁移。
- 用 WebKit/Chromium/UA 伪装真实 Safari、Chrome 或 Edge，或降低当前及前一主版本矩阵。

## Remediation Subtasks

本节把当前阻断拆成可独立规划、实现和验收的需求子任务。子任务均保持
`planning`，不因创建动作自动激活；其 `meta.depends_on` 是执行顺序约束，
不是对历史归档状态的替代证明。

| 顺序 | 阻断 | 子任务 | 优先级 | 前置 | 责任 owner |
|---:|---|---|:---:|---|---|
| 1 | B-E1 | `09-01-frontend-libs-build-mode` | P0 | — | frontend-build-chain / TinyMCE 生成闭包 |
| 2 | B-E2 | `09-01-frontend-libs-mongo-harness` | P0 | B-E1 | Go/Mongo harness |
| 3 | B-E3 | `09-01-frontend-libs-chromium-editor` | P0 | B-E1、B-E2 | TinyMCE / E2E |
| 4 | B-E4 | `09-01-frontend-libs-browser-coverage` | P1 | B-E1～B-E3 | Bootstrap / E2E / release producer-validator |
| 5 | B-E5 | `09-01-frontend-libs-real-browser-matrix` | P1 | B-E4 | browser evidence |
| 6 | B-E6 | `09-01-frontend-libs-release-reconciliation` | P1 | B-E1、B-E2、B-E4、B-E5 | CI/CD delivery / 父任务收口 |

每个子任务的 PRD 都必须保留失败 owner、复验入口和 fail-closed 约束；复杂子任务
在激活前还需补齐自身 `design.md`、`implement.md` 及实现/检查上下文。E 只消费
这些子任务产出的证据，不在 E 内越权实现或修改其他任务状态。

## Open Question

- **Q-E1（阻断激活）**：E 的任务完成是否必须等待严格 release tag 后产生的 `browser-release-matrix-v1` 预检 artifact，还是 E 可在候选提交的实现/Chromium 门禁和同一提交的真实四浏览器两版预发布证据全部闭合后完成，再由 F 在 tag commit 上生成唯一正式八行 artifact 作为发布阻断门？
  - 影响：等待模式由受保护 workflow 在候选 SHA 对应的严格 tag 上先生成仅供 E 验收的预检 artifact；该运行不创建 Release/GHCR、不改变 F 任务状态，因而不增加 E→F 的任务依赖。E 完成后，F 仍按父任务 DAG 在最终 release run 中独立生成并校验正式 artifact。非等待模式则把真实浏览器证据留在候选阶段，E 完成仍不等于发布获批。
  - **条件化完成标准**：
    - 若选择“等待 tag artifact”，受保护 producer 必须在严格 `vX.Y.Z` tag 指向候选 SHA 后生成 `browser-release-matrix-v1` 预检 artifact；其运行只上传两文件、不得创建 Release/GHCR 或标记 F 完成。E 的 `eligible_for_completion` 必须同时满足候选提交的 AC-E1..E5、E7..E9，以及校验该预检 artifact 的 AC-E6；候选 SHA 与 tag commit 必须分别记录且解析为同一 SHA。E 完成并归档后，F 才能在最终 release run 中重新生成并校验正式 artifact。该预检运行不是 Trellis 任务依赖，父任务 DAG 保持 F 发布位于 E 之后。
    - 若选择“不等待 tag artifact”（推荐），E 的 `eligible_for_completion` 只在候选提交的真实四浏览器两版预发布证据、语义 coverage 摘要和 AC-E1..E9 其余门禁闭合后成立；F 的 tag-bound 八行 artifact 是独立发布阻断门，缺失它不得宣称发布获批。
  - 在用户确认前，Q-E1 保持阻断，不执行 `task.py start`；不得把候选 SHA、tag commit 和 tag artifact 合并成一条证据链。
