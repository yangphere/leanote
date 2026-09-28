# Leanote presentation-frontend Codex Handoff

更新时间：2026-09-28；归档目录：`.trellis/tasks/archive/2026-09/09-08-presentation-frontend`；实现状态：`completed`（真实验收未关闭）；分支：`dev`；工作提交：`08bdfadb7d40c67d61f9c410846f3f2ccfb924d2`。

## Goal and scope

呈现层实现及四项审核修复已按用户“提交并归档”授权完成本地工作提交和任务归档；下一阶段是 delivery-verification 承接未运行实证。已确认三项决定：成功响应返回本次提交精确 Usn；新建 receipt 不纳入；unknown 仅页面内存保留冻结请求、离开提示、重载核对、不自动重放。无需重新询问。

## Changes

- `public/js/mutation-intents.js`：crypto 128-bit ID、冻结序列化 body、有序 IDs/action/目标/owner、pending/unknown/rejected 同 note 锁、显式原请求重试、迟到回调去重。
- `public/js/app/note.js`：即时保存和 savePool 统一；ExpectedUsn 来自确认缓存，后续使用响应精确 Usn；delete/move 严格 true，copy 完整 Item；失败和迟到响应保留当前编辑器/选择。
- 草稿显示由确认 cache + frozen save + queue 投影；重开恢复草稿，加载输入快照防止异步 cache 更新后显示旧内容；restoreDraft 使用原 editor session，不确认基线。发送失败保留 queue；copy/shared-copy 成功释放源队列，delete 成功清理已删除 IDs 的队列，move 权威读取通过后重绑定新 notebook 并发送队列。
- `app/controllers/httpserver_notes.go`：仅受保护存量成功响应附加顶层 Usn，直接取 result.USN；避免 info.Re 自定义 MarshalJSON 吞掉新增字段。旧客户端/失败/新建形状保持。
- `public/js/app/page.js`：离开仅提示，不临时发保存。模板重试提示、7 locale、manifest 和生成资源同步。
- Node 行为回归、Go mapper 测试、Playwright 连续保存 Usn/OperationId 断言；HTTP inventory 测试路径指向实际已归档文件。
- PRD/design/implement/acceptance 和 frontend/backend spec 已同步。工作实现已提交并归档，全部未完成实证已登记到 delivery-verification PRD；journal 按工作提交哈希记录，不执行 push。

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

1. 阅读归档任务和验收矩阵；以工作提交代码及实际测试优先于本交接说明，completed 不等同于发布门禁通过。
2. 由 delivery-verification 在隔离真实环境执行尚未运行的验收，记录候选 SHA、环境、结果与清理；没有运行的项目保持未勾选。
3. 当前没有自动激活交付任务，也没有推送；工作、归档及 journal 提交哈希见 Git 与开发日志。
