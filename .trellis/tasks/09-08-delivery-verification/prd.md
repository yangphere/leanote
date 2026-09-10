# 交付层：集成验证与版本发布 — PRD

## Goal

以真实可审计证据验证所有分层任务的兼容性，并建立不自动部署生产的版本制品交付。

## Requirements

- PR/push 质量门覆盖 Go 1.26/1.27、MongoDB 8.0、Node 24、JS/build、Chromium、Golden/USN、打包和容器 smoke。
- 发布严格使用 `vX.Y.Z` tag，产出可复验 tarball/SHA-256 与 Linux/amd64 非 root GHCR 镜像，禁止 `latest` 别名和自动生产部署。
- 按 `interface-http` production-config seam 验证生产配置、退出码 78、`/healthz`、外置 Mongo、上传持久化和完整 PDF 行为，必须有 Linux/container 正向证据。
- 真实 Chrome、Edge、Firefox、Safari 的 current/previous 八槽位各执行固定四项 coverage，artifact 绑定同一 commit/run/attempt 并通过 JCS digest 校验。
- 任一环境缺失、失败、清理失败或 artifact 缺失均保持 blocked，不能用局部单测替代。
- 汇合并核对 `infrastructure-persistence` 的事务/索引/TTL/outbox/token 证据、`application-identity` 的 principal/Session/API Golden 证据和 `interface-http` 的 route/session/replay 证据；任何交接矩阵缺项都保持 blocked。

## Acceptance criteria

- [ ] 所有子任务依赖、Go/JS/Golden/USN 和跨层权限回归通过。
- [ ] identity → persistence → interface 的交接矩阵逐项有 commit/run/attempt 绑定证据，且不存在把 mock、controller 直调或历史 artifact 当作真实通过的情况。
- [ ] 质量门 summary 与失败路径准确记录 discovery/execution、stage、exit code 和脱敏原因。
- [ ] 八槽浏览器 artifact、生产配置矩阵、tarball、容器、PDF 和 GHCR 证据可复核。
- [ ] 发布前验证阻止错误版本、重复 tag/release/image、跨 run artifact 和不一致 digest。
- [ ] 仅在全部证据通过后创建 Release/GHCR；否则任务状态为 blocked 并记录恢复条件。

## Out of scope

不自动部署生产、不发布 ARM64、不提交真实凭据、不删除失败 artifact、不用模拟 Safari 关闭真实浏览器门禁。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
