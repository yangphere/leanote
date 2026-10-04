# `09-08-application-identity` 验收证据矩阵

| ID | 验收内容 | 证据/命令 | 责任 | 当前状态 |
| --- | --- | --- | --- | --- |
| AC-I1 | 登录、密码兼容、注册开关、重复账号、用户资料、邮箱和头像 service contract（含空/缺失 multipart、非法 ObjectId 与落库失败） | `TestAuthServiceLoginSurfacesStorageLookupError`、`TestAuthServiceLoginMapsMissingUserToInvalidCredentials`、`TestAuthServiceThirdRegisterUsesCompositeThirdIdentity`、`TestAuthServiceThirdRegisterStoresThirdType`、`TestAuthServiceThirdRegisterReturnsZeroUserWhenRegisterFails`；`go test ./app/service -count=1`（2026-09-10 通过）；`go test ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver -count=1`（2026-09-10 通过） | application-identity | `partial`：注册、密码 token、邮箱 token、非法 ObjectId、purpose-specific 邮件超时、登录 storage/invalid_credentials 分流、第三方复合身份和相关 adapter 编译/测试已覆盖；登录 hash 矩阵、头像真实存储和完整 multipart/live HTTP 仍待下游证据 |
| AC-I2 | 用户+初始化事务/补偿、outbox 入队、每个子步骤失败、`partial_write` 语义；邮件传输失败可观察且可重试 | `TestAuthServiceRegisterBuildsTransactionalInitializationAndOutbox`、`TestAuthServiceRegisterIncludesConfiguredSharesAndCopiedNotes`、`TestAuthServiceRegisterCleansSharedResourcesWhenShareStepFails`、`TestAuthServiceRegisterCleansCopiedNotesWhenCopyStepFails`、`TestAuthServiceRegisterCompensatesActualCopiedNoteIDsWhenLaterStepFails`、`TestAuthServiceRegisterDoesNotReportSuccessWhenInitializationFails`、`TestAuthServiceRegisterFailsOnOutboxSideEffect`；`go test ./app/service ./app/db ... -count=1`（2026-09-10 通过） | application-identity + infrastructure-persistence | `partial`：identity 已消费 transaction/compensation/outbox contract，注册 plan 覆盖用户、默认 notebooks、共享 notebook/note、欢迎 note DB copy、博客/About、activation token/outbox；standalone compensation 已覆盖后续步骤失败时按实际 copied noteId 清理；真实邮件传输、文件/图片/附件复制、USN/recount 和真实 Mongo/mail runner 仍为 `partial`/`unknown` |
| AC-I3 | Web anonymous/source=none、`Session["_ID"]` 惰性持久化、认证成功轮换、旧映射失效、Cookie 损坏/过期、一次性重新登录、Session 字段投影和清理；Set/Delete/Encode/Commit 失败不得伪成功 | `TestCaptchaGetCreatesStableAnonymousSessionIDBeforePersistingCaptcha`、`TestCaptchaGetDoesNotWriteImageWhenCaptchaPersistenceFails`；`app/httpserver/session.go` contract tests + interface HTTP replay | identity + interface-http | `partial`：Captcha 已覆盖先持久化 `_ID`/验证码、失败不写 PNG；完整 `Context.SessionID`、rotation 和 writer commit error replay 尚未实现 |
| AC-I4 | API token valid/none/invalid/expired、摘要 resolver、旧 24 位 token 过渡、action-token type mismatch、storage error、logout 失效、显式 invalid 不 fallback 和 `_ID` fallback | `TestAPISessionTokenUsesRandomBase64URLAndDigestOnlyStorageForm`、`TestSessionTokenFiltersAllowOnlyStrictLegacyObjectIDs`、`TestSessionMongoResolveAndDeleteUseDigestThenLegacyFallback`、`TestApiAuthLoginRoundTrip`、`TestApiAuthLogoutClearsToken`、`TestApiAuthLogoutWithoutTokenIsIdempotent`；包测试 2026-09-10 通过 | identity + interface-http + infrastructure-persistence | `partial`：P-08 摘要存储、digest-first resolver/logout、旧 24 位 token fallback 和 logout idempotency 已有 focused evidence；Golden replay、显式 invalid 不 fallback 的完整受保护 API runner、`_ID` fallback live replay 仍 `partial` |
| AC-I5 | API Auth/User envelope、字段顺序、Content-Type、错误文案和动态字段与 Golden 一致 | `LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/...`（需 Mongo/HTTP） | interface-http + delivery | 部分 Golden，未 live replay |
| AC-I6 | admin/member/demo 使用同一 principal；覆盖 demo 配置缺失/不一致、anonymous、member、admin；跨用户资料修改和匿名访问被拒绝 | role/demo fixture + real HTTP smoke | identity + application-admin/interface-http | 需求已按 U-04 决策，待实现与真实 HTTP 证据 |
| AC-I7 | 密码/token/secret 不出现在日志、错误响应、Golden 或任务 artifact；service 不依赖 Revel/controller；storage error 不伪装为 invalid credentials | `TestAuthDoLoginMapsStorageErrorWithoutCountingCredentialFailure`、`TestPwdServiceFindPwdSurfacesDefaultLookupStorageError`；`go vet ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver`（2026-09-10 通过）；`rg -n 'github.com/revel|revel\.|controllers' app/service/AuthService.go app/service/PwdService.go app/service/SessionService.go app/service/TokenService.go app/service/UserService.go`（零命中）；`rg -n '\b(Log|Logf|LogJ)\(|fmt\.Print\w*\(' app/service app/controllers app/db \| rg -i 'pwd|password|token|secret|app\.secret'`（零命中） | identity | `partial`：聚焦静态扫描、错误分流 regression 和 vet 通过；Golden、真实错误响应和任务 artifact 的全量安全扫描仍待交付汇合 |
| AC-I8 | API route 方法/参数冲突、ApiUser first-party registration 和真实 binder 行为明确；未列方法 405，`userId` 必须与 principal 一致 | `conf/routes` + action method matrix + interface replay fixture | interface-http | 需求已按 U-05 决策，待下游 replay 证据 |
| AC-I9 | `sessions` idle TTL/TTL index、过期边界、Email/Username/(ThirdType,ThirdUserId) 唯一约束、旧 token 文档兼容 | persistence index/transaction fixture + Mongo 7/8 run | infrastructure-persistence + delivery | persistence focused contract 已通过；Mongo/并发/service 汇合仍 `partial`/`unknown`，不是 identity 规格通过 |
| AC-I10 | Mongo 7/8、邮件、浏览器、发布证据和跨任务汇合 | delivery run matrix | delivery-verification | 不属于本任务 ready 条件 |
| AC-I11 | 登录成功后的匿名 `_ID` 是否轮换、旧标识失效、Captcha 计数迁移和 API fallback 过渡 | P-01 已确认 + session/Captcha/fallback fixture | identity + interface-http | 已确认；fixture 未实现 |
| AC-I12 | CSRF 兼容基线和是否新增保护 | P-02 已确认 + 归档 C-b 设计、`rg -i csrf app conf`、interface replay 风险记录 | interface-http + delivery | 已确认保持无机制；风险/replay 待下游记录 |
| AC-I13 | API token 仅 query/form 还是增加 `Authorization` header；多来源冲突优先级和脱敏 | P-03 已确认 + binder/Golden/replay fixture | interface-http + delivery | 已确认 query/form；header 扩展不在本任务 |
| AC-I14 | 登录/找回密码/token 验证是否增加通用限流，及键、阈值、窗口和 envelope | P-04 已确认 + rate-limit/Captcha fixture | security + delivery | 已确认仅 `_ID` + Captcha；通用限流不在本任务 |
| AC-I15 | 密码/邮箱更新与 action token 消费的跨集合原子性、standalone compensation 和重试收据 | `TestPwdServiceUpdatePwdDoesNotConsumeWhenPasswordWriteFails`、`TestPwdServiceUpdatePwdDoesNotReportSuccessWhenConsumeFails`、`TestUserServiceUpdateEmailDoesNotConsumeWhenEmailWriteFails`、`TestUserServiceActiveEmailDoesNotReportSuccessWhenConsumeFails`；`go test ./app/service ./app/db ... -count=1` 通过 | identity + infrastructure-persistence | `partial`：service transaction seam 和 fail-closed focused tests 已覆盖；standalone durable receipt、真实 Mongo/mail 并发和下游公开映射仍待验证 |
| AC-I16 | Web/API 登出清理失败、缺失/无效 token 的 Cookie、`Ok`/Msg、可观测和重试语义 | `TestApiAuthLogoutClearsToken`、`TestApiAuthLogoutWithoutTokenIsIdempotent`；`go test ./app/controllers/api -count=1` 通过 | identity + interface-http | `partial`：API logout focused path 已覆盖；Web Cookie cleanup failure、真实 SessionWriter 失败和公开文案/redirect replay 待 interface-http |
| AC-I17 | 注册已提交但自动登录 Cookie 提交失败时 Web `Re`/redirect 的精确形状 | P-07 已确认 + registration/session-writer failure fixture | identity + interface-http | 已确认；numeric Code、文案和 redirect 需按契约落地 |
| AC-I18 | API session token 的随机强度、长度/编码、at-rest 形式和旧客户端兼容 | `TestAPISessionTokenUsesRandomBase64URLAndDigestOnlyStorageForm`、`TestSessionTokenFiltersAllowOnlyStrictLegacyObjectIDs`、`TestSessionMongoResolveAndDeleteUseDigestThenLegacyFallback`；`go test ./app/db -count=1` 通过 | identity + infrastructure-persistence + security | `partial`：P-08 persistence contract、fixture 和 service/controller 包测试通过；日志/backup 脱敏、Golden 和真实客户端兼容 replay 仍待 delivery |

## 证据规则

- HTTP 200 只表示 transport 成功，业务结果必须检查 `Ok`；无证据不能标记通过。
- Golden replay 只读；缺失或不匹配必须失败，不自动录制。
- 真实 Mongo/HTTP/browser/mail/release 未运行时记录 `partial` 或 `unknown`，不以 controller 直调或 mock 成功替代。
- AI 助手不得启动 Leanote 应用、开发服务器或其他应用进程，也不得调用 computer-use。需要运行中应用、浏览器、真实客户端或人工交互时，必须把待验证功能、前置条件、步骤、预期结果和建议证据整理为 Markdown checklist 交给用户手动验证。
- 人工验证项在用户回传结果前保持未勾选和 `partial`/`unknown`；回传后必须记录环境、日期、实际结果及可用证据，不能凭 checklist 已发出或用户未反馈推断通过。
- 找回密码不存在/outbox 入队成功必须得到相同公开文案；存储/入队失败也不得通过错误文案枚举账号存在性；注册 outbox 入队成功与邮件传输失败必须分开验收；注册自动登录失败必须要求重新登录。
- Token 过期使用 `now >= expiry`；action token 类型 mismatch 不适用于 API `sessions` resolver；显式 invalid API token 绝不 fallback。
- 任何唯一索引、token schema、outbox、Cookie、公开字段、方法、权限或副作用语义变化，必须先更新领域目录、Golden/fixture、兼容说明和本矩阵。
- persistence AC-P2～P6 的“contract/fixture 可消费”是 identity 实现前置；其 Mongo/邮件/受保护 runner/Golden/USN 等最终证据即使 persistence 已归档仍保持 `partial`/`unknown`，不得以“归档 completed”替代。
- P-01、P-05、P-06、P-07、P-08 已书面确认，但对应 fixture 和 P-08 persistence 摘要 contract 就绪前不得宣称实现完成；P-02～P-04 已确认保持当前兼容基线，必须把保持现状、已知风险和下游责任记录为验收证据。
- identity 的 service/adapter 结果必须作为 interface-http 和 delivery 的前置 artifact；任何 focused mock 或 controller 直调都不能升级为跨层通过。

## 2026-09-10 实现证据

- `go test ./app/service ./app/controllers -run "TestAuthServiceLogin|TestAuthServiceThirdRegister|TestPwdServiceFindPwdSurfacesDefaultLookupStorageError|TestUserServiceFindUser|TestCaptchaGet|TestAuthDoLoginMapsStorageError" -count=1`：通过。
- `go test ./app/service -run 'TestAuthServiceRegisterCompensatesActualCopiedNoteIDsWhenLaterStepFails' -count=1`：通过。
- `go test ./app/service -count=1`：通过。
- `go test ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver -count=1`：通过。
- `go vet ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver`：通过。
- `git diff --check`：退出码 0；仅 Windows 工作区 CRLF 替换提示。
- 聚焦静态扫描：身份核心 service 无 Revel/controller 依赖命中；日志/打印调用中无 `pwd/password/token/secret/app.secret` 命中。
- 新增修复证据：登录/找回密码用户查询 storage error 不再按空用户处理；Web/API 登录只将 `invalid_credentials` 映射为旧凭据错误；第三方注册按 `(ThirdType, ThirdUserId)` 查询并写入 `ThirdType`，注册失败返回零值用户；Captcha 在写 PNG 前检查 `SetCaptcha`。
- 未运行：真实 Mongo 7/8 矩阵、Golden replay、真实 HTTP/browser/mail/release、文件/图片/附件复制和跨层 protected-runner。

## 2026-09-11 复审修复证据

- 新增并完成红绿回归：`TestFileUploadImageRejectsMissingMultipartFile`；Legacy avatar/upload adapter 在索引文件项前检查缺失 multipart，空请求不再 panic。
- 新增并完成红绿回归：`TestAuthServiceDefaultNotebooksStepPreservesCleanupFailure`、`TestAuthServiceSharedResourcesStepPreservesCleanupFailure`、`TestAuthServiceCopyNotesStepPreservesCleanupFailure`；注册多写步骤保留原始写失败与 in-step cleanup 失败 cause，不再丢弃补偿错误。
- 新增并完成红绿回归：`TestLogOutboxDeliveryErrorDoesNotExposeTransportDetails`；生产 worker 的终止日志不再输出可能由 SMTP 服务端回显的邮箱或 token，原始投递失败仍保存在 durable outbox retry state。
- `go test -timeout 60s ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver ./cmd/leanote -count=1`：通过。
- `go vet ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver ./cmd/leanote`、`go build ./...`：通过。
- `09-08-application-identity`、`09-08-interface-http`、`09-08-delivery-verification` 的 `task.py validate`：通过；`git diff --check`：退出码 0，仅有 CRLF 替换提示。
- `go test -timeout 60s ./... -count=1`：未通过。真实测试库的 `sessions.SessionId` 存在 1 组历史重复，`sessions_SessionId_unique` 预检按 fail-closed 契约拒绝启动；并行 harness 随后达到 60 秒总超时。未自动删除或修复用户数据库记录。
- 真实 Mongo 7/8、真实 SMTP 投递、Golden/live HTTP、浏览器、发布和文件/图片/附件复制仍为 `partial`/`unknown`，不能由上述 focused evidence 替代。
