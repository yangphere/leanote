# CI/CD 与可复现交付（F）— 技术设计

## 1. 工作流分层

使用一个质量门实现和两个触发包装：

- `.github/workflows/quality-gate.yml`：`workflow_call` 可复用质量门；所有 job 默认
  `contents: read`，不持有 package write 权限。
- `.github/workflows/ci.yml`：`pull_request` 以及 `dev`、当前远程默认分支 `master` 的
  push 触发 quality-gate；不得与现有 `regression-baseline.yml` 重叠触发，后者必须归并或删除。
- `.github/workflows/release.yml`：仅 `vX.Y.Z` tag 触发，先调用同一 commit 的 quality-gate，
  再消费已校验的构建输出创建 GitHub Release 和推送 GHCR image。

quality-gate 按可并行 job 拆为 Go 单元/静态检查矩阵、MongoDB 8.0 集成、Node/build、Chromium
E2E、package smoke、container smoke，并以显式 job outputs 汇总。release 只消费在 tag 指向的
同一 commit 上重新验证并以 SHA-256 校验的制品，不以“分支曾经绿过”替代 tag 验证。

本任务的精确 tag 规则优先于父任务设计和 ADR-0004 中历史 `v*` 触发描述；实现只能采用第 5 节
定义的严格正则，不能把历史文字解释为允许任意 `v` 标签。

### 1.1 触发、输入与输出顺序

质量门的唯一入口按以下顺序处理：

1. `ci.yml` 在 `pull_request`、`dev` 或 `master` push 时调用
   `quality-gate.yml`；PR/fork 只获得 `contents: read`，不读取生产 secret。
2. `release.yml` 仅接受严格 `vX.Y.Z` tag，先在 tag 指向的 commit 上调用同一
   `quality-gate.yml`，再调用受保护的真实浏览器证据 workflow 生成一次
   `browser-release-matrix-v1` artifact，并校验版本事实源、矩阵 provenance 和本次 gate 的 artifact 清单。
3. 只有 gate 汇总成功且所有输入均绑定同一 commit/run，release 才能仅用交接的
   `image-build-inputs.json` 重建或载入候选镜像，先校验本地 digest 与 gate 元数据一致，再推送
   GHCR 并校验 registry digest 完全一致，最后创建 Release、上传 tarball/校验和/元数据；任一步
   失败都停止后续发布步骤，不执行补偿性覆盖。
   release 按 tag/ref 使用独立 concurrency group，`cancel-in-progress: false`；并发运行等待而不
   取消先行运行，等待本身不授予覆盖权限。

quality gate 的机器输入是触发事件、checkout SHA、固定工具链、`leanote_test` fixture、
已确认的配置接口、Q-F2 健康端点和（发布时）浏览器矩阵；机器输出是固定 job ID 的摘要、带 run/commit 绑定的
tarball 与 metadata、以及容器镜像 digest。输出 artifact 必须带唯一的逻辑名称和 SHA-256，
release 只能从当前 workflow run 下载并再次校验，不能按名称搜索或复用历史 run。

## 2. 固定工具链与服务

- Go：固定 `1.26.7`、`1.27.0` 矩阵；`go.mod` 最低 1.26，禁止 `stable`/`latest`/自动下载。
- Node：固定 `24.20.0`，使用 `npm ci`；缓存键包含 lockfile、runner OS/架构和 Node 版本。
- MongoDB：官方 8.0 镜像固定 digest，等待真实 ping 后导入最小测试数据；fixture 数据库只能是
  `leanote_test`，Go replay 的目标测试数必须非零。
- HTTP readiness：C-b 提供无需认证的 `GET /healthz`；仅 HTTP 服务已监听且 MongoDB ping 成功时返回
  `200`，任一条件未满足返回 `503`。响应正文和响应头不得泄露配置、凭据或用户数据；任何执行服务
  readiness 的 job 都必须探测该端点，`/login` 页面 `200` 不构成健康证明。
- 浏览器：`@playwright/test` 的 Chromium 是阻塞 E2E。真实 Chrome、Edge、Firefox、Safari 的当前及前一主版本属于发布前 smoke 证据；Safari 必须在真实 Safari 环境验证，Playwright Chromium 不能代替 Chrome 或 Edge。每项证据记录提交 SHA、浏览器产品/完整版本、OS、覆盖范围、认证/错误门禁和结果，且不得包含认证材料或页面内容。

依赖安装的网络边界必须显式区分：`npm ci`、锁定版本的 Playwright 浏览器安装和固定 digest
基础镜像拉取是声明的构建输入；除这些步骤外，脚本不得使用 `npx`、全局工具或联网 fallback
补齐缺失依赖。依赖下载失败必须保留原退出码并使 job 失败。

所有质量门 job 具有显式 timeout（Go/Mongo 45 分钟、Node/E2E 35 分钟、package/container 30 分钟）和
同一 ref 的 concurrency group；tag release 按 tag/ref 隔离且 `cancel-in-progress` 为 false。失败时只上传 schema
校验过的 allowlisted 脱敏 job 摘要；服务健康信息只允许出现在摘要的 `service` 字段中，最长保留 7 天；禁止上传原始 Playwright
trace/HTML report、浏览器 screenshot/video、storage state、cookie、认证头、页面正文或未脱敏服务日志。
摘要只包含工具/运行版本、阶段、脱敏页面路径、资源路径、状态码、错误类别、服务 health_path/readiness/http_status/exit_code
等可定位字段。唯一受控例外是手工 `record-export-pdf` 的单一 Golden JSON；仅 `workflow_dispatch`
上传，最长保留 7 天，且不得包含账号、Cookie、认证头或页面正文。

每个质量门 job 必须设置 `if: always()` 的最终摘要步骤，并由独立汇总 job 在
`needs: [所有七个质量门生产 job]` 且 `if: always()` 时校验所有摘要；摘要中的逻辑 job ID 固定为
`go-1_26_7`、`go-1_27_0`、`mongo-8_0`、`node-build`、`chromium-e2e`、`package-smoke`、
`container-smoke`、`summary`，不允许用未登记的别名替代。摘要格式、字段枚举、脱敏规则和
artifact allowlist 以 `research/ci-failure-summary-schema.md` 为唯一契约；checkout、setup、
服务启动等早期失败也必须写入最小 fallback（`failure.category=job_not_started` 或真实失败类别，
`status=failed`，并保留非零 `exit_code` 或 `null` 的未运行标记）。汇总 job 在缺少任一摘要、无法
通过 schema 校验或发现目标测试数为零时失败，不能以成功占位记录补齐；不得上传独立健康摘要文件。

## 3. 生成物和 tarball

Node job 先运行构建与测试，再执行 `git diff --exit-code`。这使生成资源仍可跟踪，但源码和 manifest 是唯一事实来源。

重写 `sh/package.sh`，从 `cmd/leanote` 源码包构建普通 Go 二进制，固定将二进制写入 tarball 的
`bin/leanote` 路径并设置 `0755`，再按已确认契约收入显式 allowlist；其余只包含 `conf/app.conf-default`
配置样例、`app/views`、`messages`、`public` 运行资源和必要运行脚本。明确排除 `conf/app.conf`、
`mongodb_backup`、`files`、`public/upload` 内容、`node_modules`、测试输出、日志、`.git` 和本地配置。
构建固定 `SOURCE_DATE_EPOCH`（tag commit 时间），Go 使用 `-trimpath -buildvcs=false`，归档使用
稳定路径排序、numeric owner/group、固定 mode/mtime，gzip 不写入当前时间。输出命名为
`leanote-vX.Y.Z-linux-amd64.tar.gz` 与同名 `.sha256`，CI 在干净临时目录解包，使用外部 MongoDB
和 `GET /healthz` 启动并验证（HTTP+Mongo ready 必须为 `200`，否则必须为 `503`）；连续构建比较完整
SHA-256，而不只比较文件名。不得以 `/login` 页面 `200` 代替该 readiness 证明。

## 4. 容器镜像

多阶段 `Dockerfile` 分离 Node 资源构建、Go 编译和运行时；Node/Go/Mongo/PDF 基础依赖均使用
不可变版本或 digest。运行层只包含应用、静态资源、配置样例、CA/时区及 PDF 渲染所需的明确系统
依赖；使用固定非 root UID/GID，入口不内置 MongoDB，构建平台固定 `linux/amd64`。

BuildKit 构建必须接收 tag commit 的 `SOURCE_DATE_EPOCH`，将 OCI config `created` 和
`org.opencontainers.image.created` 固定为该时间的 UTC 表示，并从同一版本事实源写入 version、
revision、source labels；不得使用构建当前时间、随机值或 registry 默认时间。release 构建显式固定
provenance/attestation/SBOM 开关及其输入：只有内容与时间可证明确定时才保留，否则使用
`--provenance=false --sbom=false`（或等价参数）并把选择记录到构建元数据。构建平台、基础镜像
digest、构建参数和最终镜像 digest 都必须进入可审计元数据。

运行配置严格消费 `research/cb-production-config-contract.md` 的 C-b v1 接口：必须显式传入
`-conf /etc/leanote/app.conf -runMode prod`，配置文件是唯一生产路径、必须为只读 regular file 且部署权限为
`0440`。Mongo 只允许 `MONGODB_URL` 经 `db.urlEnv=${MONGODB_URL}` 占位引用注入，secret 只允许
`LEANOTE_APP_SECRET` 经 `app.secret=${LEANOTE_APP_SECRET}` 占位引用注入；`db.dbname` 必须存在、与 URI 数据库路径
一致且不得为 `leanote_test`。解析顺序固定为先读该文件并验证 active `[prod]` section 的有效结构，再解析两个环境键；环境值是唯一
敏感值来源，挂载文件只提供占位符和非敏感结构。literal 值、重复键、未声明别名或其他来源造成冲突时直接失败，
不静默覆盖。缺失/不可读文件、section/键形态错误、环境值缺失或为空、公开默认/短 secret、非法 URI 或数据库名
均必须在 HTTP bind/listen、Mongo dial/ping 和 `/healthz` 可达前 fail closed，以研究材料固定的稳定错误码
（包括 `CONFIG_PATH_INVALID`、`CONFIG_RUN_MODE_INVALID`、`CONFIG_FILE_MISSING`、`CONFIG_FILE_UNREADABLE`、
`CONFIG_SECTION_MISSING`、`CONFIG_KEY_INVALID`、`CONFIG_VALUE_MISSING`、`CONFIG_VALUE_EMPTY`、
`CONFIG_SOURCE_CONFLICT`、`CONFIG_PUBLIC_DEFAULT`、`CONFIG_SECRET_INVALID`、`CONFIG_MONGO_INVALID`）退出 `78`；合法配置
但 Mongo ping 失败按 Q-F2 由 `/healthz` 返回 `503`。不得回退 `conf/app.conf`、`conf/app.conf-default`、localhost、
host/port 组合或公开 secret；日志和 artifact 只保留错误码、非敏感键名、`run_mode=prod`，不得含配置值、凭据、
完整 Mongo URI 或环境 dump。`GET /healthz` 固定返回 `application/json; charset=utf-8`：HTTP+Mongo
ready 时为 `200` 和 `{"status":"ready"}\n`，未 ready 时为 `503` 和 `{"status":"not_ready"}\n`；响应
不包含版本字段、配置、凭据或用户数据。

Dockerfile、package/container smoke 和部署文档只引用该接口表，不自行复制或发明别名。声明并挂载 `files/` 与
`public/upload/`，container smoke 启动 MongoDB 8.0 和应用，调用 `GET /healthz` 确认 HTTP 与 Mongo readiness（ready
为 `200`，未 ready 为 `503`），写入一个隔离测试文件/上传后重启容器并确认仍存在，再走真实 HTML→PDF 流程并校验
`%PDF-` 签名、非空内容和清理。首发 platform 固定 `linux/amd64`；arm64 与 PDF 依赖适配记录为 `MOD-002`，不得通过
禁用 PDF 伪装兼容。完整接口表已在规格阶段固定；C-b 的实现、测试和运行证据取得前，F 仍保持 planning。

## 5. 版本事实源、标签发布与权限

### 5.1 版本事实源（Q-F1 已确认）

`package.json` 顶层 `version` 是唯一版本输入；`package-lock.json` 根 package 的 `version` 必须
逐字匹配。所有 Node/release 脚本读取同一字段并用严格 `X.Y.Z` 校验，再将其转换为 `vX.Y.Z`
标签和 `leanote-vX.Y.Z-linux-amd64.tar.gz` 文件名。Go 发布构建必须通过 linker 将该值注入
`github.com/yangphere/leanote/app/service.BuildVersion`，`ConfigService.GetVersion()` 只返回该变量；
变量默认值固定为 `dev`，仅允许显式开发/测试场景，release/package/container smoke 必须拒绝 `dev`
或非严格 `X.Y.Z` 值。OCI `version` label、Release metadata 和 tarball metadata 也必须由该值生成，
不得保留第二个硬编码版本。

版本校验必须同时读取两个 package 文件、检查严格 tag 正则和注入值相等性；任何 JSON 缺失、版本
不一致、前导零、预发布/build metadata 或 linker 注入缺失都在构建/发布前失败。

### 5.2 标签发布与权限

release workflow 验证 tag 精确匹配 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`，且应用版本来源与 `X.Y.Z` 相同；
版本只从上述 `package.json` 来源生成，不在多个脚本手工维护。发布 job 只有
`contents: write` 与 `packages: write`，使用 GitHub 内建 token，不存长期 PAT；tag commit 必须
与 quality-gate 的 checkout SHA 相同，已有 Release、同名资产或镜像标签时失败而不覆盖。

镜像推送使用唯一完整 tag `ghcr.io/yangphere/leanote:vX.Y.Z`，禁止 `latest` 或去掉 `v` 的别名；同时
记录 commit digest label 和最终 image digest。Release 上传 tarball、校验和及
不含敏感信息的构建元数据，下载后再验证 SHA-256。任一验证或推送失败都使 workflow 失败，不发布
空附件、不继续生产部署；release 重试/错误资产删除遵循 Q-F3 已确认的人工边界。

### 5.3 发布 artifact 交接

quality-gate 应在汇总成功后按 `research/release-artifact-contract.md` 发布唯一的
`leanote-release-inputs-v1` artifact；其中的 `release-inputs.json`、固定命名 tarball、`.sha256`、
`build-metadata.json` 和 `image-build-inputs.json` 必须在同一 run 中生成。release 通过该清单按精确
artifact 名称下载，并验证：

- artifact 所属 workflow、run ID/attempt、tag ref 和 commit SHA 与当前 release 完全一致；
- 文件路径只落在 allowlist，文件名包含同一 `X.Y.Z` 与 `linux-amd64`；
- `.sha256` 使用固定格式且重新计算匹配，`build-metadata.json` 与
  `image-build-inputs.json` 中的版本、commit、SOURCE_DATE_EPOCH、platform 和镜像构建输入
  与 gate 输出一致，且 `attestation` 开关也必须被记录；`build-metadata.json.image_digest`
  使用固定 `sha256:` 格式，并要求重建候选镜像的本地 digest 与其相等，再与 GHCR 推送后返回的 digest 相等；
- 缺失、重复、跨 ref/run、哈希不匹配或部分上传立即失败，不能从旧 run 或 registry 标签回补。

文件 allowlist、清单 schema、四类文件唯一性和 `.sha256` 行格式以
`research/release-artifact-contract.md` 为唯一契约；F 不在 workflow 中维护第二套字段表。

release 预检还必须在创建 Release 或推送镜像前检查同名 GitHub Release、Release 资产和 GHCR 镜像
tag 是否已存在；任一存在即失败，不得覆盖、自动删除或从其他 run/registry tag 补偿。触发的 Git
tag/ref 必须指向当前 checkout commit，且任何移动或 force-update 都必须失败。失败重试、错误资产
删除和人工恢复只能由维护者按已记录的人工边界执行，workflow 不得自动改变这些边界。

### 5.4 浏览器矩阵证明交接（Q-F5 已确认）

浏览器矩阵不再是源码中的 tracked 文件。Q-E1 等待模式分两个阶段：E 归档前可由受保护 workflow
在严格 tag 指向的 checkout SHA 上运行一次预检，只上传供 E 验收的 `browser-release-matrix-v1`
artifact，不创建 Release/GHCR 或改变 F 状态；E 归档后，`release.yml` 在最终 release run 中
重新调用受保护的真实浏览器证据 workflow，重新生成正式 artifact。两阶段都使用 frontend-libs
约定的 smoke 定义，在真实 Chrome、Edge、Firefox、Safari 当前及前一主版本环境中执行，artifact
allowlist 恰好为：

- `release-matrix.json`：通过 `research/release-matrix-contract.md` schema 的八行矩阵，顶层和每条
  记录的 `commit` 均为 tag commit；
- `provenance.json`：`schema_version`、矩阵 `matrix_sha256`、同一 `commit`、精确 `ref`、producer
  workflow 标识、当前 release `run.id`/`run.attempt` 以及与八个槽位一一对应的脱敏
  `coverage_summaries`；每条 `release-matrix.json` 记录同时带 `coverage_summary_sha256`。

矩阵 producer 不接受可改变 checkout ref、MongoDB 版本或门禁的用户输入；它从调用 workflow 的
tag/ref 和 checkout SHA 取得绑定值。E 预检 validator 必须重新计算 `release-matrix.json` 的
SHA-256，并逐项校验 artifact 名称/唯一性、provenance schema、coverage summary digest、
producer run/attempt、tag ref、commit、八行唯一键、真实 Safari 和全部门禁；预检 tag commit
必须等于 E 的候选 SHA，且预检不得创建 Release/GHCR。最终 release validator 除上述校验外，
还必须确认 artifact 属于当前最终 release run/attempt；任何缺失、重复、跨 run/ref、摘要哈希不一致
或非真实浏览器记录都在创建 Release/GHCR 前失败。每个 producer `run.id`/`run.attempt` 只允许
一个该名称 artifact；重试产生的新 attempt 不得复用旧 attempt 的矩阵，旧 artifact 会被视为
重复/跨 attempt 并失败。两阶段 artifact 的保留期均不超过 7 天；workflow 不覆盖或删除，人工删除、
重试和恢复只能按 Q-F3 的维护者边界执行。

## 6. 安全与回滚

构建上下文由 `.dockerignore` 排除 `.git`、数据库备份、`files`、上传、日志、测试输出、本地配置
和密钥。Actions 引用固定 commit SHA，脚本关闭 shell trace 并显式 mask 测试凭据；日志不打印配置
值。quality-gate 对每个测试层执行 test discovery，零目标用例或未生成摘要视为失败。

回滚通过重新部署上一不可变 Release/镜像完成，本任务不自动控制生产环境。错误 tag 的 Release/镜像
由维护者按 GitHub 权限流程人工处理；任何重试或删除前必须有明确人工记录和权限边界，workflow 不得
静默覆盖、强制更新、自动删除或重写。

## 7. 跨任务契约

- C-b 必须提供 F 使用的无认证 `GET /healthz` 及 DB readiness 语义：HTTP 服务监听且 Mongo ping 成功
  返回 `200`，否则返回 `503`，且响应不得泄露配置；F 不在 CI 里把 `/login` 页面 200 当作数据库健康，
  也不在 F 中偷偷添加业务健康逻辑。C-b 的实现证据仍是 F 的启动前依赖门禁。
- B/C-b 完成后，`app/tests/harness` 的 Mongo 连接、服务器启动和 cleanup 入口必须不再依赖
  Revel CLI；F 的 quality-gate 只调用该唯一 harness。
- frontend-libs 的浏览器 smoke 定义和真实环境证据由受保护的真实浏览器证据 workflow 生成；其
  既有逐子任务 tracked 脱敏记录可以作为协调证据，但不是 F 的发布输入。F 不复制第二套浏览器配置，
  只验证 `browser-release-matrix-v1` artifact 的 commit、版本、四个稳定 coverage ID、摘要 digest、
  真实 Safari 和错误门禁。
- release matrix 的唯一载荷文件、八行唯一键、coverage summary digest 和字段校验见
  `research/release-matrix-contract.md`；载荷只在精确 tag commit 的 release workflow 运行中生成，
  不进入源码提交，也不要求文件内容自引用。
- Q-F5 已确认采用独立不可变证明：artifact 同时包含 `release-matrix.json` 和 provenance 清单，
  provenance 内嵌脱敏 `coverage_summaries`；release validator 必须校验当前 release run/attempt、
  tag ref、commit、载荷 SHA-256 和每个槽位的摘要 digest；workflow 不覆盖或删除 artifact，人工恢复
  遵守已记录的维护者边界。
- 已确认并同步的技术契约：linker 使用 `github.com/yangphere/leanote/app/service.BuildVersion`，未注入
  值为 `dev` 且 release smoke 拒绝；`GET /healthz` 使用固定 JSON 状态响应且不含版本字段；tarball
  二进制为 `bin/leanote`、权限 `0755`；GHCR 唯一 tag 为 `ghcr.io/yangphere/leanote:vX.Y.Z`，不生成别名。
