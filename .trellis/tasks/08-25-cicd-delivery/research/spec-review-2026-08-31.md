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
| 版本来源冲突 | `package.json` 为 `1.0.0`；`app/service/ConfigService.go:GetVersion` 返回 `2.6.1`；`conf/app.conf*` 没有统一版本键 | Q-F1 已确认以 `package.json` 顶层 `version` 为唯一机器可读来源；Go release 构建必须 linker 注入同一值，旧硬编码值不得成为第二事实源 |
| 旧交付入口 | `.travis.yml` 仍存在并使用旧 `revel` 流程；`sh/package.sh` 仍调用 `revel package`；`.github/workflows/regression-baseline.yml` 仍有旧 Revel CLI 步骤且触发所有 push | F 必须定义唯一质量门工作流，并明确替换/归并旧 workflow，避免重复运行与旧入口残留 |
| 服务健康 | 审核时当前仓库没有 `/healthz` 或等价健康端点；harness 以 `/login` 作为 readiness | 已确认由 C-b 提供无认证 `GET /healthz`：HTTP 服务监听且 MongoDB ping 成功返回 `200`，否则 `503`；响应不得泄露配置。C-b 实现证据仍是 F 的依赖门禁，`/login` 不能替代该证明 |
| 生产配置现状 | `conf/app.conf-default:27-38` 和 `conf/app.conf:27-38` 含 localhost/公开 `app.secret` 默认值；`app/db/Mgo.go:61-102` 按 `db.url` → `db.urlEnv` → host/port/user/pass 回退并记录 URL；`app/tests/harness/configuration_test.go:83-135` 覆盖环境变量及空值边界 | Q-F4 v1 接口已固定为 `-conf /etc/leanote/app.conf -runMode prod`、只读 `0440`、`MONGODB_URL`/`db.urlEnv`、`LEANOTE_APP_SECRET`/`app.secret` 和 `db.dbname` 与 URI 一致且非 `leanote_test`；运行时注入先解析但冲突不覆盖，配置错误以稳定错误码退出 `78`，Mongo 未就绪由 `/healthz` 返回 `503`，禁止默认/localhost/host-port/未声明别名回退，日志/artifact 脱敏。当前实现仍不符合，C-b 实现证据继续阻断 F |
| 数据目录 | 文件上传和 PDF 产物写入 `files/`，用户主题/头像写入 `public/upload/`；两者均非发布源码 | 镜像必须声明这两个持久化挂载点，测试必须跨重启验证且不把内容打进镜像 |
| 工具链 | 当前 `node --version` 为 v24.20.0，`go version` 为 go1.27.0；既有 workflow 使用 Go 1.26.7/1.27.0 与 Node 24.x | 为可复现交付固定补丁版本；Mongo/action/base image 也必须以摘要或等价不可变标识锁定 |
| 浏览器证据 | `playwright.config.mjs` 的 Chromium `business` 是阻断门；真实四浏览器记录位于 `docs/modernization/browser-smoke/`，当前证据仍缺项 | 发布必须消费可机器校验、绑定 commit 的 8 行矩阵；Chromium/WebKit 不得替代真实 Safari、Chrome 或 Edge |

## 规格缺口与修正方向

1. **依赖完成证明**：把“依赖 status=completed”升级为“归档任务的验收、实现、检查和真实 workflow 证据均闭合”；父任务 `[n/n done]` 不能替代归档。
2. **工作流唯一事实源**：将质量门实现为一个可被 PR/push 与 tag release 调用的 reusable workflow，或明确把现有 regression workflow 改造成该入口；不得留下重叠触发的第二套 CI。
3. **版本契约**：补充版本来源、tag 正则、提交绑定、tarball/镜像命名和冲突处理；Q-F1 已选择 `package.json` 的 `1.0.0`，实现必须移除/拒绝 `2.6.1` 这样的第二来源。
4. **确定性**：补充 `SOURCE_DATE_EPOCH`、Go `-trimpath/-buildvcs=false`、归档排序/owner/group/mode、gzip header、镜像构建平台与 base digest，并以 SHA-256 验证而非只比较文件名。
5. **服务与数据边界**：补充 C-b 无认证 `GET /healthz` 及 HTTP+Mongo `200/503` readiness 语义、响应脱敏、外部 Mongo URL/secret 注入、非 root UID、`files/` 和 `public/upload/` 卷、PDF 工具版本/路径，以及测试数据库只能是隔离的 `leanote_test`。
6. **Q-F4 生产配置接口**：已在 `research/cb-production-config-contract.md` 固定唯一 `/etc/leanote/app.conf` 路径、只读 `0440`、`MONGODB_URL`/`db.urlEnv`、`LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 匹配、来源顺序、稳定错误码和退出 `78`；运行时注入只解析占位符，不覆盖 literal/重复键/未声明来源冲突，校验先于 bind/dial/healthz，日志/artifact 脱敏。F 只能消费该表，不得发明第二套键名或 fallback。
7. **失败与隐私**：补充零测试发现失败、失败摘要 schema、artifact allowlist、原始日志/trace 禁止、清理失败仍返回非零、tag 发布不可覆盖已有资产。
8. **浏览器验收**：定义机器可读 smoke 记录字段与 8 行完整性校验，发布 commit、浏览器产品/完整版本、OS、覆盖、认证/错误门禁和结果缺一即阻断。

## 初始待确认事项（审核时记录，已按后续决策更新）

- **Q-F1 版本事实源（已确认）**：采用 `package.json` 顶层 `version` 作为唯一发布版本源，再由 Go 构建注入应用显示、`/healthz` 和 OCI metadata；`ConfigService.GetVersion()` 不得保留第二个硬编码值。该选择影响应用显示版本、tag 校验、tarball 名称、镜像标签和回滚识别。
- **Q-F2 健康端点归属（已确认）**：采用 C-b 提供不需认证的 `GET /healthz`，仅在 HTTP 服务和 Mongo ping 均就绪时返回 200，否则 503，响应不得泄露配置；F 的 smoke 不得把 `/login` 页面 200 当作数据库健康。C-b 实现证据仍留在依赖核验门禁，不是新的产品决策。
- **Q-F3 发布并发策略（已确认）**：采用按 tag/ref 隔离的 release concurrency group，`cancel-in-progress: false`；已有 Release、资产或镜像 tag 一律拒绝且不得覆盖或自动删除，触发 tag/ref 必须指向当前 commit 且不可 force-update。失败重试、错误资产删除和人工恢复仅按明确记录的人工边界执行。
- **Q-F4 生产配置接口（已确认）**：采用 `research/cb-production-config-contract.md` 的 v1 契约：显式 `-conf /etc/leanote/app.conf -runMode prod`、只读 `0440`、`MONGODB_URL`/`db.urlEnv`、`LEANOTE_APP_SECRET`/`app.secret`、`db.dbname` 与 URI 一致且非 `leanote_test`；运行时注入先解析，literal/重复键/未声明来源冲突不覆盖；配置错误以稳定错误码退出 `78`，Mongo ping 未就绪由 `/healthz` 返回 `503`，禁止默认/localhost/host-port/未声明别名回退，日志/artifact 脱敏。C-b 实现证据仍是启动前证据门，不是待确认的产品决策。

## 最终收敛记录（2026-08-31）

- PRD Goal 与实施计划统一使用精确 `vX.Y.Z` 发布标签；实施计划不再保留泛化标签触发条款，唯一匹配规则为 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`。
- PRD 验收项统一使用固定 Go `1.26.7/1.27.0`，与 R-F1/R-F2、设计和实施计划的矩阵约束一致；`go.mod` 最低语言版本仍明确为 1.26。
- 在本轮用户确认之前，Q-F1、Q-F2、Q-F3 仍是未决用户/上游契约；因此当时任务继续保持
  `planning`，不运行 `task.py start`。Q-F1、Q-F2 与 Q-F3 均已在后续用户回复中确认。

## 审核问题修复记录（2026-08-31）

- 将 tag 校验收敛为 `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`，明确禁止前导零、预发布标识和 build metadata；F 的精确规则明确优先于父任务设计及 ADR-0004 的历史 `v*` 表述。本轮不越界修改父任务或 ADR。
- 在 PRD/design/implement 中补充 BuildKit/OCI `created`、version/revision/source labels、构建平台和 provenance/attestation/SBOM 的确定性约束，避免只固定 tarball 时间而无法复现镜像 digest。
- 新增 `research/ci-failure-summary-schema.md`，定义每个 job 的统一字段、枚举、脱敏边界、`if: always()` 收尾、独立汇总 job 和 checkout/setup 早期失败 fallback；缺摘要、零发现、readiness 或 cleanup 失败均阻断。
- 新增 `research/release-matrix-contract.md`，固定受保护 `browser-release-matrix-v1` artifact 的
  `release-matrix.json` 为唯一发布输入，定义四产品 × 两版本的八行唯一键、完整字段、真实 Safari 要求、
  发布 commit 绑定和禁止占位记录，并同步实现/检查上下文清单。
- 审核发现的 `.gitignore:39` `%USERPROFILE%/` 环境变量写法问题属于实现阶段清理项；按本轮“只改 F 任务规格及研究/验收材料”的边界暂不修改，后续实现任务必须单独处理并验证。

## 复审问题修复记录（2026-08-31）

- 将 `release-matrix-contract.md` 的示例收敛为 draft 2020-12 JSON Schema，固定字段枚举、版本格式、记录数量、真实环境和 RFC3339 UTC 时间；补充 current/previous 主版本以执行时间和官方 stable channel 判定的规则。
- 将服务健康信息收敛进 `ci-summaries/<job-id>.json` 的 `service.health_path`、`readiness`、`http_status` 和 `exit_code` 字段，禁止上传独立健康 artifact，避免出现第二套脱敏/校验契约。
- 固定摘要 job ID 为 `go-1_26_7`、`go-1_27_0`、`mongo-8_0`、`node-build`、`chromium-e2e`、`package-smoke`、`container-smoke` 和 `summary`，并在 schema 与汇总规则中要求完整集合，不再允许未定义的等价 ID。
- 将 Q-F2 收敛为唯一的 C-b 健康契约：无认证 `GET /healthz`，HTTP+Mongo ready 返回 `200`，否则 `503`，响应不泄露配置；package/container smoke 和摘要 schema 禁止以 `/login` 页面 `200` 作为健康证明。
- 将 Q-F3 收敛为不可覆盖的发布并发契约：按 tag/ref 隔离 concurrency、`cancel-in-progress: false`；重复 Release/资产/镜像 tag 预检失败，触发 tag/ref 必须指向当前 commit 且不可 force-update，workflow 不自动重试、删除或补偿，人工恢复必须有明确记录与权限边界。
- 将 Q-F4 收敛为 C-b 唯一生产配置契约：运行时注入优先于挂载 prod 文件，缺失/空值/冲突/公开默认值在 ready 前失败，禁止 localhost、`conf/app.conf`、`leanote_test` 和第二套键名；具体路径、键名、错误码/日志脱敏和实现证据列为启动前依赖材料。

## 第三轮全面规格审核（2026-08-31）

### ready 叶与激活结论

按轨道优先顺序和 `task.json` 机器状态重新计算后，当前 ready 叶数量为 **0**，不能运行
`task.py start`：

| 候选 | 轨道/优先级 | 叶条件 | `meta.depends_on` | 结论 |
| --- | --- | --- | --- | --- |
| `08-25-cicd-delivery` | F / P1 | `planning`，`children=[]` | `08-25-revel-migration` 已归档，但 `08-25-frontend-libs` 仍为 `planning` | blocked |
| `08-25-frontend-libs` | E 协调父任务 / P2 | `planning`，仍有三个 children | `08-25-frontend-build-chain` 已归档 | 不是叶，且组合验收未闭合 |
| `00-bootstrap-guidelines` | 独立 / P1 | `children=[]` | 无 | 已是 `in_progress`，不是可激活的 planning 叶 |

`origin/HEAD` 当前指向 `origin/master`，因此 F 的 `master` 分支判断有现场依据；不能把
父任务 `[3/3 done]` 或归档 `status=completed` 计数当作 F ready 证明。

### 现场依赖复核

- `08-25-revel-migration/task.json` 虽为 `completed`，但归档 PRD 的验收复选框仍未闭合；当前
  `app/cmd/`、`github.com/revel/*`、`revel.`、`sh/run.sh` 和 `sh/package.sh` 仍有命中，且
  `.github/workflows/regression-baseline.yml` 仍构建 Revel CLI。该依赖的实现、验收和 workflow
  证据不满足 F Task 0。
- `08-25-frontend-libs/task.json` 仍为 `planning` 且保留三个 child；`bootstrap-5.3.md` 仅有
  待执行占位，`jquery-3.7.md` 明确缺真实 Safari 与前一主版本，`tinymce-8.md` 明确
  **BLOCKED**；仓库没有 tracked `release-matrix.json`，后续应由受保护证据 workflow 生成 artifact。
- 现有 `regression-baseline.yml` 仍对所有 push 触发，允许手工 `mongo_version`（含 7.0），
  预创建非 schema 的 summary 文件，并运行 Revel CLI/`app/cmd`；F 必须把这些行为列为迁移前
  失败而不能兼容保留。

### 规格一致性与可实施性发现

1. **生产配置接口缺失**：C-b 材料未给出容器/tarball 的配置文件挂载路径、Mongo URL 环境变量、
   `app.secret` 注入键及优先级；当前 `conf/app.conf*` 仍含公开默认值，`db.Init` 仍有 localhost/主机端口
   回退并记录完整 URL。F 原文要求“与 C-b 一致”但无法据此编写 Dockerfile、smoke 或部署文档；Q-F4
   方向现已确认由 C-b 提供唯一契约，运行时注入优先于挂载 prod 文件，缺失/空值/冲突/公开默认值
   在 ready 前 fail closed，F 不创建第二套键名。具体接口表、错误码/日志脱敏和 C-b 实现证据仍是启动前阻断。
2. **浏览器矩阵存在自引用矛盾**：`release-matrix.json` 被要求受跟踪、随 tag commit 存在，
   同时文件内顶层/记录 `commit` 必须等于该 tag commit。提交哈希由完整文件内容决定，无法在不
   重写提交的情况下把最终哈希写入自身；已增加 Q-F5，要求在实现前选择独立不可变证明、tag 后
   受保护记录或明确的两阶段签名流程，禁止手改 SHA/占位记录。
3. **release artifact 交接未闭合**：原规格要求 release 消费 gate 产物，却未定义同一 run 的
   artifact 名称、run/attempt、ref、commit 和 checksum 绑定；已增加 R-F11、design §5.1 和
   implement Task 5 的交叉校验及跨 run/ref 拒绝规则。
4. **网络依赖边界易被误读**：固定本地依赖与 Playwright/基础镜像下载并不矛盾；已在 design
   明确仅允许声明的 `npm ci`、锁定浏览器安装和 digest 拉取联网，其余不得使用 `npx`、全局或
   联网 fallback，失败保留原退出码。
5. **失败摘要约束补强**：每个质量门 job 的 `if: always()` 摘要、早期 fallback、零测试/清理
   失败和固定 job ID 已有 schema；本轮将“手工 Mongo/ref 覆盖不得绕过固定门禁”和“summary
   artifact 只能是当前 run”写入 PRD/implement，保留成功前不生成伪造摘要的规则。

### 未闭合证据门（非需求待确认）

- Q-F4 接口决策已闭合；仍需 C-b 提供实现/验收证据：入口和部署文档、每个稳定错误码与约束的单元测试、进程级退出
  `78`/不 bind/不 dial、无 fallback/完整 URI 日志的静态回归检查，以及合法配置 package/container smoke。缺一仍不得激活 F。

### 第四轮规格收敛（2026-08-31）

- 用户确认采用 Q-F5 推荐方案：矩阵不进入 tag commit，也不在源码中自引用最终 commit；release
  workflow 在精确 tag checkout SHA 上调用受保护真实浏览器证据 workflow，生成一次
  `browser-release-matrix-v1` artifact，allowlist 仅含 `release-matrix.json` 与 `provenance.json`。
- `release-matrix.json` 顶层/记录 `commit` 绑定 tag commit；`provenance.json` 绑定矩阵 SHA-256、
  精确 tag ref、producer workflow 和当前 release run/attempt。release validator 只接受当前 run
  的唯一 artifact，缺失、重复、跨 run/ref、哈希不符、schema/八行门禁失败或非真实 Safari 均阻断，
  workflow 不覆盖或删除，人工恢复遵守 Q-F3 边界。
- 已同步 PRD、design、implement、task.json 和本研究材料；Q-F5 不再是待确认事项。F 仍因 Q-F4
  的 C-b 实现证据、两个依赖的真实完成证据而保持 `planning`，本轮没有运行 `task.py start`。

### 第五轮 Q-F4 规格收敛（2026-08-31）

- 用户采用推荐方案，Q-F4 需求接口现已固定：生产只接受 `-conf /etc/leanote/app.conf -runMode prod`，文件只读 `0440`；
  Mongo/secret 只接受 `MONGODB_URL`/`LEANOTE_APP_SECRET`，分别通过 `db.urlEnv`/`app.secret` 占位引用注入；
  `db.dbname` 必须与 URI 数据库路径一致且不得为 `leanote_test`。
- 来源优先级是“先读取唯一 prod 文件结构，再解析两个运行时环境值”；运行时注入不代表静默覆盖，literal、重复键、未声明
  别名或其他来源冲突一律 `CONFIG_SOURCE_CONFLICT`。配置校验在 HTTP bind/listen、Mongo dial/ping 和 `/healthz` 前完成，
  缺失/不可读/section 或键形态错误/值缺失或为空/公开默认或短 secret/非法 URI 均按研究材料的稳定错误码退出 `78`；
  有效配置但 Mongo ping 失败只产生 Q-F2 的 `/healthz` `503`。日志/artifact 仅允许错误码、非敏感键名和 `run_mode=prod`。
- 已新增并纳入 `implement.jsonl`/`check.jsonl` 的 `research/cb-production-config-contract.md` 作为唯一接口表。当前
  `cmd/leanote/main.go`、`app/db/Mgo.go` 和仓库配置仍存在默认/host-port 回退及完整 URI 日志，因此实现证据不能在审核阶段补写；
  F 继续保持 `planning`，直到 C-b 依赖材料和两个依赖任务的真实完成证据全部闭合。

### 第六轮 C-b 正向实现证据复核（2026-08-31）

- 新增 `research/cb-production-config-evidence.md`，固定 E1-E8 正向证据矩阵、复核入口、artifact
  provenance 和关闭条件。它把规格、局部测试通过和真正的 C-b 正向实现证据分开，禁止用归档状态或占位记录勾选通过。
- 已运行的局部命令为 `go test ./cmd/leanote ./app/httpserver -run 'Test(ValidateProdSecret|Config|LoadConfig|ParseConfig)' -count=1`
  和 `go test ./app/tests/harness -run 'Test(.*Configuration|.*Config)' -count=1`，均通过；这些测试只覆盖
  通用配置解析、公开/空 secret 和 `leanote_test` harness 规则，不能证明 E1/E2/E4/E5/E7 的 canonical prod 入口、
  `0440`、稳定错误码/退出 `78`、端口/Mongo 顺序或 `/healthz`。
- E1/E2/E4/E5/E6/E7/E8 当前仍为 `FAIL` 或 `MISSING`：入口默认 `conf/app.conf`/`dev`，URI/secret 约束不完整，
  无稳定错误码和退出 `78`，存在 host/port fallback 与完整 URI 日志，无 `/healthz`，C-b 归档验收和真实 workflow 证据未闭合。
  因此不能把 C-b 正向实现证据标为通过；F 仍停留 `planning`，不运行 `task.py start`。

本轮仅修改 `.trellis/tasks/08-25-cicd-delivery/` 的 PRD、设计、执行计划、研究材料和 `task.json`；未
修改业务实现、CI workflow、生产配置、测试代码、依赖任务或运行状态，未激活任何任务。
