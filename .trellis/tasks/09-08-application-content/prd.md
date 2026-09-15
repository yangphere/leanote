# 应用层：内容、文件与媒体 — PRD

## Goal

收敛文件、附件、图片、相册、PDF 和上传持久化业务，保持资源路径、权限、清理和导出行为。

## Scope

`FileService.go`、`AttachService.go`、`NoteImageService.go`、`AlbumService.go`、`html2image`、PDF controller、File/Attach/Album controllers 及 API File。

notes 拥有 note-specific asset idempotency/receipt adapter，包括 owner、source/destination note、operation generation 和 input digest 绑定。本任务拥有通用文件根、路径安全、原子/no-clobber 发布与 Verify、下载、相册和 PDF primitive，并验收 notes adapter 可在 unknown-result 后按 Apply/Verify 合同安全消费这些 primitive。

## Requirements

- 文件、附件、图片和相册读取/删除必须同时验证用户所有权或公开分享边界。
- 上传写入使用明确的持久化根目录，基于 canonical rooted path 禁止两种分隔符的路径穿越、符号链接/junction 逃逸和静默覆盖。
- PDF executable 只能使用受控路径并通过 argv、有界超时执行，禁止 shell 拼接；callback secret/token 不得进入 argv、环境快照、日志或错误输出，失败必须保留原始错误边界。
- PDF 导出保留完整能力、Content-Type、错误和超时语义；失败必须显式暴露。
- HTTP、service、DB 之间传递稳定的 ObjectID、文件元数据和错误 envelope。
- 通用 asset primitive 必须支持稳定 destination identity、摘要校验、no-clobber publish 和可查询 Verify；不复制 notes durable receipt 状态机。
- 迁移过程中不把临时文件、真实用户数据或凭据写入版本库和 artifact。

## Acceptance criteria

- [ ] 文件/附件/图片/相册 CRUD、下载、删除和跨用户拒绝有回归。
- [ ] `leaui_image`、上传清理和静态资源路径的 service/adapter contract 通过；真实服务 smoke 由 interface/delivery 任务验证。
- [ ] PDF 成功和失败响应、Content-Type、持久化依赖、argv 参数化和超时有 service 回归；Linux/container smoke 由 delivery 任务验收。
- [ ] 路径安全测试覆盖 POSIX/Windows traversal、绝对/UNC 路径、symlink/junction、ZIP 条目/展开大小限制和不可写目录。
- [ ] 消费 `application-notes` 的 note-specific asset operation contract，以 provider contract 验证 reconcile/copy/delete/publish/verify 的稳定 identity、digest、no-clobber、unknown-result Verify 和失败可观测；不重新实现 notes receipt 状态机。
- [ ] 所有延期的 PDF/ARM64 变更只链接 MOD-002，不在本任务扩大范围。

## Out of scope

不更换 PDF 渲染器、不实现 ARM64 镜像、不改变上传 URL 或数据库 Schema。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
