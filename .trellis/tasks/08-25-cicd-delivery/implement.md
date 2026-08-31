# CI/CD 与可复现交付（F）— 执行计划

## Global Constraints

- PR/push 只验证；仅精确 `vX.Y.Z` tag 发布，永不自动部署。
- Go 1.26.7/1.27.0、Node 24.20.0、MongoDB 8.0（镜像 digest）是固定门槛。
- 首发容器仅 `linux/amd64`，必须非 root 且完整 PDF 功能可用。
- F 只能在 `08-25-revel-migration` 的真实验收闭合、`08-25-frontend-libs` 组合门禁完成并归档后
  激活；父任务子项 `[n/n done]` 不足以解除依赖。规格审核阶段不得运行 `task.py start`。
- 本任务 PRD、设计和研究材料中的精确 tag 规则优先于父任务/ADR 的历史 `v*` 描述；失败摘要和
  浏览器矩阵必须分别遵守 `research/ci-failure-summary-schema.md` 与
  `research/release-matrix-contract.md`，不得创建占位证据。

### Task 0：依赖和启动前阻断核验

- [ ] 重新读取两个依赖的归档 `task.json`、PRD、design、implement、check 和真实 workflow 证据；
      C-b 必须不再命中 `app/cmd`、`github.com/revel/*` 或 `revel.` 交付路径，frontend-libs 必须
      包含同一最终 commit 的真实 Chrome/Edge/Firefox/Safari 当前及前一主版本八行记录。
- [ ] 解决并记录 Q-F1 版本事实源、Q-F2 健康端点归属、Q-F3 release 并发/重试策略；任一阻断
      问题未确认时停留 planning，不伪造环境变量、版本或健康语义。

### Task 1：整理可复用 CI 命令与版本来源

**Files:**
- Modify: `package.json`、`package-lock.json`（仅在 Q-F1 选择 Node 来源时）
- Create/Modify: focused test, version-check and smoke scripts under `scripts/`
- Modify: the selected canonical application version source and release metadata

- [ ] 为 Go 单元/静态检查、Mongo 集成、Node build/test、Chromium E2E、生成物漂移、package smoke 和 PDF smoke 提供非交互命令，并定义真实 Chrome、Edge、Firefox、Safari 当前及前一主版本 release smoke 的记录命令或人工环境入口。
- [ ] 按 Q-F1 固定单一机器可读版本来源，增加精确 `vX.Y.Z` tag、应用显示/health、tarball 和
      OCI label 一致性检查；重复资产必须失败而不覆盖。
- [ ] 证明命令在干净 checkout、无用户 profile 和仅声明 `leanote_test` 服务的环境运行；测试发现
      为零、服务 readiness 未确认或 cleanup 失败均返回非零。
- [ ] 为每类命令设置合理超时并保留失败退出码，不用包装脚本吞错。

### Task 2：建立 PR/push 质量门

**Files:**
- Create: `.github/workflows/quality-gate.yml`、`.github/workflows/ci.yml`
- Modify/Delete: `.github/workflows/regression-baseline.yml`（归并为唯一 quality-gate，禁止重叠触发）
- Delete: `.travis.yml`
- Modify: repository status-check documentation

- [ ] `ci.yml` 只在 `pull_request`、`dev` 和当前远程默认分支 `master` push 触发；quality-gate
      设置最小只读权限、固定 action SHA、concurrency cancel 和显式 job timeout。
- [ ] 建立 Go 1.26.7/1.27.0 矩阵与 MongoDB 8.0 digest 集成 job，导入最小隔离夹具并运行
      Golden/USN；`go test -list`/等价 discovery 证明目标用例数非零。
- [ ] 用 Node 24.20.0 + `npm ci` 运行 build、JS 测试、Chromium E2E，再执行
      `git diff --exit-code` 与空 `git status --porcelain --untracked-files=all`。
- [ ] 每个 job 的最后一步使用 `if: always()` 生成一条符合 `research/ci-failure-summary-schema.md`
      的摘要；摘要中的逻辑 job ID 固定为 `go-1_26_7`、`go-1_27_0`、`mongo-8_0`、`node-build`、
      `chromium-e2e`、`package-smoke`、`container-smoke`、`summary`，独立汇总 job 使用所有 7 个质量门
      生产 job 的 `needs` + `if: always()` 校验这 8 个逻辑 ID 的摘要完整性。checkout、
      setup、服务启动等早期失败必须生成 `job_not_started` 或真实失败类别的 fallback，缺摘要、schema
      校验失败、目标测试数为零或 cleanup 失败都返回非零。只上传 allowlisted 脱敏 job 摘要，服务健康
      信息只能来自摘要的 `service` 字段，不得上传独立健康文件；明确删除且禁止上传原始 Playwright trace/HTML report、截图/视频、storage state、cookie、
      认证头、页面正文和未脱敏服务日志，artifact 保留期不超过 7 天。唯一受控例外是手工
      `record-export-pdf` 上传单一 Golden JSON，保留期同样不超过 7 天。成功时不上传无意义的大型中间物。

### Task 3：重建 tarball 交付路径

**Files:**
- Modify: `sh/package.sh`
- Create/Modify: package allowlist and smoke scripts under `scripts/`
- Modify: `.gitignore` as required for local package output only

- [ ] 用显式 allowlist 取代依赖旧 Gulp/Revel CLI 的复制逻辑，固定文件排序、路径和时间戳。
- [ ] allowlist 只包含迁移后的 Go binary、`conf/app.conf-default`、views/messages/public 和必要
      脚本；排除 `conf/app.conf`、`mongodb_backup`、`files`、`public/upload` 内容、node_modules、
      日志、缓存和测试输出。
- [ ] 使用 tag commit 的 `SOURCE_DATE_EPOCH`、Go `-trimpath -buildvcs=false`、稳定归档排序/owner/
      group/mode/mtime 与无时间 gzip，产出 `leanote-vX.Y.Z-linux-amd64.tar.gz`、`.sha256` 和元数据。
- [ ] 在干净临时目录解包，注入已确认的 prod config/外部 Mongo URL/secret，调用 Q-F2 健康端点并
      验证启动、上传卷和真实 PDF；连续两次完整 SHA-256 必须一致。

### Task 4：构建并验证 Linux/amd64 镜像

**Files:**
- Create: `Dockerfile`、`.dockerignore`
- Create/Modify: `scripts/container-smoke.*`
- Modify: deployment/configuration documentation

- [ ] 建立 Node/Go/运行时多阶段镜像，运行层使用固定非 root 用户且不包含编译工具。
- [ ] 明确 Q-F2 健康检查、外部 MongoDB/secret 注入、固定非 root UID/GID、`files/` 与 `public/upload/`
      卷和固定版本的 PDF 系统依赖；缺配置不能回退 localhost 或仓库默认 secret。
- [ ] 固定构建 platform 为 `linux/amd64`、基础镜像 digest，使用 tag commit 的 `SOURCE_DATE_EPOCH`
      固定 OCI `created` 及 version/revision/source labels；显式固定 provenance/attestation/SBOM
      开关（内容或时间不可证明确定时使用 `--provenance=false --sbom=false` 或等价参数），并把构建参数和最终
      image digest 写入元数据。扫描上下文和最终层不含凭据或用户数据。
- [ ] container smoke 验证 HTTP+Mongo 健康、上传跨重启持久化和真实 PDF `%PDF-` 签名/非空内容；清理
      失败阻断。
- [ ] 将 arm64/PDF 适配后续工作链接到 `docs/modernization-backlog.md#mod-002`。

### Task 5：建立 tag 发布流程

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: release/deployment documentation

- [ ] 仅匹配 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`（即禁止前导零、预发布和
      build metadata 的 `vX.Y.Z`）tag，先验证 semver、应用版本和完整质量门。
- [ ] 在同一 commit 调用 quality-gate；以 digest/sha256
      校验 gate 产物，任何失败都不创建 Release。
- [ ] 以最小 `contents: write` 权限创建 GitHub Release，上传固定命名 tarball、SHA-256 和构建元数据；
      已有 Release/资产时失败而不覆盖。
- [ ] 以最小 `packages: write` 权限登录 GHCR，推送 Linux/amd64 镜像并记录最终 digest；不使用长期 PAT。
- [ ] 验证非 tag run 无发布权限与发布步骤，workflow 中不存在生产部署 job。
- [ ] 按 Q-F3 用测试 tag 演练发布、重试和错误资产人工删除边界；release 不允许被 concurrency 取消。

### Task 6：端到端验收与文档收口

- [ ] 在 PR 模式运行全部 job，确认每个必需门槛真实执行且没有零测试假绿。
- [ ] 在测试 tag 演练 Release/GHCR，下载 tarball 和 pull 镜像后独立复验 SHA-256、健康、上传卷和 PDF。
- [ ] 检查工作流唯一触发、权限、固定 action/image 引用、缓存、timeout、artifact schema/留存和日志脱敏。
- [ ] 用 `docs/modernization/browser-smoke/release-matrix.json` 按
      `research/release-matrix-contract.md` 校验每个发布候选的真实四浏览器当前/前一版本八行唯一键
      记录，确认同一 commit、产品/完整版本、OS、覆盖范围、认证/错误/资源门禁、执行时间和结果齐全；
      真实 Safari 必须存在，Chromium/WebKit 不得替代，禁止占位记录。
- [ ] 更新 README/部署说明中的支持矩阵、配置、卷、架构限制及“无自动生产部署”边界。
- [ ] 复核 diff 无 `.travis.yml` 残留、旧交付脚本假设或敏感文件。

## Review Blockers Before `task.py start`

- [ ] Q-F1 版本事实源已由用户确认并在 PRD/design 中同步。
- [ ] Q-F2 健康端点及 DB readiness 归属已由 C-b 提供或明确回退方案；F 不把 `/login` 当健康端点。
- [ ] Q-F3 release 并发、重复资产和人工恢复边界已确认。
- [ ] 两个依赖的真实完成证据已重新核验；否则保持 planning。

## Rollback Point

CI 质量门、tarball 与容器可分别回退到前一可用提交，但 release workflow 不得在门槛失败时降级发布。生产回滚不在自动化范围内，由维护者选择上一不可变版本。
