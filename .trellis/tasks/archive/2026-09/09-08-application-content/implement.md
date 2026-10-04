# 应用层：内容、文件与媒体 — 执行计划

## Current state and continuation gate

- [x] `09-08-domain-contracts`、`09-08-application-notes` 已归档。
- [x] action inventory、规格审核和 evidence matrix 已建立。
- [x] 用户确认 D-C1：远程图片仅允许公网 HTTP/HTTPS，不提供内网 allowlist。
- [x] 前一轮已获得实现批准并激活；`158a6dee` 已提交 content roots/store、manifest、media 与 PDF foundation，任务当前为 `in_progress`。
- [x] 2026-09-17 重新审核已把当前实现、剩余范围、跨任务 handoff 和证据状态写回规格。
- [x] 用户确认 D-C2 visible-text/remote-fetch hard profile：`255 UTF-8 bytes`、分阶段 timeout、`5` redirects、compressed body cap 与 invalid-config fail-closed。
- [x] 已提交包含 D-C2 的最终规划摘要，并在下一条消息获得继续功能编码的明确批准；未重复 `task.py start`。

## Phase 1 — Contract and safety seams

- [x] 建立纯 `app/application/content` foundation contracts/results/errors/ports，并以 dependency closure 排除 Revel/Mongo/`app/db`/BSON。
- [ ] 补齐 26 actions 与 4 provider 所需 command/result/repository ports，并删除 controller-owned 业务后复跑 dependency scan。当前 26 callable、2 ApiNote integration、4 provider 数量已静态对账；Web attachment 与 ApiNote multipart 已由 service/content lifecycle 处理，仍缺 interface-http production root/wire closure 与真实集成证据，不能标完整完成。
- [x] 定义 paired durable roots、logical grammar、legacy prefix、opaque open 与唯一 resolver；验证 absolute/canonical、same-filesystem、non-public、no-overlap。
- [ ] 接入 `interface-http` typed production roots；content/service 已提供只接受显式 `ContentRoots` 的 initializer，且 storage/PDF 共用同一次验证得到的 canonical temporary root。当前 `cmd/leanote` 与 legacy Revel startup 仍保留 application-base compatibility mapping；由下游 `interface-http` production-config 替换后才能移除该映射并关闭“唯一生产来源”，本任务不擅自新增配置键。
- [x] 覆盖 portable traversal、absolute/drive/UNC/device/reserved、legacy prefix 和 host symlink；实现 deepest-existing-parent resolver/open。
- [ ] 补完整 junction/reparse 与 Linux runtime matrix；本批已移除 lifecycle absolute-path cross-root rename，改用 opened `os.Root` 下的 digest+size-bound copy/sync 与 exact opened-handle removal。Windows 使用 `ReOpenFile` + `FileDispositionInfoEx` 关闭 final Verify-to-remove name replacement window，并以 configured-root swap、outside-root parent symlink、destination-sync unknown retry 覆盖 root-swap/TOCTOU 与 fail-closed。非 Windows 当前对 exact removal 返回 `unsupported_filesystem`；Linux compile-only 不关闭能力项，后续必须提供等价 platform primitive 或保持显式不支持。
- [x] 实现 stable identity、streaming SHA-256、unique same-dir staging、no-replace、platform durable publication barrier、four-state Verify，并覆盖 same/different digest、concurrency 与 barrier unknown。Unix/Linux 使用 file sync + parent-directory fsync；Windows 在 link/replace 后以既有 `os.Root` 独立重开最终文件并 flush，父目录创建为 provisional；lifecycle 业务删除使用 root-relative 可写目录句柄 flush，terminal GC/staging cleanup 仅依赖安全重现与幂等 retry。
- [ ] 补 write/file-sync/close/no-replace/unsupported-fs failpoints 和真实 metadata/projection adapter Verify。

## Phase 2 — Upload, image and album

- [x] 建 bounded read、invalid upload config fail-closed、`DecodeConfig`、checked dimension/pixel/allocation、GIF frame/cumulative budget 与 GIF/JPEG/PNG/BMP foundation tests。
- [x] Web multipart image 入口限定唯一 `file` part，以配置正数 limit 经 bounded reader 读取，并统一执行 D-C2 visible text 与 decoder budget；missing-file regression 已覆盖。
- [x] Web image upload/paste/avatar/blog-logo primitive 已移入 content/service，使用真实 decoder extension 与 no-clobber content store；不再 fixed-PNG、extension-only 或 1000 MB fallback。ApiNote stable upload 以 `PreNote` generation `0` 使用相同 create-repair manifest，note committed 后严格 finalize，失败/缺 parent 时经 delete lifecycle discard。
- [x] legacy Web image upload、`CopyHttpImage` 与 legacy Web attachment upload 共用 create-repair manifest：publish 前持久化 generated identity/digest/path/owner row binding；DB 不确定结果先 Verify exact row；缺行只 rooted quarantine，保留期后二次 absence Verify 才 purge；row 后到则 restore。wire 仍只返回 generated ID + `partial_write`，不声称 response-loss replay。
- [x] Web image list/title 与 Web/API read 已收敛 owner/public/share ACL；删除 `GetFile`/base64 及 `FromFileId` authorization。paste copy 的目标 owner 由已验证 note 推导；缺少 target note identity 的 legacy cross-owner `CopyImage` fail closed。
- [x] standalone image delete 已接 stable lookup → durable manifest → deterministic quarantine → owner row/projection mutation → restore/purge → terminal marker，active retry 先 Verify absent；纯状态机与 filesystem lifecycle focused tests 通过。
- [x] delete recovery 已覆盖 metadata failure 后 restore：保留单调 `quarantined` manifest，后续调用在 mutation 前幂等 re-quarantine；row 已删且 projection/row compensation 失败时继续清理 projection，避免恢复死循环。新建 lifecycle/manifest 分片目录逐级同步父目录，sync failure 返回 `unknown_result`。
- [ ] 补 real Mongo row/projection compensation、Linux process restart、CAS/torn-write、cross-host/volume GC evidence。本批 lease 已改为每 root 最多 256 个 shard 级持久锁文件，Windows `LockFileEx`/Linux `flock` 保持 live-process exclusion，原子 owner/epoch record 绑定本次 lookup，进程死亡由内核释放；不删除可能 live 的锁文件。Windows helper process 持锁冲突、kill 后接管、legacy empty lease 与 malformed record fail-closed 已通过。delete/create scan 使用 `ReadDir(32)` 流式读取、minute-seeded 64-bit lookup-key ring ranking，并在计入 limit 前区分 eligible terminal/active；内存为 O(limit)，避免 junk、另一状态以及 shard 内外固定前缀耗尽预算，无第二 durable cursor。startup maintenance 仍为同步 5s 单次入口（GC 各 128，create recovery 64），真实 restart/长期推进证据仍 open。
- [x] album CRUD 已收敛 strict actor/album ID、blank default 服务端保护、same-owner lookup/count/update/delete 和 D-C2 name。
- [ ] 补 album Mongo integration、同 owner 重名 baseline 与真实 HTTP/browser evidence。
- [ ] `GetImages`/`GetAlbums` service 已返回 typed DB error，但 legacy direct `Page`/array wire 没有既定 failure envelope，controller 仍将错误映射为 `200` 空成功。由 `interface-http` 决定并冻结最小错误 wire（推荐非 2xx + 保留 legacy body shape，或显式失败 envelope）后补 real HTTP regression；不得继续以空数据伪装依赖成功。
- [x] 实现 public-only remote DNS/dial/redirect/address/timeout/size/decode policy 和 SSRF tests；不加入内网 allowlist/fallback。Web import 与 PDF remote resource 共用 D-C2 profile；真实公网/TLS 环境证据仍按 E-C16/E-C17 保持 partial。
- [ ] `CopyHttpImage` 的 legacy wire 只有 `src`，无法区分调用方主动重复导入与 response-loss retry；当前每次请求保持新导入语义，server-side create-repair 可按 generated identity 收敛 file/row/orphan，但调用方仍不能用原请求重定位该 identity。若要关闭 client retry/idempotency，仍须由 `interface-http` 决定 operation identity；不得把随机 file ID receipt 伪装成调用方可重试能力。

## Phase 3 — Attachment and archive

- [ ] attachment upload/list/read/delete/copy/reconcile 接入同一 roots/publish/Verify/ACL。
- [x] strict parse Web attachment/note/actor IDs，区分 note owner/uploader，并统一 owner/public/share read 与 verified update。Web upload logical path 使用 note owner namespace，row 单独保留 uploader actor；匿名 archive 不再被 adapter 额外闸门阻断，公开笔记沿 application authorization；真实 HTTP/Mongo 仍 partial。
- [x] 实现单附件 opaque stream，响应前物化并显式传播 open/stat/read/close/size mismatch；图片响应物化同步覆盖 cancel、source close、sealed-size mismatch 和 temporary cleanup。
- [x] Web/API 共用 deterministic `tar.gz` writer；focused 覆盖 entry sanitization/duplicates、close order、并发独立 artifact 与 temp cleanup，真实 HTTP/reader-close delivery 仍按 E-C14/E-C15 保持 partial。
- [x] 删除共享 `files/attach_all`/title temp、defer-in-loop、panic 和截断成功；Web/API 改用 request-scoped `0600` temporary artifact。
- [x] legacy Web attachment upload 无 `OperationId` 时接入共享 create-repair；manifest 冻结 note owner/uploader/note/generation/row/path/digest。owner row exact 后复用确定性 notes mutation 收敛 AttachNum/USN；row absent 才进入 quarantine，延迟二次 absence Verify 后 purge。稳定 `OperationId` 入口继续使用 notes receipt，两个状态机保持分离。
- [x] create manifest 冻结完整 canonical owner-row digest（Mongo datetime 归一为 UTC 毫秒）；image/attachment exact Verify 覆盖 ID、owner/note、name/title/type、album/default/lineage、path、size、created time。attachment row exact 后先 Verify projection，缺失时按当前 note generation 进入既有 notes mutation，避免无关 note 更新后永久卡在上传时旧 generation；已收敛 projection 不重复分配 USN。

## Phase 4 — Notes provider

- [x] 实现 notes `ContentAssetPort` adapter，不新增 receipt collection/state machine；command 显式绑定 actor/owner/source/destination note、generation 与既有 notes receipt identity，provider 只消费 receipt `Assets`。provider 以 root receipt `AssignedUSN` 冻结 generation；图片 copy 已改用通用 content create-repair，不再创建 `note_copy_image` workspace receipt。
- [x] Reconcile 覆盖 Files missing（不构造 AssetWork）、explicit empty 与 nonempty；完整集合通过 content `Open/Verify`、owner row、`NoteImages` 与 `AttachNum` Apply/Verify，combined content update 先修复 content-derived index、最后以 frozen Files 收敛。
- [x] Copy 消费 receipt-frozen ordered manifest，拒绝乱序/重复 identity/index；same-owner/shared stable copy 派生稳定 destination，图片复用 Publish/Verify，附件复用 create-repair，retry 不重新吸收 later source。root manifest 同时冻结 source content SHA-256 与 destination canonical row SHA-256；Apply 和 `VerifyCopyNote` 均 exact 核对 image lineage、attachment row、stable generation/index/digest。pure/focused tests 已通过，真实 Mongo、跨进程和 filesystem failpoint 证据仍使 E-C19/E-C21 保持 `partial`。
- [x] Delete 是既有 note cleanup receipt 的 post-commit repair step；附件冻结后走 durable manifest/quarantine/metadata Verify/purge，图片投影单调清除，不分配、回滚或重复 note USN/history/tombstone。
- [x] API Add/Update 与 stable Web copy、Web permanent delete 已接 provider contract；legacy 无 `OperationId` copy 保留原 non-retry-safe wire/顺序。真实 Mongo unknown-result、process restart 与 filesystem failpoint 交 delivery，当前不记 passed。

## Phase 5 — Unified PDF renderer

- [x] 建 renderer request/artifact/error、opaque admin descriptor、content-owned self-contained serializer/policy validator 和 direct process backend。
- [x] Web/API `ExportPdf` 迁到同一 application service；legacy `ToPdf` 只 bind-and-discard `appKey`，不再作为 callback credential。
- [x] focused tests 覆盖 active-content stripping、authorized local resource、direct argv/space、descriptor revalidation、local-file deny flag、stdin/request temp、timeout、bounded diagnostic 和 failure cleanup。
- [x] 按 D-C2/R-C9 加入公网 remote resource：PDF 只通过生产 wiring 的 `contentremote.Fetcher` 与 fail-closed `uploadImageSize` 读取公网图片，不持有 HTTP client/profile override；local ID/规范化 URL 按文档顺序去重。32 MiB input 与 64 MiB final/artifact hard cap 已生效；serializer 先计算无资源 markup，再为每个去重资源按 raw + base64 projected bytes + 每引用 markup 推导 adapter read cap，超预算不继续读取。
- [ ] 本地 focused 已补 caller cancel、process/root/output open/sync/identity/seek/read/close/remove failpoint、bounded stderr、64 MiB output hard cap、安全 filename 与 Web/API `application/pdf`/attachment/legacy failure body；basic validator 已检查 coherent header、classic xref subsection、trailer Size/Root、root entry 到实际 object、startxref/terminal EOF，并拒绝 embedded-token、malformed/truncated/extra-garbage。真实 wkhtmltopdf/parser artifact 与 reviewed PDF Golden 在 Windows runner 不可得，不能把本地结构校验记为真实 parser/runtime closure。
- [ ] Windows fake executable 只记 focused；Linux/container real wkhtmltopdf 验证零 outbound 及 file/private/metadata/redirect/CSS/script-fetch 负向 artifact，记 delivery delegated-unrun。

## Phase 6 — Cleanup and verification

- [ ] 核对 26 callable、2 integrations、4 providers；commented ApiFile actions 不重新路由。
- [x] 以全仓 import/route/caller scan 与 build 证明后删除无消费者 `app/lea/html2image`；同时删除未路由的 ApiFile 注释 action block，不重新发布 dormant surface。
- [ ] 删除所有已迁移 controller business helpers。Web attachment identity/path 与 ApiNote stable multipart publish/cleanup 已下沉 service；controller 仍保留 presence/open 和兼容 wire mapping，production root/wire 由 interface-http 决定，不能据此扩大本 leaf。
- [ ] 运行 changed Go `gofmt`、focused/full tests、dependency scan、Mongo/filesystem integration、`go vet ./...`、`go build ./...`、`git diff --check`。
- [ ] 若改 frontend source，遵循 presentation/Node 24 唯一来源执行 `npm ci && npm run build && npm test`；否则不触碰 bundle。
- [ ] 交接 real HTTP route/method/status/Content-Type、browser、cross-host/restart、volume、Linux PDF；未跑保持 delegated-unrun。

## Evidence record

每次记录 candidate commit/tree、UTC time、OS、Mongo version/topology、command、exit code、discovered/executed/pass/fail/skip 和脱敏原因。build、mock、历史结果、CI 包声明不能替代真实 HTTP/browser/PDF/failpoint evidence。
