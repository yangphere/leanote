# 应用层：笔记工作区与同步 — 技术设计

## Boundaries

Note/Notebook/Tag/Trash/History/Suggestion service 负责 mutation、查询、USN 和所有权；controller 只绑定输入并转换结果；db 只执行已约束查询。

## Data flow

用户请求 → 身份/所有权上下文 → workspace service → repository boundary → domain model/envelope；mutation 同步写 USN，sync 从同一用户范围读取增量。

## Invariants

所有资源条件带 `UserId`；USN 单调递增且与 mutation 配对；冲突、删除、分页和空集合保持既有语义；编辑器保存状态不被 service 静默改写。

## Rollback

按 Note/Notebook/Tag/Trash/History/Suggestion 子域回滚，并保留已通过的 domain contract。
