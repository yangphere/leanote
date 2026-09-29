# 交付层：集成验证与版本发布 — PRD

## Goal

以真实可审计证据验证所有分层任务的兼容性，并建立不自动部署生产的版本制品交付。

## Scope and review status

- 2026-09-29 选择并激活既有 P1 叶任务；`children=[]`，9 个 `meta.depends_on` 均为归档 completed。父任务只协调，当前没有更早未完成的 ready 叶。归档不等于真实验收完成。
- 本轮仅规格审核：允许本任务 PRD/design/plan/context、research/acceptance 和授权激活所需元数据；不修改业务代码、测试脚本、CI 或生成资源，不提交、不发布。
- 后续交付实现限于验收 harness、CI、制品与证据校验、运行手册；服务端/前端业务缺陷回原 owner，不在交付层复制权限、USN、receipt、路径或业务规则。重开归档 owner/增加业务范围须另获授权，本轮不创建任务。
- 2026-09-29 用户确认 Q-DV1：允许核验原 tag/commit/digest/制品后受控补建缺失 Release。当前已识别的产品决策全部收敛；本轮仍限规格审核，功能实现及真实验收尚未执行。

## Requirements

### R1 — 候选、输入输出与证据状态

输入为干净检出的完整 candidate SHA、批准的业务契约及 fixture、锁定工具链/镜像、隔离测试配置和受保护 runner。正式发布另需与项目版本一致的严格 `vX.Y.Z` tag、仓库身份及明确发布授权。缺失/畸形 SHA、输入文件、依赖或凭据在副作用前失败，禁止自动补零、切换环境或借用旧制品。

输出为逐场景证据矩阵、脱敏质量 summary、两文件 browser artifact、release-inputs/build-metadata/校验和/tarball，以及授权发布后的 GHCR digest 和 Release 资产核对结果。字段与文件 allowlist 复用现有 validator，不另建并行 schema。每条证据绑定候选、真实 run/attempt、环境、discovery/execution/pass/fail/skip、命令、退出与清理状态、artifact 和失败 owner，详见 acceptance 矩阵。

历史/本地/预检证据与最终发布证据分开：历史通过不能直接给新候选放行；本地记录不伪造 GitHub run；普通最终发布输入与浏览器 artifact 必须由该 release run/attempt 生成。独立 tag-precheck 仅验证候选，不能复用为 final，也不能替代实际发布完成。Q-DV1 恢复只消费原发布 run/attempt 的完整已验证制品，另记恢复 run/attempt 与原执行的关联，不能把原 provenance 改成恢复 run。

### R2 — 质量门与完整上游交接

PR/push 质量门覆盖 Go 1.26/1.27、Mongo 8、Node 24、JS/build、Chromium、Golden/USN、package/container smoke；具体固定 patch/image digest 从候选工作流读取。额外矩阵必须覆盖 Mongo 7 standalone 和 Mongo 8 replica-set，不能因默认 CI 只有 Mongo 8 standalone 而省略。

逐项消费 domain、identity、notes、content、publishing、admin、persistence、interface、presentation 九个上游矩阵（路径和 AC 映射见 `acceptance/evidence-matrix.md`）。缺少旧运行证据是待本任务补齐的工作，不是禁止开始补齐的循环前置；缺失业务合同或 provider 实现则阻断相关场景。历史“尚未实现”文字须与现代码核对。

### R3 — 身份、持久化与外部兼容

验证 transaction/显式补偿、唯一索引 preflight/read-back、TTL/token、outbox retry/lease/dead/未知交接；历史冲突数据必须明确拒绝就绪，不自动删除修复。identity 按已确认 P-01～P-08 验证匿名 `_ID` 稳定/登录轮换、Cookie 写回失败 envelope、32 字节 API token 摘要和旧 token 过渡、principal/角色与注册失败路径。

完整接收 identity AC-I1～I18（I10 为本任务自身的跨轨汇合）。AC-I9 的 sessions idle TTL/TTL index/过期边界、Email/Username/(ThirdType,ThirdUserId) 唯一约束及旧 token 文档兼容由 E-D02 接收；AC-I14 由 E-D03 接收，核验登录/找回密码/token 验证仅 `_ID`+Captcha、无通用限流的已确认基线与风险，不擅自新增限流。

保留无 CSRF/query-form token/`_ID`+Captcha 基线及已确认风险，不把兼容通过解释为消除风险。Golden replay 只读；旧缺陷 baseline 与批准 target 分离。既有 service→db/BSON 适配允许保留，不能借交付全库重构。

### R4 — Notes、内容与恢复一致性

按 notes action inventory 的 38 个 ID 建立 live HTTP 逐项映射；历史 21/38（17 API+4 Web）只是线索，当前候选重新执行，补齐 17 Web，不能以测试事件数或 registry 数冒充覆盖。记录 method/path、principal、字段 presence、status/Content-Type/body、owner/USN/history/receipt 和 baseline/target。

在隔离 Mongo/文件根验证 claim/apply/verify/save/commit 的 kill/restart、Mongo failpoint、跨主机/持久化卷文件故障：同操作不重复 USN/history/destination、不回拨 counter、冻结资产 manifest、unknown 可核对且不伪成功；验证 notes asset adapter 消费唯一 content publish/verify primitive。副作用故障与 cleanup 故障分别保留，未执行故障点保持 unrun。

### R5 — Publishing 与 admin 不得漏验

承接 publishing/admin 矩阵全部细分条款：分享期限/授权组合/索引、公开查询与 Host/JSONP、逐 notebook child 发布和 projection repair、主题上传/激活/公开/预览/ZIP/owner、评论 receipt/压缩/删除与邮件交接、配置、备份恢复、升级续跑、feedback/broadcast/outbox/真实 SMTP。

评论/回复必填 32 位小写 hex `submissionId`，旧缺字段请求拒绝的例外仅作用于该提交合同，不扩展到其他客户端。SMTP 入队不等于发送，未知交接不宣称 exactly-once。同源用户主题不隔离是已确认兼容风险，不能以页面通过宣称脚本安全。

### R6 — Native HTTP 与第一方交互

只使用 `cmd/leanote -runMode test` 和隔离 `leanote_test` 的原生 harness。registry/action、binder、404/405、session、locale/template、SIGTERM、旧运行时清扫按 interface AC-H1～H12 验证；不得生成第二运行时。

承接 presentation AC-PF1～PF7 全部 partial/unrun/delegated-unrun：干净候选构建零漂移、资源/iframe/编辑器、未编辑零写入、HTML 语义；update/copy/shared-copy/delete/move 中 OperationId/ExpectedUsn 生成、冻结未知重试、意图换代、stale conflict、HTTP+DB receipt；move 权威读取失败后的重读、copy/delete 队列出口及旧客户端 Golden。新建分支不擅自扩展 receipt，页面重载不恢复敏感请求体或自动重放。

### R7 — 生产配置、持久化、包与 PDF

消费唯一 production-config seam：显式只读配置、环境 secret/Mongo、错误退出 78、无仓库/localhost fallback、未就绪 503 与恢复后 200；真实进程证明错误发生在相应 bind/dial 边界之前。Linux/amd64 非 root、外置 Mongo、backup/private/public 持久化及临时目录权限遵循 `docs/modernization/cicd-delivery.md`；data/quarantine 同文件系统且 non-public，跨卷/可静态访问/重叠失败关闭，重启数据可读。旧卷迁移只在测试副本演练，旧数据保留至核验成功。

tarball 仅包含应用前缀，绝对 data roots 由安装步骤建立；镜像与包均不含真实凭据/用户数据。PDF 必须产出非空可读 PDF，验证 MIME/文件名/内容与超时取消/清理；在真实 Linux/container renderer 上验证 local-file deny、zero outbound 及 `file://`、loopback/private/metadata、redirect、CSS/script-fetch 恶意输入，合成 sentinel 不得出现在输出。HTTP 200 或一个正向 smoke URL 不足以关闭该项。

### R8 — 真实浏览器与隐私

Chrome/Edge/Firefox/Safari × current_major/previous_major 八槽；每槽固定顺序 `business-flows`、`editor-flows`、`bootstrap-components`、`leaui-image-iframe`。固定槽位的产品/完整版本、OS、执行时间和真实受保护命令版本必须可追溯，候选执行时记录版本来源，不能用 UA、Playwright WebKit 或 marker 伪装真实 Safari；环境缺失阻断该槽。

复用两文件 schema、JCS coverage digest 和 matrix 原始字节 SHA-256。四项 smoke 外仍需执行上游业务清单。只发布 allowlist 脱敏摘要，不上传 cookie/token、真实正文、完整授权 URL、trace/截图/视频或原始服务日志；CI diagnostic 输出同样不得泄漏凭据，校验 JSON 字段本身不构成脱敏证明。

### R9 — 最终发布及不确定结果

仅全部必需证据通过且发布获明确授权后，严格 `vX.Y.Z` 对应同一 commit/version；校验 tar SHA-256、build inputs、OCI 元数据及镜像 digest。禁止 latest、覆盖版本、移动 tag、ARM64 或自动部署。普通发布缺失/过期/跨 run/跨 attempt/不一致 digest 均阻断；artifact 保留期按现工作流 7 天，过期重跑，不伪造旧 provenance。若镜像已经存在而进入 Q-DV1 恢复，原制品或来源不可验证则保持阻断，不以重建制品代替原件。

现有流程先 GHCR push 再 GitHub Release。任何网络结果未知或部分发布必须保留 tag/commit/digest/资产身份，先只读核对远端；不得把“API 调用失败”当作“远端未写入”。Q-DV1 已确认允许受控补建：原发布必需门禁均通过、原制品完整可验证、远端 tag/commit 与 GHCR digest 一致，并明确确认 Release 不存在时，仅以原制品创建缺失 Release。恢复不得 build/push 镜像、删除/覆盖 Release 或资产、移动/重推 tag；任何查询未知、身份冲突或证据缺失立即阻断。Release 已完整匹配时只读确认完成，已存在但不完整/不一致时阻断，不补传或覆盖已有 Release。GHCR 与 Release 均读回匹配才可确认交付完成，并区分本次补建与此前已完成。

### R10 — 状态、责任与完成边界

任务 `in_progress` 表示已激活；场景 `unrun/partial/passed/failed/blocked` 表示证据状态，两者不能混用。必需项跳过、环境缺失、执行零项、清理失败或 artifact 缺失不算 passed；blocked 标注原因、owner、恢复条件。代码缺陷回既有 owner，交付工具缺陷留本任务。全部证据和远端结果确认前，不把本任务或父级综合验收标为完成。

## Acceptance criteria

- [ ] AC-DV1：九个上游矩阵逐行有接收 owner/场景/证据；干净候选 Go/JS/build/Golden/USN/权限/资产漂移通过，历史与当前证据分离。
- [ ] AC-DV2：identity AC-I1～I18（I10 为本任务自身的跨轨汇合）及 persistence 的 R3 合同在规定 Mongo 拓扑及真实 HTTP/mail 上通过；I9 由 E-D02 接收、I14 由 E-D03/R3 接收，风险和日志脱敏可复核。
- [ ] AC-DV3：notes 38 个唯一 action ID 完整 live mapping，含其负例与目标断言；38/38 不从测试事件数推导。
- [ ] AC-DV4：R4/内容矩阵每个适用故障点有恢复后 DB/receipt/file/USN/history 断言，不重复提交、不伪成功。
- [ ] AC-DV5：publishing/admin 所有交接条款有真实 Mongo/HTTP/SMTP/主题/备份/升级及失败证据，未知投递可对账。
- [ ] AC-DV6：native interface 与 presentation 的完整交互/恢复/旧客户端兼容通过；零编辑零写入有 browser+HTTP+DB 联证。
- [ ] AC-DV7：Linux/container 配置退出/未就绪、非 root/volumes/restart、包、PDF 正负 corpus/zero outbound/清理通过。
- [ ] AC-DV8：八槽四 coverage 的真实浏览器来源、完整版本与 final artifact 校验通过；额外业务清单和脱敏通过。
- [ ] AC-DV9：质量及发布 validators 对缺文件、错字段、错 tag/version、非法跨 run/attempt、digest 漂移、重复/未知状态有明确失败，校验失败零远端写入；恢复仅允许原执行与恢复执行的显式关联，不放宽普通 final guard。
- [ ] AC-DV10：全部必需证据通过，获具体发布执行授权后 GHCR/Release 均读回一致；按 `acceptance/release-recovery.md` 完成 Q-DV1 恢复正负例，包括响应丢失、并发和重复调用，没有覆盖、自动删除或镜像重推。

## Out of scope

不自动部署生产、不发布 ARM64、不提交真实凭据、不删除失败 artifact、不用模拟 Safari 关闭真实浏览器门禁。

## Confirmed decision and execution prerequisites

**Q-DV1 — 部分发布恢复（已确认，2026-09-29）**：用户答复“允许”，采用核验后受控补建缺失 Release，条件及禁止操作见 R9。此确认关闭产品规则待决项，不代表当前代码已经支持恢复，也不代表已对具体远端版本执行发布。

受保护 runner、八槽安装和执行命令、隔离 Mongo/SMTP/Linux 环境及发布凭据是待提供/核验的执行条件，不因尚未运行而伪造新的产品选择。当前/前一主版本须在执行时按真实来源冻结，本文不猜版本号。
