# 基础设施层：MongoDB 持久化边界 — 执行计划

- [x] 盘点 `app/db` 与 service 的 driver 类型、查询形态、配置键和错误处理；结果和未迁移清单记录在验收矩阵。
- [ ] 完成 client/collection/query/ObjectID/BSON 转换单一边界，并迁移剩余直接 driver 调用；当前 wrapper 已就位，但 service 仍有直接 Mongo/BSON 依赖（AC-P1 partial）。
- [x] 补齐查询、排序、分页、投影、批量更新、upsert、not-found、duplicate key 和 timeout 的 focused contract tests。
- [x] 实现 transaction seam；覆盖 callback 重试状态清理、standalone capability fallback、非 capability 错误不降级，以及 `partial_write`/`side_effect` 结果。
- [x] 实现 outbox enqueue/delivery seam：至少一次、稳定 idempotency key、sending lease/reclaim、独立状态更新 context、10 次上限 dead-letter 和同步可观测错误；真实邮件 transport/告警仍未运行。
- [x] 建立 `sessions.UpdatedTime` 10800 秒 TTL index、3 小时边界和原子 idle refresh；建立 token 独立文档、`(UserId,Type)` 唯一约束、同用途替换/消费、legacy 重签发退役和严格 decoder。
- [x] 建立 Email/Username/(ThirdType,ThirdUserId) 唯一索引和 duplicate 映射；并发注册 service fixture 尚未运行。
- [ ] 在当前受保护 runner 的 MongoDB 7/8 fixture 上运行 service、事务/index/outbox、Golden、USN 和所有权回归；历史本地 fixture 结果不能替代该证据。
- [x] 运行 `go mod verify`、目标 Go 构建和依赖扫描；MOD-001 的全链路 request-context 传播仍单独跟踪。

## 当前实现进度（2026-09-10，修复审查问题后）

- 已完成 `app/db` transaction-aware context seam：`InsertContext`、`UpdateContext`、`UpdateAllContext`、`UpsertContext`、`RemoveContext`、`RemoveAllContext` 与 `FindContext` 保留 Mongo session，同时继续使用有界 operation timeout。
- 已完成持久化索引模型与启动时显式创建：sessions `UpdatedTime` TTL=10800、tokens `(UserId,Type)`、users 的 Email/Username/(ThirdType,ThirdUserId) 唯一索引，以及 outbox 状态/幂等 key 索引；唯一索引 preflight/readiness 行为已写入规格，历史重复数据处理仍需受保护 runner 证据。
- 已完成 `RunUserInitialization` 的 transaction/standalone compensation 结果契约；callback 重试会清理 attempt 状态，只有 capability code 20 才允许 fallback，失败保留 `partial_write`，outbox 失败保留 `side_effect`，不在请求路径启动 goroutine。
- 已完成 session idle boundary/atomic refresh、action token 独立 ID/用途匹配/一次性消费/严格 decoder；legacy `_id=UserId` 重签发会先退役旧 token 再写独立文档，避免两个 active token。
- 已完成 Mongo/内存 outbox 的同步 claim/retry：稳定 idempotency key、sending lease/reclaim/lease owner fencing、transport 超时后的独立状态更新、解码失败保留 attempt 计数、10 次后 dead-letter 和 observable error。
- 当前可复核的本地检查：以显式 `LEANOTE_DB_TEST_URI` 运行 MongoDB 7.0 standalone 的完整 `go test ./app/db -count=1`，以及 MongoDB 8.0 replica-set 的 transaction/index/outbox/token focused fixture；`go vet ./app/db/...`、`go mod verify`、`go build ./...` 也已通过。没有显式 fixture URI 时 integration test 仍会标记 skipped；显式 URI 连接失败会 fail-closed。真实 identity/service 接入、protected runner、Golden/USN 尚未闭合。
