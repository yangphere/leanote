# GHCR 版本 tag 镜像发布工作流

## 背景

生产 Compose 配置（任务 `10-04-compose-prod-dev-split`）需要拉取 GHCR 上的 leanote 镜像，但目前推送 tag 不会产出镜像：

- `release.yml` 只存在于 `dev`，`master` 上只有 `ci.yml` 与 `regression-baseline.yml`。
- `release.yml` 依赖 `delivery-evidence`（自托管 `protected-delivery` runner 与 38 场景验证器）、受保护的 `release` 环境和 `RELEASE_APPROVED_IDENTITY`，这些尚未提供；`docs/modernization/cicd-delivery.md` 明确"a release must not be attempted"。
- GitHub 官方文档确认仓库工作流可用 `GITHUB_TOKEN` 首次发布并创建包；首次包默认为 private，目前真实远端是否已有 `leanote` 包仍未验证。

## 已确认决策（grill 结论）

1. **路径**：新增独立的轻量工作流 `.github/workflows/docker-image.yml`，推送 `v*.*.*` tag 时发布镜像到 GHCR；不修改 `release.yml`、不创建 GitHub Release、不发布 tarball。
2. **标签**：只发布精确标签 `ghcr.io/yangphere/leanote:vX.Y.Z`，不发 `latest`、`vX.Y`、`vX`。
3. **发布前门禁**：版本与 tag 校验（`scripts/check-version.mjs`）+ 复用完整 `quality-gate.yml`。
4. **同一镜像**：发布作业用 `release.yml` 相同的构建参数（`VERSION`、`REVISION`、`SOURCE_DATE_EPOCH`、`OCI_CREATED`，`linux/amd64`）构建一次，对该镜像运行 `scripts/container-smoke.sh`，通过后推送同一镜像并校验 registry digest。
5. **来源限制**：被打 tag 的提交必须是 `origin/master` 的祖先，否则拒绝发布；拒绝强制更新的 tag。
6. **与 `release.yml` 并存**：`release.yml` 保持不动。同一 tag 会同时触发它，它会在 `delivery-evidence` 卡住、其 quality-gate 重复运行；且其"拒绝已存在镜像"规则意味着该 tag 之后不能再通过它创建 GitHub Release。两条路径没有跨 workflow 原子锁；启用完整 Release 前必须禁用轻量路径并使用新 tag。这些后果写入 ADR 与 `cicd-delivery.md`。
7. **ADR**：新增 ADR-0005，记录"轻量门禁发布 GHCR 镜像、完整门禁继续作为 tarball 与 GitHub Release 的条件"，并注明它对 ADR-0004 中"全部门禁通过后才发布镜像"的偏离。
8. **首个版本**：`v1.0.0`（与 `package.json` 一致，不改版本号）；先把 `dev` 合并到 `master` 再在 `master` 提交上打 tag。**打 tag 与推送由用户执行或明确授权，本任务不自行推送。**
9. **包可见性**：首次推送后由用户在 GitHub 网页把包设为 public（人工步骤，写入清单）；项目为开源仓库，生产部署假设可匿名拉取。

## 约束

- 仅使用 `GITHUB_TOKEN` 与 `packages: write`；不引入新密钥，不依赖自托管 runner、受保护环境或审批变量。
- 工作流中的第三方 action 使用与现有工作流一致的完整 SHA 固定。
- 不自动部署任何生产环境（ADR-0004 保持有效）。
- 不改 `Dockerfile`、不改 `release.yml` 与 `quality-gate.yml`。

## 验收标准

- `docker-image.yml` 通过静态校验（YAML 解析、`actionlint` 若可用），触发器仅为 `push.tags: ['v*.*.*']`，权限最小化（默认 `contents: read`，仅发布作业 `packages: write`）。
- 新增 JS 契约测试覆盖：触发器、精确标签、复用 `quality-gate.yml`、master 祖先校验、smoke 在 push 之前、推送后 digest 校验、不含 `latest`；`npm test` 通过。
- ADR-0005 与 `docs/modernization/cicd-delivery.md` 更新（GHCR 包创建、public 设置、与 `release.yml` 并存的后果）。
- 真实运行证据在任务记录中标为 `unrun`，直到用户推送首个 `v*.*.*` tag 并确认 `docker pull ghcr.io/yangphere/leanote:v1.0.0` 成功。

## 人工步骤清单（用户执行）

1. 把 `dev` 合并到 `master`。
2. 在 `master` 提交上打 `v1.0.0` tag 并推送。
3. 等 `docker-image.yml` 首次创建并推送 private 包后，在 GitHub 把 `leanote` 包设为 public。
4. 匿名执行 `docker pull ghcr.io/yangphere/leanote:v1.0.0` 验证，并通知继续 `compose-prod-dev-split`。

## 非目标

- 不启用或修改受保护的 `release.yml` 流水线，不搭建自托管 runner 与验证器。
- 不做 ARM64（MOD-002）、不做镜像签名与 SBOM。
