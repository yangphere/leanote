# 实施计划

- [x] 读取 `.trellis/spec/backend/index.md` 与 `.trellis/spec/guides/index.md`，确认 CI/发布相关约定。
- [x] 新增 `.github/workflows/docker-image.yml`（按 design.md）。
- [x] 新增 `scripts/check-ghcr-tag-absent.mjs`，把现有包缺 tag 与显式首次创建的 registry 响应分类收敛为可测试 fail-closed 边界。
- [x] 新增 `tests/js/docker-image-workflow.test.js`（先写失败测试：触发器、标签、复用 quality-gate、master 祖先、smoke 先于 push、digest 校验、无 `latest` 及 registry 失败分类）。
- [x] 新增 `docs/adr/0005-publish-ghcr-image-with-lightweight-gate.md`，并在 ADR-0004 加一行指向 ADR-0005 的说明。
- [x] 更新 `docs/modernization/cicd-delivery.md`：GHCR 首次创建/public、与 `release.yml` 并存后果、人工步骤清单。
- [x] 验证：`node --test tests/js/docker-image-workflow.test.js`、`npm test`、`git diff --check`；`actionlint`（若可用）。
- [x] 任务记录：把真实 tag 推送、GHCR 发布、匿名 `docker pull` 标为 `unrun`。
- 回滚点：仅新增文件与文档追加，可整体 revert。
