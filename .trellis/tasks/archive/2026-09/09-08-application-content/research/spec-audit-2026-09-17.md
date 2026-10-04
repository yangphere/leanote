# `09-08-application-content` 规格复核（2026-09-17）

## Conclusion

选定 leaf 仍是 `09-08-application-content`：P1、`children=[]`，依赖 `09-08-domain-contracts` 与 `09-08-application-notes` 均已归档 completed。它已是 `in_progress`，因此本轮没有重复激活，也没有运行实现代理或修改业务代码。

前一轮规格已覆盖主要业务边界，但已与 `158a6dee` 的部分实现发生状态漂移。本次把材料改为“foundation 已实现、action integration 未完成”，并新增 root production binding、deterministic archive metadata、PDF aggregate resource budget 与量化安全预算决策。用户于 2026-09-17 采用推荐安全预算，Q-C2 已收敛为 D-C2；本轮不再存在 blocking product decision。

## Evidence inspected

- 当前 task：`task.json`、PRD、design、implement、JSONL、action inventory、evidence matrix。
- 直接依赖：archived domain-contracts/application-notes task metadata、notes `ContentAssetPort`/`AssetMutation` contract。
- 下游：application-publishing、application-admin、interface-http、presentation-frontend、delivery-verification 的 PRD/task metadata 与交接矩阵。
- 当前实现：`app/application/content`、`app/service/contentfs`、`app/service/contentpdf`、`app/service/content_runtime.go`、相关 controller/service 与测试。
- 历史提交：`158a6dee7319d0dd3e09007fd623190525f28977`；审核开始时 `dev` 与 `origin/dev` 为 `0/0`，worktree clean。
- `jbcontext search` 返回 HTTP 404；按项目约定改用目标化 `rg` 和直接读取。

## Findings and repairs

| ID | Finding | Resolution |
| --- | --- | --- |
| A-17 | PRD/implement/research 仍写 task=planning、等待 `task.py start`，但 task 已 in_progress 且已有实现提交 | 改为 current-state/continuation gate；不重复激活 |
| A-18 | 执行计划全部未勾选，无法区分 foundation 与剩余 action | 按 roots/store、manifest、media、PDF 的实际落地拆分完成/未完成条目 |
| A-19 | evidence matrix 全部 planned，忽略当前 focused tests，也可能误把 primitive 当 action 完成 | 引入 `partial`；只把 dependency scan 标 `passed`，其余按缺口保留 partial/planned/delegated-unrun |
| A-20 | content 当前从 `basePath` 派生 roots，而 interface PRD 声明 production-config/typed roots 为唯一来源 | 明确当前 initializer 是迁移接线；完成前提供 typed production initializer，禁止第二配置来源 |
| A-21 | archive 只约束名称/close，未禁止泄露宿主 mode/mtime/uid/gid/扩展属性 | 增加 regular-only、normalized deterministic header 要求与 AC-C8 evidence |
| A-22 | PDF 只有单项 image/output 限制，多个资源可在最终 64 MiB 拒绝前造成聚合内存放大 | 固定 32 MiB input、64 MiB final/artifact；要求加载前累计 raw/base64 projected bytes |
| A-23 | PDF focused test 只验证 magic+EOF，但原规格写“基本可读性” | 明确当前只属 partial；真实 parser/readability 仍未完成 |
| A-24 | repo 对 visible text 长度、remote timeout/redirect/body 没有可继承基线 | 已解决：用户采用推荐安全预算，Q-C2 收敛为 D-C2，量化值写入 PRD/design/acceptance |
| A-25 | Go module 是 1.26，当前 focused evidence 运行于 Go 1.27.1 | 证据记录 runner drift；不把它记为 Go 1.26/full gate |

## Current implementation boundary

- 已实现并 focused-tested：pure dependency boundary、portable path、root validation、no-clobber publish/four-state Verify、manifest no-replace/CAS/GC primitives、image decode budget、self-contained PDF/direct process、Web/API ExportPdf adapter。
- 部分实现：path host indirection matrix、upload config/multipart、delete lifecycle、PDF resource/readability、controller mapping。
- 后续实现已完成：public-only remote fetch、deterministic unified tar.gz、Web image/album/attachment read/archive migration、notes provider 与本机 durable recovery primitives。Web attachment upload 的 identity/path 也已下沉 service。
- 仍未完成：`interface-http` production-config 到 typed `ContentRoots` 的唯一来源绑定，以及真实 Mongo/restart/Linux/cross-host/browser/wkhtmltopdf evidence。ApiNote stable multipart 已迁至 service；controller 保留 presence/open 与 wire mapping。
- delegated-unrun：Go 1.26、Mongo-backed content integration、real HTTP/browser、Linux wkhtmltopdf、zero outbound/local-file proof、cross-host/persistent-volume/kill-restart failpoints。

## D-C2 confirmed safety budget

用户于 2026-09-17 确认：visible text 清理后最多 `255 UTF-8 bytes`、超限显式 `validation` 且不截断；既有超限 metadata 保持可读/可下载，rename/copy mutation 遇到待写入超限值时拒绝并保留原值。remote hard profile 为 DNS+dial `5s`、TLS handshake `5s`、response header `10s`、overall `30s`、最多 `5` 次 redirect、compressed body `min(uploadImageSize, 32 MiB)`；missing/invalid/zero/negative 配置 fail closed。

该 profile 是 Web/API/PDF remote-resource 的单一事实来源，adapter 不得放宽。规格已收敛；在提交最新最终规划摘要并获得下一条消息中的明确开发批准前，仍不得继续功能编码。

## 2026-09-17 remaining implementation audit

- action inventory 经 controller method、registry、route 与 provider call scan 对账为 26 callable（22 Web + 4 API）、2 integrations、4 providers、4 historical dormant surfaces。
- `service.InitContentRuntime` 已从 `basePath string` 改为显式 typed `ContentRoots`；root validation 的 canonical temporary path 同时提供给 content store 与 PDF backend，避免 PDF 对原配置别名二次解析后漂移。生产配置键、稳定错误码与 bind 时序仍由 `09-08-interface-http` 决定。
- legacy Revel 与当前 `cmd/leanote` 仍显式从 application base 构造 compatibility roots；这不是 production-config closure，必须在 interface task 绑定后删除，不能与未来配置源并存。
- Web attachment upload 不再在 controller 构造 ObjectID、storage name 或 logical path；malformed note/actor identity 在 service dependency 前失败。archive adapter 移除了匿名 actor 的重复拒绝，public note 继续由 application authorization 决定。
- `app/lea/html2image` 与未路由的 ApiFile 注释 action block 已在 caller/import/route scan 后删除。ApiBase 不再拥有 stable upload/cleanup 的 filesystem/Mongo helper；ApiNote Add/Update 通过 service lifecycle 迁移，仍需 interface-http wire 与真实运行证据。

## Verification snapshot

- `python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-content`：修改后通过（15 implement / 13 check entries）。
- `go test -timeout 60s ./app/application/content ./app/service/contentfs ./app/service/contentpdf`：通过。
- `go test -timeout 60s ./app/service -run 'Test(GetUploadLimitBytes|InitializeContentStore)' -count=1`：通过。
- `go test -timeout 60s ./app/controllers ./app/controllers/api -run 'Test(NoteExportPDF|APIExportPDF|FirstPartyNoteToPDF|RegisterHTTPExposesNoteToPDF|FileUploadImage)' -count=1`：通过。
- `go list -deps ./app/application/content` forbidden dependency scan：未命中 Revel、Mongo driver、`app/db`。
- 环境：Windows 10.0.26200 amd64，Go 1.27.1；未运行 Mongo、Go 1.26、Linux/container、真实 HTTP/browser 或真实 wkhtmltopdf。
