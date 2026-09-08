# 应用层：分享、博客与主题 — 技术设计

## Boundaries

Share/Blog/Theme/Group service 负责公开性、权限、主题和发布状态；模板、i18n、静态文件和 HTTP 为适配层；不在模板中执行资源授权。

## Data flow

请求 → 分享/用户/群组权限判定 → publishing service → domain result → 模板或 API adapter。公开页面和私有页面使用同一所有权条件核心。

## Invariants

分享撤销、过期、评论/点赞/阅读量、主题目录和模板错误保持可观察；用户上传主题通过 canonical rooted path 解析，拒绝 POSIX/Windows traversal、绝对/UNC 路径和 symlink/junction 逃逸；ZIP 导入限制条目数、单项大小和总展开大小。

## Rollback

按 share/blog/theme/group/member 子边界回滚；保留公开 URL 和既有模板资源。
