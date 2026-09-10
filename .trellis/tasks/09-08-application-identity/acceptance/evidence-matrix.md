# `09-08-application-identity` 验收证据矩阵

| ID | 验收内容 | 证据/命令 | 责任 | 当前状态 |
| --- | --- | --- | --- | --- |
| AC-I1 | 登录、密码兼容、注册开关、重复账号、用户资料、邮箱和头像 service contract（含空/缺失 multipart、非法 ObjectId 与落库失败） | `app/service/*` focused tests；`go test ./app/service -count=1` | application-identity | 未实现 |
| AC-I2 | 用户+初始化事务/补偿、outbox 入队、每个子步骤失败、`partial_write` 语义；邮件传输失败可观察且可重试 | identity service fixture 消费 persistence AC-P2/P3 + mail seam fixture | application-identity + infrastructure-persistence | persistence AC-P2/P3 通过后实施 |
| AC-I3 | Web anonymous/source=none、authenticated、Cookie 损坏/过期、一次性重新登录、Session 字段投影和清理；Set/Delete/Encode/Commit 失败不得伪成功 | `app/httpserver/session.go` contract tests + interface HTTP replay | identity + interface-http | 当前只有静态代码证据 |
| AC-I4 | API token valid/none/invalid/expired、action-token type mismatch、storage error、logout 失效、显式 invalid 不 fallback 和 `_ID` fallback | identity contract fixture 消费 persistence AC-P4/P5；`app/tests/golden/api/auth/**` | identity + interface-http + infrastructure-persistence | persistence AC-P4/P5 通过后实施 |
| AC-I5 | API Auth/User envelope、字段顺序、Content-Type、错误文案和动态字段与 Golden 一致 | `LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/...`（需 Mongo/HTTP） | interface-http + delivery | 部分 Golden，未 live replay |
| AC-I6 | admin/member/demo 使用同一 principal；覆盖 demo 配置缺失/不一致、anonymous、member、admin；跨用户资料修改和匿名访问被拒绝 | role/demo fixture + real HTTP smoke | identity + application-admin/interface-http | 需求已按 U-04 决策，待实现与真实 HTTP 证据 |
| AC-I7 | 密码/token/secret 不出现在日志、错误响应、Golden 或任务 artifact；service 不依赖 Revel/controller；storage error 不伪装为 invalid credentials | source-path `rg` 扫描、`go vet`、依赖检查 | identity | 未运行 |
| AC-I8 | API route 方法/参数冲突、ApiUser first-party registration 和真实 binder 行为明确；未列方法 405，`userId` 必须与 principal 一致 | `conf/routes` + action method matrix + interface replay fixture | interface-http | 需求已按 U-05 决策，待下游 replay 证据 |
| AC-I9 | `sessions` idle TTL/TTL index、过期边界、Email/Username/(ThirdType,ThirdUserId) 唯一约束、旧 token 文档兼容 | persistence index/transaction fixture + Mongo 7/8 run | infrastructure-persistence + delivery | 下游 partial/unknown |
| AC-I10 | Mongo 7/8、邮件、浏览器、发布证据和跨任务汇合 | delivery run matrix | delivery-verification | 不属于本任务 ready 条件 |

## 证据规则

- HTTP 200 只表示 transport 成功，业务结果必须检查 `Ok`；无证据不能标记通过。
- Golden replay 只读；缺失或不匹配必须失败，不自动录制。
- 真实 Mongo/HTTP/browser/mail/release 未运行时记录 `partial` 或 `unknown`，不以 controller 直调或 mock 成功替代。
- 找回密码不存在/outbox 入队成功必须得到相同公开文案；存储/入队失败也不得通过错误文案枚举账号存在性；注册 outbox 入队成功与邮件传输失败必须分开验收；注册自动登录失败必须要求重新登录。
- Token 过期使用 `now >= expiry`；action token 类型 mismatch 不适用于 API `sessions` resolver；显式 invalid API token 绝不 fallback。
- 任何唯一索引、token schema、outbox、Cookie、公开字段、方法、权限或副作用语义变化，必须先更新领域目录、Golden/fixture、兼容说明和本矩阵。
- 本任务不得在 persistence AC-P2～P6 未通过前进入业务编码；identity 的 service/adapter 结果必须作为 interface-http 和 delivery 的前置 artifact。
