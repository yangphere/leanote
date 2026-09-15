# Research: `09-08-application-notes` 需求规格全面审核

- Query: 审核应用层笔记工作区与同步任务的目标、范围、输入输出、流程、边界、异常、数据约束、兼容性、依赖和验收是否足以直接指导实现，并对照现有 note/notebook/tag/trash/history/suggestion/sync 行为与已归档 domain/persistence 交接。
- Scope: internal
- Date: 2026-09-11

## 结论

`09-08-application-notes` 在任务图上是 ready 叶。审核开始时 `task.json.meta.depends_on` 只包含已归档的 `09-08-domain-contracts`；由于 D-06 必须消费/扩展 persistence seam，本轮已补入同样归档且 `completed` 的 `09-08-infrastructure-persistence`，因此 ready 状态不变。但原有 PRD/design/implement **尚不足以直接进入编码**。D-01（notebook/tag 删除的新 USN tombstone）和 D-06（原子 USN、note-save 事务/补偿）已经由上游冻结，不应重新征询；除此之外，规格仍有以下高风险缺口：

1. “整个 service 文件及对应 controller”的范围与父任务的 publishing/content/interface-http 分工重叠；没有 action/use-case 级归属表。
2. “所有操作带用户所有权”没有表达共享笔记合法的 owner/actor 双主体模型，照字面实现会破坏协作。
3. D-06 只有目标，没有 note mutation 所需的 repository/transaction 接口、事务参与写集合、standalone 补偿状态机和幂等键定义；已归档 persistence 只提供用户初始化专用 runner。
4. 错误只写“稳定 envelope”，没有逐 action 的输入、成功/失败形状、错误码、HTTP 外形和 DB 状态矩阵。
5. 批量 Web 操作当前无条件返回成功；“不得伪造成功”与“外部契约不变”发生直接冲突，需明确批量失败产品语义。
6. Trash 是附件、note tombstone、content、history、tag count、notebook count 的多写流程，但规格只对 note-save 定义 partial-write；删除/移动/复制/标签删除的原子性和补偿未定义。
7. History 的保留上限、重复内容、并发更新和失败语义未定义；Suggestion 实际是反馈存储加异步邮件，既不属于典型 workspace CRUD，也与 admin/email/outbox 边界重叠。
8. 验收项是服务名称级复选框，缺少可复核的 operation matrix、失败点、发现/执行数量、Mongo 7/8 运行身份和证据状态。

因此，本轮建议只完善 `prd.md`、`design.md`、`implement.md` 和新增 `acceptance/evidence-matrix.md`；关键产品决策未确认前不得激活编码。

## Files found

### 任务、父任务与交接

- `.trellis/tasks/09-08-application-notes/{task.json,prd.md,design.md,implement.md}` — 当前任务元数据与全部人类可读规划材料；本轮已新增 acceptance 证据矩阵。
- `.trellis/tasks/09-08-business-layer-architecture/{prd.md,design.md,implement.md}` — 父任务冻结分层责任、外部兼容和交付顺序；application-notes 只应拥有 notes/workspace 应用用例。
- `.trellis/tasks/archive/2026-09/09-08-domain-contracts/{prd.md,design.md,acceptance/evidence-matrix.md,research/spec-audit-2026-09-09.md}` — D-01/D-06、API variant、ObjectID/BSON/JSON 和 unknown 证据边界。
- `.trellis/tasks/archive/2026-09/09-08-infrastructure-persistence/{prd.md,design.md,acceptance/evidence-matrix.md}` — driver、timeout、错误分类、事务/补偿基础设施与仍为 partial 的交接状态。
- `CONTEXT.md` — 外部 API、USN、所有权、Golden、未编辑 HTML 的稳定术语与不变量。

### 现有实现与测试

- `app/service/NoteService.go` — note metadata/content、sync、copy/move、tag/attachment/image/history 等多写编排；当前直接依赖 `app/db`、BSON 和包级 service 单例。
- `app/service/NotebookService.go` — notebook tree、CRUD、排序、移动、blog 传播和 sync；API 更新/删除的预读未按 owner 限定。
- `app/service/TagService.go` — legacy tags 与 NoteTag、计数、删除和 sync；DeleteTagApi 分配新 USN 却持久化旧请求 USN。
- `app/service/TrashService.go` — 软删除、彻底删除和恢复；彻底删除是多个独立写且覆盖中间失败状态。
- `app/service/NoteContentHistoryService.go` — 内容历史 newest-first 数组；名义上限 10，实际边界会增长到 11。
- `app/service/SuggestionService.go`、`app/controllers/IndexController.go` — feedback insert 与请求路径发起的无结果邮件 goroutine。
- `app/controllers/NoteController.go`、`app/controllers/NotebookController.go`、`app/controllers/TagController.go`、`app/controllers/NoteContentHistoryController.go` — Web 参数 presence、批量操作和历史入口。
- `app/controllers/api/ApiNoteController.go`、`ApiNotebookController.go`、`ApiTagController.go`、`app/controllers/api/httpserver.go` — Revel API 与局部 first-party HTTP adapter；controller 仍承担冲突预检、附件变更、内容重写和 update-map 组装。
- `app/info/{NoteInfo.go,NotebookInfo.go,TagInfo.go,SuggestionInfo.go,Api.go}` — 持久化模型、输入结构和多种响应形状。
- `app/tests/harness/usn_test.go` — note 正常同步以及 notebook/tag 删除旧缺陷的当前 Golden 证据。
- `app/tests/harness/note_save_contract_test.go`、`tests/js/note-save-contract.test.js` — Web note-save 失败 envelope、partial write 和前端仅在成功时确认缓存的边界。
- `app/controllers/api/httpserver_test.go` — first-party tag lifecycle 的局部测试，尚未断言 D-01 删除后的 tombstone/sync。

## Related specs

- `.trellis/spec/backend/database-guidelines.md:15-20`：`app/db` 是 Mongo 边界，controller 不直连 DB；USN 与 history 不变量属于 service。
- `.trellis/spec/backend/database-guidelines.md:46-92`：多写 step 必须事务化或可逆补偿；同一 step 内部分成功不能被通用补偿器忽略。
- `.trellis/spec/backend/error-handling.md:20-32`：service/db 错误向上传播，HTTP 200 不代表业务成功；note-save 每个分支和 partial write 都必须 `Ok:false`。
- `.trellis/spec/backend/quality-guidelines.md:26-30`：每个修复需聚焦回归；HTTP 走真实 server；Mongo Golden replay 保持只读。
- `.trellis/spec/guides/cross-layer-thinking-guide.md`：本任务横跨 HTTP、service、repository、Mongo 和前端保存状态，应有完整数据流与失败矩阵。
- `.trellis/spec/guides/code-reuse-thinking-guide.md:18-38`：所有权、USN、错误映射等重复规则应收敛为单一事实来源。

## Findings

### 1. 已由仓库和上游可靠确定、应直接写入规格的事实

| 主题 | 代码/文档证据 | 应冻结的规格 |
|---|---|---|
| 任务依赖 | 审核开始时 leaf 只依赖 domain；父任务允许非 identity application 在 domain 后进行，但 D-06 又要求 application-notes 与 persistence 共同实现（父 `design.md:46-53`、归档 domain 交接） | 本轮将已归档 `infrastructure-persistence` 补入 `meta.depends_on`；任务仍 ready。不得把 persistence 的未闭合跨层证据误报为依赖未满足或 application 通过 |
| D-01 | domain PRD 已决定成功 notebook/tag delete 分配新用户 USN、持久化 `IsDeleted=true` 且记录该新 USN、sync 返回 tombstone（归档 domain `prd.md:51-53,84-88`） | 不再把“是否修复”列为待确认；冲突 envelope、HTTP status/Content-Type 和非删除 action 的分页排序保持原状 |
| notebook 旧缺陷 | API `DeleteNotebookForce` 先无 owner 读取，再物理删除（`NotebookService.go:303-312`）；旧 harness 明确要求用户 USN 不变且 sync 空（`usn_test.go:57-65`） | 旧断言必须改为 known-defect before-fixture；目标断言 tombstone/new-USN/sync；not-found/conflict 外形不变 |
| tag 旧缺陷 | `DeleteTagApi` 在 122 行分配新 USN，却在 123 行写入请求 `usn`（`TagService.go:112-126`）；旧 harness 在 `usn_test.go:74-84` 固定了该缺陷 | 持久化值和返回值必须是同一个新 USN；sync(after old USN) 返回 `IsDeleted=true` 的 tag |
| sync 游标 | 三个 sync 查询均以同一用户、`Usn > afterUsn`、`Sort("Usn")`、`Limit(maxEntry)` 执行（`NoteService.go:101-111`、`NotebookService.go:118-123`、`TagService.go:139-144`） | 保持 exclusive cursor、升序和现有 default/maxEntry 外形；边界测试至少覆盖 0、精确最大值、超大 afterUsn 和分页续游标（`usn_test.go:87-123`） |
| 原子 USN 现状 | `UserService.IncrUsn` 是读、+1、写三步（`UserService.go:77-87`） | D-06 必须替换为单一原子 `$inc/findOneAndUpdate` 或 CAS+重试；并发测试证明同用户唯一、单调、无丢失，并验证不同用户隔离 |
| note-save 当前 partial | metadata 先更新并消耗 USN（`NoteService.go:507-540`），content 随后独立更新（`NoteService.go:647-673`）；harness 构造 content 缺失并要求失败 envelope（`note_save_contract_test.go:93-153`） | 事务模式必须把 metadata/content/USN 放在一个 commit 边界；失败不返回成功，且证据读取最终 DB 状态与 USN，而非只看 envelope |
| note-save envelope | Web create 成功必须 `Re{Ok:true,Item:note}`，失败必须非空 Msg（`NoteController.go:178-214,274-278`；`note_save_contract_test.go:14-91,155-214`） | 保留 per-action 的既有 envelope；禁止为“统一”把 Web `Re`、API direct model、`ReUpdate` 混成新响应 |
| 未编辑零写入 | `CONTEXT.md:39-46` 冻结未编辑存量笔记不保存；前端 contract 查验 save capture/cache 更新（`tests/js/note-save-contract.test.js:9-27`） | 验收必须记录“请求未发出、DB metadata/content/history/USN 均不变”，而不仅是页面没有报错 |
| owner/actor 分离 | shared create/update 以 owner 的资源写入，actor 通过 `HasUpdateNotebookPerm` / `HasUpdatePerm` 授权并写 `CreatedUserId`/`UpdatedUserId`（`NoteService.go:427-453,463-479,615-630`） | 所有权规格应改为：私有用例 query 同时带 owner；共享用例显式接收 `OwnerUserID` 与 `ActorUserID`，先验证 share permission，最终写查询仍同时带资源 ID + owner ID；绝不接受 controller 提供的未验证 owner |
| history ownership | history 读写都使用 noteId+userId（`NoteContentHistoryService.go:19-29,43,49-63`） | history 只能随成功 content mutation 写入，actor 写 `UpdatedUserId`，owner 写 history `UserId`；读取跨用户继续返回既有空/not-found 外形 |
| API history 实际状态 | Web history controller 活跃（`NoteContentHistoryController.go:11-15`）；API `GetHistories` 整段注释（`ApiNoteController.go:553-564`） | 不得在本任务顺手新增公开 API history action；除非产品另行批准，范围只保持现有 Web action |
| restore 实际状态 | `TrashService.recoverNote` 无外部调用（`TrashService.go:60-67`，全仓仅该定义命中） | “恢复”不能作为已有可验收功能笼统勾选；需明确是仅由 move-from-trash 完成，还是要新增显式 restore use case |
| controller 业务重复 | API controller 自己做 USN 预检、附件更新、字段 presence、内容链接替换和 update map（`ApiNoteController.go:367-490`）；Web controller也组装另一套 update map（`NoteController.go:217-264`） | 应用层定义 typed command/result；controller 只完成 binding/presence 和 response mapping。URL/HTML/attachment rewrite 属于哪一 application use case 必须列明，不能再复制 |
| application contract 的 driver 泄漏 | 旧 Note/Notebook/Tag/Trash service 经 `app/db` 使用 BSON map；这是仓库兼容边界，不等于 controller 直持 collection | 可执行静态门限定为纯 `app/application/notes` 无 Revel/Mongo/`app/db`/BSON，controller 无 collection/client；不要求全库 BSON 重构 |

### 2. 现有规格的歧义、遗漏和冲突

#### 2.1 范围与任务归属

当前 PRD 用文件名定义范围（`prd.md:7-10`），而这些文件内混有多个父任务领域：

- `NoteService.ToBlog`、`UpdateNoteContentIsBlog` 和 blog tag recount 属于 publishing；`NoteService.go:690-714`。
- attachment/image copy/update 属于 application-content；`NoteService.go:157-197,680-681,804-834` 及 API controller 上传流程 `ApiNoteController.go:381-426`。
- Revel/first-party handler registry、参数绑定、HTTP status/Content-Type 属于 interface-http；父任务 `prd.md:32-36`。
- Suggestion 邮件发送与 admin/email/outbox 责任重叠；`IndexController.go:33-43`。

建议把 Scope 改为 **use-case/action matrix**，至少逐项标出 owner、调用方和本任务是否实现：

1. notes：create/read/list/update metadata/update content/move/copy/trash/permanent-delete/sync；
2. notebooks：tree/list/create/update/reparent/reorder/delete/sync；
3. tags：list/add/reactivate/delete-and-detach/sync；
4. history：append/list/retention；
5. suggestion：create feedback（若保留）；
6. cross-domain ports：share permission、attachment/image reconciliation、publishing projection、email/outbox；notes 拥有 note-specific asset receipt adapter，不实现 content 的通用文件/路径/PDF 规则；
7. controllers：本任务只迁移调用方到 typed use case 并保留 wire contract，完整 HTTP runtime/registry 删除 Revel 仍归 interface-http。

#### 2.2 所有权规则过度概括

“所有读写、删除、恢复和移动操作强制带用户所有权条件”（`prd.md:13`）不能直接覆盖合法共享场景。当前 `UpdateNote` 先按未限定 owner 查 note，再以 share permission 判断 actor（`NoteService.go:463-479`）；这会泄露资源存在与 owner，且规则分散。另一方面，简单地改成 `GetNote(noteId, actorID)` 会把合法 shared update 变成 not-found。

规格需要明确三个主体：

- `ActorUserID`：当前认证用户，不从请求体信任；
- `OwnerUserID`：资源的持久化 owner，只能由 owner-scoped lookup 或授权关系导出；
- `Created/UpdatedUserID`：审计主体。

还要逐 action 冻结 unauthorized 的兼容外形。读取 API 当前通常伪装为 `notExists`（`ApiNoteController.go:127-141`），shared mutation 当前可返回 `noAuth`（`NoteService.go:433-436,472-475`）。不能跨端点统一成 forbidden，否则违反 domain 的 API variant 决策。

#### 2.3 D-06 设计仍不可实施

归档 persistence 的 `TransactionRunner` 和 `RunUserInitialization` 是用户初始化专用（`app/db/initialization.go:13-64`），不是通用 note mutation unit-of-work。application-notes 当前 design 只写“优先事务/无事务补偿”（`design.md:11-13`），缺少：

- note repository 的 typed query/update/insert/delete 接口；
- 原子 AllocateUserUSN 的返回与错误；
- transaction session 如何传入 notes、note_contents、history、tags、files 等 repository；
- create/update 各自的 idempotency identity；
- transaction callback 重试时，时间、ObjectID、USN、history revision 是否复用；
- standalone 模式 compensation 的 before-image、步骤顺序、清理失败结果和 retry 判定；
- “失败不消耗 USN”只适用于事务 abort，还是也要求 standalone 补偿恢复 user counter；后者在并发下不能安全简单递减；
- `partial_write` 的内部 error category 到各既有 Web/API envelope 的逐 action 映射。

建议 design 增加 `WorkspaceUnitOfWork`/`NoteMutationRepository` 概念接口与 mutation result（Committed、PartialWrite、AppliedSteps、FailedStep、Usn、RetrySafe），但不要复用名为 `UserInitializationPlan` 的身份专用概念制造错误抽象。

#### 2.4 多写流程不只 note-save

`TrashService.DeleteTrashApi` 顺序执行附件删除、note tombstone、content 删除、history 删除和 notebook recount，并反复覆盖 `ok`；最终只反映最后一次 history 删除（`TrashService.go:98-128`）。Web `DeleteTrash` 还重算 tags（`TrashService.go:69-95`）。`AddNote` 在 note insert 前已消耗 USN，随后 tag/recount 的失败被忽略（`NoteService.go:286-326`）。`CopySharedNote` 先复制图片/附件再创建 note（`NoteService.go:804-843`）。

现有 PRD 只要求 note-save partial matrix，未覆盖上述 mutation。应按 operation matrix 将写集合、成功边界、补偿和 side-effect 分类逐一列出；不能让最后一个 bool 覆盖前面错误。至少包括：

- note create：note + content + USN 是必需原子边界；tag count/notebook count/image index/attachment reconciliation 是必需还是可重建 projection；
- permanent delete：note tombstone 必须保留供 sync，content/history/attachments 的删除顺序及失败可重试性；
- move/recover：目标 notebook ownership、note owner、blog projection 和两边 notebook count；
- tag Web delete：tag tombstone + 每个 note 移除 tag 会产生多个 USN，是否为一个业务成功边界；
- copy/shared-copy：目标 note/content 与 copied file side effects 的幂等与补偿。

#### 2.5 批量操作的错误契约冲突

Web delete/move 无视每项结果并固定返回 `true`（`NoteController.go:281-309`）；copy 固定 `Ok=true`，即使返回零值 note（`NoteController.go:312-322`）；set-blog 同样固定成功（`NoteController.go:534-538`）。PRD 同时要求“失败不伪造成功”和“HTTP 外形保持”。这不是纯技术细节。

在不改变响应 shape 的前提下可以返回整体 false/`Ok:false`，但还需决定：全有或全无、best-effort+失败列表、还是遇错停止；现有 shape 对部分成功没有表达能力。必须先获得产品决策，或明确将批量契约修复拆为单独兼容任务。

#### 2.6 notebook 与 tag 的 action 差异不能被错误统一

- Web notebook delete 会拒绝非空子树/笔记，成功时已有 tombstone 但 update query 缺 owner（`NotebookService.go:280-300`）。
- API notebook delete 明确“不作笔记控制”，当前物理删除（`NotebookService.go:303-312`）。D-01 只改变物理删除为 tombstone/new-USN/sync，不应擅自新增 Web 的非空约束。
- Web tag delete tombstone tag 后会从每个 note 移除 tag并为每个 note 分配 USN（`TagService.go:100-108`、`NoteService.go:996-1019`）。
- API tag delete 只操作 NoteTag tombstone（`TagService.go:112-126`）。D-01 不授权把 Web detach 行为复制到 API。

规格应明确 Web/API action 分别保留其业务规则，只共享底层 ownership、atomic USN 和 tombstone primitive。

#### 2.7 History 规则未闭合

当前代码宣称 `maxSize=10`，但长度为 10 时先保留 10 项再 prepend 新项，最终为 11（`NoteContentHistoryService.go:14-43`）。还没有定义：

- 是否每次内容请求都新增历史，还是仅内容值实际变化时新增；
- 保存的是新内容还是被替换的旧内容（当前保存新内容，`NoteService.go:661-666`）；
- 相同内容重试是否去重；
- newest-first 和最大条数是否属于兼容契约；
- history 写失败是否使 note-save 失败；
- transaction callback 重试是否产生重复 revision。

“History 服务测试通过”不足以决定这些行为。名义上限 10 可作为仓库意图，但从 11 修为 10 是可观察变化；父任务要求除已确认缺陷外不隐式改行为，因此仍需产品确认或单独缺陷授权。

#### 2.8 Suggestion 边界未闭合

Suggestion service 只做 insert（`SuggestionService.go:15-20`），controller 无条件另起 goroutine 发邮件且不观察结果（`IndexController.go:33-43`）。它不具备 read/update/delete/ownership workspace 语义，guest 的 `UserId` 是否允许为 zero 也未定义。归档 persistence 已明确请求路径禁止无结果 goroutine、持久化 outbox 至少一次投递（persistence `prd.md:17-19`），但该 outbox 当前面向注册/找回邮件，未证明 suggestion 接入。

规格必须选择：

- 将 Suggestion 排除并交给 application-admin/email 子任务；或
- 本任务只拥有 feedback 持久化，email 通过明确 port/outbox 交给 admin；并定义匿名策略、Addr/Suggestion 长度/空值/HTML 处理、重复提交和邮件失败是否影响响应。

#### 2.9 输入与数据约束缺少 action matrix

当前输入 presence 由 controller 的 `Has` 决定，空字符串与缺失字段语义不同（`NoteController.go:217-264`；`ApiNoteController.go:428-484`）。API add 只显式校验 NotebookId（`ApiNoteController.go:216-240`）；domain 审计已把空 Title/Content requiredness 留为 unknown。当前规格未覆盖：

- ObjectID invalid/zero/missing；
- `afterUsn < 0`、`maxEntry < 0/0/过大`、page/pageSize 边界；
- Title/Content/Tag 为空、空白、长度和 Unicode；
- tag 大小写、trim、重复、逗号拆分 vs API array；
- notebook parent 必须同 owner、禁止 self-parent/cycle、deleted parent；
- moving note 到 deleted/foreign notebook；
- client CreatedTime/UpdatedTime 的零值、未来值与时区；
- duplicate NoteId 和 unknown write result 的 retry；
- HTML/Markdown、链接、图片、代码和 plugin marker 的允许规范化清单。

应引用 domain 的 input-contract/model catalog 并逐 action记录 observed/target/unknown，不能使用一个全局默认推导所有入口。

#### 2.10 验收不可复核

当前 `implement.md:9` 的 `go test ./app/service ./app/controllers/api ./app/tests/harness` 有三处问题：

1. `app/service` 目前几乎没有 notes workspace 单元测试；现有相关证据主要在 Mongo-backed harness。
2. `app/controllers/api` 的 first-party 测试只覆盖 tag lifecycle 且删除后未断言 tombstone/sync（`httpserver_test.go:236-285`）。
3. harness 的 USN test 仍把 notebook/tag 旧缺陷当作通过标准（`usn_test.go:57-84`），必须先拆分 known-defect before 与 D-01 target。

建议新增 acceptance evidence matrix，至少包含 AC-N1～AC-N12：scope/action inventory、dependency/static scan、ownership/private+shared、atomic USN concurrency、note create/update transaction、standalone compensation/idempotency、D-01 notebook delete、D-01 tag delete、trash/move/copy multi-write、history、suggestion（若保留）、real HTTP+Mongo7/8 Golden。每项记录命令、fixture、发现/执行/通过/跳过数量、commit、Mongo 版本/拓扑、退出码和状态（unrun/partial/pass/blocked）。

### 3. 建议直接补入 PRD 的需求结构

以下内容可由仓库事实直接补充，无需再次询问：

1. **Compatibility rule**：逐 action 保留现有请求方法、参数 presence、成功/失败 JSON variant、HTTP status/Content-Type、排序分页和 HTML 语义；仅 D-01/D-06 以及另行批准的 defect 可改变行为。
2. **Actor/owner rule**：认证 actor 来自 session/token；私有 query 带 actor owner；共享 query 明确 owner+actor+permission，最终资源 mutation 带 resource ID+owner；unauthorized 不泄露 owner/资源存在。
3. **USN rule**：同用户原子、唯一、严格递增；成功 mutation 的返回 USN、文档 USN、sync entry USN 相同；失败/冲突不生成可观察 mutation；不同用户互不影响。
4. **D-01 rule**：API notebook/tag delete 使用 new-USN tombstone 并进入 sync，保留原 action 的非空约束差异和 envelope。
5. **Typed result rule**：application command 返回稳定 result/error category；controller 仅映射到该 action 既有 envelope，不持有 BSON、collection 或重复 USN/权限规则。
6. **Query/error rule**：DB/uninitialized/timeout/decoder 错误不得变为空集合或成功；真正的 empty collection 才返回既有 `[]`/`null` 形状。
7. **No-change rule**：未编辑 note 不发 mutation；metadata-only/content-only/combined update 按 presence 区分；成功后前端才确认 revision/cache。
8. **Evidence rule**：unit contract、repository fake、真实 HTTP+Mongo、Golden/USN 分层记录，不用 mock 替代 Mongo7/8 或真实 HTTP。

### 4. 建议直接补入 design/implement 的技术与执行门

- 先产出 action inventory：入口 → command/query → owner/actor → repository 方法 → 写集合 → USN → result/error → wire mapper → tests。
- 设计领域中立 repository ports：纯 `app/application/notes` 禁止 Mongo/Revel/`app/db`/BSON，controller 禁止 collection/client；保留既有 `app/service → app/db` 兼容适配。
- 单一 `AllocateUserUSN` primitive 被 note/notebook/tag 所有 mutation 消费，删除 controller/service 内的第二套读写计数逻辑。
- 为 note create/update 分别定义 transaction plan 和 standalone compensation plan；transaction callback 可重试的数据（ID/time/history record）必须在 callback 外稳定生成或以幂等条件写入。
- 把 tags/notebook counts/image index 等派生 projection 标为 required-in-commit 或 repairable-after-commit；每项只能有一种恢复策略。
- 在更改实现前先将旧 notebook/tag delete 断言改成明确 known-defect ledger，再新增 D-01 target test；避免“更新 Golden 让测试绿”。
- 每个 use case 先写 service contract tests，再迁移 Web/API adapter；本 leaf 跑 Mongo 8 standalone HTTP，Mongo 7/replica-set、kill/restart/failpoint 由 delivery 接收。
- 静态门至少检查目标 application 包无 Revel/Mongo driver，controller 无 collection/query/BSON mutation，所有 resource mutation repository 调用含 owner 条件或显式 public/shared policy。

## 初始候选产品/技术决策清单

下表是研究扫描阶段列出的 8 个候选决策。D-01/D-06 的大方向不在此列，它们已经确认。主流程随后按“仓库可回答的问题不询问用户”继续收敛；这是审计时点的决策清单，最终关闭结果见后文“2026-09-11 决策关闭记录”和 PRD 的 KD-N1～KD-N5。

| ID | 待确认事项 | 推荐方案 | 不确认的影响 |
|---|---|---|---|
| Q-N1 | 本 leaf 是否拥有 `NoteService` 中 publishing、attachment/image 和 PDF 相关方法，以及完整 controller runtime 迁移？ | 按 use case 切分：本任务只拥有 workspace command/query 及跨域 ports；publishing/content/interface-http 继续由各自 leaf 实现 | 会出现跨任务重复修改和两个事实来源，无法定义本任务完成边界 |
| Q-N2 | Web 批量 delete/move/copy/set-blog 遇到部分失败时返回什么？ | 单独确认兼容修复：service 返回逐项结果；若必须保留现有 bool/`Re` shape，则整体 `Ok=false` 且不承诺原子，详细失败仅内部日志/指标；如需失败列表必须版本化/另建任务 | 当前实现伪造成功；直接改 shape 又破坏兼容 |
| Q-N3 | standalone Mongo 下 note-save 的 USN 在补偿成功后是否允许出现无 sync entry 的 gap？ | 允许 gap、禁止回拨 counter；业务返回失败并保证无 note/content 可见 partial，重试以稳定 operation identity 完成。若产品要求无 gap，则 standalone 无法满足，应把 note-save transaction 设为部署前置条件 | 并发下回拨 USN 不安全；不决定则无法实现 D-06 的“失败不消耗”在 standalone 的精确定义 |
| Q-N4 | API/Web create/update 的幂等 identity 是什么？ | update 用 `(owner,noteId,expectedUsn)`；Web create 使用客户端稳定 NoteId；API create 若不提供稳定 client key，则新增可选 idempotency key 需要兼容批准，否则只能保证 unknown-result 查询/去重的有限语义 | 网络 unknown-result 重试可能重复创建、重复 history/USN，D-06 验收无法判定 |
| Q-N5 | history 正式规则：保存旧版还是新版、何时创建、上限是否严格 10？ | 仅在内容实际变化且主 mutation commit 时追加被替换的旧内容，newest-first，严格 10，operation retry 不重复；这是行为变化，需批准 | 当前实现保存新内容且可能 11 条；无法写确定测试或事务边界 |
| Q-N6 | “恢复”是显式产品 action，还是继续由 MoveNote 从 trash 隐式完成？ | 保持现有可观察行为：move-from-trash 即恢复，不新增 URL/API；把未调用的 `recoverNote` 视为 dead candidate | PRD 当前承诺恢复测试，但仓库没有实际入口 |
| Q-N7 | Suggestion 是否属于本任务；是否允许匿名，邮件失败是否影响提交？ | 从 notes leaf 移交 application-admin；若保留，本任务只写 feedback，允许/拒绝匿名需明确，邮件通过 outbox 异步且 feedback 成功不由 transport 成败回滚 | 会重复 email/outbox 规则并继续无结果 goroutine；输入与所有权无法验收 |
| Q-N8 | Web tag delete 的“tag tombstone + 所有 notes detach”是否需要一个原子业务成功边界？ | 定义为可恢复的 multi-step command：tag tombstone 和每个 note mutation 都有各自 USN；任一步失败返回 partial_write 并可按 operation id 续跑，不尝试把无限 note 集合塞进单事务 | 大标签可能超事务限制；现有 map 结果与 Web envelope 无法表达失败/重试 |

## 主流程收敛结论

主流程复核父任务与现状后，以下三项不再作为用户问题：

- 原 Q-N1：父任务已经把 workspace、content、publishing、interface 分给独立 leaf；本任务按 use case/port 边界执行。
- 原 Q-N6：现有公开入口只有 move-from-trash，按兼容和 YAGNI 不新增 restore URL/API；未调用 helper 作为 dead-code candidate。
- 原 Q-N8：无限 tag detach 不适合单事务；按已确认的“失败不可伪造成功”与 Mongo 边界采用 durable multi-step/resume，公开 partial 映射并入批量行为决策。

在该审计时点仍需用户确认的 5 项曾在 PRD 编号为：Q-N1 批量/partial 公开映射、Q-N2 standalone USN gap、Q-N3 API create HTTP 幂等 identity、Q-N4 history 语义、Q-N5 Suggestion 归属/邮件语义；其后续关闭结果如下。

### 2026-09-11 决策关闭记录

用户确认上述 5 项全部采用推荐方案。PRD convergence pass 已将其改写为正式 KD-N1～KD-N5：批量部分失败保持既有 shape 并整体失败；standalone 允许 USN gap 且禁止回拨；API create 接受可选 client `NoteId`，未提供时不承诺盲重试安全；history 仅保存实际内容变化前的旧版、newest-first 严格 10 条且重试不重复；Suggestion/feedback 与邮件 outbox 移交 `application-admin`。这些事项不再是 blocking open questions，但仍须通过对应实现期证据验收。

## 建议的编码前关闭条件

只有满足以下条件后才适合 `task.py start`：

1. PRD 原 Q-N1～Q-N5 已关闭并固化为 KD-N1～KD-N5；移出范围的 Suggestion 已明确链接 `application-admin` 责任任务。
2. PRD 有 action/use-case、actor/owner、输入/output/error、compatibility 和 multi-write 成功边界矩阵。
3. design 明确 atomic USN repository、note transaction unit-of-work、standalone compensation/idempotency，以及跨 publishing/content/share/email 的 ports。
4. implement 以测试先行的可回滚阶段列出具体文件/行为/验证，不再以“某服务测试通过”代替覆盖矩阵。
5. acceptance/evidence-matrix.md 已建立且初始状态诚实标为 unrun/partial；旧 Golden 与 D-01 target 分离。
6. 明确 persistence 交接的可用 seam 与新增 seam 归属；已归档 persistence 的 partial/protected-runner 缺口不得自动升级为 application 通过。

## External references

未使用外部资料。本任务的兼容与产品行为事实来源是仓库代码、Golden/harness、Trellis spec 和已确认的 D-01/D-06；引入 Mongo 官方文档不会替代项目对 standalone 补偿、API envelope 和产品范围的决策。

## Caveats / Not Found

- 按要求执行了一次 `jbcontext search`，服务返回 HTTP 404 且无结果；随即停止语义重试并改用针对性 `rg` 与直接文件读取。
- 未读取 `implement.jsonl` / `check.jsonl`，遵守 trellis-research 角色隔离；因此本报告不评价 manifest 是否已完成上下文配置。
- 当前任务目录未找到 acceptance 材料；本报告只能提出证据矩阵结构，不能把尚未运行的测试标为通过。
- 未运行 Go/Node/Mongo/HTTP 测试；所有“当前行为”结论来自静态实现、现有测试断言和归档证据。Mongo 7/8、并发、failpoint、真实 HTTP 和浏览器证据仍为未运行/下游 partial，不能由本审计替代。
- `app/service` 与 controller 仍处在混合旧/新迁移状态；文件级范围会随其他 leaf 并行变化，所以正式实现前必须以 action/use-case matrix 而非文件清单锁定责任。
