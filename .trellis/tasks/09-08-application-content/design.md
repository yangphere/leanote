# 应用层：内容、文件与媒体 — 技术设计

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

standalone delete 另有 content-owned durable quarantine manifest。它与 quarantine 文件位于对应 data root 配对的 non-public quarantine root、mode `0600`、以 stable delete identity no-replace 发布，并保存恢复所需的 owner/asset/path/digest/generation/stage；paired roots 必须同 filesystem/volume 且不能落在 static-serving namespace。manifest 不保存 USN/history，也不得用于 notes provider 的第二份 receipt。

## 3. Safe logical path algorithm

1. 以 platform-neutral grammar 解析 root kind/segments；在 OS clean 前拒绝两种分隔符及非法 segment。
2. 拒绝 empty required segment、dot、NUL/control、absolute、drive-relative、UNC/device/volume、reserved name。
3. startup canonicalize 明确配置 root 并保留 root identity/handle；逐个验证 data/quarantine 配对同 filesystem、quarantine 不可被 Web/static 映射且所有 root 无重叠。
4. walk 到 deepest existing ancestor；每段不得通过 symlink/junction/reparse 逃逸。
5. 按 segment 创建缺失目录并复核；open/read/publish 前再次验证 final parent/object。
6. legacy string 先按 `files/`、`public/upload/`、`upload/` 显式表映射，再走相同算法；未知 prefix fail closed。

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
- directory sync failure 为 unknown，不能报告 durable success。
- Verify 顺序：destination bytes → owner-scoped metadata row → projection；dependency error 返回 unknown。

## 5. Multi-write plans

### Upload/copy

validate principal/owner/input/size/media/identity → publish bytes → insert/verify owner row → apply/verify projection → typed result。DB/projection unknown 先 Verify；确定失败只清理本 operation 新建且未被引用的文件。

### Delete

owner-scoped lookup/ACL → derive owner-scoped lookup key/identity → no-replace durable manifest → same-filesystem deterministic quarantine → mutate/verify projection+metadata → DB fail 则 restore，DB success 则 purge → final Verify → terminal marker → retention GC。restore/purge/GC failure 为 partial/unknown，同 identity repair。此流程替代当前 DB-first delete。

lookup key 只由 action、owner、asset kind/ID 派生，row 删除后仍可定位 manifest。显式 `OperationID` 按 owner/action scope 纳入 identity；legacy action 没有 ID 时使用 lookup key + row generation/digest。重启按下表恢复，不能依赖内存中的 `QuarantineHandle`：

| Source | Quarantine | Manifest | Row/projection | Result |
| --- | --- | --- | --- | --- |
| present | absent | present/absent | original | 继续/重做 quarantine |
| absent | present | present | original/partial | 继续 mutation 或在确定失败后 restore |
| absent | present | present | desired | purge 后 Verify |
| absent | absent | terminal | desired | retention 内 replay applied |
| absent | absent | absent | missing | legacy compatible not-found，不推断 applied |
| 其他组合或 dependency read error | 任意 | 任意 | 任意 | conflict/unknown，禁止猜测 |

manifest 状态带 version+digest；每次转换通过同目录 unique mode `0600` staging、file sync/close、atomic replace、directory sync 和 version CAS/identity lease 单调发布。torn/corrupt/stale version 保持 conflict/unknown，不原地 truncate 或静默重建。active manifest 在 final Verify 后原子缩减为不含 path/content 的 terminal marker，默认保留 7 天；GC 获取同一 lease 并复核 version/age，以 lookup key 幂等执行并对失败告警，避免永久残留或与 retry 竞态。

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

关闭顺序：entry file → tar → gzip → sink/file。所有 write/close error 都显式失败。若 HTTP headers 已提交后的 stream 失败，interface 记录 stream failure，不追加伪 JSON。并发请求不共享 temp/name。

## 10. Remote fetch

采用已确认的 `public-only`：custom client、受控 redirect、resolver/dial hook、每 hop address validation、bounded body、decoder 和全路径 cleanup。resolved/connected IP 必须是全局公网；private、loopback、link-local、multicast、unspecified、metadata 与其他特殊地址全部拒绝。

不实现 admin host/CIDR allowlist、内网兼容开关或任意 URL fallback。URL 字符串检查、仅检查第一次 DNS、连接到未验证地址或自动信任 redirect 均不满足契约。

## 11. PDF boundary

1. application 取得已授权 note snapshot；note-derived title/content/HTML/CSS/URL 始终不可信。
2. 专用 serializer 生成与现有 `file/pdf.html` 语义等价的 self-contained document：移除用户 script/event/iframe/object/embed/meta refresh/CSS 外链；仓库自带、digest 固定的 CSS/Markdown runtime 内联；已授权或按 R-C9 获取且通过 decode budget 的图片内联为 bounded data resource。
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
- `html2image`/commented ApiFile 只做 reachability cleanup，不新增 route。
- rollback 以子域为单位；调用方迁移后不得单独回退 safety resolver/no-clobber/secret tests。
