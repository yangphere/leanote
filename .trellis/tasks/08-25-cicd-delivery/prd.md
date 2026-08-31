# CI/CD 与可复现交付（F）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在后端与前端现代化轨道均完成后，用 GitHub Actions 建立可复现的 PR/push 质量门和精确 `vX.Y.Z` 标签发布流程，产出 GitHub Release tarball 与 GHCR Linux/amd64 镜像；不自动部署生产环境。

## Dependencies

- 依赖 `08-25-revel-migration` 和 `08-25-frontend-libs` 完成，并以归档任务的验收清单、实现/检查材料和真实 workflow 证据重新核验；父任务的 `[n/n done]` 子项计数不能替代完成状态。
- `08-25-revel-migration` 的当前源码必须已移除 `app/cmd`、旧 Revel 交付路径及其生产依赖；仅 `task.json.status=completed` 不构成 F 的启动证据。
- `08-25-frontend-libs` 必须先完成协调父任务的组合门禁并归档；其真实四浏览器当前/前一主版本记录缺失时，F 保持 blocked。
- 复用 `08-25-regression-baseline` 的 Golden、USN、权限和页面 smoke 契约。G 的历史 fixture 是 MongoDB 5.0；F 的 MongoDB 8.0 运行必须使用已由 `08-25-mongo-driver-migration` 迁移后的 harness，不得把 G 描述为 8.0 初始化来源。

## Readiness Gate

按现代化父任务的轨道顺序，F 是后端（Go → Revel 1.1 → Mongo 驱动 → C-b）与前端（构建链 → jQuery → Bootstrap → TinyMCE）两条轨道汇合后的候选叶。ready 的必要条件是：本任务为叶、状态为 `planning`，并且 `meta.depends_on` 的每个任务都已归档完成且有可核验的实现、验收、检查和真实 workflow 证据。当前 `08-25-frontend-libs` 仍为 planning，归档的 C-b 材料仍有未闭合验收且源码保留 Revel 命中，因此本任务不可激活；审核阶段不运行 `task.py start`。

## Specification Precedence

本任务的精确 tag、版本一致性、失败处理和发布验收契约以本 PRD、`design.md`、`implement.md` 及其研究材料为准。父任务设计和 ADR-0004 中历史遗留的 `v*` 表述只保留为背景，不能放宽本任务的精确规则；实现前必须按本任务规则校验并在任务收口时记录该文档冲突已被识别。

## Requirements

- **R-F0** 版本必须来自一个待确认的机器可读事实源（见 Q-F1），同一值用于应用显示/健康响应、tag 校验、tarball 名称、Release 元数据和 OCI labels。tag 必须完整匹配 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`，禁止前导零、预发布标识和 build metadata；版本不一致或已有同名资产时失败且不得覆盖。
- **R-F1** `pull_request` 和 `dev`、当前远程默认分支 `master` 的 push 运行同一质量门；Go 1.26.7/1.27.0 矩阵、MongoDB 8.0 集成测试、Node 24.20.0 构建与 JS 测试、Chromium 阻塞 E2E、生成物漂移、打包和 Docker smoke 均为阻断项。不得用任意分支 push 或旧 workflow 产生重复质量门。
- **R-F2** Go module 的最低版本保持 1.26；Go 1.26.7 与 1.27.0 任一失败都阻断。Node 固定 24.20.0（`>=24 <25` 仅作为本地兼容范围），必须使用 `npm ci`、提交的 lockfile 和本地依赖，不得使用 `npx`/全局/联网 fallback。
- **R-F3** 构建链运行后执行 `git diff --exit-code`，证明提交的前端生成物、模板和清单来自唯一源码且无漂移。
- **R-F4** `vX.Y.Z` tag 触发发布：在 tag 指向的同一 commit 上调用与 PR/push 相同的 reusable quality gate，验证版本一致后生成固定命名的 `leanote-vX.Y.Z-linux-amd64.tar.gz`、`.sha256` 和机器可读构建元数据，并上传 GitHub Release；质量门或资产校验失败不得创建/继续发布。
- **R-F5** 同一 tag 构建并发布 `ghcr.io/yangphere/leanote` 的 `linux/amd64` 镜像。镜像必须以固定非 root UID/GID 运行、只连接外置 MongoDB、声明 `files/` 与 `public/upload/` 持久化卷，并包含可工作的完整 PDF 功能；生产配置必须由挂载文件/明确环境变量注入，不能依赖仓库默认 secret。
- **R-F6** 发布工作流只使用最小权限的 `GITHUB_TOKEN`；PR/fork 不获得写权限。第三方 Actions、基础镜像和 MongoDB 镜像须使用不可变 commit/digest；不得使用真实数据库、SMTP 服务、Cookie、`app.secret` 或生产部署凭据，测试写入只能落在隔离的 `leanote_test` fixture。
- **R-F7** 失败必须阻止发布并保留符合 `research/ci-failure-summary-schema.md` 的可定位脱敏摘要。摘要必须使用研究材料规定的固定 job ID 集合；每个质量门 job 都必须在 `if: always()` 收尾步骤生成一条摘要；由独立汇总 job 以 `needs: ...` + `if: always()` 校验所有 job 的摘要，服务健康信息只能来自摘要的 `service` 字段，不得上传独立健康文件。若 checkout/setup/服务启动等早期步骤失败，必须生成 `job_not_started` 或对应失败类别的最小 fallback 记录；摘要缺失、测试发现为零、服务 readiness 未确认、清理失败、PDF/漂移/版本校验失败都必须非零。禁止上传原始 Playwright trace/HTML report、截图/视频、storage state、cookie、认证头、页面正文或未脱敏服务日志。手工 `record-export-pdf` 仅可上传单一 Golden JSON，最长保留 7 天；不生成“成功”占位制品。
- **R-F8** 删除或替换 `.travis.yml`、旧 `revel package`/Revel CLI 和旧 Gulp 交付路径；现有 `regression-baseline.yml` 必须归并为唯一质量门或移除重叠触发，不能与 `ci.yml` 双重运行。`sh/package.sh` 保留为锁定工具链下的 POSIX 打包入口，调用迁移后的普通 Go binary。
- **R-F9** Chromium `business` 保持 PR/push 阻断 E2E；发布前必须消费 `docs/modernization/browser-smoke/release-matrix.json` 中绑定发布 commit 的机器可读八行矩阵（契约见 `research/release-matrix-contract.md`）：真实 Chrome、Edge、Firefox、Safari 各当前及前一主版本。每条记录包含 commit、浏览器产品/完整版本、OS、覆盖范围、认证门禁、错误门禁、资源门禁、执行时间和结果；Safari 必须是真实 Safari 环境，Chromium/WebKit 不能代替 Chrome、Edge 或 Safari。
- **R-F10** tarball 与镜像构建必须可复现：固定 `SOURCE_DATE_EPOCH`、Go `-trimpath -buildvcs=false`、归档排序/owner/group/mode、gzip header、构建平台及基础镜像 digest。BuildKit/OCI config 的 `created` 和版本/revision/source labels 必须由同一 commit 时间和版本事实源生成，不得使用当前时间或 registry 默认值；provenance、attestation、SBOM 仅可在其内容和时间同样确定时保留，否则 release 构建显式关闭并记录参数。以固定 platform、完整镜像 digest 和 tarball SHA-256 做连续构建比较，同一 commit 必须一致，且独立解包/拉取复验通过。

## Acceptance Criteria

- [ ] 新 PR/push 工作流在 Go 1.26.7 与 1.27.0 均通过编译、vet/允许基线、Go 测试和 MongoDB 8.0 集成测试。
- [ ] 依赖任务的归档验收与真实 workflow 证据重新核验；C-b 源码无 Revel 交付命中，frontend-libs 组合门禁已归档，才允许 F 激活。
- [ ] Node 24.20.0 执行 `npm ci`、build、JS 测试、Chromium E2E，并在重建后以 `git diff --exit-code` 和空 `git status --porcelain --untracked-files=all` 证明零漂移；每个测试层均证明发现目标用例数非零。
- [ ] CI 构建 tarball 并在干净临时目录解包，应用可启动并通过健康检查；归档不含凭据、用户数据或本地生成物。
- [ ] CI 构建 Linux/amd64 镜像，以固定非 root UID/GID 启动，连接外部 MongoDB 8.0，通过明确的健康端点（见 Q-F2）、上传持久化和真实 PDF 生成 smoke；`files/` 与 `public/upload/` 跨重启保留。
- [ ] 一个测试 `vX.Y.Z` tag 在同一 commit 上通过完整质量门后创建 GitHub Release，附带固定命名 tarball、SHA-256 和元数据，并把同版本镜像推到 GHCR；重复 tag/资产不会覆盖。
- [ ] 非 tag 的 PR/push 不发布 Release/镜像；任何工作流都不执行生产部署。
- [ ] 工作流权限、不可变 action/image 引用、缓存键、并发、超时和 artifact 保留期显式配置；固定 job ID 的每个 job 都有 `if: always()` 摘要收尾，汇总 job 在早期失败时生成 fallback，所有摘要通过 `research/ci-failure-summary-schema.md` 的 schema 校验；Playwright 与服务失败只上传 allowlisted 脱敏 job 摘要，服务健康信息只能来自 `service` 字段，最长保留 7 天，且清理失败同样阻断。
- [ ] `docs/modernization/browser-smoke/release-matrix.json` 恰好包含同一 commit 的真实 Chrome、Edge、Firefox、Safari 当前及前一主版本八行唯一键记录；缺失、版本不符、字段/schema 不完整、非真实 Safari 或任一门禁失败都会阻断发布，禁止用占位记录伪造证据。
- [ ] 同一 commit 连续 tarball/镜像构建的完整 SHA-256 和镜像 digest 一致，且 OCI `created`/labels、provenance/attestation/SBOM 策略已固定；解包/拉取后健康、上传和 PDF 复验通过；`.travis.yml`、旧 Revel/Gulp 交付假设和重叠 CI 触发已移除，README/部署文档说明版本矩阵、镜像架构、外部配置、卷和无自动部署边界。

## Open Questions

- **Q-F1（阻断）版本事实源**：当前 `package.json` 是 `1.0.0`，而 `app/service/ConfigService.go:GetVersion()` 返回 `2.6.1`。推荐选择 `package.json` 并在 Go 构建时注入应用显示/健康/OCI metadata；也可反向选择 Go 来源，但必须让 Node 与发布工具读取同一来源。该选择影响 tag、UI、tarball、镜像和回滚识别。
- **Q-F2（阻断）健康端点归属**：推荐由 C-b 提供不需认证的 `GET /healthz`：HTTP 服务和 Mongo ping 均就绪返回 200，否则返回 503，响应不泄露配置；若不采用，必须指定现有端点及其 DB readiness 证明，不能把 `/login` 页面 200 当成数据库健康。
- **Q-F3（阻断）发布并发与重试**：推荐 tag run `cancel-in-progress: false` 且禁止覆盖已有 Release/tag/image；若允许重试或覆盖，需明确人工审批和资产替换边界。

## Out of Scope

- 自动部署、环境审批、回滚生产实例或管理生产密钥。
- Linux/arm64 多架构镜像；后续工作记录在 `docs/modernization-backlog.md`。
- Windows/macOS 容器镜像和托管 MongoDB。
- 将 GitHub Actions 替换为通用发布平台抽象。
