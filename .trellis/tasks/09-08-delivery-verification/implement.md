# 交付层：集成验证与版本发布 — 执行计划

- [ ] 读取全部子任务的验收矩阵，确认依赖和候选 commit 一致。
- [ ] 先读取并核对 `09-08-infrastructure-persistence/acceptance/evidence-matrix.md`、`09-08-application-identity/acceptance/evidence-matrix.md` 和 `09-08-interface-http` 的 route/replay 结果；缺任一前置矩阵或出现 `partial`/`unknown` 时保持 blocked。
- [ ] 读取 `09-08-application-notes/research/action-inventory.md` 并生成 38 行执行表；标记已有 21/38（17 API + 4 Web）证据，为其余 17 个 Web action 补齐 live HTTP/Golden，不从 Go test 总事件数推导 action coverage。
- [ ] 运行 Go 1.26/1.27、MongoDB 8、Node/build、Golden/USN、Chromium、package/container/PDF 质量门。
- [ ] 运行 Mongo 7 standalone 与 Mongo 8 replica-set notes scenarios；在 durable receipt 的 claim/apply/verify/save/commit 边界执行 kill/restart 和 Mongo failpoint，记录最终 receipt/resource/USN/history 状态。
- [ ] 在持久化卷/跨主机文件系统执行 publish-before-row、row-before-publish、Verify 错误和重启 failpoint，验证 digest/no-clobber/destination identity 与 provider contract。
- [ ] 消费 presentation artifact，验证 update、copy/shared-copy、delete/move batch 的第一方 `OperationId`/`ExpectedUsn` 生成、unknown-result 复用、新意图换代和 stale conflict；server receipt/mapper 缺陷回流 notes。
- [ ] 依据 `scripts/browser-release-evidence.mjs` 和 `scripts/validate-browser-artifact.mjs` 在受保护真实环境执行 Chrome/Edge/Firefox/Safari current/previous 八槽四 coverage，生成并校验不可变 artifact。
- [ ] 验证生产配置、`/healthz`、非 root、外置 Mongo、上传持久化、完整 PDF 和错误路径。
- [ ] 用严格 `vX.Y.Z` tag 验证 tarball/SHA-256、OCI 元数据、GHCR digest 和 GitHub Release 输入。
- [ ] 任一证据缺失标为 blocked，保留恢复路径；通过后才允许发布任务归档。
