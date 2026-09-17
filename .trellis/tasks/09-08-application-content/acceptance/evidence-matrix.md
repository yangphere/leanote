# Application content evidence matrix

状态：`planned` 未执行；`partial` 只覆盖 required proof 的一部分；`delegated-unrun` 已交接但无运行证据；`passed/failed` 仅在绑定 candidate、命令和 artifact 后填写。

| ID | AC | Required proof | Runner | Status |
| --- | --- | --- | --- | --- |
| E-C01 | AC-C1 | 26 callable + 2 integrations + 4 providers + 4 dormant 静态核对 | spec | partial |
| E-C02 | AC-C2 | application/content dependency scan | local Go | passed |
| E-C03 | AC-C3 | separators/dot/absolute/drive/UNC/device/reserved lexical table | unit | partial |
| E-C04 | AC-C3 | symlink/junction/reparse/deepest parent/root swap；paired quarantine same-filesystem/non-public/no-overlap | Windows+Linux fs | partial |
| E-C05 | AC-C4 | same/different digest、concurrent、fixed-pending regression | fs integration | partial |
| E-C06 | AC-C4 | write/sync/close/no-replace/dir-sync/unsupported fs failpoints | fake+fs | partial |
| E-C07 | AC-C4/10 | file + owner row + projection four-state Verify | Mongo+fs | partial |
| E-C08 | AC-C5 | multipart presence/one-file/stream limit/invalid config | app/controller | partial |
| E-C09 | AC-C5 | DecodeConfig-before-decode、width/height/pixel/allocation/frame budgets、valid/invalid GIF/JPEG/PNG/BMP corpus and cleanup | decoder | partial |
| E-C10 | AC-C6 | image owner/blog/share/foreign/FromFileId/spoof ACL | service/repo | partial |
| E-C11 | AC-C6 | attachment owner/update/read/public/share/foreign ACL | service/repo | partial |
| E-C12 | AC-C6 | invalid ID/open/stat/DB failures, no panic/empty success | negative | partial |
| E-C13 | AC-C7 | album CRUD/default/name/count/cross-owner | Mongo | partial |
| E-C14 | AC-C8 | tar.gz content/entry sanitize/duplicate names | archive unit | partial |
| E-C15 | AC-C8 | write/close failures、concurrency、temp cleanup | archive failpoint | partial |
| E-C16 | AC-C9 | public-only IP/DNS rebinding/redirect/scheme/userinfo；无 allowlist/fallback | remote fake | partial |
| E-C17 | AC-C9 | D-C2：DNS+dial 5s、TLS 5s、header 10s、overall 30s、5 redirects、compressed body `min(uploadImageSize, 32 MiB)`、invalid config fail-closed、non-2xx/invalid image/cleanup | remote integration | partial |
| E-C18 | AC-C10 | Files missing/empty/nonempty Reconcile | notes-content | partial |
| E-C19 | AC-C10 | Copy frozen manifest/stable destination/no new-source retry | notes-content | partial |
| E-C20 | AC-C10 | Delete post-commit repair，不重复/回滚 USN/history；durable quarantine manifest 不进入 notes receipt | notes-content | partial |
| E-C21 | AC-C10 | unknown Apply/Verify，无第二 receipt state | failpoint | partial |
| E-C22 | AC-C11 | self-contained serializer、恶意 HTML stripping、aggregate resource budget、direct argv/metachar/timeout/cancel/executable/input/output cleanup | fake exec | partial |
| E-C23 | AC-C11/14 | local-file deny、zero outbound、file/private/metadata/redirect/CSS/script-fetch negative cases、secret absence in URL/argv/env/log/error/artifact | sandbox/container | delegated-unrun |
| E-C24 | AC-C11 | legacy appKey bind-and-discard、PDF magic/readability/size/name/Content-Type mapper | focused+HTTP | partial |
| E-C25 | AC-C12 | per-action method/presence/status/body/binary Golden | real HTTP | partial |
| E-C26 | AC-C12 | album iframe/paste/upload/download visible behavior | real browser | delegated-unrun |
| E-C27 | AC-C13 | Mongo/file multi-write + durable manifest version/CAS/torn-write + legacy no-OperationID restart + terminal retention/GC recovery | Mongo 8 local fs | partial |
| E-C28 | AC-C13/14 | kill/restart + cross-host/persistent-volume failpoint | delivery | delegated-unrun |
| E-C29 | AC-C14 | Linux/container real wkhtmltopdf success/failure/timeout artifact | delivery | delegated-unrun |
| E-C30 | AC-C14 | non-root writable roots、volume restart、cleanup | delivery | delegated-unrun |
| E-C31 | all | gofmt、focused/full test、scan、vet、build、JS if touched、diff check | local/CI | partial |

## 2026-09-17 bound snapshot

- Candidate：`158a6dee7319d0dd3e09007fd623190525f28977`；审核开始时 `dev`/`origin/dev` 为 `0/0`，worktree clean。
- Environment：Windows `10.0.26200` amd64，`go version go1.27.1 windows/amd64`；module declaration 为 Go 1.26，因此本快照不关闭 Go 1.26 gate。
- Passed commands：
  - `go test -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentpdf`
  - `go test -timeout 60s ./app/service -run 'Test(GetUploadLimitBytes|InitializeContentStore)' -count=1`
  - `go test -timeout 60s ./app/controllers ./app/controllers/api -run 'Test(NoteExportPDF|APIExportPDF|FirstPartyNoteToPDF|RegisterHTTPExposesNoteToPDF|FileUploadImage)' -count=1`
  - `go list -deps ./app/application/content` forbidden scan：Revel、Mongo driver、`app/db` 零命中。
- E-C02 的 required proof 在该 candidate 上完整通过。E-C03～E-C09、E-C22、E-C24、E-C27、E-C31 只关闭表中部分子项；没有 Mongo、real HTTP/browser、Linux/container、wkhtmltopdf、cross-host/restart evidence。

## 2026-09-17 implementation/check batch

- Candidate：未提交工作树；未执行 commit/archive/push，不能作为 `passed` 证据。
- 本批 focused tests 覆盖 D-C2 UTF-8 byte/path cleanup、public DNS 与 connected-address 拦截、特殊地址/NAT64-private、redirect final URL、compressed body cap、cancel、image validation、deterministic tar metadata/name/dedup、255-byte PAX name、source open/read/close、request-scoped temporary materialization 与 cleanup。
- Web/API attachment list/single/all download 已共用 owner/public/share read service、logical content store 和 materialized response artifact；strict ObjectID、metadata/file identity、size mismatch、source read/close failure已有 focused negative tests。temporary writer 在响应前完成 sync/close，并独立 reopen/stat reader；匿名 batch download 保持 legacy empty-text wire。Mongo-backed ACL/DB/open failure、update/delete authorization、response reader close/remove failure observability 仍未闭合；因此 E-C11/E-C12/E-C14/E-C15 只记 `partial`。
- `TestGoldenAPIActions` 以真实 HTTP 复跑通过，确认 `api/file_getAllAttachs_noToken.json` 仍为 text/plain empty miss，且 authenticated archive 保持既有 binary headers；这只覆盖 API Golden 子集，E-C25 仍为 `partial`。
- stable shared-copy attachment 测试已通过 injected content-store adapter，证明重试消费 frozen manifest 且不吸收 later attachment；尚无跨进程/真实 filesystem failpoint，E-C19 仍为 `partial`。
- legacy Web attachment upload 无 `OperationId` 时，publish 后 row/note mutation 失败仍可能留下 orphan。当前 wire 保留生成的 file ID，unknown result 返回 `partial_write`；现有 create path 无 owner-row-absent fenced compensation，禁止以直接 unlink 伪装闭合，E-C07/E-C27 保持 `partial`。
- `CopyHttpImage` legacy wire 无 operation identity；本批不再建立调用方无法重定位的随机 receipt。每次请求仍是新导入，publish 后 row 失败返回 `partial_write` 及生成 file ID；response-loss retry/idempotency 保持 open。
- E-C16/E-C17 仍缺完整 redirect/private-host 矩阵、真实 TLS/header/overall timeout、non-2xx/body-close 和真实 HTTP 证据，保持 `partial`。
- Phase 2 focused batch：Web upload/paste/avatar/blog-logo 使用 single-part bounded reader、decoder-derived extension 与 content-store publish；Web list/title、Web/API read 使用 strict ID 和 owner/public/share ACL；`GetFile`/base64/`FromFileId` ACL 已删除。paste copy 从 note 推导 destination owner，legacy `/file/copyImage` 因缺 target note identity 对 cross-owner fail closed。
- standalone image delete 已接 durable manifest/CAS 与 deterministic quarantine/restore/purge，metadata mutation 前后状态和 retry Verify 由 pure/fake/local filesystem tests 覆盖；未执行真实 Mongo compensation、kill/restart、terminal 7-day GC integration，因此 E-C10/E-C27 仍为 `partial`。
- reviewer 回归补充：匿名 actor 可读取确由 public note 引用的图片，private permission failure 不再因引用返回顺序遮蔽后续 public grant；missing repository/store 与 nil reader 保持 dependency/storage typed error；图片 response materialization 覆盖 cancel/close/sealed-size。delete 在 metadata failure+restore 后以同一 `quarantined` manifest 先 re-quarantine 再重试，row-compensation 失败且 projection 残留时可继续收敛；lifecycle/manifest parent-directory sync failure 以 `unknown_result` fail closed。以上仅为 pure/local filesystem/focused service 证据，不替代 real Mongo、真实 anonymous HTTP、process restart 或 root-swap/TOCTOU evidence。
- reviewer 结构性 open：manifest `.lease` 没有 crash-safe takeover，进程退出可永久阻塞 lookup key；terminal GC primitive 未接 runtime；lifecycle 的 absolute-path cross-root rename 仍缺 root-swap/TOCTOU closure。Web `GetImages`/`GetAlbums` 仍把 typed DB error 映射为 `200` 空 Page/array，而 direct success wire 无既定 failure body；需 `interface-http` 冻结非空成功之外的错误 status/body 后补 real HTTP Golden。E-C04/E-C12/E-C25/E-C27 保持 `partial`。
- 普通 Web image upload 与 CopyHttpImage/legacy attachment create 一样，publish 后 row 失败尚无可重入 Verify/repair 或 fenced cleanup；随机 file ID 只用于报告 partial identity，不构成客户端可寻址 receipt。E-C07/E-C27 保持 `partial`。
- album CRUD 已在 service 端拒绝 blank default、严格解析 actor/album ID，并以 `(AlbumID, OwnerID)` 做 lookup/count/update/delete；缺真实 Mongo/HTTP/browser 与重名 baseline 证据，因此 E-C13 仍为 `partial`。ApiNote multipart 已转入 service create-repair/pre-note lifecycle；E-C08/E-C09 仍缺真实 HTTP/Mongo 与 restart 证据。
- Environment：Windows/amd64，`go version go1.27.1 windows/amd64`；base HEAD `158a6dee7319d0dd3e09007fd623190525f28977`。
- Passed：`go test -count=1 -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentpdf ./app/service/contentremote ./app/service ./app/controllers ./app/controllers/api`；`go test -count=1 -race -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentremote ./app/service`；`go test -count=1 -timeout 60s ./app/tests/harness -run '^TestGoldenAPIActions$' -v`；`go vet ./...`；`go build ./...`；application/content forbidden dependency scan；Trellis 15 implement + 13 check validate；JSON/JSONL parse；`git diff --check`。
- Full `go test -count=1 -timeout 60s ./...` 未通过：其余已输出包均通过，`app/tests/harness` 在 `TestGoBinaryRejectsUnreadableDefaultVersionOutput` 启动版本探针时触发包级 60 秒 timeout。该用例随后单独以同一 60 秒上限复跑，1.26 秒通过；因此保留 full-suite timeout，不能记 E-C31 passed。
- reviewer 最终复跑：focused/race、`go vet ./...`、`go build ./...`、API Golden 均通过；full `go test -count=1 -timeout 60s ./...` 再次因 `app/tests/harness` 包级预算失败，本次卡在 `TestGoldenWebOwnershipControllers` 的 Mongo environment 启动。该真实 HTTP 用例随后单独复跑 8.05 秒通过；full-suite 仍记 failed，E-C31 保持 `partial`。

## 2026-09-17 storage reliability follow-up

- Candidate：同一未提交工作树；未执行 commit/archive/push/status change。Windows/amd64，Go 1.27.1；Go 1.26 与 Linux runtime 仍未验证。
- Lease：per-identity `O_EXCL` 文件替换为 bounded shard lock（每 root 最多 256）：Windows `LockFileEx`、Linux `flock`，另以原子 JSON 记录 lookup key、随机 owner、单调 epoch 和 acquired time。锁文件不删除；live helper process 冲突、强制 kill 后恢复、legacy empty lease 接管、malformed record fail-closed 的 Windows 测试通过。Linux 仅可在本批记录 cross-compile，E-C27/E-C28 仍为 `partial/delegated-unrun`。
- Lifecycle：quarantine/restore 不再用 validated absolute path 执行 `os.Rename`，改为两个 opened `os.Root` 内的 digest-bound copy/sync/remove；双份同 digest 是可重试中间态。Windows configured-root swap、outside-root parent symlink、destination-sync unknown 后 retry 均通过且未修改 replacement/outside 文件。junction/reparse、Linux runtime、cross-host/persistent volume 仍 open，E-C04/E-C06 保持 `partial`。
- GC：delete/create terminal marker 通过同步 startup maintenance 执行，统一 5 秒 context、各 128 条扫描上限、显式 scanned/removed/truncated 日志；failure 在 runtime publication 前返回。固定时钟/7 天严格边界及 bounded-call tests 通过；无常驻 goroutine或第二 GC state store。真实 restart/volume 仍 open。
- Create repair：Web image upload、`CopyHttpImage`、legacy Web attachment upload 共用一个 application manifest/store/recovery seam。manifest 先于 publish；DB error 后 owner-row exact/absent/conflict Verify；absent 只 quarantine，默认 24 小时后二次 Verify 才 purge，late row 会 restore。启动恢复限 64；create terminal marker 同样进入 7 天 GC。pure failpoint tests 覆盖 ambiguous commit、quarantine、late-row restore、二次 absence discard；contentfs tests 覆盖 no-replace/CAS/list/terminal GC。legacy wire 未增加 client operation identity，因此不记录 response-loss idempotency；real Mongo unknown-result/restart 仍 open，E-C07/E-C27 保持 `partial`。
- `GetImages`/`GetAlbums` error mapper 未选择 status/body，继续记 `interface-http` open；本批没有修改该 wire。
- Passed：`go test -count=1 -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentpdf ./app/service/contentremote ./app/service ./app/controllers ./app/controllers/api`；`go test -count=1 -race -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentremote ./app/service`；`go test -count=1 -timeout 60s ./app/tests/harness -run '^TestGoldenAPIActions$' -v`；`go vet ./...`；`go build ./...`；application/content forbidden dependency scan；gofmt check；Trellis 15 implement + 13 check validate；JSON/JSONL parse；`git diff --check`。
- Linux compile-only：`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c ./app/service/contentfs -o NUL` 通过。首次沿用 Windows CGO toolchain 的交叉尝试因缺 Linux C headers 失败，未产生 runtime evidence；因此 Linux lease/rooted lifecycle 仍不得记 passed。

## 2026-09-17 notes provider follow-up

- `ContentAssetPort` 已由占位接口扩展为显式 Reconcile/Copy/Delete command + Verify contract；actor、owner、source/destination note、generation 与 operation identity 分离，provider 只读取既有 root workspace receipt 的 ordered `Assets`，拒绝 unrelated receipt kind、failed/compensated receipt、乱序和重复 identity/index。reviewer 已修复 retry 使用当前 note USN 的漂移：Reconcile/Copy 现在以 root receipt `AssignedUSN` 为冻结 generation并拒绝冲突 command generation。图片 copy 已改用通用 content create-repair，不再建立 `note_copy_image` workspace receipt；未运行的真实 Mongo/unknown-result/restart 证据使 E-C21 仍保持 `partial`。
- API Add 把实际提交或显式引用的 asset identity 冻结到 note-create receipt；`HasBody=false` 的既有 `FileId` 同样进入 operation input 与 ordered manifest，非法 ID 在 receipt 创建前拒绝。API Update 的 absent 不构造 AssetWork、explicit empty/nonempty 进入 provider；上传资产的 frozen `ContentSHA256` 使用真实 SHA-256，而不是原始字节的 hex。combined content+Files repair 最后由 provider 收敛完整 File set。focused controller/service tests 与 API Golden 通过；真实 Mongo file/row/projection ambiguous-result 未运行，E-C18/E-C21 只升为 `partial`。
- stable same-owner/shared Web copy 由 provider 消费原 receipt manifest；附件和图片均使用稳定 destination + shared create-repair，不创建 per-asset notes receipt。root manifest 冻结 source content SHA-256、destination canonical row SHA-256、stable identity/index；Apply 与 `VerifyCopyNote` exact 核对 image lineage、attachment row、generation/index/digest，缺失或畸形 digest 的 pure regression 已通过。Mongo-dependent frozen-attachment/retry tests 在本机 27017 不可用时跳过，跨进程/filesystem failpoint 未运行，因此 E-C19/E-C21 保持 `partial`。
- 真实 `TestAPIUpdateNoteFilesPresenceContract` 已证明 root `note_save` receipt 正确冻结完整 `Assets`。reviewer 区分 lease authority 后，永久 lease path + kernel lock 的 live conflict、kill 后接管、malformed record 和 missing record recreate tests 均通过；lease/record parent entry 不再误用业务 manifest 的 directory-sync gate。Windows durable publication 改为对最终文件执行 `FlushFileBuffers`，业务 removal 使用 root-relative 可写父目录句柄刷新；原 `delete_manifest_parent_sync` blocker 已解除。测试 fixture 改用每次运行唯一的 attachment identities 后，命令连续两次独立运行通过，且不需要移动或删除历史 marker。该结果只关闭本机 Windows/NTFS barrier 与 harness isolation 缺口，不替代 Linux runtime、真实 Mongo ambiguous-result、process restart 或 cross-host evidence。
- Web permanent delete 在 tombstone/USN commit 后，把有序附件 identity 冻结到原 `note_delete_cleanup` receipt；单一 `content_assets` step 使用通用 delete manifest + quarantine/restore/purge，并 Verify attachment row/file absence 与 `NoteImages` projection。pure failpoint 覆盖 metadata failure restore、同 operation retry terminal、terminal replay 和 operation conflict；真实 Mongo、kill/restart、Linux/cross-host filesystem 未运行，E-C20/E-C21/E-C27/E-C28 不关闭。
- Final verification：`go test -count=1 -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentpdf ./app/service/contentremote ./app/application/notes ./app/service ./app/controllers ./app/controllers/api`；对应 application/content/contentfs/contentremote/application/notes/service 的 `-race` batch；`go vet ./...`；`go build ./...`；串行 `TestGoldenWebOwnershipControllers` 与 `TestGoldenAPIActions` 均通过。candidate 仍为未提交工作树，E-C31 保持 `partial`。

## 2026-09-17 PDF remote and hardening follow-up

- Candidate：同一未提交工作树；未执行 commit/archive/push/status change。Windows/amd64、Go 1.27.1；真实 Linux/container renderer/parser/zero-outbound 仍未运行。
- Remote/self-contained：PDF public URL 只经生产 wiring 的 `contentremote.Fetcher` 与 fail-closed `uploadImageSize` 进入，复用 D-C2 的 scheme/userinfo、DNS/connected address、redirect、timeout、compressed cap 与 image decode budget；没有 default/direct HTTP client、callback URL、appKey、private allowlist、adapter profile override 或 fallback。应用层按文档顺序收集引用，并以 local file ID/规范化 public URL 去重；最终文档仍由结构化 policy validator 拒绝所有非 data-image 外链。
- Budget：note HTML 在 resource discovery 前执行 32 MiB hard cap；64 MiB final-document/artifact 不可由配置放宽。serializer 先生成无资源 markup，再在每个去重资源加载前按 remaining budget 推导 raw cap，成本包含 unique raw bytes、每引用 base64 projected bytes 与 `src` markup；local adapter 先 inspect size 再 read，remote adapter 把同一 cap 交给 bounded fetch。任一资源/最终文档超限返回 `too_large`，已打开 local reader、remote response body 与 process temp 均按各自 error path cleanup。
- Process/readability：caller cancel/deadline 统一为 `timeout`；focused failpoint 覆盖 process failure、root/output open、output sync、opened-file identity、seek、read/output close、root close、temp remove、bounded stderr 与 executable/config revalidation。PDF validator 不再只检查 magic+EOF，现验证 versioned header、xref target bounds、trailer `/Size`/`/Root`、`startxref` 与 terminal EOF，并拒绝 malformed/truncated/trailing bytes。该自有 basic validator 不是第三方真实 parser 证据。
- Passed：`go test -count=1 -timeout 60s ./app/application/content ./app/service/contentpdf ./app/service/contentremote ./app/service ./app/controllers ./app/controllers/api`；对应 application/content、contentpdf、contentremote、service 的 `-race` batch；`go vet ./...`；`go build ./...`；`TestGoldenAPIActions`（14.99s）。Web focused 对 `safe.pdf`、attachment delivery 与 `application/pdf` 通过；API focused 对 `api.pdf` 通过。
- Delegated/unrun：`TestGoldenExportPdf` 在 Windows 按既有合同 skip，因为 reviewed Linux `app/tests/golden/api/note/exportPdf.json` 尚不存在。real wkhtmltopdf、real parser readability、zero outbound、local-file/private/metadata/redirect/CSS/script-fetch artifact 及 Linux/container cleanup 继续归 E-C23/E-C29/E-C30 delegated-unrun；E-C22/E-C24/E-C25/E-C31 仍保持 `partial`。
- 2026-09-17 reviewer follow-up：修复 aggregate budget 的 `int64` 前置溢出、等价 remote URL（host case/default port/empty path）去重、未计数 non-image `src` 内联、serializer/export/adapter/process context 传播、本地 reader close/timeout 分类、temp-mode failure cleanup、output identity failpoint 与 unsafe disposition filename。basic validator 现在解析 classic xref subsection，并把 trailer `/Size`、`/Root` 引用、xref entry offset 和实际 root object 关联；embedded token、错误 root offset、截断 entry、`startxref` 后额外内容均有 negative regression。该检查仍不是完整 PDF parser，也不支持据此关闭真实 parser evidence。
- Follow-up verification：同一 PDF/remote/service/controller focused batch 与 race batch、`go vet ./...`、`go build ./...`、application/content dependency scan、Trellis/JSONL validation、`git diff --check` 通过；`TestGoldenAPIActions` 最终复跑 9.97s 通过。`TestGoldenExportPdf` 在 Windows 明确 skip，未生成或改写 Golden；real wkhtmltopdf/parser/zero-outbound/Linux cleanup 状态不变。

## Integrity rules

1. planned、partial、delegated-unrun 都不是 passed。
2. fake executable 不关闭 E-C29；installed package/build 不替代 runtime artifact。
3. controller direct call/mock 不关闭 E-C25/E-C26；same-host fs 不关闭 E-C28。
4. passed 必须记录 commit/tree、run/attempt、time、environment、command、exit code、artifact/digest 和脱敏 failure/skip。

## 2026-09-17 structural reviewer follow-up

- Candidate：同一未提交工作树；未执行 commit/archive/push/status change。Windows/amd64、Go 1.27.1；Go 1.26 与 Linux runtime 未验证。
- Create exactness：manifest 新增 canonical `RecordDigest` 并纳入 input/state digest；service unit tests 覆盖 image 的 name/title/album/type/default/created/lineage 与 attachment 的 name/title/type/created conflict sensitivity，BSON datetime 以 UTC 毫秒归一。真实 Mongo ambiguous-result 仍未执行，E-C07/E-C27 保持 `partial`。
- Scan：delete/create manifest scan 不再使用 `ReadDir(-1)`；按 `ReadDir(32)` 流式读取全部 256 shard，以 minute-seeded 64-bit lookup-key ring rank 保留 top `limit+1`，内存 O(limit)。eligible terminal 与 active 在计入各自 limit 前过滤，避免 shard 内外固定前缀、junk 或另一状态 starvation。测试证明 terminal 不占 active recovery budget、active 不占 terminal GC budget；真实大目录、restart/volume 仍 open。
- Lifecycle：delete/create quarantine/restore/purge 冻结 digest+size；copy/digest cancellation、cleanup directory sync、size mismatch 均有 focused regression。Windows final removal 使用同一 verified handle 的 `ReOpenFile`/`FileDispositionInfoEx`，不再按原 name unlink；非 Windows 明确 `unsupported_filesystem`，Linux 只有 compile-only，E-C04/E-C06/E-C28 不关闭。
- Attachment projection：exact row 先 Verify projection；仅缺失时读取当前 note generation 并进入现有 notes SaveNote mutation，避免旧 generation 永久 conflict，已存在 projection 不分配新 USN。真实 Mongo concurrent note mutation 仍未执行。
- Passed：focused、race、`go vet ./...`、`go build ./...`、API Golden、Web ownership Golden、application/content dependency scan、Trellis validate、Linux amd64 CGO-disabled compile-only、`git diff --check`。API Golden 首次与 Web Golden 并行时因同名 Mongo fixture container 冲突失败，按 harness 单实例约束串行重跑后通过；不记行为失败。
- Full `go test -count=1 -timeout 60s ./...` 仍未关闭：非 harness 包均输出通过；`app/tests/harness` 单包复跑在 `TestGoBinaryRejectsDefaultToolchainBelowFloor` 的 temporary-directory cleanup 阶段触发包级 60 秒 timeout。该失败与本批 content assertions 无直接命中，但 E-C31 继续为 `partial`。
- 2026-09-17 reviewer 复核：`go test` focused（application/content、application/notes、service、controllers、controllers/api、contentfs）、同范围 race、`go vet ./...`、`go build ./...`、Web/API Golden、Trellis validate 与 `git diff --check` 通过。Windows durable publication follow-up 后，`TestAPIUpdateNoteFilesPresenceContract` 使用每轮唯一 attachment identities 连续两次独立运行通过；Mongo-dependent frozen-copy tests 在本机 27017 不可用时 skip。Linux runtime、真实 Mongo ambiguous-result、process restart、cross-host/persistent-volume evidence 仍 open，不得据此把 E-C19/E-C21/E-C31 升为 passed。

## 2026-09-17 Windows durable publication follow-up

- Candidate：同一未提交工作树；未执行 commit/archive/push/status change。Windows/amd64、Go 1.27.1；Go 1.26 与 Linux runtime 仍未验证。
- 原 platform blocker 已解除：Windows 不再对普通目录句柄调用会返回 `Access is denied` 的 `File.Sync`。data/delete/create manifest 在 link/replace 后通过既有 `os.Root` 以 `O_RDWR` 独立重开最终文件并执行 `File.Sync`/`FlushFileBuffers`；open/sync/close failpoint 均返回 `unknown_result`，同一操作可通过 existing destination 或 Load 重试收敛。父目录在 child publication 前为 provisional；Unix/Linux 仍保留 staging file sync + parent-directory fsync 契约。
- lifecycle 业务 removal 不复用 Windows terminal GC/staging cleanup 的 no-op：它通过已有 root 打开父级，再用 root-relative `NtCreateFile` 获取可写目录句柄并 `FlushFileBuffers`。terminal GC/staging cleanup 仍只依赖安全重现和幂等 retry，不作为业务 mutation durable completion 证据。
- 真实 `go test -count=1 -timeout 60s ./app/tests/harness -run '^TestAPIUpdateNoteFilesPresenceContract$' -v` 在 Windows durable publication 修复后通过；fixture 使用每次运行唯一的 attachment identities，不依赖清理、移动或删除历史 marker，也不产生 `%TEMP%` 外部 artifact。该结果关闭本机 Windows/NTFS 的 `delete_manifest_parent_sync` blocker，但不关闭 E-C19/E-C21/E-C27：Linux runtime、process restart、cross-host/persistent-volume 与真实 Mongo ambiguous-result 证据仍 open。
- 最终复跑：focused batch、race batch、`go vet ./...`、`go build ./...` 与 Linux amd64 CGO-disabled compile-only 均通过；真实 HTTP 串行 `TestGoldenWebOwnershipControllers` 与 `TestGoldenAPIActions` 通过。FilesPresence 使用唯一 fixture identities 后连续两次独立运行通过（7.53s、7.70s），两次均不需要外部 cleanup 或 marker 操作。该 harness isolation 修复不改变 durable barrier 的语义，也不把未运行的跨进程、跨主机或持久卷证据记为通过。

## 2026-09-17 remaining-gap implementation follow-up

- Candidate：同一未提交工作树；未执行 commit/archive/push/status change。Windows/amd64、Go 1.27.1；Go 1.26、Linux runtime、真实 Mongo ambiguous-result/restart/cross-host/browser/wkhtmltopdf 仍未验证。
- Typed roots：`service.InitContentRuntime` 只接受显式 `ContentRoots`，不再在 service 内从 `basePath` 派生。filesystem store、manifest/lifecycle 与 PDF backend 共用同一次 `contentfs.ValidateContentRoots` 返回的 canonical temporary path，focused regression 覆盖带 `..` 别名的配置只产生同一 canonical path。`cmd/leanote` 与 legacy Revel startup 的 application-base compatibility mapping 仍待 `interface-http` production-config binding 替换，因此 E-C04/production single-source 仍为 `partial`。
- Attachment：Web upload 的 actor/note strict parse、stable/random asset ID、storage name、logical path 与 metadata construction 已从 controller 下沉 service；malformed identity 在 DB/permission dependency 前失败。batch archive 移除 service 层的匿名 actor重复拒绝，public note 可走 application authorization；API adapter 继续保留既有 no-token empty-text wire。首次 API Golden 因该 adapter gate 缺失返回额外 `Accept-Ranges` 而失败，修复后同命令复跑通过。真实匿名 Web/Mongo ACL 仍未运行，E-C11/E-C12 保持 `partial`。
- Inventory/cleanup：静态 scan 确认 22 Web + 4 API callable、2 ApiNote integrations、4 provider primitives；4 个 historical dormant surface 均无 route/registry/caller。`app/lea/html2image` 两个文件与 ApiFile 注释 action block 已删除，build 通过。ApiNote stable multipart upload、committed finalize 与 uncommitted cleanup 已从 controller 移至 `AttachService` 的 typed lifecycle；E-C01/E-C08/E-C09 仍因 real HTTP/Mongo/restart 未运行保持 `partial`。
- Passed：focused content/contentfs/contentpdf/contentremote/notes/service/controllers/API/cmd tests；对应 content/contentfs/contentremote/notes/service race batch；`go vet ./...`；`go build ./...`；Web ownership Golden；API Golden 修复后复跑；FilesPresence；Linux amd64 CGO-disabled contentfs compile-only；Trellis 15 implement + 13 check validate。
- Final review follow-up：content root startup probe 现对 data/quarantine 执行 mode/write/sync/close/rename/read-back/remove，并对 temporary 执行 mode/write/sync/close/read-back/remove；清理失败不再静默。共享笔记 Web 附件的 logical path 改按 note owner namespace 派生，`UploadUserId` 仍单独保存 uploader actor；multipart reader 由 service 有界读取并在任何提前拒绝时关闭，正常路径在 publish/DB 前检查 close error，避免 cleanup failure 被报告为成功。pure regression 覆盖 owner 与 uploader 不同、`limit+1` 读取、close failure 与拒绝路径 close。以上为 focused/local evidence，不关闭 production-config single-source、真实 shared-note Mongo/HTTP、跨卷或 Linux runtime 项。

## 2026-09-17 ApiNote pre-note recovery follow-up

- Candidate：未提交工作树；Go 1.27.1。Go 1.26、Linux、Mongo/restart/cross-host/browser/wkhtmltopdf 未验证。
- ApiNote controller 仅绑定 multipart presence/open；`AttachService` 使用 `PreNote` generation `0` 的 create manifest 发布稳定 image/attachment。父 note 已提交时先 exact row+byte Verify 再 terminal committed；父 note 不存在时先经 owner-scoped delete manifest/quarantine/metadata delete/purge/Verify，再 terminal discarded。manifest/row/file 任一漂移或 dependency error 均 fail closed。
- startup maintenance 以独立、action-filtered `api_note_asset_upload` scan（limit 64）处理 pre-note；generic create recovery 改为只扫描 non-pre-note，二者互不消耗预算。unknown future pre-note action 不会被 ApiNote recovery 删除。
- TDD：恢复、预算隔离与 action-filter regressions 先红后绿；archive anonymous test 固定 `db.Notes`，消除同包 Mongo 初始化的顺序依赖。
- Passed：`go test -count=1 -timeout 60s ./app/application/content ./app/service/contentfs ./app/service ./app/controllers/api`；`go test -race -count=1 -timeout 60s ./app/application/content ./app/service/contentfs ./app/service ./app/controllers/api`；`go vet ./...`；`go build ./...`；`python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-content`；`git diff --check`。
- `go test ./...` 的默认并行包调度会与共享 Mongo fixture 和 harness 的运行时 `app/routes` 生成/清理冲突；串行 `-p 1` 包测试未复现 content assertion failure，但不能替代独立 Mongo/harness runner。E-C27/E-C31 保持 `partial`，不把该环境限制写成已关闭的跨进程/restart 证据。
