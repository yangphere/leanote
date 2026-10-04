# Research

## GitHub Container registry

- 2026-10-04 核对官方文档：<https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry>
  - 仓库内 GitHub Actions workflow 可以用 `GITHUB_TOKEN` 发布与该仓库关联的包。
  - 首次发布会创建包，默认可见性为 private；无需预先创建空包。
  - 从 workflow 用 `GITHUB_TOKEN` 发布会自动关联 workflow 所在仓库；`org.opencontainers.image.source` 也是官方建议的关联元数据，当前 Dockerfile 已提供。
- 2026-10-04 核对可见性文档：<https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility>
  - 管理员需在包设置中把包改为 public 才能匿名拉取。
  - public 变更不可逆，必须作为人工步骤保留。

## Implementation consequence

原 PRD 的“首次发布前必须管理员预创建包”不准确，已改为工作流内显式且 fail-closed 的首次创建分支。真实 tag push、包创建、public 设置和匿名 pull 仍未执行，不能从文档或单元测试推断成功。

Docker Registry/GitHub 文档没有承诺 `NAME_UNKNOWN.detail.name` 一定存在。因此 helper 把“两次请求均发往禁止重定向的精确包端点”作为路径绑定；若 GHCR 返回 `detail.name` 则额外要求精确匹配，但不把可选字段当成首次发布的必要条件。GHCR 对首个真实 tag 的响应形状仍为 `unrun`，若实际响应不同将 fail closed。
