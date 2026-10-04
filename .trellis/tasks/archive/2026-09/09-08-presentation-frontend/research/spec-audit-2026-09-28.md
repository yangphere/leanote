# 2026-09-28 规格审核：presentation-frontend

> 历史审核快照：下文行号、未决问题和“只审核、不编码”范围描述的是本轮早期状态。后续用户已确认 Q-PF-01/Q-PF-02：存量 Web save 返回精确 committed Usn，新建 receipt 不扩展，unknown 只在同页内存重试、离开提示、重载核对且不自动重放。实现及本地回归已完成，当前状态以 PRD/design/implement 和 acceptance/evidence-matrix.md 为准；真实环境证据仍未运行。

## 1. Selection and scope

按 `.trellis/tasks/09-08-business-layer-architecture/design.md:45-53` 的迁移轨道，`interface-http` 归档后下一 ready 叶是 `09-08-presentation-frontend`，P1、无 children。该叶 `task.json.meta.depends_on` 指向 `09-08-domain-contracts` 和 `09-08-application-notes`；两者均位于 `archive/2026-09/` 且 `task.json.status=completed`。本轮已通过 `task.py start` 激活，状态 `in_progress`；未创建新任务。当前审核只修改任务规格、研究、验收和上下文清单，不运行功能编码。

## 2. Evidence consulted

| Topic | Current evidence | Consequence |
| --- | --- | --- |
| 领域/HTML | `CONTEXT.md`、`docs/adr/0003-modernize-frontend-with-generated-asset-contract.md` | 服务端渲染、未编辑零写入、编辑后语义等价、历史 URL 和浏览器矩阵为既有不变量 |
| Build | `package.json`、`package-lock.json`、`scripts/build/manifest.mjs:24-155`、`scripts/build/index.mjs:95-154` | Node 24、jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2、esbuild 0.28.2；read-only manifest import 当前为 11 JS + 7 CSS + 124 assets + 21 i18n + 1 HTML = 164 输出，7 locale，4 第一方插件 |
| Build failure/security | `scripts/build/manifest.mjs:166-270`、`tests/js/build-pipeline.test.js`、`.trellis/spec/frontend/quality-guidelines.md` | manifest/path/i18n fail-closed、staging/backup/rollback 和脱敏 artifact 已有测试约定；尚未在本轮执行构建 |
| Browser/runtime | `playwright.config.mjs:15-32`、`tests/e2e/business/editor-flows.spec.mjs:69-136`、`docs/modernization/browser-smoke/jquery-3.7.md` | 本地 Chromium 与真实多浏览器/发布前八槽不可互相代替；历史 smoke 不证明当前候选提交 |
| Notes handoff | 归档 `09-08-application-notes/{prd,design,implement}.md`、`acceptance/evidence-matrix.md`，`app/application/notes/identity.go:50-111` | KD-N6 已确认可选字段、旧客户端 `RetrySafe=false`；第一方生成/复用属于 presentation，服务端 receipt 属于 notes |
| Delivery handoff | `.trellis/tasks/09-08-delivery-verification/prd.md:13-40` | 真实 HTTP + DB receipt、跨拓扑/进程/浏览器矩阵由 delivery 汇合，不能用局部 Node 测试关闭 |

本轮运行了只读 `node --version`（v24.21.0）、`npm --version`（11.19.0）及 manifest 导入计数；规划材料通过 `task.py validate 09-08-presentation-frontend`（implement 8 条、check 7 条）和 `git diff --check`。未运行 `npm ci`、构建、Node 测试、真实 HTTP、Mongo 或浏览器。已有历史 smoke 与归档任务的通过统计不能冒充本任务当前执行。

## 3. Current code facts and specification gaps

| ID | Current fact / gap | Audit disposition |
| --- | --- | --- |
| F-PF-01 | 原 PRD 只说“版本唯一”和“构建零漂移”，未写 manifest 输入/输出安全、i18n 负向、发布回滚边界 | R-PF1/AC-PF1 固定 fail-closed、Git 跟踪、重复构建及恢复材料；164 是观察值，不是第二清单 |
| F-PF-02 | 历史 jQuery URL `/js/jquery-1.9.0.min.js` 实际由 npm 3.7.1 产生；`leaui_image` iframe 是独立浏览上下文 | R-PF2 将“单一运行时”明确为每个上下文不混版，不误删兼容 URL 或 iframe 自有同版本资源 |
| F-PF-03 | `public/js/app/note.js:399-595` 有即时保存和旧 savePool 两个发送入口；cache 使用提交对象更新，未从成功响应取得新 `Usn` | R-PF3、design §3 要求两入口同一协议、只用权威 revision；Q-PF-01 阻断具体实现 |
| F-PF-04 | `app/controllers/httpserver_notes.go:119-124,224-233` 解析可选 `ExpectedUsn` 并交给 `SaveNote`，但成功 `info.Re` 只设置 `Ok`；`app/service/note_workspace.go` 产出 `result.USN` | Q-PF-01：如何把精确已提交 USN 交给浏览器，不能据上一值加一或把 editor revision 当 USN |
| F-PF-05 | `app/controllers/httpserver_notes.go:155-181` 的 `IsNew` 分支返回 `Re.Item=created`，未消费 `OperationId`；创建已有客户端稳定 `NoteId` | Q-PF-01 同时确认新建是否需要客户端 operation generation；不能把存量 update 的保证套给 create |
| F-PF-06 | `public/js/app/note.js:485,573,1440,1705,1792-1800` 未向目标 action 发送 `OperationId`/`ExpectedUsn`；`tests/js/note-save-contract.test.js` 主要是源码正则断言 | R-PF4/AC-PF5 要求实际请求字段、意图生命周期和服务端 receipt 证据；新增行为应有可执行状态/交互回归，不只镜像源码 |
| F-PF-07 | `app/service/workspace_batch.go:17-44,68-91` 用 owner/action/有序输入绑定 outer receipt，子操作按 noteId/index 派生；前端目前按当前选择组装请求 | R-PF4/设计 §4 冻结顺序/目标/payload，同一 unknown-result 重试不能重新取当前选择 |
| F-PF-08 | `public/js/app/note.js:1405-1458` 先清空当前 note、隐藏列表、reset batch，且没有传输失败回调；move/copy 也缺 unknown-result 恢复流程 | R-PF5 要求失败不造成永久 UI 假成功，需可见错误/重试或核对；具体跨 reload 范围留 Q-PF-02 |
| F-PF-09 | 原验收把本叶真实浏览器 smoke 与 delivery 八槽矩阵混写 | AC-PF3/AC-PF7 和 evidence matrix 分开本地目标与委托证据，Safari 只认真实环境 |

## 4. Open product/contract decisions

### Q-PF-01 — Authoritative committed USN and create scope (blocks save implementation)

现有 Web save `Re.Ok=true` 不提供精确提交的 `Usn`。可评审的路径包括：服务端在成功响应中增加已提交 `Usn`，由 notes result 和 HTTP mapper 共同保证；或使用现有 `GetNoteAndContent` 的权威读取并规定并发更新、响应丢失及读回内容不匹配时如何处理。两种路径的线协议、额外请求、竞争窗口和验收不同。另需明确 `IsNew` 是否消费客户端 `OperationId`；仅稳定 `NoteId` 目前不能被表述成与存量 update 完全相同的 generation contract。没有决定，不应实现发送 `ExpectedUsn` 后续连续保存或声称 new-note unknown-result 安全。

### Q-PF-02 — Unknown-result after reload (blocks recovery acceptance)

需明确只在当前页面内存生命周期保留冻结请求，还是要求跨 reload 恢复。后者必须定义如何安全保存/重建相同请求、区分用户/权限变化、过期与撤销、避免长期保存正文/认证材料；前者必须明确 reload 后进入人工核对且不得用新 ID 自动重放。现有文档只要求“页面重载边界”有证据，未给产品结果。

## 5. Audit conclusion and next gate

本轮可把构建、运行时、编辑器、batch、异常和证据归属写成可执行规格；Q-PF-01/Q-PF-02 涉及协议与产品恢复语义，不能从当前源码可靠推出。收到决定后需同步 PRD、design、implement、验收矩阵，并在功能编码前复核 notes/interface 依赖。当前 `in_progress` 仅表示任务已激活，不代表规格决策闭合或获准跳过本轮审核边界。
