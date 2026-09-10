# `09-08-application-identity` 规格审核记录（2026-09-09）

## 1. 选叶与审核范围

按父任务 `09-08-business-layer-architecture/design.md §4` 的应用轨道顺序 `identity → notes → content → publishing → admin`，选中的 ready 叶为 `09-08-application-identity`。现场依据：

- `.trellis/tasks/09-08-application-identity/task.json` 的 `meta.depends_on=["09-08-domain-contracts"]`；
- `.trellis/tasks/archive/2026-09/09-08-domain-contracts/task.json` 已归档且状态为 `completed`；
- 其余应用叶虽同样满足领域依赖，但按父任务顺序排在 identity 之后；
- 本轮未创建新任务；该 ready 叶已激活为 `in_progress`，现将 `09-08-infrastructure-persistence` 纳入其前置依赖（`task.json` 记录 `branch=dev`、`base_branch=dev`），业务实现尚未开始。规格结论不以是否运行 `task.py start` 作为证据。

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
- 当前编排已调整为 persistence → identity → interface → delivery；identity 的业务编码必须等待 persistence 任务完成其 contract 和验收材料。其他 application 叶仍可按父任务依赖并行，但不得绕过 identity/persistence 直接推进 interface。

## 6. 审核门禁

审核阶段完成后可激活 ready 叶继续走 Trellis 生命周期；本轮已完成 U-01～U-05 的规格决策，但仍不得把激活等同于实现或真实环境证据通过。进入功能编码前必须重新读取本记录、PRD、设计和实施计划，并由下游 owner 确认其按已批准契约实现；任何偏离需先回到规格材料重新评审。
