# Notes 38-action live HTTP 验收索引

唯一业务契约来源：`../../09-08-application-notes/research/action-inventory.md`。本表只索引其 38 个稳定 action ID，不复制业务规则；执行时从该清单、native registry 和批准的 target 核对 method/input/result。历史 `observed GET`/legacy 方法不是新增允许方法的决定，冲突回 interface/notes。

历史覆盖按 notes 的 21/38 mapping 原样标识，不能作为当前合并后候选通过。每行至少附一个具名真实 HTTP case；mutation 另覆盖适用的 validation、owner、conflict、storage/timeout、多写失败、同操作重试；GET 也覆盖权限/非法输入/DB 错误。新增成功率按唯一 action ID 统计，不按 Go test event 数。

| ID | 入口 | notes 历史映射 | 当前候选 |
| --- | --- | --- | --- |
| WN-01 | /note/listNotes | mapped | unrun |
| WN-02 | /note/listTrashNotes | missing | unrun |
| WN-03 | /note/getNoteAndContent | missing | unrun |
| WN-04 | /note/getNoteContent | missing | unrun |
| WN-05 | /note/getNoteAndContentBySrc | missing | unrun |
| WN-06 | /note/searchNote | missing | unrun |
| WN-07 | /note/searchNoteByTags | missing | unrun |
| WN-08 | /note/updateNoteOrContent | mapped | unrun |
| WN-09 | /note/deleteNote | missing | unrun |
| WN-10 | /note/deleteTrash | missing | unrun |
| WN-11 | /note/moveNote | missing | unrun |
| WN-12 | /note/copyNote | missing | unrun |
| WN-13 | /note/copySharedNote | missing | unrun |
| WH-01 | /noteContentHistory/listHistories | missing | unrun |
| WB-01 | /notebook/getNotebooks | mapped | unrun |
| WB-02 | /notebook/addNotebook | missing | unrun |
| WB-03 | /notebook/updateNotebookTitle | missing | unrun |
| WB-04 | /notebook/dragNotebooks | missing | unrun |
| WB-05 | /notebook/deleteNotebook | missing | unrun |
| WT-01 | /tag/updateTag | mapped | unrun |
| WT-02 | /tag/deleteTag | missing | unrun |
| AN-01 | /api/note/getSyncNotes | mapped | unrun |
| AN-02 | /api/note/getNotes | mapped | unrun |
| AN-03 | /api/note/getTrashNotes | mapped | unrun |
| AN-04 | /api/note/getNote | mapped | unrun |
| AN-05 | /api/note/getNoteAndContent | mapped | unrun |
| AN-06 | /api/note/getNoteContent | mapped | unrun |
| AN-07 | /api/note/addNote | mapped | unrun |
| AN-08 | /api/note/updateNote | mapped | unrun |
| AN-09 | /api/note/deleteTrash | mapped | unrun |
| AB-01 | /api/notebook/getSyncNotebooks | mapped | unrun |
| AB-02 | /api/notebook/getNotebooks | mapped | unrun |
| AB-03 | /api/notebook/addNotebook | mapped | unrun |
| AB-04 | /api/notebook/updateNotebook | mapped | unrun |
| AB-05 | /api/notebook/deleteNotebook | mapped | unrun |
| AT-01 | /api/tag/getSyncTags | mapped | unrun |
| AT-02 | /api/tag/addTag | mapped | unrun |
| AT-03 | /api/tag/deleteTag | mapped | unrun |

每行执行记录必须引用 `evidence-matrix.md` 的完整 provenance 字段，并记录 method/path、principal 类别、输入字段 presence（不记录敏感原文）、status/Content-Type/body 断言、owner/USN/history/receipt 断言、Golden baseline 或批准 target 文件、命名用例和清理结果。旧 mapper 无 fixture 时先保存可审计基线，目标行为不同则独立 target，禁止 replay 改写 Golden。

汇总约束：总数 38；历史 mapped 21（17 API、4 Web）、missing 17（Web）；本轮当前候选执行 0，全部 unrun。
