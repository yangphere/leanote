# 应用层：管理与运维业务 — PRD

## Goal

收敛管理端、配置、升级、邮件和运维审计业务，明确管理员权限、失败边界和敏感信息处理。

## Scope

`ConfigService.go`、`UpgradeService.go`、`EmailService.go`、Suggestion/feedback 提交与邮件 outbox、admin controllers、管理模板和相关日志/配置适配。Suggestion/feedback 是 `application-notes` 明确交接的接收项，不再留在 notes 中作为无 owner 能力；评论通知事件与取消/交接协议是 `application-publishing` 的交接项，不在 admin 重新判定评论或回复业务权限。

## Requirements

- Admin 权限在应用服务和 HTTP 前置钩子均有明确边界，未授权请求不得执行副作用。
- 生产启动配置遵循 `interface-http` 的唯一 production-config seam，只接受显式来源，空/default secret 和不一致的 Mongo 配置必须 fail closed；本任务不复制另一套解析规则。
- `mongodump`、`mongorestore` 和 PDF 可执行文件只能来自显式 allowlist/经过校验的绝对路径。admin 仅负责保存、校验并向 `application-content` 提供 PDF executable descriptor/allow policy；PDF 进程的 argv、timeout、sandbox、输入输出和 cleanup 由 content renderer owner 统一执行，admin 不得启动 PDF 进程或实现第二套 renderer。禁止把配置拼入 shell；MongoDB 用户名/密码及 legacy PDF callback secret/token 不得进入 argv、命令日志或错误输出。
- 升级、邮件和数据维护操作记录可定位错误，不吞异常、不打印凭据。
- 管理页面与 API 的状态码、错误文本和重定向保持兼容。
- 管理业务设置通过 ConfigService 单一 seam；生产启动配置规则统一由 `interface-http` production-config seam 提供，后续消息解析器替换另记 MOD-003。
- Suggestion/feedback 必须显式定义 anonymous policy、`Addr`/`Suggestion` 的 missing/blank/长度/HTML 边界、重复提交 identity 和 durable success。持久化成功与邮件 transport 分离：邮件经 outbox 投递，transport 失败不回滚已提交 feedback，请求路径禁止无结果 goroutine。
- 接收 publishing 已确认的评论通知意图，扩充现有 outbox 的 comment 事件、稳定 comment ID/recipient 身份、状态读取与取消接口、安全 HTML 邮件模板以及 retry/dead/脱敏错误记录；状态字段（取消请求、transport handoff 时间、取消时间/原因、CAS 版本）属于 outbox 元数据，不放进不可裁决的 payload。不得把现有仅支持账号事件的 `DeliverOutbox` 当成评论投递能力。publishing 决定已发布 note 下的合法收件人、评论/计数/通知意图的共同提交与删除权限；admin 不重复查询评论权限或发起无结果 goroutine。未确认提交的意图不可被 worker 领取或发送，邮件 transport 失败不回滚已确认评论。具体状态及允许迁移以 publishing `design.md`“评论通知取消与 transport 交接”为共享合同。
- 已确认的评论删除取消先赢持久交接闸门时，pending、retry 以及已领取未交接的关联事件必须停止发送；闸门先赢或已发送则不承诺召回，不得把通知已取消报告为成功。取消与交接使用持久 CAS 版本裁决，不能以请求开始时间、领取或一次性状态读取替代；取消失败/未知提交需可读回与对账，防止已删除正文从未交接/未知事件继续发送。闸门是 SMTP 调用前的保守裁决，不声称与外部实际接受时间原子一致；不得破坏现有账号事件投递，feedback 规划中的 outbox 投递也须独立验收。

## Acceptance criteria

- [ ] Admin service/adapter 的登录、权限、用户/数据/设置/升级/邮件主要流程有回归；真实页面和启动 smoke 由 interface/delivery 任务验收。
- [ ] ConfigService 只消费 `interface-http` production-config contract，不重复解析来源、secret 或 Mongo 配置；consumer mapping fixture 通过。
- [ ] 邮件/升级失败可观测且不产生虚假的成功响应。
- [ ] 敏感值不出现在日志、summary、制品或测试 fixture。
- [ ] 管理服务和 adapter 的 mongodump/mongorestore allowlist、argv 参数化、超时、错误和敏感日志回归通过；PDF 仅验证 descriptor/allow-policy 输出与 callback secret/token 不出现在 descriptor、argv、环境快照、日志或错误输出，renderer 的 argv、sandbox、timeout、artifact 和 cleanup 由 `application-content` 验收；管理页面 smoke 由 interface/delivery 任务验收。
- [ ] Suggestion/feedback 的匿名/登录策略、输入边界、重复提交、持久化成功、outbox 投递、transport 失败和重试均有 service/DB/HTTP contract；无结果 goroutine 扫描为 0，且邮件失败不把已持久化 feedback 映射为失败。
- [ ] 与 publishing 联验评论事件入队、去重、已确认才可领取、收件人及正文安全模板；SMTP 失败 retry/dead 可观测、日志脱敏且不撤销评论，现有账号事件与本任务新增 feedback 投递分别回归；真实 Mongo/SMTP 故障注入由 delivery 补充，不把未运行记为通过。
- [ ] 与 publishing 联验取消/交接状态：删除先于交接时 pending/retry/已领取未交接均不发送，交接先于删除不承诺召回；重复删除、领取/取消并发、停用失败和未知提交均可读回/对账，不声称已取消仍可能发送的通知；publishing 的 AC-PB4-NOTIFY/AC-PB4-DELETE 未经双方证据通过不得标为已完成。
- [ ] 评论 outbox 覆盖 `unconfirmed/pending/retry/claimed/handoff_pending/handed_off/sent/cancelled/dead/handoff_unknown` 的准入、合法迁移及非法迁移拒绝；`TransportHandedOffAt` 仅表示持久发送闸门，不代表 SMTP 接受。取消 CAS 输给交接时不能返回“已取消”；交接后崩溃/SMTP 结果不明不得自动重发，删除正文不能在未知状态补发。真实 Mongo/SMTP 故障注入和受控人工对账均留证。

## Out of scope

不增加管理功能、不存储生产凭据、不替换消息配置解析器、不自动部署生产。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
