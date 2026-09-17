# `09-08-application-content` 需求规格审核（2026-09-15）

> 历史快照：本文记录激活前审核，状态字段不得作为当前事实。2026-09-17 当前态与剩余门禁见 `spec-audit-2026-09-17.md`。

## 结论

该 leaf 的依赖与轨道条件已满足，但原规格不足以直接开发：缺少 action/input/output/error inventory、统一路径与多写恢复算法、远程抓取安全策略、真实 `tar.gz` 契约、PDF owner 划分和 evidence matrix。本次已在任务目录内补齐，未修改业务代码。

任务仍为 `planning`。Q-C1 已于 2026-09-15 由用户采用推荐方案并记录为 D-C1；获得用户对本版最终规划摘要的后续明确开发批准前，不运行 `task.py start`。

## Ready 与依赖证据

- task：`09-08-application-content`，P1，`status=planning`，`children=[]`。
- `task.json.meta.depends_on`：`09-08-domain-contracts`、`09-08-application-notes`；两者均已归档 completed。
- 应用轨道 identity → notes → content → publishing → admin；identity 已在进行、notes 已完成，content 是首个可激活 planning leaf。

## 审核方法与限制

- 读取父任务、直接依赖、notes action/evidence、interface/admin/delivery sibling 规格。
- 检查 File/Attach/Album/ApiFile/Note/ApiNote controllers、File/Attach/NoteImage/Album services、notes contracts、net util、routes、album source 和 html2image reachability。
- `jbcontext search` 当前返回 404，FastCtx MCP 未暴露；转用定向 `rg` 和带行号读取。
- 当前 Windows runner 无 `wkhtmltopdf`；真实 renderer 未执行。

## 原规格问题与处理

| ID | 问题与证据 | 完善结果 |
| --- | --- | --- |
| A-01 | `conf/routes:143-145` generic actions，原规格无逐 action I/O/owner/evidence | 新增 action inventory，分 callable/integration/provider/dormant |
| A-02 | `FileController.go:93-224` 同时绑定、校验、写文件、建 DB model | R-C1 冻结纯 application 与 adapter 边界 |
| A-03 | `FileService.go:107-114,180-250` string path；`FromFileId` 参与访问 | R-C2 建单一 roots/resolver；R-C6 禁止 lineage 授权 |
| A-04 | `AttachService.go:138-184` 固定 `target+.pending`，filesystem capability/Verify 不完整 | R-C3 定义 unique same-dir stage、no-replace、fsync、four-state Verify |
| A-05 | `FileService.go:99-122` DB-first delete 后 unlink，不可恢复 | R-C4 定义 quarantine/restore/purge 与 partial/unknown |
| A-06 | `FileController.go:153-181` extension-only/paste PNG/1000 MB fallback；Attach 同类 | R-C5 要求 bounded stream、真实 decode、invalid config fail-closed |
| A-07 | controller copy 接受多个 user ID，owner/source/destination 混淆 | R-C6 收敛 owner/blog/share port，客户端 user ID 不授权 |
| A-08 | `public/album/js/main.js:98-105,159-169` 才保护默认相册；service 无保护 | R-C7 定义 blank virtual album 并服务端拒绝 rename/delete |
| A-09 | 原 AC 写 ZIP，但 `AttachController.go:155-235` 和 ApiFile 生成 tar.gz | 删除虚构 ZIP 要求；R-C8 定义 tar entry/close/concurrency |
| A-10 | batch archive 共享/标题路径、loop defer、panic、关闭前返回 | Web/API 共用 request-scoped/stream writer，错误/清理显式 |
| A-11 | `FileController.go:269-302`、`NetUtil.go:18-72` 任意 URL、无界 `http.Get/ReadAll` | R-C9 固定 public-only、逐跳地址校验、timeout/size/decode/cleanup |
| A-12 | `app/application/notes/contracts.go:80-116` 有 Apply/Verify 声明，provider 责任未闭合 | R-C10 固定 identity/generation/manifest/Files 三态及无第二 receipt |
| A-13 | `NoteController.go:429-522`、`ApiNoteController.go:713-787` 重复 shell exec，secret 在 URL/argv，无 timeout/cleanup | R-C11 统一 renderer、direct argv、stdin/0600、timeout、artifact check |
| A-14 | content/admin 都称拥有 PDF executable execution | handoff：admin 只拥有配置入口/权限；content 拥有 renderer execution |
| A-15 | `html2image/ToImage.go:7-8` 固定成功，另一实现注释/固定失败，无 caller | 作为 reachability/dead-code，不恢复功能 |
| A-16 | 自动化、HTTP/browser、跨主机与 Linux PDF evidence 混合 | 新增 evidence matrix，未执行保持 planned/delegated-unrun |

## 2026-09-16 review repair

- R-01：`file/pdf.html` 使用 raw note content，PDF 输入不能称 trusted HTML；legacy `appKey` 也不能继续作为 callback credential。R-C11 现要求 self-contained serializer、用户主动内容移除、appKey bind-and-discard、local-file deny、零 outbound 和恶意资源/脚本负向证据。
- R-02：压缩字节上限不足以防 image decompression bomb。R-C5 现要求 `DecodeConfig`、checked width/height/pixel/allocation、GIF frame budget；上传与远程导入共用限制。
- R-03：独立图片删除只有 `fileId`，随机 quarantine handle 无法跨进程恢复。R-C4 现要求 durable quarantine manifest、legacy server-derived lookup/identity、状态矩阵、final Verify 后 terminal marker 和有界 GC。
- R-04：admin 规格仍把 PDF executable execution 写成自身职责。已将 admin 责任改为配置/descriptor provider，content 执行 renderer，并把冲突从“已收敛”改为正式 handoff 证据。

## 已确认不变量

1. 不改变公开 URL/Mongo Schema；HTTP method/status/Content-Type/wire mapper 归 interface。
2. notes 拥有 receipt、USN、history；content 不复制。
3. content 拥有 storage/path/publish/Verify/download/album/remote/PDF primitives。
4. `DownloadAll` 是 tar.gz 生成，不存在 ZIP 解压输入。
5. default album 是 blank ID 虚拟 filter；rename/delete 必须服务端拒绝。
6. PDF execution 归 content；admin 提供配置，delivery 提供 Linux/container evidence。
7. `html2image` 和注释 ApiFile actions 不重新发布。

## D-C1 — 远程图片网络范围（已确认）

采用 `public-only`：公网 `http/https`；每次 DNS/connection/redirect 后拒绝 loopback、RFC1918/private、link-local、multicast、unspecified、云元数据和特殊地址，同时限制 timeout/redirect/size 并实际解码。

确认的兼容影响：自托管部署不能从内网地址导入图片。

不实现管理员 host/CIDR allowlist、内网兼容开关或任意 URL fallback；只检查 URL 字符串/第一次 DNS、自动信任 redirect 也不满足契约。

## Activation decision

`ready-for-plan-approval`（task status 仍 `planning`）。下一门禁：输出本版最终规划摘要 → 用户后续明确批准开发 → `task.py start`。
