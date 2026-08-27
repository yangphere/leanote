# CI/CD 与可复现交付（F）— 技术设计

## 1. 工作流分层

使用两个显式 GitHub Actions 工作流：

- `.github/workflows/ci.yml`：PR 和分支 push 的质量门；不持有 package write 权限。
- `.github/workflows/release.yml`：仅 `v*` tag；先执行同等质量门，再创建 GitHub Release 和 GHCR image。

CI 按可并行 job 拆为 Go 单元/静态检查矩阵、MongoDB 8.0 集成、Node/build、Chromium E2E、package smoke、container smoke。release 只消费在同一 commit 上重新验证或可验证来源的制品，不以“分支曾经绿过”替代 tag 验证。

## 2. 固定工具链与服务

- Go：1.26、1.27 矩阵；`go.mod` 最低 1.26。
- Node：24.x，使用 `npm ci`；缓存键包含 lockfile 与 runner 架构。
- MongoDB：官方 8.0 service/container，等待真实 ping 后导入最小测试数据。
- 浏览器：`@playwright/test` 的 Chromium 是阻塞 E2E。Firefox/Safari 当前及前一主版本属于发布前 smoke 证据，Safari 必须在真实 Safari 环境验证。

所有 job 有超时和取消同分支旧 run 的 concurrency group。失败时只上传 allowlisted 脱敏测试摘要与脱敏服务健康摘要，最长保留 7 天；禁止上传原始 Playwright trace/HTML report、浏览器 screenshot/video、storage state、cookie、认证头、页面正文或未脱敏服务日志。摘要只包含工具/运行版本、阶段、页面 URL、资源路径、状态码、错误类别、服务 readiness/exit code 等可定位字段。唯一受控例外是手工 `record-export-pdf` 的 Golden JSON：仅在 `workflow_dispatch` 上传该单一文件，最长保留 7 天，不得包含账号、Cookie、认证头或页面正文。

## 3. 生成物和 tarball

Node job 先运行构建与测试，再执行 `git diff --exit-code`。这使生成资源仍可跟踪，但源码和 manifest 是唯一事实来源。

重写 `sh/package.sh`，只从显式 allowlist 收集应用二进制、配置样例、views、messages、public 运行资源和必要脚本；归档路径、排序和时间戳固定。CI 在干净临时目录解包，使用外部 MongoDB 启动并请求健康端点。发布同时生成 SHA-256 文件。

## 4. 容器镜像

多阶段 `Dockerfile` 分离 Node 资源构建、Go 编译和运行时。运行层只包含应用、静态资源、配置样例、CA/时区及 PDF 渲染所需的明确系统依赖；使用固定非 root UID/GID，入口不内置 MongoDB。

运行配置通过环境变量/挂载文件注入，至少声明文件存储与 `public/upload` 等实际持久化路径。container smoke 启动 MongoDB 8.0 和应用，验证健康、一次上传写入/重启后存在，以及真实 HTML→PDF 流程输出有效 PDF。首发 platform 固定 `linux/amd64`；arm64 与 PDF 依赖适配记录为 `MOD-002`。

## 5. 标签发布与权限

release workflow 验证 tag 形如 `vX.Y.Z`，且应用版本来源与 `X.Y.Z` 相同；版本只从一个机器可读来源生成，不在多个脚本手工维护。权限默认 `contents: read`，发布 job 单独获得 `contents: write` 与 `packages: write`，使用 GitHub 内建 token，不存长期 PAT。

镜像推送不可变版本 tag 和与策略一致的便利 tag；Release 上传 tarball、校验和及构建元数据。任一验证或推送失败都使 workflow 失败，不发布空附件、不继续生产部署。

## 6. 安全与回滚

构建上下文由 `.dockerignore` 排除 `.git`、数据库备份、上传、日志、测试输出、本地配置和密钥。工作流只使用测试专用 secret，日志不打印配置值。

回滚通过重新部署上一不可变 Release/镜像完成，本任务不自动控制生产环境。错误 tag 的 Release/镜像由维护者按 GitHub 权限流程处理，不由 workflow 静默覆盖。
