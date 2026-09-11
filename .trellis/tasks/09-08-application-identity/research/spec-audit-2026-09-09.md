# `09-08-application-identity` 规格审核记录（2026-09-09）

## 1. 选叶与审核范围

按父任务 `09-08-business-layer-architecture/design.md §4` 的应用轨道顺序 `identity → notes → content → publishing → admin`，选中的 ready 叶为 `09-08-application-identity`。现场依据：

- `.trellis/tasks/09-08-application-identity/task.json` 的 `meta.depends_on=["09-08-domain-contracts","09-08-infrastructure-persistence"]`；
- `.trellis/tasks/archive/2026-09/09-08-domain-contracts/task.json` 已归档且状态为 `completed`；
- `.trellis/tasks/archive/2026-09/09-08-infrastructure-persistence/task.json` 已归档且状态为 `completed`，但其 AC-P2～P6 仍在验收矩阵中标为 `partial`；
- 其余应用叶虽同样满足领域依赖，但按父任务顺序排在 identity 之后；
- 本轮未创建新任务；该 ready 叶已处于 `in_progress` 的激活规划闸门（`task.json` 记录 `branch=dev`、`base_branch=dev`），业务实现尚未开始。规格结论不以是否重复运行 `task.py start` 或任务归档状态作为跨层证据。

审核只读取和修改任务材料，未修改 `app/`、`cmd/`、`conf/`、`sh/`、生成资源或业务测试实现。

## 2. 证据清单

| 主题 | 主要证据 | 审核结论 |
| --- | --- | --- |
| 跨层不变量 | `CONTEXT.md`、ADR-0002、领域任务 validation/spec-audit | API envelope、BSON/collection 名、所有权和 C-b Cookie 代价必须保持；真实 HTTP/Mongo/browser 由下游验证 |
| 服务实现 | `app/service/AuthService.go`、`UserService.go`、`PwdService.go`、`SessionService.go`、`TokenService.go` | 已识别无条件成功、吞错、类型遗漏、全局可变时长、部分成功和 token/session 语义缺口 |
| Web adapter | `AuthController.go`、`UserController.go`、`BaseController.go`、`CaptchaController.go`、`FileController.go`、`controllers/init.go` | 已识别 session 投影、找回密码、demo、登录次数/验证码、头像更新和公开白名单范围 |
| API adapter | `api/ApiAuthController.go`、`ApiUserController.go`、`api/init.go`、`api/httpserver.go` | Web session 与 API token 是两套机制；first-party 目前只完整注册 ApiAuth/ApiTag，ApiUser 可达性仍属下游 |
| HTTP/session | `app/httpserver/session.go`、`middleware.go`、`conf/routes`、归档 `08-25-revel-migration/design.md §4` | Cookie 编码/属性归 interface；`Session["_ID"]` 稳定匿名标识和 `_token/_userId` 回写已有迁移契约；API 方法按 U-05 固化，剩余仅是 replay 证据 |
| 公开基线 | `app/tests/golden/api/**`、`app/tests/auth_test.go`、harness | 有登录/注册/登出/用户资料部分 Golden 和一个 Mongo 登录冒烟；没有完整服务/真实 browser 证据 |

## 3. 发现的问题与规格修订

| 编号 | 现场发现 | 规格处理 | 状态 |
| --- | --- | --- | --- |
| F-01 | 原 Scope 只列五个 service 和少量 controller，遗漏邮箱激活/修改、找回密码、BaseController、admin/member、Captcha、`FileController.UploadAvatar`/`UserService.UpdateAvatar`、first-party adapter | PRD §2 建立能力/责任矩阵，design §4 明确 adapter ownership 和下游边界 | 已闭合 |
| F-02 | Web 是签名 Cookie，API 是 `sessions` 集合 token→UserId；不能视作一个 Session 实现 | PRD §3.1/§3.2/§3.3、design §2 分离 principal、session writer、token resolver | 已闭合 |
| F-03 | `SessionService.GetUserId` 对未知 token 会建 session，并忽略查询/更新时间错误 | 规定无效/存储错误 fail-closed；实现阶段需显式错误结果和 focused test | 已闭合为实施约束 |
| F-04 | `TokenService.VerifyToken` 未按 `Token.Type` 查询，`overHours` 为全局可变变量 | 规定 expected type + 注入时钟，禁止共享可变时长；U-03 已决定 action token 按用途并存并使用独立文档/复合唯一约束 | 已闭合为规格和实施约束，待实现与索引证据 |
| F-05 | 注册只检查 `AddUser`，初始化子操作和邮件 goroutine 结果被忽略，可能部分成功但返回 true | PRD/design 规定用户+必需初始化+outbox 入队原子成功边界；事务不可用时按 userId 幂等补偿，传输失败异步重试 | 已按 U-02 决策，待实现与持久化证据 |
| F-06 | 找回密码泄露用户存在性，`DoFindPassword` 固定返回成功；改密失败仍删除 token | PRD §3.5、U-01 和 design §3.3 规定不存在/入队成功统一公开文案，存储/入队失败使用不枚举存在性的统一可重试失败，且失败不消费 token | 已按 U-01 决策，待实现与 Golden 证据 |
| F-07 | Web 以 `demoUserId` 判断，API 以用户名 `"demo"` 判断，规则冲突 | 统一 demo policy；`demoUserId` 是唯一事实来源，缺失或与配置不一致 fail-closed，禁止按用户名猜测 | 已按 U-04 决策，待实现与权限证据 |
| F-08 | `openRegister != ""` 导致 `"0"` 也开启注册 | PRD 固定为当前兼容基线并要求空/0/false fixture，不擅自变更 | 已闭合（兼容基线） |
| F-09 | service 主要依赖 controller 先做 `Vd`，非法 ID/空值/重复字段职责不清 | design 规定 adapter 做绑定、service 重验业务不变量和所有权；错误分类由 adapter 映射 | 已闭合 |
| F-10 | API 文档列 `userId`，controller 从认证 session 取用户；Golden/first-party smoke 还有 GET/POST 方法差异 | U-05 已固定 identity action 方法矩阵、未列方法 405，以及显式 `userId` 只能与 principal 一致；login/logout 保留已有 GET+POST 两种证据方法，其余按 PRD 表固定；interface replay 只补充兼容证据，不重新决定授权语义 | 需求已闭合；下游 replay 证据待运行 |
| F-11 | `apiAuthBefore` 无 token 回退 `Context.SessionID` 并写回 `_token/_userId`，但当前 SessionID 是 Cookie value，不等同 Revel `Session.ID()` | 归档 `08-25-revel-migration/design.md §4` 已确认 `SessionID=Session["_ID"]`、缺失时惰性随机并经 SetSession 持久化；本任务将其作为既定 contract，interface-http 负责修复回写顺序和实现 | 已闭合（下游实现） |
| F-12 | 当前 first-party HTTP 仅注册 ApiAuth/ApiTag，ApiUser server action 未完整迁移 | 明确本任务只定义 contract/服务行为，route registration 和真实 replay 由 interface-http 承接 | 下游 unknown |
| F-13 | 现有测试不足以证明 Mongo、HTTP、浏览器、邮件、发布证据 | acceptance 与 implement 明确 partial/unknown，不把 focused test 升级为 live smoke | 已闭合为证据边界 |
| F-14 | API/主站头像上传直接访问 `Files["file"][0]`，缺少文件可能 panic；`ApiUser.UpdateLogo` 可能在 `UpdateAvatar` 失败后仍返回 Logo 成功 | PRD/design 要求 adapter 先校验 multipart，service/adapter 检查落库结果并显式返回失败；文件清理归 interface/infrastructure | 已闭合为实施约束 |

## 4. 已补充的材料

1. `prd.md` 增加范围/责任矩阵、Web/API 路由和认证边界、输入输出/错误、注册副作用、密码兼容、demo/注册开关、已决策事项和分层验收标准。
2. `design.md` 增加 `AuthenticatedPrincipal`、Session reader/writer、Token resolver、服务数据流、adapter ownership、错误分类、安全和测试设计。
3. `implement.md` 增加决策门禁、按服务/Session-token/adapter/测试分阶段顺序、验证命令和回滚危险边界。
4. `implement.jsonl`、`check.jsonl` 扩充到后端规范、跨层思考指南、领域交接、ADR、本任务研究和验收矩阵；实际 service、controller、middleware、路由与 Golden 作为实现阶段按清单读取的代码证据。
5. 新增 `acceptance/evidence-matrix.md`，逐项记录验收责任和未闭合证据。
6. 纳入归档迁移设计已确认的 `_ID` 稳定标识、session Cookie 回写时序和 `ClearSession` 兼容行为，并补充头像 multipart 缺失/落库失败边界。

## 5. 已决策事项与下游证据责任

- 本轮已采用 U-01～U-05 推荐决策，并写入 PRD/design/implement/验收矩阵：找回密码统一文案；注册事务+outbox/补偿；按用途并存 token；`demoUserId` 唯一事实来源；按 action 固化方法、405 和 principal `userId`。这些不再是待确认需求。
- `interface-http` 负责已决策的 API 方法/参数/binder、完整 route registry、Cookie writer、真实 HTTP replay；`infrastructure-persistence` 负责 Mongo 事务/索引/超时、outbox 和旧 token 文档兼容；`delivery-verification` 负责真实 Mongo、浏览器、邮件、发布汇合证据。
- 当前未运行真实 Mongo、HTTP、浏览器、邮件或发布环境；这些是实现/验收证据状态，保持 `partial`/`unknown`，不应回写为需求未决。
- 当前编排已调整为 persistence → identity → interface → delivery；identity 的业务编码必须等待 persistence 任务完成可消费的 contract、fixture 和验收材料，不能等待或假设其尚未运行的真实跨层证据已经通过。其他 application 叶仍可按父任务依赖并行，但不得绕过 identity/persistence 直接推进 interface。

## 6. 审核门禁

审核阶段完成后可继续走已激活 ready 叶的 Trellis 生命周期；本轮已完成 U-01～U-05 的规格决策，但仍不得把激活等同于实现或真实环境证据通过。进入功能编码前必须重新读取本记录、PRD、设计和实施计划，并由下游 owner 确认其按已批准契约实现；任何偏离需先回到规格材料重新评审。

## 7. 继续审核补充（2026-09-10）

### 7.1 persistence 归档状态与 identity 编码门禁拆分

重新读取 `.trellis/tasks/archive/2026-09/09-08-infrastructure-persistence/` 后确认：

- `task.json.status=completed` 只表示该任务已归档，不表示 AC-P2～P6 的所有真实环境证据已通过。
- persistence 验收矩阵仍将 AC-P2～P6 标为 `partial`：Mongo 7 standalone、Mongo 8 replica-set 和 focused fixture 已覆盖部分 transaction/outbox/TTL/token/index contract，但真实 identity 注册接入、受保护 runner、邮件传输/告警、并发 service 接入和完整跨层 replay 未闭合。
- `app/db/initialization.go` 已提供 `RunUserInitialization`/`ExecuteUserInitialization` 的 transaction 或显式 compensation 结果；`app/db/session_persistence.go` 已提供 `ResolveSessionAndRefresh` 的 3 小时 inclusive 边界；`app/db/token_persistence.go` 已提供 expected type、独立 token identity、legacy reissue 和原子 consume seam。它们是 identity 可消费的 contract，不应被误写成 identity 业务已经接入。

因此本任务的前置条件改为“persistence contract、fixture、owner 和错误语义可消费”，最终 Mongo/HTTP/mail/Golden/USN 证据继续由对应任务保持 `partial`/`unknown`。这消除了“AC-P2～P6 通过前不得编码”与“归档任务已完成”的文字冲突；随后新增的 P-08 摘要 contract 仍是 API token 编码前置。

### 7.2 新增源码证据

| 主题 | 当前源码证据 | 规格含义 |
| --- | --- | --- |
| session fixation | `app/controllers/AuthController.go:62-78` 登录成功直接写入现有 Session；未见匿名 `_ID` 轮换；`.trellis/tasks/archive/2026-08/08-25-revel-migration/design.md:116-127` 只冻结 `_ID` 稳定性和 Cookie 兼容 | 不能擅自加入轮换；P-01 需明确轮换、旧标识失效、Captcha 迁移和 API fallback |
| CSRF | `.trellis/tasks/archive/2026-08/08-25-revel-migration/prd.md:38` 明确项目当前无 CSRF 且迁移不增删 | identity 不拥有 CSRF；interface/delivery 必须记录兼容基线和风险，新增保护需另立范围 |
| API token transport | `app/controllers/api/init.go:65-76` 及 `app/controllers/api/API列表-v0.1.md` 使用 `token` 参数；未发现 `Authorization` header contract | query/form 是当前兼容基线；P-03 决定是否扩展来源及冲突优先级 |
| rate limiting | `app/controllers/AuthController.go:26-78` 只有按 session 标识的失败次数/Captcha；未发现通用限流器 | 不在 service 内隐式增加第二套策略；P-04 明确是否另建限流及其责任 |
| member 语义 | `app/controllers/member/init.go:75-99` 只检查 `Session["Username"]`；`app/info/UserInfo.go:46-58` 将 `AccountType` 定义为 normal/premium 配额字段 | `member` 角色固定为已认证路由角色，不能等同订阅等级 |
| password/token atomicity | `app/service/PwdService.go:39-62` 先更新用户再无条件 `DeleteToken`；`app/service/UserService.go:370-416` 邮箱更新和 token 验证/消费分离；persistence 当前只提供独立 token consume | P-05 必须冻结 transaction/compensation、失败后 token 保留和重试收据 |
| logout result | `app/controllers/AuthController.go:80-87` 与 `app/controllers/api/ApiAuthController.go:40-48` 忽略清理结果后返回 redirect/`Ok:true` | P-06 必须冻结清理失败的公开 envelope、Cookie 处理和重试语义 |
| register auto-login | `app/controllers/AuthController.go:129-139` 调用 `c.doLogin` 但忽略其结果；`app/controllers/api/ApiAuthController.go:51-74` 注册不自动登录 | P-07 必须冻结“已注册但未登录”的 Web `Re`/redirect，禁止伪造登录成功 |
| Cookie commit failure | `app/httpserver/registry.go:330-358` 已将 Cookie 写出调到 result 之前，但 `Encode` 错误仍被忽略；`Context.SessionID` 当前取 Cookie value，尚未实现 `_ID` 惰性持久化 | `SessionWriter` 必须暴露 commit 错误；P-01/P-06/P-07 的 adapter fixture 需覆盖错误路径 |
| API token strength/storage | `app/controllers/api/ApiAuthController.go:24-31` 使用 `db.NewObjectID().Hex()` 作为 token，并由 `SessionService` 原值写入 `sessions.SessionId`；未见长度、熵或摘要存储约束 | P-08 必须冻结生成、编码、at-rest 形式、日志/备份暴露面和旧客户端兼容；不能只替换生成器 |
| email template TTL | `app/service/EmailService.go:198`、`:225` 使用 `TokenActiveEmail` 的时长渲染 update-email/find-password 模板 | identity 实现需按 token purpose 修正，验收要比较实际模板时长，避免第二事实来源 |

### 7.3 审核时记录的待确认事项（历史状态）

当时的审核补充将以下事项显式登记到 PRD §6.1，并区分阻塞级别；用户确认后其状态见 §7.6：

- P-01 session fixation 轮换：阻塞登录 session 设计；未确认前只固定“不轮换”的兼容基线。
- P-02 CSRF：当前无机制；保持现状不阻塞 identity，但新增保护会改变 interface/presentation 范围。
- P-03 API token header：当前仅 query/form；任何扩展都需更新 binder、日志脱敏、代理/Referer 约束和 Golden。
- P-04 通用限流：当前仅 Captcha；若产品要求限流，必须先定义窗口、键、阈值、错误 envelope 和存储责任。
- P-05 密码/邮箱与 token 消费原子性：阻塞 action-token 实现，影响 transaction seam、补偿和重试。
- P-06 登出清理失败及缺失/无效 token 的幂等性：阻塞 logout 实现，影响 Web Cookie 和 API `Ok`/Msg。
- P-07 注册后 Cookie 提交失败：阻塞 Web auto-login 收口，影响 public envelope、redirect 和客户端下一步。
- P-08 API token 强度/存储：阻塞 API 登录 token 实现，影响 sessions schema、resolver/logout、日志和兼容迁移。

### 7.4 本轮审核结论

- 规格已补齐目标、范围、输入输出、业务规则、边界、异常、数据约束、兼容性、上下游责任和证据边界；U-01～U-05 仍为已批准决策。
- 当时仍有 P-01/P-05/P-06/P-07/P-08 五项关键需求不能由现有上下文可靠确定，已列为编码前阻塞确认；P-02～P-04 已记录兼容基线及扩大范围的影响。该状态已由 2026-09-10 的用户确认闭合，详见 §7.6。
- 本轮仅修改任务规格、研究和验收材料；未修改 `app/`、测试实现、CI、生产配置或生成资源。规格审核完成后停止，等待后续明确的功能编码授权。

### 7.5 已确认建议原文（2026-09-10）

用户要求先给出建议，再确认待定需求。结合现有代码和兼容边界，本轮先给出以下建议；用户已于 2026-09-10 确认全部采用，当前已作为 PRD §6.1～§6.2 的已批准实施基线：

| 编号 | 建议 | 理由与影响 |
| --- | --- | --- |
| P-01 | 登录成功轮换 `Session["_ID"]`，立即失效旧标识及 API fallback 映射；只迁移白名单非认证字段并清零失败/Captcha 计数，旧 Cookie 并发请求按 anonymous 处理。 | 直接消除 session fixation；依赖无 token fallback 的旧请求需重新登录，必须补充轮换/迁移/并发 fixture。 |
| P-02 | identity 保持无 CSRF 基线，风险交给 security/delivery 登记，新增保护另立任务。 | 避免改变现有表单/Ajax/Cookie 契约；不能把“无机制”误报成安全通过。 |
| P-03 | 继续只接受 query/form `token`；未来采用 Bearer header 时，query/header 不同值直接拒绝，相同值才合并。 | 不隐式扩大 token 泄露面或客户端兼容范围；header 扩展需独立更新 binder、脱敏和 Golden。 |
| P-04 | 保留稳定 `_ID`+Captcha，不在身份 service 中新增通用限流；IP/账号/找回密码限流由独立 security/delivery 任务定义。 | 避免第二套未定义的风控事实来源；新增限流前必须冻结键、阈值、窗口、存储和 envelope。 |
| P-05 | 只有 users 更新与 token 消费同一原子边界提交才算成功；standalone 仅在有持久化幂等收据时 compensation，否则拒绝并保留 token，返回 `partial_write`/失败。 | 防止半成功导致 token 重放或用户状态丢失；验收要覆盖收据、重试和失败注入。 |
| P-06 | missing/invalid token 作为不恢复身份的幂等登出；valid token 删除成功才 `Ok:true`；清理失败显式失败、可观测并保留重试状态。 | 兼顾重复登出与不伪造成功；需下游冻结 Web Cookie 保留和 API `Ok/Msg` 映射。 |
| P-07 | Web 自动登录 Cookie 失败时保留已注册账号，返回 `Re{Ok:false}` 和稳定 `registered_relogin_required` Code/Msg，不设认证 Cookie、不跳受保护页；API 注册不自动登录。 | 明确“已注册、未登录”，避免重复注册和伪造登录；numeric Code、文案和 redirect 仍需 interface/presentation 确认。 |
| P-08 | 新 token 由 `crypto/rand` 生成 32 字节并以无填充 base64url 返回；`sessions.SessionId` 只存 SHA-256 摘要，resolver/logout 统一摘要查询，同时兼容旧 24 位 ObjectId token 直到 3 小时 TTL 到期。 | 提升熵并降低 at-rest 泄露影响，保留旧客户端过渡；必须同步 schema/index、旧 token fixture 和日志脱敏。 |

当时在用户逐项确认前，P-01/P-05/P-06/P-07/P-08 仍为编码阻塞；P-02/P-03/P-04 即使采用建议，也要把“保持现状”和下游责任写入验收记录。用户已于 2026-09-10 确认全部采用建议，以下历史阻塞已转为实施/证据门禁。

### 7.6 用户确认与当前门禁（2026-09-10）

用户明确确认“全部采用推荐”，因此 P-01～P-08 的书面需求决策已闭合，具体取值以 PRD §6.1～§6.2 和 design §5.2 为准：

- P-01～P-08 不再是待确认需求；实现必须按已确认方案，不得回退到旧的“不轮换、明文 token、清理失败伪成功”等行为。
- P-02～P-04 已确认保持当前无 CSRF、query/form token 和 `_ID`+Captcha 基线；安全风险和未来扩展仍由 security/interface/delivery 任务记录。
- P-08 是跨任务前置：`infrastructure-persistence` 必须先提供 SHA-256 摘要查询、旧 24 位 ObjectId token 过渡和 resolver/logout 共享 contract；该 contract 未就绪前不得实现 API token 生成/解析。
- P-01、P-05、P-06、P-07 的 fixture、SessionWriter 失败映射、compensation receipt 和下游 numeric Code/文案/redirect 仍是实施证据，不得以书面确认替代测试或真实 replay。
- 本次确认只改变需求决策状态，不改变真实 Mongo/HTTP/browser/mail/release 证据状态；这些仍按验收矩阵保持 `partial`/`unknown`，也不改变本轮仅修改规格材料的边界。

## 8. 复审问题修复记录（2026-09-10）

复审发现的三项高优先级实现缺口已按本任务 PRD/design 的错误与身份契约修复：

- 认证查询错误分流：`AuthService.Login` 改用 `UserService.FindUserInfoByName`，`PwdService.lookup` 改用 `UserService.FindUserIDByEmail`；这些 seam 保留 storage error，只有缺失用户/密码错误映射为 `invalid_credentials` 或找回密码的存在性中立成功。Web/API 登录 adapter 只将 `invalid_credentials` 转为旧凭据错误文案，storage error 返回 `storage`，且不递增登录失败次数。
- 第三方身份：`ThirdRegister` 按 `(ThirdType, ThirdUserId)` 查询既有身份，创建时写入 `ThirdType`，并检查 `register` 结果；注册失败返回零值用户，避免返回未持久化身份。
- Captcha：`Captcha.Get` 在输出 PNG 前先惰性生成/持久化稳定匿名 `_ID`，并检查 `SetCaptcha`；失败返回 `storage` JSON，不写图片 body。

新增 focused regressions 已覆盖上述行为；完整包测试、`go vet`、`task.py validate` 和 `git diff --check` 通过。真实 Mongo 7/8、Golden replay、真实 HTTP/browser/mail/release 仍按验收矩阵保持 `partial`/`unknown`。
