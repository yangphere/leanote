# 应用层：身份认证与会话 — PRD

## 1. 任务定位与依赖

本任务是父任务 `09-08-business-layer-architecture` 的身份应用叶；执行依赖为领域契约和 `09-08-infrastructure-persistence` 的持久化契约。领域契约已归档，persistence 必须先完成事务/索引/outbox/token 兼容边界；本任务只在该前置证据可用后收敛身份应用服务和其适配器契约，不提前承担完整 HTTP 迁移或真实环境验收。

目标是让用户身份、密码、Web Session、API Token 和认证授权决策只有一个可测试的业务事实来源，同时保持现有 Leanote 数据库和已发布 `/api/*` 客户端的可观察语义。

## 2. 范围与责任

### 2.1 本任务拥有的业务契约

| 能力 | 现有入口/证据 | 本任务交付 |
| --- | --- | --- |
| 账号登录、注册、第三方注册 | `app/service/AuthService.go`、`app/controllers/AuthController.go`、`app/controllers/api/ApiAuthController.go` | 登录凭据判定、注册开关、重复账号、注册初始化副作用和失败结果 |
| 用户资料、密码、邮箱、头像 | `app/service/UserService.go`、`PwdService.go`、`UserController.go`、`ApiUserController.go` | 用户 ID/邮箱/用户名规范化、密码兼容、资料更新所有权、邮箱 token 语义、头像更新结果 |
| Web Cookie Session | `app/controllers/BaseController.go`、`app/httpserver/session.go` | Session 投影字段、匿名/已登录上下文、Cookie writer contract；Cookie 编码和 HTTP 属性由 interface 层实现 |
| API token session | `app/service/SessionService.go`、`TokenService.go`、`app/controllers/api/init.go`、`api/httpserver.go` | token→UserId 解析、注销失效、过期/类型校验、错误 fail-closed |
| 登录风控与头像入口 | `app/controllers/CaptchaController.go`、`app/controllers/FileController.go` | `_ID` 匿名标识、登录次数/captcha 记账、头像更新的身份边界；文件字节和响应由 adapter/infrastructure 负责 |
| 主站、API、admin/member 认证授权决策 | `app/controllers/init.go`、`app/controllers/admin/init.go`、`app/controllers/member/init.go` | 统一 `AuthenticatedPrincipal`/demo 规则和权限失败分类；各层只做适配或消费 |

### 2.2 明确不由本任务拥有

- `conf/routes` 的完整 net/http 路由注册、请求 binder、响应写入、Cookie 编码实现和真实 HTTP replay 由 `09-08-interface-http` 负责。
- Mongo driver、索引、事务/补偿、token 兼容迁移和注册 outbox 基础设施由 `09-08-infrastructure-persistence` 负责；本任务定义其所需的 service seam、错误语义和兼容验收。U-02/U-03 的持久化变更属于已批准且必须先完成的实现前置，不得绕过该任务直接在 service 中隐式落地。
- 笔记、内容、发布、管理业务自身的所有权规则由对应 application 叶负责；本任务只提供身份上下文和管理员/会员角色判定。
- 前端模板、生成资源、OAuth 新 provider、SSO、MFA、密码算法替换、用户模型/BSON 字段迁移均不在范围内。

## 3. 功能与业务规则

### 3.1 认证上下文

服务调用必须接收明确的认证上下文，而不是读取 controller、Revel session 或 HTTP 全局变量。上下文至少包含：

- `UserId`（规范化 ObjectId 字符串；匿名为空）、来源 `none`、`web_session` 或 `api_token`、匿名/已认证状态；
- token/session 解析结果（不存在、过期、类型不匹配、存储错误必须可区分）；
- 是否 demo 用户、是否管理员/会员的派生判定；匿名 principal 固定为 `role=anonymous`、`token_state=absent`。

空、非法或无法验证的凭据一律匿名/拒绝，不能通过创建新 session、返回零值用户或吞掉数据库错误制造“成功”。

### 3.2 Web 认证

- `/login`、`/doLogin`、`/logout`、`/demo`、`/register`、`/doRegister`、`/findPassword*` 及 `User.UpdateEmail/ActiveEmail` 的公开性沿用现有白名单；其余受保护主站 action 要求 `UserId` 非空。
- 登录成功把用户资料投影到 Web Session（`UserId`、`Email`、`Username`、`UsernameRaw`、`Theme`、`Logo`、布局字段、`Verified` 等），登出删除 Web 身份字段并清理当前匿名/session 记录。Web 登出只清理当前 Web session，不联动撤销独立 API token；API 登出只清理请求中已验证的那个 API token，二者不能互相替代或隐式扩大撤销范围。
- Cookie 必须由 HTTP 层统一签名和写出，应用层只消费 `SessionReader/SessionWriter`。ADR-0002 的 C-b 兼容决策适用：旧 Revel Cookie 无法解码时视为匿名并允许一次重新登录；不得伪造旧编码或静默续期损坏 Cookie。
- 匿名 session 必须维护与旧 Revel `_ID` 等价的稳定标识：`Context.SessionID` 语义为 `Session["_ID"]`，缺失时惰性生成随机 32 字符 hex 并经 session writer 持久化；登录失败次数和 Captcha 必须按此标识隔离。该约束已由归档的 Revel migration 设计确认，当前以 Cookie value 代替 `_ID` 是待修复漂移。
- 登录失败沿用现有登录次数和验证码流程：超过阈值要求验证码，成功后清零；错误响应不含密码或用户敏感字段。

### 3.3 API token 认证

- `/api/auth/login` 成功生成随机 token，在既有 `sessions` 集合写入 token→UserId 映射并返回现有 `AuthOk` 字段；`/api/auth/logout` 删除该映射并返回现有 `ApiRe` 成功信封。
- 受保护 API 从请求的 `token` 参数解析 token；当前兼容路径在无 token 时回退到 Web session identifier。该回退、`_token/_userId` 投影以及其稳定 identifier 约束必须在 adapter fixture 中固定，不能因迁移随意扩大权限。
- API before hook 继续将解析出的 `_token`/`_userId` 写回 session（回写顺序和 Cookie 持久化由 interface-http 负责）；这不是让 API token 进入 Web Cookie 的替代存储。
- 所有 session mutation（登录、注册后自动登录、登出、Captcha、API `_token/_userId`）都必须在 HTTP 状态/响应体提交前写出 Cookie；否则下一请求看不到身份是失败，不得静默忽略。
- API 认证按以下顺序处理：ApiAuth.Login/Register 和 ApiFile 的既有公开读 action 不做身份阻断，但资源所有权仍由 service 校验；受保护 action 有显式 `token` 时只解析该 token，invalid/expired/type mismatch 绝不回退；无 token 时仅允许稳定 `Session["_ID"]` 已映射到有效用户的 Web-session fallback；无映射、用户不存在或 storage error 均拒绝。API session resolver 不读取 action token 的 `Token.Type`，`type_mismatch` 仅适用于 `tokens` 中的邮箱/密码 action token。
- `ApiUser.Info/UpdateUsername/UpdatePwd/UpdateLogo/GetSyncState` 的用户只能来自认证上下文，不能信任任意请求 `userId`。兼容旧客户端时可继续接收显式 `userId` 字段，但它必须与 principal 的 `userId` 完全一致；缺失字段按 action 契约处理，非空且不一致统一拒绝（已认证主体返回 `forbidden`，无认证主体返回 `not_authenticated`），不得据此切换目标用户。该规则覆盖历史文档中的 GET/POST 与 `userId` 参数冲突，后续 replay 只验证兼容证据，不再重新决定授权语义。

身份相关 API 的允许方法在本任务内固定为以下兼容矩阵；未列方法必须返回 405，不能由通配路由隐式放行。历史 API 列表写明 GET 而当前 first-party smoke 使用 POST 的 `/auth/login`、`/auth/logout`，因此保留两种已有证据方法；这不是重新开放任意方法，后续 replay 只验证下表。

| action | 允许方法 | 关键输入/约束 |
| --- | --- | --- |
| `/api/auth/login` | `GET`, `POST` | `email`,`pwd`；成功返回 `AuthOk`，失败不区分用户不存在/密码错误 |
| `/api/auth/logout` | `GET`, `POST` | `token`；清理映射，失败不可伪造成功 |
| `/api/auth/register` | `POST` | `email`,`pwd`；受注册开关、初始化/outbox 契约约束 |
| `/api/user/info` | `GET` | 用户来自 principal；可接收但必须校验一致的 `userId` |
| `/api/user/updateUsername` | `POST` | `username`；用户来自 principal |
| `/api/user/updatePwd` | `POST` | `oldPwd`,`pwd`；用户来自 principal |
| `/api/user/updateLogo` | `POST` | multipart `file`；缺失/空文件失败 |
| `/api/user/getSyncState` | `POST` | 无可选目标用户字段；用户来自 principal |
| `/api/file/getImage`、`getAttach`、`getAllAttachs` | `GET` | 既有公开读白名单仍受资源所有权规则约束 |

### 3.4 注册及初始化

- 邮箱统一转小写并由 service 重验；新用户的邮箱是独立登录别名，`Username` 生成合法的 `[0-9A-Za-z_-]`、至少 4 个字符的小写值，冲突时按稳定后缀重试；旧数据中 `Username=email` 仍只读兼容。密码只保存 hash。`ComparePwd` 继续兼容旧 32 位 MD5 和当前 hash 格式，绝不记录或返回明文。
- `ConfigService.IsOpenRegister` 的当前事实是 `openRegister != ""`（因此 `"0"` 仍为开启）；在没有明确产品决策前以此作为兼容基线，并为 `""`、`"0"`、`"false"` 建立 fixture，不能在迁移中悄然改变。
- 创建用户后初始化三个默认 notebook、注册共享 notebook/note、复制欢迎 note、博客和 About 单页。用户记录与必需初始化必须处于同一 Mongo 事务；事务不可用时使用按 `userId` 幂等的补偿/重试，并将 `partial_write` 暴露给 adapter。任一步骤失败都不得返回 `Ok:true`。
- 激活邮件不在请求 goroutine 中直接发送。用户与 outbox 入队在同一持久化成功边界内；入队成功即返回注册成功，邮件传输失败由 outbox 重试/告警，不回滚用户；outbox 持久化失败返回 `side_effect`/`partial_write` 并执行补偿。禁止无结果 goroutine。
- 第三方注册必须按 `(ThirdType, ThirdUserId)` 检查既有身份、写入 `ThirdType`、保证唯一用户名并检查用户创建结果；失败不能返回未持久化的零值用户。Web 注册成功后的自动登录必须检查 session writer 结果；注册成功但自动登录失败时返回注册成功且要求重新登录，不得伪造已登录状态。

### 3.5 密码、邮箱和 token

- 已登录改密必须验证当前用户和旧密码；找回密码 token 只允许一次成功使用，成功后使同一用途 token 失效。
- `TokenPwd`、`TokenActiveEmail`、`TokenUpdateEmail` 的过期时长沿用 `info` 常量（2h、48h、2h）；邮件模板必须显示对应用途的实际时长。验证查询必须同时约束 token 值和用途类型；使用注入时钟，`now >= expiry` 即过期，不能使用并发可变全局时长。签发同一用户/用途的新 token 时原子替换旧 active token，确保任一时刻最多一个可用 token。
- API session token（`sessions.SessionId`）与邮箱/密码 action token（`tokens.Token`）是两种不同存储和生命周期；前者采用明确的 idle TTL（兼容基线为 `session.expires=3h`），应用按 `now >= UpdatedTime+3h` 判定过期，每次有效解析原子刷新 `UpdatedTime`，persistence 在 `UpdatedTime` 上建立 10800 秒 TTL index；后者才使用 `Token.Type` 和 `CreatedTime` 过期校验，不能混用 resolver 或过期规则。
- action token 允许不同用途并存，但每个 `(UserId, Type)` 最多一个 active token；新文档以独立 token 标识并建立 `(UserId, Type)` 唯一约束。旧 `_id=UserId` 文档在未重签发前只读兼容；同用户/用途重签发必须先原子退役旧文档（设置 `ConsumedAt`），再写独立 `_id` 新文档，任何失败不得留下两个 active token。成功消费必须原子失效，更新失败保留 token。
- 激活/修改邮箱 token 必须绑定目标用户，邮箱规范化后检查唯一性；不存在用户、重复邮箱、过期或类型错误均为失败。
- 找回密码在用户不存在或 outbox 入队成功时统一返回“若账号存在，将发送邮件”，不得泄露邮箱是否注册；内部仍区分 `not_found`、`storage`、`mail`。存储或入队失败映射为不含存在性信息的统一可重试失败，不能按“已找到用户”单独返回失败；邮件传输失败由 outbox 重试，不改变已接受请求的公开文案，也不由 controller 固定吞成成功。

### 3.6 用户资料与权限

- 修改用户名时拒绝空值、非法格式、内置 `admin`（大小写不敏感）和已被占用值，比较不区分大小写并保留 `UsernameRaw`；service 必须安全解析 UserId，非法 ObjectId 返回 `validation` 而不是 panic；成功后同步 Web session 投影。
- 修改密码、用户名、头像必须使用认证 UserId；demo 用户禁止修改的判断统一以 `demoUserId` 为唯一事实来源。缺失或与 `demoUsername` 不一致时 fail-closed、禁用 `/demo` 并记录配置错误，禁止按用户名猜测。
- admin 身份由配置绑定的管理员 UserId/已加载用户事实判定；普通已认证用户的 member route 角色为 `member`，匿名为 `anonymous`。匿名请求必须得到明确 redirect/`NOTLOGIN` 结果；权限不足不能降级为普通用户或匿名读取。
- 头像上传的文件类型、大小、路径和落库失败由相应 adapter/infrastructure 负责；缺少 multipart 文件、空内容或落库失败不得返回成功 Logo，本任务要求 `UpdateAvatar` 的所有权和持久化失败语义。

## 4. 输入、输出和错误契约

| 操作 | 输入 | 成功输出 | 失败要求 |
| --- | --- | --- | --- |
| Web/API 登录 | email 或 username、password、可选 captcha | Web session 或 `AuthOk{Token,UserId,Email,Username}` | 统一错误分类映射为现有 `wrongUsernameOrPassword`；不泄露存在性/密码 |
| Web/API 注册 | email、password、可选 from/invite | 用户与初始化/outbox 入队成功时 `Re/ApiRe{Ok:true}`；Web 可随后登录 | 注册关闭、校验、重复账号、初始化、outbox 持久化失败均失败；邮件传输失败由 outbox 重试，不回滚用户；自动登录失败不伪造已登录 |
| Web/API 登出 | Web session 或 token | redirect `/login` 或 `ApiRe{Ok:true}` | 无效 token 仍不可恢复身份；清理失败必须可观测 |
| 找回/重置密码 | email；token+新密码 | 找回请求统一返回“若账号存在，将发送邮件”；重置返回现有 `Re` 信封 | 不暴露账号存在性；DB/outbox 失败返回统一失败；token 无效、过期或密码更新失败均失败且 token 可重试 |
| 改名/改密/邮箱/头像 | 认证 UserId + 字段 | 现有 `Re/ApiRe` 或 `Logo` | 所有权、输入、demo、重复和 DB 失败分类稳定 |
| API 用户信息/同步状态 | 认证上下文 | 现有 DTO/同步字段形状 | 不接受任意 userId；匿名/无效 token 为 `NOTLOGIN` |

服务层返回可判定的领域错误/结果；controller/HTTP adapter 负责 HTTP 状态、JSON/模板、i18n 文案。现有 API 主要使用 HTTP 200 携带 `Ok`，不得把 transport 200 当作业务成功。公开文案、字段名、Content-Type、键顺序和方法差异以 Golden 为基线，未知处保持 unknown。

## 5. 兼容性与证据边界

- 保留 `users`、`sessions`、`tokens` 集合名、BSON 字段和 `info` DTO；不要求数据迁移。
- 保留 API Golden 中登录、注册、登出、用户信息/资料和 invalid/none token 的 envelope；U-05 已决定按 action 固化允许方法，未列方法返回 405；`userId` 只能与 principal 一致。任何 Golden 变化必须先更新兼容说明和 replay fixture。
- 本任务验收只覆盖 service、adapter contract、静态敏感信息扫描和可运行的 focused tests。真实 Mongo 7/8、完整 HTTP 路由、浏览器、邮件、文件系统和发布证据分别由 infrastructure/interface/delivery 任务承担；缺失证据标记 `partial`/`unknown`，不视为通过。

## 6. 已决策事项与责任

| 编号 | 已决策事项 | 影响 |
| --- | --- | --- |
| U-01 | 统一返回不泄露存在性的成功文案；内部区分 `not_found`/`storage`/`mail` | application-identity + interface/delivery | 找回密码 service、Web/API 文案和 Golden 必须遵循同一映射 |
| U-02 | 用户+初始化原子边界；outbox 入队成功即接受注册；传输失败异步重试 | application-identity + infrastructure-persistence | 事务/补偿、outbox、幂等和 `partial_write`；传输失败不回滚用户 |
| U-03 | 不同用途并存、每用途一个 active token；独立 token 文档和 `(UserId,Type)` 唯一约束；legacy 重签发先退役再新建 | application-identity + infrastructure-persistence | token schema、索引、并发消费、legacy reissue；API `sessions` resolver 不读取 action token type |
| U-04 | `demoUserId` 唯一事实来源；缺失/不一致 fail-closed；admin 由 UserId，普通已认证为 member | application-identity + application-admin/interface-http | demo、role、配置错误响应；禁止按用户名猜测 demo |
| U-05 | 按 action 固化 route 方法；未列方法 405；`userId` 必须与 principal 一致；显式 invalid token 不 fallback | interface-http + delivery | route、binder、Golden/replay 验证已决策契约，不重新定义授权语义 |

上述决策已由本轮“全部采用推荐”确认，并作为本任务的实施基线；任何偏离都必须重新更新 PRD、Golden、fixture 和下游 owner。`_ID` 稳定标识和 `_token/_userId` session 回写按已归档迁移设计作为既定契约，由 interface-http 实现并回归。需求决策已闭合，但真实 Mongo/HTTP/browser/mail/release 证据仍保持下游 `partial`/`unknown`，不等同于需求未决。

## 7. 验收标准

- [ ] 服务级回归覆盖登录（新 hash/旧 MD5/错误凭据）、注册开关/重复账号、初始化失败、登出、改名、改密、头像、邮箱 token、密码 token 和 token 失效。
- [ ] Session/token contract fixture 覆盖 `anonymous/source=none`、有效 Web/API、过期边界、action token 类型不匹配、无效 token、数据库错误、Web Cookie 一次性重新登录和 API fallback；显式 invalid token 不回退，所有拒绝结果 fail-closed。
- [ ] API/Legacy adapter fixture 与现有 Golden 保持字段、信封、文案、Content-Type 和敏感字段约束；方法按本 PRD identity action matrix，参数冲突按 principal 一致性规则验证。尚未运行的 replay 只记录为证据 `partial`/`unknown`，不表示规格未决。
- [ ] admin/member/demo 权限使用同一认证上下文和 demo 事实来源；覆盖 demo 配置缺失/不一致、anonymous、member、admin；跨用户资料修改被拒绝。
- [ ] 密码、token、Cookie secret 和上传路径不出现在日志、错误响应或任务 artifact；service 不直接写 HTTP response、不依赖 Revel controller/session。
- [ ] 运行 service/httpserver/adapter 聚焦 Go 测试、`go vet`、manifest/任务校验和 `git diff --check`；不把未运行的真实 Mongo/HTTP/browser/release 证据标为通过。

## Out of scope

OAuth/SSO/MFA、新密码算法、用户模型/BSON 字段迁移、完整 HTTP 路由迁移、Mongo driver 代码、前端页面与生成资源、邮件供应商切换，以及笔记/内容/发布领域的业务规则。U-02/U-03 所需的 outbox、索引、事务和兼容迁移由 infrastructure-persistence 承接，不属于本任务直接实现。
