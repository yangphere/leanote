# TinyMCE 8 升级（E-TM）— 执行计划

## Global Constraints

- 只使用 npm 锁定的 TinyMCE 8.8.2、自托管 `/tinymce/` 资源和显式 GPL；不保留 v4/双 runtime/在线 fallback。
- 程序化加载与真实用户修改分离；未编辑不得发送 `Content`，数据库字节不变；失败不得提前更新缓存或清 dirty。
- 保存成功必须由 `/note/updateNoteOrContent` 的结构化 `info.Re.Ok` 明确确认；不得把服务层失败或部分写入当作成功。
- 迁移三个初始化入口、全部官方插件处置、四个第一方插件、七语言和应用 TinyMCE UI 样式；不以主笔记能启动代替完整验收。

### Task 1: Freeze Runtime, Content And Save Baselines

**Files:**
- Create: `tests/fixtures/editor-html/*.html` and structured expectations
- Create: `tests/js/editor-state-contract.test.js`
- Create: `tests/js/editor-content-compat.test.js`
- Create: `tests/e2e/business/editor-flows.spec.mjs`
- Create: `app/tests/harness/note_save_contract_test.go`
- Modify: `tests/e2e/business/business-flows.spec.mjs`, `tests/e2e/business/ajax-failure.spec.mjs`
- Modify/replace: `tests/js/paste-plugin.test.js`
- Read: `research/tinymce4-runtime-inventory.md`, current note/member/plugin/save sources

- [ ] 把三个初始化入口、官方插件/toolbar、core/theme/skin/lang、四个第一方插件、`.mce-*` 样式与所有动态资源写成机器可核验 baseline；测试先对 4.1.9 通过并在缺项时失败。
- [ ] 建立普通富文本、链接、图片、表格、代码、目录标题、思维导图、混合内容及 script/object/embed/iframe 安全夹具；分别保存原始字节和允许的结构化语义。
- [ ] 对 load epoch、程序化 setContent、真实 mutation、完整 undo、title/tag-only、只读切换、note 切换和保存响应写失败优先状态测试；明确覆盖 A→B 保存、期间编辑 C、确认 B 后撤销到 B 清 dirty、继续撤销到 A 重新 dirty，以及业务/HTTP 失败不确认 revision。
- [ ] 用真实 Revel/MongoDB 回归固定 `/note/updateNoteOrContent` 的 `info.Re` envelope、`notExists`/`noAuth`/更新失败、新建权限/插入/内容写入失败和元数据成功但内容失败的部分写入结果；若现有新建 service helper 只返回 `info.Note`，先补一个 controller 专用的可判定内部结果。所有失败 `Msg` 非空且可见，现有成功 E2E 同步断言 `Ok:true` 与新建 `Item`。
- [ ] 把旧 TinyMCE 4 Clipboard 内部测试替换为行为夹具，固定单次图片粘贴/拖放、可恢复失败与无伪成功。

### Task 2: Make TinyMCE 8 Assets A Manifest-owned Closure

**Files:**
- Modify: `package.json`, `package-lock.json`, `scripts/build/manifest.mjs`, build tests/fixtures
- Modify: `app/views/note/note-dev.html`, generated `app/views/note/note.html`
- Modify: `app/views/member/blog/add_single.html`, `update_abstract.html`
- Replace generated: `public/tinymce/**`

- [ ] 安装并精确锁定 `tinymce@8.8.2`；为 core、Silver、icons/model、Oxide、所需 content skin、官方/第一方插件和七语言声明输入、输出、URL 与 Git tracking。
- [ ] 生成可读开发资源和压缩生产资源，保持 `/tinymce/` 根；更新 note 模板 marker/生成契约与两个 member 入口，显式 `license_key: 'gpl'`。
- [ ] 扩展 manifest/build-smoke，拒绝 TinyMCE 4 标识、第二 core、CDN/cloud/commercial/premium、外部 mind editor、未声明资源、404 与非确定性产物。
- [ ] 连续构建和故障注入证明发布原子性、输出计数及零 diff；不在本步骤手工改生成文件。

### Task 3: Centralize Profiles, Edit-state And Save Response

**Files:**
- Create: shared TinyMCE config/state adapter under the existing `public/js/` ownership boundary
- Modify: `public/js/app/page.js`, `public/js/common.js`, `public/js/app/note.js`
- Modify: `app/controllers/NoteController.go`; reuse `app/info/Re.go` and `app/service/NoteService.go` return contracts
- Modify: member blog templates and applicable app/theme LESS sources
- Regenerate: owned JS/CSS/template outputs through `npm run build`

- [ ] 实现 note/member shared base config 和唯一 locale/plugin/toolbar disposition；迁移 Silver/Oxide、toolbar rename、integrated/removed plugin 配置。
- [ ] 实现 per-note load epoch、persistedContent/editorBaseline/revision/confirmedRevision；所有 content mutation 通过单一入口，程序化 load/Ace/nav 不进入。
- [ ] 改造保存 payload：title/tag-only 省略 `Content`，完整 undo 回到 baseline 不保存；成功时无论有无后续编辑，都把已提交字节设为 persisted/editor 两类 baseline，只确认捕获 revision，并按当前序列化与提交字节重算 dirty。
- [ ] 让 controller 检查本次请求的 `UpdateNote`/`UpdateNoteContent` 返回值并统一返回 `info.Re`；新建 service helper 也必须暴露权限/插入/内容失败，新建成功 note 放入 `Item`，任一失败或部分写入返回 `Ok:false`/非空 `Msg`。直接保存与 `updatePoolNote` 两条 ajax 路径共用同一 `reIsOk` 成功门禁，业务/HTTP 失败均保留 dirty/cache、显示错误且不得触发成功回调。
- [ ] read-only gate 覆盖 keyboard、paste/drop、commands 和 plugins；切换/延迟事件不能跨 note。
- [ ] 逐项迁移 `.mce-*` UI 规则到最小 `.tox-*` 布局，保留内容选择器并删除 v4 theme/skin UI；验证双行 toolbar、写作模式、移动和 member editor。

### Task 4: Migrate First-party Plugins And Paste Boundary

**Files:**
- Modify: `public/tinymce/plugins/leaui_image/**`
- Modify: `public/tinymce/plugins/leaui_mindmap/**`
- Modify: `public/tinymce/plugins/leanote_nav/**`
- Modify: `public/tinymce/plugins/leanote_code/**`
- Modify: `public/js/plugins/editor_drop_paste.js` and related generated bundle
- Delete after proof: `public/tinymce/plugins/leaui_mind/**`, TinyMCE 4 paste/core/plugin/theme/skin files

- [ ] 用 UI Registry、v8 dialog/openUrl、insertContent 和公开 event/command API 迁移四插件；每个动作接入统一 mutation/read-only 边界。
- [ ] 验证 `leaui_image` 上传/选择/更新/拖放及 Bootstrap 5 iframe host 契约；失败可见、状态可重试。
- [ ] 验证本地 `leaui_mindmap` 的 `data-mind-json` 插入/编辑/重载；证明 `leaui_mind` 的外部 dead path 无运行引用后删除，不保留任何 leanote.com fallback。
- [ ] 验证 `leanote_nav` 只写外部 DOM且不 dirty；验证 `leanote_code`/Ace 序列化不携带临时 DOM。
- [ ] 让 TinyMCE 8 core 拥有普通 paste，Leanote 只处理上传/插入边界；删除 v4 Clipboard fork/补丁且保证图片单次插入。

### Task 5: Migrate Seven Locales And Remove Legacy Closure

**Files:**
- Modify: `messages/{de-de,en-us,es-co,fr-fr,pt-pt,zh-cn,zh-hk}/tinymce_editor.conf`
- Modify: i18n generator/build fixtures and generated `public/tinymce/langs/*.js`
- Modify/Create: runtime inventory and reference-proof tests

- [ ] 实现七个应用 locale 到 canonical TinyMCE code/URL 的单一映射，更新 v8 活跃 UI key；实际初始化验证 toolbar/dialog/plugin label。
- [ ] 对 `en.js`、`zh.js`、`readme.md`、全部旧官方插件/core/theme/skin和压缩副本形成 reference/manifest/request 证明；只删除证明不在闭包内的资源。
- [ ] 全仓扫描无旧 toolbar/API/plugin 名、TinyMCE 4 version、外部 editor、CDN 或未登记 fallback；同步 manifest 输出计数和测试夹具。

### Task 6: Full Verification And Release Evidence

**Files:**
- Modify: `tests/e2e/business/editor-flows.spec.mjs`, `playwright.config.mjs` as needed for smoke discovery
- Create: `docs/modernization/browser-smoke/tinymce-8.md`
- Modify: relevant Golden/USN/page-smoke expectations only when observed behavior is contract-approved

- [ ] 运行 focused Node tests、`npm ci && npm run build && npm test`、两次 build 零 diff、`npm run test:e2e:build`、`npm run test:e2e`、Golden/USN 和 `git diff --check`。
- [ ] 在真实服务/MongoDB 中断言未编辑/标题标签-only 的请求无 `Content` 且 DB 字节不变；现有 `business-flows.spec.mjs` 与 harness 断言结构化成功、业务失败和部分写入失败，`ajax-failure.spec.mjs` 断言 HTTP 失败；内容编辑、并发 revision、失败重试、插件、paste/drop 与 reload 全部通过。
- [ ] 检查所有 editor 请求 200，console/pageerror/unhandled rejection、TinyMCE deprecation/license/language 警告均为零；错误路径不伪成功。
- [ ] 在真实 Chrome、Edge、Firefox、Safari 当前/前一主版本记录脱敏 smoke；缺失环境证据明确阻断发布，不以 Chromium/WebKit 代替。
- [ ] 最终 diff 复核除 `/note/updateNoteOrContent` 最小结构化响应外，无 Schema、广泛 API/USN 重构、历史批量迁移、双 runtime、隐藏 fallback、宽泛 sanitizer、付费插件或视觉重设计。

## Rollback Point

Package/manifest、三个初始化入口、状态 adapter、`UpdateNoteOrContent` controller/envelope 与前端解包、四插件、locale、样式和生成资源作为一个迁移单元回退。任何未编辑零写入、存量语义、安全、保存响应、插件、资源或错误门禁失败都阻止合并；回滚不得留下新旧响应混用，不执行数据库变换，也不恢复成可并行选择的 TinyMCE 4 runtime。
