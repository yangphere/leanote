# 前端库现代化（E）— 协调验收设计

## 1. 任务边界

E 不再规划三个库的实现；三个子任务已经归档。E 的唯一职责是在一个候选提交上组合现有输出、发现跨子任务漂移并形成可审计结论。

```text
archived child contracts + current source/lockfile
                      │
                      v
           candidate commit SHA (E build/test)
             │        │        │
             │        │        └─ selected real-browser evidence + provenance
             │        └─ test-mode Chromium + Mongo contracts
             └─ clean Linux build + generated-asset drift
                      │
                      v
        evidence matrix: eligible | blocked
```

E 不修改图中任何运行时节点。发现失败后，只登记 owner、影响与复验入口。
真实浏览器分支按 Q-E1 选择：不等待模式消费候选 artifact，等待模式消费独立 tag artifact；后者的 tag commit 不替代 E-owned 候选 SHA。

## 2. Evidence Precedence

从高到低采用以下事实来源：

1. 绑定候选提交的真实运行、GitHub run/job 与不可变 artifact。
2. 当前 checkout 的 lockfile、manifest、配置、测试发现和生成资源。
3. 已归档子任务的 PRD、研究与提交，用于说明意图和所有权。
4. 父任务进度计数、复选框或历史口头结论，只作导航，不能证明通过。

同层证据冲突时采用失败或较弱结论。例如 task 为 `completed` 但当前提交 CI 失败时，门禁状态仍是失败。

## 3. Verification Layers

### 3.1 Static And Build

- `npm ls --all --json` 与 `package-lock.json` 的完整 `packages` 树共同证明精确版本和唯一解析；`--depth=0` 只能证明顶层，不能作为唯一性证据。
- manifest 证明旧公开 URL 指向新锁定输入，jquery-migrate 不进入生产。
- 干净 Linux checkout 证明连续构建的内容、mode、tracked 与 untracked 状态稳定：第一次构建后在 checkout 外保存每个 manifest 输出的 SHA-256、mode 和输出集合快照；第二次从同一候选 SHA 的新 checkout 执行 `npm ci && npm run build`，逐项比较快照，再分别检查 `git diff --exit-code` 与 `git status --porcelain --untracked-files=all`。

### 3.2 Contract And Chromium

- Node 单测保护生成闭包、旧 runtime 扫描、Bootstrap 交互、TinyMCE 状态与发布 schema。
- `build-smoke` 与完整 `business` project 在共享 test-mode harness 内串行运行；`build-smoke` 必须发现并执行
  `build-resource-smoke.spec.mjs`，`business` 必须发现并执行 `ajax-failure`、`bootstrap-components`、
  `business-flows`、`editor-flows`、`jquery-diagnostics` 五个文件。
- Golden/USN/所有权/note-save 明确选择一种互斥 Mongo 模式：service-backed 模式复用 CI 的 `127.0.0.1:27017`，不调用 `NewMongoEnvironment.Up()`；Chromium 自建模式才允许 e2e supervisor 创建 `leanote-test-mongo`，且运行前不得存在外部 Mongo service/URI。两种模式都必须把数据库固定为 `leanote_test`，冲突或错误 URI 在启动服务前失败。

E quality-gate allowlist 是 `node-build`、`chromium-e2e`、`mongo-8_0`。`go-1_26_7` 与 `go-1_27_0` 是外部后端/工具链证据；`package-smoke`、`container-smoke`、`summary` 归 F/父任务。E 只能对 allowlist 给出自身质量门结论，其他 job 以依赖证据记录 owner、run/job 和复验条件。

### 3.3 Real Browsers

真实矩阵的语义覆盖集合固定为以下四项；每个产品/slot 都必须逐项通过：

- `business-flows`：认证、笔记/笔记本/标签、上传/相册、博客、admin/member；
- `editor-flows`：主笔记和会员博客入口、未编辑零写入、revision/undo/只读；
- `bootstrap-components`：modal/tab/dropdown/tooltip/alert、BootstrapDialog、远程 modal、内置主题；
- `leaui-image-iframe`：`leaui_image` iframe 的身份预检、资源/错误门禁、上传/选择/跨 iframe 插入与清理。

等待 tag artifact 模式下，结构化八行 `browser-release-matrix-v1` 负责身份、版本、tag commit 和 provenance；不等待模式下，受保护 runner 必须生成独立的 `candidate-browser-matrix-v1` artifact，固定包含 `candidate-matrix.json` 与 `candidate-provenance.json`，使用候选 SHA/ref 和 `candidate_run:{id,attempt}`，不能称为 tag artifact，也不能把本验收表当作槽位载荷。两种模式的矩阵记录都必须按固定顺序写入四个 coverage ID 和 64 位 `coverage_summary_sha256`。provenance 的 `coverage_summaries` 必须恰好覆盖 8 个产品/slot，每个槽位恰好四项，项字段固定为 `id`、`discovered_count`、`executed_count`、`entrypoints`、`iframes`、`result`；每项两个计数均为正整数且 `executed_count <= discovered_count`，结果为 `passed`，入口/iframe 只能是小写、无换行的稳定标识符。摘要输入固定为去掉 digest 字段的 `{browser_product, release_slot, items}` 对象，`items` 顺序固定为四个 coverage ID；使用 RFC 8785 JSON Canonicalization Scheme（JCS）按键排序、固定数组顺序、UTF-8、无空白和无尾随换行后计算 SHA-256。validator 必须重算并校验摘要输入、槽位、commit 和 run/attempt。只有通用 `scope` 或非空 coverage 不能关闭语义覆盖门。

候选载荷直接复用 F 发布矩阵的记录字段和八槽位/版本/gate 交叉规则，唯一差异是 `candidate-matrix.json` 的 `schema_version` 固定为 `leanote.browser-smoke.candidate-matrix.v1`，顶层及记录 `commit` 固定为候选 SHA；`candidate-provenance.json` 固定为 `leanote.browser-smoke.candidate-provenance.v1`，使用 `candidate_commit`、非 tag `ref`、固定 `producer_workflow=Protected browser candidate evidence`、`candidate_run`，其中 `candidate_run.id` 为与 F `release_run.id` 同构的非零十进制字符串，`candidate_run.attempt` 为 JSON 正整数；其余字段约束与 `provenance.json` 同构。`matrix_sha256` 对候选矩阵上传文件的原始字节计算，不得解析后重序列化。候选 producer 的 workflow 标识、实际 checkout SHA/ref/run/attempt 必须由受保护 workflow 上下文提供，不能由 workflow 输入覆盖；缺少独立候选 producer 或其 validator 时，不等待模式保持 blocked。

### 3.4 Archived child contract map

组合验收必须逐项引用归档子任务的关键契约，而不是只引用其完成状态：

| 子任务 | 必须映射到 E 的证据 |
|---|---|
| jQuery | 第一方 `JQMIGRATE:` warning 为零；第三方豁免按归属表逐条登记且运行中实际命中；未登记来源 fail-closed；公共 AJAX wrapper 与直接调用的 4xx/5xx 可见失败。 |
| Bootstrap | remote modal 不使用已删除 `remote` option；BootstrapDialog 的 show/confirm/getModalBody/close；hover-dropdown 延迟/触摸/键盘行为；`leaui_image` iframe 的 tab/alert/form/upload 边界；用户上传主题字节和路径不变。 |
| TinyMCE | 三入口共用基础配置并显式 GPL；四个第一方插件初始化/插入/更新/undo/redo/保存/重载/只读/失败；paste/drop 单次插入和失败可恢复；七 locale；未编辑零写入、revision/部分写入错误语义；TinyMCE 安全默认值不放宽。 |

任一契约缺少定位、运行结果或 owner/retest 时，E 保持 blocked。

## 4. Failure State Machine

```text
planning
  └─ final plan approved -> in_progress
       ├─ any required evidence missing/failed -> blocked evidence row
       └─ every required row passed -> eligible_for_completion
```

- 缺环境、缺浏览器和缺外部 runner 是 `blocked`，不是 `passed` 或 `skipped`。
- 运行中途失败仍须执行清理；清理失败保持失败并单独记录。
- 重跑产生新证据，不覆盖旧失败记录；最终结论引用采用的 run/attempt。

## 5. Browser Evidence Lifecycle

F 已选择独立的 tag-bound artifact，现有源码内浏览器文档不再是发布真相。Q-E1 已选择等待 tag artifact；该路径采用“预检 artifact → E 验收 → 正式 release artifact”的无环两阶段，不把候选 SHA 与 tag artifact 混成不同身份：

- E 的候选提交集成证据：证明实现质量和当前构建/契约门禁；
- 受保护 tag 预检 artifact：在严格 tag 指向候选 SHA 后供 E 验收，证明四产品两主版本，但不创建 Release/GHCR 或改变 F 任务状态；
- F 的正式 tag artifact：E 归档后由最终 release run 重新生成，证明发布 commit 的四产品两主版本并授权发布。

F 的两文件 allowlist 不增加第三个文件；F owner 必须在同一 `provenance.json` 中增加严格的
`coverage_summaries` schema，并让 `release-matrix.json` 每行带
`coverage_summary_sha256`。旧的 provenance 结构和通用 `scope` producer 在该契约下均无效，
需要先更新 F 的 research contract、producer 和 validator，再重新生成 artifact。

等待模式的预检 producer 是受保护 workflow 运行而非 Trellis 任务依赖：它只能生成 `browser-release-matrix-v1` 两文件 artifact，不能发布或将 F 标记完成；E 验收后，F 按既有父任务 DAG 在最终 release run 中重新生成同名 artifact 并执行当前 run/attempt 校验。若改回不等待模式，E 则消费候选 artifact；两种模式的 artifact 不能互相冒充。

## 6. Compatibility And Rollback

- 公开 URL 与服务端/数据契约是跨三个子任务的共同不变量。
- 用户上传博客主题的原始字节和路径是不可变输入；只验证 loader/injection，不自动格式化、重写或注入 Bootstrap adapter。
- E 没有代码 rollback；验收失败时回到产生缺陷的最小拥有者提交或任务。
- 若 owner 已归档，先取得重开或扩域授权。不得在 E 创建第二事实来源、兼容 shim 或隐藏 fallback。
- 规划审核阶段可为同步父任务 DAG/生命周期事实对父 PRD 做最小说明性更新，也可为保持 E↔F 证据契约一致同步 F 的任务规格、研究、验收材料和 task notes；两类同步均不得修改实现、测试、workflow、生成资源或任务状态，任务激活后例外失效，其他父任务或业务材料仍归各自 owner。

## 7. Security And Retention

验收材料只保存脱敏摘要、提交与运行 provenance。原始 trace、截图、视频、storage state、Cookie、token、认证头、页面正文和用户数据不进入任务材料；短期 artifact 保留不超过 7 天。
