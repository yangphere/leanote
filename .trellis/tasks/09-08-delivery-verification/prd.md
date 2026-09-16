# 交付层：集成验证与版本发布 — PRD

## Goal

以真实可审计证据验证所有分层任务的兼容性，并建立不自动部署生产的版本制品交付。

## Requirements

- PR/push 质量门覆盖 Go 1.26/1.27、MongoDB 8.0、Node 24、JS/build、Chromium、Golden/USN、打包和容器 smoke。
- 发布严格使用 `vX.Y.Z` tag，产出可复验 tarball/SHA-256 与 Linux/amd64 非 root GHCR 镜像，禁止 `latest` 别名和自动生产部署。
- 按 `interface-http` production-config seam 验证生产配置、退出码 78、`/healthz`、外置 Mongo、上传持久化和完整 PDF 行为，必须有 Linux/container 正向证据；content roots 需证明 data/quarantine paired volume、non-public、non-root writable、restart persistence 及 cross-device/static-reachable/overlap fail-closed。PDF 同时消费 `application-content` 的不可信 HTML contract，证明 local-file deny、零 outbound，以及 `file://`、private/metadata/redirect/CSS/script-fetch 恶意输入不能读取或嵌入目标内容。
- 真实 Chrome、Edge、Firefox、Safari 的 current/previous 八槽位各执行固定四项 coverage，artifact 绑定同一 commit/run/attempt 并通过 JCS digest 校验。
- 任一环境缺失、失败、清理失败或 artifact 缺失均保持 blocked，不能用局部单测替代。
- 汇合并核对 `infrastructure-persistence` 的事务/索引/TTL/outbox/token 证据、`application-identity` 的 principal/Session/API Golden 证据和 `interface-http` 的 route/session/replay 证据；任何交接矩阵缺项都保持 blocked。
- identity 交接按 2026-09-10 已确认的 P-01～P-08 验证：匿名期 `_ID` 稳定且登录成功轮换、无 CSRF/query-form token/`_ID`+Captcha 兼容基线、登出和注册 Cookie 失败 envelope、32 字节 API token 摘要存储及旧 token 过渡。书面确认不替代真实 Mongo/HTTP/browser/mail/release artifact。
- notes 交接按 `research/action-inventory.md` 建立 38-action 逐项 live HTTP 矩阵。已有可追溯覆盖为 21/38（17 API + 4 Web）；必须补齐其余 17 个 Web action，不得用 `discovered 105 / passed 103` 的 Go test event 数代替 action 覆盖。
- notes/content/presentation 交接必须在真实环境验证：第一方 Web `OperationId`/`ExpectedUsn` 生成与 unknown-result 复用；note-specific asset receipt adapter 消费通用 publish/verify primitive；Mongo 7 standalone、Mongo 8 replica-set、进程 kill/restart、Mongo failpoint 和跨主机文件系统 failpoint。每项分别记录 executed/pass/fail/skip，不得以 server standalone passed 覆盖 delegated-unrun。

## Acceptance criteria

- [ ] 所有子任务依赖、Go/JS/Golden/USN 和跨层权限回归通过。
- [ ] identity → persistence → interface 的交接矩阵逐项有 commit/run/attempt 绑定证据，且不存在把 mock、controller 直调或历史 artifact 当作真实通过的情况。
- [ ] identity P-01/P-06/P-07 与 persistence P-08 摘要 token contract 的跨层 replay、失败路径和日志脱敏有绑定证据；无 CSRF 与通用限流保持现状的风险记录可复核。
- [ ] notes 38-action 矩阵逐项绑定 method/path、principal、input presence、status/Content-Type/body、owner/USN 和 Golden/target；在已证明 21/38 基础上补齐 17 个 Web action，具体缺口以 notes action inventory 为唯一清单。
- [ ] 第一方 Web 的 `OperationId`/`ExpectedUsn` 在 update、copy/shared-copy、delete/move batch 中的生成、复用、换代、stale conflict 和 unknown-result 恢复有真实浏览器 + HTTP + DB receipt 绑定证据。
- [ ] Mongo 7 standalone、Mongo 8 replica-set 分别执行 notes durable mutation；在 claim/apply/verify/save/commit 边界注入 kill/restart 与 Mongo failpoint，证明恢复后不重复 USN/history/destination asset、不回拨 counter、不伪造成功。
- [ ] 跨主机/持久化卷文件系统 failpoint 验证 no-clobber publish、digest Verify、DB row/projection 与重启续跑；与 application-content provider contract 同一 candidate 绑定。
- [ ] 质量门 summary 与失败路径准确记录 discovery/execution、stage、exit code 和脱敏原因。
- [ ] 八槽浏览器 artifact、生产配置矩阵、tarball、容器、PDF 和 GHCR 证据可复核；PDF artifact 同时包含 self-contained 正向样例、local-file/zero-outbound 观测和恶意 HTML 负向样例。
- [ ] 发布前验证阻止错误版本、重复 tag/release/image、跨 run artifact 和不一致 digest。
- [ ] 仅在全部证据通过后创建 Release/GHCR；否则任务状态为 blocked 并记录恢复条件。
- [ ] 如逐 action、浏览器、跨拓扑或 failpoint 发现 notes 服务端契约缺陷，重开 `09-08-application-notes`；交接不允许把缺陷归类为“只属交付环境”。

## Out of scope

不自动部署生产、不发布 ARM64、不提交真实凭据、不删除失败 artifact、不用模拟 Safari 关闭真实浏览器门禁。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
