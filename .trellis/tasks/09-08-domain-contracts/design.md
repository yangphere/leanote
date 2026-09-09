# 领域层：模型与跨层契约 — 技术设计

## 1. 数据流与边界

```text
HTTP/form/query or template input
              │  binding + validation (interface)
              ▼
      request DTO / application command
              │
              ▼
       domain model + result/envelope
          │                 │
          │                 └── JSON / template projection (interface/presentation)
          ▼
      persistence adapter (app/db)
          │  ObjectID/time/BSON conversion
          ▼
       MongoDB collections
```

领域任务只拥有中间的 model/result 契约和可重复的纯数据回归。HTTP 状态码、表单解析、模板渲染和 Mongo client/query 类型分别由 interface、presentation 和 infrastructure 任务拥有；任何一层不得复制另一层的字段清单或授权规则。

## 2. 依赖 seam

### 2.1 中立值类型

`app/info` 生产包必须只依赖标准库和 `app/domain` 中立领域值类型包，不得导入 `app/lea` 聚合包、Revel、Mongo driver、HTTP 或前端。ObjectID seam 的契约为：

| 操作 | 领域行为 |
|---|---|
| zero test | 可区分未设置值，不依赖字符串比较 |
| parse | 接受空字符串（zero）和严格 24 位小写/大小写 hex；其他输入返回可判定错误 |
| JSON encode | zero 为 `""`，非 zero 为 24 位 lowercase hex |
| JSON decode | 接受 JSON string 或 `null`；空串及 `null` 为 zero，非法 hex、number/object 等其他类型返回错误 |
| BSON encode/decode | 不在领域包实现；`app/db` 将其转换为 BSON ObjectId，并兼容历史空字符串/24 位 hex 文档；其他字符串或未知 BSON 类型返回可判定错误 |

时间使用标准库 `time.Time`；领域包不持有 logger。若应用服务需要日志，只注入最小的边界 sink，日志实现和敏感字段脱敏由基础设施/接口层负责。

`app/domain` 的 `ObjectID` 只负责 zero、parse、Hex 和 JSON；BSON adapter 留在 `app/db`。`lea.ObjectID` 的兼容别名或显式转换只能存在于迁移适配层，不能反向成为 `app/info` 的编译依赖；该兼容层保留至所有下游迁移完成，随后移除。D-02 已闭合，不得在实现中改用其他包名。

### 2.2 序列化所有权

| 边界 | 唯一责任 | 禁止事项 |
|---|---|---|
| domain → JSON | DTO 的字段名、声明顺序、nil/空值和时间/ID JSON 形状 | 在 controller 中复制结构或重排键 |
| domain ↔ BSON | `app/info` 物理 struct tag 是字段命名唯一来源；`app/db` 读取 tag 并负责 ObjectID/date 转换、旧文档读取兼容 | 在 `app/db` 复制字段表，或让 `app/info` 导入 driver/collection |
| request → domain | HTTP adapter 的字段绑定、类型/范围校验、重复值处理 | 从 BSON tag 推导表单契约 |
| domain → template | presentation 的显示投影和 escaping | 模板直接读取数据库文档并自行解释权限 |
| unknown/dynamic value | 一个集中 decoder/normalizer，限定 JSON-compatible 值 | 每个消费者本地 type assertion 或静默 fallback |

## 3. 模型目录与单一事实来源

在实现阶段维护固定路径 `.trellis/tasks/09-08-domain-contracts/research/model-catalog.json` 的机器可检查模型目录，并用同目录的 `model-catalog.schema.json` 校验。目录由同目录的 `generate_model_catalog.go` 从 `app/info` AST、`app/db/Mgo.go` 和已登记路线材料生成，避免手工维护第二事实来源；校验命令必须可在无外部服务的环境复现。目录至少包含 `schema_version`、源文件/版本、类型责任、消费者、secondary role、字段 wire/BSON/persistence 状态、fixture 状态、集合映射、索引证据、API variant 和已确认决策记录。目录必须覆盖 `app/info` 的 69 个实际编译导出类型；3 个仍在注释中的类型声明单独列为 `commented_declarations`，不得计入 active 类型。另须至少列出 28 个有业务读写证据的集合模型；`Blogs` 仅记录为无调用证据的待确认项。匿名嵌入字段同时记录 `json_embedded` 与展开后的 JSON 键，避免把 JSON 展平误写成嵌套对象。

- 持久化模型的字段事实来自 `app/info` 中物理存在的显式 `bson` struct tag、集合初始化和读写调用；`app/db` 只能读取这些 tag 并完成驱动转换，不得维护第二套字段表。带“仅显示/不存储”注释的字段即使仍有历史 tag，也必须在目录标成 `projection_only` 或 `unknown`，并由 adapter 明确排除写入；`contractRegistry` 只是测试清单，不是第二事实来源。
- API DTO/envelope 的字段事实来自 `conf/routes`、API controller 和 Golden；直接数组/模型与 `ApiRe`/`AuthOk`/`ReUpdate`/`Re` envelope 分开记录。
- 请求输入的事实来自 binder 注册、controller 参数和表单 Golden；缺失字段、重复字段和字符串列表解析必须有独立条目。
- 模板/内部投影不得因为可 JSON marshal 就自动加入 BSON registry；组合嵌入结构要记录展开键和字段冲突规则。

当前 registry 缺少 `HasShareNote` 和 `NoteImage`，实现必须先把缺口加入目录/fixture，再以目录驱动测试。新增类型若没有责任归属，验收失败而不是默认为“通用模型”。

## 4. Contract groups

### API and input

为每个 API action 记录：请求方法/路径、认证前置、输入来源（query/form/file/json）、绑定目标、成功状态/Content-Type/body 形状、失败状态/body 形状、动态字段和 Golden 文件。模型目录当前登记 29 个 active action；每条记录的 `evidence_status=partial` 表示静态源码/文档已核对但真实 HTTP replay 尚未运行。现有 `NOTLOGIN`、`conflict`、直接同步数组、认证成功和更新 envelope 必须作为独立 variant；不把端点统一到新协议。

请求结构的字段级静态事实单独写入 `research/input-contracts.json`，并由同目录 schema/校验脚本约束。该材料只冻结 Go 字段、wire 名称、required/optional、空值和已知约束；重复字段、form/query 解析、`Tags` 字符串/数组和状态码的运行时差异继续由 `interface-http` 验收。

输入目录还要标明 required/optional ID、整数范围/缺省值、重复字段处理、UTF-8/大小写和 `Tags` 的分隔规则。领域任务只提供字段级静态 schema fixture；缺失/重复字段、form 解析、非法 ID/整数等运行时行为由 `interface-http` fixture 验收。required ID 的非法 hex、整数溢出和未知值必须在 adapter 处显式失败，不能以 zero、默认页码或成功 envelope 继续执行；没有现状证据的范围保持 unknown。

### Persistence

每个持久化模型的回归包含：完整 BSON key 集、显式 tag、`omitempty` 零值状态、ObjectID/date 类型、nil/empty 集合、嵌入/字段冲突以及读写 round-trip。带 `omitempty` 的零 ObjectID 按当前 mongo-driver/v2 快照单独断言为零 ObjectId；若基础设施任务要改变该已观察行为，必须先取得独立兼容决定并同步更新 fixture。排序、分页和 upsert 的字段前提写入模型目录，由 infrastructure 任务验证查询实现。

模型目录同时记录唯一索引的历史意图、当前 `listIndexes`/部署 fixture 证据和状态；`ShareNote` 的 `(UserId, ToUserId, NoteId)`、`HasShareNote` 的 `(UserId, ToUserId)` 在证据闭合前标为 `unknown`，冲突属于可观察失败。`IsDeleted`/`IsTrash` 是否进入 sync、not-found 或公开读取按 action 记录，禁止跨集合套用单一规则。

### Ownership and USN

- 私有查询以 `(resource predicate AND owner predicate)` 为一个不可拆分边界；共享查询使用授予者、接收者或群组成员的显式组合；公开博客查询的例外谓词必须命名并登记。
- 领域回归记录 mutation 输入、期望返回 USN、持久化 Usn、sync 查询游标/maxEntry、排序、删除位和冲突 envelope。测试只读 fixture/Golden，不在领域包复制 service 查询。
- notebook delete 和 tag delete 的旧差异写入 known-defect ledger；按 D-01 目标，测试必须断言新用户 USN、`IsDeleted=true` tombstone 和 sync 返回删除记录，冲突 envelope/HTTP 外形保持兼容。
- USN 分配必须由 application/infrastructure 通过 Mongo `$inc`/`findOneAndUpdate` 或 CAS+重试完成；并发回归须证明同一用户的 USN 唯一、单调、无丢失，并记录冲突重试后的最终持久化值。
- note-save 的元数据与内容优先在同一事务中提交；事务失败不消耗 USN、不返回成功，重试必须幂等。无事务部署必须使用显式补偿/idempotency 并返回 `partial_write`，相关 fixture 和决策证据由 application-notes/infrastructure-persistence 维护。

### HTML/editor

数据层只保存 `persistedContent`、`Abstract` 和相关时间/所有者字段的格式契约；未编辑零写入、editor baseline、revision、危险内容和 DOM 语义的完整规则引用 ADR-0003 及其编辑器任务。领域任务不实现 sanitizer、HTML 重写或 revision 状态机。

## 5. Test and evidence architecture

1. **Domain unit**：在 `app/info` 运行 JSON、ID/时间、nil/empty、模型目录和 DTO shape 测试；测试依赖只允许标准库/中立值包。
2. **Persistence contract**：在 `app/db` 运行 BSON registry、旧文档兼容和 round-trip 测试；Mongo 7/8 集成及超时/错误证据由 infrastructure/delivery 负责。
3. **Application behavior**：应用任务消费 USN、所有权、冲突和 note-save fixture；不把 live HTTP/Mongo 作为 domain unit 的完成条件。
4. **HTTP/Golden**：interface/delivery 负责 route/action、状态码、header 和真实请求回放；domain 只提供 variant 目录与静态输入。
5. **HTML/browser**：presentation/delivery 负责真实编辑器和浏览器矩阵；domain 仅引用 ADR-0003 的数据语义。

每层记录发现数、执行数、通过/失败/blocked、commit 和环境；环境缺失只能是未运行或 blocked，不能以静态测试替代真实证据。

## 6. Migration and rollback

实施顺序：模型目录与 fixture → 中立 ID seam → app/info DB-independent 测试 → 将 BSON 适配责任交给 infrastructure → 将 API/input variant 交给 application/interface。每一步都能通过依赖扫描和契约测试单独回滚；不保留双事实来源、静默 fallback 或在 controller 中重写领域规则。领域任务回滚只撤销领域类型 seam、目录和纯契约测试，不撤销下游已独立提交的业务迁移。

## 7. 已确认决策与实施约束

- **D-01**：notebook/tag 删除分配新用户 USN，写入 `IsDeleted=true` tombstone 并进入 sync；现有冲突 envelope/HTTP 外形不变。
- **D-02**：使用 `app/domain.ObjectID`；`app/db` 做 BSON 转换；`lea.ObjectID` 兼容别名/转换仅在迁移适配层保留至下游迁移完成。
- **D-03**：动态字段保持 JSON-compatible 值域，集中拒绝不可编码值，保留 nil/empty wire 行为，不加静默 fallback。
- **D-04**：按 route/action 冻结已有 message/status/Content-Type/envelope，缺证据先补 fixture，不跨端点统一错误码。
- **D-05**：`app/info` 及 DB-independent 测试不得依赖 `app/lea`、Revel 或 Mongo driver。
- **D-06**：USN 原子递增/CAS+重试；note-save 优先同一事务，失败不消耗 USN/不返回成功；无事务时显式补偿并暴露 `partial_write`。

上述决定已闭合。若实现改变 JSON/BSON/API 字节或持久化状态，必须先同步更新目录、Golden、fixture 和下游依赖说明，并重新走评审。
