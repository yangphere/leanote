# 应用层：笔记工作区与同步 — 验收证据矩阵

本矩阵在 2026-09-11 规格审核中建立，并在 2026-09-13 按用户确认的 Web 项目边界完成闭环修正。当前 leaf 以 Web/服务端接口和功能契约为完成范围：固定摘要 Mongo 8 standalone 的真实 HTTP/Golden/USN/files-presence 是 live gate；Mongo 跨拓扑、进程/文件系统 failpoint、浏览器、PDF、发布和 Android 证据由已有 sibling 任务拥有。未运行证据仍在对应任务保持 `blocked`/`unrun`，但不得形成对本 leaf 的反向依赖。

实现期发现的协议问题 Q-N9 已于 2026-09-12 按用户确认固化为 KD-N6：Web `UpdateNoteOrContent`、copy/shared-copy、delete/move batch 接受可选客户端 operation ID，Web update 另接受可选 expected USN。服务端有/无 generation 分支的 contract 已实现；第一方 Web 生成/复用和真实 unknown-result 证据由 presentation/delivery 保持 `delegated-unrun`。

| ID | 可观察断言 | 证据入口 / 命令 | 当前状态 |
| --- | --- | --- | --- |
| AC-N1 | action inventory 完整，且本任务改变的 mutation 有可追溯 contract/fixture | `research/action-inventory.md` §10 changed-mutation crosswalk、focused tests、完整 HTTP harness | leaf contract passed / delivery delegated-unrun：38 个 callable action 已登记，WN-08/WN-09/WN-11/WN-12/WN-13 的具名 focused contract 已映射；live harness 明确映射 21/38（17 API + 4 Web）。剩余 17 个 Web action 由 delivery 逐项补齐；105/103 是 test event 数，不是 action 覆盖数 |
| AC-N2 | 纯 notes application package 与 controller/service/db 边界符合仓库结构 | `go list -deps`、静态 collection/client 与公开签名扫描 | passed：`app/application/notes` 对 Revel/Mongo/`app/db`/BSON 零依赖，公开 service mutation 签名不暴露 `bson.M`；notes controller 未直接操作 collection/client。兼容 adapter 的 ObjectID/BSON 转换及 `app/service → app/db` 是允许边界 |
| AC-N3 | private/shared read/write 执行 owner+actor 规则且不泄露数据 | owner-scoped service/DB tests；`TestGoldenWebOwnershipControllers`；`TestGoldenAPIActions` foreign-read case；`TestUpdateNoteOrContentNewPermissionFailureUsesEnvelope` | leaf server subset passed / delivery delegated-unrun：当前负向 HTTP 证据只覆盖 foreign API note read 与 unauthorized shared create；其余 private/shared read/write action-level negative matrix 未运行，不得据成功查询 Golden 宣称全面 ownership coverage |
| AC-N4 | 原子 USN、owner 隔离和成功 response/document/sync 一致 | `TestAllocateUserUSNIsAtomicPerUser`、receipt owner tests、D-01 DB/sync probes | passed：Mongo 8.0.29 standalone 32 路并发 USN 唯一递增；receipt read/claim/save owner 隔离；D-01 直接验证 response、持久化 tombstone 与 sync USN |
| AC-N5 | create/update 的 transaction/standalone、conflict、retry 与失败结果满足 D-06 | workspace runner/DB tests、note-save HTTP harness | passed：note+content+USN、before-image history、callback bookkeeping、补偿、transient-no-fallback 与失败 envelope 均通过；进程级故障注入已交给 delivery |
| AC-N6 | standalone receipt 可安全续跑且不回拨 USN；KD-N6 有/无 generation 分支明确 | operation state-machine/Mongo CAS/Web adapter tests | passed：owner-scoped receipt、lease/version CAS、Verify-before-replay、terminal result、partial/repair state 和 client-ID digest conflict 有覆盖；旧客户端保持 `RetrySafe=false`。真实 kill/restart 属于 delivery |
| AC-N7 | notebook Web/API delete 使用 new-USN tombstone 并进入 sync | D-01 target Golden + DB tombstone probe | passed：Web guard/API permissive 差异保留，response/document/sync 的 `IsDeleted=true` 与分配 USN 一致 |
| AC-N8 | tag Web/API delete 的 USN/tombstone/sync 与 Web detach 续跑语义正确 | D-01 target Golden + durable detach tests | passed：API tag delete 返回/持久化同一新 USN 并进入 sync；Web detach 冻结 before-image/顺序并保持失败 envelope |
| AC-N9 | trash/move/copy/drag/batch 不吞多写失败；KD-N6 冻结 identity/result | `research/action-inventory.md` §10、service/app/db focused tests + 已映射 HTTP Golden | leaf contract passed / delivery delegated-unrun：server-side mutation 接入 owner receipt；copy/shared-copy 在主 receipt committed 后仍续跑 pending projection receipt；stable shared-copy 重试消费根 receipt 冻结的 source image/attachment manifest，不重新枚举后来新增资产。无 generation 的 legacy shared-copy 保持单次 create/USN。未映射 Web action 的真实 mapper 与 provider/跨主机/跨进程证据交给 content/publishing/delivery |
| AC-N10 | history 保存 before-image、newest-first 严格 10 条且 retry 不重复；Suggestion 有明确 handoff | history tests + action inventory + admin PRD/implement | server passed / delegated-unrun：history contract 已通过；Suggestion 的 anonymous/input/dedup/durable-success/outbox 验收已写入 `application-admin`，但尚未运行 |
| AC-N11 | wire Golden 与 API files 三态持久化保持兼容 | 当前 21/38 action 的 HTTP replay + files-presence HTTP test | leaf mapped subset passed / delivery delegated-unrun：已映射 action 恢复 `notExists`/`noAuth`、幂等 logout、D-01 target 与稳定 notebook 顺序；附件关系/`AttachNum` 完成 `2 → 2 → 1 → 0` 精确断言。其余 17 个 Web action 的 status/Content-Type/body key/order/nil-empty/message 尚无逐 action live 证据；ExportPdf 属于 content/delivery |
| AC-N12 | 当前支持的 Web 服务端真实运行证据可复核 | 固定摘要 Mongo 8.0.29 standalone + Revel HTTP harness | server passed / delegated-unrun：当前 2026-09-15 JSON event run discovered/executed 108、passed 106、failed 0、skipped 2；skip 为 sibling-owned PDF artifact 与显式 opt-in 的冗余 login smoke。负向 ownership live 证据限于 foreign API note read 与 unauthorized shared create；其余 action-level matrix、Mongo 7 standalone、Mongo 8 replica-set、kill/restart/failpoint、浏览器和 PDF 仍在 sibling 未运行。Android 按用户决定延期且不阻塞本 Web 项目 |

## Closure ownership correction (2026-09-13)

原矩阵把 `interface-http`、`presentation-frontend`、`application-content`、`application-publishing` 和 `delivery-verification` 的后续验收反向设为 `application-notes` 完成条件，但这些任务的 `depends_on` 又包含或位于 notes 之后，形成不可关闭的依赖环。修复后的规则是：本 leaf 必须输出稳定 contract、server adapter 和当前 Mongo 8 standalone 真实 HTTP 证据；consumer/provider、全 runtime、浏览器、PDF、跨拓扑和故障注入在其 owner 任务验收。交接证据不得丢失；如 delivery 逐 action 回放发现属于 notes 的服务端缺陷，必须重开本 leaf，不得以“已交接”拒绝修复。

## Evidence recording contract

每次验证记录以下字段，不以“命令曾运行”代替结果：

- candidate commit、工作树状态；
- 命令、开始/结束时间、退出码；
- discovery / executed / passed / failed / skipped 数量；
- Mongo 版本、standalone/replica-set 拓扑、fixture restore 身份；
- HTTP server 入口与 Golden mode；
- failpoint/故障注入名称及最终 DB state；
- 脱敏失败摘要、blocked 原因和恢复条件。

## 2026-09-11 implementation run

- Candidate：工作树，未提交；Mongo `8.0.29` standalone（`db.hello()` 无 `setName`），不是 AC-N12 指定拓扑。
- `go test -json ./app/db -run 'Test(AllocateUserUSNIsAtomicPerUser|ExecuteWorkspaceMutation)' -count=1 -timeout 60s`：discovered 4，executed 4，passed 4，failed 0，skipped 0；包含真实 Mongo 32 路 atomic allocator 与 3 个 runner contract。
- `go test ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：discovered/executed 以 Go package summary 记录，4 packages passed，failed 0。
- `go vet ./app/db ./app/service ./app/controllers ./app/controllers/api`：exit 0；`go build ./...`：exit 0。
- `npm test -- --test-name-pattern "note save"`：Node runner 实际发现 132，passed 131，failed 0，skipped 1；由于仓库脚本参数位置，runner 同时执行了其他 JS contracts。该结果不替代 browser evidence。
- `LEANOTE_REQUIRE_MONGO=1 LEANOTE_TEST_MONGO_URL=mongodb://127.0.0.1:27017/leanote_test LEANOTE_GOLDEN=replay go test ./app/tests/harness -run <five note-save cases> -count=1 -timeout 5m`：discovered 5，executed 5，passed 0，failed 5（均在 environment setup）；原因是 host `mongorestore` 不在 PATH。恢复条件：安装/提供受支持的 host `mongorestore` 后重跑。
- `python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-notes`：9 implement + 9 check contexts，exit 0；`git diff --check`：exit 0。

## 2026-09-11 durable-operation run

- Candidate：未提交工作树；当前 Mongo 为 `8.0.29` standalone，仅作为补充证据。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0。
- `go test ./app/db -run 'Test(MongoWorkspaceOperationStore|RunWorkspaceMutationCommitsDurableReceipt)' -count=1 -timeout 60s`：3 focused Mongo receipt/UOW tests passed，覆盖持久化、lease、CAS、首次 desired state、digest conflict，以及业务写/USN/committed receipt 的同边界与重复调用不重写。
- `go list -deps ./app/application/notes | Select-String 'mongo-driver|revel'`：0 matches；完整 `app/service` 去驱动依赖门禁仍未通过。
- 尚未运行：Mongo 7 standalone、Mongo 8 replica-set、process kill/restart、真实 HTTP Golden 与浏览器；不得用本节结果替代 AC-N12。

## 2026-09-11 durable-operation review run

- Candidate：未提交工作树；review 修复了 apply unknown-result 先 Verify、receipt 保存失败保留本地恢复状态、committed receipt 不可改写、持久化错误脱敏、业务写前 receipt 保存失败不误报 partial，以及 create 已有 partial receipt 时继续恢复。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0；新增状态机与 Mongo CAS 回归均通过。
- `go vet ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api ./app/tests/harness/...`：exit 0；`go build ./...`：exit 0。
- `npm test -- --test-name-pattern "note save"`：runner 实际 discovered 132，passed 131，failed 0，skipped 1；该命令因参数位置运行完整 JS contracts，不替代浏览器证据。
- `python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-notes`：9 implement + 9 check contexts，exit 0；`git diff --check`：exit 0。
- 该 checkpoint 当时未闭合：projection provider 的 Verify、KD-N6 server wiring 和真实 HTTP 尚未完成；后续 checkpoint 已修复 server 范围。旧 service 的 DB/BSON 依赖后来按仓库允许的 `app/service → app/db` 边界澄清，Mongo 跨拓扑、kill/restart 和浏览器证据移交 sibling。

## 2026-09-11 durable multi-write implementation run

- Candidate：未提交工作树；未执行 commit/archive/push。
- receipt read/claim/save 已显式按 `(owner, operation)` 隔离；无 expected-USN 的 Web save identity 纳入当前 note generation，避免后续请求命中陈旧 committed receipt。
- durable state machine 新增 `repair_pending` 与显式 failure policy；focused tests 覆盖不可盲重放 provider、Verify 闭合 unknown result、replay-safe resume 不重复已完成 USN step，以及 owner-scoped Mongo CAS。
- note create/save 的 tag、notebook count、image index、share cleanup 已改为 error-returning final-state projection 并提供真实 Verify；permanent-delete 的附件/content/history/count/tag cleanup 同样可验证，且 tag recount 不再调用会复活 tombstone 的 add/reactivate 路径。
- tag detach、notebook drag/sort、trash/permanent-delete 已接 durable receipt。copy/shared-copy 与 outer batch 尚未接入 KD-N6 的可选 client operation generation；当前结果明确 `RetrySafe=false`，不以永久 request hash 改变“用户可主动重复复制/删除”的既有语义，待字段和 receipt wiring 完成后再验收有 generation 的路径。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0；包含新增 repair replay/Verify 与 failure-policy forwarding 回归。
- `go vet ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api`：exit 0；`go build ./...`：exit 0。
- `go list -deps ./app/application/notes | Select-String 'mongo-driver|revel'`：0 matches；导出 service/application `bson.M` mutation 签名扫描：0 matches。
- `python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-notes`：9 implement + 9 check contexts，exit 0；`git diff --check`：exit 0（仅 Git 的 CRLF conversion warnings）。

## 2026-09-11 third-round review run

- Candidate：未提交工作树。review 将 pending/Verify-error 的 unknown current step 强制标为 partial，禁止 durable `CurrentStep` 从重建后的 plan 消失后仍提交成功，并把 note/content compensation 收紧为 desired-state CAS，避免覆盖同资源后续写。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0；新增 changed-plan、unknown partial 与 compensation fencing 回归通过。
- `go vet ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api ./app/tests/harness/...` 与 `go build ./...`：exit 0；纯 `app/application/notes` dependency scan 对 Mongo/Revel 为 0 matches。
- `npm test -- --test-name-pattern "note save"`：runner 实际 discovered 132，passed 131，failed 0，skipped 1；参数位置导致执行完整 JS contracts，不替代真实浏览器证据。`task.py validate`：9 implement + 9 check，exit 0；`git diff --check`：exit 0。
- owner-scoped receipt 的 Mongo filter 已覆盖 Begin duplicate read、Get、Claim 与 Save；错误 owner 的 Get/Claim 不返回 receipt，committed receipt 不可改写。
- 未闭合：tag detach 的初始 note step 集合及 drag/sort 的 before generations 尚未冻结到 receipt 并在跨进程续跑时恢复；当前状态机对此类缺失 current step 会 fail closed 为 `repair_pending`。KD-N6 的 copy/shared-copy、outer batch、Web save optional generation wiring 与 unknown retry 仍待实现；完整 application/controller repository 边界与真实环境证据仍未闭合。

## Baseline versus target rules

- `app/tests/harness/usn_test.go` 中 notebook delete no-bump/sync-empty 与 tag delete stale stored USN/sync-empty 是 D-01 的 known-defect before evidence，不是目标通过条件。
- `app/tests/harness/note_save_contract_test.go` 当前只证明 partial write 不再返回成功；它没有证明 metadata/content/USN 已原子回滚。
- `tests/js/note-save-contract.test.js` 是静态前端保存状态 contract，不替代真实浏览器未编辑/编辑流程。
- 归档 domain/persistence 的 `completed` 表示各自任务生命周期结束；其 `partial`/`unknown` 证据不会自动升级为任何 sibling 的通过，也不反向阻塞已经由当前 leaf 直接证明的 contract。
- mock/service fake 只用于错误路径与编排；本 leaf 的真实 HTTP 已在 Mongo 8 standalone 执行。Mongo 7/replica-set/browser/failpoint 继续在 delivery/presentation/persistence 任务保持 `blocked`/`unrun`。

## 2026-09-12 review-fix quality run

- Candidate：未提交工作树；未执行 commit/archive/push。
- API asset cleanup 与 notebook sort/drag receipt/frozen-parent 修复已通过相关 service/controller 测试；`go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0；同范围 `go vet` 与 `go build ./...` 均 exit 0。
- `task.py validate`：9 implement + 9 check，exit 0；`git diff --check`：exit 0。`npm test -- --test-name-pattern "note save"` 超过 60 秒未完成后中断；未修改前端，浏览器证据仍为 `blocked`。
- Mongo 7 standalone、Mongo 8 replica-set、kill/restart、真实 HTTP Golden、failpoint，以及 KD-N6 copy/shared-copy/batch operation generation 仍为 `unrun`/`blocked`。

## Planning gate

KD-N1～KD-N6 已确认，Q-N9 已作为 KD-N6 固化。2026-09-13 的 closure ownership 修正消除了 sibling 反向门禁；server 范围已有实现和证据，本 leaf 的 AC-N1 inventory/changed-mutation 覆盖已通过。delivery-owned 逐 action 动态映射仍为 21/38 `delegated-unrun`。Phase 3.3 required `trellis-update-spec` 判断/记录已于 2026-09-13 完成，durable standalone mutation receipt 契约已写入 backend database guideline；本 leaf 可进入 Phase 3.4，但 commit/archive 仍需分别获得用户授权。若后续获得归档授权，当前 `branch == base_branch == dev` 需显式 `--skip-branch-validation`。

## 2026-09-12 re-review fix round

- Candidate：未提交工作树；未执行 commit/archive/push。
- API update 已移除 `SaveNote` 前的 USN 早期冲突返回，receipt-first committed replay 现在可达；显式存在的空 `Files` 也会进入 attachment reconcile。
- API create 的 stable `NoteId` receipt digest 已包含每个上传 part 的 SHA-256；相同 operation 搭配不同字节会冲突。上传失败会清理资产并尝试把 pre-upload receipt 关闭为 terminal `failed`，使 terminal TTL 生效。
- Web attachment add/delete 已改为通过 note-specific `SaveNote` lease/CAS，再执行附件 row/file/AttachNum projection；失败返回 `partial_write`，不再忽略 USN/CAS 失败。
- Web tag detach、notebook sort/drag 增加资源 generation 判断；已提交 tag receipt 保留非正文的逐 note USN，以恢复原始 `tag -> USN` 响应，不复用已重新激活资源的旧 committed receipt。
- Web delete/move/copy/shared-copy 与 batch adapter 已读取可选 `OperationId`；batch 为每个 item 固定子 operation identity，copy 为有 generation 的请求固定目标 NoteId。该 checkpoint 当时尚缺共享 copy provider 与跨进程 failpoint 证据；后续 server-side Apply/Verify 已补齐，跨进程证据移交 content/delivery。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed，failed 0；同范围 `go vet` 与 `go build ./...` exit 0；`task.py validate`：9 implement + 9 check；`git diff --check`：无 diff 错误，仅 CRLF conversion warnings。
- 该 checkpoint 当时的 Mongo 跨拓扑、kill/restart、真实 HTTP、浏览器、跨进程/failpoint 与 Node note-save 尚为 `unrun`/`blocked`；后续已补齐本 leaf 的 HTTP/Node 证据，其余已按 closure ownership correction 移交 sibling。

## 2026-09-12 review-findings repair round

- Candidate：未提交工作树；未执行 commit/archive/push，任务继续保持 `in_progress`。
- 修复 trash 的新永久删除意图区分：仅同资源 committed `note_save` receipt 作为软删除重放；receipt not-found 继续 permanent delete，其他存储错误 fail closed。
- durable receipt 新增 terminal `ResultState`；Web batch 冻结逐项结果和 copy 的原始 metadata-only response，committed retry 不再调用子 mutation 或读取后来被修改的 destination note。
- Web attachment upload 在文件发布前 claim 绑定 bytes/title/type/size 的 receipt，使用同目录 pending file + no-clobber link；outer before-state 冻结首次 note generation，使 inner note/attachment partial retry 不使用已变化的当前 USN。Web delete 接受 optional `OperationId` 并由 before-state 恢复已删除的 attachment row/path/note generation。
- shared-copy image/attachment 使用稳定 destination asset ID/path；文件发布与数据库行分别进入 durable step，最终 Apply/Verify 同时要求文件摘要和 row/AttachNum 状态，不再对未知或已被 row 引用的结果无条件删文件。
- 当前自动化：5 个聚焦 Go packages tests 通过；同范围 `go vet`、`go build ./...`、`node --test tests/js/note-save-contract.test.js`、Trellis 9+9 validate 与 `git diff HEAD --check` 均 exit 0（diff check 仅 CRLF conversion warnings）。
- 仍为 `unrun`/`blocked`：Mongo 7 standalone、Mongo 8 replica-set、kill/restart、真实 HTTP Golden、浏览器、跨进程/failpoint 与不同主机文件系统原子发布。Android 客户端空 `Files` marker 生成、设备和 E2E 证据已按 2026-09-13 用户决定延后到 sibling 重构任务，不再阻塞本 Web 项目；服务端 files presence 三态契约仍须在本任务验证。

## 2026-09-13 API files-presence run

- Candidate：未提交工作树；未执行 commit/archive/push。Android marker、设备、APK、签名和 E2E 已按用户决定延后到 sibling Android 重构，本轮只验证 Web/服务端。
- `apiNoteFilesPresent` 是 `UpdateNote` 的单一 adapter 判定边界：只有 `FilesPresent=1`/`HasFiles=1`、`Files`/任意 `Files[...]` key 或非空解码集合才触发完整 reconcile；`=0`、字段完全缺失和单独 `FileDatas[...]` 均不触发。
- `go test ./app/controllers/api -run 'TestAPINoteFilesPresence' -count=1 -timeout 60s` 与 controller/api 包级测试通过；新增真实 Revel HTTP + Mongo 持久化用例 `TestAPIUpdateNoteFilesPresenceContract`，先用 disabled markers 与孤立 `FileDatas[...]` 证明 absent 保留 2 个附件，再用 present-values 完整替换为 1 个附件，最后用 marker-present-empty 清空为 0。
- fixture 根因是两个 2014 年遗留匿名 Session 具有相同非空 `SessionId` 和 `UpdatedTime`；显式保留较早记录、移除较晚重复记录，记录总数 `27 → 26`。生产 `sessions_SessionId_unique` 与 fail-closed preflight 未修改；新增 `TestSessionFixtureHasUniqueNonEmptyStringSessionIDs` 直接解析 BSON，防止 fixture 再次引入重复身份且失败日志不输出 SessionId。
- `go test ./app/tests/harness -run '^TestSessionFixtureHasUniqueNonEmptyStringSessionIDs$' -count=1 -timeout 60s`：executed 1、passed 1、failed 0。
- `go test ./app/tests/harness -run '^TestAPIUpdateNoteFilesPresenceContract$' -count=1 -timeout 5m -v`：固定摘要 `mongo:8.0`（实际 v8.0.29）standalone 容器 restore 成功，Revel 启动与 fixture admin 登录成功；executed 1、passed 1、failed 0，四次真实更新的持久化断言完成完全 absent 保留 2、disabled marker + 孤立 `FileDatas[...]` 保留 2、present-values 精确替换为指定附件 1、present-empty 清空为 0，并逐步核对 `Note.AttachNum`。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`：5 packages passed；harness compile-only、`go vet`（含 `./app/tests/harness/...`）与 `go build ./...` exit 0；`node --test tests/js/note-save-contract.test.js`：1 passed、0 failed。

## 2026-09-13 full HTTP/Golden replay

- Candidate：未提交工作树；Mongo 使用固定摘要镜像 `docker.io/library/mongo:8.0@sha256:376f5173003b5408d7b8e6989667231c0bf0cefdce379d7c814910429d1a7a85`，实测 v8.0.29 standalone；每个真实集成用例恢复仓库 fixture 并启动 Revel server。
- 第一次完整 replay 在解除 Session fixture preflight 后暴露 3 项旧门禁：P-06 已批准幂等 logout 与旧 none/invalid Golden 冲突；Web `UpdateNoteOrContent` 将内部 `not_found` 泄漏为公开 Msg；D-01 notebook/tag target Golden 缺失。修复后又通过 replay 发现 `GetActiveNotebooks` 漏掉旧 `Usn` 排序，改为 `Usn,_id` 稳定排序而未用脆弱 Golden 掩盖。
- Web save adapter 将 `WorkspaceNotFound`/`WorkspaceUnauthorized` 映射回既有 `notExists`/`noAuth`；内部 typed error 保持不变。logout 从通用 NOTLOGIN 断言拆出，none/invalid 均显式断言 `Ok:true,Msg:""`。四份 D-01 target Golden 由真实 record 生成并 replay，通过后仅因 notebook delete 新分配一次 USN 而更新其后的 5 份 API 与 3 份 USN 快照；无其他既有 Golden 漂移。
- `go test ./app/tests/harness -count=1 -timeout 10m`：exit 0，package elapsed 92.130s。随后 JSON event run：discovered 105、passed 103、failed 0、skipped 2；跳过 `TestGoldenExportPdf`（缺少受审 Linux `wkhtmltopdf` Golden）和需显式 `LEANOTE_HTTP_INTEGRATION=1` 的冗余 `/login` smoke，其他场景已多次通过同一真实 Revel 启动路径。最终质量复核为 D-01 notebook/tag 增加持久化 `IsDeleted=true` + 分配 USN 直接断言后，完整 harness 再次 exit 0（87.449s）。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`、files-presence/fixture 聚焦 harness、`go vet`（含 harness）、`go build ./...`、Node note-save contract、Trellis 9+9 validate 与 `git diff --check` 均 exit 0。Mongo 7 standalone、Mongo 8 replica-set、kill/restart/failpoint、真实浏览器与 ExportPdf Linux artifact 仍在对应 sibling 任务 open，不再阻塞本 leaf。

## 2026-09-13 final trellis-check repair

- Candidate：未提交工作树；未执行 commit/archive/push。修复 drag committed response-loss retry 缺少父 generation 的问题，receipt 以 `parent` guard step 保留并验证首次 parent USN；同一 client operation 的重试不重复 notebook 写或 USN 分配。
- 自带 Mongo 8 standalone 容器执行 sort/drag response-loss 与 partial-resume：executed 4、passed 4、failed 0、skipped 0。无本地服务时相同测试明确 skip，不把 probe 结果当成通过。
- files-presence 目录安全与 Session BSON 精确边界聚焦测试：executed 4、passed 4、failed 0；最终候选的完整 `go test ./app/tests/harness -count=1 -timeout 10m` exit 0，elapsed 89.280s。
- 五个聚焦 Go 包、同范围 `go vet`（含 harness）、`go build ./...`、Node note-save、六个相关 Trellis task validate、纯 application dependency scan、gofmt 与 `git diff HEAD --check` 均 exit 0。
- Phase 3.3 spec update 已完成，content/admin/presentation 的接收依赖已写入 `meta.depends_on`。本 leaf 的 AC-N1 已通过；delivery-owned 动态覆盖仍为 21/38 delegated-unrun，Mongo 7/replica-set、kill/restart/failpoint、真实浏览器、跨主机文件系统和 PDF artifact 也保持 delegated-unrun。

## 2026-09-15 final trellis-check repair

- Candidate：未提交工作树；未执行 commit/archive/push。修复 mutation lookup 吞掉 Mongo 错误、notebook delete 把 count failure 当零、cycle walk 未 fail closed、controller 直接推进 receipt 状态，以及 failed/compensated receipt 保留 `ResultState`。
- `go test ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s`、`go vet`（含 harness）、`go build ./...`、`node --test tests/js/note-save-contract.test.js`、纯 application dependency scan 与 gofmt 均通过；完整 `go test ./app/tests/harness -count=1 -timeout 10m` exit 0，elapsed 80.719s。
- 本轮普通 harness 未采集新的 JSON discovery 统计，因此只记录 package pass；历史 105/103 仍仅是 test event 数，action live mapping 仍为 21/38，剩余 17 个 Web action 保持 `delegated-unrun`。Mongo 跨拓扑、kill/restart/failpoint、browser/PDF/Android 证据未运行且未升级。

## 2026-09-15 diff-review findings repair candidate

- Candidate：`git tree 731e71796bd394bc19fa4b907ef5deeaf8ba8f2b`；该 tree 在全部实现、测试、规格和任务元数据修复完成后、仅在本行用 tree 值替换 `PENDING` 前由临时 Git index 生成。工作树未提交；未执行 commit/archive/push。
- 实现修复：copy/shared-copy 的 committed-primary 重试继续完成 pending projection receipt；client `OperationId` no-op 建立 committed receipt、重放原 USN、不同输入 conflict 且不分配 USN；无 ID no-op 保持无 receipt。stable shared-copy 根 receipt 冻结并消费有序 source image/attachment manifest 与 destination content snapshot，排除重试前后来新增的源资产；legacy shared-copy 恢复先复制图片/附件，再单次 create/USN，其中 pre-create attachment 路径不再误入要求目标 note 已存在的 Web mutation。mutation lookup/count/cycle 错误 fail closed；API pre-upload receipt transition 收敛到 service；failed/compensated receipt 清除 `ResultState`。
- 测试修复：copied-image identity 分别改变 operation/source/owner；API upload path 分别改变 asset ID；containment 使用真实存在的 base 外普通文件；Mongo service fixture 只初始化一个 client 且 cleanup 错误可见。新增 copy projection、no-op receipt/USN、legacy shared-copy、frozen asset manifest、receipt terminal/digest 和 terminal redaction 回归；frozen attachment 用例在修复前稳定得到 2 个 destination attachments，修复后只得到 receipt 冻结的 1 个；legacy 用例在修复前得到空附件集合，修复后保留 1 个附件且用户 USN 仍只推进一次。
- `2026-09-15T10:21:07.4371280+08:00` 至 `10:21:10.9705029+08:00`：临时 `mongo:8.0.29` standalone、数据库 `notebook_service_receipt_test`；`go test -json ./app/application/notes ./app/db ./app/service ./app/controllers ./app/controllers/api -count=1 -timeout 60s` exit 0，discovered/executed 288、passed 283、failed 0、skipped 5、5 packages passed。skips 为 4 个需真实 API server 的 controller/api live cases及 1 个 replica-set 初始化 case；临时容器已删除。
- `2026-09-15T10:21:57.8733233+08:00` 至 `10:23:23.9456497+08:00`：固定摘要 Mongo 8.0.29 standalone、fixture restore、真实 Revel server、Golden replay；`go test -json ./app/tests/harness -count=1 -timeout 10m` exit 0，discovered/executed 108、passed 106、failed 0、skipped 2、package passed 1。skips 精确为 `TestGoldenExportPdf` 与 `TestServerServesLoginOverRealHTTP`；这两个 sibling-owned skip 不扩大本 leaf 的证据边界。
- `2026-09-15T10:21:34.8545337+08:00` 至 `10:21:40.8553563+08:00`：全量改动 Go 文件 `gofmt -l` 无输出；`go vet`（含 harness）、`go build ./...`、`node --test tests/js/note-save-contract.test.js` 均 exit 0（Node executed/passed 1、failed/skipped 0）；纯 `app/application/notes` 对 Mongo/Revel/`app/db` dependency scan 为 0 matches。
- 六个相关 Trellis task validate 全部 exit 0；当前 34 个 active/archive task 节点无 dangling dependency、无依赖环，notes 的两个依赖均为 archived `completed`。`git diff HEAD --check` 与候选 tree 生成后的元数据复核见本记录最终值。
- action live mapping 仍为 21/38；剩余 17 个 Web action、Mongo 7 standalone、Mongo 8 replica-set、kill/restart、Mongo/跨主机文件系统 failpoint、真实浏览器、PDF 和 Android 证据继续在 receiving sibling 保持 `delegated-unrun`，本轮未升级。
