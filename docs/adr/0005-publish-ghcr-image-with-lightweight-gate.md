# ADR-0005：通过轻量门禁发布 GHCR 版本镜像

- **状态**：Accepted
- **日期**：2026-10-04

## Context

生产 Compose 需要可匿名拉取的版本镜像，但 ADR-0004 的受保护 Release 工作流依赖尚未提供的 delivery runner、浏览器证据和人工批准环境。继续把 GHCR 镜像与 tarball、GitHub Release 绑定，会让 Compose 没有可用的官方镜像。

GitHub 官方文档说明，仓库内 GitHub Actions 工作流可以用 `GITHUB_TOKEN` 发布与该仓库关联的容器包；首次发布会创建包，默认可见性为 private。Dockerfile 已包含 `org.opencontainers.image.source=https://github.com/yangphere/leanote`，因此无需先在网页预创建空包，但首次发布后仍需管理员把包改为 public，匿名拉取才成立。

## Decision

- 新增 `.github/workflows/docker-image.yml`，响应无前缀版本 tag push，并提供 main 上针对已有固定 tag 的显式恢复 dispatch；严格 `X.Y.Z` Git tag 发布 `ghcr.io/yangphere/leanote:X.Y.Z` 的 `linux/amd64` 镜像。用户指定首发 Git tag 和镜像 tag 均为 `2.0.1`，随后要求同时更新 `latest` 为相同 manifest digest 的别名。
- 发布前运行严格版本校验、拒绝 forced tag、校验 tag 指向候选 SHA 且该提交是 `origin/main` 的祖先，并完整复用 `quality-gate.yml`。`main` 是版本镜像的唯一来源分支。
- 以 ADR-0004 Release 镜像相同的确定性参数构建一次，同时导出 loaded 候选与 OCI archive。先校验 archive 的原始 manifest SHA/config 与 Buildx metadata、loaded config 一致，对本地候选运行 `container-smoke.sh`，再用 `skopeo copy --preserve-digests` 发布同一个 archive。校验版本 raw manifest/config 后，通过共用版本晋升规则才复制到 latest 并做同样校验。真实 registry 复现证明 Docker Engine load/push 会重新序列化 manifest，使 Buildx digest 无法直接与 Engine push digest 比较；这里保留原始 artifact 字节，保持同一镜像与可比较的 digest。
- 推送前必须确认同名镜像 tag 不存在；查询与 push 标签必须相同。已存在包只接受结构化 `MANIFEST_UNKNOWN` 加上精确包路径的成功 tag listing。首发已完成，普通工作流不再启用 `ALLOW_INITIAL_PACKAGE_CREATE`，包缺失时明确失败；不能在误删或迁移后静默重建包。底层 helper 保留独立的显式首发策略，其结构化、身份绑定的双 404 条件不放宽；需要重新建包时另行审核该操作。认证、网络、JSON、权限、重定向或未知状态一律阻断。
- 工作流仅授予发布作业 `packages: write`，使用 `GITHUB_TOKEN`，不创建 GitHub Release、不发布 tarball、不自动部署生产。
- 轻量路径使用自己的 `docker-image-latest` 并发锁，所有版本的 push 与 dispatch 共锁，保护共享 latest。使用 `queue: max` 与 `cancel-in-progress: false` 保留最多 100 个等待运行，避免默认 single pending 被后续 tag 替换取消；满队列的新运行仍可能被取消。队列顺序不能代替版本晋升规则。它只接收无前缀版本 tag；受保护 Release 保持 `v*.*.*`，两条触发路径分离。
- 已推送 tag 的恢复不移动 tag。main 执行器校验明确输入的原提交、原 Docker image push run 与 attempt，通过 API 身份与原七个质量作业/汇总，并用共享 validator 校验下载 summary 的来源 provenance。main 执行器仍通过完整质量门，实际构建/smoke 使用原候选，registry helper 使用修复执行器。原 run 因 publish 失败整体为 failure 不否定已通过的质量证据。
- 已存在版本增加 latest 使用显式 `update_latest` 操作，要求预期 registry/config digest，拉取版本 digest 并验证平台、版本、revision、source 后 smoke，通过与新发布相同的晋升规则才复制该 digest 到 latest；不覆盖或重新构建版本。版本标签保持不可变，latest 是成功晋升的最高严格 `X.Y.Z` 版本别名。
- 晋升前用 Skopeo 成功的 `list-tags` 结果确认 Repository 与包标签身份；存在 latest 时读取其 OCI config 的 `org.opencontainers.image.version`，复用共享版本模块按数值逐段比较。候选低于或等于当前版本时跳过 latest 写入，并明确输出原因；仍允许发布缺失的旧版不可变版本。只有成功的包列表确认无 latest 时初始化别名。读取错误、无效元数据或缺失版本标签均阻断，不能把请求失败解释为 latest 不存在。

## Consequences

- ADR-0004 中“全部受保护门禁通过后才发布镜像”的规则被这个仅限版本镜像的决策取代；tarball 和 GitHub Release 仍必须通过 ADR-0004 的全部门禁。
- 无前缀 `2.0.1` 不触发 `release.yml`，也不生成其受保护 handoff；完整复用的七个质量作业与汇总仍必须通过。受保护路径使用带 `v` 的镜像标签，轻量发布不代表获得完整 Release 授权。
- 首次推送可以创建 private 包，但这不证明包已公开，也不证明匿名拉取成功。管理员必须显式改为 public，并从未认证客户端验证拉取。
- 任一次 push 后 read-back 失败都表示发布结果未知；不得以相同 tag 自动重试写入，必须先人工核对远端状态。
- 队列溢出或人工取消后的重试须先检查版本镜像是否已存在。未开始 publish 且版本不存在的运行可在 UI re-run；版本已存在时不得重推，只按原证据/摘要走显式 latest 恢复。相同或更旧版本不会修改已晋升的 latest。
- Tag push 使用该 tag 所指提交中的工作流，修改 main 不会追溯加固历史工作流。版本晋升、排队和禁建包规则仅适用于包含本次修复的工作流；禁止向不含修复的旧提交补推版本 tag。恢复 dispatch 必须在已合入修复的 main 上执行。若需强制约束历史工作流，须另行设计远端 tag 规则或可信固定发布入口，本次不修改远端规则或旧 tag。

## References

- [GitHub Container registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Configure package access and visibility](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility)
- [Control workflow concurrency and pending queues](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
- [How triggering a workflow works](https://docs.github.com/en/actions/concepts/workflows-and-actions/workflows)
