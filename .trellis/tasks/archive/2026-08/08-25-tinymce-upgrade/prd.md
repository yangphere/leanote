# TinyMCE 8 升级（E-TM）— PRD

父任务：`.trellis/tasks/08-25-frontend-libs`

## Goal

把当前自托管的 TinyMCE **4.1.9** 升级到 npm 精确锁定的 **8.8.2**。保持主笔记和会员博客编辑入口、七种语言、粘贴/拖放图片、链接、列表、表格、代码块、目录与本地思维导图能力；以显式编辑状态和存量 HTML 契约保证未编辑内容不被新版序列化器或安全默认值意外写回。

## Confirmed Baseline And Dependencies

- 前端轨道严格按 jQuery → Bootstrap → TinyMCE 执行。唯一前置 `08-25-bootstrap-upgrade` 已归档完成，因此本任务是当前 ready 叶；完成后由 `08-25-frontend-libs` 汇总，CI/CD 仍等待该父任务。
- `public/tinymce/tinymce.js` 与 `tinymce.full.js` 均声明 `majorVersion: '4'`、`minorVersion: '1.9'`。`app/views/note/note.html` 加载 `/tinymce/tinymce.full.min.js`，`note-dev.html` 和两个会员博客模板加载 `/tinymce/tinymce.js`。
- 主笔记入口在 `public/js/app/page.js:490`；会员博客 `add_single.html:52` 和 `update_abstract.html:80` 是两个独立初始化入口，不能漏验。
- 主笔记当前启用官方插件 `autolink`、`link`、`lists`、`hr`、`paste`、`searchreplace`、`tabfocus`、`table`、`textcolor`，以及第一方插件 `leaui_image`、`leaui_mindmap`、`leanote_nav`、`leanote_code`。会员博客还启用 `advlist`、`charmap`、`visualblocks`、`visualchars`、`contextmenu`、`directionality`、`fullpage`。
- 当前保存逻辑明确不使用 TinyMCE `isDirty()`，而是在每次保存检查时比较缓存 HTML 与编辑器序列化结果。TinyMCE 8 仅加载便可能产生不同序列化，因此必须先建立“程序化加载”和“真实用户变更”的状态边界。
- `scripts/build/manifest.mjs` 当前只拥有七个 TinyMCE locale 输出，尚不拥有 core/theme/icon/model/skin/plugin 资源；完整运行资源纳入 manifest 是本任务职责。

## Requirements

### R-TM1: Single Self-hosted Runtime And Build Ownership

- `package.json` 与 `package-lock.json` 精确锁定 `tinymce` **8.8.2**；所有运行时资源由 `scripts/build/manifest.mjs` 从锁定包、第一方可读源码和现有消息源生成，不从 CDN、全局安装、未锁定下载或历史生成物反向取材。
- manifest 必须显式拥有 TinyMCE core、Silver theme、默认 icon/model、Oxide UI/content skin、七语言包、四个第一方插件及其运行所需静态资源，并同步输出计数、输入存在性、Git 跟踪、资源 URL、失败原子性和零 diff 测试。可读源码是唯一编辑入口；压缩 core/plugin/skin 是生成物，不得手工同步。
- 保持 `/tinymce/` 自托管资源根；所有 tracked 模板只加载 manifest 声明的 TinyMCE 8 URL。`note-dev.html` 仍是 `note.html` 的模板源，生产笔记加载压缩资源，开发入口可加载可读资源；两个入口必须解析到同一 8.8.2 闭包。
- 每个初始化入口显式配置 `license_key: 'gpl'`。生产和测试均不得出现 cloud API key、商业 license manager、付费插件、TinyMCE 4 core、第二份 runtime 或在线 fallback。

### R-TM2: Complete Entry, Plugin And UI Migration

- 主笔记、`member/blog/add_single`、`member/blog/update_abstract` 使用同一受测试的基础配置事实来源，仅保留入口所需的插件/toolbar 差异；不得继续维护三份漂移的 TinyMCE 选项表。
- 删除 TinyMCE 4 `leanote` theme 与 `custom` skin 依赖，基于 TinyMCE 8 Silver/Oxide 和应用拥有的最小布局覆盖保持现有双行工具栏、inline 编辑区、写作模式、移动布局和对话框可用；不得复制或伪装 TinyMCE 4 DOM。应用 LESS 中依赖旧 `.mce-*` UI 结构的规则必须逐项判定为内容规则、应用布局规则或可删除旧 UI 规则，再迁移到受支持结构。
- 官方插件逐项处理：继续加载 `advlist`、`autolink`、`charmap`、`directionality`、`link`、`lists`、`searchreplace`、`table`、`visualblocks`、`visualchars`；`paste`、`hr`、`contextmenu` 和颜色能力使用 TinyMCE 8 core，不再作为独立插件；删除已移除的 `tabfocus`；删除会员博客中无可见入口且已成为付费能力的 `fullpage`，并用内容保存/重载测试证明其移除不丢失 HTML fragment。
- 工具栏名迁移为 `formatselect → blocks`、`fontselect → fontfamily`、`fontsizeselect → fontsize`，保留现有可见命令集合、字体列表、块格式、只读限制和键盘行为。任何无法等价迁移的可见命令必须返回规划，不得静默消失。

### R-TM3: First-party Plugin Contracts

- 迁移 `leaui_image`、`leaui_mindmap`、`leanote_nav`、`leanote_code` 到 TinyMCE 8 公共 API：`editor.ui.registry.*`、公开 command/event/selection/DOM API，以及 `windowManager.open` 的 v8 dialog schema 或 `openUrl` URL dialog；不得复制 core、调用移除的 v4 UI API或增加全局兼容 shim。
- `leaui_image` 保持上传、相册/URL 选择、替代文本、尺寸/标题、插入和更新现有图片、拖放与失败恢复语义，并复用已完成的 Bootstrap 5 iframe UI 与现有 `top.LEAUI_DATAS`/`GlobalConfigs`/`getMsg`/`mdGetImgSrc` 边界。
- `leaui_mindmap` 保持本地 KityMinder、插入/重开、图片预览和 `img[data-mind-json]` JSON 往返。`leaui_mind` 当前未启用且仅残留外部 `//leanote.com/public/libs/mind/edit.html` dead path；不得迁移或保留该 fallback，完成引用证明后删除插件及外部依赖，不把它合并成第二条运行路径。
- `leanote_nav` 只更新 `#leanoteNavContent` 等编辑器外部导航 DOM；初始化、标题扫描、点击、撤销/重做、粘贴或命令触发的导航刷新不得修改存储内容或标记内容为已编辑。
- `leanote_code` 保持代码块/行内代码、语言选择、Ace 初始化与清理、相关 class/data 属性、快捷键、插入/更新、保存和重载语义。四个插件分别具有初始化、主要动作、序列化、重载、只读及错误路径验收。

### R-TM4: Explicit Edit-state And Save Contract

- 编辑状态按当前 note/load epoch 隔离。程序化 `setContent`、初始化序列化、`undoManager.clear`、Ace 装饰/清理、导航 DOM 刷新、只读/可写切换和 note 切换不得被记录为用户内容修改；延迟事件不得污染已切换到的新 note。
- 每次加载保留两类基线：数据库/缓存中的原始 `persistedContent` 字节，以及加载完成后的 `editorBaseline` 序列化。用户键盘输入、剪切、粘贴、拖放、undo/redo 和四个插件的插入/更新进入同一内容 revision；保存时当前序列化与 `editorBaseline` 相同（包括编辑后完整撤销）则不发送 `Content`。
- 仅标题或标签变化时请求必须省略 `Content`，即使 TinyMCE 8 的加载序列化与数据库原文不同。打开后关闭/刷新、切换 note、只读↔可写切换、强制保存或 Ctrl/Cmd+S 均不得单独导致未编辑内容写回。
- 内容保存捕获 note id、load epoch、revision 和提交字节。成功确认后，无论期间是否已有后续编辑，都把该次提交字节同时设为 `persistedContent` 与 `editorBaseline`，并以“当前序列化是否等于该提交字节”重新计算 dirty；只确认捕获的 revision，后续 revision 继续待保存。A→B 发起保存、期间编辑到 C、B 成功时，两类基线均为 B，C 仍 dirty；随后撤销到 B 清 dirty，继续撤销到 A 必须 dirty。
- `/note/updateNoteOrContent` 的保存结果统一使用项目现有 `info.Re` 结构（`Ok`/`Msg`，新建成功的笔记放入 `Item`）。controller 必须检查每个本次实际请求的 `UpdateNote`/`UpdateNoteContent` 返回值；新建服务返回零值 note 或权限/插入失败也必须返回 `Ok:false` 和非空可见 `Msg`，不得把裸零值当成功。任一业务失败、HTTP 失败或异常都不得返回成功，前端只有在 `reIsOk` 为真后才能确认 revision、更新两类基线或显示保存成功。若元数据已写入而内容更新失败，接口明确返回失败且客户端保持本次内容可重试，不把非事务性部分写入伪装成整体成功。
- 新建笔记的首次保存仍提交其真实初始/编辑内容；共享只读笔记不能通过键盘、粘贴、拖放、插件、命令或保存边界改写内容。

### R-TM5: Stored HTML And Security Compatibility

- 未发生 R-TM4 定义的内容修改时，不调用内容保存，数据库 HTML 原文字节不变；不以 DOM 等价替代该零写入要求。
- 发生真实编辑并保存时，以 DOM 语义夹具保护普通富文本、标题、列表、链接 `href/target/rel`、图片 `src/alt/title/width/height`、表格结构与合并属性、代码文本/class/data、目录相关标题和 `data-mind-json`。只允许夹具逐项登记的空白、空元素写法、属性顺序和 TinyMCE 内部临时属性清理；宽泛 sanitizer 或“忽略未知差异”不合格。
- 不全局关闭 TinyMCE 8 的 DOMPurify、`sandbox_iframes` 或 `convert_unsafe_embeds` 安全默认值。`script`、`object`、`embed`、危险 URL/属性和 iframe 分别有固定夹具，变换或拒绝必须可见且不产生空内容伪成功；这些可执行/嵌入元素不属于 ADR 承诺的受支持语义集合。未编辑的历史记录仍由零写入规则保护。
- 不修改笔记 Schema、不批量读取或重写历史记录、不增加后端兼容分支。

### R-TM6: Paste, Drop, Upload And Error Visibility

- 删除绑定 TinyMCE 4 `paste/classes/Clipboard.js` 与 `tinymce.full.*` 内部模块的测试/补丁，改为针对 TinyMCE 8 core paste 与 Leanote 图片边界的行为测试；不得 fork TinyMCE 8 paste core。
- 固定富文本、纯文本、Office 风格内容、代码、远程图片、data URL 图片、剪贴板图片、拖放图片、Leanote 内部 HTML、非法节点和空输入夹具。单次粘贴/拖放只插入一次图片，仍需阻止浏览器与自定义上传的重复处理。
- 上传/转换失败保留用户可恢复的原输入或待重试状态，显示明确错误，不返回空字符串、不吞异常、不显示成功消息；成功插入/更新计入内容 revision 并可撤销/重做。

### R-TM7: Seven-locale Contract

- 支持 `de-de`、`en-us`、`es-co`、`fr-fr`、`pt-pt`、`zh-cn`、`zh-hk` 七个应用 locale。使用一个明确映射转换为 TinyMCE 8 RFC5646 language code，并同时设置匹配的 `language` 与根相对 `language_url`；不得因 underscore 或错误 code/filename 产生弃用警告或 404。
- `messages/<locale>/tinymce_editor.conf` 继续是项目翻译事实来源，manifest 生成 TinyMCE 8 兼容 `addI18n` 输出；迁移后的实际工具栏、对话框和第一方插件键必须在七种 locale 下显示可读文本，未知键不得显示为内部 token。
- `public/tinymce/langs/en.js`、`zh.js` 和 `readme.md` 先做全仓、manifest 与运行时请求证明；无引用则删除，有引用则纳入 manifest 和兼容测试。不得保留未登记语言副本作为隐藏 fallback。

### R-TM8: Verification, Browser And Delivery Boundaries

- 复用现有 Node 24、Playwright 1.62.1、单一 `playwright.config.mjs`、test-mode identity/run-token、新鲜写入确认和脱敏 reporter。业务用例文件使用 `.spec.mjs`；Chromium `business` 是 PR/push 阻断门禁。
- Chromium 覆盖主笔记与两个会员博客入口的加载、编辑、标题/标签-only 保存、未编辑切换/刷新、只读切换、undo/redo、粘贴/拖放、图片上传、代码块、目录、思维导图、自动/强制保存、并发保存失败和重载；同时断言资源、console、pageerror、unhandled rejection、许可和弃用告警。
- 按父任务在真实 Chrome、Edge、Firefox、Safari 当前及前一主版本执行发布 smoke，并把 commit、日期、产品/完整版本、OS、覆盖入口/iframe、identity/error gate 和结果写入 `docs/modernization/browser-smoke/tinymce-8.md`。Safari 只接受真实 Safari，Chromium 不冒充 Chrome/Edge/Safari；缺失环境证据明确为发布阻断，不记为通过。
- 通过 `npm ci && npm run build && npm test`、`npm run test:e2e:build`、`npm run test:e2e`、Golden/USN 回归、两次连续构建零 diff、manifest/引用/版本扫描和 `git diff --check`。无法获得真实服务、MongoDB、账号或真实 Safari 时必须报告验收缺口，不得使用 mock 成功路径。

## Acceptance Criteria

- [ ] **AC-TM1** `npm ls tinymce` 只显示 8.8.2；manifest 拥有完整运行闭包，所有模板从 `/tinymce/` 加载声明资源并显式 `license_key: 'gpl'`；扫描无 TinyMCE 4、CDN、商业/付费插件、第二 runtime 或未声明静态文件。
- [ ] **AC-TM2** 主笔记、会员博客单页与摘要共用基础配置并全部启动；官方插件按 R-TM2 处置，toolbar 新名称生效；Silver/Oxide 下双行工具栏、inline 编辑、写作/移动模式和对话框无功能丢失或旧 `.mce-*` UI 依赖。
- [ ] **AC-TM3** 四个第一方插件分别通过初始化、主要动作、插入/更新、undo/redo、保存、重载、只读和失败测试；`leaui_mind` 及其外部 URL 删除，`leaui_mindmap` 的本地 `data-mind-json` 往返不变。
- [ ] **AC-TM4** 真实服务/MongoDB 证明打开关闭、note 切换、只读切换、强制保存和标题/标签-only 保存不发送未编辑 `Content`，数据库 HTML 字节不变；`/note/updateNoteOrContent` 对新建/更新成功返回 `Ok:true`（新建 note 在 `Item`），对新建零值/权限失败、不存在、无权限、冲突、数据库更新失败及“元数据已成功但内容失败”的部分写入返回 `Ok:false` 与非空可见 `Msg`。前端在业务/HTTP 失败时不确认 revision、不更新基线、不显示成功并可重试；A→B 保存期间编辑 C、确认 B 后撤销到 B/A 的 dirty 结果严格符合 R-TM4。
- [ ] **AC-TM5** 存量 HTML 夹具逐项证明受支持 DOM 语义与插件标记；安全夹具证明 TinyMCE 8 默认值未被放宽，危险内容不会静默变空或伪成功；没有历史批量迁移或 Schema 变化。
- [ ] **AC-TM6** 粘贴/拖放行为夹具覆盖 R-TM6 全部输入，剪贴板/data URL 图片只插入一次；旧 TinyMCE 4 内部 `Clipboard.js`/full-bundle 测试被行为级测试取代，上传/转换失败可见且可恢复。
- [ ] **AC-TM7** 七个 locale 在三个编辑入口均加载成功，核心命令、对话框和第一方插件文本可读；无语言 404、underscore/RFC5646 warning 或未登记 `en.js`/`zh.js` fallback。
- [ ] **AC-TM8** Chromium business E2E、build smoke、Node 测试、Golden/USN、两次 build 零 diff和错误门禁全部通过；真实四浏览器两版 smoke 按脱敏记录契约齐全，否则明确阻断发布。

## Out Of Scope

- 批量规范化/重写数据库历史 HTML，修改笔记 Schema，或广泛重设计 API、USN 与认证；满足 R-TM4 所需的 `/note/updateNoteOrContent` 最小结构化响应修正除外。
- Tiny Cloud、商业 license manager、付费插件、协同编辑或任何在线编辑器 fallback。
- 用另一款编辑器替换 TinyMCE，SPA/ESM 业务重构或编辑器视觉重设计。
- 再次升级 jQuery/Bootstrap，或顺带修复与本迁移无关的历史编辑器功能。
