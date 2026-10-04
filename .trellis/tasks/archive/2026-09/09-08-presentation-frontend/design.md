# 呈现层：前端构建与编辑器运行时 — 技术设计

## 1. Boundary and data flow

`package-lock.json`、可编辑 JS/CSS、`messages/`、`app/views/note/note-dev.html` 和第一方插件源码 → `scripts/build/manifest.mjs` → staging/build/validation → 原子发布到受跟踪产物 → `app/httpserver` 静态/模板适配 → 浏览器页面。manifest 只声明生成闭包，HTTP route、服务端权限与 USN 不在构建脚本中复制。

浏览器侧 `Note` 负责页面缓存和用户操作，`LeanoteEditorSession` 负责正文 revision/dirty/load epoch，shared AJAX wrapper 负责传输及失败通知。新增的意图状态只保存一次已冻结的请求和结果确定性，不替代正文状态或服务端 receipt。

## 2. Build design and rollback

- 以当前 164 个 manifest 输出为核对基线；每项检查输入存在、输出唯一、URL/模板引用可达及 Git 跟踪。变更输出时同步调整 manifest/contract，数量按 manifest 动态推导。
- `runBuild` 的验证、staging、backup、publish、rollback 仍是单一链路；路径穿越、symlink/junction、缺失 i18n 根、动态/缺失 key、生成闭包不等、回滚失败都显式报错。失败保留原输出或可恢复 backup。
- jQuery 保留历史 URL，但内容来自锁定的 npm 包；独立 iframe 可加载同版本实例。BootstrapDialog、`leaui_image` 和 TinyMCE plugin 输出从源码构建，不维护第二份手写压缩文件。生产资源不含 migrate/CDN。

## 3. Existing-note save intent

2026-09-28 续接决定：Q-PF-01/Q-PF-02 已由用户确认。受保护存量 save 响应增加顶层 `Usn`，取本次 `SaveNote` result（包括 receipt 重放结果）；缺失、非整数或不可信 revision 不能确认新 mutation。旧客户端无 OperationId 时保持原响应，新建不新增 receipt。请求意图仅在页面内存存活，离开前提示，重载后核对且不自动重放。

冻结一次提交时记录 `noteId`、`loadEpoch`、editor capture/revision、实际存在的 metadata/content 字段、服务端确认的 `ExpectedUsn`、新 `OperationId` 和逐字可重放的请求值。`ExpectedUsn` 是提交时的笔记持久化 revision，不是 editor revision；同一个请求重试不能重新序列化当前编辑器或刷新为新的 `ExpectedUsn`。

`mutation-intents.js` 在准备完成后进入 `pending → confirmed | rejected | unknown`。`confirmed` 只在该 action 成功且取得所要求的权威 revision 后推进 cache/dirty；`rejected` 保留用户修改并呈现冲突/失败；传输中断、不可解析或未收到确定结果进入 `unknown`，仅同一冻结请求可重试或核对。新一轮输入修改不能占用旧 ID。多个同 note 请求不得并发越过未知结果。

已存在的 `captureEditorSaveContext`、`isCurrentEditorSaveCapture` 与 `LeanoteEditorSession.confirmSave` 继续约束旧响应；修复应扩展这条路径及已存在的 `savePool` 发送入口，不让二者产生不同协议。旧 load epoch 的成功可以更新对应 note 的已确认 cache，但不能触碰当前 editor session 或覆盖更新 revision。若服务端返回冲突，显示并要求重新加载/解决，不能用新 ID 盲重放旧编辑。

重新打开未决笔记时，`pendingNoteView` 将已确认缓存、冻结 save 和最新 savePool 草稿组合用于显示，不将草稿写入缓存。编辑器先载入已确认基线，再以 `restoreDraft(content, loadEpoch)` 恢复草稿。异步加载使用输入快照，完成时重新读取最新缓存投影，防止中途收到保存成功后仍显示并回写旧内容。未发送的 queue 保留；非 save mutation 期间捕获的编辑在确认结果后按 action 退出：copy/shared-copy 放行源笔记队列；delete 清理已删除 IDs 的队列；move 经权威读取核对后重绑定新 notebook 并恢复保存。

move 的 boolean 成功不含 revision，因此立即调用现有 `/note/getNoteAndContent`，按请求绑定的 noteId 消费扁平响应（重复嵌入字段 NoteId 不在响应中）。保留当前编辑器和未捕获草稿，不再通过 `changeNote` 重置当前编辑器。读取期间禁止新 mutation；核对目标 notebook、合法且不倒退的 Usn、非删除/回收站和原缓存中已有可编辑字段，防止拿远端新 Usn 覆盖并发编辑。成功只补充确认的 Usn/NotebookId，解除 reconcile 并发送最新队列；失败保留修改和缺失版本，后续保存重试读取，不重发已确认 move。缺版本与随机数能力不足使用独立错误文案。

新建 `IsNew` 路径保留稳定客户端 `NoteId` 与现有 `Re.Item` 成功形状；本轮不增加 `OperationId` 或 create receipt，不承诺与存量 update 相同的 retry-safe 保证。

## 4. Batch and page-result design

delete/move/copy/shared-copy 在按钮/菜单意图产生时先完成原始有序 `noteIds`、action、目标 notebook、共享 flag/owner 的快照，再生成 `OperationId`。服务端以 owner/action/完整有序输入绑定 receipt 并按 noteId/index 派生子 operation；前端不排序、去重、重新生成子 ID，也不在同一意图的重试时重新读取当前选中项。

成功只按现有返回形状更新 UI：delete/move 仅当原始返回值严格为布尔 `true`（`ret === true`）时确认成功；`false` 或带 `Ok:false` 的 `info.Re` 对象（包括 noteService 不可用时带 `Msg:"storage"` 的返回）都是明确失败，不能把对象真值当成成功。copy 只有 `Re.Ok=true` 且读取 `Item` 才算成功。失败或传输失败应保留/恢复列表、选择与计数，并显示错误。对 unknown-result，既不乐观重复复制/删除，也不推断逐项成功；按 Q-PF-02 的生命周期重新提交原请求或显式核对。部分成功由服务端 receipt/后续读取确认，不能在前端创造新的 partial-success 协议。

## 5. Input, error and privacy constraints

- `OperationId` 使用浏览器加密随机源生成不重用 ID；随机源缺失或生成失败为可见错误，不用 `Math.random`、时间戳或 payload hash 兜底。服务端的非空、owner/action 和 digest 校验仍为最终边界。
- `ExpectedUsn` 必须是绑定当前 note 的已确认整数；缺失/失效时先按 Q-PF-01 指定方式取权威值，失败就阻止本次受保护保存。禁止把 `Usn` 与 editor `contentRevision` 混用。
- 业务错误和 HTTP/解析错误分别处理：明确冲突、无权限、validation、not-found 和 `partial_write` 均不能标记已保存；仅结果未知保留重试 identity。共享 AJAX wrapper 的 NOTLOGIN 与错误显示继续生效，不复制认证逻辑。
- 仅保留完成协议所必需的页面内存状态；CI/browser 证据只记录脱敏摘要。不向 localStorage/IndexedDB/sessionStorage 写入正文、token、cookie 或未确认请求体；重载不自动重放。

## 6. Verification and handoff

Node contract 覆盖构建闭包、回滚、版本/URL/i18n 与纯前端意图状态；Playwright 覆盖实际页面请求字段、编辑器状态、错误可见性、iframe 与清理。真实服务测试须用已配置的隔离 `leanote_test` harness 和实际请求确认目标行为，记录候选 SHA/环境/发现与执行数量。`delivery-verification` 汇合真实浏览器八槽、HTTP+DB receipt、Mongo 拓扑及故障注入。发现 server receipt/mapper 缺陷时重开对应 notes/interface owner，不在页面脚本复制业务规则。

## 7. Rollback points

构建链、jQuery/Bootstrap 资源、TinyMCE/插件、页面 mutation 协议按 manifest 和源文件边界分别可回退；回退后重建生成资源并验证零漂移。不得只回退生成文件或留下源/产物不一致，也不得让旧客户端可选字段兼容性因前端回滚而改变。
