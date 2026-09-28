# 呈现层：前端构建与编辑器运行时 — PRD

## Goal and ownership

在现有服务端渲染页面上维护可重复的 Node 24/esbuild 前端生成链，并使 jQuery、Bootstrap、TinyMCE、第一方插件和笔记工作区的浏览器行为符合已冻结的业务/HTTP 契约。本任务拥有前端源码、模板源码、构建脚本及前端回归；`application-notes` 拥有权限、USN、receipt 和 mutation 结果，`interface-http` 拥有绑定及响应映射，`delivery-verification` 拥有跨层和发布前真实环境矩阵。

## Baseline and dependencies

- `meta.depends_on` 仅有 `09-08-domain-contracts`、`09-08-application-notes`，两者均已归档且为 `completed`。归档不代表其委托的浏览器、Mongo 拓扑或故障注入证据已通过。
- ADR-0003 固定服务端模板、历史 URL、未编辑零写入、编辑后 HTML 语义等价、Node 24、生成资源继续跟踪，以及 Chrome/Edge/Firefox/Safari 当前和前一主版本的发布前矩阵。
- 当前 `package.json` 固定 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2、esbuild 0.28.2；`scripts/build/manifest.mjs` 当前导出 164 个输出、7 个 locale 和 4 个第一方 TinyMCE 插件。这些数量是审核基线，增减须由 manifest 与测试解释，不能成为手工维护的第二清单。
- 审核起点：Web `UpdateNoteOrContent` 已接受可选 `OperationId`/`ExpectedUsn`，copy/shared-copy/delete/move 接受可选 `OperationId`，第一方浏览器尚未生成。2026-09-28 续接已接入第一方请求意图；缺字段的旧客户端仍按 `RetrySafe=false` 契约处理。

## Functional requirements

### R-PF1 — Build inputs, outputs and failure boundary

- 只支持 Node 24.x；lockfile 固定 npm 输入，manifest 是 JS/CSS、TinyMCE assets、7 组语言输出和 `note-dev.html` 到 `note.html` 的唯一源码到产物清单。编辑源码/语言/模板和 manifest，不手改已声明的生成文件。
- 所有声明输出继续由 Git 跟踪；相同输入连续构建字节稳定，干净检出执行 `npm ci && npm run build && npm test` 后 `git diff --exit-code` 为零。新增或删除输出同步修改 manifest、源码引用和契约测试，不靠固定计数掩盖缺失资产。
- 构建在发布前验证输入、输出、i18n 扫描根、staging/backup 路径及 symlink/junction 逃逸；缺失/重复/越界/不支持版本明确失败。中途失败恢复先前完整产物；恢复或清理失败时保留可恢复材料和错误信息，不把部分发布当成功。
- i18n 只扫描源码，不把生成文件再次当输入；静态 key 必须在对应语言文件中存在，动态/缺失 key 的诊断定位到 `path:line:column`。任何例外须在 manifest 中显式登记并经测试覆盖。

### R-PF2 — Runtime and URL compatibility

- 生产页面和独立 iframe 各自仅加载实际需要的库，所加载的 jQuery、Bootstrap、TinyMCE 分别使用受锁的 3.7.1、5.3.8、8.8.2；同一浏览上下文不得混入旧版本或 `jquery-migrate`。`jquery-migrate` 仅用于开发诊断且警告清零；不承诺 IE 支持。
- 保留现有公开 URL、模板引用和 self-hosted 资源，包括指向 jQuery 3.7.1 的历史路径 `/js/jquery-1.9.0.min.js`、Bootstrap/TinyMCE 路径、`leaui_image` iframe 和博客主题资源。历史文件名不代表旧 runtime；不引入 CDN 作为成功路径。
- 保留 Bootstrap 5 的 modal/tab/dropdown/dialog 行为与 reopen/cleanup，及 TinyMCE GPL、语言包、粘贴、只读、撤销/重做、上传和图片对话框契约。第一方插件限定 `leaui_image`、`leaui_mindmap`、`leanote_nav`、`leanote_code`，插件 UI、iframe 消息与资源失败均须可见、可测。

### R-PF3 — Editor state and save flow

- `LeanoteEditorSession` 是内容 dirty、load epoch、revision 与确认状态的唯一前端事实来源；`Note` cache 只镜像已确认的服务端笔记，不建立第二套 editor baseline。只读笔记和未编辑存量笔记不发 mutation，未编辑时 DB HTML、history、USN 均不变。
- 标题/标签等 metadata-only 保存不得附带未变化的 `Content`；内容保存只在明确成功且属于当前 note/load epoch 与有效 revision 时确认 dirty 状态。较早响应、切换笔记、加载竞争、连续编辑和失败不得清除较新的未保存内容。
- 实际编辑后只允许 ADR-0003 已登记的非语义 HTML 规范化；正文、链接、图片、代码块、表格和第一方插件标记语义保持。Markdown 源码/预览、富文本、共享可写与只读路径均纳入回归。
- HTTP 成功不等于业务成功；仅按逐 action 的既有 `Re.Ok`、boolean 或 `Re.Item` 形状确认。网络/解析/业务失败须显示错误，保留待处理的用户修改，避免把乐观隐藏或本地 cache 当成服务器提交。

### R-PF4 — First-party mutation identity

- 对存量笔记 `UpdateNoteOrContent` 的每个实际用户意图生成新的非空 `OperationId`，并发送该笔记在提交前由服务端确认的 `ExpectedUsn`。标题/标签/正文组合属于同一次提交意图；无内容变化时不得凭 force 参数制造保存。
- 对私有/共享 copy、共享/私有 delete 和 move 的每个单项或有序 batch 意图生成新的 `OperationId`。同一请求的 action、原始有序 noteIds、目标 notebook、共享 owner/flag 和其他输入在发送前冻结；同一意图的 unknown-result 重试原样复用 ID、顺序和 payload。新意图生成新 ID；不得以 payload hash、当前时间、推测的 USN 或从旧请求沿用的 ID 代替 generation。
- 浏览器生成 ID 须有足够随机性、不会与其他用户意图意外复用；生成能力不可用时明确失败，不静默省略字段。服务端继续负责 owner/action/input digest 绑定和稳定子 operation；前端不重算 receipt、权限或子 operation ID。
- 一个 note 的前一保存处于 pending/unknown 时，后续同 note 的变更不能带着新的意图越过它；用户可以继续编辑，但必须在前一结果确认或显式冲突处理后才提交下一笔。业务冲突不得修改并复用旧 `OperationId`；用户解决冲突形成新意图。
- 旧客户端省略可选字段时现有 HTTP/JSON 外形不变，且不宣称 retry-safe/stale-write 保护。第一方 Web 不能通过省略字段来绕开本任务的承诺。

### R-PF5 — Result, recovery and visible state

- 明确区分已确认成功、明确业务失败/冲突、以及请求已发但结果未知。unknown-result 保留同一冻结意图供重试或核对；不能先清理/重建前端状态，再以新 ID 盲重放。重试得到已提交结果时，UI/cache 只应用一次。
- `ExpectedUsn` 只能来自已确认的服务端 revision；不能使用 `previousUsn + 1`、响应时间或本地编辑次数。受保护存量保存成功响应直接返回本次提交/receipt 的精确 committed `Usn`，供下一次保存使用（Q-PF-01 已确认）。拿不到可信 revision 时不能发送自称受 stale-write 保护的新 mutation。
- batch 的单项及多项操作须保留输入顺序；copy 成功使用既有 `Re.Item` 笔记列表更新缓存。delete/move 只有原始返回值严格为 `true`（`ret === true`）才确认成功；`false`、带 `Ok:false` 的 `info.Re` 对象（包括服务不可用时带 `Msg:"storage"` 的返回）是明确失败，传输失败进入结果未知。两者均不得永久移除可见笔记、清空选择或伪造计数；未知结果只重试原意图，明确失败先保留修改并刷新核对。已提交但返回未知时不自行推断每项结果。
- 待确认请求只保留在当前页面内存；pending/unknown 时离开前提示，重载后先核对服务端状态，不自动重放旧操作或生成新 ID 盲重试（Q-PF-02 已确认）。认证材料、正文与请求体不得写入浏览器持久存储。

### R-PF6 — Error visibility and evidence hygiene

- 登录、笔记/Markdown、modal/tab/dropdown、上传/相册、admin/member/blog、iframe 的应用自有资源 4xx/5xx、`console.error`、`pageerror`、未处理 rejection 和清理失败可观察并按既有测试门禁处理；不能以吞错、mock 成功或隐式启动服务掩盖失败。
- 自动/手工证据记录候选 commit、运行环境、Node/浏览器实际版本、发现/执行/通过/失败/跳过数及清理结果。CI artifact 仅允许脱敏摘要；不得保存 cookie、token、页面正文、请求体、trace、视频或截图等敏感原始内容。

## Scope and dependencies

| 边界 | 本任务 | 其他 owner |
| --- | --- | --- |
| 构建/模板/插件 | 修改源码和 manifest，验证生成闭包与页面行为 | `delivery-verification` 做候选提交及发布环境联验 |
| Web 笔记 mutation | 生成/保留前端意图字段、维护编辑器与 UI 结果状态 | `application-notes` 拥有 USN、receipt、权限和 batch 幂等 |
| HTTP 绑定/响应 | 消费已确认的逐 action wire contract | `interface-http` 拥有 binder 和 `Re`/boolean 映射；若需新增权威 USN 返回，须由其与 notes 共同确认 |
| 浏览器支持 | 本地契约、Chromium 和可用真实浏览器 smoke 记录 | `delivery-verification` 汇合当前/前一主版本八槽 artifact；Safari 必须真实运行 |

不改产品视觉设计、公开 URL、`/api/*` 契约、业务授权/USN/receipt 实现；不引入 SPA、TypeScript、第二个编辑器状态容器或长期保存用户正文的离线队列。服务端缺陷返回其 owner，不在页面脚本模拟修复。

## Acceptance criteria

- [ ] AC-PF1：Node 24 干净检出运行 `npm ci && npm run build && npm test`，两次构建及 CI 重建对全部 manifest 声明输出零漂移；失败/回滚、路径安全和 i18n 负向测试通过。
- [ ] AC-PF2：生产加载的三个锁定库各无混版、CDN、migrate 或失效旧副本；历史 URL 与 `note-dev.html`/`note.html`、7 locale、四个第一方插件资源契约通过。
- [ ] AC-PF3：既有业务/编辑器/Bootstrap/iframe smoke 的成功、错误、上传及清理门禁通过；明确记录实际 browser/service 版本和执行结果。
- [ ] AC-PF4：未编辑零请求/零写入，metadata-only 不传 `Content`，编辑后 HTML 语义、dirty/revision、切换 note/重叠响应、只读与失败保留改动均通过针对性测试。
- [ ] AC-PF5：第一方 Web 请求逐 action 证明新意图换 `OperationId`，存量 update 带权威 `ExpectedUsn`，同一 unknown-result 重试冻结相同 payload/ID，batch 顺序不变；delete/move 只有 `ret === true` 才算成功，并以 `false` 及带 `Ok:false` 的 `info.Re`（含 `Msg:"storage"`）作为负例证明列表、选择和计数不被伪成功清理；冲突和旧客户端字段省略的边界可核验。
- [x] AC-PF6：Q-PF-01 和 Q-PF-02 经用户逐项确认，已写入 PRD/design 与可执行 Node/Go 契约测试；AC-PF5 的真实 HTTP/DB/browser 证据仍需独立验收。
- [ ] AC-PF7：真实浏览器、HTTP、DB receipt、跨进程及八槽矩阵只由实际执行关闭；本任务把未运行或委托证据以 `unrun`/`delegated-unrun` 交给 `delivery-verification`，不能以 Node/Chromium 代替 Safari 或 Mongo 故障证据。

## Confirmed decisions — 2026-09-28 continuation

- **Q-PF-01（已确认）**：允许扩展存量 Web save 成功响应，直接返回 `SaveNote` 本次提交/receipt 的精确 `Usn`；不得用读回最新版本或本地加一替代。仅对携带 `OperationId` 的受保护存量请求返回新增字段，旧客户端未提供字段时保留既有响应形状。本轮仅保护存量保存；新建沿用现有稳定 `NoteId` 和 `Re.Item`，不增加 create receipt，也不宣称同等 unknown-result 安全重试。
- **Q-PF-02（已确认）**：冻结请求只保留于当前页面内存。pending/unknown 时离开前提示；重载后必须先核对服务端状态，不能自动重放旧操作或生成新 ID 盲重试。认证材料、正文与请求体不写入浏览器持久存储。
- 上述决定由用户在本次 `$trellis-continue` 中逐项确认；授权的跨层改动限于 committed-USN Web mapper 及对应测试，服务端权限、receipt 和业务提交规则保持原有 owner。

## Implementation closeout — 2026-09-28

用户在四项审核修复和本地验证完成后明确要求“提交并归档”。本次归档记录实现与审核修复收尾，不把归档状态等同于发布验收通过。AC-PF1～PF5/PF7 尚缺的干净候选/CI、真实 HTTP/DB/browser、跨进程/failpoint 与八槽证据继续保持原 partial/unrun/delegated-unrun；由 `09-08-delivery-verification` 统一承接，本任务 acceptance/evidence-matrix.md 的未勾选操作是交接清单。

## Historical decision questions (resolved above)

- **Q-PF-01：Web save 的权威 revision 与新建字段范围。** 存量保存成功的 `info.Re` 当前不返回新 `Usn`，而新建分支忽略 `OperationId`。需确认存量保存后由哪个现有/新增接口提供精确已提交 `Usn`，以及新建笔记是否必须消费客户端 `OperationId`；若新增响应字段或 create receipt，分别需 `interface-http`/`application-notes` 认可。未定会阻断连续保存、冲突和新建 unknown-result 的可验收语义。
- **Q-PF-02：unknown-result 跨页面重载边界。** 需确认只在同一页面内存生命周期支持冻结意图重试，还是要跨 reload 显式恢复；后者需要持久意图定位、敏感数据保护、登录主体切换及过期规则。未定会阻断页面重载场景及相应验收，不影响独立的构建/运行时审核。
