# Publishing 实施决策摘要（2026-09-24 规格审核与实现前门禁版）

当前状态：ready 叶已于 2026-09-24 激活；用户同日采纳四项推荐，编码所需规格合同已冻结。`in_progress` 不代表功能实现或验收通过。本摘要只供上下文注入，真实 Mongo/HTTP/浏览器/邮件/ZIP 证据继续保持 `unrun`/`delegated-unrun`。


## 已确认实现合同（2026-09-24)

- **PB-SCHEMA-01**：`ShareNote`/`ShareNotebook` 使用可选 `ExpiresAt time.Time` / BSON `ExpiresAt`，秒级 UTC DateTime；缺字段永久有效，不使用 TTL。按 note/notebook × 个人/组分别建四个 `$type` partial unique（owner/resource/recipient）、四个 recipient-leading 查询索引，strict/error JSON Schema validator 强制互斥，`HasShareNote` 独立 owner/recipient unique 只作投影；互斥/类型/零值/重复/旧索引冲突 preflight 报告并阻止就绪，见 PRD PB-01 第 9 条与 design 索引表。
- **PB-BLOG-01**：`SetNote2Blog` 保持 raw boolean；全成功 true，任一失败/未知/部分写入 false；逐项诊断进入内部 receipt/对账，不改变 wire shape。
- **PB-RECEIPT-01**：独立 receipt 唯一 `(ActorId, SubmissionId)`；完整保留终态/较晚删除起至少 30 天，随后 publishing 每日最多 500 条按状态/outbox/对账准入 CAS 压缩并读回，不物理删除永久最小去重凭据。未终态/unknown/可重投 dead 不清理，索引增长线性须监控/规划，容量不足显式失败；不能复用 notes 的物理 TTL。
- **PB-OUTBOX-01**：稳定 comment/recipient 去重，状态 `unconfirmed/pending/retry/claimed/handoff_pending/handed_off/sent/cancelled/dead/handoff_unknown`；取消与 handoff 用版本/lease/CAS 决先后。`TransportHandedOffAt` 是调用前保守闸门而非 SMTP 接受，闸门后未知禁止自动重发；状态字段在 outbox 元数据而非 payload，见 publishing/admin design。

这些合同已确认，可以进入实现阶段；但不能把静态规格或任务状态当作 AC-PB4-NOTIFY/DELETE 或其他跨层功能证据。

本页供实施/检查上下文注入；产品规则与验收的**唯一权威**是同任务 `prd.md` PB-01～PB-03、AC-PB1～AC-PB7。逐 action 输入/输出及失败边界见 `research/action-contract-inventory.md`，完整现场证据、历史提案及被覆盖决定见 `research/spec-audit-2026-09-23.md`，未运行/移交矩阵见 `acceptance/evidence-matrix.md`。不要把规划文档当成功能运行证据。

| 决策 | 实施时不得丢失的边界 | 权威位置 |
| --- | --- | --- |
| Q-P1、Q-P2 | 分享 grant 每次访问时按 `now >= expiresAt` 失效；无字段旧记录与无期限新记录不过期；同级多组取最大 `Perm`，不越过 note 或个人授权优先级。可选秒级带偏移 RFC3339 `expiresAt`、仅已有授权更新时独立 `clearExpiresAt=true`；首次创建省略时无限期，已有 grant 的重复 Add 省略时须保留原期限，不能借旧客户端再次提交重置期限；空白、无偏移、小数秒、非法日期/不晚于服务端当前时刻、清除冲突拒绝且不改旧值。UTC 存储比较，界面按使用者时区显示/输入，夏令时模糊时刻不静默改写。 | PB-01、AC-PB1、AC-PB1-DATE；审核 §11～§16、§25～§26、§28 |
| Q-P3、Q-P7 | 普通用户和管理员均可上传并自用合法 HTML/JS 主题；只有管理员显式公开自有主题才可供他人复制安装，预览自有未激活主题仅显示已发布内容；保留重构前同源文件/URL/Host，不新增隔离域或强制跳转。 | PB-03、AC-PB5、AC-PB6；审核 §7～§10、§17 |
| Q-P4、Q-P5 | 新旧评论均纯文本展示；新提交合法 UTF-8、非全 Unicode 空白，<=2000 码点、<=8192 字节，超限不截断；公开 JSONP callback 仅接受 1～128 字节的标识符/点式路径，非法输入在互动副作用前拒绝。 | PB-02、AC-PB4、AC-PB6；审核 §18～§20 |
| Q-P6 | ZIP 压缩包 <=10 MiB、常规文件 <=100、单文件实际展开 <=5 MiB、累计 <=20 MiB、总条目 <=200、规范化路径深度 <=8；流式预算与暂存清理，不单设压缩比。 | PB-03、AC-PB5-ZIP；审核 §21～§22 |
| Q-P8、Q-P8b | 需通知时评论/计数/持久意图共同可确认提交；入队失败不留成功评论或新增计数，SMTP 确定拒绝可重试而不撤销评论。成功取消先赢发送交接 CAS 时，已领取未交接也停发；闸门先赢不可声称取消，交接结果不明禁止自动重发。`TransportHandedOffAt` 不代表 SMTP 已接受；现有 `DeliverOutbox` 尚无 comment/cancel 合同，须联动 admin。 | PB-02、AC-PB4、AC-PB4-DELETE/AC-PB4-HANDOFF；审核 §23～§24、§31 |
| Q-P9 已决 | 评论/回复提交强制 `submissionId`（128-bit 随机值、32 位小写十六进制）；缺失/非法的旧客户端请求拒绝。actor+身份绑定 note/reply/原文摘要及 comment 凭据，同次重试只回放已确认结果；同身份改正文/目标冲突，不同身份的同文可各自提交；删除后旧身份不重建/重发。comment ID/recipient 的内部 outbox 去重仍独立验收。 | PB-02、AC-PB4-RETRY；审核 §28～§29 |

上下游：domain-contracts 已归档；notes/content 提供 mutation/USN/permission/安全文件 primitive；admin 任务规格现已接收评论事件/取消与邮件交接的投递责任，仍须与 publishing 联验；interface 负责强制绑定 `submissionId` 和无身份错误，presentation 两处评论入口生成/复用安全随机身份，delivery 验证真实 HTTP/断线重试。上述及其他真实环境证据未运行保持 `unrun`/`delegated-unrun`。`.trellis/spec/backend/index.md` 的“Revel runtime removed”是**目标态描述**；当前 `go.mod`、`sh/run.sh` 仍保留旧入口，第一方 `cmd/leanote` 也存在，彻底去 Revel 属 `09-08-interface-http`。主题同源 HTML/JS 可能以访问者身份请求主站：兼容原行为不等于提供脚本隔离，交付证据须显式注明风险。Q-P9 业务规则已决但未实现；任务已激活，规格补充完成前不得进入功能编码。
