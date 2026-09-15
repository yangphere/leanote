# 应用层：笔记工作区与同步 — 执行计划

> 任务已获批准并处于 `in_progress`。2026-09-13 用户明确本仓库只以 Web/服务端接口可用与功能完整为完成边界；原计划把后续 sibling 拥有的浏览器、Android、PDF、跨 Mongo 拓扑和交付故障注入反向列为本 leaf 门禁，形成不可关闭的循环依赖。以下执行清单已按现有任务 DAG 收敛，历史 checkpoint 保留为过程证据。

## Phase 0：规格与激活门

- [x] 按父任务轨道 `identity → notes → content → publishing → admin` 选择第一个仍为 `planning` 的 ready 叶 `09-08-application-notes`；已处于 `in_progress` 的 identity 不作为待激活候选。
- [x] 核对 `09-08-domain-contracts` 与 `09-08-infrastructure-persistence` 已归档 `completed`；将两者写入 `meta.depends_on`。归档状态不把其 `partial` 真实证据升级为通过。
- [x] 读取父任务、domain/persistence 交接、backend spec、当前 service/controller 和 Golden/USN/note-save 证据。
- [x] 写入 `research/spec-audit-2026-09-11.md`，建立 actor/owner、跨域边界、D-06、multi-write、History/Suggestion 和验收缺口清单。
- [x] 重写 PRD/design/implement，建立 `research/action-inventory.md` 与初始 `acceptance/evidence-matrix.md`；本轮仅修改任务材料。
- [x] 用户确认原 Q-N1～Q-N5 全部采用推荐方案。
- [x] 完成首次 PRD convergence pass：将决定写入 requirements/design/acceptance，并以 KD-N1～KD-N5 记录正式约束。
- [x] `implement.jsonl`/`check.jsonl` 均含 9 条真实 spec/research 条目；`task.py validate`、`git diff --check`、JSONL/路径/未决项检查均通过。
- [x] 向用户呈现更新后的最终规划摘要，并在后续消息取得明确实现批准。
- [x] 仅在上述门禁闭合后运行 `python ./.trellis/scripts/task.py start 09-08-application-notes`。

### Implementation-discovered planning gate

- [x] 通过 controller、service 与第一方 Web 调用确认：`UpdateNoteOrContent`、copy/shared-copy、delete/move batch 均无客户端 operation ID，Web update 无 expected USN。
- [x] 将兼容方案、旧客户端行为、跨任务归属和拒绝方案的影响写入 PRD Q-N9、design 与 evidence matrix；本轮只修改任务规格/研究/验收材料。
- [x] 用户于 2026-09-12 确认采用 Q-N9 推荐方案；原 KD-N1～KD-N5 与先前实现批准不替代该决定。
- [x] 将 Q-N9 固化为 KD-N6，并把约束无损并入 R-N3/R-N6/R-N8/R-N9、AC-N6/AC-N9、action inventory 和实施步骤；字段可选、旧客户端兼容和 `RetrySafe=false` 边界均已记录。
- [x] 重新执行 PRD convergence、Trellis validate 与规格 diff 检查，并呈现新的最终规划摘要；本轮不恢复业务实现。
- [x] 用户于 2026-09-12 以“采用推荐”确认新最终规划摘要；已恢复 Phase 2 执行，未改变 KD-N6 的可选字段与旧客户端兼容边界。

## Phase 1：Web 服务端契约与持久化

- [x] action inventory 覆盖 38 个可调用 Web/API action，并记录 actor/owner、presence、wire mapper、写集合、USN 与跨域 handoff。这是 scope completeness，不等于 dynamic coverage completeness。
- [x] `app/application/notes` 提供 typed result、durable receipt/state machine、repository/UOW 与最小跨域 ports，且对 Revel/Mongo/`app/db`/BSON 零依赖。
- [x] atomic USN、note create/update transaction/standalone、history、trash/move/copy/batch、notebook/tag D-01 与 KD-N6 server wiring 已实现并有 focused tests。
- [x] notebook/tag target Golden 与数据库 tombstone/USN/sync 直接断言通过；旧 wire 兼容映射未被 typed error 泄漏破坏。
- [x] API files presence 在单一 adapter 边界区分 absent、disabled marker、present-values 与 present-empty，真实 HTTP 持久化验证附件集合与 `AttachNum` 的 `2 → 2 → 1 → 0`。

## Phase 2：兼容 HTTP 与质量门

```powershell
gofmt -w <本任务修改的 Go 文件>
go test ./app/service ./app/db ./app/controllers ./app/controllers/api -count=1
go vet ./app/service ./app/db ./app/controllers ./app/controllers/api
npm test -- --test-name-pattern "note save"
python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-notes
git diff --check
```

- [x] 聚焦 Go 测试设置 60 秒超时；完整真实 HTTP harness 使用独立 10 分钟上限并在约 90 秒内完成。
- [x] 完整 harness 在固定摘要 Mongo 8.0.29 standalone 上 discovered 105、passed 103、failed 0、skipped 2；两个 skip 分别属于 delivery/content 的 Linux PDF artifact 和显式 opt-in 的冗余 login smoke。该统计不代表 38-action 覆盖；逐 action 核对为 21/38，剩余 17 个 Web action 已写入 delivery 接收清单。
- [x] 五个聚焦 Go 包、`go vet`（含 harness）、`go build ./...`、Node note-save、Trellis validate 与 diff hygiene 通过。
- [x] 多轮 review 已识别并修复 projection retry/no-op receipt、mutation lookup 吞错、receipt 第二状态机入口、terminal result 清理和测试误报空间；本轮修复及复验见文末 2026-09-15 checkpoints。Session fixture 只删除一条明确重复记录，生产唯一索引与 fail-closed preflight 保持不变。

## Phase 3：跨任务交接（不反向阻塞本 leaf）

- Mongo 7 standalone、Mongo 8 replica-set、进程 kill/restart、跨主机文件系统 failpoint 与受保护 runner：交给 `infrastructure-persistence` / `delivery-verification`。
- 第一方 Web `OperationId`/`ExpectedUsn` 生成复用、未编辑/编辑真实浏览器行为：交给 `presentation-frontend` / `delivery-verification`。
- attachment/image/PDF provider 与 Linux ExportPdf artifact：交给 `application-content` / `delivery-verification`。
- 分享/博客 projection provider：交给 `application-publishing`；完整 route/runtime 迁移：交给 `interface-http`。
- Android marker、设备、APK、签名和 E2E：按用户决定延后到 Android sibling 重构。

## Completion gate

KD-N1～KD-N6 的 Web 服务端实现与当前 Mongo 8 standalone 证据已完成；本 leaf 的 AC-N1 inventory/changed-mutation 覆盖已通过。逐 action 动态映射仍为 21/38，`delivery-verification` 已接收剩余 17 个 Web action 并保持 `delegated-unrun`。sibling 拥有的跨拓扑、浏览器、PDF、发布、Android 和进程/文件系统故障注入继续保持 `partial`/`blocked`，不反向阻塞 `application-notes`。Phase 3.3 的 required `trellis-update-spec` 判断与记录已于 2026-09-13 完成：`.trellis/spec/backend/database-guidelines.md` 新增 durable standalone mutation receipt 契约。本 leaf 已可进入 Phase 3.4；commit 与 archive 仍需分别获得用户授权。本任务 `branch == base_branch == dev`；若后续获得归档授权，必须在确认单分支条件后显式使用 `task.py archive --skip-branch-validation`，不得将该参数常规化到其他任务。

## 2026-09-11 implementation checkpoint

已落地 atomic USN、note-specific transaction/compensation runner、Web/API combined save、D-01 notebook/tag tombstone、before-image history、typed Web batch result、owner-scoped notebook/tag/move 写和 drag 基础校验。完整状态与命令见 `acceptance/evidence-matrix.md`。

本 checkpoint 不关闭 Phase 2～8：standalone durable receipt/跨进程 unknown-result、全部 repository/port 去 BSON 边界、逐 action Golden、Mongo 7 standalone、Mongo 8 replica-set、真实 HTTP 与浏览器证据仍未完成。禁止据此执行 finish/archive。

## 2026-09-11 durable-operation checkpoint

- 新增纯 `app/application/notes` 契约层：typed category/result、`WorkspaceRepository`、`WorkspaceUnitOfWork`、permission/content/publishing ports，以及不依赖 Revel/Mongo/BSON 的 durable operation state machine。
- 新增 `workspace_operations` Mongo adapter、startup indexes、lease + version CAS、input digest 冲突检测、查询/claim/save API。standalone 只在明确 transaction unsupported 后进入该路径；transient transaction error 不降级。
- note create/save 使用稳定语义 identity；transaction callback 内把 committed receipt 与业务写一并提交，standalone receipt 持久化首次 before/desired state、assigned USN、current/applied/failed step。未知步骤结果在重试时先 Verify；USN 允许 gap 且从不回拨。
- note create/save 的 tag/count/image/share projection 使用独立 durable repair receipt；主 mutation 与 repair 失败继续分开报告。
- focused application/Mongo/service/controller tests 通过，但 receipt 尚未接入 Web tag detach、notebook drag、trash cleanup、copy/shared-copy 与 batch orchestration，Phase 2～8 仍不得关闭。

## 2026-09-11 durable multi-write checkpoint

- receipt store 的 Get/Claim/Save 均显式携带 owner；standalone runner 区分 compensation 与 durable repair，unknown provider 只有 Verify 成功或 `ReplaySafe=true` 才允许继续。
- note projection 的 tag/notebook count/image/share provider 与 permanent-delete 的 attachment/content/history/count/tag cleanup 都具备 error-returning Apply 和 final-state Verify；附件删除可重复，tag recount 不会复活已 tombstone tag。
- Web tag detach、notebook drag/sort、trash/permanent-delete 已接 stable durable receipt；focused runner/Mongo tests覆盖 owner isolation、partial resume、unknown-result Verify 和已完成 USN step 不重复。
- 删除两个无调用的导出 `bson.M` workspace mutation 入口；`app/application/notes` 仍保持无 Revel/Mongo/BSON 依赖。
- copy/shared-copy 与 outer batch 的公开请求尚未接入 KD-N6 的可选 client operation generation，当前不宣称 `RetrySafe`；完成字段、receipt 和前端生成/复用的实现及证据前，不得用永久 hash 折叠请求来伪造完成。
- Phase 2～8 仍未关闭：完整 controller/repository 迁移、逐 action Golden、Mongo 7 standalone、Mongo 8 replica-set、真实 HTTP、浏览器与 copy/batch idempotency contract 均需后续证据或协议输入。

## 2026-09-12 review-fix quality run

- Candidate：未提交工作树；本轮继续保留全部既有用户改动，未执行 commit/archive/push。
- 修复 API 上传失败后的文件/资产清理：稳定资产仅在 owner/resource 行和物理文件均可验证时重用；数据库状态未知时返回 `partial_write`；创建失败清理先确认 note 未提交，图片已有其他 note 引用时不删除；清理路径限制在 `revel.BasePath` 内。
- 修复 notebook sort/drag receipt 身份：operation identity 直接使用请求 notebook ID；receipt 查询非 not-found 错误 fail closed；drag receipt 冻结并校验父 notebook 的 owner、active 状态和 USN。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0；同范围 `go vet` exit 0；`go build ./...` exit 0；`task.py validate` 9 implement + 9 check exit 0；`git diff --check` exit 0。
- `npm test -- --test-name-pattern "note save"` 超过 60 秒未完成后中断；本轮未修改前端，不能替代真实浏览器证据。
- 仍未运行并保持 `blocked`/`unrun`：Mongo 7 standalone、Mongo 8 replica-set、kill/restart、真实 HTTP Golden、浏览器、failpoint，以及 KD-N6 copy/shared-copy/outer-batch 的完整客户端 operation generation。

## 2026-09-12 re-review fix round

- API update 让 `SaveNote` 作为唯一 receipt-first 冲突边界；显式空 `Files` 也触发 reconcile。API create 在建立 pending receipt 前读取上传字节摘要；相同 client `NoteId` 的不同字节由同一 operation scope 产生 digest conflict。上传失败后清理资产并关闭 pre-upload receipt 为 terminal failed。
- Web attachment add/delete 改为 note-specific `SaveNote` lease/CAS + 可验证 projection，避免先改变附件再忽略 note USN 失败；失败映射为 `partial_write`。
- tag detach 使用当前 tag generation 区分重新激活后的新删除，并从终态 receipt 的非正文 `StepUSNs` 恢复原始逐 note 返回映射；notebook sort/drag 对已提交 receipt 校验最终 generation 后才决定重放/新身份。
- Web delete/move/copy/shared-copy 从 `OperationId` 兼容参数读取；batch 为每个 item 生成稳定子 operation，copy 有 generation 时固定目标 NoteId。共享 copy 的图像/附件复制仍未达到跨进程可查询去重，保留为未闭合风险。
- 本轮验证：5 个聚焦 Go 包测试通过；同范围 vet/build 通过；Trellis 9+9 context validate 通过；`git diff --check` 无 diff 错误（仅 CRLF 转换提示）。真实 Mongo 拓扑、HTTP/browser、failpoint、Node note-save 仍未运行，任务不进入 finish/archive。

## 2026-09-12 review-findings repair checkpoint

- trash 新永久删除 intent 仅在找不到旧 `note_save` receipt 时继续；committed 软删除重放与存储故障分别返回原结果/fail closed。
- receipt terminal `ResultState` 只保留重放响应所需的数据，不保存 note content/token/secret；batch committed retry 恢复冻结逐项结果与 copy 原始 response，不重新执行业务 mutation。
- Web attachment upload 在 side effect 前绑定 bytes + metadata digest，使用 no-clobber publication；outer before-state 冻结原 note generation，Web delete 由 optional `OperationId` 和 before-state 提供 partial-write 恢复入口。
- shared-copy 图片/附件采用稳定 destination asset identity/path 和 durable receipt；最终 step 的 Apply/Verify 同时证明文件摘要与数据库 row/projection，未知结果不做无条件文件清理。
- focused Go tests、同范围 vet、build、Node note-save contract、Trellis validate 与 diff check 已通过；真实 Mongo 7/8、HTTP/browser、kill/restart、跨进程 failpoint 和不同主机文件系统证据仍为 `unrun`/`blocked`。Android 空文件 marker 的客户端/设备证据已延后到 sibling 重构任务，不再阻塞本 Web 项目。本 checkpoint 不触发 finish/archive/commit。

## 2026-09-13 API files-presence checkpoint

- API `UpdateNote` 的文件集合 presence 已收敛到单一 adapter helper，并通过聚焦单测覆盖 absent、严格值为 `1` 的新/兼容 marker、disabled marker、`Files[...]`、非空解码集合和孤立 `FileDatas[...]`。字段缺失不构造 `AssetMutation`，显式空 marker 构造空集合 reconcile，非空集合执行完整替换。
- 新增真实 Revel HTTP + Mongo 持久化回归：完全缺失 files 字段保留 2 个附件，disabled marker + 孤立 `FileDatas[...]` 继续保留 2 个，present-values 精确替换为指定附件 1 个，marker-present-empty 最终清空为 0；每步同时校验附件 ID 集合与 `Note.AttachNum`。fixture 中两个 2014 年遗留匿名 Session 的相同非空 `SessionId` 属于明确坏数据；显式保留较早记录并移除较晚重复记录，新增直接 BSON uniqueness 回归，生产唯一索引与 fail-closed preflight 均未弱化。
- 固定摘要 Mongo 8.0.29 standalone 的 fixture restore、Revel 启动、admin 登录及 files-presence HTTP 持久化场景 executed 1 / passed 1 / failed 0；focused controller/API 与五个服务端包测试、harness compile-only、同范围 `go vet`（含 harness）、`go build ./...` 及 Node note-save contract 1/1 均通过。Android marker、设备、APK、签名和 E2E 不属于本 Web 项目门禁；该 checkpoint 当时尚未完成完整 HTTP replay，Mongo 跨拓扑与 failpoint/browser 后续按 closure ownership correction 移交 sibling。

## 2026-09-13 full HTTP/Golden replay checkpoint

- Session fixture blocker 解除后完整 harness 首次执行到所有场景，并据真实 RED 修复四项契约漂移：Web save 的 `not_found → notExists`/`unauthorized → noAuth` mapper、identity P-06 的 missing/invalid token 幂等 logout Golden/断言、四份 D-01 notebook/tag delete+sync target Golden，以及 `GetActiveNotebooks` 的 `Usn,_id` 稳定顺序。内部 typed error、生产唯一索引和 fail-closed preflight 未弱化。
- D-01 record/replay 仅导致 notebook delete 后续 USN 统一增加 1，更新 5 份 API 与 3 份 USN 既有 Golden；无其他快照漂移。完整 harness package exit 0（92.130s）；JSON event run discovered 105、passed 103、failed 0、skipped 2，跳过 ExportPdf 受审 Linux Golden和显式环境变量控制的冗余 login smoke。最终检查补强 notebook/tag 持久化 `IsDeleted=true` + 分配 USN 直接断言后，完整 harness 再次 exit 0（87.449s）。
- 五个聚焦服务端包、fixture/files-presence harness、同范围 vet、build、Node note-save、Trellis validate 与 diff check 均通过。该 checkpoint 原先把 Mongo 跨拓扑、kill/restart/failpoint、浏览器和 ExportPdf 记作本 leaf 阻塞；2026-09-13 closure ownership correction 已将这些证据恢复到其 sibling owner，Android 证据继续按用户决定延后。

## 2026-09-13 final trellis-check repair

- 修复 drag committed replay：父 notebook generation 现在作为 durable `parent` guard step 冻结并校验，响应丢失后的同一 `OperationId` 重试不再因缺少 `StepUSNs["parent"]` 固定失败。
- 新增 sort/drag 的真实 Mongo response-loss 与 partial-resume 回归，并补同一 operation ID 不同输入产生 digest conflict 的 identity 断言；四条 Mongo 用例在自带 Mongo 8 standalone 容器上 passed 4 / failed 0。
- files-presence harness 的唯一临时目录及 bounded empty-directory cleanup、Session fixture 的 26 条总数/保留记录/删除记录/非空 SessionId 唯一性回归均通过。三个接收 sibling 已将 `09-08-application-notes` 加入各自的 `meta.depends_on`，相关六个任务 validate 均通过且无依赖环。
- 最终候选再次通过五个聚焦 Go 包、`go vet`（含 harness）、`go build ./...`、Node note-save、完整 HTTP/Golden harness（89.280s）、gofmt 与 diff hygiene。本 leaf 的 AC-N1 已通过；delivery-owned 动态映射仍为 21/38 `delegated-unrun`，跨拓扑、浏览器、PDF 与 failpoint 也仍为 receiving sibling 的 delegated-unrun，不被本轮结果升级。

## 2026-09-15 final trellis-check repair

- mutation 专用 note/notebook/tag/trash lookup 改为显式传播 Mongo 错误；Web notebook delete 的 child/note count 失败不再被当成零后继续删除，reparent/drag cycle walk 遇存储错误 fail closed。
- API create 的 pre-upload receipt Begin/Fail transition 从 controller 收敛到 notes service；failed/compensated receipt 清除已捕获的 `ResultState`，committed replay 所需结果保持不变。新增未初始化存储与 terminal redaction 回归。
- 最终候选通过五个聚焦 Go 包、`go vet`（含 harness）、`go build ./...`、Node note-save、纯 application dependency scan、gofmt、完整 HTTP/Golden harness（80.719s）与六个相关 Trellis task validate。该普通 harness run 只证明 package exit 0，不重新声称 105/103 事件统计或扩大 21/38 action 映射。
- 17 个 Web action、Mongo 7/replica-set、kill/restart/failpoint、真实浏览器、跨主机文件系统、PDF 和 Android 证据继续在 receiving sibling 保持 `delegated-unrun`；本轮未执行 commit/archive/push。

## 2026-09-15 diff-review findings repair

- 修复 copy/shared-copy 已提交主 receipt 跳过 pending projection repair，以及 client `OperationId` no-op 不建 receipt 的 durable replay 缺口；同 ID/同输入恢复原 USN，同 ID/不同输入 conflict，no-op 不分配 USN。
- 收紧 stable shared-copy：根 receipt 在 destination create 前冻结 source image/attachment identity manifest，重试从 receipt 与已提交 destination content snapshot 恢复，不能吸收后来新增的源资产。无 `OperationId` 的 legacy 路径恢复先复制资产、再单次 `AddNoteAndContent`；pre-create attachment 不再误入要求目标 note 已存在的 Web mutation，保持附件集合、一次 USN 与历史错误行为。
- 补齐 operation/source/owner 与 asset-ID identity 隔离、真实 base 外文件 containment、Mongo fixture 单 client/可见 cleanup、copy receipt terminal/digest、projection repair、no-op USN、legacy USN 和 frozen manifest 回归。规格/验收按 21/38 mapped subset、精确 ownership negative subset 和 receiving sibling `delegated-unrun` 边界同步。
- 当前 JSON 证据：五个聚焦 Go 包 discovered/executed 288、passed 283、failed 0、skipped 5；完整 HTTP/Golden harness discovered/executed 108、passed 106、failed 0、skipped 2。`go vet`（含 harness）、`go build ./...`、Node note-save、gofmt、application dependency scan、六个 task validate、依赖图与 diff hygiene 均通过。精确候选、命令、时间、拓扑和 skip 见 `acceptance/evidence-matrix.md`。
- 任务继续保持 `in_progress`；未执行 commit/archive/push。17 个 Web action及 Mongo 跨拓扑、kill/restart/failpoint、真实浏览器、PDF、跨主机文件系统与 Android 证据未被本轮升级。
