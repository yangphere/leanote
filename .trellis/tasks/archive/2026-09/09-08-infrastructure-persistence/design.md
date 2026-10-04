# 基础设施层：MongoDB 持久化边界 — 技术设计

## Boundaries

`app/db` 是唯一 MongoDB driver 边界，内部拆分 client、collection、query、ObjectID/BSON 转换和错误映射；service 只依赖仓库语义，不暴露 driver 类型。领域层的中立 ObjectID/时间值在此处转换，禁止领域包反向依赖 driver。

## Data flow

应用 service → typed repository/query wrapper → MongoDB 7/8；wrapper 在每次操作内创建有界 timeout 并立即 cancel，返回领域可识别结果。注册初始化和 outbox 通过同一 transaction seam 提交；不支持 transaction 时转入按 `userId` 幂等补偿并返回可判定的 partial result。driver 重试 transaction callback 时清空本次 attempt 的 `AppliedSteps/FailedStep`，只有最终提交才设置 `Committed`；仅 standalone capability 错误允许 compensation，瞬时/提交状态错误不得重放。

## Invariants

collection/字段/BSON 不变；所有权条件由查询构造器强制；排序/分页/更新/upsert/not-found/duplicate key 与旧行为一致。`sessions.UpdatedTime` 是 idle TTL 时间源，TTL index 为 10800 秒，应用边界使用 `now >= UpdatedTime+3h`；有效解析必须原子刷新。`tokens` 使用独立 `_id` 和 `(UserId,Type)` 唯一约束，同用途签发原子替换旧 active token，消费使用 token+type+未消费条件原子失效。旧 `_id=UserId` 文档默认只读；重签发时先设置其 `ConsumedAt` 使其退役，再写入独立 `_id` 新文档，绝不保留两个 active。注册必需初始化与 outbox 入队同一成功边界，补偿按 `userId` 幂等且失败可观测；Email/Username/(ThirdType,ThirdUserId) duplicate 必须映射为稳定领域错误。

Outbox 使用 `pending/retry/sending/sent/dead` 状态机。claim 时写入 `LeaseUntil=now+2m`、新 `LeaseId` 并递增 attempts；只允许 lease 到期的 `sending` 被 reclaim，写回同时匹配 `LeaseId` 防止旧 worker 覆盖新 owner。transport 使用自己的 bounded context，状态更新使用新的 bounded context，因此 transport 超时仍会落 retry/dead。最多 10 次后进入 dead。投递为至少一次，`IdempotencyKey` 建立唯一部分索引并由 key 派生稳定 event id；未知写入结果通过 key 读回既有事件。Payload 只使用普通 `map[string]any`。

唯一索引创建前执行只读 preflight；发现重复数据返回包含集合、索引和冲突计数的 readiness 错误，禁止自动删除或跳过索引。

## Rollback

以 driver contract、MongoDB 7/8 Golden/USN 和 `go mod verify` 为边界回滚；MOD-001 单独规划。
