# CI/CD 与可复现交付（F）— 执行计划

## Global Constraints

- PR/push 只验证；`v*` tag 才发布，永不自动部署。
- Go 1.26/1.27、Node 24.x、MongoDB 8.0 是固定门槛。
- 首发容器仅 `linux/amd64`，必须非 root 且完整 PDF 功能可用。

### Task 1：整理可复用 CI 命令与版本来源

**Files:**
- Modify: `package.json`、`go.mod`
- Create/Modify: focused test and smoke scripts under `scripts/`
- Modify: application version source and package metadata

- [ ] 为 Go 单元/静态检查、Mongo 集成、Node build/test、Chromium E2E、生成物漂移、package smoke 和 PDF smoke 提供非交互命令。
- [ ] 固定单一机器可读版本来源，增加 tag 与应用版本一致性检查。
- [ ] 证明命令在干净 checkout、无用户 profile 和仅声明服务的环境运行。
- [ ] 为每类命令设置合理超时并保留失败退出码，不用包装脚本吞错。

### Task 2：建立 PR/push 质量门

**Files:**
- Create: `.github/workflows/ci.yml`
- Delete: `.travis.yml`
- Modify: repository status-check documentation

- [ ] 配置 PR 与目标分支 push 触发、最小只读权限、concurrency cancel 和 job timeout。
- [ ] 建立 Go 1.26/1.27 矩阵与 MongoDB 8.0 集成 job，导入最小夹具并运行 Golden/USN。
- [ ] 用 Node 24.x + `npm ci` 运行 build、JS 测试、Chromium E2E，再执行 `git diff --exit-code`。
- [ ] 失败时上传测试报告、浏览器 trace/screenshot 和服务日志，成功时不上传无意义的大型中间物。

### Task 3：重建 tarball 交付路径

**Files:**
- Modify: `sh/package.sh`
- Create/Modify: package allowlist and smoke scripts under `scripts/`
- Modify: `.gitignore` as required for local package output only

- [ ] 用显式 allowlist 取代依赖旧 Gulp/Revel CLI 的复制逻辑，固定文件排序、路径和时间戳。
- [ ] 排除配置密钥、Mongo 数据/备份、上传、日志、缓存与测试输出。
- [ ] 在干净临时目录解包，连接 MongoDB 8.0，启动应用并请求健康端点。
- [ ] 连续打包两次比较内容摘要，并生成 SHA-256 校验文件。

### Task 4：构建并验证 Linux/amd64 镜像

**Files:**
- Create: `Dockerfile`、`.dockerignore`
- Create/Modify: `scripts/container-smoke.*`
- Modify: deployment/configuration documentation

- [ ] 建立 Node/Go/运行时多阶段镜像，运行层使用固定非 root 用户且不包含编译工具。
- [ ] 明确外部 MongoDB 配置、健康检查、文件/上传卷和 PDF 系统依赖。
- [ ] 固定构建 platform 为 `linux/amd64`，扫描上下文和最终层不含凭据或用户数据。
- [ ] container smoke 验证健康请求、上传跨重启持久化和真实 PDF 输出签名/内容。
- [ ] 将 arm64/PDF 适配后续工作链接到 `docs/modernization-backlog.md#mod-002`。

### Task 5：建立 tag 发布流程

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: release/deployment documentation

- [ ] 仅匹配 `v*` tag，先验证 semver、应用版本和完整质量门。
- [ ] 以最小 `contents: write` 权限创建 GitHub Release，上传 tarball、SHA-256 和构建元数据。
- [ ] 以最小 `packages: write` 权限登录 GHCR，推送不可变版本的 Linux/amd64 镜像。
- [ ] 验证非 tag run 无发布权限与发布步骤，workflow 中不存在生产部署 job。
- [ ] 用测试 tag 演练发布，记录失败恢复和人工删除错误发布的边界。

### Task 6：端到端验收与文档收口

- [ ] 在 PR 模式运行全部 job，确认每个必需门槛真实执行且没有零测试假绿。
- [ ] 在测试 tag 演练 Release/GHCR，下载 tarball和 pull 镜像后独立复验健康、上传和 PDF。
- [ ] 检查工作流权限、缓存、timeout、artifact 留存和日志脱敏。
- [ ] 更新 README/部署说明中的支持矩阵、配置、卷、架构限制及“无自动生产部署”边界。
- [ ] 复核 diff 无 `.travis.yml` 残留、旧交付脚本假设或敏感文件。

## Rollback Point

CI 质量门、tarball 与容器可分别回退到前一可用提交，但 release workflow 不得在门槛失败时降级发布。生产回滚不在自动化范围内，由维护者选择上一不可变版本。
