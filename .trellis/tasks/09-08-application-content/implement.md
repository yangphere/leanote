# 应用层：内容、文件与媒体 — 执行计划

- [ ] 盘点 File/Attach/NoteImage/Album/PDF 的 service、controller、静态资源和 fixture。
- [ ] 收敛所有权、路径解析、临时文件和清理策略，删除重复安全判断。
- [ ] 收敛 canonical rooted path、ZIP 条目/展开大小限制和 symlink/junction 拒绝，补齐 POSIX/Windows traversal、上传、下载、删除、跨用户拒绝和错误响应测试。
- [ ] 将 PDF executable 收敛为 allowlist + argv + timeout，补充 shell 元字符、空格/引号回归；验证 callback secret/token 不出现在进程参数、环境快照、日志或错误输出，并覆盖超时、失败和清理路径。
- [ ] 验证 `leaui_image` iframe、图片/附件元数据和 PDF Content-Type/失败语义。
- [ ] 记录 Linux/container 持久化目录和完整 PDF smoke 由 delivery 任务执行，并登记延期 MOD-002。

验证：目标 Go 测试、Mongo Golden、路径/PDF 回归和 `git diff --check`；`scripts/package-smoke.sh`、`scripts/container-smoke.sh` 的真实环境执行由 delivery 任务负责。
