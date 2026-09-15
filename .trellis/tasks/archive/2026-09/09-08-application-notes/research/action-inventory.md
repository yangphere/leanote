# 应用层：笔记工作区与同步 — Action Inventory

## 1. Contract authority

本清单在 2026-09-11 规格审核阶段依据 `conf/routes`、现有 Web/API controllers、可读前端调用和 `app/tests/golden` / `app/tests/harness` 建立。它定义本任务必须迁移的入口和责任边界，不把当前缺陷升级为兼容要求。

- `conf/routes` 对 Web action 和 `/api/:controller/:action` 使用 `*`/catch-all；表中的 GET/POST 是现有第一方调用或 Golden 观察值。本任务保持这些观察值，不新增 method enforcement；完整 route/method 收敛归 `interface-http`。
- 所有已认证 Web action 的 actor 来自 session，API action 的 actor 来自 token；请求中的 `UserId`、`FromUserId` 或资源 owner 不能覆盖 principal。
- 正常响应继续使用 HTTP 200 与 `application/json; charset=utf-8` 的逐 action body；未登录的受保护 API 继续使用 `{"Ok":false,"Msg":"NOTLOGIN"}`。`ApiAuth.Logout` 按 identity P-06 的已批准例外处理：缺失/无效 token 是不恢复身份的幂等登出并返回既有 `ApiRe` 成功形状。具体 key 顺序、nil/empty 和 message 以现有 Golden 或修改前新增的 named baseline 为准；当历史 Golden 与后续已批准决策冲突时，须记录决策依据并更新该 Golden。
- 表中“目标修复”只包括 D-01、D-06 和 PRD KD-N1～KD-N6；其他观察到的行为先冻结，证据缺口不授权新增校验、统一 envelope 或改变分页排序。

## 2. Shared input and result rules

| 类型 | 规则 |
| --- | --- |
| ObjectID | required ID 必须区分 missing、blank、invalid、not-found；target application command 不接受 zero-ID fallback 或 panic。adapter 的公开 message 按各 action baseline 映射。 |
| owner / actor | 私有 query/mutation 使用 `(resource ID, OwnerUserID)`；shared read/update/copy 先由 verified share relation 推导 owner，再校验 actor permission，最终写仍带 owner 条件。 |
| update presence | Web `UpdateNoteOrContent` 的 `Desc`、`ImgSrc`、`Title`、`Tags`、`Content`，API `UpdateNote` 的 `Desc`、`Title`、`IsTrash`、`IsBlog`、`Tags[0]`、`NotebookId`、`Content` 继续按 presence 区分 absent 与显式空值。API 文件集合是三态输入：缺少 `Files`/`FilesPresent`/`HasFiles` 及任何 `Files[...]` key 时保持现有关系；`FilesPresent=1` 或兼容别名 `HasFiles=1` 且集合为空时清空；任意 `Files[...]` 或非空解码集合表示完整替换。`FileDatas[...]` 不能单独触发 reconcile。 |
| sync | predicate 为 owner + `Usn > afterUsn`，按 `Usn` 升序；`maxEntry == 0` 的 API 默认值为 100。负数、超大值和 page 边界在改 mapper 前以 named baseline 冻结。 |
| mutation result | application 返回 typed success/error、committed USN、retry safety、partial/side-effect 和可选逐项结果；adapter 不通过零值 model/bool 猜测成功。 |
| batch | delete/move/copy/tag-detach 非全批原子；任一 item 失败时既有 boolean/`Re` shape 整体 false/`Ok:false`。已有 `Item` 的 copy action只返回已成功项；公开失败列表需版本化，当前只做内部可观测。 |
| create idempotency | Web create 使用客户端稳定 `NoteId`；API create 接受可选 client `NoteId`，未提供时由服务端生成且不承诺 unknown-result 后盲重试安全。 |
| Web operation generation（KD-N6，原 Q-N9） | `UpdateNoteOrContent`、copy/shared-copy、delete/move batch 增加可选客户端 `OperationId`，Web update 另增加可选 `ExpectedUsn`。提供时按 owner + action + canonical input digest 绑定 receipt；同 ID 不同输入为 conflict，同 ID/同输入返回原提交结果；batch 按输入顺序冻结子 operation identity。省略字段时保持旧 wire contract 且 `RetrySafe=false`；不得用永久 hash 去重。第一方 Web 生成和重试复用交接 `presentation-frontend`。 |
| history | 仅内容实际变化且主 mutation commit 时保存被替换旧版；newest-first 严格 10 条；相同内容、callback retry 和 unknown-result retry 不重复。 |

## 3. Web note and history actions

| ID | 现有入口与输入 | Application use case / owner | 公开结果与错误边界 | 写集合、USN、跨域 port | 基线证据 |
| --- | --- | --- | --- | --- | --- |
| WN-01 | GET `/note/listNotes`; `notebookId`, page | `ListNotes(actor, notebook, trash=false, page)`；notebook 必须同 owner | direct `[]Note`，保留排序、分页、nil/empty；storage 不得伪装空集合 | read-only | `NoteController.go:145-148`; `golden/web/note_listNotes.json` |
| WN-02 | GET `/note/listTrashNotes`; page | `ListNotes(actor, trash=true, page)` | direct `[]Note`；真实 empty 与 query failure 分开 | read-only | `NoteController.go:150-154`; `public/js/app/notebook.js:607` |
| WN-03 | observed GET `/note/getNoteAndContent`; `noteId` | `GetNoteAndContent(actor, owner, note)`；允许经 permission port 的 shared read | direct `NoteAndContent`，not-found/unauthorized 外形按 pre-change baseline | read-only；`SharePermissionPort` | `NoteController.go:156-159` |
| WN-04 | GET `/note/getNoteContent`; `noteId` | `GetNoteContent(actor, owner, note)` | direct `NoteContent`；不泄露 foreign resource | read-only；`SharePermissionPort` | `NoteController.go:171-175`; `public/js/app/note.js:798` |
| WN-05 | GET `/note/getNoteAndContentBySrc`; `src` | `GetBySource(actor, src)` owner-scoped | `Re{Ok,Item:NoteAndContentSep}`，未找到保持 `Ok:false` | read-only | `NoteController.go:161-169`; `public/js/app/page.js:1437` |
| WN-06 | POST `/note/searchNote`; `key`, page | `SearchNotes(actor, key, page, UpdatedTime desc)` | direct `[]Note`；当前 regex/literal 行为先以 baseline 冻结，storage 不得空成功 | read-only | `NoteController.go:340-344`; `public/js/app/note.js:1581` |
| WN-07 | GET `/note/searchNoteByTags`; repeated `tags`, page | `SearchNotesByTags(actor, tags, page, UpdatedTime desc)` | direct `[]Note`；tag normalization 保持 Web 现状 | read-only | `NoteController.go:346-350`; `public/js/app/tag.js:375` |
| WN-08 | POST `/note/updateNoteOrContent`; `NoteOrContent` + field presence + optional `OperationId`/`ExpectedUsn` | create 或 metadata/content/combined update；actor 来自 session，shared owner 由 permission 推导 | `Re`; create success 为 `Ok:true, Item:Note`，update success 为 `Ok:true`；validation/conflict/permission/storage/partial 均 `Ok:false` 且 `Msg` 非空；有 generation 时相同 operation 重试返回原结果，无 generation 时 `RetrySafe=false` | D-06：note+content+one USN；combined update 同一 unit-of-work；history 规则；KD-N6 receipt；content/publishing ports | `NoteController.go:178-271`; `note_save_contract_test.go`; `tests/js/note-save*.test.js`; KD-N6 |
| WN-09 | POST `/note/deleteNote`; repeated `noteIds`, `isShared`, optional `OperationId` | private trash/permanent-delete 或删除 shared reference；逐 item owner/permission | existing boolean；任一失败整体 `false`，不新增公开 item errors；有 generation 时按冻结的 batch 子 operation 续跑 | 每个成功资源独立 USN；permanent-delete durable cleanup；KD-N6 receipt；share port | `NoteController.go:281-296`; `public/js/app/note.js:1440`; KD-N1/KD-N6 |
| WN-10 | legacy `/note/deleteTrash`; `noteId` | `PermanentDelete(actor, owner, note)`；保留 deprecated alias 直到 interface-http 决定移除 | existing boolean；所有 required step 完成才 true | tombstone+new USN 后 durable content/history/assets cleanup | `NoteController.go:298-301` |
| WN-11 | POST `/note/moveNote`; repeated `noteIds`, `notebookId`, optional `OperationId` | `MoveNotes(actor, items, target)`；target 同 owner、active、无权限泄露；move-from-trash 是唯一 restore | existing boolean；任一失败整体 false，不承诺全批原子；有 generation 时按冻结的 batch 子 operation 续跑 | 每 note 一次 USN；source/target count 和 publishing projection 分类为 required/repairable；KD-N6 receipt | `NoteController.go:303-310`; `public/js/app/note.js:1705`; KD-N1/KD-N6 |
| WN-12 | POST `/note/copyNote`; repeated `noteIds`, `notebookId`, optional `OperationId` | `CopyNotes(actor, owned sources, target)` | `Re{Ok,Item:[]Note}`；partial 时 `Ok:false` 且 Item 仅含成功项；有 generation 时 destination/sub-operation identity 稳定 | 每 destination identity/USN 稳定；`ContentAssetPort`；KD-N6 receipt；counts/projection | `NoteController.go:312-323`; `public/js/app/note.js:1792`; KD-N1/KD-N6 |
| WN-13 | POST `/note/copySharedNote`; repeated `noteIds`, `notebookId`, `fromUserId`, optional `OperationId` | `CopySharedNotes(actor, verified owner/share, target)`；不信任裸 `fromUserId` | `Re{Ok,Item:[]Note}`；partial 规则同 WN-12；有 generation 时 unknown-result 可按 receipt 续跑 | permission + asset copy ports；KD-N6 destination/sub-operation identity | `NoteController.go:325-336`; `public/js/app/note.js:1795`; KD-N6 |
| WH-01 | GET `/noteContentHistory/listHistories`; `noteId` | `ListHistories(actor, owner, note)` | direct `[]EachHistory`，newest-first，最多 10；foreign/not-found 不泄露 | read-only；history 只由 WN-08/API-N08 的 content commit 写 | `NoteContentHistoryController.go:12-16`; `public/js/plugins/history.js:113`; KD-N4 |

## 4. Web notebook and tag actions

| ID | 现有入口与输入 | Application use case / owner | 公开结果与错误边界 | 写集合、USN、跨域 port | 基线证据 |
| --- | --- | --- | --- | --- | --- |
| WB-01 | GET `/notebook/getNotebooks` | `GetNotebookTree(actor)` | direct `SubNotebooks`；保留树顺序、`Subs` 和 nil/empty | read-only | `NotebookController.go:20-24`; `golden/web/notebook_getNotebooks.json` |
| WB-02 | POST `/notebook/addNotebook`; client `notebookId`, `title`, optional `parentNotebookId` | `CreateNotebook(actor, clientID, title, parent)`；parent 同 owner、active、no cycle | success direct `Notebook`，failure boolean `false` | notebook + one USN | `NotebookController.go:31-48`; `public/js/app/notebook.js:807` |
| WB-03 | POST `/notebook/updateNotebookTitle`; `notebookId`, `title` | `RenameNotebook(actor, owner, noteBook, title)` | existing boolean；storage 不得伪装成功 | conditional owner write + one USN | `NotebookController.go:50-53`; `public/js/app/notebook.js:774` |
| WB-04 | POST `/notebook/dragNotebooks`; JSON string `data={CurNotebookId,ParentNotebookId,Siblings}` | `ReparentAndReorder(actor, current, parent, siblings)`；所有 IDs 同 owner、active、无重复、no self/cycle | existing boolean；invalid JSON/ID/partial 为 false，不 panic；不承诺全批原子 | 每个 distinct notebook 将 parent/seq 合并为一次 conditional write 和一个 USN；stable receipt 续跑且不重复 | `NotebookController.go:61-74`; `NotebookService.go:334-354`; `public/js/app/notebook.js:152` |
| WB-05 | observed GET `/notebook/deleteNotebook`; `notebookId` | Web guarded delete：拒绝 active child 或 non-trash note | `Re{Ok,Msg}`，保留 guard messages | D-01 owner-scoped tombstone + new USN；不级联 | `NotebookController.go:26-29`; `public/js/app/notebook.js:847` |
| WT-01 | POST `/tag/updateTag`; `tag` | `AddOrReactivateTag(actor, normalized-by-Web-rule tag)` | `Re{Ok:true,Item:NoteTag}` on committed success；failure不得固定 true | tag upsert/reactivate + one USN | `TagController.go:16-22`; `golden/web/tag_updateTag.json` |
| WT-02 | POST `/tag/deleteTag`; `tag` | durable Web delete-and-detach；owner-scoped tag + paged owned notes | `Re{Ok,Item}`；任一步失败 `Ok:false`，Item 只含已成功 note→USN map，不新增 error list | tag tombstone USN + each detached note USN；stable operation receipt | `TagController.go:24-30`; `public/js/app/tag.js:341`; KD-N1/D-01 |

## 5. API note actions

所有下列入口均经 `/api/:controller/:action` 认证；未登录/无效 token 的 first-party baseline 保持 `ApiRe{Ok:false,Msg:"NOTLOGIN"}`。

| ID | 现有入口与输入 | Application use case / owner | 公开结果与错误边界 | 写集合、USN、跨域 port | 基线证据 |
| --- | --- | --- | --- | --- | --- |
| AN-01 | GET `/api/note/getSyncNotes`; `afterUsn`, `maxEntry` | `SyncNotes(actor, exclusiveCursor, limit)` owner-scoped | direct `[]ApiNote`，USN ascending；0 limit→100 | read-only sync/tombstones | `ApiNoteController.go:51-56`; `golden/usn/note_getSyncNotes_*` |
| AN-02 | GET `/api/note/getNotes`; optional `notebookId`, page | `ListNotes(actor, notebook, false, page)` | invalid notebook → `ApiRe` message；success direct `[]ApiNote` | read-only | `ApiNoteController.go:61-69`; `golden/api/note_getNotes*.json` |
| AN-03 | GET `/api/note/getTrashNotes`; page | `ListNotes(actor, trash=true, page)` | direct `[]ApiNote` | read-only | `ApiNoteController.go:73-76`; `golden/api/note_getTrashNotes.json` |
| AN-04 | GET `/api/note/getNote`; `noteId` | `GetNote(actor, owner, note)` private | invalid/not-found → current `ApiRe` messages；success direct `ApiNote` | read-only | `ApiNoteController.go:127-142`; `golden/api/note_getNote*.json` |
| AN-05 | GET `/api/note/getNoteAndContent`; `noteId` | `GetNoteAndContent(actor, owner, note)` | direct `ApiNote` including normalized Content；invalid/not-found exact shape must be captured before mapper change | read-only；content URL mapping remains content adapter seam | `ApiNoteController.go:146-153`; `golden/api/note_getNoteAndContent.json` |
| AN-06 | GET `/api/note/getNoteContent`; `noteId` | `GetNoteContent(actor, owner, note)` | direct `ApiNoteContent`; auth errors keep `ApiRe` variant | read-only | `ApiNoteController.go:196-212`; `golden/api/note_getNoteContent*.json` |
| AN-07 | POST `/api/note/addNote`; `ApiNote`, required active owned `NotebookId`, optional client `NoteId`, optional Files | `CreateNote(actor, optionalClientID, fields)`；client `UserId` ignored | success direct `ApiNote` with Content/Abstract cleared as today；failure current `Re` shape/message | D-06 note+content+one USN；asset upload/reconcile via `ContentAssetPort`; optional NoteId idempotency | `ApiNoteController.go:216-337`; `golden/api/note_addNote.json`; KD-N3 |
| AN-08 | POST `/api/note/updateNote`; required `NoteId`,`Usn`; optional fields by presence；files 按 absent / marker-present-empty / present-values 三态输入 | `UpdateNote(actor, owner, expectedUSN, patch)`；metadata/content/combined one success boundary；files absent 保留关系，`FilesPresent=1`/`HasFiles=1` 空集合清空，非空 `Files[...]` 完整替换 | success direct `ApiNote` with Content cleared/current fields；failure `ReUpdate{Ok:false,Msg,Usn}` | D-06 one committed USN; history; files/content/publishing ports; conflict before side effects | `ApiNoteController.go:466-687`; `api_note_files_presence_test.go`; `golden/api/note_updateNote.json` |
| AN-09 | POST `/api/note/deleteTrash`; `noteId`,`usn` | `PermanentDelete(actor, owner, expectedUSN)` | `ReUpdate`; conflict/not-found/storage distinct | note tombstone + new USN precedes durable content/history/assets cleanup | `ApiNoteController.go:547-551`; `golden/usn/note_deleteTrash*.json` |

## 6. API notebook and tag actions

| ID | 现有入口与输入 | Application use case / owner | 公开结果与错误边界 | 写集合、USN | 基线证据 |
| --- | --- | --- | --- | --- | --- |
| AB-01 | GET `/api/notebook/getSyncNotebooks`; `afterUsn`,`maxEntry` | `SyncNotebooks(actor,cursor,limit)` | direct `[]ApiNotebook`; USN ascending；0 limit→100 | read-only tombstone sync | `ApiNotebookController.go:51-57`; `golden/usn/notebook_getSyncNotebooks_*` |
| AB-02 | GET `/api/notebook/getNotebooks` | `ListNotebooks(actor)` | direct flat `[]ApiNotebook` from full sync baseline | read-only | `ApiNotebookController.go:62-65`; `golden/api/notebook_getNotebooks.json` |
| AB-03 | POST `/api/notebook/addNotebook`; `title`,`parentNotebookId`,`seq` | `CreateNotebook(actor,serverID,title,parent,seq)`；invalid/non-owned/deleted parent rejected | success direct `ApiNotebook`; failure current `Re` | notebook + one USN | `ApiNotebookController.go:69-83`; `golden/api/notebook_addNotebook.json` |
| AB-04 | POST `/api/notebook/updateNotebook`; `notebookId`,`title`,`parentNotebookId`,`seq`,`usn` | `UpdateNotebook(actor,owner,expectedUSN,fields)`；same-owner/no-cycle | success direct `ApiNotebook`; failure `ApiRe` | conditional write + one USN | `ApiNotebookController.go:87-98`; `golden/api/notebook_updateNotebook.json` |
| AB-05 | POST `/api/notebook/deleteNotebook`; `notebookId`,`usn` | API permissive delete；不复用 Web non-empty guard | `ApiRe{Ok,Msg}` | D-01 owner-scoped tombstone + new USN；不级联 notes | `ApiNotebookController.go:102-105`; `golden/usn/notebook_delete*.json` |
| AT-01 | GET `/api/tag/getSyncTags`; `afterUsn`,`maxEntry` | `SyncTags(actor,cursor,limit)` | direct `[]NoteTag`; USN ascending；0 limit→100 | read-only tombstone sync | `ApiTagController.go:21-27`; `golden/usn/tag_getSyncTags_*` |
| AT-02 | POST `/api/tag/addTag`; `tag` | `AddOrReactivateTag(actor, API tag rule)` | success direct `NoteTag`; auth/error variant保持 | tag + one USN | `ApiTagController.go:45-48`; `golden/api/tag_addTag.json` |
| AT-03 | POST `/api/tag/deleteTag`; `tag`,`usn` | API tombstone only；不执行 Web detach | `ReUpdate{Ok,Msg,Usn}` | D-01 returned/stored/sync USN identical | `ApiTagController.go:52-55`; `golden/usn/tag_delete*.json` |

## 7. Internal operations and port inventory

| ID | Operation | Required input/output invariant | Owner / evidence |
| --- | --- | --- | --- |
| IN-01 | `AllocateUserUSN` | input owner；output atomic return-after USN。并发唯一、严格递增、无重复、不回拨；仅失败 standalone receipt 可留下 gap | notes application contract；Mongo adapter |
| IN-02 | `WorkspaceUnitOfWork` | transaction callback 接收稳定 operation/note/time/history identities；commit note/content/history/USN required set 或全部 abort | notes application + persistence transaction runner shape |
| IN-03 | standalone operation receipt | operation ID、before/desired state、assigned USN、applied/failed step、retry status；按 owner + action + canonical input digest 隔离，查询/补偿/续跑幂等，不隐藏 fallback | notes application + Mongo adapter；KD-N2/KD-N3/KD-N6 |
| IN-04 | notebook/tag count projection | 每项只能是 required-in-commit 或 durable repairable-after-commit；单一 recount/repair seam，不允许无结果 goroutine | notes application；最终 provider 由 action inventory 确认 |
| IN-05 | Web tag detach iterator | owner-scoped、分页列出关联 notes；每 note 独立 USN；receipt 能从失败位置续跑且不重复 | notes application；WT-02/D-01 |
| IN-06 | `SharePermissionPort` | input actor/owner/resource/action；output typed allowed/not-found/unauthorized/storage，不返回可绕过 owner query 的裸资源 | provider `application-publishing` |
| IN-07 | note-specific asset receipt adapter + `ContentAssetPort` | notes adapter 绑定 owner、stable operation、source/destination note 与 digest；通用 reconcile/copy/delete/publish/verify primitive 的 unknown result 可查询/去重 | adapter owner `application-notes`；primitive provider/acceptance owner `application-content` |
| IN-08 | `PublishingProjectionPort` | input committed note identity/USN + desired projection；required/repairable 分类唯一且结果可观察 | provider `application-publishing` |
| IN-09 | history append/retention | input owner/actor/note/revision/before-content；commit 后 newest-first 严格 10，revision 幂等 | notes application；KD-N4 |

## 8. Explicit handoffs and exclusions

| 入口/能力 | 责任任务 | 本任务交接内容 |
| --- | --- | --- |
| `Note.Index` 页面组合、模板选择、PJAX | `presentation-frontend` / `interface-http` | notes query DTO，不迁移页面 runtime |
| `/note/setNote2Blog`, `/notebook/setNotebook2Blog`, `IsBlog` projection | `application-publishing` | typed publishing port；批量 fixed-success defect 证据 |
| `/note/toPdf`, `/note/exportPdf`, `/api/note/exportPdf`, upload/file/attach/image | `application-content` | stable note/content identity 与 asset operation identity；不实现文件/PDF |
| API `GetHistories` 注释块 | none / out of scope | 当前没有可调用 action，不新增公开 API |
| Suggestion/feedback + email | `application-admin` | KD-N5：durable feedback success、outbox、禁用无结果 goroutine |
| route method enforcement、status/Content-Type runtime、Revel removal | `interface-http` | 本任务提供 typed result category 并保留逐 action mapper shape |

## 9. Implementation and acceptance trace

- WN-01～WN-13、WH-01、WB-01～WB-05、WT-01～WT-02、AN-01～AN-09、AB-01～AB-05、AT-01～AT-03 均须至少有一个 service contract case；mutation 还须覆盖 validation、not-found/unauthorized、conflict（适用时）、storage/timeout 和每个 multi-write failure point。带 KD-N6 字段的 action 另须覆盖 input digest 冲突、相同 operation 重试、expected-USN conflict 和 batch 子 operation 顺序/冻结。
- 每个 adapter 在迁移前先绑定现有 Golden；当前没有 Golden 的 action 新增 `baseline-before` fixture，D-01/D-06/KD-N1～KD-N6 target 使用独立文件，不能覆盖已知缺陷证据。
- AC-N1 在本清单覆盖 action 且本任务改变的 mutation 具备 focused contract/target fixture 后通过。当前 live harness 明确触达 API 17 个 action，以及 Web `listNotes`、`getNotebooks`、`updateTag`、`updateNoteOrContent` 4 个 action，共 21/38；其余 17 个 Web action 没有可审计的逐 action live HTTP 映射，继续由 `delivery-verification` 以 `delegated-unrun` 接收。不得用完整 harness 的 105 个 test event 冒充 38-action 覆盖。
- 当前 leaf 的真实 HTTP/Golden/USN/files-presence 由 `acceptance/evidence-matrix.md` 记录；Mongo 跨拓扑、浏览器 no-change、PDF 与跨任务 port provider 的实现/交付证据分别移交已有 `infrastructure-persistence`、`presentation-frontend`、`application-content`、`application-publishing`、`interface-http` 和 `delivery-verification`，不构成本清单的循环完成门禁。

## 10. Changed Web mutation evidence crosswalk

下表只证明本 leaf 修改过的 Web mutation 已有具名 service/receipt contract；它不把 service test 冒充真实 HTTP mapper 证据。`live HTTP` 为 `delegated-unrun` 的 action 继续由 `delivery-verification` 逐项回放。

| Action | Focused contract / target fixture | Leaf status | Live HTTP mapper |
| --- | --- | --- | --- |
| WN-08 update/no-op | `TestSaveNoteClientNoOpPersistsReceiptAndConflictsOnChangedInput`; `TestCommittedNoteRetryRequiresProjectionReceiptWhenGenerationAdvanced`; `TestUpdateNoteOrContentPropagatesUpdateFailureEnvelope`; `TestUpdateNoteOrContentNewSuccessUsesItemEnvelope`; `TestUpdateNoteOrContentReportsPartialWriteAsFailure`; `TestUpdateNoteOrContentNewPermissionFailureUsesEnvelope`; `TestUpdateNoteOrContentNewInsertFailureUsesEnvelope` | service + current mapped HTTP passed | mapped in current 21/38 |
| WN-09 delete batch | `TestWorkspaceBatchRejectsMalformedObjectIDs`; `TestTrashedNoteDeleteDecisionContinuesNewPermanentDelete`; `TestWorkspaceBatchItemOperationIdentityIsStableAndDistinct` | focused contract passed | `delegated-unrun` |
| WN-11 move batch | `TestRestoreMoveRetryStateFreezesOriginalGenerationAndPublicTime`; `TestReplayCommittedMoveUsesFrozenCommittedUSN`; `TestWorkspaceBatchIdentityBindsOwnerAndOrderedInput` | focused contract passed | `delegated-unrun` |
| WN-12 copy batch | `TestCopyNoteRetryResumesPendingCreationProjections`; `TestCopyOperationDigestBindsDestinationNotebook`; `TestFrozenWorkspaceBatchResultPreservesOriginalCopyResponseWithoutNoteContent` | focused contract passed | `delegated-unrun` |
| WN-13 shared-copy batch | `TestCopySharedNoteRetryResumesPendingCreationProjections`; `TestCopySharedNoteRetryUsesFrozenAttachmentManifest`; `TestCopyNoteImagesWithManifestIgnoresImageOutsideFrozenSet`; `TestCopySharedNoteWithoutOperationKeepsLegacySingleUSNCreate`; `TestStableCopiedImageIdentityBindsOperationSourceAndOwner`; `TestStableWebAttachIDReusesClientOperationAndScopesNote` | focused contract passed | `delegated-unrun` |
