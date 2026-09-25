# application-admin action contract inventory

审核日期：2026-09-25。此清单用于实现和验收范围；通配路由的最终 HTTP method/status/header replay 由 `interface-http` 负责。本清单不把现有 controller 调用当作真实 HTTP 证据。

| Action/入口 | 当前绑定与输入 | 当前输出/副作用 | 实现时必须冻结的合同 | owner |
| --- | --- | --- | --- | --- |
| `Index.Suggestion` / `/suggestion` | `Addr`、`Suggestion`、可选 32 位小写十六进制 `submissionId`；旧前端只发送 `Suggestion`；公开 action，可匿名 | `info.Re` JSON；当前仅写 `suggestions`，并由请求路径无结果 goroutine 触发邮件 | `Addr` 空值允许，非空单个 addr-spec/254 字节；`Suggestion` 合法 UTF-8、trim 后非空、2000 码点/8192 字节；登录 actor 可建立 receipt；匿名携带 ID 直接 validation 拒绝，匿名无 ID 仍可提交但 `RetrySafe=false`、不参与唯一索引；feedback 文档、receipt、outbox 三者逐项 strict read-back，receipt 保存 digest/recipient snapshot/outbox IDs，冲突零写入；`feedbackRecipients` 固定内部列表，durable enqueue 后保留旧 `Re` | admin + interface |
| `Admin.Index/T/GetView` | view/template selector `t/view` | HTML template；当前可能投影全局字符串配置 | admin principal 先于模板；模板 selector allowlist；secret redaction；未知模板失败，不把配置错误当空页面 | admin |
| `AdminUser.Index` | sorter、keywords、pageSize | 用户列表 HTML/分页 | sorter allowlist、分页边界、DB error、敏感字段不出页面；真实页面由 interface/delivery | admin |
| `AdminUser.Register` | email、pwd | `Re`；创建用户及身份初始化/邮件副作用 | identity-owned principal/注册事务和 outbox；admin 只做授权/adapter 回归，不复制注册规则 | identity + admin |
| `AdminUser.ResetPwd/DoResetPwd` | userId、pwd | HTML/`Re`；修改其他用户密码 | 目标 userId 由 admin principal 授权；密码/token 不进日志；identity contract 错误原样分类 | identity + admin |
| `AdminSetting.DoBlogTag` | recommendTags、newTags | `Re`；两键配置更新 | 两键全量预检、逐键结果、失败不覆盖缓存；不以最后一次 `re.Ok` 覆盖先错 | admin |
| `AdminSetting.ShareNote` | shared user、权限数组、note/notebook IDs | `Re`；共享配置更新 | consume notes/publishing ownership contracts；输入长度/ID/perm 校验；逐字段 read-back | admin + notes/publishing |
| `AdminSetting.DoDemo` | demoUsername、demoPassword | `Re`；demo config 写入 | identity 的 `demoUserId` 唯一事实来源；密码不得通用投影/日志；失败不保存半套配置 | identity + admin |
| `AdminSetting.ExportPdf` | executable path | `Re`；更新 PDF config | 只返回 typed descriptor/allow-policy；content 独占 renderer | admin + content |
| `AdminSetting.DoSiteUrl/DoSubDomain/OpenRegister/HomePage/UploadSize` | 字符串、数组、float | `Re`；全局配置更新 | normalize、presence、数值边界、逐键 read-back、生产 seam 不重复解析；保持旧 wire shape | admin + interface |
| `AdminSetting.Mongodb` | mongodump/mongorestore paths | `Re`；配置更新 | allowlist/absolute/regular-file/canonical root；secret 不落配置响应；无效输入零写入 | admin |
| `AdminData.Backup` | remark | `Re`；mongodump、文件目录、metadata | 非公开 `mongodb_backup` root；最多 30 份或 20 GiB；argv/timeout/credential channel、文件/metadata strict read-back、partial/unknown | admin + delivery |
| `AdminData.Restore` | opaque createdTime | `Re`；先保护备份再 mongorestore | 只读登记 metadata；保护失败停止；目标 root/db identity 验证；不可伪造成功 | admin + delivery |
| `AdminData.Delete/UpdateRemark` | createdTime、remark | `Re`；metadata/文件变更 | createdTime 不作路径；root containment、CAS/read-back、重复/未知语义 | admin |
| `AdminData.Download` | opaque createdTime | tar.gz binary | 安全相对名、symlink/traversal/预算、逐文件错误/cleanup；不可 panic/空成功 | admin + delivery |
| `AdminUpgrade.UpgradeBlog` | 无显式输入 | 当前无稳定返回值 | 必须有 checkpoint、统计和错误返回；controller 不把 nil 当成功 | admin |
| `AdminUpgrade.UpgradeBetaToBeta2/3To4` | 当前 userId 来自 session | `Re`；多集合 migration、版本标记 | 幂等 step、resume、USN/index/read-back、partial/unknown | admin + delivery |
| `AdminEmail.Set/Template` | SMTP config、subject/body templates | `Re`；配置更新 | CR/LF/template parse/secret redaction；空 masked password 不清空 | admin |
| `AdminEmail.SendEmailToEmails/SendToUsers2/SendToUsers` | recipient list/filter、subject/body、verified、saveAsOldEmail、可选客户端 `batchId` | 当前 `Re`，其中 users 发送可能 request goroutine | 新请求 batchId 为 32 位小写十六进制；旧无 ID 生成并通过 `Re.Id` 返回 reconciliation ID 且 `RetrySafe=false`；批次和 `(batchId,recipientId)` event identity 可重放/冲突；recipients bounded/dedup；HTTP 200/`Re.Ok=true` 只表示已入队；transport retry/dead；旧 `Re` 兼容 | admin |
| `AdminEmail.DeleteEmails/List` | log IDs、sort、keywords | `Re`/HTML；EmailLog 读写 | ID/selector 校验、敏感字段 redaction、查询错误不空成功 | admin |
| comment transport | publishing 已确认 comment event | outbox/SMTP 状态 | 新事件 BSON typed `CommentId`/`RecipientId`/`IdempotencyKey`/`EventVersion` metadata 和 partial filtered unique index；迁移只允许 `_id + Version + LeaseId` CAS/read-back；旧 payload-only event 严格只读兼容，缺失/冲突进入 `handoff_unknown`；CAS state machine、cancel read-back、旧账号 event 隔离 | admin + publishing + delivery |

## 证据边界

- 现有 `conf/routes` 使用 `/:controller/:action` 通配；本清单不能替代 method/status/Content-Type/redirect 的真实 HTTP replay。
- `/suggestion` 当前实现允许匿名且在持久化成功前没有 outbox receipt；该事实只说明待修缺口，不表示任何新规则已实现。
- comment outbox 的 focused memory/Mongo tests 已存在，但 Mongo/SMTP 真实竞态、人工 `handoff_unknown` 对账、metadata/index 迁移仍为 `unrun`。
