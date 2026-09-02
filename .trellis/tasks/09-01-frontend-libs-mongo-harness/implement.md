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

- [ ] harness 增加三态判定（设计见 design.md §2）：`LEANOTE_REQUIRE_MONGO=1` ⇒ service-backed（零 docker 调用，消费默认 URI 或 `LEANOTE_TEST_MONGO_URL` 覆盖，覆盖 URI 库名必须恰为 `leanote_test`）；未设 ⇒ 自建（现状 `Up()`）。
- [ ] service-backed 的每测试隔离恢复（机制与取舍见 design.md §3）：与自建模式的容器内恢复语义等价。
- [ ] `startBaselineServer` 接入判定（11 处调用点行为不变或改善，不逐点复制逻辑）。

## Task 2: fail-closed 校验与 supervisor 预检

- [ ] URI 库名 ≠ `leanote_test`、service ping 不通、宿主 mongorestore 缺失——启动前明确非零失败。
- [ ] e2e supervisor `Up()` 前显式断言 27017 无监听，占用时明确报错（替代 docker 125）。

## Task 3: 回归用例（fake `commandRun`，environment_test.go 先例）

- [ ] REQUIRE=1 路径零 docker 调用断言。
- [ ] 各 fail-closed 错误路径（URI 库名、ping、缺工具、端口占用）单元覆盖。
- [ ] 隔离恢复调用次数/顺序断言（每测试一次 `--drop`）。

## Task 4: 本地与 CI 验证

- [ ] 本地（Windows/WSL，未设 REQUIRE）自建模式全绿：`go test ./app/tests/... -count=1`。
- [ ] CI `mongo-8_0` job 全绿，日志无 `leanote-test-mongo` / `port is already allocated`；记录 run/job URL 与发现/执行数。
- [ ] 若本地具备 service+mongorestore 条件，补一次 REQUIRE=1 本地验证（可选，CI 为最终证据）。

## Task 5: Provenance 与交接

- [ ] 记录修复提交 SHA、CI run/job、模式与脱敏参数，填入 PRD AC 勾选。
- [ ] 通知 E：AC-E5 retest 输入就绪；B-E3 以本任务输出为 harness 前提。

## Completion Gate

- [ ] PRD 全部 AC 勾选；无业务 API/数据结构改动；两模式隔离等价有测试与 CI 证据。
