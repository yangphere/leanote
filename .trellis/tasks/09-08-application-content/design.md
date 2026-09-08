# 应用层：内容、文件与媒体 — 技术设计

## Boundaries

File/Attach/NoteImage/Album/PDF service 负责资源生命周期和权限；filesystem/PDF/Mongo 是基础设施；HTTP adapter 只处理下载、上传和错误响应。

## Data flow

资源请求 → 所有权/分享判定 → metadata service → storage/DB/PDF boundary →安全路径和结果 envelope。上传临时文件在成功或失败后均有明确清理。

## Invariants

以 canonical rooted path 为唯一文件边界，拒绝 POSIX/Windows 分隔符穿越、绝对/UNC 路径和 symlink/junction 逃逸；ZIP 解压限制条目数、单项大小和总展开大小。
不静默覆盖；ObjectID/metadata/Content-Type 保持；PDF 只通过 allowlist executable、argv 和有界超时执行，callback secret/token 不进入 argv、环境快照、日志或错误输出，完整能力不能通过降级隐藏。

## Rollback

按文件、附件、相册、图片和 PDF 子边界回滚；不得删除已验证的上传清理或安全测试。
