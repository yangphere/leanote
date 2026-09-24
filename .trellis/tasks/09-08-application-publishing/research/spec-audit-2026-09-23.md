# `09-08-application-publishing` 规格审核（2026-09-23）

> 注：本文件 §§1～29 记录 2026-09-23 的历史审核过程和当时状态；当前任务状态、补充结论及编码门禁以 §30（2026-09-24）为准。

## 1. 选叶与阶段

按父设计 `.trellis/tasks/09-08-business-layer-architecture/design.md` §4 的应用轨道 identity → notes → content → publishing → admin，当前 `application-identity` 状态 `in_progress`，notes/content 已完成归档。`application-publishing/task.json` 是无子任务的 `planning` 叶，`meta.depends_on=["09-08-domain-contracts"]` 且依赖任务已完成归档；`application-admin` 也满足元数据依赖，但排在 publishing 之后。**选中 ready 叶是 `09-08-application-publishing`。**

本轮没有创建新任务，也没有改业务实现。`task.py current --source` 初始为 none；“依赖 ready”不等于“规格审核完成/可进入实现”。初审时仍有关键决策未决，故未运行会把任务从 `planning` 改为 `in_progress` 的 `task.py start`；不以 `task.py validate` 通过代替用户最终规划批准。下文 §6～§28 按当时用户答复顺序记录决策演进；其中“仍待 Q-P1/Q-P8b/Q-P9”等是历史状态。最新决策和门禁以 §29 为准。

## 2. 证据与范围核对

| 主题 | 现场证据 | 审核结论 |
| --- | --- | --- |
| 上游 | 归档 `domain-contracts/prd.md`、`application-notes/prd.md`、`application-content/prd.md`；父 `design.md`/`prd.md` | `ToCommendId`、`Perm`、owner/USN/receipt、public image 引用、notes/content permission port 已有契约；不能跨任务另建一套 |
| 授权和多写 | `app/service/ShareService.go:321-374`, `:379-443`, `:447-485`, `:619-657`, `:737-849`; `app/service/GroupService.go:30-46`, `:169-225` | 缺少 owner 验证、直接/组优先级实现依赖查询顺序、授予 delete-then-insert、`HasShareNote`/组删除及批量删除忽略部分失败；退组后 permission 需和文件消费一致 |
| 博客公开与互动 | `app/service/BlogService.go:43-70`, `:656-832`, `:935-960`, `:975-1085`; `app/service/NoteService.go:70-76`; `app/controllers/NoteController.go:370-376`; `app/service/EmailService.go:265-342` | 部分 ID 路径不匹配 owner；评论写入未检查 `CanComment`、回复错 note 可成立、计数和通知未随主操作确认、批量发布固定成功；邮件 outbox 目前不支持 comment 事件 |
| 主题和上传 | `app/service/ThemeService.go:267-365`, `:371-475`, `:512-633`; `app/controllers/member/MemberBlogController.go:315-501`; `app/lea/archive/zip.go:130-199` | 文件名仅检查 `..`，Path 直接拼接；ZIP 解压无展开预算/跨平台组件/链接检查；图片原名与扩展名直用；activation、import 的文件/DB 原子性未说明 |
| 公开界面/预览 | `conf/routes:50-116`, `app/controllers/BlogController.go:50-101`, `:607-657`, `:751-865`; `app/controllers/PreviewController.go:24-101`; `public/blog/js/share_comment.js:19-28`, `:75-80`, `:216-287` | 公开 HTML、JSON/JSONP、preview theme 来自不同入口；UI 登录/CanComment 门禁并不等于 service 授权；`callback` 安全例外已定（§20），HTTP 证据未运行 |
| 下游复用 | `app/service/contentpdf/export_adapter.go:62-109`, `app/application/content/image.go:120-155`, `app/application/content/path.go:21-55`; `tests/e2e/business/business-flows.spec.mjs:359-365` | PDF 内有独立 grant 查询，必须并到 permission port；content 已有统一路径/图片接口；现有浏览器 smoke 只测 `/blog` 加载，不是本任务完整证明 |
| 元模型 | `app/info/ShareNotebookNoteInfo.go:76-89`, `:138-149`, `app/info/BlogInfo.go:76-114`, `app/info/ThemeInfo.go:13-38`; `app/httpserver/response_test.go:54-60` | 分享无 `ExpiresAt`；回复 BSON 键现为 `ToCommendId`；JSONP callback 当前有允许 `cb(1)` 的原样测试，改变时必须协调 HTTP 兼容 |

## 3. 原规格缺漏及修订

| 编号 | 问题 | 本轮规格处理 |
| --- | --- | --- |
| F-PB1 | 原 Scope 只列四个 service，遗漏发布 `Note.SetNote2Blog`、单页/设置/推荐、组成员动作、主题图片/ZIP、跨 content/PDF 权限消费者 | PRD 能力边界、`research/action-contract-inventory.md` 逐组列动作、输入/输出与 owner，验收 AC-PB1～PB6 |
| F-PB2 | 旧任务 PRD 写有“过期分享”，但旧模型、路由及 v1.0 实现均无到期字段/访问校验 | Q-P1 用户决定新增访问时失效、逐条可选期限、未填写/旧记录不过期，不用定时任务；owner 可显式改期/续期，采用具体日期时间、按使用者时区显示、UTC 存储/比较，新提交截止时间须晚于服务端当前时刻；不能误称旧功能回归，见 §12～§16 |
| F-PB3 | “权限一致”没有个人/组、note/notebook 与多个组冲突的合成规则，存在越权风险 | note 优于 notebook、同级个人优于组；Q-P2 已确认同级多组取更高权限（`1` 胜 `0`），明确 DB 顺序不构成规则，见 §11 |
| F-PB4 | 旧代码忽略删除/投影失败，根因是授权/组清理/辅助表有多个事实来源 | PRD/design 规定 grant 单一事实来源、幂等 owner key、可核查多写、permission port，禁止 `Ok:true` partial |
| F-PB5 | 原 Blog 范围笼统，漏掉公共状态、owner slug/ID、草稿/回收站、评论开关、错误计数与邮件 goroutine | 定义公开谓词、actor/reply 约束、跨 notes USN、admin outbox 与错误结果；Q-P4 已确认新旧评论纯文本、新提交拒绝纯空白/超过 2000 码点或 8 KiB，见 §18～§19 |
| F-PB6 | 原主题仅要求 canonical path，没有 ZIP bomb/文件原子性/同源脚本威胁模型和确定的预算 | 规定统一 rooted/no-follow、非公开暂存/清理和图片校验；Q-P6 展开预算已由用户裁决，详见 §22。同源脚本风险另见 §10，重构不引入跨域隔离或改 URL |
| F-PB7 | “稳定响应”未对旧 RenderJSON/JSONP、逐 email、主题 iframe/ZIP 和 HTTP 方法划分责任 | 动作清单记录形状，真实 replay 与 Host/JSONP 留 interface；回归和 delegated-unrun 状态分开 |
| F-PB8 | 原计划无中断、并发、权限错误和证据责任细节 | `implement.md` 按授权→发布→主题及相关测试排序，`acceptance/evidence-matrix.md` 显示未运行和跨任务 owner |
| F-PB9 | 评论邮件是无结果 goroutine；现有 `DeliverOutbox` 仅支持账号事件，不能宣称直接使用 admin outbox | Q-P8 已定评论/计数/必要持久化通知意图一同可确认提交，入队失败不留成功评论/计数，SMTP 失败重试且不撤销评论；须扩充 admin outbox，不能误认现有已支持，见 §23 |

## 4. 历史待决项：不能从代码推出目标行为（现由 §26 覆盖）

当时仅 Q-P1 的 API/UI 具体映射和无效日期处理未确认（见 §12～§16）；同一现状可能对应不同产品需求，决定后须同步 PRD/design/implement/动作清单/验收矩阵并重过规划门。Q-P2～Q-P8 已由用户裁决（见 §10～§23）；Q-P8b 删除后待发送事件的取消规则见 §24。Q-P1 剩余合同已在 §25 调查、§26 获用户确认；本表保留当时的证据与问题，不作为当前阻断项。

| ID/优先级 | 问题与依据 | 推荐选项与不同选择的影响 |
| --- | --- | --- |
| Q-P1 / 接口合同待联审 | 用户已确定分享者逐条可选期限、旧授权永久有效、owner 显式改期/续期，并采纳具体日期时间、使用者时区/UTC 存储比较；新提交截止时间须严格晚于服务端当前时刻，等于或早于时拒绝且不改授权，立即停止访问走撤销。旧 UI/API 均无期限输入；尚需设计具体日期格式、精度、无效日期、时区输入及 API 省略/显式清除的外形。 | 建议解析失败/不合法日期拒绝且不变更授权；旧请求省略时创建为无限期、更新保留原期限，明确清除才取消期限。对照现有 `ShareController` 创建/修改签名和分享弹窗后，与 interface/presentation 联审字段/精度及清除编码；不能让空值歧义导致权限扩大或把已过期旧授权的改权误当新期限提交。 |

## 5. 证据、验收与激活结论

- 本轮只进行代码/文档静态审查，未运行 Mongo、浏览器、真实 HTTP、ZIP 导入或新服务测试，也未修改实现。`task.py validate` 验证的是 context manifest 格式，不是 PRD 决策或功能合格。
- `acceptance/evidence-matrix.md` 中 AC-PB1～PB7 的功能证据仍为 `unrun`/`delegated-unrun`；新增 AC-PB4-RETRY 的 Q-P9 已决、但未运行。本叶不能将 interface/delivery 的 HTML/静态/跨环境证据记为已通过。
- 当前结论：**依赖条件 ready，Q-P1～Q-P9 产品规则均已决，规格可提交最终规划摘要供审核**。Q-P1 已采用严格秒级 RFC3339 `expiresAt`、更新时独立 `clearExpiresAt=true`、首次创建无限期/重复 Add 保留原期限（见 §26、§28）；admin 评论投递/取消已写入其任务规格而非只在本叶宣称交接（见 §28）；Q-P9 强制评论/回复提交身份并显式不兼容旧无身份客户端（见 §29）。旧主题同源脚本风险显式记录，不将兼容性记为隔离安全通过。当前不运行 `task.py start`，不编辑 `app/`、`conf/`、`public/` 或业务测试；最终摘要仍须由用户在**之后新消息**明确批准才能激活。

## 6. 用户补充及本轮落实（2026-09-23）

§6～§9 保留先后讨论的历史记录；其隔离域、跳转、活动主题公开 UI 扩展等提案**不是当前生效规格**，最终兼容选择以 §10 为准。

用户最初确认“需要接受管理员上传主题并且可以启用使用该主题”。将其作为硬性可观察需求：管理员上传合法主题 ZIP → 可定位其 owner-scoped 主题 → 显式激活 → 其博客实际渲染该主题；只存文件、仅列出主题、只返回 `Ok:true` 均不足以验收。导入失败不能覆盖当前 active；激活成功应核对 `Theme.IsActive`/`UserBlog.ThemeId` 与实际渲染路径一致。未授权、坏包、DB/文件中断仍须 fail closed。此答复当时未决定他人安装、普通用户上传与 URL/域隔离；前两项由 §7/§8 补充，隔离提案已由 §10 撤回。

当前 `app/controllers/member/MemberBlogController.go:446-501` 的 ImportTheme 与 `:410-428` 的 ActiveTheme/PublicTheme 为分立 action；`app/service/ThemeService.go:371-398`、`:434-475`、`:512-553` 也将激活、导入、公开和安装分开。`ImportTheme` 当前仅返回 `ok,msg`，上传后应通过现有主题列表/业务结果定位 themeId，不能在未评审外部契约时假设新增 wire 字段。使用范围及普通用户上传权限已在 §7/§8 讨论，Q-P7 最终兼容决议见 §10。

## 7. 用户裁决：显式公开与自主安装（2026-09-23）

用户采纳“管理员显式公开主题，其他用户再自主安装使用；上传不自动公开”。因此管理员导入/激活/实际使用、管理员公开、其他用户安装是三个分立的验收阶段；公开必须由真实管理员对自有、已验证主题显式发起，可以公开当前 active 而不必先切换主题；安装由其他已认证用户主动发起，复制到其 own theme 并成功启用。未公开主题不进入可安装列表，也不能凭 source themeId 直接安装；取消公开禁止新安装，失败/竞争不得报成功或破坏原 active。三个内置主题仍按既有可选来源保留。此答复当时不代表普通用户自行上传、同源 HTML/JS 隔离或下架后既有副本策略已决；前两项现已由 §8 部分裁决。

证据：`app/service/ThemeService.go:390-398` 的 `PublicTheme` 仅按管理员身份切换 `IsDefault`，`GetDefaultThemes` (`:261-265`) 只检查该标记；`InstallTheme` (`:512-553`) 同样只检查该标记，复制完成后调用激活但未确认激活结果。`app/views/member/blog/theme.html` 的公开按钮只在其他主题区域，而当前 active 没有该按钮。实现期必须把资格校验收敛到服务，保留既有 action 对活动主题的显式操作能力，但不强增旧 UI 不存在的活动主题公开按钮；不能把“不可安装”误作“静态文件不可直接访问”。当时提出的隔离验收已由 §10 撤回，旧同源风险需单列记录。

## 8. 历史阶段：普通用户上传与隔离提案（2026-09-23，隔离部分已由 §10 覆盖）

用户当时采纳“保留普通用户自行上传并启用自己的 HTML/JS 主题，跨用户脚本来源隔离是上线前提”。普通用户仍能导入合法 ZIP、预览、主动激活并实际使用；不得自动成为他人可安装来源。主题模板、内联/外链 JS 与 `theme.json` 展示值均按不可信输入处理；当时将防止主题脚本获得主站管理界面或其他用户授权上下文视为上线门禁，**该隔离要求随后被 §10 的保留旧行为决议撤回**，不再按任何隔离域、Cookie 属性、URL 跳转或互动改造实施，也不能宣称现状安全。

现状依据：`app/controllers/BlogController.go:50-64` 将用户主题交给博客模板渲染，`:146-179` 的主题资源 URL 是以 `/` 开头的站内路径；`conf/routes:118-128` 将 `/public/*` 与 `/upload/*` 作为静态路由；`app/views/member/blog/theme.html:56,74,120` 将主题 `Info.Desc` 用 `raw` 放入登录后的 member 页面；`conf/app.conf-default:46-49` 允许非 HttpOnly Cookie 及域共享，`BlogController.go:79-100` 还支持自定义域/子域。仅把 JS 移去另一静态域名不能解决主题内联脚本在原页面来源运行；仅把隐藏市场列表当作私有文件也无法解决公开静态路由。现有无 CSRF 基线按 identity/interface 既定决策保持，不能误称隔离已覆盖该风险。

此时待确认是否允许调整旧 `/blog`/`/preview` 页面来源；用户已在 §9 裁决可以将旧页面链接跳转到隔离来源。子域/自定义域、Cookie/预览授权与已登录评论/点赞如何兼容，以及管理员取消公开后既有安装副本策略仍未决；这会影响 Host、凭据作用域、部署要求和 interface/presentation/delivery 的验收范围，不能自行选择。未解决前规格仍未 ready-for-implementation，且无真实跨账号浏览器证明时不得上线。

## 9. 历史阶段：旧页面跳转提案（2026-09-23，已由 §10 撤回）

用户采纳建议：旧 `/blog`、`/preview` 页面链接继续可访问，可跳转到与主站及其他博客 owner 相互隔离的来源，最终浏览器地址允许改变。这是对旧页面 URL 保持同源要求的**有界例外**，不是许可将所有 `/blog/*` 接口改为跳转、放宽 owner/预览授权或改变 JSON/JSONP envelope。`/blog` 缺少 owner 时按现状落到管理员博客；其余页面根据已验证的 owner/资源状态构造受控目标，不信任 Host/query/主题字符串作为任意重定向目标。旧来源不再渲染用户自定义 HTML、主题 404 或提供可同源执行的上传脚本旁路。跳转后单页、分类、文章及 query 的可到达性、循环/跨 owner/无权/不存在及响应状态均需真实 HTTP + 浏览器验收。

现场证据：`conf/routes:50-88,91-116` 的 `/blog` 同时含 GET 页面与 `*` 评论/点赞、GET JSON/JSONP；`app/controllers/BlogController.go:575-602` 的空 owner 首页选择管理员用户，`:79-100` 按 Host 解析自定义域/子域；`app/controllers/PreviewController.go:24-47` 依赖 session 中的 themeId/当前用户；`public/blog/js/common.js:329-340,379-405` 登录后回跳当前页面，互动接口通过 `siteUrl` 指向主站并使用 JSONP。只更换页面域名将使预览的认证状态、主题脚本跨域访问主站及已登录评论/点赞成为独立问题；不能把兼容问题当作已解决。具体隔离域名与 Cookie/凭据作用域、预览跨来源授权、互动是否留在博客页还是转到主站、下架后既有安装副本均仍待后续 Q-P7 决策；隔离实施与真实验证仍 blocked-by-decision/delegated-unrun。

## 10. 用户最终澄清：保留重构前主题做法（2026-09-23）

用户问“重构之前的主题功能是怎样，就保留原样的做法”。以实际旧代码和历史为准，取代 §8/§9 的隔离域、页面跳转与跨域登录交互提案，也不新增活动主题公开按钮：

| 重构前行为 | 证据及本任务兼容边界 |
| --- | --- |
| 登录的普通用户与管理员都可导入 ZIP、编辑/预览/激活自己的主题 | `app/controllers/member/MemberBlogController.go:231-293,410-419,446-501`、`app/service/ThemeService.go:89-97,371-383,434-475`；相对程序根目录导入至 `public/upload/<Digest3(userId)>/<userId>/themes/<themeId>`，保存 owner 记录。仅上传/激活不设 `IsDefault`，不自动出现在可安装列表；管理员及普通用户都能在原博客 URL 实际使用。 |
| 公开是管理员单独的操作，安装是其他用户的操作 | `app/service/ThemeService.go:261-265,390-398,512-553`；管理员通过切换自有主题 `IsDefault` 公布或撤销，公开列表来自该标记。`app/views/member/blog/theme.html:92-104` 的公开按钮仅在“其他主题”，不把活动主题按钮当作既有 UI。安装先复制到安装者主题目录并自动激活；撤销公开阻止新安装，已复制的自有副本不依赖原来源继续使用。原代码只检查 `IsDefault` 且忽略激活失败，这是待修越权/伪成功缺陷，不是应复制的行为。 |
| 主题随现有程序路径、现有 URL/Host 渲染，无新隔离域或跳转 | `app/controllers/BlogController.go:50-64,146-179`、`app/controllers/PreviewController.go:24-47`、`conf/routes:50-128`；模板与资源仍按原路径投送，`/blog`/`/preview` 以及现有 JSON/JSONP、登录评论点赞保持旧外形。原本因自定义域存在的 JSONP 不是要新增的跨域依赖。 |

`git blame` 表明 `ThemeService.PublicTheme`/`ImportTheme` 主流程始于 2014 年，现有实现仍保持基本交互；2026 年的 DB 迁移改了存储适配但没有建立新隔离域。**已记录风险，不视为安全通过**：上传文件位于程序目录不自动使普通用户提供的 HTML/JS 可信；页面和登录后台同源时，恶意主题脚本可借访问者权限发起同源请求，`app/views/member/blog/theme.html:56,74,120` 的 `Info.Desc|raw` 也是单独的注入面。本任务不暗中承诺跨账户脚本隔离，不把“上传不自动公开”误写为静态文件不可访问；仍需修文件越界、owner 校验、错误传播、半包/并发等已经发现的缺陷，并由交付证据明确提示旧风险。Q-P5 已由 §20、Q-P6 已由 §22、Q-P8 主规则已由 §23 裁决；Q-P1 日期输入细则及 Q-P8b 删除后待发策略未完成前不启动实现。

## 11. 用户裁决：同级多组取更高权限（2026-09-23）

用户针对“同时属于只读和可写两个组、且没有个人授权”确认“使用权限更大的授权”。Q-P2 已决：同一资源级别的多个**当前有效组**授权，任一 `Perm=1` 则允许更新，否则有 `Perm=0` 则只读，均无有效授权则拒绝；权限来自同一 owner 的同一资源，不跨 owner 合并。此决议只解决**同级多组**冲突，不改变先选 note、再选 notebook，以及同级个人优于组的既有优先级；例如 note 级个人只读不被 note 级组可写覆盖，note 级组只读不被 notebook 级组可写覆盖。不要以 Mongo 返回顺序或第一条 grant 作判定；权限判断与分享列表的 `Perm` 投影、notes/content/PDF 消费者保持一致。

验收必须覆盖 `0+1→1`、`0+0→0`、更高优先级的只读覆盖下层可写、唯一可写组撤销/退组/删除后降级或失权，以及成员/查询失败不授权。宽松组覆盖限制组是此规则的明确取舍，不能在实现中悄悄改回本轮先前建议的“只读优先”。Q-P5 已由 §20、Q-P6 已由 §22、Q-P8 已由 §23 裁决；仍未决 Q-P1 日期输入细则及 Q-P8b，任务继续处于规格审核阶段。

## 12. 用户澄清：过期分享是新增的访问时授权规则（2026-09-23）

用户先采纳移除过期分享建议，随后明确纠正：“到了过期时间后，不能再查看这个分享了吧，不需要额外的定时任务来做，就是在其他人访问时，判断下时间是否过期，那就可以保留这个过期分享的功能。”以后者为 Q-P1 **生效范围决议**。查证 `app/info/ShareNotebookNoteInfo.go:76-89,138-149` 的 `ShareNotebook`/`ShareNote` 只有 `CreatedTime`；`app/controllers/ShareController.go:19-55,61-80,155-197` 的创建/组分享接口无期限输入；`app/views/share/note_notebook_share_user_infos.html:24-39,72-88` 无期限控件；`app/service/ShareService.go:379-443,471-485` 的授权读取不比较日期。`git show 6987a388:app/info/ShareNotebookNoteInfo.go` 与同版本 `ShareService.go` 也无到期字段/读取判定；旧任务 PRD 在提交 `2ea240f0` 写过“过期分享”，那是**计划中的验收描述，不是旧程序已有功能**。

本任务保留该需求但按**新增能力**规划：分享授权须能附带独立持久化、可比较的截止时间（是否可不填及旧记录规则已由 §13 裁决）；每次服务端授权查询先排除 `now >= expiresAt` 的 grant 再执行 note/notebook、个人/组及多组优先级，拒绝由到期 grant 单独授予的列表、内容、修改、图片/附件/PDF 读取；其他独立有效 grant 按既定规则重算。`HasShareNote` 等辅助投影及尚未清理的数据库记录不能绕过授权；无需定时任务或依赖 TTL index 提前删除。已传到浏览器/下载的内容无法追溯撤回，应对用户说明。改期/续期需求已由 §14 确认，具体日期时间、使用者时区及 UTC 规则已由 §15 确认，新提交期限须晚于服务端当前时刻已由 §16 确认；API/UI 具体字段/精度及无效日期处理仍待联审，禁止擅设固定期限或让旧分享静默失效。

## 13. 用户裁决：可选期限与旧授权兼容（2026-09-23）

用户对建议“分享者可为每条分享选填到期时间，未填写及已有分享均不过期”答复“采用建议”。Q-P1 至此已确认：个人或组的 note/notebook 分享 grant 各有独立可选截止时间；新建时不填、旧客户端不传或旧 BSON 记录没有该字段，均按无期限处理，不能从 `CreatedTime` 推断期限或在迁移时给既有 grant 批量设置默认到期。到期检查仍由实际访问方每次授权时执行，不依赖清理任务；有效 grant 优先级和已到期 grant 的剔除规则见 PRD PB-01。

对现有只改权限的 action，遗漏新增截止时间字段必须保留该 grant 原有期限，不得通过更新实现的先删后建静默变为永久分享；这属于保持上述既定期限的必要兼容约束。原本待确认的 owner 显式延长、缩短或清空截止时间（包括已过期授权）现由 §14 确认；前端输入/API 字段的具体形式仍未决。尚未写业务实现或设定新的到期长度。后续验收应覆盖逐 recipient/组的独立期限、旧数据、未填与已有期限的区别，以及旧版更新不意外续期。

## 14. 用户裁决：分享者显式改期与续期（2026-09-23）

用户对“分享者可显式延长、缩短或取消已有分享期限，包括续期已过期分享；普通权限修改不自动续期”的建议答复“采用建议”。Q-P1 的更新规则因此明确：只有真实 grant owner 可对其自有的每条个人/组 note/notebook 授权显式改期/取消期限；已到期记录可以由 owner 明确设置未来期限或取消期限后重新生效，不要求先撤销再新建。被分享人或其他用户不能续期；仅调整 `Perm` 的旧版请求、缺少到期字段的更新均保留原期限，不自动放行。失效记录若写入失败或结果未知，必须保持拒绝访问或先读回确认再回报，不能靠内存响应称续期成功。

本节记录当时待决项；绝对日期时间与时区选择已由后续 §15 覆盖，过去/当前时间规则由 §16 覆盖。无效日期的边界、精度和新旧 API/页面如何表达明确清除与字段缺失仍待联审。Q-P3 的草稿预览已由 §17 裁决，Q-P4 的评论输入已由 §18～§19 裁决，Q-P5 的 callback 安全例外由 §20、Q-P6 预算由 §22、Q-P8 主规则由 §23 裁决；旧主题同源风险及 Q-P8b 仍阻断规划门，未激活或开始业务编码。

## 15. 用户裁决：具体截止日期时间与时区（2026-09-23）

用户对“采用具体日期时间，按用户时区展示、以 UTC 比较；相对时长另需定义预设值和起算点”的建议答复“采用建议”。Q-P1 的输入方式已决：分享者逐条选填具体截止日期时间，界面按使用者时区呈现，服务端转换为绝对时刻并统一以 UTC 持久化和比较；不引入相对时长预设。旧版未带期限的创建仍为无限期、更新仍保留原期限；本次裁决没有决定具体字段/格式、精度、无效日期及过去/当前时间的处理，也没有授权用空值隐式清除既有期限。

本节记录当时的过去/当前时间问题，已由后续 §16 裁决。具体 API/UI 字段、明确清除的编码及无效日期处理仍待 interface 联审；Q-P3 已由 §17、Q-P4 已由 §18～§19、Q-P5 已由 §20、Q-P6 已由 §22、Q-P8 已由 §23 裁决，Q-P8b 仍未决；未开始功能编码。

## 16. 用户裁决：显式新期限必须晚于当前时刻（2026-09-23）

用户对“新建或修改分享时，到期时间早于或等于当前时间一律拒绝；需要立即停止访问时撤销分享”的建议答复“采用建议”。Q-P1 由此补齐时间边界：服务端只在**显式提交新期限**时用服务端时钟校验 `expiresAt > now`；等于或早于当前时刻拒绝本次操作，保留原授权及期限，不产生新建即失效的 grant。已存记录到期后，在旧客户端仅改权限或其他未附新期限的请求中，不得把旧期限误判为新提交，亦不得暗中清空或续期。已过期记录须由 owner 显式提交未来新期限或明确取消期限才能重新授权；如需立即终止尚有效的授权，使用现有撤销分享。

依据当前 `app/controllers/ShareController.go` 的创建/改权 action 仅接收资源、收件人或组及 `perm`，`app/views/share/note_notebook_share_user_infos.html` 的旧弹窗没有到期字段；新 API/UI 格式、精度、无效日期拒绝策略和“省略/明确清除”编码须继续与 interface/presentation 联审，不能臆造老版本客户端如何传递新增参数。Q-P3 草稿预览已由后续 §17、Q-P4 评论输入已由 §18～§19、Q-P5 callback 已由 §20、Q-P6 ZIP 预算已由 §22、Q-P8 主规则已由 §23 裁决；Q-P8b 仍是用户产品决策；任务未激活、未开始功能编码。

## 17. 用户裁决：主题预览仅显示已发布内容（2026-09-23）

用户对“保持重构前行为：未启用的自有主题可预览已发布内容，不展示草稿”的建议答复“采用建议”。Q-P3 已决：登录用户可选自己拥有的未激活主题，在现有 `/preview/*` 路由下渲染本人的有效已发布博客内容；不因预览而激活主题或改变任何内容的公开状态。预览仍要保持正式博客的 owner 与发布谓词（`IsBlog=true`、`IsTrash=false`、`IsDeleted=false`），列表/搜索/分类/标签/文章详情及单页均不得借预览暴露未发布、撤销发布、回收站或删除内容，即使访问者是作者也不例外；不增加草稿预览路由。匿名、伪造他人 themeId 或 URL userId 的请求不得得到他人的主题或私人内容。

现有 `app/controllers/PreviewController.go:24-68` 从 session 和当前用户取得主题，再调用 `Blog` 页面；`app/service/BlogService.go:43-62` 的文章查询只取公开笔记。原实现对其他预览路径与误传 owner 的行为仍须逐路径回归，不把重构前可能的越权或 404 差异当作应保留的兼容语义。此决议只冻结产品范围，不构成预览权限、静态资源或真实 HTTP 验收已通过；Q-P1 的 API/UI 映射及 Q-P8b 仍须收敛，Q-P4 的空白/长度由 §19、Q-P5 的 callback 由 §20、Q-P6 预算由 §22、Q-P8 主规则由 §23 裁决；任务保持 planning。

## 18. 用户裁决：评论正文统一按纯文本显示（2026-09-23）

用户对“评论正文统一按纯文本处理，把 HTML 标签作为文字显示；若支持富文本则需额外界定标签、清理规则及旧评论兼容”的建议答复“采用建议”。Q-P4 的格式范围已确定：新旧评论均不将正文作为富文本；`<tag>`、脚本文字等按字面内容展示而不执行，不丢弃标签或通过存储层预先做 HTML 转义。保持 JSON/JSONP 中原有正文文本字段，博客页面、预览及邮件模板需要按实际输出上下文转义，防止脚本注入及重复编码；不回写或截断历史正文。此决议不等于现有模板已安全，真实浏览器、邮件和旧评论样本仍须验收。

查证 `app/service/BlogService.go:699-723` 只以 `content==""` 验证并将原文写入 `BlogComment.Content`；`app/controllers/BlogController.go:825-862` 将评论经 JSON/JSONP 输出；`public/blog/js/share_comment.js:96-103,160-173` 用模板 HTML 插入页面；`app/service/EmailService.go:488-533` 把原文送进可配置邮件模板。需要检查所有这些展示边界以及已有存量 HTML 评论，不能把 JS 端 UI 检查当作 service 安全规则。Q-P4 的空白与长度限制已由后续 §19、Q-P5 的 callback 安全例外已由 §20、Q-P6 预算已由 §22、Q-P8 主规则已由 §23 裁决；Q-P1 接口合同及 Q-P8b 亦未决，任务仍为 planning。

## 19. 用户裁决：评论空白与长度上限（2026-09-23）

用户对“纯空白拒绝、正文最多 2000 个 Unicode 码点且 UTF-8 不超过 8 KiB、超限直接拒绝不截断”的建议答复“采用建议”。Q-P4 输入规则已决：新提交的评论和回复统一在服务端校验合法 UTF-8、非全 Unicode 空白、码点数 `<= 2000` 且 UTF-8 字节数 `<= 8192`；任一不满足就拒绝，原文不被截断，评论记录、计数与通知均不发生副作用。空白检查不意味着自动裁剪提交文本；已有评论仅安全呈现，不以新规则回写、截断或删除。错误由 application 明确标识，HTTP/JSONP 适配保留旧响应外形并联审非法输入失败映射。

对合法 UTF-8，每个码点最多 4 字节，2000 码点至多 8000 字节，因此在本合同下“8 KiB 但未超过 2000 码点”不可达；8 KiB 是附加容量约束，验收不能构造伪造的独立超字节界用例。以多字节字符、表情、Unicode 空白、`2000/2001` 码点和历史超长数据覆盖实际边界。Q-P5 callback 兼容已由 §20、Q-P6 ZIP 预算已由 §22、Q-P8 通知主规则已由 §23 裁决；Q-P8b 删除后待发事件与 Q-P1 分享期限 API/UI 合同仍阻断规划，任务保持 planning。

## 20. Q-P5 用户裁决：收窄公开 JSONP callback（2026-09-23）

用户对“在 HTTP 输入边界仅接受有限长度的 JavaScript 标识符或点式路径，拒绝 `cb(1)` 等表达式”的建议答复“采用建议”。经审查 `app/controllers/BlogController.go:763-901` 多个公开 JSONP action 直接将 callback 传给 `RenderJSONP`，`app/httpserver/response.go:105-127` 的底层 renderer 原样拼入脚本；`app/httpserver/response_test.go:54-60` 固定 primitive 拼接但没有证明公开接口允许任意表达式。内置 `public/blog/js/common.js:39-51` 使用固定合法 `jsonpCallback`。故在旧控制器与新 HTTP adapter 的共同入口执行相同校验，不留直通旧路径；本叶只定合同，下游 interface 负责实际实现与 HTTP replay。

合同：callback 完整匹配 1～128 字节 ASCII 标识符片段 `[A-Za-z_$][A-Za-z0-9_$]*`，或用点连接多个非空片段（允许 `jsonpCallback`、`foo.bar_1`、`$._cb`），不自动裁剪/修复；显式空、`cb(1)`、`foo..bar`、括号/引号/换行/转义及超过 128 字节一律拒绝，不能输出可执行 JSONP 或先执行评论/点赞等副作用。JSONP-only action 缺失 callback 也拒绝，`GetComments` **仅参数缺失**时保留旧 JSON 分支与其不同的 Item 形状；成功回调保持旧 `callback(json);`、`application/javascript; charset=utf-8` 和各 action DTO。错误 HTTP status/body 尚须在 interface 联审、真实请求冻结；此处不把 renderer 原样测试当作公开安全承诺，也不假定所有外部第三方客户端均未使用超长 callback 或表达式：此有界兼容收窄可能使其失效，交付时必须注明。验收见 AC-PB6 与 `research/action-contract-inventory.md` B-JSONP；本轮未执行接口测试，Q-P6 预算已由 §22、Q-P8 通知主规则已由 §23 裁决，Q-P1 API/UI 与 Q-P8b 仍待收敛，未激活。

## 21. Q-P6 预算调查与待确认提案（2026-09-23）

现场证据：`app/controllers/member/MemberBlogController.go:486-489` 仅在读取文件后拒绝大于 10 MiB 的 ZIP；`:389-392` 的图片入口拒绝大于 5 MiB；`app/service/ThemeService.go:560-570` 在解压之后对模板文件数设 100 门槛；`app/lea/archive/zip.go:130-199` 当前以不受限的 `io.Copy` 解压，未在输入或写入过程中对展开尺寸/目录条目数/深度施加预算。`public/blog/themes/default`、`elegant`、`nav_fixed` 的在库文件数量分别为 23/21/21、总大小分别为 186719/134878/189381 字节、最大文件分别为 111868/85060/139058 字节、相对路径深度均为 2；这只能作为内置兼容样本，不代表全部旧用户主题，也未运行真实 ZIP 导入。

**当时待确认、现已由 §22 用户采纳的提案**：保留 ZIP 压缩上限 10 MiB、主题文件上限 100；新增单个常规文件展开不超过 5 MiB、全部常规文件累计展开不超过 20 MiB、ZIP 条目（含目录）不超过 200、去掉允许的单个主题顶层文件夹后相对路径不超过 8 层。目录与文件预算在解压前预检、解压期间用限额流计数并在越限时停止及清理非公开暂存；不单设压缩比上限，因为展开大小和条目已受限、极高压缩率的合法 HTML/CSS 也不应因比率被误拒。明确的取舍：旧 ZIP 若超过新增预算会被拒，需要提供错误而非部分主题；如选择更高预算，要增加磁盘/CPU 暴露并扩充验收矩阵。本轮已将确切预算同步 PRD/design/implement/清单/验收矩阵，证据仍未运行。

## 22. Q-P6 用户裁决：流式安全预算（2026-09-23）

用户对 §21 推荐的数值和不另设压缩比上限答复“采用建议”。归一合同：ZIP 压缩包 `<=10 MiB`，常规文件 `<=100`，每个常规文件实际展开 `<=5 MiB`、全部常规文件展开合计 `<=20 MiB`，全部目录及文件条目 `<=200`，允许的单个主题顶层文件夹移除后规范化相对路径深度 `<=8`；上限值本身可通过，超出任何一项均拒绝，ZIP header 声称大小不替代对实际流量限额。不单设压缩比阈值，但有效大小/条目/路径上限仍强制执行。失败/中断在不可公开 stage 清理，不改变旧 active、metadata 和可安装列表；老用户 ZIP 超出新预算时会收到明确失败，不按旧行为静默接受或发布半包。PRD 的 AC-PB5-ZIP 与验收矩阵分别核对边界值、伪造大小、高压缩比合法文本、目录洪泛和跨账户导入；目前没有真实 ZIP/浏览器运行证据。Q-P8 主规则已由 §23 确认；Q-P1 接口细则和 Q-P8b 删除后待发规则仍阻断规划，暂不启动任务或功能编码。

## 23. Q-P8 用户裁决：评论提交与必要通知意图共同确认（2026-09-23）

用户对“评论与通知意图都持久化才算提交成功；入队失败不留下成功评论或计数，邮件发送失败则重试、不撤销评论”的建议答复“采用建议”。证据：`app/service/BlogService.go:699-739` 当前评论插入和计数写入分离，即使插入失败也可能执行 `go sendEmail`；`app/db/outbox.go:29-83` 已有可持久化、支持幂等键的 outbox 基元；`app/service/EmailService.go:265-342` 的 `DeliverOutbox` 现仅处理激活和重置密码事件，评论事件/模板/无 token payload 尚未支持；`app/db/initialization.go:64-143` 的用户初始化有事务及事务不支持时的补偿，但不能直接假定补偿足以保护评论与投递免于可见半写。

合同：旧有应通知规则的对象由真实 note 和本 note 的合法回复确定；有应通知对象时评论、计数与持久化通知意图作为一次可确认成功，无收件人时不产生通知事件；通知入队失败/未知提交不返回 `Ok:true`，不能留下可见评论/多计数或可投递的孤儿意图。需要真实事务或具备 pending 可见性闸门、幂等身份与对账的可恢复流程，不能再从请求路径发无结果 goroutine；实际部分写入必须返回明确 `partial_write` 并修复，不能暗自当成功。admin 须扩充 comment event/安全邮件模板/失败 retry-dead 可观测，SMTP 失败不撤销评论；不把重试等同于邮件 exactly-once 投递。comment body 在邮件 HTML 输出边界转义，含用户信息的 payload/错误日志按最小化和脱敏原则设计。PRD AC-PB4 与动作清单 B-NOTIFY、验收矩阵 AC-PB4-NOTIFY 明确证据责任，均未运行。

当时尚未确定 Q-P8b：`app/service/BlogService.go:775-800` 的删除是硬删除，既有邮件发送是无结果 goroutine、没有持久化待发事件；引入 outbox 后，评论已删除但事件尚未投递时，是取消通知还是继续告知原收件人？该历史待决现由 §24 的用户裁决覆盖。Q-P1 到期字段的 HTTP/UI 合同仍待联审；任务继续处于规划审核状态。

## 24. Q-P8b 用户裁决：删除评论时停止尚未交接的通知（2026-09-23）

用户对“删除评论时取消尚未开始发送的通知；已交给邮件服务或已发送的消息不能撤回”的建议答复“采用建议”。领取待发事件本身不构成发送交接：删除先于 transport 实际交接时，pending、retry 和 worker 已领取但未交接的事件均不能再发送；交接先于删除或已发送的邮件不可召回。保留有权删除及旧 HTTP 形状，不给无权删除取消通知的能力。现有 `BlogService.DeleteComment` 硬删除/减计数、`app/db/outbox.go` 和 `DeliverOutbox` 均未实现评论事件取消；这一条是**后续实现契约，不是已有能力**。

publishing 须以稳定 comment ID 关联 outbox 意图及删除结果；admin 须提供跨 worker 可观察、与 transport 交接可确定先后顺序的取消/投递协议，不能仅凭读取一次状态、硬删除评论或 worker 已领取便报告停发。成功删除和停用未交接意图要有共同确认或可恢复的失败状态；如果停用失败或提交结果未知，不得将仍可投递的通知报告成已取消，需读回/对账且防止已删除评论正文在后续发送中泄露。不声称 SMTP exactly-once 或已交接邮件可撤回。PRD AC-PB4-DELETE、设计删除接缝、执行矩阵、动作清单 B-NOTIFY-DELETE 及验收矩阵 AC-PB4-DELETE 均需覆盖重复删除、领取/交接/取消竞态、失败与未知提交，并和 admin/interface/delivery 联验；此处未运行任何投递验证。

## 25. Q-P1 剩余接口/界面合同的证据与当时候选建议（见 §26）

`app/controllers/ShareController.go:19-55,116-123,155-193` 中个人分享创建只接收 `emails[]`/`perm`，改权只接收 `toUserId`/`perm`；组分享添加及修改复用同一 action。`app/service/ShareService.go:321-374,619-640,737-815` 的组改权先删再建，须避免把已设期限抹为永久。`app/info/ShareNotebookNoteInfo.go:76-89,138-149` 仅有 `CreatedTime`；`app/views/share/note_notebook_share_user_infos.html:19-56,72-88,122-183` 和 `public/js/app/share.js:472-588` 只有权限及删除操作，个人分享旧页面为 POST 创建/GET 改权，组分享为 POST；旧请求不存在日期字段或清除信号。现有事实不能推出新字段的名字、精度及清除语义。

**当时候选合同，现已由 §26 确认：**沿旧 action/结果外形增加可选 `expiresAt`，提交 RFC3339 带 `Z` 或数值时区偏移的秒级绝对时间，服务端严格校验合法日期、格式及 `expiresAt > now`；提供独立显式 `clearExpiresAt=true` 仅用于更新已有授权，两个字段同时出现或空白日期值均拒绝。参数省略时创建无期限、修改保留原期限；客户端发送单条 `emails[]` 时只影响该 grant，多邮箱同次提交共享同一指定期限，仍逐条持久化。界面按使用者当前时区展示本地日期时间及明确的时区提示；转换到带偏移时间再提交，无法无歧义表示的本地时间（夏令时跳过/重复）须拒绝或引导选择明确偏移，而不能静默改到另一时刻；已设期限必须显式点“清除期限”，仅把输入框留空不能自动清除。无效输入拒绝且不修改权限/期限，不得将旧版仅改权请求误作新期限。落地时与 interface/presentation 联验，交付证据不能由静态审查代替。

## 26. Q-P1 用户确认：期限输入、清除与错误合同（2026-09-23）

用户在收到 §25 候选及“多一个清除字段避免旧请求意外放宽权限”的取舍说明后答复“采用建议”。已据此在 PRD PB-01/AC-PB1-DATE、`design.md` 分享合同、`implement.md` 执行负例、动作清单 S1/S4/S6、验收矩阵 AC-PB1-DATE 中冻结：可选 `expiresAt` 精确到秒且要求 RFC3339 `Z` 或数值时区偏移（不接受小数秒、无偏移或非法日历时刻），统一 UTC 存储/比较；服务端只对显式新期限校验晚于其当前时间。省略字段的新建无期限，省略字段的更新保留已有期限（包括已过期 grant），重复授予不得重置期限；仅已有授权的 owner 可用独立 `clearExpiresAt=true` 清除期限。空白日期、非法清除标志、两个字段同传、创建时清除及过期/当前新期限都拒绝，不修改授权；旧个人 email 逐人结果、改权 bool 和组 `Re.Ok` 的合法响应形状保持，跨收件人的每条 grant 独立持久化。

界面按用户时区显示/输入并给出时区提示，将无歧义本地时间转换为带偏移的绝对时刻提交；夏令时跳过/重复的歧义值不得静默调整，必须拒绝或引导明确偏移，清空输入框不替代显式“清除期限”。接受此合同并不证明 HTTP/浏览器已运行：interface/presentation/delivery 需在各自负责阶段覆盖真实日期输入、原响应、权限生效及跨时区/夏令时边界；无环境保持 `delegated-unrun`。截至本轮无阻断产品决策，但旧主题同源脚本风险仍未解决且必须显式移交；最终规划摘要获用户后续明确批准以前，任务状态仍为 `planning`，不得写业务实现。

## 27. 规划复核：旧运行入口与后端指南的时序（2026-09-23）

`.trellis/spec/backend/index.md:5` 将“Revel runtime removed”表述为既成事实，但 `go.mod:16`、`app/init.go:6`、`sh/run.sh:8` 仍有 Revel 运行入口，`cmd/leanote/main.go:23,108` 则已有第一方 HTTP 框架；`09-08-interface-http/prd.md:5,14,30` 将**彻底去除** Revel 生产依赖明确留给后续接口任务。这是指南的目标态与当前过渡态错位，而非分享/博客的产品待决项。实现上下文需要同时包含本审核记录和 backend guideline；本叶以现有旧入口兼容回归、新适配器合同为界，不依据指南一句话误判旧控制器已下线，亦不在本叶擅自删除 Revel 代码或修改共享指南。真实 HTTP 接口淘汰/发布证据继续由 interface/delivery 负责。

## 28. diff-review 修订：重复授予、admin 交接与 Q-P9（2026-09-23）

- 原动作清单 S1 将未携带 `expiresAt` 的旧 Add 无条件写作“省略即无限期”，与 §26 的“重复授予不得重置期限”冲突。`ShareService.AddShareNoteToUserId` 等旧实现采用先删后插，重放旧请求会把已有期限意外变成永久。现已在 PB-01、design、implement、S1、决策摘要和 AC-PB1-DATE 固定按唯一键的**实际存在状态**区分：仅无授权时首次创建为无限期；已有授权再 Add 是更新，省略即保留期限，显式清除仅命中已有授权。混合新/已有 recipient 的批量清除标志先整批拒绝，不能先改一部分；旧无字段响应保持原外形。
- 评论通知最初仅在 publishing 规格/矩阵指派给 admin，但 `application-admin/prd.md`、`design.md` 和 `implement.md` 原来只列 feedback/账号邮件 outbox，无法保证下游接收。现已在 admin 的三份任务材料加入 comment kind、邮件模板、retry/dead、已确认才可领、删除与 transport 交接可确认停发/终态、跨 worker 竞态及 publishing 联验；其现有 `DeliverOutbox` 仍**未实现**评论事件或取消。publishing 的 AC-PB4-NOTIFY/AC-PB4-DELETE 未获双方和真实环境证据前不得标通过；这是交接验收门禁，不额外声称父任务轨道顺序造成死锁。
- `BlogController.CommentPost(noteId,content,toCommentId,callback)` 没有提交身份，`BlogService.Comment` 每次调用 `db.NewObjectID()`；`db.EnqueueOutboxEvent` 仅在**传入相同** idempotency key/ID 时可回读同一事件。故 design 所述 comment ID/recipient 只能防止同一已识别评论的内部 outbox 重试重复入队；若首次评论和通知已提交但 HTTP 应答丢失，原样重发会生成新 comment ID 和第二条通知，服务端也无法从同文请求推断它与用户有意再次发同文不同。不能用 actor+内容+时间窗冒充幂等身份，否则会吞掉合法的两条评论。
- **Q-P9（产品/兼容待确认）**：建议新客户端每次有意提交生成稳定身份，网络重试沿用该身份，以 actor/note/reply/请求摘要校验并与评论/计数/意图共同持久化；旧未携带身份的客户端是保留原样可发布、但无法保证跨请求断网重试不重复，还是强制提交身份而使旧客户端不兼容？这两种行为不能同时无条件保证。当前只冻结单次操作/同一 comment ID 的内部去重，不擅自新增 HTTP 字段或降低“跨请求重复不发通知”的承诺；待用户裁决并补 AC-PB4-RETRY 的旧/新客户端验收及 interface/presentation 映射，才可重新报送最终规划摘要。Q-P1～Q-P8b 仍属已决，Q-P9 唯一新的产品阻断；任务继续 `planning`，未进入功能编码。

## 29. Q-P9 用户裁决：不兼容无身份的旧评论客户端（2026-09-23）

用户对“新客户端为每次有意评论提供稳定提交 ID；旧无身份客户端继续评论但断网重发无法去重，或强制提交 ID 并破坏旧接口”的选择答复“**不需要兼容旧客户端**”。因此 Q-P9 已决：`CommentPost` 的评论与回复必须携带 `submissionId`（128-bit 安全随机身份、32 位小写十六进制）；缺失、显式空、非法格式在任何评论、计数或通知写入前拒绝，不能静默分配新身份代替客户端。每次有意提交换新 ID，网络重试沿用原 ID；同一已认证 actor+ID+note/reply/原文只能确认一次，响应丢失后重试读回同一评论结果，同 ID 不同输入 conflict 且零写入；不同 actor 不能串线。服务端把身份与请求摘要、comment ID、提交状态、必要通知意图关联持久化，未知写入状态需查证/对账，评论删除保留防重建/重发的墓碑语义；没有通知对象也须去重评论和计数。comment ID/recipient 的 admin outbox 去重另行验收，不以此冒充跨请求幂等；不承诺邮件 transport 恰好一次发送。

本裁决**仅限评论/回复提交**；分享期限的旧请求兼容、主题 URL/Host、合法 JSON/JSONP 成功响应、其他互动接口保持之前合同。`interface-http` 负责必填绑定/非法值与同身份冲突的真实 HTTP 错误，`presentation-frontend` 负责 `public/blog/js/common.js:396`、`public/blog/js/share_comment.js:269` 两处生成新身份及重试复用，delivery 验证真实断网/响应丢失；本轮仅更新 publishing 的 PRD/design/implement、动作清单、实施摘要和验收矩阵，未编辑业务实现。AC-PB4-RETRY 从 `blocked-by-decision` 改为 `unrun`，其跨层证据仍 `delegated-unrun`。现可呈交最终规划摘要，待用户**收到摘要后另行明确批准**才运行 `task.py start`；用户本条兼容裁决不视为开始功能编码的批准。


## 30. 当前规格审核补充（2026-09-24）

本节是当前状态和规格结论，优先于 §§6～29 中记录的历史规划状态。历史材料中的 `planning`、等待 `task.py start` 或等待用户批准均为当时的过程记录；本叶已在不创建新任务的前提下激活，`task.json.status=in_progress`，但尚未进入功能编码，也没有新增运行证据。

### 30.1 已依据现有上下文冻结的补充规则

- PRD 范围表已统一为管理员和普通用户均可上传并自用主题；只有管理员显式公开自己拥有且已验证的主题，其他用户才可主动安装。
- 组 owner 沿现有 `GroupService.IsExistsGroupUser`、`GetMineAndBelongToGroupIds` 语义视为有效成员，即使没有 `GroupUser` 文档；普通成员删除动作不能移除 owner，删除组才终止该资格，不补写伪成员记录。
- 所有博客互动必须绑定真实 note/comment：`DeleteComment` 校验 `comment.NoteId == noteId`；`LikeComment` 校验 actor、评论所属 note、公开谓词和评论开关；列表、统计、点赞、阅读数 fail closed；回复目标必须存在并属于同一已发布 note。
- 自定义主题的编辑、导出、删除和文件读写仅限 owner，管理员也不能跨 owner；内置主题只读。非 active 删除必须清理 metadata、文件树和静态入口；导出临时根和下载名必须受 owner 约束；失败/未知需读回或补偿，不能遗留静态孤儿。
- `theme.json` 的 `Name`、`Version`、`Author`、`AuthorUrl` 若存在必须为字符串；异常类型返回可识别 validation/template 错误，不得由类型断言 panic。
- `SetNote2Blog` 已按用户 2026-09-24 采纳的推荐冻结为原始 boolean 外形：所有 note mutation/USN/receipt 均确认成功才为 `true`，任一失败/未知/部分写入为 `false`；逐项诊断和修复通过内部 receipt/对账提供，当前旧实现无条件返回 `true` 不能作为实现基线。
- URL title、摘要和单页投影不得先清空、多写后覆盖或忽略第二写入；排序必须是已有 single ID 的完整合法排列，未知/重复/缺失 ID 零写入或显式 `partial_write`。

### 30.2 审核时发现的阻断合同（已于 2026-09-24 解除）

1. **分享期限 schema（PB-SCHEMA-01）**：审计时现有 `ShareNote`/`ShareNotebook` 只有 `CreatedTime`，现已冻结为可选 `ExpiresAt time.Time` / BSON `ExpiresAt`、秒级 UTC DateTime；缺字段永久有效，不使用 TTL 访问控制；索引 preflight 历史冲突阻止静默启动，迁移/回滚不得改变旧记录语义。
2. **SetNote2Blog 失败合同（PB-BLOG-01）**：当前 controller 丢弃逐项结果且无条件返回 `true`；现已冻结为 raw boolean，全部确认成功才 `true`，任一失败、未知或部分写入为 `false`，并通过内部 receipt/对账定位修复。
3. **评论提交凭据（PB-RECEIPT-01）**：现已冻结独立 receipt，唯一键 `(ActorId, SubmissionId)`，绑定 note/reply/正文 SHA-256/comment ID/状态/删除墓碑/通知意图 ID/时间；同键同摘要回放，冲突零写入，删除后保留墓碑；GC 仅在保留期届满、关联 outbox 终态且完成对账后执行，不使用未经确认的 TTL。
4. **评论 outbox 状态机（PB-OUTBOX-01）**：现已冻结 comment/recipient payload、取消请求、transport handoff 时间、取消时间/原因及 CAS 版本；领取不等于交接，删除先于交接时取消，交接先于删除时不可召回，失败/未知进入可读对账状态。

### 30.3 审核结论

规格已完成一轮全面审核并同步到 PRD、设计、执行计划、实施摘要和验收矩阵。用户于 2026-09-24 采纳四项推荐，上述 4 项合同已冻结，规格阻断解除，可以进入后续功能编码；本轮仍未修改 `app/`、`conf/`、`public/`、业务测试或路由，也未运行真实 Mongo/HTTP/浏览器/邮件/ZIP 验证。

## 31. diff-review 修复：索引、GC、交接与上下文（2026-09-24）

本节补足 §30 的原则级合同，优先于其中“具体状态迁移/索引仍待确定”的概括性表述；仅改任务规格/研究/验收/context manifest，不表示业务已实现。

1. `ShareNote`/`ShareNotebook` 同时含个人和组收件人；旧 `ShareNote` 模型注释只列个人唯一键，实际 `PersistenceIndexModels` 未声明分享索引。现按集合 × 收件人类型冻结四个 partial unique、四个 recipient-leading 查询索引，以及 `HasShareNote` 的独立唯一辅助投影。双/无/非法 recipient、历史重复及旧同名索引冲突在 preflight 报告并阻止就绪；不靠 partial filter 悄悄排除脏数据。详见 PRD PB-01 第 9 条及 design 索引表。
2. notes 的 `workspace_operations` 以 `TerminalAt` 建 30 天终态 TTL，但评论身份删除后若被网络重试会重建，故只借用 30 天**完整记录**保留时长，不复用物理 TTL。publishing 每日最多 500 条、状态/对账/outbox 门禁后压缩并读回，永久保留最小身份/目标/摘要/删除墓碑，容量线性增长必须监控/规划；容量不足报失败，不暗自放松跨请求去重。详见 design“评论凭据保留与 GC”。
3. 旧 outbox 只有 `pending/sending/retry/sent/dead`，worker 领取后立即调用 transport，不能支持删除与发送交接的可确认先后。publishing/admin 共享十态迁移、版本/lease/CAS、取消结果和 `handoff_unknown` 人工对账。Mongo CAS 不能与外部 SMTP 接受原子提交；`TransportHandedOffAt` 是调用前保守不可撤回闸门，**不能**声称实际 SMTP 接受或 exactly-once。取消先赢必须停发；交接先赢不得谎称取消成功；闸门后崩溃禁止盲重发。物理 SMTP 准确接受时刻与 DB 原子一致不可保证，此限制必须随真实故障注入证据移交。
4. publishing 的 implement/check manifest 纳入 admin 三份交接材料和 publishing 决策摘要、动作清单、验收矩阵；admin 的 implement/check manifest 反向纳入 publishing 三份合同及摘要/验收。admin 既有归档 `application-content` 旧路径缺失是独立存量问题，不因本次同步默默删除或修复。`task.py validate` 只证明引用结构，不证明 Mongo/SMTP/HTTP 行为。

MongoDB 官方 Partial Indexes 文档仅列 `$exists:true`（不含 `$exists:false`）和 `$type` 等过滤操作，JSON Schema 文档支持 `oneOf`/`not`/`required`/`bsonType`，`collMod` 支持 strict/error validator。故初版草案的互斥 partial filter 已在本轮修正为 `$type` partial index + strict/error JSON Schema 互斥验证；脏旧记录先 preflight/人工修复，不能用索引遗漏当作兼容。此处是可实施合同与文档依据，仍需真实 Mongo 7/8 建索引/validator 及回滚测试。
