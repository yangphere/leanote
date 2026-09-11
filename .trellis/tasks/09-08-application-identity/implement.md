# 应用层：身份认证与会话 — 执行计划

本文件是规格审核后的实施顺序；P-01～P-08 已于 2026-09-10 确认采用推荐方案，以下步骤可进入实施准备。真实 Mongo/HTTP/browser/mail/release 证据仍按验收矩阵独立闭合。

## 阶段 0：实施前门禁（规格审核完成后才能编码）

- [x] 读取本任务 `prd.md`、`design.md`、`research/spec-audit-2026-09-09.md`、persistence 归档验收矩阵和领域任务交接材料。
- [x] U-01～U-05 已采用推荐决策：统一找回密码文案、注册事务+outbox、按用途并存 token、`demoUserId` 唯一事实来源、按 action 固化 API 方法和 principal `userId`。
- [x] 核对 `09-08-infrastructure-persistence` 已提供可消费的 transaction/compensation、outbox、TTL/index、token schema/旧文档兼容 contract、fixture 和错误分类。该任务虽已归档为 `completed`，AC-P2～P6 的真实跨层证据仍为 `partial`；它们是最终验收门禁，不得被写成身份实现前置的“全部通过”。
- [x] 在 persistence contract 中补齐 P-08 的摘要查询、旧 24 位 ObjectId token 过渡和 resolver/logout 共享查询；该依赖完成前不得实现 API token 生成/解析。
- [x] 将已批准决策和 PRD §6.1 P-01～P-08 的确认结果同步到 `interface-http`/`delivery-verification` 任务；实现前确认 route method matrix、SessionWriter、Golden/replay 和真实环境汇合责任；`_ID` 稳定标识和 `_token/_userId` 回写按已归档迁移设计执行。
- [x] 评审 PRD §6.2 的 P-01～P-08 推荐方案并记录逐项确认；用户已确认全部采用推荐方案。
- [x] P-01、P-05、P-06、P-07、P-08 的书面决策门已闭合；P-02～P-04 已确认保持兼容基线并记录风险和下游 owner。
- [x] 由 `task.py validate`、所有 `implement.jsonl/check.jsonl` 路径存在性检查和 `git diff --check` 确认任务材料完整。

## 阶段 1：服务契约与错误收敛

- [x] 在 `AuthService`、`UserService`、`PwdService`、`SessionService`、`TokenService` 内收敛身份、输入、所有权、错误分类和时钟/存储 seam；service 绑定 `Vd` 精确规则并安全解析 ObjectId。
- [x] 保留旧 MD5 与当前 hash 的密码验证；修复无条件成功、忽略 DB/副作用结果、无效 token 建 session、token 类型遗漏、并发可变过期时长和错误凭据吞 storage error。
- [x] 消费 persistence contract 实现注册事务/outbox/幂等补偿、token 独立文档与 `(UserId,Type)` 唯一约束，并为每个失败点增加 focused regression；不得在 service 内建立第二套 Mongo 事实来源。
- [x] 按 P-05 确认结果实现密码/邮箱更新与 token 消费的 transaction 或显式 compensation；验证失败时 token 保留、`partial_write` 和重试收据语义。
- [x] 按 P-08 确认 API token 的随机强度、编码、存储形式和兼容迁移；同步 resolver、logout、日志脱敏和 Golden/fixture，不能只替换 token 生成器。

## 阶段 2：Web/API adapter contract

- [ ] 将 Web session 投影、API token resolver、demo policy 和 anonymous/member/admin principal 统一接入显式接口；SessionWriter Set/Delete/Encode/Commit 错误不得静默；service 不再依赖 Revel controller/session。
- [ ] 按已确认的 P-01 实现 session 标识轮换，补齐旧 `_ID`/fallback 映射失效、白名单字段迁移、Captcha 计数清理和旧 Cookie 并发请求按 anonymous 处理的 fixture。
- [ ] Legacy Auth/User 和 first-party API Auth/User 只负责绑定、映射和响应；不复制业务校验或自行信任 `userId`。
- [ ] 保持既有 API envelope、字段顺序、Content-Type 和公开文案；按 action 固化允许方法，未列方法返回 405，显式 `userId` 必须与 principal 一致；所有变化先更新 Golden/fixture/兼容说明。
- [ ] 按 P-06/P-07 实现登出清理失败和注册后 Cookie 提交失败的明确内部结果与 adapter 映射；不得返回伪成功或伪造已登录状态。

## 阶段 3：测试和证据

- [ ] 运行 service/httpserver/adapter contract、Golden normalization 和敏感信息扫描；覆盖 anonymous/source=none、有效、过期边界、action-token 类型错误、数据库错误、权限失败、跨用户、demo 配置错误和 SessionWriter 写失败。
- [ ] 按已确认的 P-02～P-04 记录 CSRF（现状无机制）、API token 传输（当前 query/form）和通用限流（当前仅 Captcha）的风险；按 P-08 验证 32 字节 CSPRNG、base64url、摘要存储、旧 token 过渡及 resolver/logout 一致性。若未来扩大能力，先更新对应责任任务和验收范围。
- [x] 运行 `go test ./app/service ./app/httpserver ./app/controllers/api ./app/controllers -count=1`、`go vet ./app/service`，必要时运行受支持的 Mongo fixture 测试；真实 HTTP/browser/release 证据不得在本任务伪造。
- [x] 更新本任务 acceptance evidence matrix，记录命令、结果、fixture 路径和仍未闭合的下游证据。

## 2026-09-10 当前收口状态

- 已完成并验证：注册事务/outbox plan、共享 notebook/note、欢迎 note 的 Mongo `notes`/`note_contents` 复制、action token 用途/过期、密码/邮箱与 token 消费原子 seam、P-08 API token 摘要存储和旧 24 位 token 过渡、API login/logout focused path。
- 已修复 review 发现的注册复制笔记补偿缺口：`copy_notes` step 成功后如后续 `activation_token` 或 outbox 失败，standalone compensation 使用 `CopySharedNote` 实际返回的 copied noteId 清理；`Apply` 入口重置 step 闭包状态，避免 transaction retry 污染补偿列表。
- 已修复复审发现的身份错误分流缺口：`AuthService.Login` 和 `PwdService.FindPwd` 改用返回错误的 `UserService` 查询 seam，storage error 不再变成空用户、凭据错误或找回成功。
- 已修复复审发现的第三方注册缺口：`ThirdRegister` 按 `(ThirdType, ThirdUserId)` 查询既有身份，创建用户时写入 `ThirdType`，并检查注册持久化结果，失败返回零值用户。
- 已修复复审发现的 Captcha 写出顺序缺口：`Captcha.Get` 在写 PNG 前生成/持久化稳定匿名 `_ID` 对应验证码，`SetCaptcha` 失败返回 `storage` JSON。
- 新增/确认回归：`TestAuthServiceRegisterCompensatesActualCopiedNoteIDsWhenLaterStepFails` 覆盖后续步骤失败时按实际 generated noteId 补偿。
- 新增/确认回归：`TestAuthServiceLoginSurfacesStorageLookupError`、`TestAuthServiceLoginMapsMissingUserToInvalidCredentials`、`TestPwdServiceFindPwdSurfacesDefaultLookupStorageError`、`TestAuthServiceThirdRegisterUsesCompositeThirdIdentity`、`TestAuthServiceThirdRegisterStoresThirdType`、`TestAuthServiceThirdRegisterReturnsZeroUserWhenRegisterFails`、`TestCaptchaGetDoesNotWriteImageWhenCaptchaPersistenceFails`、`TestAuthDoLoginMapsStorageErrorWithoutCountingCredentialFailure` 覆盖复审三项高优先级问题。
- 未在本任务关闭：完整 Web SessionWriter commit 失败 replay、admin/member principal 汇合、Golden replay、真实 Mongo 7/8、真实 HTTP/browser/mail/release、文件/图片/附件字节复制和内容/USN/recount 真实一致性。

## 验证命令

```text
go test ./app/service ./app/controllers -run "TestAuthServiceLogin|TestAuthServiceThirdRegister|TestPwdServiceFindPwdSurfacesDefaultLookupStorageError|TestUserServiceFindUser|TestCaptchaGet|TestAuthDoLoginMapsStorageError" -count=1
python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-identity
git diff --check
go test ./app/service ./app/controllers/api ./app/controllers -count=1
go test ./app/httpserver -count=1
go test ./app/service ./app/db ./app/controllers ./app/controllers/api ./app/httpserver -count=1
go vet ./app/service
```

Mongo、真实 HTTP、浏览器、邮件和发布验证必须使用各自任务规定的 harness/环境；环境不可用时记录 `partial`/`unknown`，不启用随机配置或静默 fallback。

## 回滚点与危险边界

- Auth/User 服务、Session/Token 存储、controller adapter 三个边界分别可回滚；不得回滚领域模型契约或删除用户/session/token 数据。
- 任何改变 token 主键、token 索引、Cookie 格式、API envelope、公开方法、权限白名单或注册副作用的改动，先更新 Golden/fixture、兼容说明和下游依赖，再实现；U-03 的旧文档兼容必须由 persistence 任务提供。
- 不修改 `conf/routes`、生成资源、前端文件、生产配置或其他 application 叶的业务代码；路由可达性和真实 HTTP 迁移由 `interface-http` 承接。
