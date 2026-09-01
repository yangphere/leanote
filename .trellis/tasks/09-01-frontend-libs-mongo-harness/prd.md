# B-E2 统一 Mongo service 与 E2E harness 生命周期

父任务：`.trellis/tasks/08-25-frontend-libs`
阻断编号：B-E2　优先级：P0　执行序号：2　责任 owner：Go/Mongo harness

## Goal

建立互斥且可审计的 MongoDB 测试环境选择，消除 CI 已提供 service 与 Chromium 自建 harness 同时抢占固定 `27017` 导致 Golden、USN、所有权和 note-save 套件失败的问题。

## Confirmed Defect

- CI `33477561244` / attempt `1` 的 Mongo job 已提供 MongoDB 8.0 service，但测试仍启动 `leanote-test-mongo`，形成第二个固定端口占用。
- 测试数据库必须是 `leanote_test`；同时发现 service URI 与 supervisor、或 URI 指向其他数据库时必须在应用启动前失败。

## Dependencies And Order

- 前置：B-E1（构建可重复，避免环境修复被生成物漂移污染）。
- 本任务完成后，B-E3 才可将 Chromium/Go 失败归因于编辑器行为，而不是 harness 生命周期。

## Requirements

1. 明确且只能选择一种模式：
   - service-backed：消费 `mongodb://127.0.0.1:27017/leanote_test` 或显式等价 `LEANOTE_TEST_MONGO_URL`，恢复 fixture 一次并设置 `LEANOTE_REQUIRE_MONGO=1`，禁止调用 `NewMongoEnvironment.Up()`；
   - Chromium 自建 harness：仅在没有外部 service/URI 时由 supervisor 独占 `leanote-test-mongo`，负责创建和销毁。
2. 在应用/测试启动前校验来源互斥、数据库名、连接可达性和凭据环境；冲突、缺失或错误数据库必须 fail closed，不得 fallback 到默认库。
3. 运行中或失败后都必须执行清理；清理失败单独记为失败，并保留容器/进程归属和复验入口。
4. 运行记录只保存脱敏 URI（不得包含凭据）、模式、数据库名、命令、发现/执行数量与 run/job provenance。

## Acceptance Criteria

- [ ] service-backed 模式不启动第二个固定端口容器，Go/Mongo 套件发现数大于零并通过。
- [ ] 无外部 service 时，自建模式独占容器并在退出时完成清理；未提供模式时不启动应用。
- [ ] 同时检测到两种来源、URI 非 `leanote_test`、认证失败或端口冲突时均在启动前返回明确非零失败。
- [ ] 候选 CI `mongo-8_0` job 通过，或保留失败根因、owner、run/job 和可重复复验命令。

## Out Of Scope

- 不修改 MongoDB collection、字段、BSON 类型或业务 API。
- 不修复 TinyMCE、浏览器 coverage、package/container 发布流程。

## Handoff And Retest

将唯一模式协议和一次 fixture 恢复证据交给 B-E3 与 E 验收；任何并行启动 helper 或清理遗漏都必须重新跑完整 service/harness 选择测试。
