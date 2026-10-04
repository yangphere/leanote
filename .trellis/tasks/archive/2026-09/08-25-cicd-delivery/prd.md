# CI/CD 与可复现交付（F）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在后端与前端现代化轨道均完成后，用 GitHub Actions 建立可复现的 PR/push 质量门和精确 `vX.Y.Z` 标签发布流程，产出 GitHub Release tarball 与 GHCR Linux/amd64 镜像；不自动部署生产环境。

## Dependencies

- 正式发布依赖 `08-25-revel-migration` 和 `08-25-frontend-libs` 完成，并以归档任务的验收清单、实现/检查材料和真实 workflow 证据重新核验；父任务的 `[n/n done]` 子项计数不能替代完成状态。Q-E1 等待模式下，受保护浏览器 producer 可在 E 归档前运行一次仅供 E 验收的 tag 预检；该预检不是 F 发布完成或任务依赖。
- `08-25-revel-migration` 的当前源码必须已移除 `app/cmd`、旧 Revel 交付路径及其生产依赖；仅 `task.json.status=completed` 不构成 F 的启动证据。
- `08-25-frontend-libs` 必须在 F 创建 Release/GHCR 前完成协调父任务的组合门禁并归档；其真实四浏览器当前/前一主版本执行能力和证据生成契约缺失时，F 正式发布保持 blocked。等待模式的 tag 预检 artifact 可在 E 归档前生成，但不得创建 Release/GHCR 或改变 F 任务状态。
- 复用 `08-25-regression-baseline` 的 Golden、USN、权限和页面 smoke 契约。G 的历史 fixture 是 MongoDB 5.0；F 的 MongoDB 8.0 运行必须使用已由 `08-25-mongo-driver-migration` 迁移后的 harness，不得把 G 描述为 8.0 初始化来源。

## Readiness Gate

按现代化父任务的轨道顺序，F 是后端（Go → Revel 1.1 → Mongo 驱动 → C-b）与前端（构建链 → jQuery → Bootstrap → TinyMCE）两条轨道汇合后的候选叶。正式发布 ready 的必要条件是：本任务为叶、状态为 `planning`，并且 `meta.depends_on` 的每个任务都已归档完成且有可核验的实现、验收、检查和真实 workflow 证据。Q-E1 等待模式的 tag 预检只验证发布输入，不改变该 ready 条件，也不要求 F 在 E 归档前创建 Release/GHCR。当前 `08-25-frontend-libs` 仍为 planning，归档的 C-b 材料仍有未闭合验收且源码保留 Revel 命中，因此 F 正式发布不可激活；审核阶段不运行 `task.py start`。

## Specification Precedence

本任务的精确 tag、版本一致性、失败处理和发布验收契约以本 PRD、`design.md`、`implement.md` 及其研究材料为准。父任务设计和 ADR-0004 中历史遗留的 `v*` 表述只保留为背景，不能放宽本任务的精确规则；实现前必须按本任务规则校验并在任务收口时记录该文档冲突已被识别。

## Resolved Decisions

- **Q-F1 已确认（2026-08-31）**：`package.json` 顶层 `version` 是唯一机器可读版本事实源；当前值为 `1.0.0`，`package-lock.json` 根 package 的 `version` 必须与其一致。Node 构建、release 校验、tarball/Release 命名和 OCI labels 均直接读取该字段。Go release 构建通过 linker 注入 `github.com/yangphere/leanote/app/service.BuildVersion`，`ConfigService.GetVersion()` 只返回该变量；变量未注入时固定为 `dev`，仅允许显式开发/测试场景，release、package 和 container smoke 必须拒绝它，不得返回第二个硬编码版本。
- **Q-F2 已确认（2026-08-31）**：C-b 必须提供无需认证的 `GET /healthz`。仅当 HTTP 服务已监听且 MongoDB ping 成功时返回 `200`，响应为 `Content-Type: application/json; charset=utf-8` 和精确正文 `{\"status\":\"ready\"}\n`；任一条件未满足返回 `503`，正文为 `{\"status\":\"not_ready\"}\n`。响应正文和响应头不得泄露配置值、凭据、用户数据或版本字段。CI、打包 smoke 和容器 smoke 必须探测该端点及其状态码，不得用 `/login` 页面 `200` 代替 HTTP+Mongo readiness 证明；C-b 的实际实现和验收仍属于 F 的依赖门禁。
- **Q-F3 已确认（2026-08-31）**：release tag 使用按 tag/ref 隔离的 concurrency group，`cancel-in-progress: false`；并发运行等待而不取消先行运行。发布预检发现已有同名 GitHub Release、Release 资产或 GHCR 镜像 tag 时必须失败；触发 tag/ref 必须保持不可移动并指向当前 commit，任何 force-update 都必须失败。禁止覆盖、自动删除或从其他 run/registry tag 补偿。失败后的重试、错误资产删除或人工恢复只能由维护者在明确记录的人工边界内执行，workflow 不得自动改变这些边界。
- **Q-F4 已确认并收敛（2026-08-31）**：C-b v1 的唯一生产入口是显式 `-conf /etc/leanote/app.conf -runMode prod`；缺失/非 canonical 参数分别以 `CONFIG_PATH_INVALID`/`CONFIG_RUN_MODE_INVALID` 拒绝。该文件必须为只读 regular file，部署权限固定为 `0440`。Mongo 连接只接受 `MONGODB_URL`，由 prod section 的 `db.urlEnv=${MONGODB_URL}` 占位引用提供；secret 只接受 `LEANOTE_APP_SECRET`，由 `app.secret=${LEANOTE_APP_SECRET}` 占位引用提供；`db.dbname` 必须存在、与 URI 数据库路径一致且不得为 `leanote_test`。校验顺序固定为先读取唯一配置文件并验证 active `[prod]` 结构，再解析两个环境值；“运行时注入优先”只表示敏感值的唯一来源，不表示静默覆盖或改变文件结构校验顺序。literal 值、重复键、未声明别名或其他来源造成的冲突必须失败。缺失/不可读配置、section/键形态错误、环境值缺失或为空、公开默认/短 secret、非法 Mongo URI 或数据库名均须在 HTTP bind/listen、Mongo dial/ping 和 `/healthz` 可达前以稳定错误码失败并退出状态 `78`；有效配置但 Mongo ping 失败按 Q-F2 返回 `503`。禁止回退 `conf/app.conf`、`conf/app.conf-default`、localhost、host/port 组合或 `leanote_test`；日志与 artifact 只能保留错误码、非敏感键名和 `run_mode=prod`，不得包含配置值、凭据、完整 URI 或环境 dump。F 只消费 `research/cb-production-config-contract.md`，不自行发明别名；C-b 的实现证据仍是启动前门禁。
- **Q-F5 已确认（2026-08-31）**：浏览器发布矩阵不再作为 tag commit 中的 tracked 文件，也不要求文件内容自引用最终 commit。release workflow 在精确 `vX.Y.Z` tag commit 上调用受保护的真实浏览器证据 workflow，生成一次性 `browser-release-matrix-v1` artifact（载荷 `release-matrix.json` 加 provenance 清单）；载荷内的 `commit` 必须等于 tag commit，矩阵每行必须按固定顺序包含四个稳定 coverage ID 及 `coverage_summary_sha256`，provenance 必须绑定当前 release workflow 的 run/attempt、tag ref、载荷 SHA-256 和八槽位脱敏 `coverage_summaries`，摘要按 F 的 JCS 规则重算。artifact 只允许当前 run/attempt 下载和校验；重试 attempt 不得复用旧 artifact，重复或旧 attempt artifact 必须阻断，workflow 不覆盖或删除；人工删除/重试遵守已记录的维护者边界。该方案消除自引用循环，且不授权创建缺证据的占位矩阵。

## Technical Contracts（已确认）

以下契约已由用户确认，并在 C-b 接口和发布制品研究材料中作为唯一事实源同步；实现不得引入第二套字段或命名：

- **运行时版本与健康响应**：linker 符号为 `github.com/yangphere/leanote/app/service.BuildVersion`，默认未注入值为 `dev`；`ConfigService.GetVersion()` 只返回该变量，发布、打包和容器 smoke 拒绝 `dev` 或非严格 `X.Y.Z` 值。`GET /healthz` 不包含版本字段，ready 返回 `200` 与 `application/json; charset=utf-8`、精确正文 `{\"status\":\"ready\"}\n`，未 ready 返回 `503` 与 `{\"status\":\"not_ready\"}\n`。
- **发布二进制布局**：`sh/package.sh` 从 `cmd/leanote` 构建普通 Go 二进制，tarball 内固定为 `bin/leanote`，权限 `0755`；container entrypoint 只调用该路径，不把 `cmd/leanote` 源码目录当作归档二进制。
- **GHCR tag 映射**：release tag `vX.Y.Z` 唯一映射为 `ghcr.io/yangphere/leanote:vX.Y.Z`；重复预检、manifest、推送和拉取复验均使用该完整字符串，不生成或回退到 `latest` 或去掉 `v` 的别名。

## Requirements

- **R-F0** 版本只读 `package.json` 顶层 `version`（Q-F1），并要求 `package-lock.json` 根 package 的 `version` 与其完全一致；同一值用于应用显示、Go linker 注入值、tag 校验、tarball 名称、Release 元数据和 OCI labels，`/healthz` 按 Q-F2 只返回固定状态字段。tag 必须完整匹配 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`，禁止前导零、预发布标识和 build metadata；版本不一致、linker 注入缺失/为 `dev` 或已有同名资产时失败且不得覆盖。
- **R-F1** `pull_request` 和 `dev`、当前远程默认分支 `master` 的 push 运行同一质量门；Go 1.26.7/1.27.0 矩阵、MongoDB 8.0 集成测试、Node 24.20.0 构建与 JS 测试、Chromium 阻塞 E2E、生成物漂移、打包和 Docker smoke 均为阻断项。不得用任意分支 push 或旧 workflow 产生重复质量门。
- **R-F1a** 质量门的 `push` 触发只允许 `dev` 和当前远程默认分支 `master`；`workflow_dispatch` 只能用于明确登记的诊断/Golden 记录入口，不得提供 MongoDB 版本、代码 ref 或其他参数来绕过固定门禁。PR/fork 使用只读 `GITHUB_TOKEN` 和本次运行生成的隔离测试身份，不依赖外部生产 secret。
- **R-F2** Go module 的最低版本保持 1.26；Go 1.26.7 与 1.27.0 任一失败都阻断。Node 固定 24.20.0（`>=24 <25` 仅作为本地兼容范围），必须使用 `npm ci`、提交的 lockfile 和本地依赖，不得使用 `npx`/全局/联网 fallback。
- **R-F3** 构建链运行后执行 `git diff --exit-code`，证明提交的前端生成物、模板和清单来自唯一源码且无漂移。
- **R-F4** `vX.Y.Z` tag 触发发布：在 tag 指向的同一 commit 上调用与 PR/push 相同的 reusable quality gate，验证版本一致后生成固定命名的 `leanote-vX.Y.Z-linux-amd64.tar.gz`、`.sha256` 和机器可读构建元数据；tarball 内二进制固定为 `bin/leanote`、权限 `0755`，并上传 GitHub Release。质量门或资产校验失败不得创建/继续发布。release concurrency 必须 `cancel-in-progress: false`，已有 Release、资产或镜像 tag 时预检失败且不得覆盖；触发 tag/ref 必须指向当前 commit 且不得 force-update；重试、删除或恢复只允许走明确的人工边界。
- **R-F5** 同一 tag 构建并发布 `ghcr.io/yangphere/leanote:vX.Y.Z` 的 `linux/amd64` 镜像，禁止 `latest` 或去掉 `v` 的别名。镜像必须以固定非 root UID/GID 运行、只连接外置 MongoDB、声明 `files/` 与 `public/upload/` 持久化卷，并包含可工作的完整 PDF 功能；生产配置必须由挂载文件/明确环境变量注入，不能依赖仓库默认 secret。
- **R-F6** 发布工作流只使用最小权限的 `GITHUB_TOKEN`；PR/fork 不获得写权限。第三方 Actions、基础镜像和 MongoDB 镜像须使用不可变 commit/digest；不得使用真实数据库、SMTP 服务、Cookie、`app.secret` 或生产部署凭据，测试写入只能落在隔离的 `leanote_test` fixture。
- **R-F7** 失败必须阻止发布并保留符合 `research/ci-failure-summary-schema.md` 的可定位脱敏摘要。摘要必须使用研究材料规定的固定 job ID 集合；每个质量门 job 都必须在 `if: always()` 收尾步骤生成一条摘要；由独立汇总 job 以 `needs: ...` + `if: always()` 校验所有 job 的摘要，服务健康信息只能来自摘要的 `service` 字段，不得上传独立健康文件。执行 HTTP readiness 的 job，其 `service.health_path` 必须为 `/healthz`，并按 Q-F2 记录 HTTP+Mongo readiness 的结果；不得记录或接受 `/login` 作为健康证明。若 checkout/setup/服务启动等早期步骤失败，必须生成 `job_not_started` 或对应失败类别的最小 fallback 记录；摘要缺失、测试发现为零、服务 readiness 未确认、清理失败、PDF/漂移/版本校验失败都必须非零。禁止上传原始 Playwright trace/HTML report、截图/视频、storage state、cookie、认证头、页面正文或未脱敏服务日志。手工 `record-export-pdf` 仅可上传单一 Golden JSON，最长保留 7 天；不生成“成功”占位制品。
- **R-F8** 删除或替换 `.travis.yml`、旧 `revel package`/Revel CLI 和旧 Gulp 交付路径；现有 `regression-baseline.yml` 必须归并为唯一质量门或移除重叠触发，不能与 `ci.yml` 双重运行。`sh/package.sh` 保留为锁定工具链下的 POSIX 打包入口，调用迁移后的普通 Go binary。
- **R-F9** Chromium `business` 保持 PR/push 阻断 E2E；发布前必须由受保护真实浏览器证据 workflow 在 tag commit 上生成并由 release workflow 消费 `browser-release-matrix-v1` artifact（载荷 `release-matrix.json`，契约见 `research/release-matrix-contract.md`）。载荷必须是绑定发布 commit 的机器可读八行矩阵，覆盖真实 Chrome、Edge、Firefox、Safari 各当前及前一主版本。每条记录必须按固定顺序恰好包含 `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`，并以 `coverage_summary_sha256` 绑定 provenance 中对应槽位的四项正整数 discovered/executed 摘要；认证门禁、错误门禁、资源门禁、执行时间和结果也必须齐全并通过。Safari 必须是真实 Safari 环境，Chromium/WebKit 不能代替 Chrome、Edge 或 Safari。
- **R-F10** tarball 与镜像构建必须可复现：固定为 tag checkout SHA 的 Git committer timestamp 的 `SOURCE_DATE_EPOCH`、Go `-trimpath -buildvcs=false`、归档排序/owner/group/mode、gzip header、构建平台及基础镜像 digest。BuildKit/OCI config 的 `created` 和版本/revision/source labels 必须由同一 commit 时间和版本事实源生成，不得使用当前时间或 registry 默认值；provenance、attestation、SBOM 仅可在其内容和时间同样确定时保留，否则 release 构建显式关闭并记录参数，三类开关均须进入 release 输入清单。以固定 platform、完整镜像 digest 和 tarball SHA-256 做连续构建比较，同一 commit 必须一致，且独立解包/拉取复验通过；推送后 registry digest 必须等于 gate 元数据中的 image digest。
- **R-F11** quality gate 必须输出可验证的 run/commit 绑定制品清单：每个 tarball、`.sha256`、构建元数据和镜像构建输入只能来自当前 tag commit 的同一 quality-gate run；release 下载时必须校验 run ID、commit SHA、文件名、SHA-256 和预期 artifact allowlist，禁止跨 run、跨 ref、部分上传或旧成功 run 复用。
- **R-F12** 容器和 tarball 必须严格消费 `research/cb-production-config-contract.md` 的 C-b v1 接口：显式使用 `/etc/leanote/app.conf` 与 `-runMode prod`，只读权限为 `0440`；Mongo 仅使用 `MONGODB_URL`/`db.urlEnv`，secret 仅使用 `LEANOTE_APP_SECRET`/`app.secret`，`db.dbname` 与 URI 数据库路径精确一致且非 `leanote_test`。先验证唯一文件和 active `[prod]` 有效配置视图，再解析两个环境占位值；运行时值是唯一敏感来源，挂载文件只提供占位符和非敏感结构，literal、重复键、未声明来源/别名、缺失、空值、公开默认/短 secret、非法 URI、localhost 或数据库名冲突均在服务 ready 前 fail closed，稳定错误码对应退出状态 `78`。配置有效但 Mongo ping 失败必须由 `/healthz` 返回 `503`；F 不得回退 `conf/app.conf`、`conf/app.conf-default`、host/port 或公开 secret，日志/artifact 不含配置值、凭据、完整 URI 或环境 dump。

## Acceptance Criteria

- [ ] 新 PR/push 工作流在 Go 1.26.7 与 1.27.0 均通过编译、vet/允许基线、Go 测试和 MongoDB 8.0 集成测试。
- [ ] 依赖任务的归档验收与真实 workflow 证据重新核验；C-b 源码无 Revel 交付命中，frontend-libs 组合门禁已归档，才允许 F 激活。
- [ ] Node 24.20.0 执行 `npm ci`、build、JS 测试、Chromium E2E，并在重建后以 `git diff --exit-code` 和空 `git status --porcelain --untracked-files=all` 证明零漂移；每个测试层均证明发现目标用例数非零。
- [ ] CI 构建 tarball 并在干净临时目录解包，确认 `bin/leanote` 存在且权限为 `0755`；应用可启动并通过 `GET /healthz`（ready 为 `200`/`{\"status\":\"ready\"}\n`，否则为 `503`/`{\"status\":\"not_ready\"}\n`）；归档不含凭据、用户数据或本地生成物。
- [ ] CI 构建 Linux/amd64 镜像，以固定非 root UID/GID 启动，连接外部 MongoDB 8.0，通过 Q-F2 定义的 `GET /healthz`、上传持久化和真实 PDF 生成 smoke；不得以 `/login` 页面 `200` 代替健康证明，`files/` 与 `public/upload/` 跨重启保留。
- [ ] 一个测试 `vX.Y.Z` tag 在同一 commit 上通过完整质量门后创建 GitHub Release，附带固定命名 tarball、SHA-256 和元数据，并把 `ghcr.io/yangphere/leanote:vX.Y.Z` 镜像推到 GHCR；release concurrency 使用 `cancel-in-progress: false`，已有 Release、资产或该完整镜像 tag 均拒绝且不会覆盖，触发 tag/ref 移动或 force-update 必须失败，失败重试/人工删除仅按已记录的人工边界执行。
- [ ] 非 tag 的 PR/push 不发布 Release/镜像；任何工作流都不执行生产部署。
- [ ] `push` 仅匹配 `dev`/`master`，手工诊断入口不得改变 MongoDB 8.0、checkout SHA 或其他固定门禁；fork PR 不获得写权限且使用本次运行生成的隔离测试身份。
- [ ] 工作流权限、不可变 action/image 引用、缓存键、并发、超时和 artifact 保留期显式配置；固定 job ID 的每个 job 都有 `if: always()` 摘要收尾，汇总 job 在早期失败时生成 fallback，所有摘要通过 `research/ci-failure-summary-schema.md` 的 schema 校验；Playwright 与服务失败只上传 allowlisted 脱敏 job 摘要，服务健康信息只能来自 `service` 字段，最长保留 7 天，且清理失败同样阻断。
- [ ] release 只下载当前 tag quality-gate run 产生且通过 `research/release-artifact-contract.md` 的 artifact 名称、文件 allowlist、commit/run ID、版本、`SOURCE_DATE_EPOCH` 和 SHA-256 交叉校验的制品；缺失、重复、额外文件、跨 ref、跨 run 或部分上传均非零失败，不得复用历史成功 artifact。
- [ ] 受保护真实浏览器证据 workflow 在当前 tag commit 上只生成一次 `browser-release-matrix-v1` artifact；其中 `release-matrix.json` 恰好包含同一 commit 的真实 Chrome、Edge、Firefox、Safari 当前及前一主版本八行唯一键记录，每行按固定顺序包含四个稳定 coverage ID 和 `coverage_summary_sha256`，provenance 清单绑定当前 release run/attempt、tag ref、载荷 SHA-256 及八槽位 `coverage_summaries`。缺失、重复、版本不符、字段/schema 不完整、摘要 digest 不一致、非真实 Safari、artifact 跨 run/ref 或任一门禁失败都会阻断发布，禁止用占位记录伪造证据。
- [ ] 矩阵生成发生在 tag commit 之后的受保护 workflow 运行中，载荷不进入源码提交且不自引用最终 commit；release validator 校验 artifact 名称、唯一性、producer/release run 绑定、tag ref、commit、矩阵 SHA-256、四个 coverage ID 和 RFC 8785 JCS 摘要 digest，workflow 不覆盖或删除 artifact。
- [ ] F 的 smoke 与部署文档逐字引用 C-b v1：`-conf /etc/leanote/app.conf -runMode prod`（参数缺失/不符分别为 `CONFIG_PATH_INVALID`/`CONFIG_RUN_MODE_INVALID`）、只读 `0440`、`MONGODB_URL`/`db.urlEnv`、`LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 与 URI 一致且非 `leanote_test`；先验证文件结构再解析环境占位值，运行时值为唯一敏感来源且冲突不覆盖，所有配置错误在 bind/dial/healthz 前以稳定错误码和退出 `78` 失败，Mongo ping 未就绪仅返回 `503`，日志和 artifact 不含配置值或完整 Mongo URI。
- [ ] 同一 commit 连续 tarball/镜像构建的完整 SHA-256 和镜像 digest 一致，且 OCI `created`/labels、provenance/attestation/SBOM 策略已固定；解包/拉取后健康、上传和 PDF 复验通过；`.travis.yml`、旧 Revel/Gulp 交付假设和重叠 CI 触发已移除，README/部署文档说明版本矩阵、镜像架构、外部配置、卷和无自动部署边界。
- [ ] `release-inputs.json`、tarball、校验和、构建元数据和镜像构建输入按 `research/release-artifact-contract.md` 的固定文件名、schema、allowlist、run/attempt/ref/commit 和哈希规则交接；二进制固定为 `bin/leanote`/`0755`，GHCR tag 固定映射为 `ghcr.io/yangphere/leanote:vX.Y.Z`。

## Activation Evidence Gate

Q-F4 的需求接口和运行时/发布技术契约已经闭合；上游实现证据与依赖完成证据仍是独立的激活阻断。
C-b 必须在其实现/验收材料中引用
`research/cb-production-config-contract.md` 和 `research/cb-production-config-evidence.md`；F Task 0 只能在 E1-E8
全部为 `PASS` 后收集并关闭证据，包括明确的运行入口和部署文档、每个错误码及约束的单元测试、退出状态 `78`/不 bind/不 dial
的进程级测试、无 host/port/localhost/`leanote_test` fallback 和完整 URI 日志的静态/回归证据，以及合法配置的
package/container smoke（`/healthz` 的 `200/503` 固定 JSON 与脱敏摘要）。任何一项缺失、测试发现为零、artifact provenance
不匹配，或两个依赖的真实完成证据未闭合，均保持 `planning`，不运行 `task.py start`。已确认的
`BuildVersion`、`/healthz`、`bin/leanote` 和 GHCR tag 契约必须继续由研究材料作为唯一事实源校验。

## Out of Scope

- 自动部署、环境审批、回滚生产实例或管理生产密钥。
- Linux/arm64 多架构镜像；后续工作记录在 `docs/modernization-backlog.md`。
- Windows/macOS 容器镜像和托管 MongoDB。
- 将 GitHub Actions 替换为通用发布平台抽象。
