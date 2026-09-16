# 应用层：内容、文件与媒体 — 执行计划

## Activation prerequisites

- [x] `09-08-domain-contracts`、`09-08-application-notes` 已归档。
- [x] action inventory、规格审核和 evidence matrix 已建立。
- [x] 用户确认 D-C1：远程图片仅允许公网 HTTP/HTTPS，不提供内网 allowlist。
- [ ] 把决策写回规格，提交最终规划摘要并获得明确开发批准。
- [ ] 仅随后执行 `task.py start`；此前不改业务代码。

## Phase 1 — Contract and safety seams

- [ ] 建立纯 `app/application/content` commands/results/errors/ports，并加 Revel/Mongo/`app/db`/BSON scan。
- [ ] 定义 `ContentRoots`、paired durable data/quarantine roots、logical grammar、legacy prefix 表和唯一 resolver；启动时验证 same-filesystem、non-public 与 no-overlap。
- [ ] 先写 traversal、absolute/drive/UNC/device、reserved、symlink/junction/reparse、deepest parent、TOCTOU failing tests，再实现 safe resolver/open。
- [ ] 实现 stable identity、streaming SHA-256、unique same-dir staging、no-replace、fsync、four-state Verify。
- [ ] 覆盖 existing same/different digest、concurrent、fixed-`.pending` regression、unsupported fs、dir sync、unknown result。

## Phase 2 — Upload, image and album

- [ ] 先写 multipart presence/one-file/stream limit/config-invalid，以及 `DecodeConfig`、checked dimension/pixel/allocation、GIF frame/cumulative budget 与 GIF/JPEG/PNG/BMP decoder regression。
- [ ] 把 image upload/paste/avatar/blog-logo primitive 移入 content；移除 extension-only、fixed-PNG、1000 MB fallback。
- [ ] 收敛 image list/title/read/delete/copy ACL；移除 `FromFileId` authorization，拆分 source/destination permission。
- [ ] 实现 stable lookup key → durable quarantine manifest → deterministic quarantine → owner DB/projection → purge/restore → terminal marker/7-day GC 的 delete flow；覆盖 legacy no-OperationID restart 和 retry/Verify。
- [ ] 收敛 album CRUD；服务端保护 blank default，验证 same-owner count/name。
- [ ] 实现 public-only remote DNS/dial/redirect/address/timeout/size/decode policy 和 SSRF tests；不加入内网 allowlist/fallback。

## Phase 3 — Attachment and archive

- [ ] attachment upload/list/read/delete/copy/reconcile 接入同一 roots/publish/Verify/ACL。
- [ ] strict parse IDs，区分 note owner/uploader，覆盖 share read/update。
- [ ] 实现单附件 opaque stream，显式传播 open/stat/close。
- [ ] Web/API 共用 deterministic `tar.gz` writer；覆盖 entry sanitization/duplicates、close order、concurrency、temp cleanup。
- [ ] 删除共享 `files/attach_all`/title temp、defer-in-loop、panic 和截断成功。

## Phase 4 — Notes provider

- [ ] 实现 notes `ContentAssetPort` adapter，不新增 receipt collection/state machine。
- [ ] Reconcile 覆盖 Files missing/empty/nonempty 和 file+row+projection Verify。
- [ ] Copy 消费 receipt-frozen ordered manifest，稳定 destination，retry 排除后来新增 source。
- [ ] Delete 是 post-commit repairable projection，失败不重复/回滚 USN、history、tombstone。
- [ ] API Add/Update 与 Web copy/delete 有 provider contract；跨进程/filesystem failpoint 交 delivery。

## Phase 5 — Unified PDF renderer

- [ ] 建 renderer request/artifact/error，明确 admin descriptor provider、content-owned self-contained serializer/policy validator 与 interface wire adapter。
- [ ] Web/API 迁到同一 service；停止 secret callback URL 和 duplicate controller exec。
- [ ] 先写 untrusted HTML active-content stripping、authorized bounded resource embedding、direct argv/metachar/space、executable validation、local-file deny、stdin/0600、timeout/cancel、bounded stderr tests。
- [ ] 验证 Markdown window-status、PDF magic/readability、filename/Content-Type 和所有 cleanup/error。
- [ ] Windows fake executable 只记 focused；Linux/container real wkhtmltopdf 验证零 outbound 及 file/private/metadata/redirect/CSS/script-fetch 负向 artifact，记 delivery delegated-unrun。

## Phase 6 — Cleanup and verification

- [ ] 核对 26 callable、2 integrations、4 providers；commented ApiFile actions 不重新路由。
- [ ] 以 reachability/build 证明后删除无消费者 `html2image`，并删除已迁移 controller business helpers。
- [ ] 运行 changed Go `gofmt`、focused/full tests、dependency scan、Mongo/filesystem integration、`go vet ./...`、`go build ./...`、`git diff --check`。
- [ ] 若改 frontend source，遵循 presentation/Node 24 唯一来源执行 `npm ci && npm run build && npm test`；否则不触碰 bundle。
- [ ] 交接 real HTTP route/method/status/Content-Type、browser、cross-host/restart、volume、Linux PDF；未跑保持 delegated-unrun。

## Evidence record

每次记录 candidate commit/tree、UTC time、OS、Mongo version/topology、command、exit code、discovered/executed/pass/fail/skip 和脱敏原因。build、mock、历史结果、CI 包声明不能替代真实 HTTP/browser/PDF/failpoint evidence。
