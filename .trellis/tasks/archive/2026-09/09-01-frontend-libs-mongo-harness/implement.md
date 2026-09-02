# B-E2 实施计划

## Task 0: Activation Gate

- [x] design.md / implement.md 已按 workflow.md 复杂任务护栏补齐（契约在 PRD，机制在 design.md）。

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`）并通过双轴 code-review（计数/隔离表述/删除补录等发现已修复）。
- [x] 用户已批准激活实现（"批准激活实现"，2026-09-02）；`task.py start` 已运行，状态 `in_progress`。

## Global Constraints

- 实现阶段允许修改 `app/tests/harness/**`（含 `cmd/e2e`、`cmd/env`）与回归测试；不改业务 API、Mongo 数据结构与 workflow 触发器（现有 `LEANOTE_REQUIRE_MONGO=1` 命令行已足够）。
- 模式判定按调用点读取环境，不得包级缓存（configuration_test 的 `t.Setenv` 窗口约束）。
- 复验基线：`99abfab` 谱系；提交在 `dev` 上，每提交聚焦单一目的。

## Task 1: 模式判定与 service-backed 路径

- [x] harness 增加三态判定（设计见 design.md §2）：`LEANOTE_REQUIRE_MONGO=1` ⇒ service-backed（零 docker 调用，消费默认 URI 或 `LEANOTE_TEST_MONGO_URL` 覆盖，覆盖 URI 库名必须恰为 `leanote_test`）；未设 ⇒ 自建（现状 `Up()`）。
- [x] service-backed 的每测试隔离恢复（机制与取舍见 design.md §3）：与自建模式的容器内恢复语义等价。
- [x] `startBaselineServer` 接入判定（单点门控，11 处调用点零改动；`fixtureDatabase` 直连改用解析 URI）。

## Task 2: fail-closed 校验与 supervisor 预检

- [x] URI 库名 ≠ `leanote_test`、service ping 不通、宿主 mongorestore 缺失——启动前明确非零失败（`RestoreServiceFixture` fail-closed 次序）。
- [x] e2e supervisor `Up()` 前显式断言 27017 无监听并拒绝 REQUIRE/URL service 声明（`assertSupervisorEnvironment`）。

## Task 3: 回归用例（fake `commandRun`，environment_test.go 先例）

- [x] REQUIRE=1 路径零 docker 调用断言（`TestRestoreServiceFixtureNeverInvokesDocker`）。
- [x] 各 fail-closed 错误路径（URI 库名、ping、缺工具、端口占用、来源冲突、supervisor 声明）单元覆盖。
- [x] 隔离恢复断言：恰好一条 mongorestore 命令且参数序确定；失败路径零恢复命令。

## Task 4: 本地与 CI 验证

- [x] 本地自建模式全绿：`go test ./app/tests/... -count=1 -p 1 -timeout 25m`（harness 103.6s / auth ok / cmd-e2e ok，Docker 29.7.2）。
- [x] CI [mongo-8_0 job `100081573898`](https://github.com/yangphere/leanote/actions/runs/33576516744/job/100081573898) success：日志禁行 0，三包 `ok`（app/tests 0.008s、harness 34.853s、cmd/e2e 0.004s）。
- [x] 本地无宿主 mongorestore，跳过（按计划标注为可选；CI 为最终证据）。

## Task 5: Provenance 与交接

- [x] 修复提交 `073127f`（材料 `16099ba`）；provenance 已填入 PRD AC；记录仅含脱敏 URI（`SanitizeMongoURI` 剥离凭据）。
- [x] 待归档确认后由 E 收口阶段重置 AC-E5；B-E3 现在可将 chromium 失败归因于编辑器行为（harness 生命周期已消歧：run 33576516744 的 chromium 失败不再涉及 Mongo 冲突）。

## Completion Gate

- [x] PRD 全部 AC 勾选；无业务 API/数据结构改动（diff 仅 app/tests/harness/**）；两模式隔离等价有测试与 CI 证据。
