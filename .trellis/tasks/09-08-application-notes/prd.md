# 应用层：笔记工作区与同步 — PRD

## Goal

收敛笔记、笔记本、标签、回收站、内容历史、建议和同步 USN 业务，保证客户端可见的增量同步与资源所有权不变。

## Scope

`NoteService.go`、`NotebookService.go`、`TagService.go`、`TrashService.go`、`NoteContentHistoryService.go`、`SuggestionService.go` 及对应 Web/API controllers。

## Requirements

- 所有读写、删除、恢复和移动操作强制带用户所有权条件。
- 每个 mutation 正确递增 USN；`GetSync*` 按既有分页、排序、冲突和删除语义返回。
- 未编辑笔记关闭不保存；真实编辑保存保持文本、链接、图片、代码和插件标记语义。
- 失败、冲突、部分写入和 not-found 返回稳定 API envelope，不伪造成功。
- service API 与 HTTP/DB 实现解耦，避免 controller 重复查询或更新规则。

## Acceptance criteria

- [ ] Note/Notebook/Tag CRUD、Trash、History 和 Suggestion 服务测试通过。
- [ ] USN add/update/delete/conflict/sync Golden 在独立用户数据上通过。
- [ ] 跨用户访问返回既有 not-found/forbidden 语义且不泄露数据。
- [ ] note save 的只读、成功、失败和 partial write 回归通过。
- [ ] 应用层不直接依赖 Revel controller/result 或 Mongo collection 类型。

## Out of scope

不重写编辑器 UI、改变同步协议、批量重写历史 HTML 或引入新的协作协议。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
