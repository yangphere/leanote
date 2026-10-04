# GHCR 版本 tag 镜像发布工作流

## 背景

生产 Compose 配置（任务 `10-04-compose-prod-dev-split`）需要拉取 GHCR 上的 leanote 镜像，但目前推送 tag 不会产出镜像：

- 规划时的历史状态是：`release.yml` 只存在于 `dev`，当时 `master` 上只有 `ci.yml` 与 `regression-baseline.yml`。用户随后决定把 `main` 设为默认分支和版本镜像来源。
- `release.yml` 依赖 `delivery-evidence`（自托管 `protected-delivery` runner 与 38 场景验证器）、受保护的 `release` 环境和 `RELEASE_APPROVED_IDENTITY`，这些尚未提供；`docs/modernization/cicd-delivery.md` 明确"a release must not be attempted"。
- GitHub 官方文档确认仓库工作流可用 `GITHUB_TOKEN` 首次发布并创建包；首次包默认为 private，目前真实远端是否已有 `leanote` 包仍未验证。

## 已确认决策（grill 结论）

1. **路径**：新增独立的轻量工作流 `.github/workflows/docker-image.yml`，推送无 `v` 的 `X.Y.Z` tag 时发布镜像到 GHCR；已推送的固定 tag 若在写入前失败，可从 `main` 显式 dispatch 恢复，必须绑定原提交与原质量证据。不修改 `release.yml`、不创建 GitHub Release、不发布 tarball。
2. **标签**：Git tag 和镜像版本 tag 均为 `X.Y.Z`。用户后续要求同时发布 `ghcr.io/yangphere/leanote:latest`；版本 tag 保持不可覆盖，`latest` 更新到同一个已验证版本 manifest digest，不发其他短版本标签。
3. **发布前门禁**：复用 `scripts/version.mjs` 的严格版本规则校验无前缀 tag + 复用完整 `quality-gate.yml` 的七个质量作业与汇总。仅供受保护 Release 的 `release-inputs` 不在无前缀 tag 上生成。
4. **同一镜像**：发布作业用 `release.yml` 相同的构建参数（`VERSION`、`REVISION`、`SOURCE_DATE_EPOCH`、`OCI_CREATED`，`linux/amd64`）构建一次，对该镜像运行 `scripts/container-smoke.sh`，通过后推送同一镜像并校验 registry digest。
5. **来源限制**：被打 tag 的提交必须是 `origin/main` 的祖先，否则拒绝发布；拒绝强制更新的 tag。`main` 是版本镜像唯一来源。
6. **与 `release.yml` 并存**：`release.yml` 保持不动，只响应 `v*.*.*`；轻量工作流只响应无前缀版本，两条触发路径分离。无前缀 `2.0.1` 不触发受保护 Release。轻量发布不授权 tarball 或 GitHub Release；受保护路径仍需独立完成全部门禁。这些后果写入 ADR 与 `cicd-delivery.md`。
7. **ADR**：新增 ADR-0005，记录"轻量门禁发布 GHCR 镜像、完整门禁继续作为 tarball 与 GitHub Release 的条件"，并注明它对 ADR-0004 中"全部门禁通过后才发布镜像"的偏离。
8. **首个版本**：用户最新指定 Git tag 和镜像标签均为 `2.0.1`；将 package/lock 版本同步为 `2.0.1`。先把 `dev` 合并到 `main` 再在 `main` 提交上打 tag。用户已授权主流程执行远端合并、默认分支设置、tag 与发布；实现子任务本身不操作远端。
9. **包可见性**：首次推送后由用户在 GitHub 网页把包设为 public（人工步骤，写入清单）；项目为开源仓库，生产部署假设可匿名拉取。

## 约束

- 仅使用 `GITHUB_TOKEN` 与 `packages: write`；不引入新密钥，不依赖自托管 runner、受保护环境或审批变量。
- 工作流中的第三方 action 使用与现有工作流一致的完整 SHA 固定。
- 不自动部署任何生产环境（ADR-0004 保持有效）。
- 不改 `Dockerfile`、不改 `release.yml` 与 `quality-gate.yml`。
- 保留已推送的 `2.0.1` tag（指向 `dea2306c32f27e648d9438cbc8b67cec6151b6cc`）；恢复时不得移动 tag 或用 main 执行器提交代替镜像候选。
- 恢复输入为 `tag`、`expected_commit`、`source_run_id`、`source_run_attempt`。验证原 Docker image push run 的仓库、工作流、tag、SHA、attempt，以及七个成功质量作业和汇总。原 run 可因 publish 失败整体为 failure；不能以执行器的质量结果替代候选证据。执行器仍须通过完整质量门。为已存在且核实的版本增加 main 上的显式 `update_latest` 操作，绑定预期 registry digest，只写 `latest`，不重推版本。

## 验收标准

- `docker-image.yml` 通过静态校验（YAML 解析、`actionlint` 若可用），接受无前缀版本 tag（glob 筛选后严格 `X.Y.Z` 校验）与 main 的显式恢复/已有版本晋升 dispatch；所有版本按同一镜像发布锁串行化，防止共享 `latest` 竞态。权限最小化（默认 `contents: read`，仅来源证据验证作业 `actions: read`，仅发布作业 `packages: write`）。
- 新增 JS 契约测试覆盖：触发器、版本/latest 标签、复用 `quality-gate.yml`、main 祖先校验、普通 CI 的 `dev`/`main` push 分支、smoke 在远端写入之前、发布前候选身份、推送后 digest 校验；相关测试通过。
- 首包响应分类明确：显式允许创建时，接受 manifest/listing 双 `NAME_UNKNOWN`，或 manifest `MANIFEST_UNKNOWN` + listing `NAME_UNKNOWN`；均须为结构化 404，若有 identity 必须匹配。现有包仍要求 `MANIFEST_UNKNOWN` + 精确 listing 200。单独 404、未知 JSON、身份冲突和权限/网络错误均拒绝。
- 恢复验证覆盖 source-run 身份/attempt/质量失败、summary provenance 不匹配及 candidate/executor 分离；实际构建与版本/revision/epoch/smoke 都绑定固定候选，registry 检查使用修复执行器 helper。
- `latest` 必须与版本 manifest digest 相同；版本发布/回读失败不得更新 latest。已发布 `2.0.1` 的晋升必须先验证 registry digest、候选 config、platform/version/revision 并对拉取镜像 smoke，只复制已有 manifest 到 latest，拒绝重新构建或重推版本。
- ADR-0005 与 `docs/modernization/cicd-delivery.md` 更新（GHCR 包创建、public 设置、与 `release.yml` 并存的后果）。
- 真实运行证据在任务记录中标为 `unrun`，直到真实推送 `2.0.1` tag、工作流发布成功并确认拉取 `ghcr.io/yangphere/leanote:2.0.1`；公开与匿名拉取单独记录。

## 人工步骤清单（用户执行）

1. 把 `dev` 合并到 `main`。
2. 把仓库默认分支设为 `main`，在选定的 `main` 提交上打 `2.0.1` tag 并推送（已授权主流程执行）。
3. 等 `docker-image.yml` 首次创建并推送 private 包后，在 GitHub 把 `leanote` 包设为 public。
4. 匿名执行 `docker pull ghcr.io/yangphere/leanote:2.0.1` 验证，并通知继续 `compose-prod-dev-split`。

## 非目标

- 不启用或修改受保护的 `release.yml` 流水线，不搭建自托管 runner 与验证器。
- 不做 ARM64（MOD-002）、不做镜像签名与 SBOM。
