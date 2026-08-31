# CI/CD 与可复现交付（F）— 执行计划

## Global Constraints

- PR/push 只验证；仅精确 `vX.Y.Z` tag 发布，永不自动部署。release tag 按 tag/ref 使用独立 concurrency
  group，`cancel-in-progress: false`；已有 Release、资产或镜像 tag 时失败且不覆盖，触发 tag/ref 移动
  或 force-update 必须失败，失败重试/删除只能按明确人工边界执行。所有需要服务 readiness 的 smoke 统一调用
  C-b 的无认证 `GET /healthz`：HTTP+Mongo ready 为 `200`，否则为 `503`；不得使用 `/login` 页面 `200` 代替。
- Go 1.26.7/1.27.0、Node 24.20.0、MongoDB 8.0（镜像 digest）是固定门槛。
- 首发容器仅 `linux/amd64`，必须非 root 且完整 PDF 功能可用。
- F 只能在 `08-25-revel-migration` 的真实验收闭合、`08-25-frontend-libs` 组合门禁完成并归档后
  激活；父任务子项 `[n/n done]` 不足以解除依赖。规格审核阶段不得运行 `task.py start`。
- 本任务 PRD、设计和研究材料中的精确 tag 规则优先于父任务/ADR 的历史 `v*` 描述；失败摘要和
  浏览器矩阵必须分别遵守 `research/ci-failure-summary-schema.md` 与
  `research/release-matrix-contract.md`，不得创建占位证据。
- 生产配置只消费 `research/cb-production-config-contract.md`：显式
  `-conf /etc/leanote/app.conf -runMode prod`、只读 `0440`，Mongo 使用
  `MONGODB_URL`/`db.urlEnv`，secret 使用 `LEANOTE_APP_SECRET`/`app.secret`，并要求
  `db.dbname` 与 URI 数据库路径一致且非 `leanote_test`。运行时注入先于挂载文件解析但
  不覆盖冲突；参数/配置错误使用研究材料固定的稳定错误码并在 bind/dial/healthz 前退出 `78`，Mongo ping
  未就绪仅由 `/healthz` 返回 `503`。禁止所有默认/localhost/host-port/未声明别名回退，
  日志和 artifact 不得包含配置值、凭据或完整 URI。

### Task 0：依赖和启动前阻断核验

- [ ] 重新读取两个依赖的归档 `task.json`、PRD、design、implement、check 和真实 workflow 证据；
      C-b 必须不再命中 `app/cmd`、`github.com/revel/*` 或 `revel.` 交付路径，frontend-libs 必须
      提供真实 Chrome/Edge/Firefox/Safari 当前及前一主版本的 smoke 定义、执行能力和
      `browser-release-matrix-v1` 证据 workflow 契约。
- [x] Q-F1 已确认并记录：版本只读 `package.json` 顶层 `version`，Go release 构建通过 linker 注入同一值。
- [x] Q-F2 已确认并记录：C-b 提供无认证 `GET /healthz`，HTTP+Mongo ready 返回 `200`，否则 `503`，响应不泄露配置；C-b 实现证据仍需在 Task 0 复核。
- [x] Q-F3 已确认并记录：release 按 tag/ref 隔离 concurrency 且 `cancel-in-progress: false`；已有 Release、资产或镜像 tag 拒绝且不覆盖，触发 tag/ref 必须指向当前 commit 且不可 force-update，workflow 不自动重试/删除，人工恢复须有明确边界。
- [x] Q-F4 接口已确认并记录：C-b v1 固定 `/etc/leanote/app.conf`、`0440`、`MONGODB_URL`/`db.urlEnv`、
      `LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 与 URI 一致且非 `leanote_test`，运行时注入先于挂载文件
      解析但冲突不覆盖，配置错误以稳定错误码退出 `78`，Mongo 未就绪由 `/healthz` 返回 `503`；F 只消费接口表。
      C-b 实现证据按 `research/cb-production-config-evidence.md` 的 E1-E8 矩阵核验；Q-F5 已采用独立不可变
      `browser-release-matrix-v1` artifact 方案，任一阻断问题未确认时停留 planning，不伪造环境变量、版本、健康语义或矩阵记录。

### Task 1：整理可复用 CI 命令与版本来源

**Files:**
- Modify: `package.json`、`package-lock.json`（仅在 Q-F1 选择 Node 来源时）
- Create/Modify: focused test, version-check and smoke scripts under `scripts/`
- Modify: the selected canonical application version source and release metadata

- [ ] 为 Go 单元/静态检查、Mongo 集成、Node build/test、Chromium E2E、生成物漂移、package smoke 和 PDF smoke 提供非交互命令，并定义真实 Chrome、Edge、Firefox、Safari 当前及前一主版本 release smoke 的记录命令或人工环境入口。
- [ ] 为受保护真实浏览器证据 workflow 定义固定入口：仅消费 release tag 的 checkout SHA，生成一次
      `browser-release-matrix-v1` artifact（`release-matrix.json` + `provenance.json`），不得接受可改变
      ref、MongoDB 版本或门禁的输入；release validator 只消费当前 run/attempt 的 artifact。
- [ ] 从 `package.json` 顶层 `version` 读取唯一版本，先校验 `package-lock.json` 根 package
      的 `version` 完全一致，再增加精确 `vX.Y.Z` tag、Go linker 注入、应用显示/`GET /healthz`、tarball
      和 OCI label 一致性检查；重复资产或未注入版本必须失败而不覆盖。
- [ ] 证明命令在干净 checkout、无用户 profile 和仅声明 `leanote_test` 服务的环境运行；测试发现
      为零、服务 readiness 未确认或 cleanup 失败均返回非零。
- [ ] 为生产配置校验器消费 `research/cb-production-config-contract.md` 的唯一接口表：校验
      `-conf /etc/leanote/app.conf -runMode prod`、只读 `0440`、`MONGODB_URL`/`db.urlEnv`、
      `LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 与 URI 一致且非 `leanote_test`；运行时注入
      先于挂载文件解析但 literal/重复键/未声明来源冲突不覆盖。覆盖每个稳定错误码、退出 `78`、
      不 bind/不 dial、URI/secret 约束、无 localhost/host-port/默认回退和日志/artifact 脱敏。
- [ ] 为每类命令设置合理超时并保留失败退出码，不用包装脚本吞错。

### Task 2：建立 PR/push 质量门

**Files:**
- Create: `.github/workflows/quality-gate.yml`、`.github/workflows/ci.yml`
- Modify/Delete: `.github/workflows/regression-baseline.yml`（归并为唯一 quality-gate，禁止重叠触发）
- Delete: `.travis.yml`
- Modify: repository status-check documentation

- [ ] `ci.yml` 只在 `pull_request`、`dev` 和当前远程默认分支 `master` push 触发；quality-gate
      设置最小只读权限、固定 action SHA、concurrency cancel 和显式 job timeout；release tag 使用
      `cancel-in-progress: false` 且按 tag/ref 隔离；不得通过
      `workflow_dispatch` 的 Mongo/ref 输入绕过固定 8.0 fixture、checkout SHA 或质量门。
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
- [ ] 在干净临时目录解包，严格以 `-conf /etc/leanote/app.conf -runMode prod`、只读 `0440` 和
      `MONGODB_URL`/`LEANOTE_APP_SECRET` 占位接口注入外部 Mongo/secret；验证 `db.dbname` 与 URI 一致且非
      `leanote_test`，冲突/缺失/空值/公开默认/短 secret/非法 URI/localhost/host-port fallback 均以稳定错误码
      在 ready 前退出 `78`。调用 Q-F2 的 `GET /healthz` 并验证 HTTP+Mongo readiness、启动、上传卷和真实 PDF；
      ready 必须为 `200`，未 ready 必须为 `503`，且不得以 `/login` 页面 `200` 代替；连续两次完整 SHA-256 必须一致。

### Task 4：构建并验证 Linux/amd64 镜像

**Files:**
- Create: `Dockerfile`、`.dockerignore`
- Create/Modify: `scripts/container-smoke.*`
- Modify: deployment/configuration documentation

- [ ] 建立 Node/Go/运行时多阶段镜像，运行层使用固定非 root 用户且不包含编译工具。
- [ ] 消费 `research/cb-production-config-contract.md` 的唯一配置接口：只读 `/etc/leanote/app.conf`、
      `-runMode prod`、权限 `0440`、`MONGODB_URL`/`db.urlEnv`、`LEANOTE_APP_SECRET`/`app.secret`、
      `db.dbname` 与 URI 一致且非 `leanote_test`；运行时注入先于挂载文件解析但冲突不覆盖。缺失、空值、
      公开默认/短 secret、非法 URI、localhost/host-port fallback 均以稳定错误码和退出 `78` 在 ready 前失败。
      同时明确 Q-F2 `GET /healthz`（HTTP+Mongo ready=`200`，否则=`503`，不泄露配置）、固定非 root UID/GID、
      `files/` 与 `public/upload/` 卷和固定版本的 PDF 系统依赖；不得回退 `conf/app.conf`、默认 secret 或未声明别名。
- [ ] 固定构建 platform 为 `linux/amd64`、基础镜像 digest，使用 tag commit 的 `SOURCE_DATE_EPOCH`
      固定 OCI `created` 及 version/revision/source labels；显式固定 provenance/attestation/SBOM
      开关（内容或时间不可证明确定时使用 `--provenance=false --sbom=false` 或等价参数），并把构建参数和最终
      image digest 写入元数据。扫描上下文和最终层不含凭据或用户数据。
- [ ] container smoke 调用 `GET /healthz` 验证 HTTP+Mongo 健康（ready=`200`，否则=`503`）、上传跨重启
      持久化和真实 PDF `%PDF-` 签名/非空内容；不得用 `/login` 页面 `200`，清理失败阻断。
- [ ] 将 arm64/PDF 适配后续工作链接到 `docs/modernization-backlog.md#mod-002`。

### Task 5：建立 tag 发布流程

**Files:**
- Create: `.github/workflows/release.yml`
- Create: protected real-browser evidence workflow (called by `release.yml`)
- Modify: release/deployment documentation

- [ ] 仅匹配 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`（即禁止前导零、预发布和
      build metadata 的 `vX.Y.Z`）tag，先验证 semver、应用版本和完整质量门。
- [ ] 在同一 commit 调用 quality-gate；以 digest/sha256
      校验 gate 产物，任何失败都不创建 Release。
- [ ] 在同一 release run、同一 tag checkout SHA 上调用受保护的真实浏览器证据 workflow；只接受一次
      `browser-release-matrix-v1` artifact，按 `research/release-matrix-contract.md` 校验两文件 allowlist、
      provenance、run/attempt、tag ref、commit、矩阵 SHA-256、八行唯一键和真实 Safari 门禁；不从源码
      tracked 文件、旧 run/attempt 或占位记录补回矩阵；重试 attempt 的旧 artifact 视为重复并阻断，
      不由 workflow 自动删除。
- [ ] release 只下载当前 tag quality-gate run 的机器可读 artifact 清单及其 allowlisted 文件，
      交叉校验 workflow/run/attempt、tag ref、commit SHA、版本、文件名和完整 SHA-256；缺失、重复、
      跨 run/ref、部分上传或哈希不匹配均失败，不从历史 run 或 registry tag 回补。
- [ ] 以最小 `contents: write` 权限创建 GitHub Release，上传固定命名 tarball、SHA-256 和构建元数据；
      预检发现已有 Release 或资产时失败而不覆盖或自动删除；触发 tag/ref 必须指向当前 commit，任何移动或 force-update 都失败。
- [ ] 以最小 `packages: write` 权限登录 GHCR，推送 Linux/amd64 镜像并记录最终 digest；预检发现已有
      同名镜像 tag 时失败而不覆盖；不使用长期 PAT。
- [ ] 验证非 tag run 无发布权限与发布步骤，workflow 中不存在生产部署 job。
- [ ] 按 Q-F3 用测试 tag 演练并发等待、重复 Release/资产/镜像 tag 的预检失败和失败后人工恢复边界；
      release 不允许被 concurrency 取消，触发 tag/ref 不可移动，workflow 不自动重试、删除或覆盖。

### Task 6：端到端验收与文档收口

- [ ] 在 PR 模式运行全部 job，确认每个必需门槛真实执行且没有零测试假绿。
- [ ] 在测试 tag 演练 Release/GHCR，下载 tarball 和 pull 镜像后独立复验 SHA-256、`GET /healthz` 的
      HTTP+Mongo readiness、上传卷和 PDF；已有 Release、资产或镜像 tag 的预检必须失败，触发 tag/ref
      移动或 force-update 也必须失败，失败后的重试/删除仅按人工边界执行。
- [ ] 检查工作流唯一触发、权限、固定 action/image 引用、缓存、timeout、artifact schema/留存和日志脱敏。
- [ ] 从当前 release run 的 `browser-release-matrix-v1` artifact 按
      `research/release-matrix-contract.md` 校验每个发布候选的真实四浏览器当前/前一版本八行唯一键
      记录，确认同一 commit、产品/完整版本、OS、覆盖范围、认证/错误/资源门禁、执行时间和结果齐全；
      真实 Safari 必须存在，Chromium/WebKit 不得替代，禁止占位记录；同时校验 `provenance.json` 的
      artifact 名称、唯一性、run/attempt、tag ref 和矩阵 SHA-256，不能从 tracked 文件或其他 run 补回。
- [ ] 更新 README/部署说明，逐字引用 C-b v1 的 `/etc/leanote/app.conf`、`0440`、`MONGODB_URL`/`db.urlEnv`、
      `LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 匹配、优先级、稳定错误码/退出 `78` 与 fail-closed 规则，以及支持矩阵、卷、
      架构限制及“无自动生产部署”边界。
- [ ] 复核 diff 无 `.travis.yml` 残留、旧交付脚本假设或敏感文件。

## Review Blockers Before `task.py start`

- [x] Q-F1 版本事实源已由用户确认并在 PRD/design 中同步：`package.json` 顶层 `version`，当前为 `1.0.0`。
- [x] Q-F2 健康端点契约已确认：C-b 提供无认证 `GET /healthz`，HTTP+Mongo ready 返回 `200`，否则 `503`，响应不泄露配置；Task 0 仍须核验 C-b 的真实实现证据，F 不把 `/login` 当健康端点。
- [x] Q-F3 release 并发、重复资源和人工恢复边界已确认：按 tag/ref 隔离 concurrency、`cancel-in-progress: false`；已有 Release、资产或镜像 tag 拒绝且不覆盖，触发 tag/ref 必须指向当前 commit 且不可 force-update，workflow 不自动重试/删除，人工恢复须有明确记录与权限边界。
- [x] Q-F4 具体接口已确认并同步：`-conf /etc/leanote/app.conf -runMode prod`、只读 `0440`、
      `MONGODB_URL`/`db.urlEnv`、`LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 与 URI 一致且非 `leanote_test`，
      运行时注入先解析、冲突不覆盖，稳定错误码在 bind/dial/healthz 前以退出 `78` 失败，日志/artifact 脱敏。
- [ ] Q-F4 C-b 实现证据已提供并同步：`research/cb-production-config-evidence.md` 的 E1-E8 全部为 `PASS`，并有入口/部署、
      每个错误码与约束测试、进程级退出 `78`/不 bind/不 dial、fallback/完整 URI 日志静态检查及合法配置 package/container
      smoke 的 run/ref/commit/SHA-256 provenance；缺一仍不得激活。
- [x] Q-F5 浏览器矩阵采用已确认的独立不可变证明：release workflow 在 tag commit 上调用受保护真实
      浏览器证据 workflow，生成一次 `browser-release-matrix-v1` artifact（矩阵 + provenance），由
      validator 绑定当前 run/attempt、tag ref、commit 和 SHA-256；不进入源码提交、不自引用、不覆盖或删除。
- [ ] 两个依赖的真实完成证据已重新核验；否则保持 planning。

## Rollback Point

CI 质量门、tarball 与容器可分别回退到前一可用提交，但 release workflow 不得在门槛失败时降级发布。生产回滚不在自动化范围内，由维护者选择上一不可变版本。
