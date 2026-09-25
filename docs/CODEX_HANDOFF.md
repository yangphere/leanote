# Codex Handoff

更新时间：2026-09-25

## Goal

继续完成 Trellis ready 叶 `.trellis/tasks/09-08-application-publishing`：在不创建新任务的前提下，完成分享、博客、评论、群组、主题与预览的应用层实现、测试、规格同步和验收准备。

## Current state

- 当前任务：`application-publishing`，状态 `in_progress`，分支 `dev`。
- 任务已按轨道优先顺序激活；依赖为已归档的 `09-08-domain-contracts`。
- 规格审核已先于功能编码完成；用户已采纳全部推荐方案，四项实现合同和 Q-P10 公共查询/Host 合同已冻结。
- 工作区存在大量未提交改动，包含本任务已有改动和本轮修复；禁止 `reset`、`clean` 或覆盖无关改动。
- 尚未提交、归档、合并或 push；没有创建新 Trellis 任务。

## Completed work

- 完成 PRD、设计、实现说明、研究材料和验收矩阵的规格补充，纳入 notebook 发布传播、projection repair、预览 fail-closed、分享期限和完整 query/Host 合同。
- 修复评论 mutation 恢复时使用可变闭包 owner 的问题，恢复校验改用稳定的文章 owner。
- 修复博客 URL title 更新先清空再写入的问题，避免失败后留下空值。
- 将博客摘要、单页新增/修改/删除、`UserBlog.Singles` 投影和单页排序纳入持久 mutation、read-back 与补偿流程。
- 为分享公开入口增加 ObjectID 校验；修正 `HasSharedNote` 使用错误集合 guard 的问题。
- 完成主题路径、模板、图片、ZIP 和 metadata 校验增强。
- 将主题激活改为现有 workspace mutation 的事务/补偿路径，覆盖 `Theme.IsActive`、`UserBlog.ThemeId`、恢复快照和最终 read-back；操作身份绑定当前激活状态，避免切换主题后旧 committed receipt 阻止再次激活。
- 将主题删除改为 staged rename；metadata 删除或物理清理失败时尝试恢复 metadata 和目录，保留可恢复状态并返回失败。
- 为主题删除清理失败和 metadata 删除失败补充回归测试。
- 评论确认回放先核对 workspace committed receipt，回复通知只选择被回复者；outbox 共享已确认状态判断并脱敏未知 SMTP 结果。
- 评论通知的 SMTP 明确未接受结果现进入 retry/dead，结果不明仍留在 `handoff_unknown` 且不自动重发；DATA 明确拒绝、正文接受后 QUIT 失败、内存 dead 不重投与预交接租约有聚焦测试。真实 Mongo/SMTP 证据未运行。
- Host 规范化由服务查询与 DB 预检共用；预检拒绝历史非规范、错误类型和重复映射，可信代理下仍验证原始 Host，重复子域名不随机选 owner。
- 已运行 `gofmt`，并完成局部静态复核。

## Pending work

- 两项 `task.py validate` 已通过；admin 仍有 publishing PRD/design 超长导致的上下文注入截断警告。最终四层 diff-review 和合并结论仍待完成。
- 修复 standalone 评论提交的中间态可见窗口及可能的孤儿通知；补齐 `handoff_unknown` 人工对账、receipt 保留期压缩、永久墓碑和容量监控。确认评论的长期回放目前仍依赖会在 30 天后 TTL 删除的 workspace operation，需改为永久凭据可独立核对。
- 补足主题激活、主题删除、分享非法 ID 和分享投影中断的真实 Mongo 回归证据。
- 补足真实 HTTP、浏览器、跨 owner/Host、SMTP/outbox、ZIP 故障注入及跨进程并发证据。
- 根据最终审查结果同步任务验收材料；未通过的真实环境证据保持 `unrun`、`partial` 或 `unknown`，不能改写为 passed。
- 用户未授权提交、归档、合并或 push，后续仍需等待单独指令。

## Important decisions

- 不创建新任务；持续使用 `.trellis/tasks/09-08-application-publishing`。
- 规格审核阶段已结束，当前允许实现；规格变更仍需同步 PRD、design、implement、research 和 acceptance 材料。
- 分享期限使用显式 RFC3339 绝对时间，服务端按 UTC 秒存储和比较；缺字段保持旧语义，显式清除必须使用 `clearExpiresAt=true`。
- 公开博客统一要求有效 owner、`IsBlog=true`、`IsTrash=false`、`IsDeleted=false`；失败、未知提交和 projection 未确认不得返回伪成功。
- 评论正文按纯文本保存和输出边界转义；`submissionId` 是必填的 32 位小写十六进制跨请求幂等身份。
- 主题上传不自动公开；主题安装必须重新校验公开来源并复制为安装者自己的副本；预览失败必须不写 session、不改 active/public 状态、不回退到其他主题。
- 真实 Mongo、HTTP、浏览器、SMTP/outbox、ZIP 和跨进程证据不能用单元测试替代。

## Changed files

当前工作区全部变更如下（包含本轮之前已有的未提交改动）：

- 规格：`.trellis/tasks/09-08-application-publishing/acceptance/evidence-matrix.md`、`.trellis/tasks/09-08-application-publishing/design.md`、`.trellis/tasks/09-08-application-publishing/implement.md`、`.trellis/tasks/09-08-application-publishing/prd.md`、`.trellis/tasks/09-08-application-publishing/research/action-contract-inventory.md`、`.trellis/tasks/09-08-application-publishing/research/implementation-decision-brief.md`、`.trellis/tasks/09-08-application-publishing/research/spec-audit-2026-09-23.md`
- Controller：`app/controllers/BlogController.go`、`app/controllers/NoteController.go`、`app/controllers/PreviewController.go`、`app/controllers/ShareController.go`、`app/controllers/member/MemberBlogController.go`
- DB：`app/db/Mgo.go`、`app/db/mongo_collection.go`、`app/db/outbox.go`、`app/db/persistence_contract_test.go`、`app/db/persistence_indexes.go`、`app/db/persistence_mongo_test.go`、`app/db/workspace_mutation.go`、`app/db/workspace_operation_store.go`、`app/db/share_schema.go`、`app/db/share_schema_test.go`
- Domain：`app/domain/blog_host.go`，供服务查询和数据库预检共用 Host 规范化规则。
- Model：`app/info/BlogInfo.go`、`app/info/NoteInfo.go`、`app/info/ShareNotebookNoteInfo.go`
- 文件与归档：`app/lea/File.go`、`app/lea/file_test.go`、`app/lea/archive/zip.go`、`app/lea/archive/zip_test.go`
- Service：`app/service/BlogService.go`、`app/service/EmailService.go`、`app/service/NoteService.go`、`app/service/NotebookService.go`、`app/service/ShareService.go`、`app/service/ThemeService.go`、`app/service/UserService.go`、`app/service/attachment_access.go`、`app/service/content_runtime.go`、`app/service/contentpdf/export_adapter.go`、`app/service/contentpdf/export_adapter_test.go`、`app/service/image_actions.go`、`app/service/outbox_worker_test.go`
- 新增 service 代码与测试：`app/service/blog_comment_test.go`、`app/service/blog_publishing.go`、`app/service/blog_publishing_test.go`、`app/service/blog_query.go`、`app/service/blog_query_test.go`、`app/service/blog_read_test.go`、`app/service/share_permission.go`、`app/service/share_permission_test.go`、`app/service/theme_security_test.go`
- Controller 测试：`app/controllers/NoteController_test.go`、`app/controllers/member/member_blog_controller_test.go`、`app/controllers/preview_controller_test.go`
- 前端：`public/blog/js/common.js`、`public/blog/js/share_comment.js`
- 本交接文件：`docs/CODEX_HANDOFF.md`

## Test results

- `go test ./app/db ./app/service -count=1 -timeout 60s`：本轮通过；新增的内存 outbox/本地脚本化 SMTP 用例覆盖明确拒绝 retry、dead、未知交接、预交接租约与 DATA/QUIT 分流。
- 主题、分享、评论 mutation 针对性测试：通过。
- `go vet ./...`：通过。
- `go build ./...`：通过。
- `npm test`：通过，131 passed、1 skipped、0 failed。
- `gofmt`：已执行。
- `git diff --check`：通过；Git 输出的 LF/CRLF 提示是换行转换警告，不是 diff 错误。
- `go test ./app/domain ./app/service ./app/db ./app/controllers ./app/controllers/member ./app/lea/... -count=1 -timeout 60s`：通过；其中需要 Mongo 的用例会跳过。
- `TestCommentConfirmedReplayAfterUnpublishReturnsOriginalResult`、`TestCommentNotificationTargetsReplyAuthorOnly`、`TestSharePreflightRejectsDocumentsOutsideInstalledValidator`、`TestCommentOutboxMongoUsesHandoffGate`、`TestCustomDomainPreflightRejectsCanonicalCollisions`、`TestLookupUserBlogBySubDomainRejectsAmbiguousOwner` 经 `-v` 确认均跳过，原因是 `127.0.0.1:27017` 拒绝连接；Docker Desktop Linux daemon 也不可连接。
- Publishing/admin `task.py validate` 均通过；admin 的 publishing PRD/design 上下文注入仍有截断警告。
- 未运行 `go test ./...` 和需要初始化 MongoDB 的 `go test ./app/tests/...`；未运行真实 HTTP、浏览器、SMTP/outbox、ZIP 故障注入和跨进程并发验证。

## Known issues

- 任务仍是 `in_progress`；自动化及 Trellis 校验通过不等于完整验收或可合并。
- 当前代码的 standalone 评论写入在 receipt/通知确认前仍可能暴露评论/计数中间态；通知 worker 的结果分流已有局部验证，但缺少人工对账及真实 Mongo/SMTP 验证；receipt GC/容量合同和尚未摆脱 30 天 operation TTL 的长期回放依赖仍未实现。以上均为合并阻断。
- 真实 Mongo、HTTP、浏览器、SMTP/outbox、ZIP 和跨进程证据仍为 `unrun`/`partial`/`unknown`；当前 Go 单测、vet 和 build 不证明这些运行时契约。
- 工作区包含大范围未提交改动，无法仅凭当前状态安全区分本轮与更早改动；后续不得使用破坏性 Git 命令清理。
- 主题激活和主题删除的新增回归测试主要覆盖纯逻辑/文件补偿 helper；数据库事务、standalone compensation 并发和文件系统故障注入仍需真实 Mongo/故障环境验证。
- `jbcontext` 当前无法使用，因为仓库没有建立索引；本次已改用定向文件读取和 `rg`/Git 状态作为证据。

## Next steps

1. 先为评论列表、公开计数和通知 worker 建立共同的持久发布可见性判定，关闭 standalone 中间态与孤儿通知；再补人工对账和永久 receipt 的跨 30 天回放/GC/容量合同，并运行真实 Mongo 测试。
2. 执行真实 HTTP/浏览器/SMTP/outbox/ZIP 验收，并把未运行项保持为明确的 `unrun` 或 `delegated-unrun`。
3. 做最终实现、测试、规格和任务元数据四层 diff-review；阻断项关闭后再给合并结论。
4. 只有在用户明确授权后，才进行本地 commit、Trellis archive/journal；不要自行 push。
