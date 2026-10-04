# 交付验收矩阵

审核日期：2026-09-29。审核基线：`3b963562`，开始审核时工作树干净；本轮仅激活任务和修改规划材料，未运行产品验收。正式候选尚未冻结，当前候选状态均为 `unrun`，不把归档、历史工作树测试或规格校验升级为通过。

同日复审基线为 `e1dfd907`（初始工作树干净），任务此前已激活；此次只补规格，未重复激活。首次审核记录保留在文末，复审新增候选/门禁/恢复负例见 `gate-integrity.md`，不改变任何产品实证状态。

## 上游交接与责任

下表 owner 均位于 `.trellis/tasks/archive/2026-09/09-08-<owner>/`，其 `acceptance/evidence-matrix.md` 是交接明细入口。已确认 PRD/design 决定目标行为，代码与运行结果决定是否达到目标；历史矩阵中的“尚未实现”须与较晚实现核对。

| ID / PRD | 必须覆盖的目标与异常 | 上游入口 / 缺陷 owner | 当前候选 |
| --- | --- | --- | --- |
| E-D01 / AC-DV1 | 模型 JSON/BSON/ID/nil/empty、presence、API envelope/字段顺序、所有权、USN tombstone/sync、Golden baseline 与批准 target 分离 | domain-contracts AC-D1～D9；notes、persistence、interface 消费 | unrun |
| E-D02 / AC-DV2 | Mongo 7 standalone、8 standalone、8 replica-set；transaction/显式补偿、索引 preflight/read-back、session TTL、token 唯一约束、outbox lease/retry/dead、未知提交；显式接收 identity AC-I9 的 sessions idle TTL/TTL index/过期边界、Email/Username/(ThirdType,ThirdUserId) 唯一约束及旧 token 文档兼容；旧冲突数据不自动删改 | infrastructure-persistence AC-P1～P7；application-identity AC-I9 | unrun |
| E-D03 / AC-DV2 | identity AC-I1～I18（I10 为本任务自身的跨轨汇合）、P-01～P-08；I9 的持久化证据由 E-D02 接收；匿名 `_ID` 稳定/认证轮换、Cookie 失败 envelope、principal/demo、摘要 token/legacy 过渡、注册多写失败及真实邮件；I15～I18 的密码/邮箱更新与 token 消费跨集合原子性、登出清理失败、注册后 Cookie 提交失败、token 强度/存储与脱敏；I14 由本行及 PRD R3 接收，保留仅 `_ID`+Captcha、无通用限流及无 CSRF/query-form token 的已确认基线与风险 | application-identity；interface/admin/persistence 联验 | unrun |
| E-D04 / AC-DV3 | notes 38 action 逐项 live HTTP：method/path/principal/presence/status/Content-Type/body/owner/USN/Golden-target；历史 21/38 仅是补齐线索 | application-notes/research/action-inventory.md；本任务 notes-action-replay.md | unrun |
| E-D05 / AC-DV4 | claim/apply/verify/save/commit kill/restart、Mongo failpoint；no-op/同 ID 重试/更换输入冲突；一次 USN/history/destination、冻结 source manifest、repair 完成、不回拨 counter；跨主机/持久化卷故障 | notes receipt + content provider + persistence | unrun |
| E-D06 / AC-DV4、7 | content E-C01～E-C31：ACL、bounded multipart/image decode/remote fetch、Files 三态、PreNote、archive/cleanup、roots/quarantine/durable manifests；legacy Web image/attachment create 与 CopyHttpImage 的 partial_write、owner-row Verify、quarantine/late-row repair；D-H7/MOD-004 的 CopyHttpImage 每请求新导入不按 response-loss 幂等验收；PDF 正向/恶意 corpus、local-file deny/zero outbound | application-content；production/wire consumer 归 interface | unrun |
| E-D07 / AC-DV5 | publishing AC-PB1～PB7 及细分项：分享期限/权限/索引、Host/query/JSONP、逐 child publish receipt/projection repair、submissionId 跨保留期回放、评论通知取消/未知交接、主题/预览/公开/ZIP/owner | application-publishing；admin/interface/presentation 联验 | unrun |
| E-D08 / AC-DV5 | admin AC-A1～A9：角色矩阵、多键 config read-back、真实 backup/restore/credential channel/预算/清理、升级 checkpoint/restart、Suggestion/broadcast receipt/outbox/SMTP、日志脱敏 | application-admin；publishing/identity/persistence 联验 | unrun |
| E-D09 / AC-DV6 | interface AC-H1～H12：registry/action 对账、404/405、binder/session/locale/template、native harness、production exit 78、未就绪恢复、SIGTERM、旧运行时零依赖；D-H6/AC-H9 的 GetImages/GetAlbums 依赖失败 500 + JSON 空 Page/[]、成功不变，与 Attach.GetAttachs 的 200 legacy error 分别验收 | interface-http；registry 与 notes 38 action 为不同分母 | unrun |
| E-D10 / AC-DV6、8 | presentation AC-PF1～PF7：干净候选重建零漂移、资源/iframe、编辑器、零编辑零写入、Web OperationId/ExpectedUsn、未知重试、move 权威读取恢复、copy/delete 队列出口、旧客户端 Golden | presentation-frontend；收集 browser+HTTP+DB receipt | unrun |
| E-D11 / AC-DV7 | Linux/amd64 tarball/image、非 root、外置 Mongo、paired volumes/backup root、升级复制只读核对、restart persistence、生产配置负例、PDF 可读文件和失败清理 | delivery；业务/provider 缺陷回原 owner | unrun |
| E-D12 / AC-DV8 | Chrome/Edge/Firefox/Safari current/previous 八槽 × 四 coverage；真实版本/provenance/JCS；附加业务清单不能被 32 个 smoke 结果替代 | delivery；受保护 browser workflow | unrun |
| E-D13 / AC-DV9、10 | 质量门、summary 数量/退出/脱敏、严格 tag、tar SHA-256、OCI/digest、跨 workflow 关联、重复/部分发布、Release/GHCR 最终读回；已确认恢复合同见 `release-recovery.md`；当前候选绑定、缺门禁、远端误判空、跨分支版本与普通发布读回见 `gate-integrity.md` GI-01～GI-09 | delivery；具体发布执行沿明确授权范围 | unrun |

## 历史证据边界

- notes mapping=21/38（17 API、4 Web），余 17 Web 无逐项 live 映射。native harness 后续新增回放不自动更新旧通过率；当前候选按 38 个 ID 重新对账。
- interface 2026-09-28 有 Mongo 8 standalone/native HTTP/Golden 历史通过；明确未执行 Mongo 7/replica-set、failpoint、browser、PDF、生产/container/backup。合并后候选需要复验。
- presentation 工作提交 `08bdfadb7d40c67d61f9c410846f3f2ccfb924d2` 仅关闭实现；本任务承接全部 partial/unrun/delegated-unrun 行和未勾选手工清单。
- publishing/admin 的 Mongo/SMTP、主题/备份/升级及故障注入缺口同等阻断交付。既有无 CSRF/限流现状及同源主题脚本不隔离风险随证据移交，兼容通过不等于新增安全保证。

## 每次执行记录

必须填写 `evidence_id / AC / scenario_id / candidate_commit(full SHA) / workflow或本地runner / run_id / attempt / UTC时间 / command或具名手工步骤 / fixture标识 / OS与工具链、Mongo拓扑、浏览器完整版本 / discovered、executed、passed、failed、skipped / exit_code / status / cleanup_status / 脱敏失败分类 / artifact定位与SHA-256 / 缺陷owner及恢复条件`。本地验证使用真实本地 run 标识，不伪造 GitHub provenance。

- discovery 与 execution 分列；必需场景 skip、执行零项、清理失败、artifact 缺失/过期不能 passed。排除场景须有上游明确约束，不临时放宽。
- task 的 `in_progress` 仅表示已激活；证据用 `unrun / partial / passed / failed / blocked`。blocked 注明 decision/environment/artifact/provider 原因，不改成未经工具定义的生命周期状态。
- 接收上游历史原文时保留其来源和原状态，不直接喂给当前证据 validator；例如 admin AC-A9 的 `in-progress`、旧 `delegated-unrun` 不代表 passed。按实际已执行断言登记 partial，完全未跑登记 unrun，缺条件登记 blocked；本候选未跑始终不得继承历史通过。Mongo 8 standalone 已在 E-D02 明确作为独立必需场景，无须为补交接而修改归档 owner 矩阵。
- 受保护 runner 仅发布 allowlist 摘要；禁止原始 HTTP、正文、Mongo URL、cookie/token、trace/截图/视频或服务日志。PDF 使用合成无敏感内容样例，观测仅输出脱敏断言。
- 同时保留首个失败与 cleanup 失败，不能用清理成功覆盖原失败。重跑创建新 attempt，原记录不覆写。

## 实现阶段工具验证（2026-09-29）

最新关闭口径：用户明确排除需要真实环境验证的功能，并授权直接提交、归档。下列本地工具检查为本次交付证据；E-D01～13 及所有真实场景的既有 unrun/partial/blocked 保留，不转换成通过。完整受保护场景执行器、17 Web replay 及外部运行条件不再阻断本次任务关闭，但仍阻断声称综合联验或实际发布成功。本节后文“未提交/未归档/仍为 in_progress”为执行检查时的历史快照。

用户后续确认受保护环境尚未部署，先完成仓库工具，真实验收明确保留阻断。

授权：用户“批准激活进入实现”。候选分类：working-tree，dev / e1dfd907 + 未提交改动；Windows/amd64、Node 24.21.0、Go 1.27.1。本节不修改 E-D01～13 的真实产品状态，不回填归档 owner 矩阵。

- 首轮全量 npm test：205 tests，204 passed、1 Windows 平台 mode 测试 skipped、0 failed，约 197 秒。随后增加任务归档目录解析、缺失退出码、checksum 原字节篡改用例，对全部变更相关套件重新定向回归；不把首轮计数冒充后续最终全套计数。
- npm run build 通过；紧接 git diff --exit-code -- public app/views/note/note.html 为 0，生成资源零漂移。与全量 Node suite 串行。
- GOTOOLCHAIN=local go build ./... 和 go vet ./... 均通过。没有新增 Go 业务代码；未运行真实 Mongo/HTTP/Golden/USN 验收。
- actionlint v1.7.7 检查 release.yml、release-recovery.yml、delivery-evidence.yml、quality-gate.yml 通过；shellcheck/pyflakes 子检查未启用。首次仅缺自托管标签声明，已添加 .github/actionlint.yaml 及精确 gitignore 例外后复验。
- 回归按红绿执行：过期但自洽的 summary、未记录/非零退出、缺门禁/错误锁名、源版本与 attempt、缺失/过期/跨 attempt 原 artifact、远端 JSON/权限不明、错误资产读回、checksum 换行字节篡改。所有远端行为使用受控内存 fetch seam，真实 GitHub/GHCR 写入数为 0。
- 补充子进程协议测试：在临时 Git fixture 运行明确标记的 synthetic child，验证成功输出、exit 7、超时和 cleanup failed 的传播与临时文件清理。它只证明执行协议，不是 Mongo/HTTP/browser 场景执行器。
- 最终定向回归：七个交付相关测试文件合计 58 tests、58 passed、0 failed/skip；之后顺序执行四个 workflow 的 actionlint、git diff --check、全部改动 JS 语法与冲突/尾随空白检查均通过。范围核对 34 文件，均为交付工具/CI/tests/docs/spec/本叶材料，无业务实现或生成资源差异。首次范围检查与测试并行时扫入尚未清理的临时 fixture，测试结束后顺序复验通过，没有删除用户文件。
- 最后 task validate 通过，implement/check 各 17 entries；current 仍为本叶，status=in_progress、children=[]。未创建新任务、未提交、未归档、未 push、未真实发布。
- 发布普通/恢复工作流已接线；完整受保护场景执行器、逐项真实环境与 cleanup 映射仍未完成，当前 release 必须阻断。38 action 目录不是 38 项已实现；17 Web replay 未补齐。明确缺口、owner 和恢复条件见 research/implementation-2026-09-29.md。
- 原生独立 trellis-check 因代理协议错误未运行；已做主流程 diff/测试复核及只读探索代理交叉检查。不能宣称独立质量门或最终综合验收通过。

## 本轮规格审核

归档收尾补充（2026-09-29）：工作提交 62b9e2caa26ec98a4b23cd672364ed7da613b59a；按用户最新范围关闭并移动到 archive/2026-09。归档后 implement/check 各 17 引用校验通过，交付目录仍解析为 203 项。额外回归暴露测试夹具固定活动任务路径的问题；按目录 delivery 来源初始化临时活动 PRD，验证活动 → 归档 → 重复目录拒绝，delivery-evidence 六项测试最终 6/6 通过。该夹具修正与 14 条上下文引用及两处来源路径修复随归档提交记录；没有改动发布运行时门禁。真实环境项继续保留原状态，本次关闭不代表其通过。

以下为历史审核记录；后续实现授权及代码验证见“实现阶段工具验证”，不据历史“仅规格”表述抹去已授权实现。


- 激活前发现 manifests 共 7 个归档失效路径，修正后 validate 通过并成功激活。
- 产品测试、Mongo、SMTP、HTTP、browser、Docker/PDF、GitHub/GHCR：本轮均未执行。
- 当前 branch/base_branch 同为 `dev`，start 提示将来 PR/archive 分支校验限制；本轮不切分支、不提交、不归档。
- 首次审核上下文校验：`task.py validate .trellis/tasks/09-08-delivery-verification` 通过，implement/check 各 14 entries；当前任务查询确认本叶为 active。
- 只读规格检查通过：9 个依赖均 archived completed；38 个 action 与上游 ID/入口集合一致，历史 mapped=21、missing=17、当前全部 unrun；两份清单各自无重复且所有引用存在；AC-DV1～10 和 E-D01～13 完整。校验脚本首次误把上游 IN 内部操作算入 action，收紧到七类公开 action 前缀后通过，未修改源清单。
- 首次审核 `git diff --check` 通过（仅 Windows 换行转换提示）；当时含未跟踪文件共 10 个改动，全部在本叶目录；Markdown 无尾随空白/冲突标记，notes 相对来源链接可解析。task.json 仅有 start 所需 status/branch 更新及工具格式化，依赖未变。
- Q-DV1 于 2026-09-29 获用户“允许”确认：仅核验后受控补建缺失 Release。当前已识别产品决策全部收敛，恢复矩阵仍全为 unrun；本轮不进入功能编码，受保护浏览器与真实服务条件仍未核验。
- 决策同步后重新验证：task validate 通过（implement/check 各 15 entries），所有引用存在、各清单无重复，Q-DV1 无残留待决表述；恢复矩阵 RR-01～RR-12 共 12 项、全部 unrun。`git diff --check` 和改动范围检查通过，累计 11 个文件均属于本叶，没有业务/CI/script 变更，也没有远端写入。
- 2026-09-29 审核修复：此前 AC/证据覆盖检查仅证明本地编号完整，未证明上游 identity 全量接收；本次对照归档 identity 矩阵 AC-I1～I18 补全 E-D03 和 AC-DV2，明确 I10 为本任务汇合、I9 由 E-D02 接收、I14 由 E-D03/R3 接收。design §5、Phase 1、RR-09 同步要求 recovery 按目标 tag 派生与普通发布完全相同的 `release-refs/tags/<tag>`，覆盖独立 workflow/分支 dispatch；该项仍待实现和真实验收。task.json 已补末尾换行；task validate（两份清单各 15 entries）、JSON/末尾字节检查及 `git diff --check` 通过。未创建任务、未改业务/CI/script、未提交或调整分支，产品与发布证据仍为 unrun。
