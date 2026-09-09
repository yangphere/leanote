# `09-08-domain-contracts` 规格审核记录（2026-09-08）

## 结论

已按父任务的轨道优先顺序选中并激活 ready 叶 `09-08-domain-contracts`。该任务为叶节点，`task.json.meta.depends_on` 为空；应用层、持久化层和呈现层任务均直接或间接依赖它。

原规格方向正确，但把持久化模型、API DTO、Web/模板视图模型和请求绑定输入混为一个“模型契约”，并把 API、USN、所有权、HTML 和驱动迁移的验收边界混在一起。以下问题已在本轮 PRD、技术设计和执行计划中拆开并补足。审核阶段先只修改任务规格与研究材料；D-01～D-04、D-06 获确认后，已进入受边界约束的领域实现。

## 审核依据

| 证据 | 现场事实 | 用途 |
|---|---|---|
| `CONTEXT.md:11-46` | 定义 `/api/*`、USN、用户所有权、Golden、Schema 与未编辑 HTML 不变量 | 冻结术语和跨层不变量 |
| `docs/adr/0001-stage-modernization-as-contract-first-dag.md` | 采用契约优先 DAG，子任务依赖写入 `meta.depends_on` | 确定本任务先于应用/适配层 |
| `docs/adr/0002-replace-mgo-and-revel-behind-compatibility-boundaries.md:8-25` | Mongo driver 迁移在 `app/db`，字段/集合/ObjectId JSON 兼容；随后迁移 Revel | 确定领域与基础设施边界 |
| `docs/adr/0003-modernize-frontend-with-generated-asset-contract.md:18-26` | 未编辑 HTML 字节不变；真实编辑仅允许登记的非语义规范化 | 确定 HTML 契约来源，不重复定义 sanitizer |
| `app/info/model_contract_test.go:48-306` | 真实模型测试检查 205 个旧 BSON tag、BSON round-trip、JSON goldens 和显式 tag | 评估现有模型覆盖 |
| `app/info/model_contract_generated_test.go:256-421` | `fixtureKeySets`、`jsonFullGoldens`、`contractRegistry` 是生成快照；registry 有 35 个类型，JSON full golden 有 42 个条目 | 识别覆盖边界和缺口 |
| `app/db/Mgo.go:17-58,116-158` | 观察到 28 个业务集合，其中 `has_share_notes`、`note_images` 对应模型在 registry 中未出现 | 补齐持久化清单 |
| `app/info/Api.go:11-118`、`app/info/Re.go:6-18` | API 返回/输入结构没有统一 envelope；`ApiRe`、`AuthOk`、`ReUpdate`、`Re` 并存 | 约束 API DTO 与 envelope 规则 |
| `app/tests/harness/usn_test.go:11-207` | note、notebook、tag 的 mutation/sync/conflict 回放；明确记录 notebook delete 与 tag delete 的基线差异 | 区分规范不变量和已知缺陷 |
| `go list -deps ./app/info`（seam 实现前） | 基线输出包含 `github.com/revel/*` 和 `go.mongodb.org/mongo-driver/v2/*` | 证明原依赖 seam 尚未完成 |

## 覆盖盘点

### 类型分类

截至审核时 `app/info` 共有 69 个实际编译的导出类型；另有 3 个处于注释块中的唯一类型声明（`TagNote`、`TagsCounts`、`ShareNotebooksByUsers`；`ShareNotebooksByUser` 的注释版结构与 active map 同名，不重复计数），它们只作为 `commented_declarations` 记录，不计入 active 模型目录：

- **持久化模型**：以 `app/db/Mgo.go` 的业务集合为准，包括 `Notebook`、`Note`、`NoteContent`、`NoteContentHistory`、`ShareNote`、`ShareNotebook`、`HasShareNote`、`User`、`Group`、`GroupUser`、`Tag`、`NoteTag`、`TagCount`、`UserBlog`、`Token`、`Suggestion`、`Album`、`File`、`Attach`、`NoteImage`、`Config`、`EmailLog`、`BlogLike`、`BlogComment`、`Report`、`BlogSingle`、`Theme`、`Session`。`Blogs` 集合虽在 DAO 中声明，但当前未找到业务读写调用，不能据此虚构模型契约。
- **API DTO/envelope**：`ApiNote`、`NoteFile`、`ApiNoteContent`、`ApiUser`、`ApiNotebook`、`ApiRe`、`AuthOk`、`ReUpdate`、`Re`。其中 `ApiNote` 既被请求绑定又被响应/同步使用，后续可拆为内部类型，但线上 JSON 字节必须不变。
- **请求绑定输入**：`NoteOrContent`、`UserAccount`、`UserBlogBase`、`UserBlogComment`、`UserBlogStyle` 以及 API 中的 `ApiNote`/`NoteFile`。绑定字段名、缺失字段的零值、重复表单值和 `Tags` 的字符串/数组差异必须单独记录，不能从 BSON tag 推导。
- **Web/模板和内部投影**：`Page`、`BlogInfoCustom`、`Post`、`ArchiveMonth`、`Archive`、`Cate`、`BlogItem`、`BlogCommentPublic`、`BlogUrls`、`UserAndBlog`、`UserAndBlogUrl`、`UserBlogBase`、`ShareNotebooks`、`Notebooks`、`ShareNoteWithPerm`、`ShareUserInfo` 及共享映射/中间结构。这些类型可被 JSON 或模板消费，但不应自动当成 Mongo 文档。

现有生成快照覆盖 35 个 registry 类型和 42 个 JSON full golden，但没有覆盖 `ApiNote`、`ApiUser`、`ApiRe`、`AuthOk`、`ReUpdate`、`Re`、`NoteFile`、`NoteImage`、`HasShareNote` 等实际边界类型；`HasShareNote` 在 `app/service/ShareService.go` 中确实写入 `has_share_notes`，`NoteImage` 在 `NoteImageService.go` 中读写 `note_images`。因此“覆盖所有对外 API 和 Mongo 字段”不能用当前 registry 数量代替，必须建立固定路径 `.trellis/tasks/09-08-domain-contracts/research/model-catalog.json`，并按 `model-catalog.schema.json` 和标准库校验脚本复核完整清单。

### 序列化事实

- API JSON 默认使用 Go 导出字段名和声明顺序；当前多数结构没有 `json` tag。字段名、字段顺序和数组顺序属于客户端可观察行为，除非有明确兼容批准，不得用重排或新 `omitempty` 改写。
- `time.Time` 继续使用标准库 JSON 的 RFC3339Nano 表示；Golden 以 UTC `Z` 形式归一化，BSON 侧使用日期类型。比较时间时按瞬时相等，不因时区位置差异制造漂移。
- nil slice/map 在 JSON 中保持 `null`，非 nil 空 slice 保持 `[]`；`Re.List`/`Item` 等 nil interface 保持 `null`。BSON `omitempty` 只按字段 tag 和驱动实际规则判断。
- `lea.ObjectID` 当前 JSON 零值为 `""`、非零值为小写 24 位 hex；兼容输入还需把 JSON `null` 映射为 zero，number/object 和非法字符串必须报错。BSON 使用 ObjectId。驱动迁移快照明确记录：定义为 `[12]byte` 的 ObjectID 即使带 `omitempty`，零值在 mongo-driver/v2 下会按零 ObjectId 出现，不能误套用 mgo 的旧 `MarshalError` 结论。
- Golden 动态值只替换已登记的 ObjectID、时间、token、logo 和 Unix 时间字段；不得对 JSON 键排序、删除未知字段或把任意错误响应归一化成成功。
- `ToGroup`、`Group.Users`、`ThemePath` 等字段在 struct 上可能仍带历史 `bson` tag，但注释明确其为显示投影或派生值；struct-level round-trip 只能证明可编码，不能证明已持久化。目录必须区分物理 tag 来源与 `read_write`/`projection_only`/`unknown` 状态，adapter 负责排除投影字段写入。

### 数据约束事实

- `ShareNotebookNoteInfo.go` 的注释表达 `ShareNote` 唯一键为 `(UserId, ToUserId, NoteId)`、`HasShareNote` 唯一键为 `(UserId, ToUserId)` 的历史意图；本轮没有 `listIndexes` 或部署 fixture 证明运行时索引，因此目录状态必须为 `historical_intent`/`unknown`，由 infrastructure 任务闭合，不能写成已确认运行约束。
- 业务代码大量使用 `db.MustObjectIDFromHex`，非法 ID 可能以 panic 结束；当前规格没有明确输入边界和错误映射，因此已改为要求 adapter 显式校验，并把未登记的 message/status 列为 unknown。
- `Usn`/`Seq`/分页参数由多个 controller/service 直接接收整数，统一范围、溢出和缺省值没有单一事实来源；规格现要求按 action 建目录，不能静默夹值或借用其他端点默认值。
- `Tags` 同时存在 form 逗号字符串和 API 数组，`Re.List`/`Item`、`Theme.Info` 等动态值也缺少集中 schema；规格要求保留现有 wire shape，并通过集中校验显式记录值域。

### 行为事实和基线缺陷

- API 未形成单一响应形状：未登录通常为 HTTP 200 的 `{"Ok":false,"Msg":"NOTLOGIN"}`；冲突为 HTTP 200 的 `Ok=false, Msg=conflict, Usn=0`；同步接口返回数组；认证成功使用 `AuthOk`；更新使用 `ReUpdate` 或 `Re`。领域规格只能冻结这些可观察变体，不能要求所有端点套用一个新 envelope。
- 所有权条件必须与资源条件在同一查询中表达；公开博客读取是有意的 `IsBlog/IsTrash/IsDeleted` 例外，共享读取还需 `ToUserId` 或群组成员条件。`GetNoteById` 这类不带 owner 的内部 helper 不能被私有 API 直接复用，是否清理由应用任务处理。
- `TestUSNMutationPairsAndConflicts` 记录两个已知基线差异：删除 notebook 不增加用户 USN 且同步为空；删除 tag 会增加用户 USN，但记录仍保留旧输入 USN 且同步为空。这与 `CONTEXT.md` 的理想 mutation→sync 不变量冲突，不能在领域任务中擅自判定“修复”或“永久兼容”。
- `note_save_contract_test.go` 证明更新失败、权限失败、重复 ID 和元数据成功但内容失败必须返回可判定失败 envelope；当前部分写入不是事务回滚。领域任务冻结“不得伪造成功”，事务性、USN 消耗和重试幂等性由 D-06 交给 application-notes 与 infrastructure-persistence 明确决策。

## 必须保留的跨层不变量

1. **身份与编码**：领域 ID 不携带 Revel/Mongo 类型；API ID 和时间字节与 Golden 一致；BSON 转换集中在持久化边界。
2. **模型责任**：数据库文档、API DTO、模板投影和请求输入拥有各自清单与转换责任；同一字段的命名/空值规则必须引用唯一来源。
3. **所有权**：私有资源查询强制带 owner；共享资源带显式授予条件；公开博客仅允许登记的公开谓词；not-found 与 forbidden 不泄露资源存在性。
4. **USN**：对已确认的 note/notebook/tag 基线逐项回放 mutation、返回 USN、sync 游标、分页和冲突；已知差异必须标注，不得在本任务隐式改变。
5. **失败可见**：invalid ID、not-found、duplicate key、冲突、部分写入和 DB 错误必须映射到可观察失败；无依据时先增加 fixture/决策记录，不能返回空对象伪装成功。
6. **HTML**：沿用 ADR-0003 的 persisted bytes/editor baseline/revision 和逐项 DOM 语义夹具；领域任务只冻结数据契约，编辑器状态机由呈现/应用任务验收。

## 已确认决策及影响

以下事项在审核阶段无法仅凭代码可靠推断，已由用户确认采用推荐方案；这些决策现在是实现约束，不再是开放门：

| 编号 | 审核问题 | 已确认方案 | 影响 |
|---|---|---|---|
| D-01 | notebook delete、tag delete 的 USN/sync 差异是永久兼容还是应单独修复 | 已决定修复：删除分配新用户 USN，写入 `IsDeleted=true` tombstone 并进入 sync；冲突 envelope/HTTP 外形保持不变 | application-notes 必须更新 mutation/sync 回放；旧差异只作 known-defect 对照 |
| D-02 | `app/info` 的中立 ID 类型放在哪个无框架包，以及 `lea.ObjectID` 兼容别名/转换的寿命 | 已决定新建 `app/domain.ObjectID`；`app/db` 提供 BSON adapter；兼容转换保留至所有下游迁移完成后移除 | `app/info` 依赖图必须收敛，包名不可再变更 |
| D-03 | `Re.List`/`Item`、`Theme.Info` 等动态字段是否继续接受任意 JSON 值 | 已决定保持 JSON-compatible 值域，集中拒绝不可编码值，保留 nil/empty wire 行为，不加静默 fallback | DTO 拆分必须维持既有 wire shape |
| D-04 | duplicate key、invalid ID、private not-found 在各 API 端点的最终 message/status 是否全部沿用当前字符串 | 已决定按 route/action 冻结已有 message/status/Content-Type/envelope；缺证据先补 fixture，不跨端点统一错误码 | interface-http 必须逐端点登记，缺口仍标 unknown |
| D-06 | USN 并发分配、note-save 部分写入后的持久化状态和重试幂等语义由哪个应用/持久化任务负责，以及是否在本轮修复 | 已决定 USN 采用 `$inc`/`findOneAndUpdate` 或 CAS+重试；note-save 优先同事务，失败不消耗 USN/不返回成功；无事务须显式补偿并暴露 `partial_write` | application-notes 与 infrastructure-persistence 共同实现和回归并发、部分写入与幂等 |

`D-05` 已确定为硬性依赖边界：生产 `app/info` 及 DB-independent 测试不得引用 `app/lea`、Revel 或 Mongo driver；兼容转换只允许在下游适配层。该项不再作为开放决策。

已确认决策不得通过隐藏 fallback 规避；仅缺少运行证据的具体端点、Mongo 索引或浏览器场景仍保持 unknown/未运行，并由责任下游补齐。

## 本轮规格修复清单

1. PRD 新增模型分类、序列化/空值、API 变体、所有权、USN 缺陷、HTML 来源和依赖扫描边界。
2. 设计文档将中立 ObjectID seam、JSON/BSON/API/模板/输入责任分层，并声明现有测试覆盖不足和兼容转换位置。
3. 执行计划拆分 DB-independent 模型验证、Golden/USN 回放、依赖扫描和下游交接；不再把需要 Mongo/真实 HTTP 的检查当作领域任务本地完成条件。
4. `check.jsonl`/`implement.jsonl` 纳入本审核材料及直接证据文件，避免只引用宽泛 workflow 文件。
5. 固定模型目录 artifact 路径和 `model-catalog.schema.json`/`validate_model_catalog.py` 校验入口，明确 69 个 active 类型与 3 个注释声明的计数边界。
6. 将 BSON tag 的物理来源、projection-only 状态、历史索引意图、ObjectID `null`、USN 并发和 note-save 部分写入责任同步到设计、执行计划和证据矩阵。

## 实现阶段补充

模型目录已将 29 个 active API action 展开为方法、认证前置、请求来源/字段、成功/失败外形和证据状态；真实 HTTP 尚未运行，因此统一保留 `evidence_status=partial`，没有把静态来源误报为端到端通过。请求绑定结构的字段级静态契约另存于 `input-contracts.json`，并由 `input-contracts.schema.json` 与 `validate_input_contracts.py` 校验；form 重复值、`Tags` 解析、非法整数/ID 及 Content-Type 仍是下游 unknown/待运行证据。

## 审核边界

本记录证明规格审核已完成且 D-01～D-04、D-06 已获明确决策；模型目录和中立 ID seam 已开始实现并有局部静态证据，但真实 Mongo/HTTP/浏览器证据仍未运行。D-05 是已固定的依赖边界；后续下游功能编码须按本记录和交接门继续进行。

## 2026-09-09 实施补充

在用户确认全部推荐决策后，领域叶完成了中立 `ObjectID`、`app/info` 依赖收敛、JSON 动态字段边界和 `app/db` BSON 适配回归。`Re` 与 `Theme` 的 `MarshalJSON` 现在统一经过集中 JSON-compatible 校验，并保留既有字段顺序与 nil/空集合形状；非法值错误包含字段路径。完整命令结果见 `research/validation-2026-09-09.md`。

本叶仍不实现 D-01 notebook/tag 删除同步或 D-06 USN/笔记事务：两项已决定的目标由 `application-notes` 与 `infrastructure-persistence` 负责，当前仅提供可交接的契约和证据边界。
