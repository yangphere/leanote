# 应用层：笔记工作区与同步 — 技术设计

## 1. Design objective

本任务按 use case 而不是旧文件边界收敛 notes workspace。纯 `app/application/notes` 拥有 durable command/result、actor/owner、USN 与状态机契约；`app/service` 依照仓库既有结构编排业务并通过 `app/db` 适配 Mongo；controller 只做输入 presence、principal、兼容 DTO 转换和逐 action wire mapping。任何共享、内容媒体、发布或 HTTP runtime 规则都通过显式 port/交接连接，不在本任务复制。notes 仍拥有 note-specific asset operation 的 idempotency/receipt adapter；通用文件根、安全路径、发布/校验、下载、相册和 PDF primitive 由 `application-content` 拥有。

## 2. Boundary map

```text
Web/API adapter
  request presence + authenticated ActorUserID
                    │
                    ▼
Workspace use case ─────► SharePermissionPort (publishing)
  command/query            ContentAssetPort (content)
  owner/actor              PublishingProjectionPort (publishing)
  conflict/USN/result
                    │
                    ▼
WorkspaceRepository / WorkspaceUnitOfWork
  owner-scoped typed operations
                    │
                    ▼
app/db Mongo adapter
  BSON + timeout + transaction/session + error mapping
```

生产 `app/application/notes` package 不导入 Revel、Mongo driver、`app/db` 或 BSON。`app/service → app/db` 是仓库既有允许边界，兼容 controller 可解析 ObjectID 或组装旧 update document，但 notes mutation controller 不直接操作 collection/client。跨域 port 只表达本任务所需的最小能力；对方任务仍是权限、文件生命周期、博客投影和邮件投递规则的唯一 owner。

## 3. Application contracts

### 3.1 Identity and permission

所有 command/query 显式携带：

- `ActorUserID`：来自 principal；
- `OwnerUserID`：私有资源等于 actor，共享资源由 owner-scoped resource lookup + permission port 推导；
- resource ID / expected USN / field presence；
- 可选稳定 operation identity；Web create 使用客户端稳定 `NoteId`，API create 接受可选客户端 `NoteId`，未提供时不承诺 HTTP unknown-result 后盲重试安全。

私有 repository 方法始终以 `(resource ID, OwnerUserID)` 查询。共享 mutation 先验证 actor 对 owner/resource 的权限，再使用同一个 owner 条件写入；审计字段写 actor。not-found 与 unauthorized 的公开映射仍按 action inventory，不建立一个跨端点通用 HTTP 错误。

### 3.1.1 Confirmed Web operation generation contract (KD-N6; originally Q-N9)

现有 Web adapter 只接收资源与业务字段：`UpdateNoteOrContent`、`CopyNote`、`CopySharedNote`、`DeleteNote` 和 `MoveNote` 都没有客户端 operation ID，Web update 也没有 expected USN。服务端不能从相同 payload 判断第二次请求是网络重试还是用户主动重复操作，因此不得用永久 payload hash 折叠请求，也不得把当前路径标为 retry-safe。

已确认采用以下兼容协议：

- 上述 mutation 增加可选 `OperationId`；仅 `UpdateNoteOrContent` 增加可选 `ExpectedUsn`。省略字段的旧客户端保持现有请求/响应外形，但 application result 必须为 `RetrySafe=false`，不承诺 unknown-result 重试或 stale-write 检测。
- `OperationId` 由客户端按一次用户意图生成，并在同一请求的 unknown-result 重试中复用；服务端按 owner + action 隔离，绑定 canonical input digest，同 ID 不同输入返回 typed conflict。
- update 在提供 `ExpectedUsn` 时做 owner-scoped generation compare；operation receipt 使相同 `OperationId` + 相同输入返回原提交结果。copy/shared-copy 首次执行时把 destination note/content/asset identity 冻结进 receipt；外层 batch 为每个输入 item 冻结子 operation identity 和顺序。
- 服务器端的可选字段、校验、receipt 与兼容 mapper 属于当前 notes leaf；第一方 Web 生成、持久化和复用字段属于 `presentation-frontend`。在后者落地前，现有第一方 Web 仍按旧客户端处理。
- 任何新增字段都不改变 route method、status、Content-Type 或 response body shape；无字段时不引入服务端自动重试、静默去重或隐藏 fallback。

实现边界：服务器端可选字段、校验、receipt 与兼容 mapper 属于当前 notes leaf；第一方 Web 生成、持久化和跨 unknown-result 重试复用字段属于 `presentation-frontend`。在后者落地前，现有第一方 Web 仍按旧客户端处理，因此不宣称旧路径 `RetrySafe`。若未来需要必填 idempotency key 或改变 wire shape，必须另行版本化，不得在本任务中隐式引入。

### 3.2 Typed command/result

设计阶段冻结以下概念，不要求沿用具体 Go 名称：

```text
WorkspaceCommandResult
  Status: committed | rejected | partial_write | side_effect
  Error: validation | not_found | unauthorized | conflict |
         duplicate | storage | timeout | partial_write | side_effect
  USN: optional committed USN
  RetrySafe: bool
  AppliedSteps / FailedStep: internal observability
  Items: optional per-item batch results
```

controller 只把该结果映射到当前 action 的 `Re`、`ReUpdate`、direct model/array 或 boolean；不能根据零值 model/bool 猜测成功，也不能将 storage error 变为空集合。

### 3.3 Repository ports

至少需要以下领域中立能力：

- owner-scoped note/notebook/tag/history query 与 conditional mutation；
- `AllocateUserUSN(owner)` 的原子 return-after；
- note create/update/delete 所需的 before-image 与 compare-by-expected-USN；
- notebook parent/target existence、owner、deleted 与 ancestor/cycle 检查；
- tag tombstone、按 owner 分页列出待 detach notes；
- sync query：owner + exclusive cursor + ascending USN + limit；
- note-specific unit-of-work runner，不复用身份专用 `UserInitializationPlan`。

repository 返回 typed found/conflict/storage/timeout，而不是由 service 解析 driver error 或 `bson.M`。

## 4. USN and sync design

### 4.1 Allocation

所有 note/notebook/tag 可见 mutation 使用一个 atomic allocator。同一次成功业务 mutation 只分配所需 USN；result、document 和 sync entry 对同一资源使用相同值。事务 callback retry 不在 callback 外产生第二次提交，也不复用旧 read-before-write counter。

### 4.2 Sync contract

- predicate：`OwnerUserID` 且 `Usn > afterUsn`；
- order：`Usn` ascending；
- limit：沿用逐 action `maxEntry` 行为；
- payload：保留现有 Note/Notebook/NoteTag API shape、nil/empty 和字段顺序；
- tombstone：D-01 notebook/tag 与已有 note permanent-delete 均以 resource document 的新 USN 返回；
- cursor：客户端以最后一项 USN 续游，精确最大值和超大值返回现有空数组。

同一用户并发测试必须验证唯一、严格递增和无重复；不同用户独立。standalone 在已分配 USN 后失败允许留下 gap，但任何模式都禁止 counter 回拨。

## 5. Mutation success boundaries

| Use case | Required commit boundary | Repairable projection / external port | Failure result |
| --- | --- | --- | --- |
| note create | note + note_content + one USN | tag/notebook count、image index 按最终分类修复 | transaction abort；standalone partial_write/续跑；无 client NoteId 不承诺盲重试安全 |
| note update | expected-USN check + requested metadata/content + one USN + history rule | image index、counts、publishing projection | conflict/rejected；partial 不能成功 |
| API notebook delete | owner-scoped tombstone + new USN | none | not-found/conflict/storage |
| Web notebook delete | child/note guard + tombstone + new USN | none | guarded rejection/storage |
| API tag delete | tag tombstone + new USN | none | not-found/conflict/storage |
| Web tag delete | tag tombstone + per-note detach USNs | durable resumable operation | partial_write；既有 shape 整体 false/`Ok:false` |
| notebook reparent/reorder | each distinct notebook merged parent/seq patch + one USN | durable resumable multi-item operation | validation/partial_write；既有 boolean false |
| move/from-trash | note target/status + one USN | source/target count、publishing projection | rejected/partial_write |
| permanent delete | note tombstone + new USN | content/history/assets cleanup with durable retry | partial_write until cleanup closes |
| copy/shared-copy | stable destination note/content identity + one USN | content asset copy、counts、publishing projection | compensate/resume, no duplicate destination |
| batch operations | per-item use case result | 既有 bool/`Re` shape；已有 `Item` 保留成功项 | 任一失败整体 false/`Ok:false`，never fixed success |

copy/shared-copy 与外层 batch 的 stable identity 按 KD-N6 由客户端可选 operation generation 提供，并由服务端 receipt 绑定；只有字段存在时才能把 retry 目标落实为可验收行为。旧请求路径仍不得标记为 `RetrySafe`，但这属于待实现/待证据的兼容分支，不再是规格阻塞。

每个 projection 在 action inventory 中只能选择：

1. `required-in-commit`：失败则主 mutation 不提交；或
2. `repairable-after-commit`：主记录保存 durable repair state，失败返回可观察 side-effect/partial 状态并由唯一 worker/command 重试。

禁止无结果 goroutine和“最后一个 bool 覆盖前面失败”。

## 6. Note create/update unit of work

### 6.1 Transaction-capable topology

1. 在进入 callback 前解析/验证 input，生成稳定 operation ID、note ID、normalized time 和 history revision identity。
2. callback 内 owner-scoped 读取、expected-USN compare、原子分配 USN 并执行必需写。
3. 所有必需写成功后才 commit；driver callback retry 使用同一稳定 identity 和幂等条件。
4. abort 后 note/content/history/user USN 均无可见变化；返回 action-compatible failure。

### 6.2 Standalone topology

使用 note-specific durable operation receipt 记录 operation identity、before-image/desired state、assigned USN、applied steps、failed step 和 retry status。补偿不能在并发下递减 user counter；已分配 USN 后失败允许留下 gap，命令返回失败/`partial_write` 并以同一 operation identity 查询、补偿或续跑。Web create 使用稳定 client `NoteId`；API create 仅在提供客户端 `NoteId` 时承诺幂等重试，未提供时继续服务端生成且不得声称盲重试安全。

### 6.3 History

history 与 content mutation 使用同一 operation/revision identity；仅在内容实际变化且主 mutation commit 时保存被替换的旧内容，newest-first 严格保留 10 条，callback retry/unknown-result retry 不重复。现有 API `GetHistories` 注释块不构成公开 action，本任务只保留现有 Web list。

## 7. Delete, move, copy and batch flows

- API/Web notebook/tag delete 共享底层 tombstone primitive，但保留 action-specific guard/side effect。
- permanent-delete 先保证 tombstone 可供 sync，再按 durable cleanup state 删除 content/history/assets；任何失败保留可重试状态。
- move-from-trash 是当前唯一恢复入口；目标 notebook 必须 owner/status 有效，源/目标 count 只能通过一个 projection seam 更新。不新增公开 restore action，未调用的 `recoverNote` 作为 dead-code candidate。
- copy/shared-copy 在复制文件前固定 destination identity；asset port 的 unknown result 必须可查询/去重。
- Web batch service 返回逐项结果；adapter 保持既有 boolean/`Re` shape，任一失败整体 false/`Ok:false`，已有 `Item` 的 action 保留已成功项。application 不丢弃失败，公开失败列表留待版本化设计。

## 8. Cross-domain seams

| Port | Provider task | This task may do | This task must not do |
| --- | --- | --- | --- |
| SharePermissionPort | application-publishing | check read/update grant for owner+actor+resource | 重写 share/group policy |
| ContentAssetPort | notes 拥有 note-specific receipt adapter；application-content 拥有并验收通用 primitive | notes 绑定 owner、note/source/destination、operation generation 和 digest，再消费 content 的 publish/verify/reconcile/copy/delete primitive | notes 解析通用文件根/路径/PDF 规则；content 复制 notes receipt 状态机 |
| PublishingProjectionPort | application-publishing | request IsBlog/public/tag projection update | 复制博客/主题规则 |
| HTTP result mapper | interface-http | expose stable application category | 改 route registry、method 或 runtime |
| Feedback/outbox | application-admin | 只交接现有缺陷和 outbox/durable-success 约束 | 实现 feedback、发送邮件或维护 admin policy |

### 8.1 API 文件集合 presence 边界

API note update adapter 将文件集合解码为三态，再交给唯一的 asset reconciliation 边界：

- 无 `Files`/`FilesPresent`/`HasFiles` 及任何 `Files[...]` key：absent，不调用 reconcile，保留现有关系；
- 显式 marker 且解码后集合为空：present-empty，调用 reconcile 删除全部关系；
- 任意 `Files[...]` key 或非空集合：present-values，按完整集合替换。

`FilesPresent` 是推荐的新 marker，`HasFiles` 只作为已有服务端兼容别名。不在这里要求 Android 端立即生成 marker；Android 调用与设备 E2E 归 sibling 重构任务。本 Web 项目只以服务端 contract 与真实 HTTP 验证三态语义。

## 9. Error and compatibility matrix

`research/action-inventory.md` 在实现准备阶段逐 action 固定输入/输出。设计约束为：

- input validation 和 principal resolution 在外部副作用前完成；
- service category 保持稳定，HTTP mapper 逐 action 保留现有 message/status/Content-Type/body shape；
- storage/timeout/decoder/uninitialized error 向上传播；
- only true empty query 可映射现有空数组/null；
- D-01 target Golden 与旧 known-defect before evidence 分开，不能覆盖历史文件制造通过；
- note content、HTML、用户数据和敏感配置不进入错误或日志。

## 10. Migration order and rollback

1. 冻结 action inventory、旧 defect ledger 和 target acceptance，不改行为。
2. 引入 typed result、owner/actor 和 repository ports；以静态依赖扫描为回滚点。
3. 实现 atomic USN 与 note unit-of-work；transaction/standalone 分别验证。
4. 迁移 note、notebook、tag、trash/history use case，每组独立回滚。
5. 让现有 controller 使用 typed use case；wire Golden 零非预期 diff。
6. 在固定摘要 Mongo 8 standalone 上执行真实 HTTP、Golden、USN/permission 和 files-presence 证据；将跨 Mongo 拓扑、进程/文件系统 failpoint 与前端 no-change 交给已有 sibling 任务。

回滚不得恢复 read-before-write USN、物理 notebook delete、stale tag USN、固定成功或吞 storage error。PRD 的 KD-N1～KD-N6 是已确认设计门，不能靠实现默认值或隐藏 fallback 绕过；KD-N6 的服务器端字段与 receipt wiring 在本任务闭合，第一方 Web 生成/复用及浏览器证据由 `presentation-frontend`/`delivery-verification` 接续。
