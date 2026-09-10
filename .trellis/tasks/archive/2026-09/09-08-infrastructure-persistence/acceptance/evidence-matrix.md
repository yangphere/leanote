# `09-08-infrastructure-persistence` 验收证据矩阵

| ID | 验收内容 | 证据/命令 | 当前状态 |
| --- | --- | --- | --- |
| AC-P1 | `app/db` 是唯一 Mongo driver 边界；查询、ObjectID/BSON、timeout、not-found、duplicate 不吞错 | `go test ./app/db/...`、静态依赖扫描、`go vet ./app/db/...` | partial：app/db wrapper、context seam、ObjectID/BSON、错误分类和测试已实现；service 直接 BSON 依赖迁移尚未闭合 |
| AC-P2 | 注册用户、默认资源、必需初始化和 outbox 入队处于同一 transaction；无 transaction 时按 `userId` 幂等补偿 | transaction fixture + Mongo 7/8 run | partial：MongoDB 7 standalone fallback、MongoDB 8 replica-set committed transaction、callback retry/partial fixture 已通过；真实 identity 注册接入尚未开始 |
| AC-P3 | outbox worker 传输失败可重试、可观测，禁止请求路径无结果 goroutine；`sending` lease 可 reclaim；transport timeout 后状态更新仍落 retry/dead；稳定 idempotency key 防重复入队；入队失败返回 `side_effect`/`partial_write` | outbox seam fixture + failure injection + Mongo lease/idempotency fixture | partial：Mongo/内存同步 claim/retry、lease/reclaim、独立状态更新、10 次 dead-letter 和 idempotency contract 已通过；真实邮件 transport/告警仍未运行 |
| AC-P4 | `sessions.UpdatedTime` 10800 秒 TTL index、`now >= UpdatedTime+3h` 过期边界和有效解析原子 refresh | Mongo index fixture + clock-controlled tests | partial：TTL index、clock boundary、atomic refresh 在 MongoDB 7 standalone fixture 已通过；identity resolver、受保护 runner 汇合尚未接入 |
| AC-P5 | action token 独立 `_id`、`(UserId,Type)` 唯一约束、同用途原子替换/消费、旧 `_id=UserId` 只读兼容；重签发先退役 legacy，禁止两个 active；decoder 对显式零值/非整数 fail-closed | token schema/index/concurrency/legacy-reissue fixture | partial：新 token issue/resolve/consume、严格 decoder、legacy reissue contract 已通过；当前环境无 Mongo，受保护 runner、并发和 service 接入未闭合 |
| AC-P6 | Email/Username/(ThirdType,ThirdUserId) 唯一索引和并发 duplicate 映射稳定，历史重复数据必须 fail-ready | Mongo 7/8 concurrent duplicate fixture + index preflight fixture | partial：MongoDB 7 standalone 的索引模型、Email duplicate/concurrency 与重复数据 preflight 已通过；并发注册 service fixture 未运行 |
| AC-P7 | Mongo 7/8、Golden/USN、所有权、`go mod verify` 和目标 Go 构建证据可复核 | protected runner artifact | partial：MongoDB 7 standalone 全量 app/db、MongoDB 8 replica-set focused transaction/index/outbox/token、vet、`go mod verify`、`go build ./...` 已运行；protected runner、Golden/USN 证据仍缺失 |

## 交接规则

- AC-P2～P6 通过前，`09-08-application-identity` 不进入业务编码。
- identity 只能消费本矩阵确认的 repository/transaction/outbox/token seam，不在 service 中私自建立第二套 Mongo 事实来源。
- transaction callback 的 driver 重试、code 251/263 非 capability 错误和重复索引 preflight 均属于当前实现/运维门禁；任何未运行的 Mongo、邮件、HTTP、浏览器和发布证据保持 `partial`/`unknown`。
- Mongo、邮件、HTTP、浏览器和发布证据缺失时保持 `partial`/`unknown`，不能用 mock 或 controller 直调标记通过。
