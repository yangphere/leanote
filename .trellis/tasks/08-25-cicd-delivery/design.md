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

## 2. 固定工具链与服务

- Go：固定 `1.26.7`、`1.27.0` 矩阵；`go.mod` 最低 1.26，禁止 `stable`/`latest`/自动下载。
- Node：固定 `24.20.0`，使用 `npm ci`；缓存键包含 lockfile、runner OS/架构和 Node 版本。
- MongoDB：官方 8.0 镜像固定 digest，等待真实 ping 后导入最小测试数据；fixture 数据库只能是
  `leanote_test`，Go replay 的目标测试数必须非零。
- 浏览器：`@playwright/test` 的 Chromium 是阻塞 E2E。真实 Chrome、Edge、Firefox、Safari 的当前及前一主版本属于发布前 smoke 证据；Safari 必须在真实 Safari 环境验证，Playwright Chromium 不能代替 Chrome 或 Edge。每项证据记录提交 SHA、浏览器产品/完整版本、OS、覆盖范围、认证/错误门禁和结果，且不得包含认证材料或页面内容。

所有质量门 job 具有显式 timeout（Go/Mongo 45 分钟、Node/E2E 35 分钟、package/container 30 分钟）和
同一 ref 的 concurrency group；tag release 的 `cancel-in-progress` 为 false。失败时只上传 schema
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

重写 `sh/package.sh`，只从显式 allowlist 收集迁移后的 `cmd/leanote` 二进制、`conf/app.conf-default`
配置样例、`app/views`、`messages`、`public` 运行资源和必要运行脚本；明确排除 `conf/app.conf`、
`mongodb_backup`、`files`、`public/upload` 内容、`node_modules`、测试输出、日志、`.git` 和本地配置。
构建固定 `SOURCE_DATE_EPOCH`（tag commit 时间），Go 使用 `-trimpath -buildvcs=false`，归档使用
稳定路径排序、numeric owner/group、固定 mode/mtime，gzip 不写入当前时间。输出命名为
`leanote-vX.Y.Z-linux-amd64.tar.gz` 与同名 `.sha256`，CI 在干净临时目录解包，使用外部 MongoDB
和明确的健康端点启动并验证；连续构建比较完整 SHA-256，而不只比较文件名。

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

运行配置通过挂载的 prod 配置文件和已确认的外部 Mongo URL/secret 注入（变量名与 C-b 的配置契约
一致，见 Q-F2）；不得把仓库默认 `app.secret` 或 localhost Mongo fallback 放入镜像。声明并挂载
`files/` 与 `public/upload/`，container smoke 启动 MongoDB 8.0 和应用，调用健康端点确认 HTTP 与
Mongo readiness，写入一个隔离测试文件/上传后重启容器并确认仍存在，再走真实 HTML→PDF 流程并校验
`%PDF-` 签名、非空内容和清理。首发 platform 固定 `linux/amd64`；arm64 与 PDF 依赖适配记录为
`MOD-002`，不得通过禁用 PDF 伪装兼容。

## 5. 标签发布与权限

release workflow 验证 tag 精确匹配 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`，且应用版本来源与 `X.Y.Z` 相同；
版本只从 Q-F1 确认的一个机器可读来源生成，不在多个脚本手工维护。发布 job 只有
`contents: write` 与 `packages: write`，使用 GitHub 内建 token，不存长期 PAT；tag commit 必须
与 quality-gate 的 checkout SHA 相同，已有 Release、同名资产或镜像标签时失败而不覆盖。

镜像推送版本 tag 与 commit digest label，记录最终 image digest；Release 上传 tarball、校验和及
不含敏感信息的构建元数据，下载后再验证 SHA-256。任一验证或推送失败都使 workflow 失败，不发布
空附件、不继续生产部署；release 重试/错误资产删除遵循 Q-F3 的人工边界。

## 6. 安全与回滚

构建上下文由 `.dockerignore` 排除 `.git`、数据库备份、`files`、上传、日志、测试输出、本地配置
和密钥。Actions 引用固定 commit SHA，脚本关闭 shell trace 并显式 mask 测试凭据；日志不打印配置
值。quality-gate 对每个测试层执行 test discovery，零目标用例或未生成摘要视为失败。

回滚通过重新部署上一不可变 Release/镜像完成，本任务不自动控制生产环境。错误 tag 的 Release/镜像
由维护者按 GitHub 权限流程人工处理，不由 workflow 静默覆盖或重写。

## 7. 跨任务契约

- C-b 必须提供 F 使用的无认证健康端点及 DB readiness 语义；F 不在 CI 里把 `/login` 页面 200
  当作数据库健康，也不在 F 中偷偷添加业务健康逻辑。
- B/C-b 完成后，`app/tests/harness` 的 Mongo 连接、服务器启动和 cleanup 入口必须不再依赖
  Revel CLI；F 的 quality-gate 只调用该唯一 harness。
- frontend-libs 的最终四浏览器记录以受跟踪、机器可读的 release matrix 为输入；F 只验证记录的
  commit、版本、真实 Safari 和错误门禁，不复制第二套浏览器配置。
- release matrix 的唯一文件、八行唯一键和字段校验见 `research/release-matrix-contract.md`；由
  frontend-libs 协调任务负责生成并维护，F 只消费绑定 tag commit 的记录。
