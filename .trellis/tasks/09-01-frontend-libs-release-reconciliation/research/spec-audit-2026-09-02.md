# B-E6 规格审核与根因核实（2026-09-02）

## 结论

三个失败 job 的根因全部定位到行级（一个脚本条件缺陷、一个参数类型误用、一个 summary 默认值语义错误），均可小修恢复；F 状态冲突只能登记不能篡改；**最终发布结论必须保持阻断**（B-E5 裁决后 8 槽 artifact 不可得）。PRD 按此重写；无用户裁决项（发布阻断是既定契约的推论，非新决策）。

## Ready Selection Evidence

B-E1..B-E5 全部归档（B-E5 为 blocked 形态），B-E6 `meta.depends_on` 全满足，是最后一叶。E 显示 [9/10 done]。

## 三 job 根因（最新 run 33604371491 证据 + 行级定位）

### D1 package-smoke：分支名被当 tag 断言

- 证据：job `100165108289` `release tag must match vX.Y.Z: dev`，exit 1。
- 根因：`sh/package.sh:7-9`——`TAG=${RELEASE_TAG:-${GITHUB_REF_NAME:-}}`；dev push 的 `GITHUB_REF_NAME=dev` 非空 → `node scripts/version.mjs "$TAG"` 触发 `version.mjs:21` 断言。脚本本意是"有 tag 上下文才校验"，但 `GITHUB_REF_NAME` 对分支 push 也非空。
- 修复方向：仅当 `GITHUB_REF` 以 `refs/tags/` 开头（或显式 `RELEASE_TAG`）才进入断言；tag 上下文（release.yml 路径）语义不变。

### D2 container-smoke：RFC3339 传入整型 `SOURCE_DATE_EPOCH`

- 证据：job `100165108314` `invalid SOURCE_DATE_EPOCH: 2026-09-02T07:35:44Z: strconv.ParseInt ... invalid syntax`。
- 根因：quality-gate.yml "Build deterministic container candidate" 步骤 `created=$(date -u -d "@$epoch" +RFC3339)` 后 `--build-arg SOURCE_DATE_EPOCH="$created"`；buildkit 要求整型秒。Dockerfile:21 `ARG SOURCE_DATE_EPOCH` 同时被 :24 用作 OCI `org.opencontainers.image.created` 标签（该处 RFC3339 才是对的）——单参数承载两种类型需求是缺陷本源。
- 修复方向：`SOURCE_DATE_EPOCH="$epoch"`（整数）+ 新增 `ARG OCI_CREATED`（RFC3339）专供标签；workflow 同步传两 arg。

### D3 summary：失败 job 被写成 `job_not_started`

- 证据：最新 run 的 `ci-summary-container-smoke` artifact 实测 `status:failed / stage:job_not_started / category:job_not_started`，而该 job 实际运行到 docker build 才失败（E 的 AC-E8 原始投诉至今存活）。summary job 的 `Error: quality-gate job container-smoke failed` 是 validate-summaries 对失败 job 的正确传播，随 D1/D2/D3 修复转绿。
- 根因：`scripts/ci/write-summary.mjs:104`——`stage: forcedFallback ? 'job_not_started' : (CI_STAGE || (status==='passed' ? 'complete' : 'job_not_started'))`；失败且未显式传 `CI_STAGE` 时一律 `job_not_started`，且 workflow 各 Write summary 步骤从不设置 `CI_STAGE`。
- 修复方向：write-summary 成功执行（非 forcedFallback）即证明 job 已启动——stage 默认 `complete`（生命周期事实），失败细节由 `failure.category/message/exit_code` 承载；forcedFallback 路径保持 `job_not_started` 语义不变（summary 生成本身失败）。同步补契约测试。

### D4 F 状态冲突（登记，不篡改）

F（`archive/2026-09/08-25-cicd-delivery`）`status=completed` 与其 notes"上游真实证据阻断"并存。PRD Req 3 明确禁止篡改；归档任务无合法 unarchive 生命周期命令。处置=E evidence matrix 与父任务 notes 登记"事实冲突、发布阻断维持"（F 历史记录原样）。

### D5 发布阻断推论（既定契约，非新决策）

B-E5 裁决跳过 Safari → 8 槽 `browser-release-matrix-v1` 不可得 → F 发布门（Req 4/AC-4）保持阻断。B-E6 的终局结论形态：**D1-D3 修复后 dev push 上三门禁全绿 + F 冲突登记 + 发布结论 blocked（缺 8 槽 artifact）**。不得宣称 `eligible_for_completion`/发布获批。

## 影响面清单（design 落细）

`sh/package.sh`（D1）、`quality-gate.yml` 与 `release.yml`（D2 同型构建 arg 两处）、`Dockerfile`（D2 拆 arg）、`scripts/ci/write-summary.mjs`（D3）、`tests/js/release-contract.test.js` + `tests/js/` summary 契约测试（D1-D3 锁步）。不碰 release.yml 发布语义、不碰 browser producer/validator。

### 评审后补录（2026-09-02 code-review 双轴）

- Design §1 片段重写为 `set -eu` 安全的 `case` 版本（原 `${GITHUB_REF#refs/tags/}` 无守卫展开在无 `GITHUB_REF` 的本地执行下触发 unbound variable）。
- D2 影响面补全：release.yml:102 存在同型 RFC3339→整型缺陷，纳入修复（参数类型缺陷≠发布语义变更）；Dockerfile 标签无空值路径（两 workflow 恒传 `OCI_CREATED`）。
- 恢复被重写静默丢弃的三项旧需求：package/container 验证范围枚举（tarball/SHA-256/OCI/非 root/外部 Mongo/持久化路径/PDF smoke）、发现/执行数量与失败 owner 记录、两阶段 artifact 分离规则（现 Req 6/7）。

## 实现期调查补录（2026-09-02，四轮 CI 诊断轮）

D1-D3 修复落地后，package/container smoke 首次真正运行到应用启动段，揭开多层潜伏缺陷并逐层修复：

1. **执行位缺失**（`37c2ee01`）：两 smoke 脚本 100644 但被 workflow 直接调用 → exit 126；`git update-index --chmod=+x` + mode 回归断言。
2. **诊断能力**（`fe734046`/`94525989`）：应用日志此前被完全吞掉，违反"失败保留原始原因"；package-smoke 落盘 app.log 失败时输出尾部 40 行，container-smoke 输出 docker logs 尾部。
3. **readiness deadline**（`353f28ed`）：60s→180s（冷启动保护；后续轮证明启动仅 ~3s，此项保留为防御性余量）。
4. **未解之谜（诚实登记为 blocked）**：run `33626805206`（`353f28ed`）诊断轮：应用 3 秒启动并打印 `leanote starting`（initDatabase 表观成功，无 `mongo readiness unavailable` 或任何 ERROR 日志行），server 监听 19090 并响应 healthz，**但返回 503 not_ready**（`db.Ping` 失败）——与启动期 dialMongo 的 Ping 成功矛盾。此前轮次的 60s "慢启动"表象实为该 503 路径立即使 EXPECT_READY=true 断言失败所致。容器 smoke 同症状（curl 56 重置后同路径）。
   - 复验命令：push 后观察 run；或本地复现需 canonical `/etc/leanote/app.conf`（Windows 不可直建，建议 WSL/Linux 环境 + 打包 tarball + `/etc/hosts` 加 `mongo-smoke.internal`）。
   - owner：B-E6（深挖需新会话/诊断轮；候选方向：`db.Ping` 与 `dialMongo` 的 Ping 语义差异、healthz 处理时 client 状态、503 响应来源验证）。

## 审核过程 provenance

- `gh run view 33604371491` 三 job 日志（错误行精确到步）；`ci-summary-container-smoke` artifact 下载实测 job_not_started。
- `sh/package.sh`、`scripts/version.mjs`、`Dockerfile:21/24`、`write-summary.mjs:104`、quality-gate.yml package/container/summary 三 job 步骤序逐行读。
- F 归档 task.json/notes 复核；B-E5 归档结论（8 槽不可得）作为 D5 输入。
- 未修改任何实现；未激活任务。
