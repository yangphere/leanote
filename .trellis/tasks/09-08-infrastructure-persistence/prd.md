# 基础设施层：MongoDB 持久化边界 — PRD

## Goal

把 MongoDB 驱动、查询/更新、ObjectID、BSON、超时和错误映射收敛到 `app/db` 的单一基础设施边界，让应用服务不再感知具体 driver。

## Requirements

- 使用官方 `mongo-driver/v2`，支持 MongoDB 7.0–8.0，保持 collection、字段名、BSON 类型和排序分页语义。
- 明确表达 Find/Sort/Skip/Limit/One/All/Count/Insert/Update/UpdateAll/Upsert/Remove/Select 等现有用法。
- not-found、duplicate key、空集合、更新结果和 ObjectID 表现与领域契约一致。
- 负责领域中立 ObjectID/时间值与 Mongo BSON 的单向转换；领域包不得反向引入 driver。
- 每次 DB 操作使用显式有界 timeout；全链路 request context 传播另记 MOD-001。
- 业务 service 不直接持有 driver client/collection/query 类型。
- 为 identity service 提供用户初始化事务边界；Mongo transaction 不可用时提供按 `userId` 幂等的补偿/重试结果，不把部分写入报告为成功。
- transaction callback 可能被 driver 重试；每次 callback 必须清理 attempt 状态，只有最终 commit 才能报告 `Committed=true`。仅明确的 standalone capability 错误（如 code 20 且含 replica-set/mongos capability 语义）允许转 compensation；`251`、`263` 等事务状态/瞬时错误不得降级重放。
- 为注册和找回密码提供持久化 outbox：用户/必需初始化与 outbox 入队处于同一成功边界；worker 传输失败可重试并可观测，禁止请求路径启动无结果 goroutine。
- outbox 必须声明明确状态机：`pending/retry -> sending -> sent|retry|dead`。`sending` 携带 lease（`LeaseUntil` 和每次 claim 新生成的 `LeaseId`），worker 崩溃或解码失败后可被后续 worker reclaim；状态写回必须同时匹配 `LeaseId`，过期 worker 不得覆盖已 reclaim 的状态。解码失败也必须保留本次 claim 后的 attempt 计数，不能绕过 10 次上限。传输失败使用独立的状态更新 timeout，不得因 transport context 已超时而永久卡在 `sending`。重试上限为 10 次，超过后进入 `dead` 并告警，不得再次自动投递。
- outbox 采用至少一次投递语义；调用方必须提供稳定 `IdempotencyKey`（无 key 时由调用方保留首次生成的 event `_id`），同一 key 的未知结果重试必须返回既有事件，不得创建第二个事件。Payload 通过普通 `map[string]any` 暴露，不向 service 泄露 driver `bson.M`。
- 建立 `sessions.UpdatedTime` 的 10800 秒 TTL index，并支持 idle TTL 的原子刷新；应用层按 `now >= UpdatedTime+3h` 先行拒绝过期 session。
- 为 action token 建立独立文档标识和 `(UserId,Type)` 唯一约束；同一用户/用途再次签发时原子替换旧 active token，旧 `_id=UserId` 文档提供只读兼容或明确迁移计划。
- 旧 `_id=UserId` token 的重签发策略固定为“先原子标记旧文档 `ConsumedAt` 使其失效，再以独立 `_id` 写入新文档”；若任一步失败必须返回错误并可观测，不能同时保留两个 active token。旧文档只读兼容仅适用于未被重签发的 token。
- 为 Email、Username 和 `(ThirdType,ThirdUserId)` 建立唯一性与 duplicate key 映射，确保并发注册不会产生第二个身份。

## Acceptance criteria

- [ ] `app/db` contract tests 在 MongoDB 7/8 通过，且无数据迁移。
- [ ] 全量 service 查询可通过兼容边界，静态扫描无直接 driver collection 使用。
- [ ] 领域值类型与 Mongo BSON 转换集中在 `app/db`，`app/info` 依赖扫描不出现 Mongo driver。
- [ ] 用户所有权条件、排序、分页、upsert 和批量更新有回归。
- [ ] timeout、连接失败、not-found 和 duplicate key 错误可定位且不吞错。
- [ ] 用户注册初始化事务/补偿、outbox 入队/重试、每个失败点和 `partial_write`/`side_effect` 结果有 fixture；邮件传输失败不回滚已入队用户。
- [ ] `sessions.UpdatedTime` TTL index、3 小时边界和有效解析刷新通过 Mongo fixture；`tokens` `(UserId,Type)` 唯一约束、原子替换/消费和旧文档兼容通过 fixture。
- [ ] 唯一索引创建前必须执行重复数据 preflight；发现历史重复时 readiness 失败并报告集合/索引/冲突计数，禁止自动删除或静默跳过索引。
- [ ] Email/Username/(ThirdType,ThirdUserId) 唯一索引及并发 duplicate 映射通过 Mongo 7/8 fixture。
- [ ] `go mod verify`、目标 Go 版本构建和 Golden/USN 通过。

## Out of scope

不修改 MongoDB Schema、不把 `context.Context` 贯穿 controller/service、不替换消息配置解析器。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
