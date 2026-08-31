# F 规格审核研究（2026-08-31）

## 审核结论

当前 F（`08-25-cicd-delivery`）是 P1/F 轨道的唯一规划叶，但不满足 ready：

- `task.json.meta.depends_on` 声明 `08-25-revel-migration` 与 `08-25-frontend-libs`。
- `08-25-frontend-libs/task.json` 仍为 `planning`，`[3/3 done]` 只代表子任务进度；其协调验收仍要求真实 Chrome、Edge、Firefox、Safari 的当前及前一主版本证据。现有 `docs/modernization/browser-smoke/jquery-3.7.md` 明确缺 Safari、缺前一主版本，`tinymce-8.md` 也明确为 BLOCKED。
- `archive/2026-08/08-25-revel-migration/task.json` 虽为 `completed`，但其归档 PRD 的验收清单仍未勾选；当前源码仍有 `app/cmd/`、`github.com/revel/*` 和 `revel.` 命中。因此归档状态不能作为 F 的实现前置证明。

因此本轮不运行 `task.py start`，也不修改业务实现。

## 已核实事实

| 主题 | 证据 | 对 F 的约束 |
| --- | --- | --- |
| 基线 Mongo | `archive/2026-08/08-25-regression-baseline/prd.md` 的 G-AC8/R-G5 固定 MongoDB 5.0；`archive/2026-08/08-25-mongo-driver-migration/design.md` 将 CI 切到 8.0 | F 复用 Golden/USN/harness 契约，但 8.0 运行时来自 B 迁移后的 harness，不得声称来自 G 的 8.0 初始化 |
| 版本来源冲突 | `package.json` 为 `1.0.0`；`app/service/ConfigService.go:GetVersion` 返回 `2.6.1`；`conf/app.conf*` 没有统一版本键 | 必须先确定唯一机器可读来源，并让 tag、UI、tarball 和 OCI labels 使用同一值 |
| 旧交付入口 | `.travis.yml` 仍存在并使用旧 `revel` 流程；`sh/package.sh` 仍调用 `revel package`；`.github/workflows/regression-baseline.yml` 仍有旧 Revel CLI 步骤且触发所有 push | F 必须定义唯一质量门工作流，并明确替换/归并旧 workflow，避免重复运行与旧入口残留 |
| 服务健康 | 当前仓库没有 `/healthz` 或等价健康端点；harness 以 `/login` 作为 readiness | 容器/打包 smoke 需要明确健康端点及 DB readiness 语义；端点归属需与 C-b 对齐，不能由 F 隐式假设 |
| 数据目录 | 文件上传和 PDF 产物写入 `files/`，用户主题/头像写入 `public/upload/`；两者均非发布源码 | 镜像必须声明这两个持久化挂载点，测试必须跨重启验证且不把内容打进镜像 |
| 工具链 | 当前 `node --version` 为 v24.20.0，`go version` 为 go1.27.0；既有 workflow 使用 Go 1.26.7/1.27.0 与 Node 24.x | 为可复现交付固定补丁版本；Mongo/action/base image 也必须以摘要或等价不可变标识锁定 |
| 浏览器证据 | `playwright.config.mjs` 的 Chromium `business` 是阻断门；真实四浏览器记录位于 `docs/modernization/browser-smoke/`，当前证据仍缺项 | 发布必须消费可机器校验、绑定 commit 的 8 行矩阵；Chromium/WebKit 不得替代真实 Safari、Chrome 或 Edge |

## 规格缺口与修正方向

1. **依赖完成证明**：把“依赖 status=completed”升级为“归档任务的验收、实现、检查和真实 workflow 证据均闭合”；父任务 `[n/n done]` 不能替代归档。
2. **工作流唯一事实源**：将质量门实现为一个可被 PR/push 与 tag release 调用的 reusable workflow，或明确把现有 regression workflow 改造成该入口；不得留下重叠触发的第二套 CI。
3. **版本契约**：补充版本来源、tag 正则、提交绑定、tarball/镜像命名和冲突处理；`1.0.0` 与 `2.6.1` 的选择仍待用户确认。
4. **确定性**：补充 `SOURCE_DATE_EPOCH`、Go `-trimpath/-buildvcs=false`、归档排序/owner/group/mode、gzip header、镜像构建平台与 base digest，并以 SHA-256 验证而非只比较文件名。
5. **服务与数据边界**：补充健康端点、外部 `MONGODB_URL`/secret 注入、非 root UID、`files/` 和 `public/upload/` 卷、PDF 工具版本/路径，以及测试数据库只能是隔离的 `leanote_test`。
6. **失败与隐私**：补充零测试发现失败、失败摘要 schema、artifact allowlist、原始日志/trace 禁止、清理失败仍返回非零、tag 发布不可覆盖已有资产。
7. **浏览器验收**：定义机器可读 smoke 记录字段与 8 行完整性校验，发布 commit、浏览器产品/完整版本、OS、覆盖、认证/错误门禁和结果缺一即阻断。

## 待确认事项（不擅自假设）

- **Q-F1 版本事实源（阻断）**：推荐选 `package.json` 作为唯一发布版本源，再由 Go 构建注入 UI/health/OCI metadata；若选择 `ConfigService.GetVersion()`，则需反向让 Node/发布工具读取同一来源。选择会影响应用显示版本、tag 校验、tarball 名称、镜像标签和回滚识别。
- **Q-F2 健康端点归属（阻断）**：推荐由 C-b 提供不需认证的 `GET /healthz`，仅在 HTTP 服务和 Mongo ping 均就绪时返回 200，否则 503，响应不得泄露配置；若不接受，需要明确 F 使用的现有端点及其 DB readiness 证明，且不能把 `/login` 的页面 200 当作数据库健康。
- **Q-F3 发布并发策略（阻断）**：推荐 release tag 运行 `cancel-in-progress: false` 且禁止覆盖已存在的 Release/tag/image；若允许重试或覆盖，需明确人工审批和资产替换边界。

## 最终收敛记录（2026-08-31）

- PRD Goal 与实施计划统一使用精确 `vX.Y.Z` 发布标签；实施计划不再保留泛化标签触发条款，唯一匹配规则为 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`。
- PRD 验收项统一使用固定 Go `1.26.7/1.27.0`，与 R-F1/R-F2、设计和实施计划的矩阵约束一致；`go.mod` 最低语言版本仍明确为 1.26。
- Q-F1、Q-F2、Q-F3 仍是未决用户/上游契约，不在规格审核阶段擅自选择；因此任务继续保持 `planning`，不运行 `task.py start`。

## 审核问题修复记录（2026-08-31）

- 将 tag 校验收敛为 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`，明确禁止前导零、预发布标识和 build metadata；F 的精确规则明确优先于父任务设计及 ADR-0004 的历史 `v*` 表述。本轮不越界修改父任务或 ADR。
- 在 PRD/design/implement 中补充 BuildKit/OCI `created`、version/revision/source labels、构建平台和 provenance/attestation/SBOM 的确定性约束，避免只固定 tarball 时间而无法复现镜像 digest。
- 新增 `research/ci-failure-summary-schema.md`，定义每个 job 的统一字段、枚举、脱敏边界、`if: always()` 收尾、独立汇总 job 和 checkout/setup 早期失败 fallback；缺摘要、零发现、readiness 或 cleanup 失败均阻断。
- 新增 `research/release-matrix-contract.md`，固定 `docs/modernization/browser-smoke/release-matrix.json` 为唯一输入，定义四产品 × 两版本的八行唯一键、完整字段、真实 Safari 要求、发布 commit 绑定和禁止占位记录，并同步实现/检查上下文清单。
- 审核发现的 `.gitignore:39` `%USERPROFILE%/` 环境变量写法问题属于实现阶段清理项；按本轮“只改 F 任务规格及研究/验收材料”的边界暂不修改，后续实现任务必须单独处理并验证。

## 复审问题修复记录（2026-08-31）

- 将 `release-matrix-contract.md` 的示例收敛为 draft 2020-12 JSON Schema，固定字段枚举、版本格式、记录数量、真实环境和 RFC3339 UTC 时间；补充 current/previous 主版本以执行时间和官方 stable channel 判定的规则。
- 将服务健康信息收敛进 `ci-summaries/<job-id>.json` 的 `service.health_path`、`readiness`、`http_status` 和 `exit_code` 字段，禁止上传独立健康 artifact，避免出现第二套脱敏/校验契约。
- 固定摘要 job ID 为 `go-1_26_7`、`go-1_27_0`、`mongo-8_0`、`node-build`、`chromium-e2e`、`package-smoke`、`container-smoke` 和 `summary`，并在 schema 与汇总规则中要求完整集合，不再允许未定义的等价 ID。
