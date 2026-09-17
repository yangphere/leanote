# 应用层：内容、文件与媒体 — PRD

## Goal

把图片、附件、相册、批量下载、远程图片导入、PDF 渲染和笔记资产投影收敛为可测试的应用层能力。所有资源操作必须具有可信 actor、明确 owner、单一文件根/路径策略、可恢复的文件与 Mongo 一致性边界和稳定错误分类，同时保持现有 Web/API URL、成功响应外形、下载文件名与 Content-Type 兼容。

本任务是 `application-notes` 的下游 provider：notes 拥有 note-specific durable receipt/idempotency、USN 与 history 状态机；content 只提供由 stable operation identity、generation、destination 和 digest 约束的 Apply/Verify primitive，不建立第二套 receipt 事实来源。

## Confirmed baseline

- `09-08-domain-contracts` 与 `09-08-application-notes` 已归档完成，是本任务的直接依赖。
- 本 leaf 已在前一轮明确批准后激活，当前 `status=in_progress`。提交 `158a6dee7319d0dd3e09007fd623190525f28977` 已落地纯 content contract、logical path、roots/no-clobber store、standalone delete manifest、media budget 和统一 PDF renderer/runtime 基础；它不是本轮规格审核产生的实现。
- 当前仍未完成 standalone delete 业务流整合、remote fetch、统一 `tar.gz`、image/attachment/album 全量迁移、notes provider 和 delivery 证据。本轮审核只校正规格与证据状态，不把基础 primitive 的存在写成 action 完成。
- 2026-09-17 在 Windows/amd64、Go 1.27.1 上，`app/application/content`、`app/service/contentfs`、`app/service/contentpdf` 及选定 service/controller focused tests 通过；模块声明仍为 Go 1.26，当前运行不能替代 Go 1.26、Mongo、Linux/container、真实 HTTP/browser 或跨进程证据。
- `conf/routes` 仍以通配 method 和 generic controller/action route 暴露 content actions；HTTP method、binder、status、Content-Type 和旧 wire mapper 归 `interface-http`。
- Web image/album/attachment、remote fetch、`tar.gz` 与 PDF Export 已迁入 content/service；ApiNote multipart 仅在 controller 做 presence/open，stable image/attachment publish、pre-note finalization 与失败清理由 service 的 create-repair/delete lifecycle 接管。仍未迁移的 production root binding、HTTP wire mapper 和真实环境证据不得据此视为完成。
- 当前附件批量下载产物是 `tar.gz`，不是 ZIP；旧规格中的 ZIP 解压约束不适用。
- notes 已声明并接入 `AssetMutation.Apply/Verify` 与 `ContentAssetPort`；content 不创建第二份 notes receipt，真实 Mongo/restart/failpoint 仍是独立证据门。
- `app/lea/html2image` 与无路由的 ApiFile 注释 action 已在 reachability scan 后删除；它们不应重新发布为能力。

## Actors and authorization

- `ActorUserID` 只来自可信 session/token principal，不接受 query、form、multipart 或 JSON 中的 user ID 覆盖。
- `OwnerUserID` 来自 owner-scoped metadata lookup；私有资源读取和 mutation 在同一 repository query 中同时限定资源 ID 与 owner。
- note 图片/附件的共享访问只消费 `application-publishing` 的 read/update permission port；公开博客图片仅在资源确被已发布 note 引用时可读。
- `UploadUserID`、`CreatedUserID`、`FromFileId` 和客户端 `userId/toUserId` 都不是 ACL；`FromFileId` 只作 lineage/幂等校验。
- application result 区分 unauthorized 与 not-found；逐 action adapter 可为防泄漏映射成相同旧外形。

## Scope and ownership

| 子域 | 本任务拥有 | 跨任务边界 |
| --- | --- | --- |
| content storage | `ContentRoots`、安全逻辑路径、open/read/publish/verify/quarantine/delete/cleanup | 不接受任意绝对路径 |
| image | 本地上传、paste、avatar/blog logo 上传 primitive、列表、标题、删除、copy、远程导入 | avatar commit 归 identity；blog 设置归 publishing；HTML escaping 归 presentation |
| attachment | 上传、列表、单个下载、删除、note 全量 `tar.gz`、copy/reconcile | note receipt 归 notes；HTTP stream 归 interface |
| album | list/add/rename/delete 和默认相册规则 | 默认相册为 `albumId=""` 虚拟集合 |
| note provider | `ReconcileNote`、`CopyNote`、`DeleteNote` 的 Apply/Verify | 不复制 notes receipt/USN/history |
| PDF | renderer service、受控 executable、argv、timeout、临时输入输出和清理 | admin 只提供配置入口；interface 映射；delivery 做 Linux/container smoke |
| dormant | `html2image` 与注释 ApiFile actions reachability | 不重新发布未使用功能 |

完整 callable action、输入、输出和 owner 见 `research/action-inventory.md`。

## Functional requirements

### R-C1 — Typed application boundary

- 建立纯 `app/application/content` command/result/port 契约；不得导入 Revel、Mongo driver、`app/db` 或 BSON，也不得暴露 OS 绝对路径或 HTTP writer。
- controller 只做 principal、presence-aware 参数/multipart 绑定和旧响应映射；权限、大小、媒体、路径、publish/verify、归档与 PDF 只有一个应用层事实来源。
- `app/service` 可作为现有 Mongo/filesystem adapter 迁移载体，但 controller 不再创建 ObjectID、构造持久化路径、读写文件、打 tar 或启动进程。
- 稳定错误分类至少含：`validation`、`unauthorized`、`not_found`、`conflict`、`too_large`、`unsupported_media`、`unsafe_path`、`storage_unavailable`、`unsupported_filesystem`、`timeout`、`renderer_failed`、`partial_write`、`unknown_result`、`dependency`。

### R-C2 — Single roots and safe paths

- 所有新资源只使用注入的 `ContentRoots`：private files、public upload、temporary，以及与每个 durable data root 配对的 non-public quarantine root。每个 quarantine root 必须与对应 data root 位于同一 filesystem/volume 以支持 atomic rename，但不得位于 Web/static 可达目录、不得与 data/temporary/其他 root 重叠。根必须是显式绝对路径、可解析；缺失、相对、不可访问、跨设备或冲突 fail closed。
- `interface-http` 的 production-config seam 是生产 root 来源与绑定的唯一事实来源；content 只接收 typed roots 并执行 canonical/same-filesystem/non-public/no-overlap 验证。`InitContentRuntime(ContentRoots)` 已是显式 typed initializer；当前 entrypoint 的 application-base compatibility mapping 只属迁移接线，后续不得与 production-config 并存为第二配置来源。
- DB 只保存受支持 logical path；兼容 `files/...`、`public/upload/...`、既有 `upload/...` 时先由显式 root-kind 表归一化，再走同一 resolver。未知 prefix 返回 `unsafe_path`。
- 拒绝 `.`/`..`、正反斜杠穿越、NUL/控制字符、绝对路径、drive-relative/volume、UNC/device、reserved device name 和 canonical escape。
- 不存在的目标检查 deepest existing ancestor；到 root 的每段不得经 symlink、junction、reparse point 或其他逃逸 indirection。创建后和打开前复核。仅 `filepath.Clean` 或字符串前缀不构成证明。
- legacy row 不能安全解析时显式失败，不得为兼容回退到任意路径。

### R-C3 — Stable identity, no-clobber and Verify

- 可重试写入由 `OperationID + Generation + OwnerID + AssetKind + SourceIdentity` 派生稳定 destination；SHA-256 digest 绑定资源字节和影响输出的元数据。
- 在 destination 同目录创建唯一 mode `0600` staging，完成 write、file sync、close 后以 no-replace primitive 发布，再同步目录。禁止固定 `.pending`、truncate staging、普通 overwrite rename 或先删目标。
- destination 已存在且完整 SHA-256 一致时返回 already-applied；摘要不同返回 conflict，绝不覆盖。文件系统无法证明 no-replace/directory durability 时返回 `unsupported_filesystem`，不得降级。
- Verify 同时检查文件、SHA-256、owner-scoped Mongo row 和 note image/attachment projection，返回 `applied/not_applied/conflict/unknown`；读取错误不是 not-applied。
- unknown-result 后先 Verify；仅 not-applied 且 provider 声明 replay-safe 时重放 Apply。content 不持久化第二份 note receipt。

### R-C4 — Multi-write lifecycle and cleanup

- upload/copy/reconcile/delete 列出必需文件写、Mongo row、projection、补偿和 Verify 顺序，不能用最后一个 bool 覆盖较早失败。
- delete 不得先不可逆删 DB row 再 unlink。先将文件移入同文件系统唯一 quarantine，再做 owner-scoped metadata/projection mutation；DB 失败恢复，DB 成功后 purge。restore/purge/unknown 以同 identity 可 repair。
- standalone image/attachment delete 必须在移动文件前，以 no-replace 方式原子发布 mode `0600` 的 durable quarantine manifest；manifest 至少冻结 delete identity、owner、asset kind/ID、原 logical path、content digest、metadata/projection generation 和当前阶段。它是通用 content lifecycle 记录，不是 note receipt；notes provider delete 继续只使用 notes receipt，禁止再建一份 note 状态机。
- manifest/terminal stage 更新必须带 version+digest，以同目录 unique mode `0600` staging 完成 write/sync/close 后原子替换并 directory sync；并发 worker 通过 version CAS/identity lease 保证阶段单调前进。torn/corrupt/stale version 返回 conflict/unknown 并保留现场，禁止 truncate、静默重建或按文件是否存在猜测成功。
- delete identity 必须可在进程重启后重建：先由 `action + owner + asset kind + asset ID` 派生不依赖 DB row 的 stable lookup key；显式 `OperationID` 按 owner/action scope 纳入 identity，旧入口没有 `OperationID` 时再以 lookup key + metadata generation/digest 派生。quarantine 路径由该 identity 确定性派生且 no-clobber，不能把进程内随机 handle 当作唯一定位方式。重试通过 lookup key 找到 active/terminal manifest，并按 source/quarantine/manifest/row/projection 状态矩阵继续、恢复、清除或返回 conflict/unknown。
- purge 与最终 Verify 完成后，active manifest 原子转为不含路径/内容的 terminal marker，默认保留 7 天后 GC；保留期内相同 identity 重放既有成功结果。marker 已 GC 且 row/quarantine 均不存在时，legacy 无 `OperationID` 请求只能返回兼容 not-found，禁止无证据声称 already-applied。GC 获取同一 identity lease、复核 terminal version/age 后执行；失败可观测且按 lookup key 幂等重试，不能无界积累。
- staging/quarantine/PDF/archive 临时文件在成功、validation failure、timeout、cancel 和 process failure 后都有明确清理；清理失败可观测且不得伪装成功。
- concurrent retry/restart 不产生重复 row/projection、覆盖资源或无界残留。

### R-C5 — Upload and media validation

- multipart 先限定单个 `file` part 和流式最大字节数，再读取/写入；不依赖客户端 Content-Length 或读完整 body 后才判断。
- `uploadImageSize`、`uploadAvatarSize`、`uploadBlogLogoSize`、`uploadAttachSize` 必须是正数；missing/invalid/zero/negative 返回配置错误，禁止回退 1000 MB。
- 图片扩展、sniffed type 与真实 decoder 一致；有效 GIF/JPEG/PNG/BMP 基线保留，伪扩展、截断、非图片拒绝。paste 不得无条件标为 PNG。上传与远程导入在完整 decode 前必须先 `DecodeConfig`，使用防溢出乘法检查 width/height/pixel/预计 allocation；内建 hard profile 为单边 `16384`、总像素 `25,000,000`、预计解码内存 `128 MiB`。GIF 另限制最多 `256` 帧，全部帧累计像素仍不得超过 `25,000,000`；未配置 override 时使用该 profile，可选部署配置只允许收紧，出现但 missing value/invalid/zero/negative/放宽硬上限时 fail closed。
- 完整 decode/逐帧验证必须在上述 budget 内完成；header 合法但超预算、尺寸乘法溢出、后续帧损坏或实际 allocation 超预算均返回 `unsupported_media`/`too_large` 并清理，不得仅凭压缩字节上限继续。
- 元数据保存实际文件名、logical path、size、owner、album 和安全显示名；附件另含 note owner、uploader actor、type、stable ID。DTO 不暴露 OS path。
- 原始文件名、image title 与 album name 去除路径、NUL/控制字符；album name、image title 与 attachment display filename 按清理后 UTF-8 编码最多 `255 bytes`，存储名由服务端生成。超限统一返回 `validation`，不得静默截断；既有超限数据仍允许读取/下载，但 rename/copy mutation 遇到待写入的超限值时整体拒绝并保留原值。
- avatar/blog logo primitive 成功不等于 identity/publishing commit 成功；content 不修改 session 或发布状态。

### R-C6 — Read/list/copy/delete authorization

- required ObjectID 的 missing、blank、invalid、not-found 分开处理；外部输入不得进入 `MustObjectIDFromHex` 或 panic path。
- image list/search/page 只返回 actor 自有 row；album filter 同 owner。page/default/limit/sort/empty shape 由 action Golden 冻结。
- image/attachment read 在打开文件前完成 owner/blog/share ACL；open failure 是 storage error，不返回 nil handle 或空成功。
- copy 分别验证 source read 和 destination write；destination owner 由已验证 note/port 推导，不接受任意 `toUserId`。同 stable operation 返回同 destination ID。
- image 删除前捕获“被 note 引用”当前基线并定义 projection repair，不留下未报告断链。

### R-C7 — Album rules

- `albumId=""` 是“默认/未分组”虚拟相册，只用于 list/upload filter；服务端拒绝 rename/delete，不依赖前端按钮。
- persisted album CRUD 都以 `(AlbumID, OwnerID)` 查询；name trim 后非空、有界 UTF-8、无控制字符。同 owner 重名行为按现有 HTTP/Mongo baseline 冻结，不无证据新增唯一性。
- 非空相册删除保持 `has images` 业务失败；count/delete 均同 owner，跨用户图片不影响判断。
- application 返回原始 name DTO；presentation 负责 escaping，禁止双重转义。

### R-C8 — Attachment download and tar.gz

- 单附件/全量下载先做 note/attachment ACL，再解析 logical path；not-found、unauthorized、unsafe path、open/stat/read/close 分为 typed result。
- `DownloadAll` 生成 gzip-compressed tar。禁止共享固定目录或 note-title 固定临时路径；优先流式，必须落盘时使用 request-scoped mode `0600` temp 并清理。
- entry 仅安全 base name，拒绝分隔符、dot entry、absolute/drive/UNC、NUL/控制字符；重复名以确定性后缀消歧，不能覆盖/丢项。
- 只写 regular-file entry；header 使用固定 mode，清空 uid/gid/uname/gname/link target，并使用稳定业务时间或固定 epoch，不复制宿主文件 mode、mtime、owner 或扩展属性。相同 manifest 与字节必须生成相同 entry 顺序和 header，不能泄露主机元数据。
- 逐项 file、tar、gzip、sink close 顺序和错误全检查；失败不 panic，不把截断 archive 返回成功。
- 下载名保持 `<sanitized note title>.tar.gz`，空标题为 `all.tar.gz`；Content-Disposition/Content-Type 由 interface Golden 冻结。

### R-C9 — Remote image fetch

- 只接受规范化 `http/https`，拒绝 userinfo、fragment、非 HTTP scheme；限制 DNS/connect/TLS/header/overall timeout、redirect 次数和响应字节。
- 每次 DNS、连接和 redirect 后重验目标，阻止 DNS rebinding/redirect 绕过；只接受成功状态和可解码图片，连接/body/temp 始终关闭。
- resolver、dial、TLS、response-header、overall、redirect count 和 compressed response bytes 使用 D-C2 单一 hard profile：DNS+dial `5s`、TLS handshake `5s`、response header `10s`、overall `30s`、最多 `5` 次 redirect，compressed body cap 为 `min(uploadImageSize, 32 MiB)`。相关配置 missing/invalid/zero/negative 时 fail closed；该 profile 是 Web/API/PDF remote-resource 的唯一事实来源，adapter 不得覆盖或放宽。
- 不信任 URL 文件名/扩展；由 decoder 与服务端 ID 生成目标并走统一 publish/metadata/cleanup。
- **D-C1 已确认（2026-09-15）：只允许公网。** 所有 resolved/connected address 必须是全局公网；拒绝 loopback、RFC1918/private、link-local、multicast、unspecified、Unix socket、云元数据及其他特殊用途地址。不得增加内网 host/CIDR allowlist 或默认放行开关。该安全修复明确不支持自托管部署从内网地址导入图片。

### R-C10 — Notes provider handoff

- provider 实现 notes 的 `ReconcileNote/CopyNote/DeleteNote`，每 step 提供 Apply/Verify；operation identity 与 committed generation 来自 notes receipt。
- Reconcile 对 API Files 三态执行 projection：missing 保持、present-empty 清空、present-nonempty 完整替换；只接收 owner-scoped validated asset IDs。
- Copy 使用 notes receipt 冻结的有序 source manifest；destination ID/path/digest 稳定，retry 不重新枚举后来新增源资产。
- Delete 的 content 清理是 note commit 后 repairable projection，不影响已提交 USN/tombstone；失败保持 pending/partial 并可 Verify。
- provider 不分配 USN、不写 history、不决定 note permission、不提交 notes receipt。

### R-C11 — PDF renderer

- Web/API 共用 renderer service；application 接收已授权 note snapshot，但 note content、标题、HTML/CSS 属性和其中的 URL 始终是不可信输入，不得标记为“可信 HTML”。旧 `Note.ToPdf` route 的 HTTP 兼容由 interface 决定，本任务停止把它当 credential transport。
- renderer 只接收由结构化 serializer 生成的 self-contained document：用户片段移除 script、事件属性、iframe/object/embed、meta refresh、CSS `url/@import` 和其他主动内容；仓库自带 CSS/Markdown 脚本按固定 digest 内联，已授权本地图片或经 R-C9 获取的公网图片在各自 size/decode budget 内转为 bounded data resource。最终文档不得含 `file:`、`http(s):`、protocol-relative 或其他可导航/取资源 URL。
- 单次 note HTML hard cap 为 32 MiB，最终 self-contained HTML hard cap 为 64 MiB，PDF artifact hard cap 为 64 MiB。资源加载前按去重后的引用集合累计原始字节与 base64 projected bytes；markup + projected resource bytes 超过最终文档上限时在继续读取/编码前返回 `too_large`。单项图片预算不能替代聚合预算，失败不得保留已加载资源或临时 artifact。
- `Note.ToPdf` 的 legacy `appKey` 仅可由 interface 为 wire compatibility 绑定和丢弃；content renderer、serializer、process、URL、日志、错误和 artifact 均不得读取、传播或验证该值。任何依赖 appKey 的 callback fetch 都是拒绝路径，不是兼容 fallback。
- executable 配置入口和 allow policy 的唯一事实来源归 admin；content 只消费 descriptor，并在执行时复核 canonical path、regular executable、无 symlink/reparse 替换及 descriptor policy identity，不能重新解析配置或建立第二套 allowlist。content 拥有 process、timeout/cancel、temp input/output 和 cleanup。
- direct executable + argv，禁止 `/bin/sh -c`、`cmd /C` 和字符串命令。HTML 经 stdin 或 request-scoped mode `0600` temp；启用 renderer 的 local-file-access deny，进程只可访问 request input/output，不能读取 content roots、应用配置或任意 OS path。secret/token 不进 URL、argv、env、log、error、artifact。
- 对 finalized document 执行结构化 policy 校验并在真实运行证据中证明零 outbound connection；`file://`、loopback/private/metadata URL、redirect、CSS resource 和 script-driven fetch 均必须失败且不能把目标内容带入 PDF。字符串扫描、CSP 或“当前模板不会请求”不能单独作为隔离证明。
- 仅在 exit success、output 存在、size 有界且 PDF magic/基本可读性通过后返回。stderr 有界脱敏；timeout、non-zero、invalid PDF、open/read/close/cleanup 各有错误。
- Markdown 保持 `--window-status done` 等必要语义且受总 timeout；文件名保持安全 title/`Untitled.pdf`，Content-Type `application/pdf`。
- local fake executable 只证明 argv/timeout；真实 Linux/container wkhtmltopdf artifact 由 delivery，未运行保持 delegated-unrun。

### R-C12 — Compatibility, observability and evidence

- 不改变公开 URL、response 字段、成功 binary disposition 或 Mongo Schema；无法保持的 wire 差异须另行确认。
- 错误只在终止边界记录一次，含安全 ID/category，不含文件内容、私密 title、token、完整路径或远程 URL query。
- 自动化、Mongo/filesystem integration、真实 HTTP、browser、跨主机/restart failpoint 和 Linux PDF artifact 分层记录；mock/旧 artifact/focused test 不互相替代。
- 当前 runner 无 wkhtmltopdf；CI 声明的 Debian package 版本不等于 runtime smoke。

## Current implementation baseline

| Area | 2026-09-17 state | Remaining completion boundary |
| --- | --- | --- |
| pure contracts/errors/path | implemented and focused-tested | action commands/results/repositories and all controller migrations |
| roots/store | typed adapter validation、opaque open、no-clobber publish、four-state Verify implemented | production-config typed binding、Linux/Windows indirection matrix、unsupported-fs/full failpoints |
| delete recovery | identity、manifest、no-replace/CAS store、terminal GC primitive implemented | image/attachment delete orchestration、Mongo/projection、restore/purge/restart matrix |
| media | bounded read、hard decode budget、GIF/JPEG/PNG/BMP validation implemented | multipart/action integration、metadata/ACL/publish/cleanup and D-C2 visible-name limits |
| PDF | self-contained serializer、descriptor、direct process、Web/API ExportPdf adapter implemented | aggregate resource pre-budget、R-C9 public remote resources、strong readability、real sandbox/Linux evidence |
| remote/archive/provider | not implemented | R-C8、R-C9、R-C10 in full |

基础 primitive 通过不等于任一 26 callable action、2 integration 或 4 provider 已完成；逐 action 完成状态只以 acceptance matrix 的绑定证据为准。

## Acceptance criteria

- [ ] AC-C1：26 个 callable Web/API action、2 个 ApiNote integration、4 个 provider primitive 均有 actor/owner、I/O、error、mapper、evidence；4 个 dormant surface 不重新发布。
- [ ] AC-C2：纯 `app/application/content` 无 Revel/Mongo/`app/db`/BSON/OS path 依赖；controller 不再拥有文件/tar/http/exec 业务。
- [ ] AC-C3：单一 roots/resolver 覆盖双分隔符 traversal、absolute/drive/UNC/device、reserved name、symlink/junction/reparse、deepest parent、legacy path、TOCTOU，以及 paired quarantine 的 same-filesystem、non-public、no-overlap 配置门禁。
- [ ] AC-C4：publish/Verify 覆盖 stable identity、SHA-256、unique same-dir staging、no-clobber/fsync、同/异摘要、concurrency、unsupported fs、unknown 及 file+row+projection；standalone delete 覆盖 durable manifest、legacy server-derived identity 和每个 process-loss 状态的恢复。
- [ ] AC-C5：upload/remote decode 覆盖 multipart presence/one-file/stream size、invalid config fail-closed、`DecodeConfig`、width/height/pixel/allocation/frame budget、真实 GIF/JPEG/PNG/BMP、伪扩展/截断/后续帧损坏、metadata、cleanup、foreign owner；album name、image title 与 attachment display filename 清理后按 UTF-8 编码不超过 `255 bytes`，超限返回 `validation` 且不截断，既有超限值可读/下载，rename/copy 不写入超限新值且失败时保留原值。
- [ ] AC-C6：image/attachment/album list/read/update/delete/copy 有 owner/blog/share matrix；`FromFileId` 不授权；invalid ID/open/DB error 不 panic/空成功。
- [ ] AC-C7：blank default album 的 list/upload 保持，rename/delete 服务端拒绝；name、same-owner、non-empty delete、foreign count 回归通过。
- [ ] AC-C8：Web/API 单附件和全量 `tar.gz` 覆盖安全/重复 entry、regular-only 与 normalized/no-host-metadata header、确定性顺序/输出、真实内容、close order、concurrency、无共享 temp、失败无 panic/截断成功。
- [ ] AC-C9：公网限定 fetch policy 覆盖 scheme/userinfo、DNS/connection/redirect 逐跳重验、private/loopback/link-local/metadata/特殊地址、DNS+dial `5s`、TLS handshake `5s`、response header `10s`、overall `30s`、最多 `5` 次 redirect、compressed body `min(uploadImageSize, 32 MiB)`、non-2xx、invalid image 和 cleanup；hard-profile 配置 missing/invalid/zero/negative 时 fail closed，实现中不存在内网 allowlist、默认放行开关或 Web/API/PDF adapter 放宽。
- [ ] AC-C10：provider 证明 reconcile/copy/delete/publish/verify 消费 stable operation/generation/manifest，Files 三态正确，unknown 先 Verify，且无第二 receipt/USN/history。
- [ ] AC-C11：Web/API 共用 PDF renderer；不可信 note HTML 经 self-contained serializer，用户主动内容被移除，资源先授权/受限后内联；32 MiB input、64 MiB final-document/artifact 与 pre-load aggregate projected-byte budget 生效；descriptor consumer、direct argv、stdin/0600、timeout/cancel、bounded stderr、PDF validation、cleanup、secret absence 与恶意 URL/active-content 的结构化拒绝 focused tests 通过。真实 local-file deny/零 outbound 运行证据按 AC-C14 交接，不由 fake executable 冒充。
- [ ] AC-C12：URL/body/binary filename/Content-Type/error mapper Golden 保持；route/method/status real HTTP 由 interface，页面/browser 由 presentation/delivery。
- [ ] AC-C13：Mongo/filesystem integration 证明 upload/copy/delete 多写失败、durable quarantine manifest、legacy no-OperationID restart、restore/purge/cleanup retry、concurrency；跨主机/restart 保持 delegated-unrun 直到 delivery 执行。
- [ ] AC-C14：Linux/container wkhtmltopdf、零 outbound/local-file deny、恶意 HTML 负向 PDF artifact、persistent volume、跨主机 failpoint 与真实 browser/HTTP 已交接 delivery，不以 local fake/build 替代。

## Out of scope

- 不更换 PDF renderer、不实现 ARM64、不自动部署生产。
- 不改变 URL、Mongo Schema、note USN/history/receipt、sharing 规则或前端 HTML escaping。
- 不恢复 `html2image`，不重新开放注释 ApiFile mutation/list actions。
- 不手改 `public/` bundle；若后续需要可见前端变化，由 presentation 与 Node 24 唯一 source/build 处理。

## Confirmed decision

### D-C2 — Visible text and remote-fetch hard profile

用户于 2026-09-17 确认采用推荐安全预算：

- trim/control/path 清理后的 album name、image title、attachment display filename 均最多 `255 UTF-8 bytes`；超限返回 `validation`，不静默截断。既有超限 row 仍可读取/下载；rename/copy mutation 遇到待写入的超限值时显式失败并保留原值。
- remote fetch 使用 DNS+dial `5s`、TLS handshake `5s`、response header `10s`、overall `30s`、最多 `5` 次 redirect；compressed body cap 为 `min(uploadImageSize, 32 MiB)`，任一配置 missing/invalid/zero/negative 时 fail closed。
- 上述限制由 content 单一 profile 暴露给 Web/API/PDF remote-resource 路径；adapter 不得各自覆盖或放宽。

D-C2 与 D-C1 共同关闭本轮 product/risk decision；后续实现和验收必须直接以这些量化约束为准。

## Continuation gate

任务已经是 `in_progress`，本轮不重复激活。D-C1 与 D-C2 均已确认，不再存在 blocking product decision；提交本次最新最终规划摘要并获得下一条消息中的明确开发批准前，仍不继续功能编码。规格审核、继续实现、质量检查、提交、归档和推送仍是独立门禁。
