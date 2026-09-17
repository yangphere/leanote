# Content action inventory

## Rules

- Current method 来自 generic `*` route 或 first-party caller，不代表未来 method 决策；method/status/Content-Type 由 `interface-http` 冻结。
- principal 均由 session/token 注入；当前 `userId/toUserId` 是不可信参数，目标 contract 不以其授权。
- response 逐 action 保持 `Re`、`ApiRe`、direct JSON/boolean、text/template 或 binary；content 返回 typed result。
- evidence 以 `acceptance/evidence-matrix.md` 为准；foundation focused tests 可为 partial，不等于对应 action passed。

## 2026-09-17 implementation overlay

- C-W22/C-A04 已接入同一 PDF application service 并有 focused adapter tests，但真实 HTTP、强 readability、remote resources 与 Linux sandbox 未闭合，状态仍为 partial。
- C-W01～C-W10 中 Web upload/paste/avatar/blog-logo、list/title、read、delete、copy-http 已接入 content/service primitive；`CopyImage` 只允许 server-verified note destination 或 same-actor legacy request，缺 target note identity 的 cross-owner legacy request fail closed。Mongo/HTTP/browser/restart evidence仍 partial。
- C-W16～C-W20 album CRUD 已接 strict owner-scoped service boundary；C-A01 image read 共用同一授权读取链。ApiNote stable image/attachment multipart 由 controller presence/open 转入 `AttachService` 的 pre-note create-repair lifecycle；C-A02/C-A03 attachment download 已在上一批迁移。
- C-P01 的 filesystem publish/Verify primitive 已实现；C-P02～C-P04 已由一个 production `ContentAssetPort` adapter 接入既有 notes receipt，Reconcile/Copy/Delete 各有 Apply/Verify，但 real Mongo unknown-result、restart 与跨 filesystem evidence 仍 partial/delegated-unrun。
- Storage reliability follow-up：C-W01～C-W04、C-W10 与无 `OperationId` 的 C-W11 已接同一 create-repair manifest；generated identity 只支持 server-side bounded recovery，不改变 legacy client retry 语义。C-W07 delete manifest 使用 crash-recoverable shard OS lock/owner epoch，delete/create terminal GC 接同步 startup maintenance；真实 Mongo/Linux/restart/cross-host evidence 仍 partial/open。
- C-D01～C-D04 决策不变；不得用“尚未迁移”作为重新发布 dormant surface 的理由。
- 静态复核数量为 22 Web + 4 API callable、2 ApiNote integrations、4 provider primitives，未发现新增 route/action。Web attachment upload 的 actor/note strict parse、asset identity、logical path 与 metadata construction 已下沉 service；匿名 `DownloadAll` 与单附件读取统一允许 public-note authorization。
- content runtime initializer 只接受 typed `ContentRoots`；filesystem store 与 PDF backend 消费同一次 root validation 返回的 canonical temporary path。`interface-http` 仍负责把唯一 production-config source 绑定到该 contract；当前 entrypoint application-base compatibility mapping 不是完成证据。
- ApiNote Add/Update 通过 C-P02 provider 收敛 projection/Verify；multipart body 的 stable image/attachment publish、finalize 与 create-failure cleanup 也已由 `AttachService` 使用 typed create-repair/delete lifecycle。controller 只保留 presence/open 与旧 wire 映射；real HTTP/Mongo/restart evidence 仍未完成，不能据此关闭 C-I01/C-I02。

## Callable Web actions (22)

| ID | Action/URL | Input | Actor/owner and target | Compatible output | Evidence |
| --- | --- | --- | --- | --- | --- |
| C-W01 | File.UploadBlogLogo `/file/uploadBlogLogo` | multipart file | session actor；actor public-logo primitive | template ViewArgs | upload/media/cleanup + HTTP |
| C-W02 | File.PasteImage `/file/pasteImage` | file, optional noteId | actor owns initial；shared owner from update port | Re + stable image ID | owner/share/copy/provider |
| C-W03 | File.UploadAvatar `/file/uploadAvatar` | file | actor upload；identity owns avatar/session commit | Re/demo mapper | cross-domain + identity replay |
| C-W04 | File.UploadImageLeaui `/file/uploadImageLeaui` | file, blank/persisted albumId | actor owner | Re + File DTO, no path | decoder/album/iframe |
| C-W05 | File.GetImages `/file/getImages` | observed GET; albumId,key,page | actor-only list/filter | direct Page, page size 12 | owner/paging/search Golden |
| C-W06 | File.UpdateImageTitle `/file/updateImageTitle` | observed POST; fileId,title | owner-scoped | Re.Ok | ID/title/foreign |
| C-W07 | File.DeleteImage `/file/deleteImage` | observed GET; fileId; no client operation ID | owner delete lifecycle；server-derived lookup key/identity | Re Ok/Msg | durable quarantine manifest/terminal marker/DB/projection/retry/restart |
| C-W08 | File.OutputImage `/file/outputImage` | noteId,fileId | owner/blog/share reference | inline binary; non-leaking miss | ACL/path/Content-Type |
| C-W09 | File.CopyImage `/file/copyImage` | current userId,fileId,toUserId | server-derived source/destination permission | Re Ok/Id | spoof negative + Apply/Verify |
| C-W10 | File.CopyHttpImage `/file/copyHttpImage` | src URL | actor destination；D-C1 public-only policy | Re Ok/Id/Msg | SSRF/redirect/size/decode |
| C-W11 | Attach.UploadAttach `/attach/uploadAttach` | noteId,file,optional OperationId | update actor；note owner | Re + Attach DTO, no path | bounded/stable/provider |
| C-W12 | Attach.DeleteAttach `/attach/deleteAttach` | attachId,optional OperationId | owner/verified updater per baseline | Re Ok/Msg | ACL/quarantine/Verify |
| C-W13 | Attach.GetAttachs `/attach/getAttachs` | noteId | note read permission | Re Ok/List | private/shared/public/DB error |
| C-W14 | Attach.Download `/attach/download` | attachId | note/attach read | binary/title; legacy miss text | ACL/path/disposition |
| C-W15 | Attach.DownloadAll `/attach/downloadAll` | noteId | note read | safe-title.tar.gz/all.tar.gz | tar/names/close/concurrency |
| C-W16 | Album.Index `/album/index` | none | authenticated actor | album template | interface/browser |
| C-W17 | Album.GetAlbums `/album/getAlbums` | observed GET | actor list | direct array | owner/error/order Golden |
| C-W18 | Album.DeleteAlbum `/album/deleteAlbum` | observed GET; albumId | owner persisted；blank protected | Re；has images | default/foreign/count |
| C-W19 | Album.AddAlbum `/album/addAlbum` | observed GET; name | actor/server ID | Album or false | name/owner/DB/wire |
| C-W20 | Album.UpdateAlbum `/album/updateAlbum` | observed GET; albumId,name | owner persisted；blank protected | boolean | default/name/foreign |
| C-W21 | Note.ToPdf `/note/toPdf` | noteId, current appKey | legacy callback；target renderer不用 secret URL，route auth/compat 由 interface 冻结 | template/text route delegated | no renderer dependency/secret |
| C-W22 | Note.ExportPdf `/note/exportPdf` | noteId | owner/blog/share read | PDF safe title/Untitled; text error | renderer + HTTP/Linux |

## Callable API actions (4)

| ID | Action/URL | Input/owner | Output | Evidence |
| --- | --- | --- | --- | --- |
| C-A01 | ApiFile.GetImage `/api/file/getImage` | token actor, fileId, owner/blog/share | inline binary/current empty miss | API ACL/path/Content-Type |
| C-A02 | ApiFile.GetAttach `/api/file/getAttach` | token actor, fileId, note read | binary/title/current No Such File | ACL/disposition/error |
| C-A03 | ApiFile.GetAllAttachs `/api/file/getAllAttachs` | token actor, noteId/read | tar.gz/current empty miss | shared archive + artifact |
| C-A04 | ApiNote.ExportPdf `/api/note/exportPdf` | token actor, noteId/read | PDF or ApiRe error | shared renderer + mapper/Linux |

## ApiNote asset integrations (2)

| ID | Receiver/input | Content responsibility | Evidence |
| --- | --- | --- | --- |
| C-I01 | ApiNote.AddNote；Files + optional stable identities | publish only through provider；owner from note command | create transaction/standalone + row/projection Verify |
| C-I02 | ApiNote.UpdateNote；Files missing/empty/nonempty + receipt generation | Reconcile Apply/Verify，不复制 receipt | three-state + unknown retry/conflict |

## Provider primitives (4)

| ID | Primitive/input | Result contract | Consumer evidence |
| --- | --- | --- | --- |
| C-P01 | Publish/Verify；operation,generation,owner,kind,source,destination,SHA-256 | no-clobber；four-state file+row+projection | upload 与所有 note steps |
| C-P02 | ReconcileNote；receipt identity,note owner/ID,Files presence/manifest | deterministic；unknown first Verify | ApiNote update receipt |
| C-P03 | CopyNote；receipt identity/generation,source/destination,frozen manifest | stable destinations；不 re-enumerate | copy/shared-copy receipt |
| C-P04 | DeleteNote；receipt identity/generation,owner/note,cleanup manifest | post-commit repairable；不写 USN/history | permanent-delete recovery |

## Dormant/not-to-publish surfaces (4)

| ID | Surface/evidence | Decision |
| --- | --- | --- |
| C-D01 | historical ApiFile.CopyImage block；无 route/registry/caller | 注释实现已删除；do not route/revive |
| C-D02 | historical ApiFile.GetImages block；无 route/registry/caller | 注释实现已删除；do not route/revive |
| C-D03 | historical ApiFile.UpdateImageTitle/DeleteImage block；无 route/registry/caller | 注释实现已删除；do not route/revive |
| C-D04 | historical `app/lea/html2image`；全仓仅 self references，无 caller/route | package 已删除；不恢复 |

## Evidence ownership

- content：pure contract、focused unit/integration、local filesystem/Mongo failpoints、provider consumer contract。
- interface：http registry/method/binder/status/Content-Type/body/binary 与真实 HTTP。
- presentation：album iframe/visible escaping 及 source/generated assets。
- delivery：real browser、Mongo topology、restart、cross-host/volume filesystem、Linux/container wkhtmltopdf artifact。
