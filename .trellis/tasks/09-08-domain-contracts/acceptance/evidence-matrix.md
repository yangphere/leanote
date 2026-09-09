# `09-08-domain-contracts` 验收证据矩阵

本矩阵随规格审核建立。当前只记录证据要求和现场基线，不宣称功能实现或真实集成已通过。

| ID | 断言 | 证据入口 | 责任 | 当前状态 |
|---|---|---|---|---|
| AC-D1 | `app/info` 每个实际编译导出类型有唯一责任分类；28 个有业务读写证据的集合均有模型映射；`HasShareNote`/`NoteImage` 纳入清单，3 个注释声明单独记录 | `.trellis/tasks/09-08-domain-contracts/research/model-catalog.json`、`research/model-catalog.schema.json`、`research/generate_model_catalog.go`、`research/validate_model_catalog.py` | domain | 目录已生成并通过校验（69 类型、29 集合；API/fixture 仍按 partial/unknown 标注） |
| AC-D2 | 每个持久化模型的 BSON key/tag/omitempty/zero/round-trip、ID/date/nil/empty 契约可重复；ObjectID `null -> zero` 与 projection-only 状态有独立证据 | 模型目录字段条目；`app/db/object_id_test.go`、`app/db/mongo_compat_test.go`；`app/db/Mgo.go` 集合清单 | domain（目录/JSON） + infrastructure（BSON/driver） + delivery | domain ID/JSON 与 adapter ObjectID 编码、标准 ObjectId/历史空字符串/hex 解码及未知类型失败回归已通过；全模型 BSON 探针仍需由 infrastructure 接管 |
| AC-D3 | 所有 API DTO/envelope 与请求绑定 variant 的字段/顺序/状态码/Content-Type/错误均有 fixture 或 unknown 记录；领域只提供静态输入 schema，运行时 binder 由 interface 验收 | `.trellis/tasks/09-08-domain-contracts/research/model-catalog.json`、`.trellis/tasks/09-08-domain-contracts/research/input-contracts.json`、`conf/routes`、API controllers、`app/tests/golden/api/**` | domain（schema） + interface + application | 29 个 active action 已登记为 partial；4 个请求结构已提供字段级静态 schema，真实 binder/HTTP replay 仍由 interface 补齐 |
| AC-D4 | `app/info` 及其 `-test` 依赖图不含 Revel/Mongo driver；领域 JSON/ID 回归 DB-independent | `go list -deps ./app/info`、`go list -deps -test ./app/info`、`go test ./app/info` | domain | 已通过：默认生产与测试依赖图无 Revel/Mongo/lea；legacy BSON 探针仅显式 `-tags mongo_contract` |
| AC-D5 | note/notebook/tag mutation、USN、sync、冲突、所有权和删除边界均可回放；已知缺陷单独标识 | `app/tests/harness/usn_test.go`、`app/tests/golden/usn/**` | application-notes + delivery | 基线已记录；D-01 已决，待实现新 USN tombstone/sync 目标 |
| AC-D6 | 未编辑内容零写入，真实编辑只发生 ADR-0003 登记的非语义 HTML 规范化 | `docs/adr/0003-modernize-frontend-with-generated-asset-contract.md`、editor/browser fixtures | presentation + delivery | 由下游负责；本任务不运行浏览器 |
| AC-D7 | 领域任务不修改 service/controller/db/CI/生成资源；Mongo/HTTP/浏览器/PDF/发布证据不被局部测试替代 | Git path check、Trellis validate、下游证据矩阵 | domain + delivery | 领域实现仅改 `app/domain`、`app/info`、`app/lea` adapter 与任务材料；真实下游证据仍未运行 |
| AC-D8 | `ShareNote`/`HasShareNote` 唯一索引的历史意图与实际部署状态可区分；冲突不可吞掉 | 模型目录 `unique_indexes`；Mongo `listIndexes` 输出或部署 fixture（由 infrastructure 提供） | infrastructure + delivery | 当前仅有历史意图，`unknown` |
| AC-D9 | 并发 USN 分配唯一、单调、无丢失；note-save 元数据成功/内容失败的持久化状态、失败 envelope、USN 消耗和重试幂等性可回放 | application/infrastructure concurrency fixture、partial-write fixture、D-06 决策记录 | application-notes + infrastructure-persistence + delivery | 未运行；D-06 已决，待实现原子分配与事务/显式补偿 |

## 证据规则

- 领域本地门禁只运行 DB-independent 测试、`go vet ./app/info`、依赖扫描和 `git diff --check`。
- Mongo 7/8、真实 HTTP、真实浏览器、PDF、容器和 GHCR 证据必须由责任下游任务记录 discovery/execution、commit、run/attempt、退出码和脱敏原因；环境缺失保持 `未运行` 或 `blocked`。
- Golden 动态字段只能按已有归一化清单替换；不能通过重排键、删除未知字段或把错误响应改写为成功来制造通过。
- D-01～D-04、D-06 已确认，相关行仍须等待实现与运行证据才能标记为 completed；D-05 是已固定的依赖边界。若实现改变行为，必须同步更新模型目录、fixture 和依赖任务。
