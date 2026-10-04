# 应用层：笔记工作区与同步 — PRD

## Goal

把笔记、笔记本、标签、回收站、内容历史和增量同步收敛为可测试的 Web 服务端用例，使每次可见 mutation 都具有明确的 actor/owner、原子 USN、持久化成功边界和稳定错误结果，同时保持已发布 Web/API 客户端的逐 action wire contract。除上游已确认的 D-01、D-06 及本任务已确认的兼容性修复外，不改变公开行为；浏览器、Android、PDF、完整 HTTP runtime 和跨 Mongo 拓扑交付验证由已有 sibling 任务承接，不形成反向依赖。

## Background and confirmed baseline

- `09-08-domain-contracts` 已归档并冻结：用户所有权查询、API variant、USN 同步、D-01 notebook/tag 删除 tombstone，以及 D-06 原子 USN 和 note-save 事务/显式补偿。
- `09-08-infrastructure-persistence` 已归档并提供 Mongo driver、timeout、错误分类、通用 transaction runner 形状和 focused fixture；其现有初始化 plan 面向身份注册，不是可直接复用的 note unit-of-work。该任务遗留的跨版本/拓扑和受保护 runner 证据由 `delivery-verification` 汇总，本任务只要求当前支持的 Mongo 8 standalone + 真实 HTTP 证明 notes 契约。
- 当前 `UserService.IncrUsn` 是先读后写；API notebook 删除物理删除且不产生新 USN；API tag 删除返回新 USN 却持久化旧 USN；这些是待 D-01/D-06 修复的已知缺陷，不是兼容通过标准。
- 当前 Web note-save 使用 `info.Re`，API create/update/delete 使用各自 direct model、`ReUpdate` 或错误 variant。HTTP 200 不代表业务成功，不得把这些 variant 合并成新 envelope。
- 当前 Web 未编辑笔记由前端避免发出保存；成功响应后才确认 revision/cache。服务端仍必须保证空操作、重试和失败不会伪造成功。

## Actors and authorization model

- `ActorUserID` 来自可信 session/token principal，不接受请求体覆盖。
- `OwnerUserID` 是资源持久化 owner。私有用例必须在同一 repository query 中同时带资源条件和 owner 条件。
- 共享用例显式携带 actor、owner 和 permission；owner 只能由 owner-scoped lookup 或已验证的分享关系导出，最终 mutation 仍以 `(resource ID, owner ID)` 限定。
- `CreatedUserID` / `UpdatedUserID` 记录真实 actor，不能替代 owner 查询。
- 未授权外形逐 action 保持现状：读取通常不得泄露资源存在；已有 shared mutation 的 `noAuth` 不能被全局重写成新的 forbidden variant。

## Scope and use-case ownership

### In scope

| 子域 | 本任务拥有的用例 | 必须保持的边界 |
| --- | --- | --- |
| note query | 私有/获授权共享的 get、list、search、sync | owner 条件、现有空集合/排序/分页/direct DTO 形状 |
| note mutation | create、metadata-only update、content-only update、combined update、move/from-trash、copy、trash、permanent delete | typed command/result、USN、幂等、多写成功边界 |
| notebook | tree/list/create/update/reparent/reorder、Web guarded delete、API delete、sync | parent 同 owner、无环、D-01、Web/API 删除差异 |
| tag | list/add/reactivate、Web delete-and-detach、API delete、sync | 标准 tag identity、计数、D-01、Web/API side-effect 差异 |
| history | 随成功 content mutation append、现有 Web list、retention | 内容实际变化时保存被替换的旧版，newest-first、严格 10 条、重试不重复 |
| adapter handoff | 让现有 Web/API controller 调用 typed use case 并按原 action 映射结果 | 不在本任务迁移完整 HTTP runtime/route registry |

### Cross-domain ownership and ports

- 分享权限由 `application-publishing` 拥有；本任务只消费 read/update permission port，不复制分享规则。
- 本任务拥有 note-specific asset idempotency/receipt 适配：把 note owner、source/destination note、operation generation、input digest 和 mutation 成功边界绑定到 durable step。`application-content` 拥有通用文件根、路径安全、原子/no-clobber 发布与校验、下载、相册和 PDF primitive，并负责验收这些 primitive 能被 notes adapter 按 Apply/Verify 合同消费。两个任务都不复制对方的事实来源。
- blog/public projection、blog tag recount 和 `IsBlog` 传播由 `application-publishing` 拥有；本任务只调用 port 并保持现有 wire 字段。
- binder、route method、HTTP status/Content-Type、session writer 和 Revel 删除由 `interface-http` 拥有；本任务只提供 application result mapper 所需的稳定 category。
- Suggestion/feedback 及邮件 outbox 归 `application-admin`；本任务只保留交接约束，不实现相关业务。

## Functional requirements

### R-N1 — Per-action compatibility

每个入口必须在 action inventory 中记录 method/path、参数来源与 presence、认证、command/query、成功 body、失败 body、HTTP status/Content-Type、排序/分页、当前 Golden 和目标 fixture。缺少证据时标为 `unknown`，不能用统一默认推导。D-01/D-06 只改变其明确覆盖的持久化与同步语义。

### R-N2 — Typed application boundary

controller 负责输入绑定/presence、principal 与逐 action response mapping；权限、USN、冲突、mutation 顺序和业务校验只有一个事实来源。纯 `app/application/notes` 契约与状态机不得导入 Revel、Mongo driver、`app/db` 或 BSON，也不得把 `bson.M` 当作公开 command。依照仓库既有分层，`app/service` 可以通过 `app/db` 适配 Mongo，controller 可以在兼容边界解析 ObjectID 或组装旧 update document，但 notes mutation controller 不得直接操作 collection/client，也不得另写 USN、权限或 receipt 状态机。

### R-N3 — Input and data constraints

- required ObjectID 必须严格解析；missing、blank、invalid、not-found 分开映射，不允许 panic 或 zero-ID fallback。
- notebook parent/target 必须属于同一 owner、未删除、不能 self-parent 或形成 cycle；move/copy 的目标 notebook 同样校验。
- update 必须区分字段 absent 与显式空值；metadata-only、content-only 和 combined update 不能互相覆盖未提交字段。
- API note update 的文件集合必须在服务端区分 presence：缺少 `Files`/`FilesPresent`/`HasFiles` 表示保持当前图片和附件关系；显式 `FilesPresent=1` 或兼容别名 `HasFiles=1` 且集合为空表示删除全部关系；非空 `Files[...]` 表示完整集合替换。判定必须收敛在一个 adapter 边界，不能以伪造空 file 条目表达清空。
- notebook reparent/reorder 必须先解析完整 JSON 并验证所有 notebook 同 owner、未删除、无重复 ID、无 self/cycle；同一 notebook 的 parent/seq 合并为一次 conditional write 和一个 USN。多 notebook 操作不承诺全批原子，失败返回现有 boolean false，并以稳定 operation identity 续跑而不重复已提交项。
- sync 使用 `Usn > afterUsn`、`Usn` 升序和现有 `maxEntry` 兼容边界；负数、零、超大值及默认值按 action fixture 冻结。
- tag 的 trim、大小写、重复、Web 逗号字符串和 API 数组规则按现状分别冻结；不得跨入口隐式统一。
- client time、Title/Content requiredness、搜索字符串 regex/literal 语义和 HTML/Markdown 规范化中尚无 Golden 的部分，必须先捕获 named `baseline-before` 再迁移；在取得证据前保持当前输入与输出，不静默扩张校验。
- Web `UpdateNoteOrContent`、`CopyNote`、`CopySharedNote`、`DeleteNote`、`MoveNote` 接受可选 `OperationId`，其中 update 另接受可选 `ExpectedUsn`。提供时按 owner + action 隔离并绑定 canonical input digest；同一 operation ID 搭配不同输入返回 conflict。省略字段时保持旧请求/响应外形，并明确 `RetrySafe=false`，不使用永久 payload hash 代替客户端 identity；批量请求为每个 item 冻结子 operation identity 与顺序。

### R-N4 — Atomic user USN

- 使用单一 `AllocateUserUSN` repository primitive，以 `$inc + return-after` 或有界 CAS+重试原子分配。
- 同一用户的成功 mutation USN 唯一、严格递增且 sync 可见；不同用户互不影响。
- 成功结果中的 USN、持久化文档 USN 和对应 sync entry USN 必须一致。
- not-found、unauthorized、validation failure 和 conflict 在业务写入前结束，不产生资源 mutation；事务 abort 不消耗 USN。
- standalone 在已经分配 USN 后发生失败时允许留下无 sync entry 的 gap，但禁止回拨 counter；命令返回失败/`partial_write`，并以稳定 operation identity 安全续跑，不暴露 note/content 半成品。

### R-N5 — D-01 notebook/tag delete

- API notebook delete 保留现有“不检查非空 notebook”的 action 规则，但改为 owner-scoped `IsDeleted=true` tombstone，分配并持久化新 USN，`getSyncNotebooks(after oldUsn)` 返回该 tombstone；不级联删除/移动 notes。
- Web notebook delete继续拒绝活跃子 notebook 或非 trash note；成功使用同一个 owner-scoped tombstone primitive。
- API tag delete 只 tombstone `NoteTag`，返回/持久化同一个新 USN，并由 `getSyncTags` 返回；不得擅自复制 Web detach 行为。
- Web tag delete 保留“tag tombstone + 从相关 notes 移除 tag”的行为，并使用可续跑 multi-step command；任一步失败时保持现有 bool/`Re` shape 并返回整体 false/`Ok:false`，不承诺全批原子，也不新增失败列表字段。
- 旧 no-bump/physical-delete/stale-USN Golden 只保留为 named known-defect before evidence，不能更新成新的通过基线。

### R-N6 — Note create/update success boundary (D-06)

- create 的必需边界至少包含 note metadata、note content 和一次 user USN；Web create 沿用客户端稳定 `NoteId`，API create 接受可选客户端 `NoteId` 作为幂等 identity。旧客户端未提供时继续由服务端生成，并明确不支持 HTTP unknown-result 后的盲重试。
- combined update 的必需边界至少包含 expected USN conflict check、metadata/content update 和一次 user USN；content-only 与 metadata-only 各只产生一次可见 mutation。
- transaction 模式把必需写入同一 commit；callback retry 复用稳定 operation ID、note ID、time、history revision identity 和业务输入，不得重复 USN/history。
- transaction 失败后不产生可见 note/content mutation、不消耗 USN、不返回成功。
- standalone 模式必须显式返回 `partial_write`/retry state，不能把 compensation 当成功；允许 USN gap、禁止 counter 回拨，并以稳定 operation identity 查询、补偿或续跑。API create 未提供 `NoteId` 时不得声称盲重试安全。
- KD-N6 提供的 `OperationId`/`ExpectedUsn` 只扩展可选输入，不改变既有 route method、status、Content-Type 或 response body shape；相同 operation ID、相同 canonical input 的 unknown-result 重试返回原提交结果，缺少 operation generation 的旧客户端仍不承诺 retry-safe 或 stale-write 检测。

### R-N7 — No-change, HTML and history

- 未编辑的存量笔记不发送 mutation；验收同时断言 metadata、content、history、USN 均不变。
- 实际编辑只允许 ADR-0003 已登记的非语义 HTML 规范化，文本、链接、图片、代码块、表格和第一方插件标记语义等价。
- history 仅在内容值实际变化且主 mutation commit 时保存被替换的旧内容；history owner 是 note owner，`UpdatedUserID` 是 actor。读取保持现有 Web action，禁止顺手新增公开 API history action。
- history 按 newest-first 严格保留 10 条，并使用稳定 revision/operation identity 去重；主 mutation 失败、相同内容请求或 transaction/unknown-result retry 不能产生孤立或重复 history。

### R-N8 — Trash, move, copy and projections

- trash/permanent-delete、move/from-trash、copy/shared-copy 必须分别列出必需写、可重建 projection、外部 side effect、补偿顺序和 retry identity。
- permanent delete 必须先保证 note tombstone/new USN 可供 sync；content/history/attachment/image 清理不得用最后一个 bool 覆盖此前失败。
- move/from-trash 校验 note owner/actor permission 和 target notebook owner/status，并维护源/目标 notebook count；按既有行为将其作为唯一恢复入口，不新增独立 restore URL/API，未调用的 `recoverNote` 作为 dead-code candidate。
- copy/shared-copy 的 note/content/asset destination identity 在提供 `OperationId` 时首次执行即冻结到 receipt；shared-copy 根 receipt 还须冻结有序的源图片/附件 identity manifest，重试使用已提交 destination content snapshot 与该 manifest，不得重新枚举后来新增的源资产。文件/图片/附件复制经 content port 完成，unknown result 必须先 Verify 或按明确 replay-safe 合同续跑，不能重复资源。第一方 Web 字段的生成和跨 unknown-result 重试复用交接 `presentation-frontend`。
- notebook/tag counts、image index、blog projection 等每项只能被标为 required-in-commit 或 repairable-after-commit，并具有唯一修复入口；不得以无结果 goroutine伪装完成。

### R-N9 — Batch mutations

批量 delete/move/copy 的 application result 必须记录逐项 success/error 和是否出现 partial write；adapter 保持现有 bool/`Re` shape，任一失败时整体返回 false/`Ok:false`，不承诺全批原子。请求提供 `OperationId` 时，服务端按输入顺序冻结每个 item 的子 operation identity，重试不得重复已提交项；省略时保持旧行为且不标为 `RetrySafe`。已有 `Item` 字段的 action 保留已成功项；其他逐项失败只进入内部可观测信息，不新增公开失败列表字段。批量 set-blog 归 `application-publishing`，本任务只交接同类缺陷证据。

### R-N10 — Errors and observability

- validation、not-found、unauthorized、conflict、duplicate、storage、timeout、partial_write、side_effect 分开；DB/uninitialized/decoder 错误不能变成空集合、零值模型或成功。
- service 返回 typed category 和安全上下文；adapter 按 action 映射既有 `Re`、`ReUpdate`、direct model/array 或 boolean 外形。
- 错误只在终止边界记录一次，不包含 note content、token、凭据或用户私密字段。
- 空集合只在真实查询成功且无结果时返回现有 `[]`/`null` 形状。

### R-N11 — Suggestion/feedback handoff

Suggestion/feedback 从 notes workspace 移交 `application-admin`，本任务不修改其持久化、匿名策略、输入校验或公开响应。后续 admin 规格必须定义 anonymous policy、Addr/Suggestion 长度/空值/HTML、重复提交和 durable success，并通过 outbox 投递邮件；feedback 已持久化的成功不得因邮件 transport 失败而回滚，且禁止请求路径无结果 goroutine。

### R-N12 — Evidence integrity

service/repository contract、真实 HTTP、Golden/USN 和扩展交付证据分层记录。当前 leaf 的完成证据是当前支持的 Mongo 8 standalone 上的真实 Web/API HTTP replay、focused application/service/repository tests、D-01/D-06 target 和 files-presence 三态持久化回归。Mongo 7 standalone、Mongo 8 replica-set、进程 kill/restart、跨主机文件系统 failpoint、浏览器矩阵和 PDF artifact 继续由 `infrastructure-persistence`、`presentation-frontend`、`application-content` 与 `delivery-verification` 验收；它们的 `partial`/`blocked` 不反向阻塞本 leaf。每次运行仍记录 discovery/execution/pass/fail/skip、candidate、Mongo version/topology、命令、退出码和脱敏失败原因。

38-action inventory 是范围清单，不是动态覆盖证据。当前 harness 可核对到 17 个 API action 和 4 个 Web action，共 21/38；`discovered 105 / passed 103` 是 Go test event 统计，不能替代逐 action 映射。剩余 17 个 Web action 的 live HTTP/Golden 矩阵交给 `delivery-verification` 从本 inventory 消费并验收；本 leaf 的 AC-N1 以“清单完整且本任务改变的 mutation 有 focused contract/target fixture”为通过边界，delivery 的 38-action 动态覆盖在其完成前保持 `delegated-unrun`，两者不得互相冒充。

本任务的发布门禁只要求 Leanote Web/服务端接口可用且功能契约完整。Android 客户端是 sibling 重构项目；其 marker 生成、设备、APK、签名和端到端调用证据延后到 Android 重构时验证，不阻塞本 Web 项目验收。本项目仍必须用服务端 contract/HTTP 证据证明“字段缺失保留”、“显式空集合清空”和“非空集合替换”三种行为。

## Acceptance criteria

- [x] AC-N1：action inventory 已覆盖 38 个 in-scope Web/API callable action，并明确 actor/owner、input presence、result/error、wire mapper 和跨域 port；`research/action-inventory.md` §10 将 WN-08/WN-09/WN-11/WN-12/WN-13 映射到具名 focused contract/fixture。当前 live harness 可追溯覆盖 21/38（17 API + 4 Web）；剩余 17 个 Web action 的逐项 HTTP/Golden 映射已被 `delivery-verification` 接收并保持 `delegated-unrun`，不得用 focused test 或 harness 总事件数替代，也不反向阻塞本 leaf。
- [x] AC-N2：纯 `app/application/notes` 对 Revel/Mongo driver/`app/db`/BSON 无生产依赖；notes controller 不直接持有 collection/client，也不复制 USN、权限或 durable-operation 状态机。`app/service → app/db` 和兼容 adapter 的 ObjectID/BSON 转换按仓库既有边界保留。
- [x] AC-N3（leaf server subset）：owner-scoped service/DB contract 已通过；当前真实 HTTP 负向证据明确限于 foreign API note read 与 unauthorized shared create。其余 private/shared read/write 的逐 action negative matrix 由 `delivery-verification` 保持 `delegated-unrun`；成功 list/share Golden 不得被记作跨用户拒绝证据。
- [x] AC-N4：并发 USN fixture 证明同用户唯一、严格递增、无重复且不回拨；durable receipt tests 证明 owner 隔离；D-01 HTTP/DB/sync 断言证明成功 response/document/sync 的 USN 一致。扩展拓扑矩阵交给 delivery。
- [x] AC-N5：note create、metadata-only/content-only/combined update 的 transaction/standalone、conflict、retry 和 failure-result contract 满足 D-06；focused runner/DB tests 与真实 note-save HTTP replay 均通过。进程级故障注入交给 delivery。
- [x] AC-N6：standalone compensation/idempotency 允许 USN gap、禁止回拨；失败明确为 `partial_write`，稳定 operation identity 可续跑且不暴露 note/content 半成品。API create 仅在提供客户端 `NoteId` 时承诺幂等重试；Web mutation 提供 KD-N6 字段时按 receipt 安全续跑，省略时明确 `RetrySafe=false`。
- [x] AC-N7：D-01 notebook Web/API delete 使用新 USN tombstone 并进入 sync，保留各自非空约束与 envelope。
- [x] AC-N8：D-01 tag Web/API delete 返回/持久化同一新 USN 并进入 sync；Web detach 可续跑，任一失败保持既有 shape 并整体返回 false/`Ok:false`。
- [x] AC-N9（leaf server contract）：trash/permanent-delete、move/from-trash、copy/shared-copy、notebook reparent/reorder 和批量 mutation 不以固定成功吞掉多写失败；有 KD-N6 generation 时冻结 destination/子 operation/result，shared-copy 还冻结首次 source image/attachment manifest 并在重试排除后来新增资产，且 copy/shared-copy 在主 create receipt committed 后仍完成独立 projection repair receipt。无 generation 的 legacy shared-copy 保持先复制资产、再单次 create/USN；旧客户端不伪造 retry-safe。具名 focused contract 见 action inventory §10；未映射 Web action 的真实 HTTP mapper、provider 与跨进程故障证据由 content/publishing/delivery 接续。
- [x] AC-N10：history 仅在内容实际变化且主 mutation commit 时保存被替换旧版，newest-first、严格 10 条且重试不重复；Suggestion 已形成 `application-admin` 交接，不进入本任务实现。
- [x] AC-N11（leaf mapped subset）：当前可审计的 21/38 action 真实 HTTP Golden 保持 status/Content-Type/body key/order/nil-empty/message；D-01/D-06 变化使用独立 target fixture；API update 的 absent、present-empty、present-values 文件三态服务端持久化回归通过。剩余 17 个 Web action 的逐 action wire matrix 明确为 `delivery-verification` 的 `delegated-unrun`，本项不得解释为 38/38 动态通过。
- [x] AC-N12：固定摘要 Mongo 8 standalone 上的真实 Web/API HTTP、USN、D-01、files-presence，以及明确限于 foreign API note read 和 unauthorized shared create 的负向 ownership 证据可复核；其余 ownership action-level negative matrix 保持 `delivery-verification` 的 `delegated-unrun`。本 leaf 不拥有 Mongo 7/replica-set/kill/failpoint、浏览器、PDF 或 Android 证据，这些由已有 sibling 任务验收。

详细状态与证据入口见 `acceptance/evidence-matrix.md`。

## Confirmed product and compatibility decisions

以下决定截至 2026-09-12 已获用户确认，是后续实现和验收的正式约束，不得以兼容 fallback 或实现便利改写。

| ID | 已确认事项 | 正式约束 | 排除的替代方案 |
| --- | --- | --- | --- |
| KD-N1 | Web 批量 delete/move/copy 以及 Web tag detach 部分失败的公开结果 | service 返回逐项结果；保持现有 bool/`Re` shape，任一失败整体 false/`Ok:false`，不承诺全批原子；已有 `Item` 的 action 保留已成功项，其他详细项只做内部可观测。失败列表需要版本化后另行设计 | 固定成功吞错；未版本化直接增加公开字段 |
| KD-N2 | standalone 补偿成功后允许 USN gap | 允许 gap、禁止回拨；无可见 note/content partial，返回失败并用 operation ID 安全续跑 | 并发回拨 counter；把零 gap 伪装成 standalone 可保证能力 |
| KD-N3 | API create 的 HTTP unknown-result 幂等 identity | update 使用 `(owner,noteId,expectedUsn)`；Web create 沿用稳定 client `NoteId`；API create 接受可选 client `NoteId`，旧客户端未提供时维持服务端生成且不承诺盲重试安全。必填 idempotency key 只能版本化引入 | 忽略已有 `NoteId`；对旧客户端新增必填字段；无 identity 仍宣称幂等 |
| KD-N4 | history 保存旧版、创建条件和上限 | 仅内容实际变化且主 mutation commit 时保存被替换的旧内容，newest-first、严格 10 条、operation retry 不重复 | 固化“保存新版且可达 11 条”的现有缺陷 |
| KD-N5 | Suggestion/feedback 归属和邮件语义 | 移交 `application-admin`，notes leaf 不实现；admin 后续定义匿名/校验并使用 outbox，feedback durable success 不因邮件 transport 失败回滚 | 在 notes 中复制 email/admin 规则；继续使用无结果 goroutine |
| KD-N6（原 Q-N9） | Web mutation 的可选客户端 operation generation 与更新代际检查 | `UpdateNoteOrContent`、`CopyNote`、`CopySharedNote`、`DeleteNote`、`MoveNote` 增加可选 `OperationId`；update 另增加可选 `ExpectedUsn`。字段存在时按 owner + action 隔离、绑定 canonical input digest；同 ID 不同输入返回 conflict；相同 ID/相同输入返回原提交结果；copy destination 与 batch item 的 note/content/asset identity、子 operation identity 和顺序首次执行即冻结。字段省略时保持旧 wire contract，application result 为 `RetrySafe=false`，不承诺 unknown-result 重试或 stale-write 检测。第一方 Web 负责生成与跨 unknown-result 重试复用字段，归 `presentation-frontend`；新增字段不改变 route method、status、Content-Type 或 response body shape | 永久 payload hash 会把用户主动重复操作误判为重试；服务端每次生成新 ID 无法闭合 unknown-result。若不引入可选字段，只能保持相关 retry/stale-write 验收未完成，或另行版本化接口；禁止隐藏 fallback |

## Out of scope

- 不迁移完整 HTTP runtime/registry、删除 Revel、改变 route method 或新增公开 history/restore API。
- 不实现通用 attachment/image/upload/PDF、分享/博客/主题、邮件/admin 领域规则；notes 仅拥有 note-specific asset receipt 适配并消费显式 content primitive。
- 不重写编辑器 UI、批量改写历史 HTML、改变同步游标协议、数据库 collection/BSON key 或引入新协作协议。
- 不以本任务测试替代 sibling 的 Mongo 跨拓扑、浏览器、PDF 或 delivery artifact；这些证据在各自任务保持 `unrun`/`blocked`，但不形成对本 leaf 的循环完成门禁。
