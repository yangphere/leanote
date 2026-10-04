# `09-08-domain-contracts` 规格复审记录（2026-09-09）

## 复审结论

选中的 ready 叶仍是 `09-08-domain-contracts`：`task.json.meta.depends_on=[]`，当前任务图中其余领域、应用、持久化、接口、呈现和交付叶均直接或间接依赖它。现场 `task.json.status=in_progress`，因此本轮没有重复执行 `task.py start`。

本次复审只修改任务规格、研究和验收材料。当前 checkout 在本轮开始前已存在 `ab11900f`、`4c518fb2`、`073e98d7` 等领域实现提交；这些历史提交不应被误报为本轮审计授权，也不代表下游 D-01/D-06 或真实集成证据已经完成。

## 依据与现场差异

| 检查项 | 现场证据 | 结论/影响 |
|---|---|---|
| ready/dependency | `09-08-domain-contracts/task.json`、父任务 10 个 child 的 `meta.depends_on` | 领域叶唯一无依赖；保持已激活状态，其他叶不可越过该契约直接实现 |
| 模型目录 | `model-catalog.json`、`validate_model_catalog.py` | 69 个 active 类型、3 个注释声明、28 个业务集合 + `blogs` declared-only、29 个 API action 的计数边界成立 |
| 请求输入覆盖 | `UserBlogBase`/`UserBlogComment`/`UserBlogStyle` 在 `MemberBlogController` 绑定并由 `BlogService` 写入，但原 `input-contracts.json` 未登记 | 规格缺口已修复：目录与静态输入契约纳入 3 个博客设置部分更新载荷；字段 presence、空值覆盖、跨字段校验仍由下游 fixture 验收 |
| API 同步返回形状 | `TagService.GeSyncTags` 返回 `[]info.NoteTag`，`tag_getSyncTags.json` 为 Tag 对象数组 | 原目录的 `[]string` 错误已修复为 `[]NoteTag`，避免下游生成错误 DTO |
| DTO 与 BSON tag 责任 | `ApiNoteContent`/`ApiNotebook` 仅在 API controller 组装/返回；`UserAccount` 与 `UserBlog*` 通过 `UpdateByQI`/`UpdateByQMap` 作为部分更新载荷写入；`EachHistory` 是 `NoteContentHistory.Histories` 的嵌套持久化值 | 原目录把两个 API DTO 误标为 persistence secondary；已移除该 secondary，将四类更新载荷的带 tag 字段标为 `write_only`，并仅为 `EachHistory` 保留嵌套 persistence secondary；更新载荷完整 BSON fixture 仍为 unknown，不能由 struct round-trip 代替 |
| API 文档/Golden 方法冲突 | `auth_logout`、`notebook_deleteNotebook`、`note_deleteTrash` 的 Golden 使用 POST，而文档声明 GET；`conf/routes` 对 API 使用通配方法；`user_getSyncState` 的 Golden 使用 GET，而文档声明 POST | 未擅自选定兼容方法；目录新增 `compatibility_notes`，要求 interface fixture 逐 action 回放并保留未知边界 |
| user/info 输入冲突 | `ApiUser.Info()` 从认证 session 读取用户，旧 API 文档仍列 `userId` 参数 | 以 controller 的 session 路径作为当前静态观察，同时把文档冲突列为 unknown；不得让迁移层自行接受任意 userId |
| getSyncState 类型冲突 | controller 写入 `time.Now().Unix()`（整数），Golden 归一化后以 `UNIX_TIME_TOKEN` 表示 | 成功形状改为 `LastSyncTime:int64`/`LastSyncUsn:int`，文档字符串描述保留在 compatibility note，待 HTTP replay 决定公开兼容形状 |
| file auth 冲突 | `apiCommonUrl` 将三个读取 action 列入白名单；无 token Golden 返回文本，旧 API 前言却说除登录/注册外均需 token | 保留代码/Golden 的可观察白名单事实并显式记 note；不得在 HTTP 迁移时静默扩大或收紧权限 |
| AddNote requiredness | 文档要求 `Title`/`Content`，当前 controller 只显式拒绝无效 `NotebookId` | 输入契约把文档 required 与运行时语义分开，empty Title/Content rejection 保持 unknown；下游必须以 fixture 决定，不得凭文档或零值推断 |
| service-only 输入 | `UserAccount` 只在 `UserService.UpdateAccount` 作为更新载荷使用，未找到直接 controller binder | 保留字段契约但标注 service-only、binder unknown；不把它伪装为已存在的管理员 HTTP 输入 |

## 已实施的材料修复

1. `generate_model_catalog.go` 将 3 个博客设置结构归为 `request_input`，并记录真实 member-controller/service 消费者；重新生成目录。
2. API 目录修正 `getSyncTags` 的 `[]NoteTag` 返回形状、补充其与旧文档 `[type.Tag]` 的 `compatibility_notes`，修正 `getSyncState` 字段类型，并为方法、session 参数、file auth 和 AddNote requiredness 冲突加入 `compatibility_notes`；校验器对已知冲突 action 强制保留备注。
3. `NoteOrContent` 消费者清单移除无证据的 `app/service`，方法冲突备注改为准确描述 `conf/routes` 的通配方法。
4. `model-catalog.schema.json` 与校验器增加 `compatibility_notes` 约束。
5. `input-contracts.json` 增加 3 个博客设置部分更新结构、来源、字段和 runtime unknown；校验器的固定计数从 4 调整为 7。
6. PRD、技术设计、执行计划和证据矩阵同步 7 个输入结构、28+1 集合计数和下游 unknown 责任。

## 仍未闭合事项（不得擅自假设）

- D-01 notebook/tag 删除的新用户 USN、tombstone 和 sync 返回仍由 `application-notes` 实现与回放；旧差异只作 known-defect 对照。
- D-06 原子 USN、note-save 事务/补偿、`partial_write` 和重试幂等仍由 `application-notes` 与 `infrastructure-persistence` 实现与回放。
- 29 个 API action 的真实 HTTP 方法、状态码、Content-Type、错误消息、binder 重复字段和 multipart precedence 尚未全部 replay；目录统一保持 `partial` 或逐项 unknown。
- Mongo `listIndexes`、Mongo 7/8、完整 BSON round-trip、真实浏览器/HTML、PDF、容器和发布证据不属于本领域复审的可替代通过条件。
- 当前分支已有领域实现提交，因此“编码前审计”这一时序门在历史上已发生；本轮只完成材料复审与修补，未新增业务代码，也未把现有实现自动判为规格通过。

## 复审门禁

后续进入新功能编码前，必须重新读取本文件与 `prd.md`/`design.md`/`implement.md`，运行目录和输入契约校验，并对上述 unknown 保持 fail-closed。任何改变 JSON/BSON/API 方法、字段或权限的实现，都必须先同步目录、Golden、fixture 和下游依赖说明，再重新评审。
