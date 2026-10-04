# application-admin 规格审核记录（2026-09-25）

## 结论

按父任务应用轨道 `identity → notes → content → publishing → admin`，当前选中的 ready 叶是 `09-08-application-admin`。`task.json.meta.depends_on` 所列基础依赖均已归档完成；本轮审核将其补齐为直接消费的六个已完成叶：domain、infrastructure、identity、notes、content、publishing。未创建任务，审核期间不修改业务实现、测试、配置或生成资源。

规格经过本轮重写后，目标、边界、错误分类、输入输出、交互流程、数据约束、兼容性、上下游 owner 和验收证据入口已具名。Q-A1～Q-A6 原先不能由仓库可靠推出，已由用户于 2026-09-25 全部采用推荐方案并写入 PRD §8、design、implement 和验收矩阵。任务可以按已冻结合同进入实现准备；真实环境证据仍须单独执行。

## 选择与依赖证据

| 叶 | status | `meta.depends_on` | 依赖状态 | 轨道结论 |
| --- | --- | --- | --- | --- |
| application-admin | in_progress | domain-contracts、infrastructure-persistence、application-identity、application-notes、application-content、application-publishing | 六个归档 `task.json.status=completed` | 当前轨道首个未完成 leaf，已激活并完成规格决策 |
| interface-http | planning | identity、notes、content、publishing、admin、persistence、domain | admin 尚未完成 | 后置，不 ready |
| presentation-frontend | planning | domain、notes | 已完成但按轨道晚于 admin | 后置，不抢占 admin |
| delivery-verification | planning | 全部应用/接口/呈现叶 | admin/interface/presentation 未完成 | 汇合层，最后执行 |

`task.py current --source` 初始为 none；父任务的 `[6/10 done]` 计数没有被用作 readiness 依据。ready 结论来自叶状态、显式依赖和轨道顺序三者同时满足。

## 审核范围与证据

### 已阅读的任务/规格材料

- `.trellis/tasks/09-08-business-layer-architecture/{prd,design,implement,task}.md/json`
- 当前 admin 的 `prd.md`、`design.md`、`implement.md`、`task.json`
- 已归档 domain/persistence/identity/notes/content/publishing 的 PRD、设计、验收矩阵和决策摘要
- `.trellis/spec/backend/{index,error-handling,quality-guidelines,database-guidelines,logging-guidelines}.md`
- `docs/adr/0001-stage-modernization-as-contract-first-dag.md`、`docs/CODEX_HANDOFF.md`

### 已阅读的实现与入口

| 证据 | 观察 | 对规格的影响 |
| --- | --- | --- |
| `app/service/ConfigService.go:43-88,106-176` | 初始化/更新忽略部分 DB 错误；更新先改变内存，controller 多次赋值覆盖 `re.Ok` | 必须定义 cache/Mongo read-back 和逐键失败，不得以最后一次 bool 作为成功 |
| `app/service/ConfigService.go:356-519` | backup/restore 使用 `/bin/sh -c`，命令含 host/db/user/password/path；delete 直接 `RemoveAll` | 必须把 argv、timeout、credential channel、root containment、unknown/partial 纳入 AC |
| `app/service/SuggestionService.go:15-20`、`app/controllers/IndexController.go:33-43` | feedback 只有 insert，邮件在返回前启动无结果 goroutine | 必须定义 durable feedback+outbox success、重试 identity 和匿名边界 |
| `app/service/EmailService.go:269-406,709-788` | comment renderer/SMTP context 已有；批量用户邮件仍有 goroutine；header/recipient 已有部分校验 | admin 规格需区分旧账号 event、comment 和 feedback；去除 admin request goroutine |
| `app/db/outbox.go:19-61,272-494` | comment 状态、确认、handoff、取消 CAS 已存在；状态和取消元数据仍与 payload/LastError 有耦合 | 旧“未实现”陈述必须删除，补充 typed metadata/index、批量取消读回和未知交接对账 |
| `app/service/BlogService.go:1413-1645,1680-1765` | comment/计数/通知 intent 在 workspace mutation 中共同确认；删除先取消 outbox 再删 comment | admin 不得重做 comment 权限/共同提交，只接 transport 合同 |
| `app/db/persistence_indexes.go:60-110,151-199` | outbox 唯一键为 IdempotencyKey，缺 `(Kind, CommentId, RecipientId)`；comment receipt 有 actor/submission 唯一索引 | 新 comment metadata/index 和历史冲突/preflight 必须进入规格；feedback receipt 应复用 persistence 原则而非正文猜测 |
| `app/controllers/admin/{init,AdminSettingController,AdminData,AdminEmailController,AdminUpgradeController}.go` | admin auth 是 session username 比较；多键写入、下载/panic、模板和设置 action 范围较大 | action inventory 单独落盘；principal seam、binary error、secret redaction 和逐 action 验收不能省略 |
| `conf/routes`、`app/controllers/init.go` | `/suggestion` 位于公开白名单，其他 admin 通过通配 route；当前公开性和旧 wire 需保持 | anonymous 基线写入 PRD；HTTP method/status 交给 interface/delivery 实证 |

## 严重发现与修正

### R1 — comment outbox 基线过时且与代码冲突

原 design/implement 将 comment outbox 写成“现有五态、尚未实现”。当前代码已经有十态、确认闸门、handoff gate、取消 CAS、EmailService comment renderer 和 focused tests。继续保留旧表述会导致重复实现、错误依赖和验收遗漏。

**修正**：PRD §2、ADM-COMMENT，design §2/§6，implement §5 改为“部分实现基线 + 明确剩余合同”：typed CommentId/RecipientId 元数据、索引、批量取消读回、旧 event 兼容、真实 Mongo/SMTP/人工对账仍是 admin 联验范围。

### R1 — feedback 关键输入和重试合同缺失

原规格只写“要定义 anonymous/长度/重复 identity”，没有说明当前 route 是公开的、旧客户端可能只发送 Suggestion、持久化和邮件目前分离失败、也没有任何数值上限或 identity 形态。直接编码会擅自改变匿名行为或用正文/时间窗口猜测去重。

**修正**：保留公开/匿名兼容基线；将 Addr 语义、rune/byte 上限、submission identity、收件人配置列为 Q-A1～Q-A3；固定 feedback+outbox durable success、HTML/header escaping、`RetrySafe=false` 的旧请求和 transport 独立失败规则。

### R1 — backup/restore/download 的安全和成功边界不完整

原规格覆盖 executable allowlist，却遗漏 backup root、metadata/path binding、Delete/Download、symlink/traversal、archive budget、保护备份失败、未知命令结果和当前 controller panic。现实现还直接 shell 拼接凭据并日志记录命令。

**修正**：ADM-DATA、design §4、action inventory 和 AC-A3 增加 root-relative manifest、argv/timeout/credential channel、strict read-back、保护备份、safe tar、预算、cleanup 和 partial/unknown 语义；PDF 明确只交 descriptor。

### R1 — 配置更新和 secret 投影未定义一致性

原规格没有约束多键更新的先错覆盖、内存缓存先于 DB 成功、通用 admin view 泄露 `emailPassword`/`demoPassword`。这会让请求返回成功但 cache/Mongo 分叉，或把密码渲染到模板。

**修正**：ADM-CONFIG、design §3、AC-A2/A8 固定 preflight → durable write → read-back → atomic cache，secret redaction 和 masked input 语义。

### R2 — 身份和 admin 任务责任重叠

注册激活、找回密码、改邮箱和邀请当前分散在 User/Email service；identity 任务已拥有其业务规则和 outbox。若 admin 直接把所有 `SendEmail*` 改成 comment 状态机会破坏旧 event。

**修正**：PRD/设计把 identity event 列为兼容 consumer，admin 只拥有广播、feedback 和 comment transport；每个 `Kind` 独立回归。

### R2 — 验收只有主题句，没有逐 action 和真实证据边界

原 AC 没有列出 admin data download、升级无返回值、模板/日志、comment identity/index、HTTP route inventory，也没有区分 focused tests 与 Mongo/SMTP/HTTP/browser/failpoint 证据。

**修正**：新增 `research/action-contract-inventory.md`、`acceptance/evidence-matrix.md`，PRD AC-A1～A9 和实现计划逐项映射 owner、命令、状态和未运行证据。

## 已冻结的实现前合同

1. `/suggestion` public/anonymous 是当前兼容基线；是否改为 login-only 不可在实现中静默决定。
2. feedback durable success = feedback 文档与必要 outbox/receipt 均已确认写入；SMTP transport 失败不回滚已提交 feedback。
3. comment 业务权限和共同提交归 publishing；admin 只拥有 typed outbox transport、状态/CAS、取消读回和安全模板。
4. production-config 只有 interface-http 一个 producer；admin 只消费结构化 contract。
5. PDF renderer 只有 application-content 一个 owner；admin 只输出 descriptor/allow-policy。
6. 真实环境证据不由 task status、静态扫描、build 或 focused unit test 替代。

## 已确认决策（2026-09-25）

用户确认全部采用推荐方案，冻结以下实现前合同：

1. **Q-A1 feedback 输入**：`Addr` 是可选联系邮箱；空值允许；非空必须是单个 addr-spec、最多 254 字节，拒绝显示名、CR/LF 和控制字符。`Suggestion` 必须合法 UTF-8、trim 后非空，最多 2000 个 Unicode 码点和 8192 字节；trim 只用于校验，不改写正文。
2. **Q-A2 feedback identity**：登录新客户端使用 128-bit 随机、32 位小写十六进制 `submissionId`；匿名携带 ID fail closed，匿名旧请求无 identity 时仍可提交但标记 `RetrySafe=false`。唯一键为 `(ActorId, SubmissionId, Kind=feedback)`，仅对有 identity 的登录 receipt 建立；相同 identity 和摘要回放原 receipt，目标/正文冲突零写入；不按正文/时间猜测去重；终态 receipt 至少保留 30 天。
3. **Q-A3 feedback recipient**：使用独立 `feedbackRecipients` 内部收件人列表，最多 20 个，启动和更新时规范化、校验、去重；`Addr` 不直接成为 SMTP `To`，默认不作为 `Reply-To`；收件人缺失或无效时在 feedback 写入前 fail closed。
4. **Q-A4 enqueue 返回语义**：feedback 和 admin broadcast 在 outbox/receipt durable 写入并读回后保持 HTTP 200/旧 `Re` 外形，`Re.Ok=true` 只表示“已入队”；SMTP 由 worker 处理，部分/未知写入返回失败和可对账 ID，SMTP 失败不回滚已确认业务写入。
5. **Q-A5 backup 边界**：默认使用应用数据目录下的非公开 `mongodb_backup` root；最多 30 份或 20 GiB；单次 Download 最多 100,000 个文件、10 GiB 源文件、4 GiB 生成归档；Restore 仅允许同一 configured database identity，且必须先确认保护备份。
6. **Q-A6 comment legacy migration**：新 comment event 写入 typed `CommentId`/`RecipientId`/`IdempotencyKey`/`EventVersion` metadata，并建立带类型过滤的唯一索引。旧 payload-only event 只读兼容；缺失或冲突拒绝发送并进入 `handoff_unknown`/人工对账；不在启动时静默回填，迁移必须经过 preflight、dry-run、分批 CAS、读回并保留旧字段。

这些决策解除规格阻塞，不代表任何实现或真实运行证据已通过。AC-A5 不再是 `blocked-by-decision`；所有 AC 仍按验收矩阵记录 `unrun`、`partial` 或 `delegated-unrun`，直到对应边界实际验证完成。

## 验证记录

- 审核前 `python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-admin` 因两个 context manifest 指向已归档后不存在的 `.trellis/tasks/09-08-application-publishing/*` 而失败（10 个文件不存在错误）。本轮将路径改为 `.trellis/tasks/archive/2026-09/09-08-application-publishing/*`，并加入本任务的 research/acceptance 材料。
- 审核阶段未运行应用、未调用 computer-use、未修改业务实现或测试；真实 Mongo/HTTP/SMTP/browser/failpoint 证据保持 `unrun`/`delegated-unrun`。

## 2026-09-25 repair 记录

- 按 diff-review 修正匿名身份：匿名携带 `submissionId` fail closed 为 `validation`，只有匿名旧请求无 ID 可提交且 `RetrySafe=false`；不使用共享 sentinel，也不进入 receipt 唯一索引。
- 固定 broadcast 客户端 `batchId`（32 位小写 hex）；旧请求生成 reconciliation ID、写入 receipt 并通过旧 `Re.Id` 返回且标记不可重试；补充 batch/body/recipient digest replay/conflict。
- 将 feedback durable success 明确定义为 feedback 文档、receipt、outbox 三者在同一事务/持久补偿边界内提交并逐项 strict read-back；SMTP transport 失败不回滚已确认的三者记录。
- 补齐 `feedbackReceipts` BSON schema、collection、partial unique index 名称/keys/filter、recipient snapshot、outbox IDs 和 30 天 terminal GC。
- 补齐 canonical `ConfiguredDatabaseIdentity` 字段与长度前缀 SHA-256、typed `CredentialProviderRef` owner、保护备份定义、GC lease/CAS 及孤儿/超限处理。
- 补齐 `upgradeCheckpoints` collection/schema/unique key、确定性 step identity、lease/fencing/read-back；context manifest 增加 identity/notes/persistence、backend database/logging 和 interface-http 权威材料。
- 本轮只编辑任务规格/研究/验收/manifest；未运行真实 Mongo、HTTP、SMTP、浏览器或 failpoint，AC-A1～A5/A8 仍 `unrun`，AC-A6/A7 `partial`，AC-A9 `in-progress`。

## 实现批次后的基线校正（2026-09-25）

上文“当前实现”段落记录的是规格审核前快照，不能作为修复后的现状。修复批次已将 feedback 写入 durable receipt/outbox、将 backup/restore 切换到 argv 与受控 stdin、加入 protected restore backup/identity digest、补齐安全配置入口与数组预检，并把 `/suggestion` 内部错误映射为稳定分类键。focused 验证已记录在 `acceptance/evidence-matrix.md`；真实 Mongo/SMTP/HTTP/工具进程和故障注入仍未运行。
