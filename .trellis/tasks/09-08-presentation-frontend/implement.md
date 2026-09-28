# 呈现层：前端构建与编辑器运行时 — 执行计划

## Phase 0：规格审核与激活（本轮）

- [x] 按父任务迁移轨道与 `task.json.meta.depends_on` 选中 ready 叶 `09-08-presentation-frontend`；domain-contracts 和 application-notes 已归档 completed，未创建新任务。
- [x] 运行 `task.py start` 激活该叶；当前 `in_progress` 只表示激活，不表示需求已确认或可编码。
- [x] 核对 ADR、`CONTEXT.md`、前端 spec、manifest/build/tests、模板/编辑器、notes 服务端协议、delivery 交接；审计结果见 `research/spec-audit-2026-09-28.md`。
- [x] 将目标、范围、数据流、输入/输出、失败、兼容、权限边界和验收方法收敛到 PRD/design/acceptance；规格审核阶段没有修改业务代码、生成资源或测试。
- [x] Q-PF-01：用户确认受保护存量 Web save 返回本次提交的精确 `Usn`；新建 receipt 不在本轮范围，已同步 PRD/design/验收和 mapper 测试。
- [x] Q-PF-02：用户确认仅页面内存重试、离开提示、重载后核对且不自动重放；已同步 PRD/design/验收与行为测试。
- [x] 运行 `task.py validate 09-08-presentation-frontend`、`git diff --check` 和改动路径复核；确认只有本任务规划材料/激活元数据及既有用户改动。

**编码门禁**：2026-09-28 续接中用户已确认 Q-PF-01/Q-PF-02（见 PRD）。可继续存量 save 与 batch 的内存意图协议；新建 receipt、持久恢复仍不在范围内。

## Phase 1：构建与运行时

- [x] 从 manifest 动态核对 164 个输出、Git 跟踪、7 locale 和四插件；新增 mutation 模块作为 app 输入，未新增第二套输出清单。
- [x] 当前工作树完成 Node 24 `npm ci`、标准构建和 Node 回归；最终两次构建全部输出 `drift=[]`、`untracked=[]`。这不等同于已提交候选干净检出/CI 零 diff。
- [ ] 干净候选/CI 重建，以及逐浏览上下文的真实库资源、插件、iframe、modal/tab/dropdown 验收。

## Phase 2：笔记前端协议

- [x] 用户三项决定落盘：存量成功返回精确 committed Usn；新建 receipt 不在范围；unknown 仅页面内存，离开提示，重载核对、不自动重放。
- [x] 即时保存和 savePool 统一经 submitSave；crypto 生成 128-bit OperationId，ExpectedUsn 取确认缓存，后续提交取响应精确 Usn。
- [x] pending/unknown/rejected 锁定同 note；未知结果重试冻结 body/ID；只应用一次；batch 冻结有序 IDs、action、目标和共享 owner。
- [x] delete/move 只认原始 true；copy 只认完整 Item。失败保留可见状态；迟到 delete/move 响应不清理新编辑器或新选择。move 后自动读取权威 Usn，核对未发生远端编辑后恢复保存，不重置当前编辑器。
- [x] 保存期间继续编辑及改回原值进入现有 queue；重开笔记恢复草稿，不把草稿写成已确认 cache；发送失败保留队列。
- [x] 修复异步加载期间 cache 被成功回调更新后仍显示旧内容的竞态，新增先失败后通过的行为测试。序列化失败不显示保存成功；copy/shared-copy 成功释放源笔记队列，delete 成功删除对应队列，move 经权威读取后重绑定目标再发送，不自动写回旧 notebook。
- [x] Web mapper 直接返回 SaveNote result.USN；绕开 info.Re 的 MarshalJSON 嵌入陷阱，保持无 OperationId/失败/新建旧响应形状。修复 HTTP inventory 测试指向已归档任务的路径。
- [x] 页面状态、重试按钮、7 locale、模板、manifest 和生成资源同步；Playwright 编辑器用例补充 OperationId/ExpectedUsn/连续精确 Usn 断言。
- [ ] 真实 HTTP/browser/Mongo receipt 故障回放、旧客户端 Golden；新增浏览器断言尚未实际执行。

## Phase 3：验证与交付

- [x] Go controllers/httpserver 测试（每包 timeout 60s）、go build ./...、相关 go vet 通过。
- [x] 规格同步：frontend/state-management 的请求意图、草稿恢复、异步加载、失败规则，以及 backend/error-handling 的精确 Usn 响应规则。
- [x] 已执行 Node 全量及针对性测试、构建、Playwright discovery、task validate 和 diff 检查；最新命令、计数与产物哈希统一记录在 acceptance/evidence-matrix.md，历史结果不代替本轮验证。
- [ ] 干净候选/CI、真实 HTTP/DB/browser、Mongo 7/8、跨进程/failpoint 和 Safari 八槽由 presentation/delivery 继续验收；不得用 Node/build 替代。
- 本地收尾按工作提交 → 任务归档 → journal 记录执行；用户已于 2026-09-28 明确授权，实际提交哈希以 Git 与 journal 为准，不 push。归档仅关闭实现工作，未执行验收继续由 delivery-verification 承接。

## Review execution

本轮 trellis-implement/trellis-check 子代理因传输错误无法启动（encrypted agent_message unsupported），没有写文件。主会话已直接实施并复核实现、测试、规格和任务元数据；不将此记为独立子代理审查通过。

2026-09-28 审核修复续接：两类子代理仍在启动阶段遇到同一传输错误。主会话直接修复四项审核问题，并增加 move/copy/delete 完成后的队列出口、无新编辑时读取重试、失败/迟到读取及切换编辑器的行为回归。测试通过真实生产函数和读取回调恢复 Usn，不手动补缓存。旧签名和 R-PF5 待定文案已修正；真实 HTTP/DB/browser 等未运行项继续保持未勾选。
