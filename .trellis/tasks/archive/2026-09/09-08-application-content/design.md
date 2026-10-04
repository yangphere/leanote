# 应用层：内容、文件与媒体 — 技术设计

## 0. Current implementation map

`158a6dee7319d0dd3e09007fd623190525f28977` 已实现 application content 基础 contract、portable logical path、validated roots、opaque open、no-clobber publish/four-state Verify、standalone delete manifest/CAS store、media hard budget，以及 self-contained PDF serializer/process/Web+API adapter。它没有完成 image/attachment/album action 迁移、delete orchestration、remote fetch、archive writer 或 notes provider。

后续开发必须扩展现有 primitive，不能在 controller/service 旁边再建第二套 path、publish、manifest、media 或 PDF 规则。实施状态以 `implement.md` 和 `acceptance/evidence-matrix.md` 为准；本节只记录 architecture baseline。

## 1. Boundaries

```text
Web/API adapter
  principal + presence binding + legacy mapper
                         |
                         v
app/application/content (pure use cases/contracts)
  authorization + lifecycle + typed errors
          |                    |                    |
          v                    v                    v
metadata repository      content store        PDF/remote ports
  Mongo adapter          filesystem adapter   exec/http adapters
          ^
          |
application-notes durable receipt -- Apply/Verify provider
```

- `app/application/content` 只依赖 context、领域 ObjectID/value object 和自有 ports。
- controller 不构造 OS path、不读全量上传、不打 archive、不启动进程、不直接访问 collection。
- Mongo/filesystem/HTTP/process 都可注入；failpoint test 不冒充真实环境。
- notes receipt 是 note recovery 的唯一 durable coordinator；content 对同 identity/generation deterministic Apply/Verify。

## 2. Core contracts

目标语义如下，具体 Go 名称在编码前按相邻包定稿：

```go
type DurableContentRoot struct {
    Data       LogicalRoot
    Quarantine LogicalRoot // same filesystem, outside any served namespace
}

type ContentRoots struct {
    PrivateFiles DurableContentRoot
    PublicUpload DurableContentRoot
    Temporary    LogicalRoot
}

type AssetIdentity struct {
    OperationID   string
    Generation    int
    OwnerID       domain.ObjectID
    Kind          AssetKind
    SourceID      string
    DestinationID domain.ObjectID
    Digest        [32]byte
}

type VerificationStatus string // applied/not_applied/conflict/unknown

type ContentStore interface {
    Resolve(LogicalPath, AccessMode) (ResolvedHandle, error)
    Publish(context.Context, PublishRequest) (PublishResult, error)
    Verify(context.Context, VerifyRequest) (FileVerification, error)
    Quarantine(context.Context, DeleteRequest) (QuarantineHandle, error)
    Restore(context.Context, QuarantineHandle) error
    Purge(context.Context, QuarantineHandle) error
}

type AssetRepository interface {
    // owner-scoped image, attachment, album and projection operations
}

type ResourcePermissionPort interface {
    CanReadNote(context.Context, ObjectID, ObjectID, ObjectID) (bool, error)
    CanUpdateNote(context.Context, ObjectID, ObjectID, ObjectID) (bool, error)
    IsPublishedReference(context.Context, ObjectID, ObjectID) (bool, error)
}
```

`ResolvedHandle` 不把 absolute path 返回 application/controller；仅 filesystem adapter 持有。DTO 返回 logical path 或 opaque stream/artifact。

standalone delete 另有 content-owned durable quarantine manifest。它与 quarantine 文件位于对应 data root 配对的 non-public quarantine root、mode `0600`、以 stable delete identity no-replace 发布，并保存恢复所需的 owner/asset/path/digest/size/generation/stage；paired roots 必须同 filesystem/volume 且不能落在 static-serving namespace。manifest 不保存 USN/history，也不得用于 notes provider 的第二份 receipt。

## 3. Safe logical path algorithm

1. 以 platform-neutral grammar 解析 root kind/segments；在 OS clean 前拒绝两种分隔符及非法 segment。
2. 拒绝 empty required segment、dot、NUL/control、absolute、drive-relative、UNC/device/volume、reserved name。
3. startup canonicalize 明确配置 root 并保留 root identity/handle；逐个验证 data/quarantine 配对同 filesystem、quarantine 不可被 Web/static 映射且所有 root 无重叠。
4. walk 到 deepest existing ancestor；每段不得通过 symlink/junction/reparse 逃逸。
5. 按 segment 创建缺失目录并复核；open/read/publish 前再次验证 final parent/object。
6. legacy string 先按 `files/`、`public/upload/`、`upload/` 显式表映射，再走相同算法；未知 prefix fail closed。

生产 root 配置由 `interface-http` production-config seam 绑定为 typed input；content adapter 负责验证和持有 handle。当前从 application base 派生固定目录的 initializer 只保留到 typed production initializer 接入，不得成为第二套生产配置或 fallback。

Windows/POSIX lexical cases 跨平台单测；symlink/junction/reparse 用 host integration。当前主机跑不了的用例保持 planned/delegated。

## 4. Publish/Verify state

```text
absent -> stage/write/sync/close -> staged -> no-replace -> published -> row/projection -> applied
              |                              |                         |
              v                              v                         v
         not_applied              unsupported/conflict           Verify all
```

- `CreateTemp(destinationDir, opaquePrefix)` 等价方式生成 unique staging，禁止固定 `.pending`。
- no-replace primitive 无法证明时返回 `unsupported_filesystem`。
- 写时 SHA-256，发布后完整重读。same identity+digest 幂等成功；digest 不同 conflict。
- durable publication 按平台使用可证明的 barrier：Unix/Linux 在 staging file sync/close 与原子 link/replace 后同步目标父目录；Windows 在 link/replace 后通过既有 `os.Root` 以 `O_RDWR` 独立重开最终业务文件并执行 `File.Sync`/`FlushFileBuffers`，open/sync/close 任一步失败均为 unknown，不能报告 durable success。Windows 新建父目录在 child publication 前仅为 provisional，空目录项丢失可由幂等重试重建，不对普通目录句柄调用会返回 `Access is denied` 的 `File.Sync`。
- Verify 顺序：destination bytes → owner-scoped metadata row → projection；dependency error 返回 unknown。

## 5. Multi-write plans

### Upload/copy

validate principal/owner/input/size/media/identity → publish bytes → insert/verify owner row → apply/verify projection → typed result。DB/projection unknown 先 Verify；确定失败只清理本 operation 新建且未被引用的文件。

### Delete

owner-scoped lookup/ACL → derive owner-scoped lookup key/identity → no-replace durable manifest → digest+size verified deterministic quarantine → mutate/verify projection+metadata → DB fail 则 restore，DB success 则 purge → final Verify → exact opened-handle removal → terminal marker → retention GC。不得在 Verify 与按名称 unlink 之间留下 replacement window；平台不能针对同一已验证句柄删除时返回 `unsupported_filesystem`，不做路径 fallback。restore/purge/GC failure 为 partial/unknown，同 identity repair。此流程替代当前 DB-first delete。

lookup key 只由 action、owner、asset kind/ID 派生，row 删除后仍可定位 manifest。显式 `OperationID` 按 owner/action scope 纳入 identity；legacy action 没有 ID 时使用 lookup key + row generation/digest。重启按下表恢复，不能依赖内存中的 `QuarantineHandle`：

| Source | Quarantine | Manifest | Row/projection | Result |
| --- | --- | --- | --- | --- |
| present | absent | present/absent | original | 继续/重做 quarantine |
| absent | present | present | original/partial | 继续 mutation 或在确定失败后 restore |
| absent | present | present | desired | purge 后 Verify |
| absent | absent | terminal | desired | retention 内 replay applied |
| absent | absent | absent | missing | legacy compatible not-found，不推断 applied |
| 其他组合或 dependency read error | 任意 | 任意 | 任意 | conflict/unknown，禁止猜测 |

manifest 状态带 version+digest；create manifest 另冻结 adapter canonicalized owner-row metadata digest，只有所有稳定 ownership/output 字段完全匹配才是 exact。每次转换通过同目录 unique mode `0600` staging、file sync/close、atomic replace、platform publication barrier 和 version CAS/identity lease 单调发布；Windows 对最终 manifest 文件重开并 flush，Unix/Linux 同步父目录。torn/corrupt/stale version 保持 conflict/unknown，不原地 truncate 或静默重建。active manifest 在 final Verify 后原子缩减为不含 path/content 的 terminal marker，默认保留 7 天；GC 获取同一 lease 并复核 version/age，以 bounded `ReadDir(n)` 流式扫描、minute-seeded 64-bit lookup-key ring ranking 和 active/terminal 独立预算执行，禁止 `ReadDir(-1)`、shard 内外固定前缀 starvation 或第二 durable cursor。Windows terminal GC/staging cleanup 没有可用的普通目录 flush，只依赖安全重现与幂等重试，不得据此宣称业务 mutation durable；lifecycle 的业务删除另以 root-relative 可写目录句柄 `FlushFileBuffers` 作为 removal barrier。

identity lease 与 manifest/data durable commit 的权威边界不同：永久 `.lease` 文件上的 `LockFileEx`/`flock` 是 live mutual exclusion authority，进程终止由内核释放；owner/epoch record 只作诊断与 fencing。lease 路径只创建、不删除，record 使用 unique stage、file sync、close 和 atomic rewrite，malformed record fail closed。若崩溃使新 lease/record 的 parent directory entry 未持久化，业务 manifest/row/content 均未因此丢失，且原内核锁已释放；后继可安全重建 lease/record。因此 lease/record parent directory 不套用 manifest/data 的强制 directory-sync unknown 规则，后者仍严格 fail closed。

### Notes provider

notes 传 committed generation、receipt-owned operation ID 和 frozen manifest。content Apply 不写 receipt，Verify 不改变 notes 状态。DeleteNote 是 post-commit repairable cleanup，失败不回滚 USN/tombstone。

## 6. Upload/media pipeline

```text
bounded multipart stream
 -> safe display name
 -> extension + sniff + decoder
 -> stable ID/logical destination
 -> no-clobber publish
 -> owner row/projection
```

- limit reader 读取 `configuredLimit+1` 并在超限停止；invalid config 先失败。
- 只接受一个 `file` part；missing/empty/multiple 保留 typed detail。
- decoder 是有效性事实来源，但压缩字节上限不等于解码预算。先 `DecodeConfig` 并以 checked multiplication 应用内建 hard profile：width/height 各不超过 `16384`、总像素不超过 `25,000,000`、预计 allocation 不超过 `128 MiB`；GIF 最多 `256` 帧且累计像素不超过同一上限。未配置 override 时直接使用 hard profile；可选配置只能降低，随后完整 decode/DecodeAll 仍必须验证实际结构与预算。
- 原名只作 display metadata；storage/archive name 均服务端生成/净化。

## 7. Authorization matrix

| Operation | Source read | Destination write |
| --- | --- | --- |
| image list/title/delete | `(FileID, ActorID)` owner query | actor owner |
| note image/attachment read | owner row + note reference | public reference 或 publishing read port |
| paste to shared note | actor owns initial image | note owner from update-permission port |
| copy/shared-copy | source owner/read port | committed destination note owner |
| attachment upload/delete | note update permission | note owner，uploader 不是 owner |
| album CRUD | `(AlbumID, ActorID)` | actor owner |
| PDF | note read permission | request artifact only |

`FromFileId`、`UploadUserID` 与客户端 user IDs 不授权。

## 8. Album

- default album 是 application value，wire filter 为 blank ID，不落 Mongo。
- blank update/delete 返回 protected validation；persisted ID strict parse/owner query。
- delete 先 owner-scoped count 再 owner-scoped delete；count error 不是 has-images/empty success。
- name 仅 trim/control/length；escaping 留给 presentation。

## 9. tar.gz

先加载已授权、deterministic attachment manifest，再依序 safe-open。entry 只用跨平台安全 base name；重复名按 manifest 顺序生成 `name (2).ext`。禁止 separator、PAX link/device entry。

每个 entry 固定为 regular file，mode 归一化，uid/gid/uname/gname/link target 清空；mtime 使用稳定业务时间或固定 epoch。不得把宿主文件的 mode、owner、mtime、ACL 或扩展属性写入 archive。相同 manifest 与内容产生相同 entry 顺序和 header。

关闭顺序：entry file → tar → gzip → sink/file。所有 write/close error 都显式失败。若 HTTP headers 已提交后的 stream 失败，interface 记录 stream failure，不追加伪 JSON。并发请求不共享 temp/name。

## 10. Remote fetch

采用已确认的 `public-only`：custom client、受控 redirect、resolver/dial hook、每 hop address validation、bounded body、decoder 和全路径 cleanup。resolved/connected IP 必须是全局公网；private、loopback、link-local、multicast、unspecified、metadata 与其他特殊地址全部拒绝。

D-C2 已确认统一 hard profile：DNS+dial `5s`、TLS handshake `5s`、response header `10s`、overall `30s`、最多 `5` 次 redirect，compressed body 为 `min(uploadImageSize, 32 MiB)`；missing/invalid/zero/negative 配置 fail closed。实现把这些值集中为 Web/API/PDF remote-resource 共用且 adapter 不可放宽的单一 profile，并用 fake clock/resolver/dialer 覆盖每个边界。

不实现 admin host/CIDR allowlist、内网兼容开关或任意 URL fallback。URL 字符串检查、仅检查第一次 DNS、连接到未验证地址或自动信任 redirect 均不满足契约。

## 11. PDF boundary

1. application 取得已授权 note snapshot；note-derived title/content/HTML/CSS/URL 始终不可信。
2. 专用 serializer 生成与现有 `file/pdf.html` 语义等价的 self-contained document：移除用户 script/event/iframe/object/embed/meta refresh/CSS 外链；仓库自带、digest 固定的 CSS/Markdown runtime 内联；已授权或按 R-C9 获取且通过 decode budget 的图片内联为 bounded data resource。
   - note HTML 输入最多 32 MiB；最终 self-contained HTML 和 PDF artifact 各最多 64 MiB。
   - 先去重引用并累计 raw/projected-base64 bytes；预计 markup + encoded resources 超过 64 MiB 时停止继续加载，避免“先读入大量资源、最后才拒绝”。
3. structured policy validator 拒绝 finalized document 中的 `file:`、`http(s):`、protocol-relative、可导航 URL 或非内联资源；字符串扫描/CSP 只能作补充门禁。
4. `Note.ToPdf` legacy `appKey` 只在 interface bind 后丢弃；content 永不读取/传播/验证，任何 callback URL fetch 均拒绝。
5. admin 是 executable config/allow policy 的唯一事实来源并提供 descriptor；content 不解析配置或复制 allowlist，只在 use-time 复核 canonical path、regular executable、无 symlink/reparse 替换及 descriptor policy identity。admin 不执行 renderer。
6. direct argv；HTML 走 stdin 或 mode `0600` request temp，output 为 request temp；启用 local-file-access deny，renderer 不能访问 content roots/配置/任意 OS path。Markdown 保留 `--window-status done` 和固定内联 runtime，共享 context timeout。
7. process exit 后验证 output size、PDF magic/basic reader，再交 opaque artifact 给 adapter；真实 Linux/container 记录零 outbound connection，并以 file/private/metadata/redirect/CSS/script fetch 恶意输入验证隔离。
8. response 完成/cancel/timeout/non-zero/invalid/open/close 都清理；stderr 有界脱敏。

不再生成含 appKey 的 callback URL。handoff 已同步为 admin 只拥有配置入口/descriptor/权限，content 是唯一 renderer execution owner；admin task 依赖 content contract，不得复制 process seam。

## 12. Error mapping

| Category | Meaning | Adapter rule |
| --- | --- | --- |
| validation/too_large/unsupported_media | input/limit | 保持 action-specific failure body |
| unauthorized/not_found | ACL/absent | 可按旧契约隐藏存在性 |
| unsafe_path/conflict | escape/digest mismatch | fail closed，不暴露 path/digest |
| storage_unavailable/dependency | filesystem/Mongo/port | 不变为空成功/not-found |
| unsupported_filesystem | 无 durable no-clobber 保证 | 显式运维失败 |
| timeout/renderer_failed | external process/network | Web/API 分别映射 |
| partial_write/unknown_result | 多写不完整/结果不可知 | 保留 identity，Verify/repair |

错误只在 outer terminal boundary 记录一次；内部 wrapping 保留 cause，public error 不含 content、secret、URL query 或 absolute path。

## 13. Compatibility and rollout

- action inventory 冻结 URL、observed method、input presence、body/binary filename/Content-Type；content test 证明 category，interface test 证明 mapper。
- 按 contract/root → upload/image/album → attachment/archive → notes provider → PDF 的顺序迁移；每段完成后删除旧 controller 业务，不长期双实现。
- 已提交 foundation 后的实际顺序为：按 D-C2 完成 root production binding/failpoints → upload/image/album/delete integration → attachment/archive → notes provider → remote-resource/PDF hardening → cleanup。PDF 已接入的部分不得被后续阶段回退到 callback URL 或 controller exec。
- `html2image`/commented ApiFile 只做 reachability cleanup，不新增 route。
- rollback 以子域为单位；调用方迁移后不得单独回退 safety resolver/no-clobber/secret tests。
