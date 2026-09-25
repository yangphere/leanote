# 应用层：分享、博客与主题 — 执行计划（实现阶段）

## 前置门禁（任务已激活；用户已批准进入功能实现）

- [x] Q-P1～Q-P8b 产品决策及其研究/验收材料已收敛，完整合同以 `prd.md` PB-01～PB-03 和 AC-PB1～PB7 为准。Q-P1 现采用可选秒级 RFC3339 `expiresAt`、更新时独立 `clearExpiresAt=true`；旧请求省略字段不改变已有期限，显式无效日期/空值/冲突参数拒绝且不改授权，界面按用户时区显示与提交绝对时刻。Q-P2 同级多组取高，Q-P3 预览只显示已发布内容，Q-P4 新旧评论纯文本及新提交限制，Q-P5 callback 安全兼容例外，Q-P6 ZIP 流式安全预算，Q-P7 保留旧主题 URL/Host 和普通用户/管理员自用及管理员公开/其他用户安装，Q-P8 评论/必要通知意图共同确认、Q-P8b 删除先于邮件交接即停发。不实施隔离域或旧链接跳转；保留并显式记录同源脚本风险，不能称隔离已通过。
- [x] Q-P9 用户明确不要求旧评论客户端兼容：`CommentPost` 的评论/回复提交一律强制 `submissionId`，无身份请求在写入前拒绝；新有意提交用新身份，网络重试保留原身份，admin 的 comment ID/recipient outbox 去重与跨 HTTP 请求去重分别验收。合同已同步 PRD/design/动作清单/研究/验收矩阵；接口适配和浏览器负责传递/生成及真实请求负例，不以规划已决当成功能通过。
- [x] 已激活 ready 叶 `09-08-application-publishing`；`task.json.status=in_progress` 只代表任务进入工作流，不代表规格、实现或验收已通过。当前先完成规格审核，保持业务实现和测试不变。
- [x] 分享 grant 的 `ExpiresAt` BSON、四个个人/组 `$type` partial 唯一索引、strict/error 互斥 validator、收件人查询索引、`HasShareNote` 唯一投影索引与历史冲突 preflight 已按 PRD/design 冻结；缺字段永久有效，不使用 TTL。实际安装与 Mongo 验证仍未运行。
- [x] `SetNote2Blog` 的 raw boolean 已冻结为逐项全部确认成功才 `true`，任一失败/未知/部分写入为 `false`，内部 receipt/对账保留逐项诊断；当前实现无条件返回 `true`，不可照抄。
- [x] 发布动作清单已补入通配路由暴露的 `/notebook/setNotebook2Blog`；notebook 传播必须逐个 child note 消费独立 mutation/USN/receipt，不能以父 notebook 一次成功或无结果后台任务代替子项结果。发布后的标签、`NoteContent.IsBlog` 和公开投影须进入同一确认提交或持久 repair receipt。
- [x] 共享列表、统计/列表读取和预览主题失败边界已冻结：查询必须绑定 owner、资源及 `IsTrash=false`/`IsDeleted=false`，DB 错误不得伪装为空；主题 ID 无效、非 owner 或读取失败时不得先写 session `themeId`、改变 active/public 状态或回退渲染另一主题。
- [x] **Q-P10 公共查询与 Host 合同已确认**：`pageSize`/`sort` 是公共 query 参数，`BlogController`/HTTP adapter presence-aware 绑定后交给 `BlogService`；仅缺省 pageSize 时读取 owner `PerPageSize`，缺失/零值/超范围配置回退 10 并记录配置问题，显式 pageSize 非法/零值/越界/溢出 400 且不查询。page 缺省 1、范围 1..10000；sort 只允许 `PublicTime`、`CreatedTime`、`UpdatedTime`、`Title`，缺省及旧 `SortField` 空值/未知值回退 `PublicTime`；公共 query 不提供 `isAsc`，方向只取当前 owner `UserBlog.IsAsc`，缺失旧值按 false 为降序，主字段与 `_id` 同方向。搜索为 trim 后大小写不敏感字面子串，非法 UTF-8/边界超限 400，Mongo `$regex` 必须使用 `regexp.QuoteMeta`。Host 按 `RemoteAddr` 是否命中配置可信代理 CIDR/IP allowlist 决定是否采信 `Forwarded`/`X-Forwarded-Host`；不可信时忽略，可信时多值/逗号链/重复 host/两来源冲突 400。canonicalizer 同时作用于 request/forwarded Host、默认配置域名和存储 custom domain；malformed Host 400、非法参数 400、未知 Host 或 owner 冲突 404、DB error 用既有 500/service envelope，400/404/500 均不查询、不能空成功。
- [x] 评论 receipt 字段/唯一键/状态已冻结；完整记录至少保留确认终态或删除较晚者起 30 天，再由 publishing 按批压缩非身份负载，永久最小凭据维持去重；关联 outbox 非确定终态/对账未完成不能压缩，不设 TTL。GC/容量运行证据仍待实施。
- [x] publishing/admin 共用评论 outbox 状态表、版本 CAS、发送闸门与 `handoff_unknown` 对账合同已冻结；现有五态 outbox 尚未实现，真实 SMTP 接受不可冒充原子事实。
- [ ] 核实 `domain-contracts` 的可消费字段/存储索引、`application-identity` principal、notes mutation/USN、content 的 root/image、admin outbox 和 `interface-http` 的映射责任；依赖未完成的运行证据保持 delegated-unrun，而非凭任务归档状态认定通过。
- [ ] 开始功能编码前重新验证 context manifests：已加入 publishing/admin 双向 handoff 与实施摘要/动作清单/验收矩阵；其余 backend/database/quality 和归档 notes/content 合同仍须按目标包补齐。完整审核日志只按需读取，不注入超长历史材料。
- [x] 已冻结合同已同步 PRD/design/research/acceptance 并完成规格补充；这不代表功能编码、admin 交接实现或运行验收通过。
- [x] Q-P10 已确认并同步全部受影响材料，规格层整体编码门禁解除；这不代表功能已经实现或验收通过。真实依赖、Mongo/HTTP/浏览器/邮件/ZIP 证据仍须按矩阵实际运行，不能由静态规格替代。

## 批次一：分享和群组

- [ ] 对照 `research/action-contract-inventory.md` 建 Share/Group 授权动作及 owner/recipient/membership/资源状态矩阵；先写非法/越权、同级多组 `0+1→1` 与 `0+0→0`、个人/组及 note/notebook 优先级、唯一可写组退出或撤销后的降级、分享者创建时逐条可选期限/旧记录缺失和未填时永久有效/旧版改权限保留期限、仅 owner 显式延长/缩短/取消和已过期 grant 续期、按使用者时区呈现绝对日期时间且 UTC 比较、显式提交新期限等于/早于服务端当前时刻拒绝、写失败不恢复访问、时钟精确到期边界与已有其他有效授权、DB 错误/多写失败与并发回归，再实现唯一权限 seam 和可选到期字段持久化；共享列表和 notes/content/PDF 需在访问时复用判定，不以定时清理替代。按 AC-PB1-DATE 覆盖秒级 RFC3339 带偏移解析、错日期/空值/无偏移/小数秒拒绝、缺省与显式清除互斥、多人 email 与组授权、界面时区/夏令时、旧客户端不带参数及真实 HTTP 错误外形。
- [ ] 在 notes/content/image/PDF 现有调用边界消费同一 seam；删除复制的权限判定；为草稿/回收站、组删除与 DB error 做各 consumer 等价测试。
- [ ] 逐 email map 和 owner/member 退出外形由 adapter 翻译；其方法、模板渲染与真实请求留 interface/delivery 验证。索引缺失/历史重复需在 Mongo preflight 失败，不能假设 comment 中的唯一声明已落地。
- [ ] 针对同一个人/组 × note/notebook 的重复 Add 增加已设期限（含已过期）的无字段请求 fixture：只可改权限、不得重置期限；不存在 grant 的无字段 Add 才创建无期限。显式新期限只由 owner 改期，清除仅命中已有 grant；多人 email 混合已有与新 recipient 且携带清除时整批预检拒绝，无任何人被修改。

## 批次二：博客业务

- [ ] 发布/取消发布及 notebook 传播对接 notes receipt/USN；覆盖 `/note/setNote2Blog` 与 `/notebook/setNotebook2Blog`，对 notebook 本身和每个 child note 保存独立 mutation/receipt/USN 结果，逐项失败、未知或部分写入均不得返回 raw `true`。标签、`NoteContent.IsBlog`、摘要和公开投影必须由同一确认提交或稳定 identity 的 repair receipt 推进，禁止请求路径无结果 `go func`；列表/详情/单页/搜索/域名的 ID/slug owner/published 状态矩阵按已确认 Q-P10 contract 实现。
- [ ] 发布传播完成后做 projection repair/read-back：失败或未知状态必须可恢复、可对账并阻止未确认投影被公开读取；不能以父 notebook 已写入、raw boolean 或旧投影值冒充整批成功。
- [x] Q-P10 contract 已冻结；实现搜索/列表/分类/归档/域名时必须使用统一有界输入和 Host canonicalization seam，搜索词先转义为字面匹配，sort 只接受 allowlist，Host 映射 owner-scoped，重复映射/解析错误 fail closed，并按已确认的 400/404/服务错误外形补服务、HTTP 和页面 replay。
- [ ] 用户博客设置/主题关联采用 presence-aware 输入；评论、回复、点赞、计数、管理员推荐入口及 Q-P8 确认的通知意图各自覆盖认证、权限、失败；评论新旧正文中的 HTML 标签须在页面/预览/邮件按纯文本显示，JSON/JSONP 不做预转义、输出端按上下文转义，补 XSS 和双重编码回归；新增评论及回复须覆盖空字符串、全 Unicode 空白、非法 UTF-8、合法 2000/超限 2001 码点与 8 KiB 限额，不截断且被拒请求不写入、不更新计数、不投递通知，旧超限记录读取不丢失。`BlogService.Report` 当前无公开 caller，不新增举报 URL；发送 transport 由 admin 邮件边界负责，不误用目前只支持账号事件的 outbox。
- [ ] Q-P8 的评论提交矩阵：有通知对象时确保评论/计数/稳定身份 outbox 意图一同确认，无对象时仅评论/计数同成败；测试评论插入失败、计数失败、入队失败、事务不支持、事务未知提交/可恢复 pending、并发和重复请求，断言未确认不返回成功、不留下可见评论/新增计数或可投递孤儿事件。联验 admin 扩充 `DeliverOutbox` 的评论 kind、去重入队、收件人/纯文本安全模板、发送失败 retry/dead 及脱敏错误；邮件 transport 失败不删除已提交评论，不能保证邮件 exactly-once，旧无结果 goroutine 不得留在请求路径。
- [ ] 先写 `submissionId` 缺失/空白/非法格式拒绝、相同 actor+身份但 note/reply/正文变化冲突、不同 actor 相同身份隔离、删除后原身份重试不重建的服务回归；再以“提交已成功但 HTTP 响应丢失→原身份重发”和“有意重复发完全同文→不同身份”的对照 fixture 覆盖有/无收件人、并发同键、未知提交、读回错误及 comment/计数/outbox/凭据的共同确认。interface/presentation 应在真实 JSON/JSONP 请求中冻结缺身份/冲突错误形状并传递安全随机身份；admin 验证同一 comment ID/recipient 去重，不能拿内部出站重试代替 AC-PB4-RETRY。
- [ ] Q-P8b 删除矩阵：成功删除评论与计数时停用同 comment ID 的待发送通知；覆盖 pending/retry、worker 已领取但尚未交接邮件服务，以及交接前删除/交接后删除、重复删除、并发领取与取消。停用失败或提交未知不得回报“通知已取消”，须可读回/对账并阻止已删除正文在后续发信中泄露；与 admin 联验 outbox 取消及终态可观测，日志脱敏；已交接或成功送出的通知不声称可撤回。现有 outbox 无取消合同，设计阶段不能把领取事件当作已发送。
- [ ] 核对 `09-08-application-admin` PRD/design/implement 已接收评论事件、取消与 transport 交接；完成其 comment kind/worker/取消状态及 publishing 联验的 AC-PB4-NOTIFY/AC-PB4-DELETE 证据前，不将发布评论通知列为已完成，不能用 feedback outbox 测试代替。
- [ ] 对比现有 HTML/JSON/JSONP/模板 key 和 `ToCommendId` 字段；按已决 Q-P5 将公开 callback 在 Revel 控制器/新 HTTP adapter 的共用输入边界校验：只接收 1～128 字节完整匹配的 ASCII 标识符或非空点段路径，拒绝表达式 `cb(1)`、显式空、空点段、超长、换行及其他脚本语法；JSONP-only action 省略 callback 时拒绝，`GetComments` 仅省略 callback 时沿旧 JSON 分支。非法值在业务读/写前阻断且不得生成可执行 JSONP；合法回调的 JSONP envelope、Content-Type、各 Item 及 renderer 原样 primitive 测试保留。与 interface 联审非法响应 status/body，真实 HTTP replay 覆盖读/写 action 的合法、缺失/空、注入和边界长度，不能以服务测试替代。

## 批次三：主题与预览

- [ ] 为三个内置、至少一款管理员上传主题及一款普通用户自有 HTML/JS 主题创建 owner、root/no-follow、multipart、ZIP、循环/缺失/非法模板、并发激活、文件/metadata 部分写入的聚焦 fixture；两种角色都要验证导入→自行激活→在原博客 URL/Host 实际渲染且不自动列入可安装主题、失败不改变旧 active，记录 Windows/Linux 路径差异。
- [ ] Q-P6 已决预算用内置三主题兼容包与用户主题 ZIP 做等于上限允许、上限加一拒绝的独立测试：压缩包 10 MiB、常规文件 100、单常规文件实际展开 5 MiB、累计常规文件展开 20 MiB、文件加目录条目 200、可选单顶层文件夹后路径 8 层；另外验证伪造 header 尺寸、空目录洪泛、高压缩比合法文本、非法链接/越界、途中中断与旧已安装主题仍可用。ZIP 预检元数据并对实际解压流逐字节累计，超限立即终止并清理暂存；既有 active、metadata 与公开列表均不改变，失败错误须可观察，不以解压后统计或压缩比阈值替代。
- [ ] 聚焦旧公开→安装合同：管理员用既有主题管理操作切换自有主题的公开状态，当前 active 主题公开按钮不是旧 UI 能力，不强增新入口；非管理员不可公开，未公开/已取消公开/伪造 ID/其他账户主题不可安装。公开列表只含内置来源和管理员有效公开来源；安装时重查来源权限/完整性，复制出安装者独立副本并按旧行为自动激活；模拟公开状态竞争、复制失败、DB/激活失败，断言无假成功、不损坏旧 active；取消公开后已有安装副本继续可用。
- [ ] 复用 content path/image/store，独立实现受限 ZIP 格式 adapter（不得把 tar.gz primitive 当作 ZIP）；输入包大小和条目元数据在边界预检，实际解包时仍按 Q-P6 限额流式计数；输入包隔离暂存、验证后发布，失败核对/清理；不新增主题专用域名或把原本可执行的 HTML/JS 当作已隔离的可信内容。
- [ ] MemberBlog 页面仅绑定并映射服务结果，保持用户自有上传/激活、管理员在其他主题上公开、普通用户选公开主题安装的原交互；`Info.Desc|raw` 的已知脚本风险单独记录，若改变其展示须显式评审兼容。预览只使用当前已认证 principal 的主题，可选未激活主题但不改变 active/公开状态；主题 ID 无效、非 owner、DB/文件读取失败时 fail closed，且不得先写 session `themeId`、覆盖已有 session、改变 active/public 状态或回退到另一主题。列表/搜索/详情/单页复用正式博客发布谓词并按已确认 Q-P10 contract 处理 query/Host，匿名访问、他人 themeId/URL owner、未发布草稿、撤销发布及删除/回收站内容全部验收为不可见。真实静态资源、浏览器与发布动作仍由后续任务验收。
- [ ] 向 interface/presentation/delivery 交接原 `/blog`/`/preview`、`/public/*`/`/upload/*`、Host/自定义域与主站 JSONP/登录互动合同；真实 HTTP/页面 replay 应保持旧入口和响应，不新增隔离跳转。记录同源 HTML/JS 与主题元数据 raw 展示的风险、受影响的登录访问者及未验证安全状态，不以兼容通过等同安全通过。

## 规格审核补充（2026-09-24）

- [ ] **授权 schema**：实现 `ShareNote`/`ShareNotebook` 的可选 `ExpiresAt time.Time` / BSON `ExpiresAt`，秒级 UTC；缺字段永久有效；访问控制不依赖 TTL。按 design 四个 `$type` partial unique、四个 recipient-leading、一个 `HasShareNote` 唯一索引及 strict/error JSON Schema 互斥 validator 实施；先运行 preflight，历史重复/非法 recipient/旧同名冲突均报告且阻止静默启动，修复后读回索引/validator 定义；迁移/回滚不得改变旧记录语义。
- [ ] **博客资源绑定**：补充 `DeleteComment` 的 `comment.NoteId == noteId`、`LikeComment` 的 actor/note/公开谓词、公开列表/统计/点赞/阅读数 fail-closed、回复同 note 存在性检查；请求参数不得成为归属事实。
- [ ] **发布与投影**：`SetNote2Blog` 保持 raw boolean，所有 mutation/USN/receipt 确认成功才 true，任一失败/未知/部分写入 false；内部保留逐项诊断和对账。对 URL title、摘要和单页排序建立零伪成功合同；排序只接受已有 single ID 的完整排列，未知/重复/缺失 ID 及第二写入失败必须零写入或显式 `partial_write`。
- [ ] **主题生命周期**：所有自定义主题的编辑/导出/删除只允许 owner，包括管理员；内置只读。非 active 删除须清理 metadata、文件树和静态入口；导出临时根与下载名必须 owner-scoped；`theme.json` 元数据类型错误返回 validation/template 错误，不得 panic；失败/未知需读回或补偿，不能留下静态孤儿。
- [ ] **评论提交凭据**：实现独立 receipt 唯一键 `(ActorId, SubmissionId)`，同键回放、不同目标/摘要冲突零写入；删除后永久最小墓碑。终态/删除较晚者起满 30 天且关联 outbox 确定终态、对账完成后，publishing 每日最多 500 条 CAS 压缩并读回；失败重试/积压告警，永久身份索引增长监控与容量不足显式失败；不建物理 TTL。
- [ ] **评论 outbox**：与 admin 按设计状态表实现 comment/recipient 去重、取消/交接元数据和 `_id/status/version/lease/CancelRequested` CAS；覆盖各状态非法迁移、领取未交接取消、闸门后崩溃/SMTP 未知不自动重发、旧事件兼容。删除先赢 CAS 停发，交接先赢不承诺召回；失败/未知读回对账，不得虚报已停发或已投递。

本节是实现合同与证据清单。四项规格合同及 Q-P10 已于 2026-09-24 由用户确认，用户已批准进入功能实现；业务代码必须按上述合同实现并补足真实依赖证据。本轮允许修改 `app/`、`conf/`、`public/` 和业务测试，但不得把静态规格或任务状态当作运行验收证据。

## 验证与回滚

- 针对性 Go 测试各批次 `go test ./app/service/... ./app/application/... -run '<相关用例>' -count=1 -timeout 60s`；Mongo fixture 按已有隔离环境单独运行，不向默认 60s 单测混入服务启动。
- 完工再做 `gofmt`、`go vet ./...`、`go test ./...`、`go build ./...`、`npm test`、`git diff --check`、`python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-publishing`；需要 Mongo/浏览器/真实 HTTP 的证据记录发现数、执行数、环境与失败，不可标成本叶已运行。
- 回滚分为授权、公共投影、主题文件/metadata 三个边界；任一部分失败不得用旧代码双栈或静默 fallback。下游 interface/delivery 验收前不删旧 URL/模板资源。

## 本轮实现修复记录（2026-09-24）

- [x] 主题 ID、owner、内置/active 状态和主题根路径统一 fail closed；模板读取、写入、删除、预览路径拒绝 traversal、绝对路径和越界 symlink。
- [x] 主题删除改为 owner-scoped 的暂存移除并在 metadata 删除失败时恢复；成功删除清理 metadata、文件树和静态入口。主题导出仅允许 owner，使用 owner-scoped 临时目录和清理后的下载名。
- [x] 首次主题创建、复制/安装及导入失败清理完整目录；`CopyDir` 传播目录读取/复制错误，拒绝 symlink，并修复目标文件截断与文件句柄关闭。
- [x] MemberBlog 的模板/图片/ZIP 入口拒绝空 multipart，图片列表/删除/上传和主题导入不再拼接 raw filename；模板读取失败不再固定返回成功。
- [x] 聚焦 Go 回归、服务/控制器/工具包测试、`go vet ./...`、`go build ./...`、`npm test` 和 task context validate 已运行并通过；全仓 `go test ./...` 的 harness 仍因本机 Mongo/基线服务未启动在 60 秒等待超时，真实 Mongo/HTTP/浏览器/邮件/ZIP 流式验收仍未运行。

## 2026-09-25 继续执行记录

- [x] 评论原身份回放先核对持久 receipt 与已提交 workspace 操作，避免文章下架后把原成功结果误判失败，或把中间态误报成功；pending receipt 恢复仍需真实 Mongo 故障注入。回复通知只选择被回复者，顶级评论只选择文章 owner。
- [x] 评论 outbox 的已确认意图状态由共享判断覆盖 worker 领取、交接、重试与终态；删除验证不再将可人工重试的 `dead` 误判为已取消。内存实现的未知交接错误改为固定脱敏代码。
- [x] Host 规范化收敛到 `app/domain/blog_host.go`，数据库预检按规范化域名识别旧值与冲突，输出数量和脱敏摘要；可信代理也先验证原始 `Request.Host`，重复子域名归属查询不按数据库顺序挑选 owner。真实 Mongo 的索引/validator/预检仍未验证。
- [x] 聚焦 Go 包测试、vet、build、`npm test`、两项 task validate 和 diff check 已通过；新增 Mongo 用例经 `-v` 确认跳过。环境与执行数量见 `acceptance/evidence-matrix.md`。
- [ ] 解决 standalone 评论提交中间态可见、确定拒绝与未知交接分流及人工对账、receipt GC/容量监控；补齐真实 Mongo/SMTP/HTTP/浏览器/ZIP/跨进程证据后再做合并审查。当前不提交或归档。
