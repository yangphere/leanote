# CI/CD 与可复现交付（F）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在后端与前端现代化轨道均完成后，用 GitHub Actions 建立可复现的 PR/push 质量门和 `v*` 标签发布流程，产出 GitHub Release tarball 与 GHCR Linux/amd64 镜像；不自动部署生产环境。

## Dependencies

- 依赖 `08-25-revel-migration` 和 `08-25-frontend-libs` 完成。
- 复用 `08-25-regression-baseline` 的 Golden、USN、MongoDB 8.0 初始化和页面 smoke。

## Requirements

- **R-F1** PR 和 `dev`/默认分支 push 运行：Go 1.26/1.27 矩阵、MongoDB 8.0 集成测试、Node 24.x 构建与 JS 测试、Chromium 阻塞 E2E、生成物漂移检查、打包和 Docker smoke。
- **R-F2** Go module 的最低版本保持 1.26；任一受支持工具链失败都阻断。Node 只支持 24.x LTS，必须使用 `npm ci` 和锁文件。
- **R-F3** 构建链运行后执行 `git diff --exit-code`，证明提交的前端生成物、模板和清单来自唯一源码且无漂移。
- **R-F4** `v*` tag 触发发布：验证 tag/应用版本一致，复用已通过的质量门，生成可复现 tarball、校验和与变更说明，并上传 GitHub Release。
- **R-F5** 同一 tag 构建并发布 `ghcr.io/yangphere/leanote` 镜像，首期只支持 `linux/amd64`。镜像必须非 root 运行、外置 MongoDB、声明持久化目录，并包含可工作的完整 PDF 功能。
- **R-F6** 发布工作流只使用最小权限的 `GITHUB_TOKEN`；不得写入真实数据库、SMTP、Cookie、`app.secret` 或生产部署凭据。
- **R-F7** 失败必须阻止发布并保留日志/测试产物；不自动跳过 Mongo、E2E、PDF 或生成物校验，不生成“成功”占位制品。
- **R-F8** 删除或替换 `.travis.yml` 与依赖旧 Gulp/Revel CLI 的交付路径；`sh/package.sh` 保留为可从锁定工具链调用的跨环境打包入口。

## Acceptance Criteria

- [ ] 新 PR/push 工作流在 Go 1.26 与 1.27 均通过编译、vet/允许基线、Go 测试和 MongoDB 8.0 集成测试。
- [ ] Node 24.x 执行 `npm ci`、build、JS 测试、Chromium E2E，并在重建后以 `git diff --exit-code` 证明零漂移。
- [ ] CI 构建 tarball 并在干净临时目录解包，应用可启动并通过健康检查；归档不含凭据、用户数据或本地生成物。
- [ ] CI 构建 Linux/amd64 镜像，以非 root 用户启动，连接外部 MongoDB 8.0，通过健康检查、上传持久化和真实 PDF 生成 smoke。
- [ ] 一个测试 `v*` tag 可创建 GitHub Release，附带 tarball 和 SHA-256 校验文件，并把同版本镜像推到 GHCR。
- [ ] 非 tag 的 PR/push 不发布 Release/镜像；任何工作流都不执行生产部署。
- [ ] 工作流权限、缓存键、并发取消、超时和 artifact 保留期显式配置，失败日志足以定位阶段。
- [ ] `.travis.yml` 和旧交付假设已移除，README/部署文档说明版本矩阵、镜像架构、外部配置、卷和手工部署边界。

## Out of Scope

- 自动部署、环境审批、回滚生产实例或管理生产密钥。
- Linux/arm64 多架构镜像；后续工作记录在 `docs/modernization-backlog.md`。
- Windows/macOS 容器镜像和托管 MongoDB。
- 将 GitHub Actions 替换为通用发布平台抽象。
