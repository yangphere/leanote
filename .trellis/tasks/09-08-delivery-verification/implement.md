# 交付层：集成验证与版本发布 — 执行计划

- [ ] 读取全部子任务的验收矩阵，确认依赖和候选 commit 一致。
- [ ] 先读取并核对 `09-08-infrastructure-persistence/acceptance/evidence-matrix.md`、`09-08-application-identity/acceptance/evidence-matrix.md` 和 `09-08-interface-http` 的 route/replay 结果；缺任一前置矩阵或出现 `partial`/`unknown` 时保持 blocked。
- [ ] 运行 Go 1.26/1.27、MongoDB 8、Node/build、Golden/USN、Chromium、package/container/PDF 质量门。
- [ ] 依据 `scripts/browser-release-evidence.mjs` 和 `scripts/validate-browser-artifact.mjs` 在受保护真实环境执行 Chrome/Edge/Firefox/Safari current/previous 八槽四 coverage，生成并校验不可变 artifact。
- [ ] 验证生产配置、`/healthz`、非 root、外置 Mongo、上传持久化、完整 PDF 和错误路径。
- [ ] 用严格 `vX.Y.Z` tag 验证 tarball/SHA-256、OCI 元数据、GHCR digest 和 GitHub Release 输入。
- [ ] 任一证据缺失标为 blocked，保留恢复路径；通过后才允许发布任务归档。
