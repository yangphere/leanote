# ADR-0005：通过轻量门禁发布 GHCR 版本镜像

- **状态**：Accepted
- **日期**：2026-10-04

## Context

生产 Compose 需要可匿名拉取的版本镜像，但 ADR-0004 的受保护 Release 工作流依赖尚未提供的 delivery runner、浏览器证据和人工批准环境。继续把 GHCR 镜像与 tarball、GitHub Release 绑定，会让 Compose 没有可用的官方镜像。

GitHub 官方文档说明，仓库内 GitHub Actions 工作流可以用 `GITHUB_TOKEN` 发布与该仓库关联的容器包；首次发布会创建包，默认可见性为 private。Dockerfile 已包含 `org.opencontainers.image.source=https://github.com/yangphere/leanote`，因此无需先在网页预创建空包，但首次发布后仍需管理员把包改为 public，匿名拉取才成立。

## Decision

- 新增 `.github/workflows/docker-image.yml`，仅响应无前缀版本 tag push；严格 `X.Y.Z` Git tag 发布 `ghcr.io/yangphere/leanote:X.Y.Z` 的 `linux/amd64` 镜像。用户指定首发 Git tag 和镜像 tag 均为 `2.0.1`。
- 发布前运行严格版本校验、拒绝 forced tag、校验 tag 指向 `GITHUB_SHA` 且该提交是 `origin/main` 的祖先，并完整复用 `quality-gate.yml`。`main` 是版本镜像的唯一来源分支。
- 以 ADR-0004 Release 镜像相同的确定性参数构建一次，对这个本地候选运行 `container-smoke.sh`，随后推送同一 tag；本地期望值来自 Buildx metadata 的 `containerimage.digest`，推送后与 registry manifest digest 比对。
- 推送前必须确认同名镜像 tag 不存在；查询与 push 标签必须相同。已存在包只接受结构化 `MANIFEST_UNKNOWN` 加上精确包路径的成功 tag listing；首次建包只在本工作流显式启用，并要求对 `yangphere/leanote` 的 manifest 与 tag-list 精确端点都返回结构化 `NAME_UNKNOWN`（若响应带 `detail.name`，还必须匹配）。认证、网络、JSON、权限、重定向或未知状态一律阻断。
- 工作流仅授予发布作业 `packages: write`，使用 `GITHUB_TOKEN`，不创建 GitHub Release、不发布 tarball、不自动部署生产。
- 轻量路径使用自己的 tag 并发锁，避免 rerun 互相竞态。它只接收无前缀 tag；受保护 Release 保持 `v*.*.*`，两条触发路径分离。

## Consequences

- ADR-0004 中“全部受保护门禁通过后才发布镜像”的规则被这个仅限版本镜像的决策取代；tarball 和 GitHub Release 仍必须通过 ADR-0004 的全部门禁。
- 无前缀 `2.0.1` 不触发 `release.yml`，也不生成其受保护 handoff；完整复用的七个质量作业与汇总仍必须通过。受保护路径使用带 `v` 的镜像标签，轻量发布不代表获得完整 Release 授权。
- 首次推送可以创建 private 包，但这不证明包已公开，也不证明匿名拉取成功。管理员必须显式改为 public，并从未认证客户端验证拉取。
- 任一次 push 后 read-back 失败都表示发布结果未知；不得以相同 tag 自动重试写入，必须先人工核对远端状态。

## References

- [GitHub Container registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Configure package access and visibility](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility)
