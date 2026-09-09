# 领域层：模型与跨层契约 — 执行计划

> 本文件是规格审核后的后续实现计划。Phase 0 已完成，D-01～D-04、D-06 已按用户确认闭合；现在可进入 Phase 1/2/3 功能编码，但仍须遵守领域任务边界。

## Phase 0：规格与依赖门（本轮完成）

- [x] 核对父任务、下游任务、`meta.depends_on`、`CONTEXT.md`、ADR 和现有模型/Golden/USN 材料。
- [x] 记录模型分类、序列化事实、覆盖缺口、已知 USN 缺陷和 D-01～D-04、D-06 决策：`research/spec-audit-2026-09-08.md`；D-05 作为已固定的依赖边界保留。
- [x] 明确领域任务与 application/infrastructure/interface/presentation/delivery 的责任边界；未修改业务实现。
- [x] 通过 `task.py validate 09-08-domain-contracts`、`git diff --check` 和改动路径检查后，才允许请求进入功能编码阶段。

## Phase 1：建立唯一模型目录（已获实现批准）

1. [x] 运行 `go run ./.trellis/tasks/09-08-domain-contracts/research/generate_model_catalog.go`，从 `app/info` 导出声明、`app/db/Mgo.go` 集合初始化、读写调用、binder 注册、`conf/routes` 和 Golden 生成 `.trellis/tasks/09-08-domain-contracts/research/model-catalog.json`；为每个类型记录责任、字段来源、消费者和现有 fixture 状态，并按 `research/model-catalog.schema.json` 校验。
2. [x] 将 28 个有业务读写证据的集合逐一映射到持久化模型，并显式登记 `HasShareNote`、`NoteImage`；`Blogs` 等无调用证据项保持 unknown。
3. [x] 为全部 29 个 active API action 建立 variant 索引（方法、认证前置、请求来源/字段、成功/失败形状、状态码和 Content-Type）；真实 HTTP replay 仍标为 `partial`，由 `interface-http`/delivery 补齐。
4. [x] 对 `ToGroup`、`Group.Users`、`ThemePath` 等 projection-only 字段记录物理 struct tag、读写证据和 persistence 状态；没有真实写入证据时不得由 round-trip fixture 推断为已持久化。

## Phase 2：中立值类型 seam（已获实现批准）

1. [x] 在 `app/domain` 提供 `ObjectID` zero/parse/Hex/JSON 契约和错误类型；`time.Time` 保持标准库语义。
2. [x] 让 `app/info` 生产代码和 DB-independent 测试改用中立类型；移除对 `app/lea` 聚合包、Revel、Mongo driver、HTTP 和前端的编译依赖。遗留 BSON 探针保留在 `model_contract*_test.go`，仅通过显式 `-tags mongo_contract` 编译，由下游 infrastructure 接管。
3. [x] 将旧 `lea.ObjectID` 的兼容 alias/转换限制在适配层，并记录迁移期限；不在领域包实现 BSON codec 或 Mongo client。

## Phase 3：纯领域契约回归（已获实现批准）

1. [ ] 以真实 `app/info` 类型和目录驱动 JSON full/zero golden，锁定字段名、声明顺序、嵌入展开、nil/empty、时间和 ObjectID 形状；当前先补齐 ObjectID、`Re`/`Theme.Info` 动态值回归，完整 golden 缺口由目录和下游继续收敛。
2. [x] 为 `ApiNote`、`NoteFile`、`NoteOrContent`、`UserAccount` 及 member-blog 的 `UserBlogBase`、`UserBlogComment`、`UserBlogStyle` 增加字段级、DB-independent 静态 schema fixture；其中 `UserAccount` 当前无直接 binder 证据，博客设置的字段 presence/跨字段校验仍为 unknown。缺失/重复字段、form/query 解析、非法 ID/整数和 `Tags` 字符串/数组的运行时行为仍由 `interface-http`/对应 application fixture 验收。动态 `List/Item` 已通过集中 `ValidateJSONValue` 拒绝不可编码值。
3. [ ] 将 BSON key/tag/round-trip 测试的 driver 依赖迁入 `app/db` 合约测试；当前遗留探针仅用 `mongo_contract` 显式编译，领域包只验证不依赖 driver 的字段目录和 JSON 行为。
4. [x] 将 notebook/tag 旧 USN 差异作为基线记录，同时按 D-01 为下游回归冻结新 USN tombstone/sync 目标；不在本领域任务修改 service/controller。按 D-06 为 USN 并发原子分配、note-save 事务失败与显式 `partial_write` 补偿预留 application/infrastructure 回归入口。

## Phase 4：跨层交接（待下游）

- 将模型目录和 API variant 索引交给五个 application 任务；它们负责服务错误/权限/USN 行为和真实业务回归。
- 将 ObjectID/BSON 转换、集合映射、排序/分页、超时和 Mongo 7/8 证据交给 `infrastructure-persistence`。
- 将 route、binder、Session、状态码/Content-Type 和 Golden replay 交给 `interface-http`；将 HTML/editor/browser 证据交给 `presentation-frontend`/`delivery-verification`。
- D-01～D-04、D-06 的已确认决策、影响和更新后的 fixture 必须在交接记录中可追溯；D-05 是已固定的依赖边界。仅缺少运行证据的端点保持 unknown，不得重新打开已确认决策。

## Required validation（按责任拆分）

领域任务本地门禁（不需要 Mongo、真实 HTTP 或浏览器）：

```powershell
python ./.trellis/scripts/task.py validate 09-08-domain-contracts
go test ./app/info
go vet ./app/info
go list -deps ./app/info
go list -deps -test ./app/info
pwsh -NoProfile -File ./.trellis/tasks/09-08-domain-contracts/research/validate_info_dependencies.ps1
python ./.trellis/tasks/09-08-domain-contracts/research/validate_model_catalog.py ./.trellis/tasks/09-08-domain-contracts/research/model-catalog.json ./.trellis/tasks/09-08-domain-contracts/research/model-catalog.schema.json
python ./.trellis/tasks/09-08-domain-contracts/research/validate_input_contracts.py ./.trellis/tasks/09-08-domain-contracts/research/input-contracts.json ./.trellis/tasks/09-08-domain-contracts/research/input-contracts.schema.json
git diff --check
```

验收脚本必须断言上述两个依赖图均不含 `github.com/revel` 或 `go.mongodb.org/mongo-driver`，模型目录校验脚本（在 Phase 1 生成 `model-catalog.json` 后运行）必须只依赖 Python 标准库并同时检查 schema 版本、69 个 active 类型、commented declarations、集合/索引状态和已确认决策引用，并记录测试发现/执行数量。BSON 的 `app/db` 测试、Mongo 7/8、Golden/USN live replay、HTTP/浏览器、PDF、容器和发布证据不作为本任务本地完成条件，由对应下游任务执行并保留原始结果；环境缺失只能标为未运行或 blocked。

## Rollback points

- 模型目录/fixture 阶段：只回滚 `.trellis/tasks/09-08-domain-contracts` 材料和纯测试。
- 值类型 seam 阶段：以 `app/info` 依赖扫描和 JSON 回归为边界回滚，不恢复对框架聚合包的隐式依赖。
- 下游 adapter/application 以各自任务提交和契约为边界独立回滚；领域任务不修改或回滚其业务实现。

## Completion gate

只有在 Phase 1 的模型目录按 schema 校验通过、Phase 2/3 的实现与领域回归通过、D-01～D-04、D-06 的决策证据已写入、D-05 依赖边界保持通过且下游接收材料后，才能把本任务标记为 completed。当前状态仍是 `in_progress`，因为功能实现和下游接收尚未完成。
