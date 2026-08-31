# TinyMCE 8 升级（E-TM）— 技术设计

## 1. Ownership And Runtime Closure

`tinymce@8.8.2` 是唯一 core 输入。`scripts/build/manifest.mjs` 把运行闭包拆成可审计条目：core、`themes/silver`、默认 icons/model、`skins/ui/oxide`、所需 content skin、七语言输出、官方插件、四个第一方插件及其 image/mindmap 静态资源。manifest 同时拥有可读开发资源和压缩生产资源，构建测试校验版本、输入、输出、URL、Git 跟踪与引用闭包。

自托管根保持 `/tinymce/`。模板不手写另一份资源清单：`note-dev.html` 使用可读 core，生成的 `note.html` 使用压缩 core，会员博客入口通过共享配置加载同一闭包。TinyMCE 计算插件/skin base URL 所需的目录结构由 manifest 固定。任何网络请求到 Tiny Cloud、`leanote.com` 思维导图页或未声明路径均失败。

## 2. Shared Configuration And UI Boundary

新增一个浏览器全局可用、可由 Node 静态测试的配置模块，输出：

- 基础配置：inline、GPL、URL 转换、font/block formats、locale mapping、Silver/Oxide、menubar/statusbar、只读和事件接线；
- note profile：主笔记官方插件、四个第一方插件、双行 toolbar、Ace/保存状态 adapter；
- member-blog profile：只包含其可见命令，不包含 `fullpage`、`tabfocus` 或第一方 note 插件。

三个入口只传 selector、locale、profile 和必要 host adapter。旧 `leanote` theme、`custom` skin 和重复初始化对象删除。应用 LESS 先按运行职责分类：编辑内容选择器（例如 `.mce-item-table`）保留/迁移为内容语义；编辑器 chrome/layout 迁到 Silver/Oxide 的稳定 `.tox-*` 结构；纯 v4 dialog/button/icon 规则删除。覆盖保持紧贴 Leanote 布局，不复制 Oxide 源码。

官方插件处置由 PRD R-TM2 的唯一表驱动。构建/配置测试断言 integrated/removed/premium 名称不再出现在 `plugins` 中，旧 toolbar 名不再出现，可见命令仍在对应 profile。

## 3. Edit-state Model

编辑状态不直接等同 TinyMCE `isDirty()`，也不再仅用“缓存原文 vs 当前序列化”判断。每个活动编辑会话保存：

| Field | Meaning |
| --- | --- |
| `noteId` / `loadEpoch` | 隔离 note 切换及延迟事件 |
| `persistedContent` | 服务端/缓存中的原始字节，未编辑时不得覆盖 |
| `editorBaseline` | 当前 note 程序化加载完成后的编辑器序列化 |
| `contentRevision` | 用户内容动作的单调 revision |
| `confirmedRevision` | 最近成功保存确认的 revision |
| `loading` | `setContent`、Ace/导航初始化期间的事件抑制边界 |

状态流：

1. 切换 note 时先增加 epoch 并进入 `loading`，调用 `setContent`、清 undo、完成 Ace/导航初始化，然后记录 `editorBaseline` 并退出 `loading`。旧 epoch 的回调直接丢弃。
2. 键盘、cut/paste/drop、undo/redo 和插件插入/更新通过一个 `markContentMutation` 入口增加 revision。导航外部 DOM、Ace 临时属性和程序化 setContent 不调用该入口。
3. 保存前若当前序列化等于 `editorBaseline`，视为没有待提交内容，即使发生过随后完全撤销的动作；标题/标签可独立提交，payload 省略 `Content`。
4. 内容保存捕获 note/epoch/revision/serializedContent。成功回调无条件把本次 `serializedContent` 同时写入 `persistedContent` 和 `editorBaseline`，只把捕获的 revision 标记为 confirmed，再用当前序列化与本次提交字节比较 dirty。若当前仍为 B 则清 dirty；若已编辑到 C 则保留 dirty；从 C 撤销到 B 清 dirty，继续撤销到保存前的 A 重新 dirty。失败不修改持久化缓存、baseline 或 revision。
5. read-only gate 同时作用于键盘、paste/drop、命令和第一方插件，而不只拦 keydown。

现有 `Note.curHasChanged`/`curChangedSaveIt` 仍拥有 title/tag/new-note/derived fields，但内容字段必须通过上述 adapter 决定。

## 4. Save Response Boundary

`/note/updateNoteOrContent` 不能继续丢弃 `NoteService.UpdateNote` 与 `UpdateNoteContent` 的 `(ok, msg, usn)` 并无条件返回 `true`。本任务复用 `app/info/Re.go` 的 `info.Re`，为该端点建立最小统一 envelope：整体成功为 `Ok:true`；新建成功时 `Item` 携带现有 note 对象；新建服务必须把权限、插入和内容写入结果转换为可判定结果，零值 note 或其他失败均为 `Ok:false`，`Msg` 必须非空，优先保留服务层的 `notExists`、`noAuth`、`conflict`，空消息使用明确的保存失败消息。若现有 `info.Note` 返回值无法区分 Mongo 插入失败，允许只调整 controller 专用 service helper 的内部返回类型，不改变 HTTP 端点或其他调用方。前端解包新建 `Item`，并对新建/更新都先调用 `reIsOk`，只有业务成功才能确认 revision、推进两类 baseline 和显示成功。

controller 按请求字段检查返回值：元数据更新失败后不继续内容更新；元数据已成功而内容更新失败时，承认既有 Mongo 写入不是事务，返回整体失败并让客户端保留内容与 revision 以便重试，绝不把部分写入报告为成功。该边界不引入新 Schema、USN 算法、事务层或通用 API 重构；它只是让现有保存状态机获得可判定的业务结果。

## 5. First-party Plugin Migration

- `leaui_image`：按钮/菜单改用 UI Registry；旧 `open({ url })` 改为 TinyMCE 8 `openUrl` 或等价公开 URL-dialog API，保留 Bootstrap 5 iframe 与 `top.LEAUI_DATAS` 边界。图片插入/更新最终调用统一 mutation adapter。
- `leaui_mindmap`：使用本地 mindmap URL dialog；保留 `data-mind-json`、预览 data URL 和编辑已有节点。`leaui_mind` 仅残留外部 leanote.com dead path，不属于可保留能力，完成引用证明后直接删除。
- `leanote_nav`：保留 event-driven heading scan，但事件名按 v8 规范迁移；输出只写 editor 外的导航容器。测试用 observer/请求断言证明其刷新不改变 `getContent` 或 revision。
- `leanote_code`：listbox/button 改 UI Registry 和 v8 对话框 schema；selection insertion 改 `editor.insertContent`；Ace 的 UI 装饰在序列化副本中清理，真实代码变更进入 mutation adapter。

每个插件返回最小公开 metadata，便于启动测试确认注册成功；不导出 TinyMCE 私有对象，不实现 v4 API facade。

## 6. Paste, Content And Security

TinyMCE 8 core 拥有普通 paste；Leanote adapter 只拥有图片上传/插入、内部 HTML 约束和错误呈现。旧 `paste/classes/Clipboard.js` patch 与 full-bundle 字节测试删除，由行为夹具验证“浏览器 paste 与自定义图片处理只能有一个 owner”。

内容兼容测试保存两份期望：原始字节用于未编辑零写入路径，结构化 DOM expectation 用于真实编辑保存路径。比较器只规范化登记的空白、空元素和属性顺序；链接、图片、表格、代码及 `data-mind-json` 分字段比较，不做宽泛字符串清洗。

TinyMCE 8 的 DOMPurify、iframe sandbox 和 unsafe embed conversion 保持默认。安全夹具把受支持语义与可执行/嵌入内容分开：未编辑时后者因不保存而保留数据库原文；用户真实编辑后按 v8 安全规则转换/拒绝，并产生非空、可观察结果。实现不得用 `valid_elements: '*[*]'`、禁用 sandbox 或 post-filter 恢复危险属性绕过门禁。

## 7. Locale Mapping

配置模块集中维护应用 locale → TinyMCE RFC5646 code → manifest URL 映射，例如 `zh-cn → zh-CN → /tinymce/langs/zh-cn.js`。`language` 使用 canonical code，`language_url` 保持现有根相对 URL，生成文件内部 `addI18n` code 与 `language` 一致。

`messages/<locale>/tinymce_editor.conf` 仍是翻译源；生成器只改变 TinyMCE 8 所需封装/键集合。语言测试在七种 profile 初始化中读取实际 toolbar/dialog label，并扫描 console/404。`en.js`、`zh.js`、`readme.md` 只按引用清单决定删除或纳管，不作为 runtime fallback。

## 8. Verification And Evidence

Node 层覆盖 manifest closure、配置 profile、plugin registration、locale mapping、保存状态转换（含 A→B 保存、期间编辑 C、确认 B 后撤销到 B/A）、HTML comparator 和 paste/upload adapter。真实服务/MongoDB 的响应契约回归放在 `app/tests/harness/` 与现有 `tests/e2e/business/business-flows.spec.mjs`；Playwright `tests/e2e/business/editor-flows.spec.mjs` 覆盖主笔记和两个 member 入口，`ajax-failure.spec.mjs` 覆盖 HTTP 失败的可见错误。每次写入前调用 fresh identity confirmation，结束后清理本次 run-token 创建的数据。

`browser-smoke` project 必须发现 editor flow 或等价 editor smoke，四浏览器发布结果写入 `docs/modernization/browser-smoke/tinymce-8.md`。报告只含 commit、产品/完整版本、OS、覆盖入口/iframe、identity/error gate 与结果，不含正文、凭据、Cookie、header、截图或 storage state。

## 9. Rollback

TinyMCE package/manifest、三个初始化入口、状态 adapter、`UpdateNoteOrContent` 的 `info.Re` 响应与前端解包、四插件、应用样式及 locale 作为一个原子迁移单元。任一资源闭包、未编辑零写入、内容兼容、保存响应、插件或错误门禁失败都回退整个任务；不得留下新旧响应混用、双 core、runtime feature flag 或 TinyMCE 4 隐藏 fallback。数据库没有迁移，因此回滚不执行数据变换。
