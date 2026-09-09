# 应用层：笔记工作区与同步 — 技术设计

## Boundaries

Note/Notebook/Tag/Trash/History/Suggestion service 负责 mutation、查询、USN 和所有权；controller 只绑定输入并转换结果；db 只执行已约束查询。

## Data flow

用户请求 → 身份/所有权上下文 → workspace service → repository boundary → domain model/envelope；按 D-01 的 notebook/tag delete 原子写入新 USN tombstone，sync 从同一用户范围读取增量；旧差异仅作为回归前基线。

## Invariants

所有资源条件带 `UserId`；notebook/tag delete 按 D-01 分配新 USN、写入 `IsDeleted=true` tombstone 并与 sync 配对。USN 按 D-06 采用原子递增/CAS+重试；note-save 优先同一事务，失败不消耗 USN/不返回成功，无事务时显式补偿并暴露 `partial_write`。冲突、分页和空集合保持既有语义；编辑器保存状态不被 service 静默改写。

## Rollback

按 Note/Notebook/Tag/Trash/History/Suggestion 子域回滚，并保留已通过的 domain contract。
