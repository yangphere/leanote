# 应用层：分享、博客与主题 — 技术设计（审核稿）

## 边界和单一事实来源

`app/service` 的 publishing 业务 seam 负责授权、发布状态、组成员有效性、主题状态与评论规则；`app/info` 固定 wire/BSON；`app/db` 只实现 owner-scoped 读写、索引和事务/错误传播。纯应用合同不得引入 Revel/HTTP writer/原始 Mongo collection。现有 `app/service → app/db` 兼容适配可保留，不要求全库 BSON 重写。

本叶已于 2026-09-24 激活。用户同日采纳四项推荐，分享期限 schema、`SetNote2Blog` raw boolean 失败合同、评论提交凭据生命周期和评论 outbox 取消/transport handoff 状态机现已冻结；`in_progress` 仍不是功能完成或运行证据。

HTTP adapter 负责 principal 获取、presence-aware 参数/multipart 绑定、Host、路由方法、旧 `info.Re`/JSONP/模板映射；模板和静态资源只消费已验证投影，不在 adapter 或模板重算授权。`application-notes` 的 mutation receipt/USN 是 publish note 状态转换的唯一来源；通用文件根、`ParseLogicalPath`/`ParseStoredPath`、图片 decoder/store 由 content 复用，不另建 theme 专属可绕过存储。

提供可返回 `(allowed bool, err error)` 的读/写权限 seam，输入至少包含 actor、真实 owner、note/notebook ID 和资源状态。notes 的 edit/copy、content 的附件/图片、contentpdf 的 `MongoPDFRepository.CanReadNote` 必须消费同一规则；其现有内联 ShareNotes/ShareNotebooks/Groups 查询只能视为待迁移重复逻辑，不能在最终验收中留作第二事实来源。对公开博客图片使用同一可见性谓词并再校验 note 确实引用该文件。

## 分享判定与生命周期

 1. principal→解析/验证资源 owner（owner-scoped 查询）→验证权限 `0/1` 与目标用户或组→grant 命令；recipient 退出单独使用 recipient scoped 查询。服务不信任外部传入 `userId` 声称的 owner。先筛除已删除/回收站资源和非法/空 ObjectID，再查询 grant；not found/forbidden/DB error 不转换为允许。共享列表的 note 查询还必须同时带 owner、所属 notebook、`IsTrash=false`、`IsDeleted=false` 条件；查询错误返回 storage error，不能把空结果当作“没有共享内容”。
2. permission seam 每次读取都先基于同一时钟快照排除已到期（`now >= expiresAt`）或失效的 grant，再按资源级别选 note 授权（无有效 note grant 才回落 notebook），同一级别选直接 recipient 授权（无有效直接 grant 才使用 group）；同级所有当前有效组 grant 按 `Perm` 最大值合成，任一 `1` 胜 `0`，只有 `0` 则只读，无有效 grant 则拒绝。过期 grant 不阻断其他独立有效授权，不能让 notebook 的可写或 group 的可写越过更高层的**有效** note/个人只读。列表投影、内容读写、notes/content/PDF 消费相同授权结果，不得仅在某个 controller 检查时间；`HasShareNote` 等辅助记录即便未清理也不能授权或泄露到期内容。禁止按 `Sort("-ToUserId")` 或集合顺序决定，多个冲突组、到期边界、撤销唯一可写组、退组/删组及 DB 错误要有对抗性 fixture。group owner 按已有 `GetMineAndBelongToGroupIds` 含 owner 的规则视为有效成员，即使没有 `GroupUser` 文档；这是本叶冻结的兼容语义，不得要求为 owner 补写伪成员。删除组才终止 owner 成员资格，普通成员删除动作不能移除 owner；权限仍须按当前数据库态重算。
3. 授予/修改用受控 owner/resource/recipient 唯一键实现幂等一致写，不使用旧 delete-then-insert。真正的唯一约束取决于 infrastructure 的 index preflight；缺少索引不能假装竞态已解决。`HasShareNote` 只能从成功 grant 派生，删除最后一个 grant/组离开时同步修复；跨 collection 多写优先同事务，不能使用事务时须有持久 intent、补偿/重试与对账，不以进程内 goroutine 修复。写失败/未知状态返回 error/partial_write，并提供核查原记录是否已应用的证据。
4. 分享 service 返回逐 email 的稳定结果条目，controller 只翻译既有 map/re envelope；个人/组 grant、改权和退出要复用同一个授权 seam。关联资源删除由 notes 触发，权限 port 立即不可读，即便清理仍 pending。
5. 旧 `ShareNote`/`ShareNotebook` 仅存 `CreatedTime`，无法由它推导有效期；新增每条 grant 可选的持久化到期元数据，缺字段的旧记录与未填的新记录一律解释为不过期，无需批量回填或设默认时长。创建入口由分享者按现有 actor/owner 边界设置；期限采用具体日期时间，界面按使用者时区显示与输入，提交时转换为可验证的绝对时刻，服务端统一以 UTC 持久化/比较，不引入相对时长预设。只在显式提交新期限时校验其严格晚于同一服务端时钟快照的当前时刻；等于/早于时拒绝并保持原记录及权限不变，立即停止访问改用撤销分享；仅改权限未附到期参数时不得因旧期限现已过期而误判为新提交。旧版修改权限请求未提供到期参数时必须保留原记录的期限，不得因更新实现为先删后建而抹掉期限。仅 owner 可显式对原 grant 延长/缩短/清除期限，过期 grant 仍可由 owner 明确续期；在新的期限/清除操作成功持久化前不能恢复读取，写入失败/未知结果必须维持拒绝或先核查确认，不能用空值歧义隐式续期。权限由访问时比较决定，无需后台任务或 Mongo TTL index 才能生效；过期数据可留存供 owner 管理/审计，但任何 recipient 列表、读、改、资源下载均不得因记录尚存而授予访问。时间判断可用注入时钟写精准边界测试。
6. HTTP 分享创建/改权与组授权动作采用 presence-aware 输入：`expiresAt` 仅接收秒级 RFC3339 且显式携带 `Z` 或 `±HH:MM`，拒绝空白、小数秒、无偏移及非法日期；后端在写入前解析并统一 UTC，校验 `expiresAt > serverNow`。`expiresAt` 省略时创建不过期、更新保留旧期限；仅更新已有 grant 接受明确 `clearExpiresAt=true` 取消期限，单纯清空输入框、缺字段或错误的清除标志都不能放宽权限。两个字段同传或创建时传清除标志拒绝，无效请求不得部分写入个人/组 grant（含 `emails[]` 批量）。旧个人分享创建保持逐 email `Re` 结果、改权保持 bool，组授权保持 `Re.Ok`；新字段仅作为额外可选输入，不改变旧无字段请求外形。浏览器按使用者时区展示与输入，并标出时区，不能自行默默调整夏令时跳过/重复的本地时刻；选择无歧义的时刻或明确数值偏移才转为 RFC3339 后提交，服务端不接受模糊本地时间。适配器需对真正的字段缺失与显式空/非法值做区别，并由 interface/presentation 在真实请求/页面中冻结错误及显示形状。

7. 上述“创建”以存储唯一键当前不存在为准，不以旧 HTTP action 名称含 Add 为准；已有 grant 被 Add 再次命中时为更新，无 `expiresAt` 则在 owner-scoped 原子更新中保留原期限，不能走 delete-then-insert 或把旧 BSON 缺字段当作重置指令。显式新期限可改期，`clearExpiresAt=true` 仅实际已存在时允许；多人请求中任何 recipient 尚无 grant 且要求清除时，在所有写入前拒绝整批，DB 失败仍按原逐人结果和 partial_write 合同处理。

### 分享索引合同

| 集合 | 索引名 | keys（顺序固定） | 约束 / partial filter |
| --- | --- | --- | --- |
| `share_notes` | `share_notes_owner_note_person_unique` | `UserId, NoteId, ToUserId` | unique；`ToUserId: {$type: "objectId"}` |
| `share_notes` | `share_notes_owner_note_group_unique` | `UserId, NoteId, ToGroupId` | unique；`ToGroupId: {$type: "objectId"}` |
| `share_notebooks` | `share_notebooks_owner_notebook_person_unique` | `UserId, NotebookId, ToUserId` | unique；`ToUserId: {$type: "objectId"}` |
| `share_notebooks` | `share_notebooks_owner_notebook_group_unique` | `UserId, NotebookId, ToGroupId` | unique；`ToGroupId: {$type: "objectId"}` |
| `share_notes` / `share_notebooks` | `share_notes_recipient_person_owner_note` / `share_notebooks_recipient_person_owner_notebook` | `ToUserId, UserId, NoteId` / `ToUserId, UserId, NotebookId` | 非唯一；partial filter `ToUserId: {$type: "objectId"}` |
| `share_notes` / `share_notebooks` | `share_notes_recipient_group_owner_note` / `share_notebooks_recipient_group_owner_notebook` | `ToGroupId, UserId, NoteId` / `ToGroupId, UserId, NotebookId` | 非唯一；partial filter `ToGroupId: {$type: "objectId"}` |
| `has_share_notes` | `has_share_notes_owner_recipient_unique` | `UserId, ToUserId` | unique；两字段 `$type: "objectId"` 的 partial filter，非零由写入校验及 preflight 保证；仅辅助投影 |

Mongo partial filter 支持 `$type`，但不支持 `$exists:false`；它只区分要建索引的 recipient 类型，**不负责互斥**。授权写入和 preflight 均要求 `UserId`、资源 ID、恰好一个 recipient 是非零 BSON ObjectID，另一 recipient 字段**不存在**；`null`、双字段、双缺失、错误类型及零值均是脏数据，不能因不进入 partial index 就静默忽略。完成历史修复后，两 grant 集合启用 `validationLevel: strict`、`validationAction: error` 的 JSON Schema：`UserId`/资源 ID 的 `bsonType: objectId` 和 `required`；`ToUserId`/`ToGroupId` 属性若存在必须为 `objectId`；`oneOf` 两分支分别 `required: [ToUserId]` 且 `not: {required: [ToGroupId]}`，反向同理。零 ObjectID 在应用写入和 preflight 单独拒绝；`HasShareNote` 两字段同样必填、类型/零值校验。上述 `has_share_notes` 唯一索引的 partial filter 要求 `UserId`、`ToUserId` 均为 BSON ObjectID。四个 grant 索引覆盖 owner/resource/recipient 原子 upsert 和 owner 清单，recipient-leading 索引覆盖 recipient + owner/resource 授权查询；组成员集合仍从当前 Group/GroupUser 关系取值，不依赖投影授权。不得建跨两类 recipient 的普通唯一索引或假定旧模型注释已创建数据库索引。

迁移次序：在新写入启用前检查历史两类型重复键、字段互斥/类型/零值、投影重复与现存同名但选项/keys 不兼容索引/validator；输出每集合/索引/冲突类别的数量、脱敏键摘要与少量脱敏样例供人工修复，错误使索引/validator 安装及应用就绪/启动失败，不能删数据、自动选赢家或只为新记录建立绕过旧记录的索引。人工修复需独立授权和备份/回滚计划，修复后重跑 preflight、建索引/validator 并读回实际定义；旧 `ExpiresAt` 缺失/null 不纳入重复身份，仍永久有效。唯一约束、validator 与 recipient 索引在真实 Mongo 上对并发个人/组 × note/notebook、旧空值及中断重启验证。

## 发布、互动与通知

- 发布 read model 只看 owner scoped 的 note 主记录及当前 `IsBlog && !IsTrash && !IsDeleted`；slug/ID、列表/分类/标签/搜索/统计、单页、上传图片匿名引用以及正文的可见性一致。URL/Host binding 属 HTTP，不能只凭 ID 绕过 owner；管理员推荐操作的管理员身份属于 admin，publishing 只执行经授权命令及投影。
- `/note/setNote2Blog` 与通配路由暴露的 `/notebook/setNotebook2Blog` 都只使用 notes 的已有 operation/USN 入口；notebook 传播按每个 child note 产生独立 mutation/receipt/USN 结果，不能共享父 notebook 的一次 USN。publishing 维护 blog tags 与公开 projection，不独立再分配 USN；逐项失败保留内部 result 与明确总体失败，不新增公开列表字段。取消发布或删除先使公共 read 不可见，再补偿派生统计/缓存，无法确认则显式 pending，而非用旧值继续放行。
- 发布后的标签重算、NoteContent 的 `IsBlog` 镜像和其他公开 projection 不得通过请求路径 `go func` 尽力完成。它们必须属于同一确认提交，或写入有稳定 identity 的持久 repair receipt，由可重试/可对账流程推进；未确认时 raw boolean 只能为失败/未知，不能把父记录已写入当成整批成功。
- Q-P10 已确认：`pageSize` 与 `sort` 是公共 query 参数，HTTP adapter/`BlogController` 做 presence-aware 解析，`BlogService` 只接收已验证结构化值；仅在 `pageSize` 缺省时读取 owner `PerPageSize`。缺失、零值或超范围的旧 `PerPageSize` 使用 10 并记录配置问题，显式 query 的零值、越界、非法格式或整数溢出则 400 且不查询。`page` 缺省 1、范围 1..10000；pageSize 范围 1..100。`sort` 仅允许 `PublicTime`、`CreatedTime`、`UpdatedTime`、`Title`；缺省及旧 `SortField` 空值/未知值统一使用博客默认 `PublicTime`，设置写入拒绝未知值，未知字段不得进入 Mongo sort。公共 query 不提供/覆盖排序方向；`isAsc` 只取当前 owner 的 `UserBlog.IsAsc`，缺失旧值按 `false` 为降序，主字段与 `_id` 使用同方向；结果追加同方向 `_id` 稳定排序。
- Q-P10 Host seam 先按部署配置的可信代理 CIDR/IP allowlist 判断 `Request.RemoteAddr`；未命中、无法解析或未配置均不可信并忽略转发头。可信代理才可使用 `Forwarded`/`X-Forwarded-Host`，且每个来源只能有一个有效 Host；多值、逗号链、重复 `Forwarded host=` 或两来源 canonicalize 后冲突均 400，不猜测链路位置。canonicalizer 统一应用于 request/forwarded Host、默认配置域名和存储 custom domain：malformed Host 400，默认域名根域精确匹配，custom domain 完整精确匹配，仅允许单标签默认子域，多级子域拒绝；重复映射由唯一约束/preflight 阻断。非法查询参数 400，未知 Host/冲突 owner 404，DB error（数据库错误）使用既有 500/服务错误 envelope；400/404/500 拒绝路径不执行 DB query、不能返回空成功。博客和预览消费同一 seam。
- 评论/点赞使用已认证 actor，校验 note 公共状态、评论开关、reply 归属及管理员/作者删除边界；存储变更与计数更新要一致，重复/并发验证 toggle 结果。新提交评论/回复在应用入口统一验证合法 UTF-8、非全 Unicode 空白、码点数 `<= 2000` 且 UTF-8 字节数 `<= 8192`；检验空白但不改变原文，超限直接拒绝、不截断，任何记录/计数/通知均不变；旧评论即使超限也不回填/截断。2000 个有效 Unicode 码点最多 8000 字节，8 KiB 是冗余防护，验收不构造不可能的“字节超限但码点不超限”样本。新旧评论正文均是纯文本，HTML 标签作为原样文字保存、展示，不按富文本执行，不对旧 BSON 内容做破坏性重写；JSON/JSONP 沿旧 DTO 输出原文本，浏览器模板/DOM 和通知邮件等 HTML sink 必须在输出边界按上下文转义，不能因预编码出现双重转义。`ToCommendId` 保持现有存储键。旧评论邮件是无结果 goroutine、现有 `DeliverOutbox` 只支持账号事件；Q-P8 已确定用可持久化通知意图替换无结果 goroutine，评论事件与投递模板须由 admin 邮件边界扩充，不得假定目前已支持。
- Q-P8 的写入接缝：由 publishing 判定真实已发布 note 下的收件人（旧行为为非作者顶级评论通知博客 owner，回复通知本 note 的被回复评论作者），验证回复归属，不信任请求提供的收件人；需要通知时，评论、计数和以稳定 comment ID/recipient 构成幂等身份的 outbox 意图须作为同一可确认提交，完成前不让投递 worker 领取。优先复用数据库事务；若环境不支持事务，需有可恢复的 pending intent/可见性闸门与对账，不可仅依靠尽力删除来宣称回滚成功。入队失败或提交未知时不返回成功、不留下可见评论/新增计数或可投递的孤儿事件；若发生物理部分写入，应报告 `partial_write` 并继续对账，不能把未确认结果转为 `Ok:true`。无需通知的评论仅需评论与计数一致。SMTP 失败不改已确认评论/计数，outbox 记录 retry/dead 和脱敏错误以供重试/排查；重试必须防止重复**入队**，不能对邮件 transport 承诺不可证明的 exactly-once 发送。
- Q-P8b 的删除接缝：有权删除评论时必须根据稳定 comment ID 找到其未投递的通知意图，与评论/计数删除形成可确认的停用或可恢复的失败状态；仅删除评论记录不能证明事件已经取消。admin 邮件 worker 在实际交给 transport 前必须与取消操作协调并重查停用状态；领取事件本身不等于开始发送，删除先于邮件交接时 pending/retry/已领取但未交接事件均不得发送，交接先于删除则不可撤回。失败或提交结果未知不能宣称已取消仍可投递的事件；需通过状态读取/对账确认，不得单凭本地 worker 内存状态裁决竞态。现有 outbox 尚无评论事件或取消合同，具体可确认状态转移由 publishing/admin 联审；不承诺已经送入 SMTP 的邮件 exactly-once 或可召回。
- Q-P9 已决：HTTP `CommentPost` 增加**必填** `submissionId`，服务端在副作用前仅接受长度 32 的小写十六进制（128-bit 随机身份）；缺失、显式空或非法值均拒绝，旧无身份客户端不兼容。浏览器每次有意提交以安全随机源生成新值，重试复用原值；外部调用者也必须提供，不能从正文或时间窗口构造。由已认证 actor 和 `submissionId` 构成唯一 operation key，保存经验证的 note ID、reply ID、原始正文摘要和关联 comment ID/状态作为可查询的提交凭据；同键同内容读回同一已确认结果，同键不同内容报冲突，不以客户端身份决定 owner/收件人。评论、计数、凭据与必要 outbox 意图原子确认或经持久 pending/可见性闸门恢复，不能把 comment ID/recipient 的**内部** outbox 幂等冒充跨请求幂等；未知结果 read-back/reconcile 失败须返回未知/partial_write 并保持可恢复，客户端不能更换身份重发。删除后保留不能再次发布的凭据墓碑，原身份重试只返回冲突/已删除状态且不新建或发送；任何收件人投递事件仍用稳定 comment ID/recipient 键。唯一键/历史数据/持久凭据生命周期须经 DB preflight 验证；不支持随意 TTL 清理导致同键旧请求被当新评论。重复响应沿旧 `Re`/JSONP 的合法成功外形返回同一 Item，缺身份/冲突的错误 status/body 由 interface 真实请求冻结，其他互动 action 不强制该字段。
- 预览主题的授权失败必须 fail closed 且无副作用：主题 ID 无效、非当前 principal owner、数据库/文件读取失败时，不得写入或覆盖 session 的 `themeId`，不得改变 active/public 状态，也不得借 Blog fallback 渲染另一主题。只有 owner-scoped 主题成功读回后才建立预览上下文。

### 评论凭据保留与 GC

- publishing 拥有独立 `comment_submission_receipts` 集合，唯一 `(ActorId, SubmissionId)`；状态至少区分 `pending`、`reconciling`/`unknown`、`committed`、`deleted`、`aborted`，后 3 类须经读回/对账确认方可写 `TerminalAt`，删除另写 `DeletedAt`，两者只前进不回退。完整凭据至少保留到 `max(TerminalAt, DeletedAt) + 30 天`；与 notes 的 30 天终态 TTL **仅共享保留时长**，不能复用其会物理删除身份的 TTL 索引。
- 每日由 publishing 的有界维护任务扫描符合条件的记录，单批最多 500 条，以 `(状态, TerminalAt, _id)` 稳定遍历；在版本/CAS、状态未变化、`ReconciledAt` 已确认、所有关联 outbox 均为 `sent` 或 `cancelled`（`dead` 仅经 admin 明确关闭且不可重投才算）、不存在 `handoff_unknown`/可恢复步骤时，将大体积恢复信息压缩为永久最小凭据，不物理删除身份键。最小凭据保留 actor/submission、note/reply/正文 SHA-256、comment ID、终态/删除时间和无正文的原响应核对信息；活跃评论仍能从稳定 comment ID 读回相同成功结果，已删除的旧 ID 永远只回 deleted/conflict，查不到评论/读回异常不能重建。`pending`、未知、未对账、未终态 outbox 与待人工重试 `dead` 永不进入压缩扫描。
- 每条压缩先核对关联 outbox/评论、CAS 更新后读回版本及最小凭据；超时/写入/读回失败保持原记录可恢复，记录脱敏错误并在下轮指数退避重试及报警，不推进扫描游标越过未确认记录。唯一键无 TTL；`(Status, TerminalAt, _id)` 扫描索引与唯一键的行数/字节、索引字节、扫描积压、最老待压缩年龄和容量预测须可观测。永久墓碑使占用随累计提交线性增长，不宣称硬上限；上线前记录部署可用存储配额、预估提交率与保留期容量预测，并设置积压/可用空间告警；无法证明容量预算时部署门禁保持未通过。配额/磁盘不足时显式拒绝提交，绝不通过静默驱逐放松去重。修改物理保留期或终止永久身份须重新取得产品裁决。

### 评论通知取消与 transport 交接

| 状态 | 允许迁移与发送资格 |
| --- | --- |
| `unconfirmed` | 与评论/计数/receipt 一同提交；worker 不可见。共同确认后才进 `pending`，失败走可恢复对账，不单独发布。 |
| `pending` / `retry` | 仅到期且未取消可由版本 CAS 领取为 `claimed`；删除可 CAS 为 `cancelled`。确定的未接受发送错误按现有退避/最多 10 次进 `retry` 或 `dead`。 |
| `claimed` | worker 有 lease/owner/version，但未交接；删除可 CAS 为 `cancelled`，正常 CAS 进 `handoff_pending`；租约过期且未进闸门可安全重新领取。 |
| `handoff_pending` | 预交接闸门仍可 CAS 取消；worker 必须以当前 lease、version、`CancelRequested=false`、`TransportHandedOffAt` 缺失为条件 CAS 进 `handed_off`，CAS 失败必须丢弃本地 payload，绝不调用 transport。 |
| `handed_off` | CAS 成功持久化 `TransportHandedOffAt` 和递增版本，是发送交接的保守线性化点；随后只允许该 lease 调用 transport，不能称 SMTP 已接受。确定接受进 `sent`，确定拒绝进 `retry/dead`，无响应/进程崩溃进 `handoff_unknown`。 |
| `handoff_unknown` | 可能已被 SMTP 接受；禁止自动重发，admin 依据可验证 transport 结果人工对账为 `sent` 或确认未交接后 `cancelled`/受控重试，无法证明则保持 unknown。已删除正文绝不由此状态重发。 |
| `sent` / `cancelled` / `dead` | `sent`/`cancelled` 是投递/取消确定终态；`dead` 是失败终态但人工重新打开前必须重查评论删除与取消请求。终态不可由过期 lease 重新发送。 |

outbox 稳定身份为 `(CommentId, RecipientId)`（已有 `IdempotencyKey` 唯一键承载），额外按 `(Kind, CommentId, Status)` 查找待停用事件；payload 只含必要 recipient 与安全模板数据，不保存未验证的请求收件人或凭据。所有状态迁移 CAS 匹配 `_id`、kind、期望 status/version、有效 lease（worker 路径）与取消标志，并递增版本；删除端先经权限/归属验证，再在同一可恢复提交里对每个未交接事件 CAS `CancelRequested=true`、`Status=cancelled`、`CancelledAt`、原因，且评论不可见/计数/receipt 墓碑一致。CAS 失败时读回：`cancelled` 可确认停发，`handed_off/sent` 只能报告不可撤回，`handoff_unknown` 或读回错误返回明确 unknown/partial_write 并持续对账，不能报告“通知已取消”。worker 不可凭领取时的旧 payload 再送。

Mongo CAS 与 SMTP 接受不原子：`TransportHandedOffAt` 是调用前的**保守不可取消闸门**，不是 SMTP 接受时间；它后面的进程丢失即使实际尚未调用也只能标 unknown，不能自动重试或虚报 sent。成功删除先赢取消 CAS 时不进入闸门；交接闸门先赢，删除可以停用评论但不能承诺召回，且不得把通知已取消报告为成功（旧 bool 只能为失败/未知，内部保留 partial 诊断）。因此“成功删除先于交接”以两者**已确认的持久裁决**为序，发起删除请求的墙钟时间不等于成功取消；闸门已赢但 SMTP 尚未开始时不能虚报取消成功。对“物理 SMTP 已接受的准确先后”不能作跨系统 exactly-once 保证，验收应测定该保守裁决以及闸门/调用间崩溃，不把数据库时间戳冒充外部确认。admin 负责投递与对账 API，publishing 负责删除和 receipt 一致性，两侧共享同一状态合同。
- `UserBlog*` 绑定由 adapter 识别字段 presence，service 保留对跨字段值域的校验；`CanComment=false`/第三方模式时不允许后门直调本站评论服务。URL title 在 owner 范围内唯一；投影读取失败返回错误而非空博客伪 404。

## 主题文件与渲染

- theme identity = `(ownerID, themeID)`；内置三种固定主题是只读来源，普通用户与管理员都可上传、激活、预览或编辑自己的主题，任何导入/激活均不自动公开；管理员主题还须可以用于实际博客渲染和显式公开。激活需使 `Theme.IsActive`、`UserBlog.ThemeId` 和最终主题路径一致，不能只在接口返回 `Ok:true`；现有 ImportTheme 响应不提供 themeId，需通过已有主题列表定位，不能擅自改 wire shape。`Theme.Path` 的既有存储/URL 形状保持，但每次解析前必须校验 owner 及允许的相对前缀/路径组件，再用绑定的根路径解析和 no-follow 文件操作；不能把 DB 字符串与请求文件名直接拼成绝对路径。静态 URL 只能指向已完成提交、归属于博客 owner 的主题文件，不能把未安装/暂存文件当成资源；不把管理员自用主题等同于供其他用户安装的 `IsDefault`。生产用模板错误对匿名者不暴露文件内容/主机路径，授权预览可返回脱敏定位信息。
- 公开与安装沿用旧命令：管理员对自有且验证通过的主题显式切换 `IsDefault`，普通用户不能公开、编辑或导出管理员源主题；现有页面的公开按钮只在管理员“其他主题”区域，不能把新增当前 active 主题按钮当作既有行为。内置三种和管理员已公开的主题可供安装，未公开不得凭 themeId 安装；服务除 `IsDefault` 还应验证实际来源/owner，不能接受伪造记录。安装须再次核验来源并复制文件及 metadata 为安装者自有主题，随后按旧行为自动激活，文件/DB/激活失败不报成功或破坏旧 active。取消公开仅禁止后续安装，先前已复制的用户自有主题继续可用；不可让公开/安装直接对未提交 ZIP 或其他用户的主题树读写。
- ZIP 导入：multipart 的 missing/空/过大先失败；压缩体在接收时限 10 MiB（不先无界加载）；对 ZIP 中全部条目预检路径、类型、重名和 header 声称尺寸，常规文件最多 100、目录和文件总条目最多 200，移除允许的单个主题顶层文件夹后规范化相对路径最多 8 层。解包至不可公开 stage 时再次按实际读取字节流式计数，单常规文件不超过 5 MiB、累计常规文件不超过 20 MiB；声明尺寸可信度不足以替代实时限制，等于预算允许、超出立即拒绝并清理暂存，不能产生 metadata/active/公开半包；不单设压缩比阈值。验证 `theme.json`、JSON 值和实际模板依赖/缺失/循环，成功后以 no-clobber 原子发布/记录 metadata。任何中断先通过持久状态查证，再补偿/清理；不允许遗留静态可访问半包。ZIP 导出仅从当前 owner 主题树读，经相同 no-follow 限界，下载名不能由未经清理的 Theme.Name 直接构造。压缩格式维持 ZIP；content 的 tar.gz 下载器不是 ZIP 解包替代品。
- 单文件模板/图片读写也走同一根路径与权限边界；图片需验证真实 media/解码限额，上传名不能充当存储名。模板 parse/引用检查在提交前执行，metadata 与文件状态一致。active theme 更新 `Theme.IsActive` 和 `UserBlog.ThemeId` 多写须原子或显式恢复，删除活动主题禁止；preview 只能取当前已认证 principal 拥有的主题，可以用未激活主题而不改变 active/公开状态。预览内容与正式博客复用同一发布谓词（有效 owner、`IsBlog=true`、`IsTrash=false`、`IsDeleted=false`）；列表、搜索、标签、分类、文章和单页都不得展示未发布草稿、撤销发布、回收站或已删除内容，即使 principal 是作者本人也不例外。深链中其他 userId/主题 ID 不得改变归属或绕过发布判定，不增加草稿预览入口。
- Q-P7 的最终兼容选择是维持旧主题运行方式而非另建跨域体系：`ImportTheme` 将 ZIP 解至程序根目录下 `public/upload/<Digest3(userId)>/<userId>/themes/<themeId>` 并创建 owner 记录，`Blog.render` 按既有 `/blog`、`/preview` 路由和当前 Host 用对应模板渲染，`/public/*`、`/upload/*` 等旧静态路由仍服务主题资源；存在既有博客域名/自定义域及 `common.js` 对主站的 JSONP 调用，但不额外迁移或强制跨域跳转。保持既有评论/点赞/预览与 Cookie 兼容，由 interface/presentation 做原 URL/响应 replay。模板不允许因重构伪装成另一 owner，也不能把上传 ZIP 未公开可安装误认作静态文件不可读。
- 保留用户可执行 HTML/JS 和同源页面的旧风险：文件放进程序目录不等于可信源码，已登录访问者在同源情况下可执行主题脚本；`theme.html` 的 `Info.Desc|raw` 亦需单独评审。文件路径、owner、越权和写入错误须收敛，不能把这些待修缺陷当作兼容基线；同源脚本跨账户安全**未得到保证**，不能以本叶运行/测试证明已隔离或安全接受。先前 Q-P7 隔离域与旧链接跳转提案由用户“保留原样”覆盖，若未来产品要求消除此风险需单独明确范围，不在本任务隐含更换域名/登录流。

## 规格审核补充（2026-09-24）

以下条款补充并优先于本页较早的概括性描述；它们只完善规格，不表示实现已经存在。

- **授权数据**：`ShareNote`/`ShareNotebook` 使用可选 `ExpiresAt time.Time` / BSON `ExpiresAt`，秒级 UTC DateTime；缺失或 null 永久有效，访问时按 UTC 判定；不使用 TTL 删除。索引 preflight 必须发现历史重复并阻止静默排除/删除，迁移/回滚保持缺字段永久语义。
- **互动资源绑定**：`DeleteComment` 必须校验评论 `NoteId == noteId`；`LikeComment` 必须校验 actor、评论归属、公开谓词和互动开关；评论列表、统计、点赞、阅读数均 fail closed，不得暴露私有/回收站/删除 note；回复目标必须存在并属于同一 note。任何请求提供的 comment/reply/recipient 只作候选输入，不作归属事实。
- **发布批量结果**：`SetNote2Blog` 继续返回 raw boolean；每个 note 的 mutation/USN/receipt 全部确认才为 `true`，任一失败、未知或部分写入为 `false`；逐项结果只进内部诊断及可恢复对账，不改变 wire shape。
- **博客投影**：URL title、摘要以及 `UserBlog.Singles` 的增删改排序必须以单一可恢复写入或显式对账表达；禁止先清空、多写后覆盖、忽略第二写入结果。排序请求必须是现有 single ID 的完整排列，未知、重复、缺失 ID 零写入或返回 `partial_write`。
- **主题 owner/lifecycle**：所有自定义主题的编辑、导出、删除、模板和图片读写均以 `(ownerID, themeID)` 约束，管理员也不能跨 owner；内置主题只读。非 active 删除须清理 metadata、文件树和静态入口，失败/未知须可读回或可恢复，禁止静态孤儿。导出临时文件只能位于 owner 受限根，下载名必须清理，清理失败不可静默。`theme.json` 的 `Name`、`Version`、`Author`、`AuthorUrl` 非字符串必须返回可识别 validation/template 错误，不得 panic。
- **评论凭据**：receipt 独立存储，唯一键 `(ActorId, SubmissionId)`；持久化 note/reply/正文摘要、comment ID、状态、删除墓碑、通知意图 ID 和时间；同键同摘要回放、不同摘要冲突零写入；删除后不重建/不重发。保留期届满且关联 outbox 终态、完成对账后才允许 GC，不使用未经确认的 TTL。
- **评论 outbox**：publishing/admin 必须共同实现 comment/recipient payload、取消请求、交接时间、取消时间、原因和 CAS 版本；领取不等于交接，删除先于交接可取消，交接先于删除不可召回，失败/未知必须可读回并对账。

## 错误与验收接缝

应用结果区分 validation、not_found、permission、conflict、storage/partial_write、template_invalid；明确传播数据库查询异常/文件异常。HTTP 侧对照 `conf/routes` 保持现有 response body 类型、key、方法及 status/redirect；只允许为修复安全缺陷显式评审兼容性差异。

Q-P5 已确认收窄公开 JSONP callback：在 Revel 原控制器和新 HTTP adapter 的共同输入边界完成一次 presence-aware 校验，不只校验新 adapter 而漏掉旧入口；只接受完整匹配的 ASCII 标识符片段 `[A-Za-z_$][A-Za-z0-9_$]*` 或以 `.` 连接的非空片段，总长 1～128 字节，不 trim、解读转义或拼接任意表达式。非法、显式空、超长 callback 在取数或产生副作用前拒绝，错误不可作为可执行 JSONP 返回；JSONP-only action 缺失 callback 也拒绝，`GetComments` **仅在参数缺失时**保持旧 JSON 分支。合法回调保留既有 `callback(json);`、`application/javascript; charset=utf-8` 及各 action 的旧 Item；底层 `JSONPResult` 原样拼接测试是 primitive 证据，不可作为公开入口未校验的理由。具体拒绝的 HTTP status/body 由 interface 联审并做真实请求冻结，旧外部客户端使用表达式或超过预算的长名称会不兼容，不能静默放行。

与下游交接：identity 主体/注册初始分享，notes USN/receipt，content 通用 root/image/archive，admin 扩充评论 outbox 事件/安全模板/SMTP 重试与管理员授权（当前 `DeliverOutbox` 未支持），interface 路由/静态/Host/JSONP 及评论失败映射，presentation 模板资源和 JS，delivery 真 Mongo/HTTP/browser/ZIP/发布证据。逐项责任/未运行状态见 `acceptance/evidence-matrix.md`。

## 回滚和阻断

分享权限、博客公开性、主题路径分别具备独立回滚点。旧数据及 DB/schema 迁移必须保持缺字段永久语义；唯一索引 preflight 发现历史冲突时阻止静默启动。评论 receipt GC 必须晚于保留期、关联 outbox 终态和对账完成。任务已激活且规格合同已确认；`task.py validate` 只能验证材料结构，不能代替功能实现或 Mongo/HTTP/浏览器/邮件运行证据。Q-P5 是经确认的 callback 安全兼容例外，Q-P6 的新预算需明确旧包超限的可见错误及不破坏 active，Q-P7 的兼容决议不构成同源脚本安全证明。
