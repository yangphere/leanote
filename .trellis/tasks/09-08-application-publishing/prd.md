# 应用层：分享、博客与主题 — PRD

## Goal

收敛分享、博客、评论、群组、主题和预览业务，保持公开页面、用户上传主题和权限语义。

## Scope

`ShareService.go`、`BlogService.go`、`ThemeService.go`、`GroupService.go`、Blog/Share/Preview controllers、member controllers 和相关模板/i18n 调用。

## Requirements

- 私有资源与公开分享的权限边界必须由 service 统一表达。
- Blog、评论、点赞、阅读量、主题和预览保持 URL、参数、HTML 和错误语义。
- 内置与上传主题的资源解析必须通过 canonical rooted path，不得越出允许目录；模板错误要可定位。
- 页面渲染只消费应用服务结果，不能在模板或 controller 中重复业务查询。
- 公开页面与 API 的用户/资源所有权条件必须可审计。

## Acceptance criteria

- [ ] 分享创建/读取/撤销、博客内容/评论/统计、群组和主题服务测试通过。
- [ ] 三个内置主题与至少一个上传主题的 service/path/template contract 测试通过；真实页面 smoke 由 interface/delivery 任务验收。
- [ ] 未授权、过期分享、未知主题和模板错误均有稳定响应。
- [ ] publishing service 输出的公开资源路径和 i18n 契约稳定；新 HTTP/模板边界的页面兼容由 interface/delivery 任务验收。
- [ ] 不引入 SPA、视觉重设计或新的公开 URL。

## Out of scope

不新增社交功能、不改博客主题设计、不改变公开 URL/API 或引入新的模板引擎。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
