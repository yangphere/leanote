# Leanote presentation-frontend Codex Handoff

更新时间：2026-09-28；目录：`.trellis/tasks/09-08-presentation-frontend`；状态：`in_progress`；分支：`dev`；HEAD：`5090f340`。

## Goal and scope

继续当前呈现层任务，落实第一方存量保存及 batch 请求身份、编辑器状态和构建资源闭包。用户已明确确认三项决定：成功响应返回本次提交精确 Usn；新建 receipt 不纳入；unknown 仅页面内存保留冻结请求、离开提示、重载核对、不自动重放。无需重新询问。

## Changes

- `public/js/mutation-intents.js`：crypto 128-bit ID、冻结序列化 body、有序 IDs/action/目标/owner、pending/unknown/rejected 同 note 锁、显式原请求重试、迟到回调去重。
- `public/js/app/note.js`：即时保存和 savePool 统一；ExpectedUsn 来自确认缓存，后续使用响应精确 Usn；delete/move 严格 true，copy 完整 Item；失败和迟到响应保留当前编辑器/选择。
- 草稿显示由确认 cache + frozen save + queue 投影；重开恢复草稿，加载输入快照防止异步 cache 更新后显示旧内容；restoreDraft 使用原 editor session，不确认基线。发送失败保留 queue；copy/shared-copy 成功释放源队列，delete 成功清理已删除 IDs 的队列，move 权威读取通过后重绑定新 notebook 并发送队列。
- `app/controllers/httpserver_notes.go`：仅受保护存量成功响应附加顶层 Usn，直接取 result.USN；避免 info.Re 自定义 MarshalJSON 吞掉新增字段。旧客户端/失败/新建形状保持。
- `public/js/app/page.js`：离开仅提示，不临时发保存。模板重试提示、7 locale、manifest 和生成资源同步。
- Node 行为回归、Go mapper 测试、Playwright 连续保存 Usn/OperationId 断言；HTTP inventory 测试路径指向实际已归档文件。
- PRD/design/implement/acceptance 和 frontend/backend spec 已同步。保留会话开始已有改动，未提交、归档、journal 或 push。

## Validation

当前未提交工作树的具体结果以 `acceptance/evidence-matrix.md` 为准。

- npm ci 成功，Node v24.21.0 / npm 11.19.0；164 个 manifest 产物连续两次 SHA256 零漂移且全部跟踪。
- 最新 app.min.js SHA256、Node 回归计数与完整验证命令见验收矩阵的“review-fix evidence”；不再复制旧产物哈希作为当前结果。
- 原实现阶段 Go controllers/... + httpserver（timeout 60s）、go build ./...、相关 go vet 通过；本轮未改 Go，实现审核另运行已有 JSON Golden 核对读取响应。
- Playwright discovery 不等同实际浏览器执行；task validate、diff 和最新构建验证结果见验收矩阵，真实环境证据仍未齐。
- 子代理实现/审查两次均在启动阶段因 encrypted agent_message 传输能力失败，没有产出；主会话直接实现与复核。不要声称独立子代理检查通过。

## Remaining evidence and limitations

- AC-PF6 已通过。其他 AC 保留 partial/unrun/delegated-unrun；没有启动应用、Mongo 或浏览器。
- 真实 HTTP/DB receipt、跨进程、kill/restart/failpoint、HTML 语义及 Safari 等八槽由 presentation/delivery 继续执行；手工操作及预期见验收矩阵。
- 当前 npm ci + 构建可重复性不等同于已提交候选的干净检出/CI 零 diff。
- move 的 boolean 响应不提供 Usn，因此失效缓存 revision 并主动读取 `/note/getNoteAndContent`。读取失败可通过后续保存重试，即使没有新编辑也可恢复；并发服务端编辑、目标不符或无效 revision 不得自动覆盖，保留草稿并提示先备份后核对。明确 rejected 保留锁，不自动改 ID 重放。
- 新建沿用原路径，不承诺 OperationId/create receipt 保证。unknown 请求离开页面后不会持久恢复。

## Next steps

1. 阅读当前 diff、任务和验收矩阵；以代码/实际测试优先于本交接说明。
2. 在隔离真实环境执行尚未运行的验收并记录候选 SHA、环境、结果与清理；没有运行的项目保持未勾选。
3. 经单独授权再进入本地提交、归档、journal 流程；不要 push。交接文件保持工作区文档，不默认纳入业务提交。
