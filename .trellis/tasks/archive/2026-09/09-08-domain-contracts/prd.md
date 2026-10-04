# 领域层：模型与跨层契约 — PRD

## Goal

把 Leanote 当前由 `app/info`、Golden、USN 回放和 ADR 共同表达的稳定业务语义整理为一套可执行、可追溯的领域契约，供应用服务、Mongo 持久化、HTTP 适配和前端呈现共同消费。该任务先冻结边界和回归材料，再允许下游任务编码；不借模型重构之名改变公开行为。

## Context and observed baseline

- `CONTEXT.md` 冻结了 `/api/*`、USN、用户所有权、Mongo Schema、Golden 和未编辑 HTML 的项目术语与不变量。
- `app/info` 同时承载数据库文档、API 响应 DTO、请求绑定结构、模板投影和内部组合结构；它们不能仅凭是否有 `bson` tag 来分类。
- 当前模型快照注册 35 个类型、记录 205 个旧 BSON tag，并提供 BSON round-trip 与 42 个 JSON full goldens；这不是完整 API/持久化覆盖。`HasShareNote` 和 `NoteImage` 已被业务服务读写，却未进入注册快照。
- 审核基线中的 `app/info` 曾通过 `app/lea` 间接带入 Revel 和 Mongo driver；本实现通过中立值类型 seam 收敛，后续改动不得重新引入该依赖方向。
- API 存在多个并行响应形状（直接数组/模型、`ApiRe`、`AuthOk`、`ReUpdate`、通用 `Re`），不能在迁移时臆造一个新 envelope。

## Scope

### 1. 模型与责任分类

建立一份穷举清单，覆盖 `app/info` 的每个导出类型，并为每个类型标注唯一主责任；跨边界复用只能作为显式的 secondary role，不得产生第二套字段事实：

1. **持久化模型**：与 `app/db/Mgo.go` 业务集合对应的 `Notebook`、`Note`、`NoteContent`、`NoteContentHistory`、`ShareNote`、`ShareNotebook`、`HasShareNote`、`User`、`Group`、`GroupUser`、`Tag`、`NoteTag`、`TagCount`、`UserBlog`、`Token`、`Suggestion`、`Album`、`File`、`Attach`、`NoteImage`、`Config`、`EmailLog`、`BlogLike`、`BlogComment`、`Report`、`BlogSingle`、`Theme`、`Session`。DAO 中声明但当前无业务读写证据的 `Blogs` 集合必须单独标注为“待确认”，不得虚构 schema。
2. **API DTO/envelope**：`ApiNote`、`NoteFile`、`ApiNoteContent`、`ApiUser`、`ApiNotebook`、`ApiRe`、`AuthOk`、`ReUpdate`、`Re`。`ApiNote` 既作为绑定输入又作为输出/同步值时，目录只设一个 primary role，并在 secondary role 中记录用途；线上 JSON 必须字节兼容。
3. **请求绑定输入**：`NoteOrContent`、`UserAccount`、博客设置部分更新输入 `UserBlogBase`、`UserBlogComment`、`UserBlogStyle` 以及 API 的 `ApiNote`/`NoteFile`。其中 `UserAccount` 当前是 service-only 的管理命令载荷，暂无直接 controller binder 证据，必须保持该 unknown 状态；领域只冻结字段、required/optional 和 wire shape。绑定字段名、缺失字段零值、重复表单值、字段 presence、`Tags` 字符串/数组及博客设置的跨字段校验由对应 interface/application 验收。
4. **Web/模板和内部投影**：`Page`、博客/分享组合结构、`NoteAndContent`、`Notebooks`、`ShareNotebooks`、`ShareNoteWithPerm`、`ShareUserInfo` 等。它们可被模板或 JSON 消费，但不能自动视为 Mongo 文档。

### 2. 序列化契约

- **JSON**：保留当前导出字段名、声明顺序、嵌入展开方式、数组顺序和 `time.Time` 的 RFC3339Nano 表示；不得借新增 `json:"...,omitempty"` 删除现有字段。nil slice/map/interface 保持 `null`，非 nil 空 slice 保持 `[]`。动态字段只接受可被 `encoding/json` 表示的值，拒绝不可编码值时必须返回可观察错误。
- **ObjectID**：领域值类型不得依赖 Mongo/Revel；零值 JSON 为 `""`，非零值为小写 24 位 hex；为保持当前 `lea.ObjectID` 输入兼容，JSON `null` 也映射为 zero，number/object 和非法字符串必须报错。BSON 由 `app/db` 转换为标准 ObjectId。读取旧文档时允许空字符串和 24 位 hex，其他字符串必须报错。现有 mongo-driver/v2 快照中带 `omitempty` 的零 ObjectID 会出现零 ObjectId，这一事实须单独测试，不能套用 mgo 的旧 `MarshalError` 假设。
- **时间**：JSON 使用标准库 RFC3339Nano；BSON 使用日期类型；比较按瞬时相等，Golden 归一化为 UTC `Z`，不因时区位置改变字段语义。
- **BSON**：逐字段冻结 key、`omitempty`、空值、嵌入和排序/分页依赖的字段；所有实际持久化字段必须有显式 tag。`app/info` 的 driver-agnostic struct tag 是字段命名的唯一来源，`app/db` 只负责读取该来源并执行 ObjectID/date 转换，不得维护第二套字段表。`ApiNoteContent`/`ApiNotebook` 的历史 tag 只作为 API 字段快照，不构成 DB 读写证据；`UserAccount` 与 `UserBlog*` 这类 partial-update/service command 字段标为 `write_only`，不可冒充完整文档。带有“仅显示/不保存”注释的字段（如 `ToGroup`、`Group.Users`、`ThemePath`）必须在目录中标注 persistence read/write 状态；在真实读写证据闭合前保持 unknown，不得从 struct-level round-trip fixture 推断为已存储。

### 3. API、输入和错误

- 建立 route/action → 请求 DTO → 成功形状 → 失败形状 → 状态码/Content-Type 的契约索引，索引来源为 `conf/routes`、API controller 和现有 Golden；缺失证据的端点标为 unknown 并补 fixture，不以默认值填空。文档、controller 与 Golden 的方法/参数/类型冲突必须在目录 `compatibility_notes` 中逐项记录；当前已知冲突包括 `user/info` 的 userId 与 session 输入、`getSyncState` 的 GET/POST 和 Unix 时间整数、部分 action 的 GET/POST 观察差异、file 读取 token 白名单，以及 `getSyncTags` 的实际 `[]NoteTag` 返回形状。
- 现有变体必须保持：未登录通常是 HTTP 200 的 `Ok=false, Msg=NOTLOGIN`；冲突是 HTTP 200 的 `Ok=false, Msg=conflict, Usn=0`；同步接口返回数组；认证成功返回 `AuthOk`；更新操作按现有端点使用 `ReUpdate` 或 `Re`。不得把直接模型/数组强行包进新 envelope。
- `Ok`、`Msg`、`Code`、`Id`、`List`、`Item`、`Usn` 及动态 `List/Item` 的 nil/空值行为必须有 fixture；错误不能被吞掉、改写为成功或通过零值模型伪装。
- invalid ID、not-found、forbidden、duplicate key、USN conflict、部分写入和数据库失败必须逐操作记录当前可观察语义。若当前代码没有稳定映射，先登记决策和回归材料；本任务不跨端点统一错误码。

### 4. 数据约束与边界

- ID 输入按字段目录标注 required/optional：required ID 必须是严格 24 位 hex；optional blank 映射 zero；格式错误、溢出和未知 BSON 类型必须在边界返回可判定错误，不得让 `MustObjectIDFromHex` 之类 panic 穿过 HTTP。
- `Usn`、`Seq`、分页页码/大小保持现有整数 wire type；不得静默截断、负数转正或把越界值夹到默认值。具体允许范围和缺省值必须逐 action 从 controller/validator/Golden 登记，缺证据即 unknown。
- 唯一性约束必须列入持久化目录并由 infrastructure 通过 `listIndexes`/部署 fixture 验证：`ShareNote` 的 `(UserId, ToUserId, NoteId)`、`HasShareNote` 的 `(UserId, ToUserId)` 目前仅是历史意图，证据闭合前标为 unknown；索引冲突不得被吞成成功，其他索引不能凭模型名称推断。
- 删除使用 `IsDeleted`/`IsTrash` 等既有字段语义；软删除是否进入 sync、not-found 或公开读取按现有 action 逐项登记，不在领域层统一重写。
- 字符串按现有 UTF-8/大小写/空串语义传递；`Tags` 的 form 逗号分隔和 API 数组表示不得互相误解，去空白、去重和排序只有在已有证据时才允许。

### 5. 业务不变量

- 私有资源的 owner 条件必须和资源条件在同一查询边界表达；共享资源还要带显式 `ToUserId`/群组成员条件；公开博客读取只能使用登记的公开谓词。not-found 与无权限不得泄露资源存在性。
- 对 note、notebook、tag 的 mutation、返回 USN、`GetSync*` 游标、排序、分页、删除位和冲突进行可重复回放。D-01 已决定修复 notebook/tag 删除同步缺陷：成功删除必须分配新的用户 USN，保存 `IsDeleted=true` 的 tombstone 并使用该新 USN，sync 返回删除记录；现有冲突 envelope 和 HTTP 外形保持不变。旧行为仍保留在 known-defect ledger，作为回归前后对照，不得继续作为通过标准。
- USN 分配必须在并发 mutation 下保持唯一、单调且无丢失；D-06 已决定由 application/infrastructure 使用 Mongo `$inc`/`findOneAndUpdate` 或 CAS+重试实现原子分配，并用并发回归证明。当前先读后写实现只能作为待修复基线，不能宣称不变量通过。
- note save 的元数据/内容写入优先放入同一事务；事务失败时不消耗 USN、不返回成功，重试必须幂等。若部署环境不支持事务，必须显式采用经过评审的补偿/idempotency 方案并返回 `partial_write`，不得隐藏 fallback 或伪造全成功。D-06 的责任归属为 `application-notes` 与 `infrastructure-persistence` 共同实现和回归。
- 沿用 ADR-0003：未发生内容修改时不写回存量 HTML 字节；真实编辑只允许逐项登记的非语义 HTML 规范化，文本、链接、图片、代码块、表格和第一方插件标记的 DOM 语义保持等价。编辑器 revision 状态机由应用/呈现任务实现，本任务只冻结数据和验收入口。

### 6. 依赖与兼容性

- 生产 `app/info` 及其 DB-independent 测试不得依赖 Revel、HTTP writer、Mongo client/collection、前端代码或 `app/lea` 的框架聚合包；该限制已确定，不再作为待决策项。D-02 已固定新建 `app/domain`（包名 `domain`）承载中立 `ObjectID`；`lea.ObjectID` 仅作为迁移期兼容别名/显式转换，待所有下游迁移完成后移除，兼容转换只能由适配层使用。
- 不改变公开 URL/API、Mongo collection 名、BSON key、ObjectID JSON 形状、时间格式或 Schema，不做历史数据迁移。
- 本任务只产出领域类型 seam、契约目录、DB-independent 回归和缺陷/决策材料；service/controller/db/HTTP/前端业务迁移由依赖任务实现。为保证 `domain.ObjectID` 的 BSON 形状不回退，允许 `app/db` 适配层仅接入既有 `CodecRegistry`，不得借此迁移查询或业务规则。

## Acceptance criteria

- [x] `research/model-catalog.json` 按 `research/model-catalog.schema.json` 完成；每个 active 导出类型有唯一 primary role 和消费者，secondary role 显式登记，28 个有业务读写证据的集合均映射到模型，`Blogs` 等无证据集合列入待确认清单；`HasShareNote`、`NoteImage` 不再被 registry 漏报；D-01～D-04、D-06 以 decided 状态记录并附影响与证据。
- [ ] 持久化契约测试覆盖清单中的每个实际文档字段：完整 BSON key、显式 tag、`omitempty`/零值矩阵、ObjectID（含零值、`null` JSON 输入和旧字符串读取）、时间、nil/空集合和 round-trip；driver-dependent 测试归 `app/db`/infrastructure，Mongo 7/8 运行证据由 `infrastructure-persistence`/delivery 任务提供。
- [ ] API DTO/envelope 契约覆盖所有已登记 public API 响应变体；当前模型目录已登记 29 个 active action，并以 `partial` 保留尚未运行的 HTTP 证据。领域材料只提供字段/输入 schema（`input-contracts.json` 覆盖 `ApiNote`、`NoteFile`、`NoteOrContent`、`UserAccount`、`UserBlogBase`、`UserBlogComment`、`UserBlogStyle`），真实 binder 的缺失/重复/非法输入、`Tags` 解析、博客设置字段 presence/跨字段规则、状态码/Content-Type 和错误响应由 `interface-http`/对应 application fixture 或明确 unknown 记录。现有 42 个 JSON golden 只能作为子集，不能作为完成证明。
- [x] 中立 ObjectID seam 完成后，`go list -deps ./app/info` 与 `go list -deps -test ./app/info` 均不出现 Revel 或 Mongo driver；BSON adapter 的 driver 依赖仅在 `app/db`，JSON/hex 行为在领域侧有 DB-independent 回归。
- [ ] USN/所有权/冲突/删除回放材料可由下游任务直接消费，并逐项标注基线通过、目标修复或已知缺陷；D-01 的删除目标必须验证新 USN、tombstone 和 sync 返回，D-06 的并发与事务/补偿语义必须有独立 fixture，不能把旧差异隐式改成“通过”。
- [ ] HTML 契约引用 ADR-0003 的 fixture/DOM 语义和未编辑零写入门禁；本任务不引入 sanitizer、编辑器状态机或批量 HTML 迁移。
- [ ] 领域任务的本地验证仅要求 DB-independent Go 测试、`go vet ./app/info`、依赖扫描和 `git diff --check`；目录 schema 校验命令必须可复现。真实 Mongo、HTTP、浏览器、PDF 和发布证据明确留给下游任务，不因环境缺失伪造通过。
- [ ] 本任务改动路径仅限领域类型 seam（获批准进入实现阶段后）、契约测试/fixture、`.trellis/tasks/09-08-domain-contracts/**` 和研究/验收材料；`app/db` 只允许上述 BSON adapter 接入，不修改查询/业务逻辑、CI 或生成资源。

## Out of scope

- 不迁移或重写 controller/service、Mongo driver/collection、HTTP 路由、Revel 启动链、模板、前端编辑器或发布流程。
- 不改变公开 URL/API/Schema、错误文案的跨端点统一策略或历史 HTML；发现既有缺陷只进入缺陷 ledger，缺少运行证据的细节标记为 unknown。
- notebook/tag 删除按 D-01 修复为新 USN tombstone 并进入 sync；其他 mutation 是否同步仍按 action 证据逐项登记，不跨集合推导统一规则。
- 不创建第二套 fixture/模型事实来源；测试必须引用真实 `app/info` 类型、现有 Golden/USN 或明确登记的最小输入。

## 已确认决策

以下决策由用户确认采用推荐方案；实现和验收必须按此执行，不得重新开放或用隐式默认值替代：

1. **D-01 — 删除同步修复**：notebook/tag 成功删除分配新的用户 USN，保存 `IsDeleted=true` tombstone 并使用新 USN，sync 返回删除记录；现有冲突 envelope 和 HTTP 外形保持不变。影响：`application-notes` 必须更新 mutation 与 sync 回放，客户端兼容以新 tombstone 语义为准。
2. **D-02 — 中立 ID seam**：新建 `app/domain` 包，提供 zero/parse/Hex/JSON 的 `domain.ObjectID`；`app/db` 负责 BSON 转换；`lea.ObjectID` 只保留迁移期兼容别名/显式转换，所有下游迁移完成后移除。影响：`app/info` 的生产与 DB-independent 测试依赖图必须收敛。
3. **D-03 — 动态 JSON 值域**：`Re.List`、`Re.Item`、`Theme.Info` 等继续接受 JSON-compatible 值；集中校验不可编码值，保留 `nil -> null` 和空 slice/map 的现有行为，不新增静默 fallback。影响：DTO 拆分不得改变 wire shape。
4. **D-04 — route/action 错误映射**：按 route/action 冻结现有 message、status、Content-Type 和 envelope；缺证据端点先补 fixture，禁止跨端点统一错误码。影响：HTTP 任务必须逐端点登记并对缺口保持 unknown。
5. **D-06 — 原子 USN 与 note-save 事务**：USN 用 `$inc`/`findOneAndUpdate` 或 CAS+重试原子分配；note 元数据、内容和 USN 优先同一事务，失败不消耗 USN、不返回成功；不支持事务时须采用显式补偿/idempotency 并暴露 `partial_write`。影响：`application-notes` 与 `infrastructure-persistence` 共同负责实现、并发和部分写入回归。

`D-05`（领域生产代码及 DB-independent 测试不得导入 `app/lea`、Revel 或 Mongo driver）已经是本任务的硬性边界，不再作为待决策项；兼容转换只能由下游适配层承载。

## Notes

本 PRD 是已完成决策闭合的可实施版本。它冻结当前可证实的行为和上述目标修复；完成规格审核不等于功能实现或真实集成证据已完成。未能由现有上下文确定的端点细节仍须以逐端 fixture 标为 unknown，不得擅自推断。
