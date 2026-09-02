# B-E2 统一 Mongo service 与 E2E harness 生命周期

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E2　优先级：P0　执行序号：2　责任 owner：Go/Mongo harness
基线分支：`dev`　复验基线：`99abfab` 谱系（B-E1 已归档）
技术设计见 `design.md`，证据链见 `research/spec-audit-2026-09-02.md`。

## Goal

建立互斥且可审计的 MongoDB 测试环境选择，消除 CI 已提供 service 与测试自建 harness 同时抢占固定 `27017` 导致 Golden、USN、所有权和 note-save 套件失败的问题。

## Confirmed Defect

- [mongo-8_0 job `99904829590`](https://github.com/yangphere/leanote/actions/runs/33522450969/job/99904829590)（run 33522450969，`99abfab`；原始缺陷 run `33477561244` / job `99759909276` 同因）：job 级 fixture 恢复进 service 成功后，`startBaselineServer` 测试（golden/note-save/usn/smoke，全仓共 11 处调用；CI 日志 10 个失败——golden_test.go:138 的 ExportPdf 在缺 wkhtmltopdf 时先 `t.Skipf`，未触达 `Up()`）在 `docker run -d --rm --name leanote-test-mongo -p 27017:27017` 处 `exit status 125`，`Bind for 0.0.0.0:27017 failed: port is already allocated`。
- 根因：`MongoEnvironment.Up()` 无条件自建容器，而 CI 已以 service 占用 27017；harness 无任何模式选择机制。
- `app/tests/auth_test.go` 已是 service 兼容形态（拨号门控 + 直连），不在修复范围内。

## Dependencies And Order

- 前置 B-E1 已归档（mode 漂移门禁全绿）；本任务在 `99abfab` 谱系上实施。
- 本任务完成后，B-E3 才可将 Chromium/Go 失败归因于编辑器行为，而不是 harness 生命周期。

## Requirements

1. **模式互斥**：三种运行形态互斥且由运行环境声明，禁止探测式 fallback——`LEANOTE_REQUIRE_MONGO=1` ⇒ service-backed（**禁止任何 docker 调用**，消费默认 URI 或 `LEANOTE_TEST_MONGO_URL` 覆盖，覆盖 URI 库名必须恰为 `leanote_test`）；未设 ⇒ 自建（现状保持，本地 DX 不变）；e2e supervisor 恒为自建且 `Up()` 前必须显式断言 27017 无监听。模式判定按调用点读取环境，不得包级缓存（configuration_test 的 `t.Setenv` 窗口约束；机制见 design.md §2）。
2. **隔离等价**：job 级 bootstrap 恢复（CI 现有步骤，服务非 harness 测试）与 harness 每测试隔离恢复两层并存、不得混淆；service-backed 的每测试隔离恢复不得依赖 docker；两模式下 11 处 `startBaselineServer` 调用点的隔离语义必须等价（手段与取舍见 design.md §3）。
3. **fail-closed**：URI 库名 ≠ `leanote_test`、service ping 不通、service 模式缺恢复工具、supervisor 自建前端口被占——均须在启动前返回明确非零失败，不 fallback 到默认库或隐式等待（校验次序见 design.md §4）。
4. **清理与回归用例**：运行中或失败后必须清理，清理失败单独记为失败并保留容器/进程归属与复验入口；修复必须包含聚焦回归用例，至少单元级证明 service 路径零 docker 调用与各 fail-closed 错误路径（用例设计见 design.md §5）。
5. **记录**：运行记录只保存脱敏 URI（无凭据）、模式、数据库名、命令、发现/执行数量与 run/job provenance。

## Acceptance Criteria

- [x] 单元级：`mongo_mode_test.go` 的零 docker 断言（恰好一条 mongorestore 命令、无 docker 前缀）+ 凭据擦除断言（restore 失败不含 user:secret@、保留脱敏 URI）通过；CI 级：[mongo-8_0 job `100081573898`](https://github.com/yangphere/leanote/actions/runs/33576516744/job/100081573898)（run 33576516744，`073127f`）success，日志禁行计数 0；评审加固后复验 [job `100090082722`](https://github.com/yangphere/leanote/actions/runs/33579336426/job/100090082722)（run 33579336426，`d37e3b8`）再次 success、禁行 0、三包 ok（harness 37.6s）。
- [x] 隔离等价：自建模式本地全量通过（harness 103.6s，容器生命周期完整）；service-backed 模式 CI 全量通过（harness 34.9s，评审加固后 37.6s；宿主每测试 --drop 恢复 + users==2 等价校验），三包全部 `ok` 且 0 失败。
- [x] 自建独占与清理：本地全量 11 处调用点 Up/Down 成对通过；supervisor 预检：`AssertPortFree` 单元测试（占用→明确报错，空闲→nil）+ 守卫拒绝 service 声明两分支测试通过。
- [x] fail-closed 单元级全覆盖：URI 库名≠leanote_test、非 mongodb scheme、ping 不通（unreachable）、缺宿主 mongorestore、恢复后 users 校验失败、REQUIRE 未设但 URL 已设（两种来源混合）、supervisor 检测到 REQUIRE/URL 声明——各分支断言错误语义，且失败路径零恢复命令执行。
- [x] [run 33576516744 / job `100081573898`](https://github.com/yangphere/leanote/actions/runs/33576516744/job/100081573898) success；复验命令即该 job 的 `LEANOTE_GOLDEN=replay LEANOTE_REQUIRE_MONGO=1 go test -p 1 ./app/tests/... -count=1 -timeout 30m`。

## Out Of Scope

- 不修改 MongoDB collection、字段、BSON 类型或业务 API。
- 不修复 TinyMCE/编辑器行为、浏览器 coverage、package/container 发布流程；chromium-e2e job 的完整复验属 B-E3。
- 不新增或修改 workflow 触发器（现有 `LEANOTE_REQUIRE_MONGO=1` 命令行已足够）；不改 E 验收矩阵（E 自行重置）。

## Handoff And Retest

将唯一模式协议、隔离等价与一次 bootstrap 恢复证据交给 B-E3 与 E 验收；任何并行启动 helper 或清理遗漏都必须重新跑完整 service/harness 选择测试。
