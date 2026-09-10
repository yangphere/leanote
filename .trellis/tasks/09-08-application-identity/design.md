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

匿名 principal 固定为 `state=anonymous,user_id="",source=none,token_state=absent,role=anonymous`。有效 Web session 使用 `source=web_session,token_state=absent`，有效显式 API token 使用 `source=api_token,token_state=valid`，API 无 token fallback 使用 `source=web_session,token_state=absent`。任何 `storage_error`、非法 ObjectId、空 UserId 或用户不存在都不能生成 authenticated principal。角色和 demo 派生只从配置绑定的 UserId/管理员配置及已加载用户事实计算，不从请求字段或字面用户名猜测；若 member 未来代表订阅等级，必须另设领域决策，不能复用本字段。

### 2.2 Session reader/writer

- 应用只依赖以下可报告错误的接口，不直接使用 `*revel.Controller`、`revel.Session` 或 HTTP response：`SessionReader.Get(key) (string, bool, error)`、`SessionWriter.Set(key,value string) error`、`SessionWriter.Delete(key string) error`、`SessionWriter.Commit() error` 和明确的 `SessionID` 值；`Commit` 包含 Cookie 编码/写出，编码失败必须作为 `Commit` 错误返回。
- Web Session writer 投影 `BaseController.SetSession` 当前字段，清理身份时删除 `UserId/Email/Username/UsernameRaw/Logo/Verified` 等身份字段；归档迁移设计要求 `ClearSession` 对历史小写 `theme` 键的怪癖原样保留，不能在本任务顺手改成另一种公开 Cookie 形状。
- Cookie codec 由 interface 层注入；ADR-0002 规定 HMAC、`HttpOnly`、`SameSite=Lax`、可配置 `Secure`，无法解码旧 Revel Cookie 时返回 anonymous 并允许重新登录。
- 匿名 session 的稳定标识是 `Session["_ID"]`：缺失时惰性生成 crypto 随机 32 字符 hex，并经 `SetSession` 落入 Cookie。API fallback 使用该标识；不能把每次刷新后变化的 Cookie value 误当成旧 Revel `Session.ID()`。
- 每个 `SetSession/DeleteSession` 都必须在 `ApplyResult`/响应体提交之前刷新 Cookie；任何 Set/Delete/Encode/Commit 错误都必须中止成功响应并映射为既有 storage/失败 envelope。登录、注册后自动登录、Captcha 和 API `_token/_userId` 回写都要有下一请求可见的回归。

### 2.3 API session resolver

API 登录 token 存在 `sessions` 集合的 `SessionId` 字段，解析只负责 token→UserId、`UpdatedTime`/idle TTL 和用户存在性；兼容基线为 3 小时，应用按 `now >= UpdatedTime+3h` 判定过期，persistence 在 `UpdatedTime` 上设置 10800 秒 TTL index 并在有效解析时原子刷新它。它不读取 `tokens.Token.Type`。显式 token invalid/expired/storage_error 绝不 fallback。无 token 时仅以 `Session["_ID"]` 查已有有效映射，成功后把 `_token`/`_userId` 投影回 session；无映射不得创建 session。Cookie 写出顺序和失败映射由 interface-http 保证。

### 2.4 Action token resolver

`TokenService` 通过显式 `Resolve(token, expectedType, now)` seam 查询 `tokens` 集合的 token 值、用途类型、过期时间和 UserId，并返回分类错误。过期比较使用注入时钟和 `info` 常量，禁止共享可变 `overHours`。邮箱/密码 token 的一次性消费必须在持久化边界提供原子失效或补偿。

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
4. 邮件传输由 outbox worker 负责重试和告警；请求路径不得启动无结果 goroutine。自动登录 session writer 失败时注册结果仍明确为“已注册、需重新登录”。

### 3.3 找回密码、邮箱 token

- `FindPwd` 先按规范化邮箱生成带用途 token/outbox；用户不存在或 outbox 入队成功时对外统一返回“若账号存在，将发送邮件”，内部区分 `not_found`、`storage`、`mail`。存储/入队失败映射为不含存在性信息的统一可重试失败，邮件传输失败由 outbox 重试；日志不带邮箱明文或 token。
- `UpdatePwd(token,pwd)` 先验证 `TokenPwd`、用户和新密码 hash，再原子更新密码并消费 token；更新失败不得删除 token。
- `ActiveEmail`/`UpdateEmail` 验证正确用途、过期时间、目标 UserId、邮箱唯一性后更新；失败保留 token 以便重试，成功才消费。

### 3.4 用户资料、demo 和角色

`UserService` 只接受认证 UserId 作为资料更新主体，内部用安全解析后的 `_id` 条件执行更新。邮箱、用户名、第三方身份由唯一索引和 duplicate 映射共同保证；用户名校验绑定 `Vd` 精确规则，`admin` 大小写不敏感。multipart 缺失/空文件在 adapter 早返回，不能索引不存在的文件项。demo 判定封装为单一 policy：`demoUserId` 缺失或与用户名配置不一致时 fail-closed，Web/API/admin/member 适配器不得各自比较字面用户名。兼容旧 API 时，显式 `userId` 只能作为与 principal 的一致性断言；不一致不得改变资料更新主体，并按 `forbidden`/`not_authenticated` 拒绝。

## 4. 适配器矩阵

| 适配器 | 应用调用 | 负责内容 | 不得负责 |
| --- | --- | --- | --- |
| Legacy Web Auth/User | `AuthService`、`PwdService`、`UserService` | 参数绑定、Vd、i18n、redirect/template、session writer | 自行查用户、demo/权限规则、吞掉 service 错误 |
| Legacy API Auth/User | 同上 + `SessionService` | token 参数、`ApiRe/AuthOk`、multipart 解析 | 任意 userId 信任、token 类型/过期判定副本 |
| first-party net/http API | 同上 | `httpserver.Context` 转换、注册表和真实 HTTP 状态 | 改动业务 envelope 或把未注册 action 当作已迁移 |
| main/admin/member before hook | principal resolver | anonymous/role/demo 结果到 redirect/`NOTLOGIN` | 复制用户查询、按用户名判断角色或绕过 demo policy |

当前 `app/controllers/api/httpserver.go` 只注册 ApiAuth 和 ApiTag；ApiUser first-party 注册/路由可达性属于 `interface-http` 下游工作。本任务提供 contract fixture 和 service 行为，不把缺失注册宣称为完成。下游 replay 只验证已批准的 action 方法、principal `userId` 和 405 行为，不再把这些授权规则列为待决策。

## 5. 错误和安全设计

内部错误分类固定为：`invalid_credentials`、`not_authenticated`、`forbidden`、`validation`、`duplicate`、`token_invalid`、`token_expired`、`type_mismatch`、`storage`、`side_effect`、`partial_write`。adapter 将其映射到既有 `Re/ApiRe`、`NOTLOGIN`、`cannotUpdateDemo`、localized message；不新增公开字段或泄露底层错误/凭据。API 使用 HTTP 200 envelope 的 action 继续保持该兼容形状，first-party 新 action 的 transport status 由 interface-http matrix 固定。

数据库错误只能在终止边界记录一次并保留 wrapped cause；服务不得 log-and-return、返回空数据或忽略子操作结果。日志、Golden、错误响应和任务材料均不得出现明文 password、token 或 app.secret。

## 6. 测试设计

- 纯 service：密码兼容、规范化、重复账号、注册每个副作用失败点、outbox/补偿、token 类型/过期边界、一次性消费、跨用户更新和 demo policy。
- contract fixture：Web anonymous/source=none、authenticated/session refresh；API no-token fallback/invalid/valid/expired/action-token type mismatch/storage error；admin/member/demo 配置缺失和不一致；现有 API Golden 的字段/Content-Type/文案。
- persistence fixture：`sessions` idle TTL/TTL index、`tokens` `(UserId,Type)` 唯一约束、Email/Username/第三方复合身份并发 duplicate、旧 token 文档兼容。
- adapter fixture：SessionWriter Set/Delete/Encode/Commit 任一失败不得返回伪成功；找回密码不存在/存在统一文案；注册自动登录失败要求重新登录。
- 安全静态扫描：服务包不得导入 Revel/controller；不得有密码/token 日志；不得出现无条件成功或忽略返回值的认证路径。
- 真实 Mongo、HTTP、浏览器、邮件和 multipart 只在对应下游任务运行；缺失环境保持 `unknown`。

## 7. 回滚与兼容

实现按 Auth/User、Session/Token、adapter contract 三个可独立提交边界推进。若 token schema、Cookie 或 envelope 回归失败，回滚对应边界而不回退已归档的 domain-contracts；禁止保留双 resolver、隐藏 fallback 或同时维护两套 demo 规则。
