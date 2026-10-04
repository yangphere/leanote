# 应用层：身份认证与会话 — 技术设计

## 1. 架构边界

```text
HTTP/legacy controller binder
        │  (输入、Cookie、token 参数、响应 envelope)
        ▼
Identity application contract
  AuthService / UserService / PwdService
  SessionService / TokenService / principal policy
        │  (显式 seam，不读取 controller/session 全局)
        ▼
Domain models + persistence/mail/file interfaces
```

应用层拥有身份和授权决策；`app/controllers`、`app/controllers/api` 和 `app/httpserver` 只把 HTTP/模板/Session 表达转换成应用请求。`interface-http` 负责 route registry、binder、Cookie 编码、响应状态和真实 replay；`infrastructure-persistence` 负责 Mongo driver、超时、索引、事务/补偿实现。

## 2. 核心契约

### 2.1 AuthenticatedPrincipal

统一的认证结果至少包含：

```text
Principal {
  state: anonymous | authenticated
  user_id: canonical ObjectId string | ""
  source: none | web_session | api_token
  token_state: absent | valid | invalid | expired | type_mismatch | storage_error
  is_demo: bool
  role: anonymous | member | admin
}
```

匿名 principal 固定为 `state=anonymous,user_id="",source=none,token_state=absent,role=anonymous`。有效 Web session 使用 `source=web_session,token_state=absent`，有效显式 API token 使用 `source=api_token,token_state=valid`，API 无 token fallback 使用 `source=web_session,token_state=absent`。任何 `storage_error`、非法 ObjectId、空 UserId 或用户不存在都不能生成 authenticated principal。角色和 demo 派生只从配置绑定的 UserId/管理员配置及已加载用户事实计算，不从请求字段或字面用户名猜测。`member` 只表示已认证用户可访问会员中心路由；订阅/配额仍由 `User.AccountType` 等独立业务规则判断，不能复用认证角色。

### 2.2 Session reader/writer

- 应用只依赖以下可报告错误的接口，不直接使用 `*revel.Controller`、`revel.Session` 或 HTTP response：`SessionReader.Get(key) (string, bool, error)`、`SessionWriter.Set(key,value string) error`、`SessionWriter.Delete(key string) error`、`SessionWriter.Commit() error` 和明确的 `SessionID` 值；`Commit` 包含 Cookie 编码/写出，编码失败必须作为 `Commit` 错误返回。
- Web Session writer 投影 `BaseController.SetSession` 当前字段，清理身份时删除 `UserId/Email/Username/UsernameRaw/Logo/Verified` 等身份字段；归档迁移设计要求 `ClearSession` 对历史小写 `theme` 键的怪癖原样保留，不能在本任务顺手改成另一种公开 Cookie 形状。
- Cookie codec 由 interface 层注入；ADR-0002 规定 HMAC、`HttpOnly`、`SameSite=Lax`、可配置 `Secure`，无法解码旧 Revel Cookie 时返回 anonymous 并允许重新登录。
- 匿名 session 的稳定标识是 `Session["_ID"]`：缺失时惰性生成 crypto 随机 32 字符 hex，并经 `SetSession` 落入 Cookie。API fallback 使用该标识；不能把每次刷新后变化的 Cookie value 误当成旧 Revel `Session.ID()`。
- 匿名期间 `_ID` 保持稳定；登录成功时轮换为新的 session 标识并立即失效旧匿名标识及其 API fallback 映射。只迁移白名单非认证字段并清零失败/Captcha 计数，旧 Cookie 的并发请求按 anonymous 处理；不得把 Cookie value 作为新的匿名事实来源。
- 每个 `SetSession/DeleteSession` 都必须在 `ApplyResult`/响应体提交之前刷新 Cookie；任何 Set/Delete/Encode/Commit 错误都必须中止成功响应并映射为既有 storage/失败 envelope。登录、注册后自动登录、Captcha 和 API `_token/_userId` 回写都要有下一请求可见的回归。注册已落库但自动登录 Cookie 提交失败必须使用单独的 `registered_relogin_required` 内部结果，不能伪造已登录；其 Web 公共 envelope 见 PRD §6.1 P-07。

### 2.3 API session resolver

API 登录 token 存在 `sessions` 集合的 `SessionId` 字段，解析只负责 token→UserId、`UpdatedTime`/idle TTL 和用户存在性；兼容基线为 3 小时，应用按 `now >= UpdatedTime+3h` 判定过期，persistence 在 `UpdatedTime` 上设置 10800 秒 TTL index 并在有效解析时原子刷新它。它不读取 `tokens.Token.Type`。显式 token invalid/expired/storage_error 绝不 fallback。无 token 时仅以 `Session["_ID"]` 查已有有效映射，成功后把 `_token`/`_userId` 投影回 session；无映射不得创建 session。已发布客户端通过 query/form 的 `token` 参数，本任务不增加 header 来源；未来 Bearer 扩展必须遵守 query/header 冲突拒绝规则。新 token 按 P-08 使用 32 字节 CSPRNG + 无填充 base64url，persistence 在 `SessionId` 中保存 SHA-256 摘要并在过渡期兼容旧 24 位 ObjectId token；resolver/logout 必须共享同一摘要查询。Cookie 写出顺序和失败映射由 interface-http 保证。

### 2.4 Action token resolver

`TokenService` 通过显式 `Resolve(token, expectedType, now)` seam 查询 `tokens` 集合的 token 值、用途类型、过期时间和 UserId，并返回分类错误。过期比较使用注入时钟和 `info` 常量，禁止共享可变 `overHours`。邮箱/密码 token 的一次性消费必须在持久化边界提供原子失效或补偿。密码/邮箱更新与 token 消费跨 `users`/`tokens` 集合时，优先使用同一事务；standalone 只有在存在持久化、幂等、可重试收据时才允许 compensation，否则拒绝敏感操作并保留 token，返回 `partial_write`/失败。更新失败保留 token，消费失败不得返回成功。

action token 允许不同用途并存，每个 `(UserId,Type)` 最多一个 active token；同一用户/用途再次签发时原子替换旧 active token，不得短暂产生两个可用值。新 token 文档使用独立标识并建立 `(UserId,Type)` 唯一约束；旧 `_id=UserId` 文档只在未重签发时只读兼容，重签发必须先标记 legacy `ConsumedAt` 再写新文档。`Resolve(token, expectedType, now)` 必须同时匹配 token 值和用途类型，消费使用 token+type+未消费条件的原子失效。

## 3. 服务职责与数据流

### 3.1 登录

1. adapter 绑定并做字段存在性/基本格式检查，传入 `AuthService.Login`。
2. service 规范化 email/username，读取用户并用 `ComparePwd` 验证旧 MD5 或当前 hash。
3. 失败返回 `invalid_credentials`，不得区分“用户不存在”和“密码错误”；数据库错误向上返回，不变成空用户。
4. Web adapter 调用 session writer；API adapter 通过 `SessionService.SetUserId` 写 token 映射并返回 `AuthOk`。

### 3.2 注册

1. service 再次规范化/校验邮箱和密码，按 `app/lea/Vd.go` 的精确规则检查，生成合法用户名并检查 Email/Username 唯一性。
2. 用户、默认 notebook、共享资源、欢迎 note、博客和 About 单页初始化处于同一 Mongo 事务；事务不可用时按 `userId` 执行幂等补偿/重试。
3. outbox 入队与用户初始化处于同一持久化成功边界；初始化或 outbox 写入失败返回 `partial_write`/`side_effect`，禁止无条件 `true`。
4. 邮件传输由 outbox worker 负责重试和告警；请求路径不得启动无结果 goroutine。自动登录 session writer 失败时注册结果仍明确为“已注册、需重新登录”，但具体 Web `Re`/redirect 形状必须按 P-07 冻结。

注册初始化在 `AuthService.registrationPlan` 中表达为可测试 step：`user`、`default_notebooks`、可选 `shared_resources`、可选 `copy_notes`、`user_blog`、`about_single`、`activation_token` 和 outbox 入队。`shared_resources` 只消费既有 `registerSharedUserId`、`registerSharedNotebooks`、`registerSharedNotes` 配置，写入 `has_share_notes`、`share_notebooks`、`share_notes`；配置中的 ObjectID 或权限非法时 fail-closed 为 `configuration`，不得 panic。`copy_notes` 为每个配置 note 预先分配目标 noteId，复制 Mongo `notes`/`note_contents` 到新用户 life notebook 并强制非博客；文件、图片、附件字节复制、USN/recount 与内容领域的完整同步不是本 identity 任务的关闭证据，必须在 notes/content/delivery 侧保留 `partial`/`unknown` 直到真实环境验证。

### 3.3 找回密码、邮箱 token

- `FindPwd` 先按规范化邮箱生成带用途 token/outbox；用户不存在或 outbox 入队成功时对外统一返回“若账号存在，将发送邮件”，内部区分 `not_found`、`storage`、`mail`。存储/入队失败映射为不含存在性信息的统一可重试失败，邮件传输失败由 outbox 重试；日志不带邮箱明文或 token。
- `UpdatePwd(token,pwd)` 先验证 `TokenPwd`、用户和新密码 hash，再在 transaction seam 中更新密码并消费 token；更新失败不得删除 token。若部署只能 compensation，必须有幂等步骤和 `partial_write` 收据，避免“密码已更新但 token 永久可重放”或“token 已消费但密码未更新”。
- `ActiveEmail`/`UpdateEmail` 验证正确用途、过期时间、目标 UserId、邮箱唯一性后更新；失败保留 token 以便重试，成功才消费。跨集合严格原子性与重试语义以 P-05 的确认结果为准。

### 3.4 用户资料、demo 和角色

`UserService` 只接受认证 UserId 作为资料更新主体，内部用安全解析后的 `_id` 条件执行更新。邮箱、用户名、第三方身份由唯一索引和 duplicate 映射共同保证；用户名校验绑定 `Vd` 精确规则，`admin` 大小写不敏感。multipart 缺失/空文件在 adapter 早返回，不能索引不存在的文件项。demo 判定封装为单一 policy：`demoUserId` 缺失或与用户名配置不一致时 fail-closed，Web/API/admin/member 适配器不得各自比较字面用户名。兼容旧 API 时，显式 `userId` 只能作为与 principal 的一致性断言；不一致不得改变资料更新主体，并按 `forbidden`/`not_authenticated` 拒绝。

### 3.5 登出与注册后的 session 失败

- Web 登出必须先得到 session 清理结果，再决定响应；API 登出只处理请求中已验证的 token。缺失/无效 token 按不恢复身份的幂等登出处理；有效 token 只有删除映射成功才报告成功。任一持久化清理失败都要保留 wrapped cause、记录可观测事件并阻止 `Ok:true` 伪成功，保留可重试的 Cookie/token 状态；Web/API 的公开字段和文案沿用既有 `Re/ApiRe` 形状。
- 注册用户、默认资源和 outbox 已提交而自动登录 Cookie 的 Set/Delete/Encode/Commit 失败时，业务结果是“账号已注册、当前未登录”；不得回滚已提交注册，也不得把 response 写成已登录。Web 返回 `Re{Ok:false}` 和稳定 `registered_relogin_required` Code/Msg，不设置认证 Cookie、不跳受保护页面；numeric Code、i18n 文案和 redirect 由 interface/presentation 按此契约冻结，API 注册不自动登录。
- Session fixation 按已确认的 P-01 处理：匿名期间 `_ID` 稳定，认证成功时轮换并失效旧标识及 fallback 映射；只迁移白名单非认证字段并清零 Captcha 计数，旧 Cookie 并发请求按 anonymous 处理。

## 4. 适配器矩阵

| 适配器 | 应用调用 | 负责内容 | 不得负责 |
| --- | --- | --- | --- |
| Legacy Web Auth/User | `AuthService`、`PwdService`、`UserService` | 参数绑定、Vd、i18n、redirect/template、session writer | 自行查用户、demo/权限规则、吞掉 service 错误 |
| Legacy API Auth/User | 同上 + `SessionService` | token 参数、`ApiRe/AuthOk`、multipart 解析 | 任意 userId 信任、token 类型/过期判定副本 |
| first-party net/http API | 同上 | `httpserver.Context` 转换、注册表和真实 HTTP 状态 | 改动业务 envelope 或把未注册 action 当作已迁移 |
| main/admin/member before hook | principal resolver | anonymous/role/demo 结果到 redirect/`NOTLOGIN` | 复制用户查询、按用户名判断角色或绕过 demo policy |

当前 `app/controllers/api/httpserver.go` 只注册 ApiAuth 和 ApiTag；ApiUser first-party 注册/路由可达性属于 `interface-http` 下游工作。本任务提供 contract fixture 和 service 行为，不把缺失注册宣称为完成。下游 replay 只验证已批准的 action 方法、principal `userId` 和 405 行为，不再把这些授权规则列为待决策。

## 5. 错误和安全设计

内部错误分类固定为：`invalid_credentials`、`not_authenticated`、`forbidden`、`validation`、`duplicate`、`token_invalid`、`token_expired`、`type_mismatch`、`storage`、`side_effect`、`partial_write`、`session_commit`、`logout_cleanup_failed`、`registered_relogin_required`、`configuration`。adapter 将其映射到既有 `Re/ApiRe`、`NOTLOGIN`、`cannotUpdateDemo`、localized message；不新增公开字段或泄露底层错误/凭据。API 使用 HTTP 200 envelope 的 action 继续保持该兼容形状，first-party 新 action 的 transport status 由 interface-http matrix 固定。

数据库错误只能在终止边界记录一次并保留 wrapped cause；服务不得 log-and-return、返回空数据或忽略子操作结果。日志、Golden、错误响应和任务材料均不得出现明文 password、token 或 app.secret。现有 Web 无 CSRF 机制，identity 不新增/删除该行为；现有登录失败只通过稳定 `_ID` 的次数/Captcha 门控，通用限流若需增加必须由单独决策定义。

## 5.1 规格决策门

用户已于 2026-09-10 确认 PRD §6.1 的 P-01～P-08 全部采用 §6.2 方案。功能编码可以进入准备阶段，但必须先完成本文件、研究记录、persistence contract handoff 和验收矩阵的同步；真实 Mongo/HTTP/browser/mail/release 证据仍不得提前标记通过。

### 5.2 已确认实施约束

- P-01：匿名期间 `_ID` 稳定，认证成功边界轮换并失效旧 fallback 映射；只迁移白名单非认证字段并清零 Captcha 计数，旧 Cookie 并发请求按 anonymous 处理。
- P-02～P-04：保持无 CSRF、query/form token 和 `_ID`+Captcha 基线；新增能力须由独立 security/interface/delivery 任务定义。
- P-05：跨集合原子提交是成功定义；standalone 仅在有持久化 compensation receipt 时继续，否则保留 token 并失败返回。
- P-06：invalid/missing token 是不恢复身份的幂等登出；有效 token 删除成功才 `Ok:true`；清理失败显式失败并保留重试状态。
- P-07：Web 使用 `Re{Ok:false}` + `registered_relogin_required` Code/Msg 表示已注册未登录，不设置认证 Cookie、不跳转受保护页面；numeric Code、文案和 redirect 由下游按该契约冻结。
- P-08：32 字节 CSPRNG + 无填充 base64url；`SessionId` 存 SHA-256 摘要，resolver/logout 统一摘要查询，并在过渡期兼容旧 24 位 ObjectId token；persistence contract 必须先扩展。

## 6. 测试设计

- 纯 service：密码兼容、规范化、重复账号、注册每个副作用失败点、outbox/补偿、token 类型/过期边界、一次性消费、跨用户更新和 demo policy。
- contract fixture：Web anonymous/source=none、`Session["_ID"]` 惰性持久化、authenticated/session refresh、P-01 轮换或保持现状；API no-token fallback/invalid/valid/expired/action-token type mismatch/storage error；admin/member/demo 配置缺失和不一致；现有 API Golden 的字段/Content-Type/文案。
- API token fixture：按已确认的 P-08 验证 32 字节 CSPRNG、无填充 base64url、摘要存储、resolver/logout 一致性、旧 24 位 ObjectId 过渡和日志/错误脱敏。
- persistence fixture：`sessions` idle TTL/TTL index、`tokens` `(UserId,Type)` 唯一约束、Email/Username/第三方复合身份并发 duplicate、旧 token 文档兼容。
- adapter fixture：SessionWriter Set/Delete/Encode/Commit 任一失败不得返回伪成功；找回密码不存在/存在统一文案；注册自动登录失败要求重新登录且验证 P-07 的公共 envelope；登出清理失败验证 P-06 的公开结果。
- 安全静态扫描：服务包不得导入 Revel/controller；不得有密码/token 日志；不得出现无条件成功或忽略返回值的认证路径。
- 真实 Mongo、HTTP、浏览器、邮件和 multipart 只在对应下游任务运行；缺失环境保持 `unknown`。

## 7. 回滚与兼容

实现按 Auth/User、Session/Token、adapter contract 三个可独立提交边界推进。若 token schema、Cookie 或 envelope 回归失败，回滚对应边界而不回退已归档的 domain-contracts；禁止保留双 resolver、隐藏 fallback 或同时维护两套 demo 规则。
