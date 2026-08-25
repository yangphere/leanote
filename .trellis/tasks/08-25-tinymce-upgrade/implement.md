# TinyMCE 8 升级（E-TM）— 执行计划

## Global Constraints

- 目标固定 TinyMCE 8.8.2，自托管并显式使用 GPL 许可。
- 未编辑不保存且持久化字节不变；编辑后只允许已记录的非语义归一化。
- 只保留四个实际使用的第一方插件，不保留 TinyMCE 4 或双运行时。

### Task 1：固定内容与插件回归基线

**Files:**
- Create: `tests/fixtures/editor-html/*.html`
- Create: `tests/js/editor-content-compat.test.js`
- Create: `tests/e2e/editor-flows.spec.js`
- Modify: `tests/js/paste-plugin.test.js`
- Read: `public/js/app/page.js`、`public/js/common.js`、`public/js/app/note.js`、`public/tinymce/plugins/`

- [ ] 收集普通富文本、链接、图片、表格、代码块、目录、思维导图及混合 HTML 夹具，注明允许的非语义归一规则。
- [ ] 为只读路径写失败优先测试：不得产生 dirty/save，数据库读取前后字节相同。
- [ ] 为实际编辑路径写 DOM 语义比较测试，逐项保护文本、链接、图片、代码和插件标记。
- [ ] 扩展粘贴夹具和失败路径，记录 TinyMCE 4 基线行为及确认需要修正的缺陷。
- [ ] 全仓核验 `leaui_mindmap` 与 `leaui_mind` 的引用、注册和资源加载，形成删除证据。

### Task 2：接入锁定的 TinyMCE 8 自托管资源

**Files:**
- Modify: `package.json`、`package-lock.json`、`scripts/build/manifest.mjs`
- Modify: templates and scripts that load TinyMCE
- Replace: `public/tinymce/` generated core/theme/icons/model/skin/language assets

- [ ] 安装并锁定 `tinymce@8.8.2`，manifest 只复制运行必需资源和第一方插件。
- [ ] 更新页面加载路径及初始化入口，显式设置 `license_key: 'gpl'`。
- [ ] 添加构建测试，拒绝 CDN、TinyMCE 4 标识、第二份 core、缺失 skin/language 和未声明资源。
- [ ] 运行最小编辑器启动 smoke，先暴露所有 API、许可和资源错误。

### Task 3：迁移初始化、粘贴与四个第一方插件

**Files:**
- Modify: `public/js/app/page.js`、`public/js/common.js`、`public/js/app/note.js`
- Modify: `public/tinymce/plugins/leaui_image/**`
- Modify: `public/tinymce/plugins/leaui_mindmap/**`
- Modify: `public/tinymce/plugins/leanote_nav/**`
- Modify: `public/tinymce/plugins/leanote_code/**`
- Delete after proof: `public/tinymce/plugins/leaui_mind/**`

- [ ] 迁移事件、命令、selection、dialog、DOM 与序列化调用，只保留 Leanote 语义 adapter。
- [ ] 分别验证图片、思维导图、目录和代码插件的初始化、编辑、保存及重新载入。
- [ ] 将粘贴规则迁移到公开事件和单一清理函数；失败保留可恢复输入并显示错误。
- [ ] 合并 `leaui_mind` 中经证明仍需要的唯一能力后删除该副本；若无独有能力直接删除。
- [ ] 每完成一个插件即运行对应 JS 测试和 Chromium E2E，不以最终整体验收替代局部证据。

### Task 4：验证持久化与跨浏览器行为

**Files:**
- Modify: `tests/e2e/editor-flows.spec.js`
- Modify: editor save/autosave tests under `tests/js/`
- Modify: release smoke documentation selected by the parent task

- [ ] 通过真实服务和 MongoDB 8.0 证明只读路径无保存请求且 DB 字节不变。
- [ ] 执行实际编辑、自动保存和重新加载，使用语义比较器验证允许/禁止变化。
- [ ] Chromium 阻塞覆盖撤销/重做、粘贴、上传、代码、目录、思维导图和错误路径。
- [ ] 记录当前及前一主版本 Chrome/Edge/Firefox/Safari smoke；明确真实 Safari 环境与结果。
- [ ] 检查控制台无异常、deprecation、许可提示或资源 404。

### Task 5：移除旧资源并完成验收

- [ ] 删除 TinyMCE 4 core、废弃桥接、确认无引用的插件和旧构建输入。
- [ ] 运行 `npm run build && npm test`、`npm run test:e2e`、Golden/USN 回归和页面 smoke。
- [ ] 连续运行两次构建并确认 `git diff --exit-code`，同时扫描 production manifest 无旧 core/CDN。
- [ ] 复核 diff 未批量重写历史 HTML、改变 Schema、引入双 runtime 或隐藏 fallback。

## Rollback Point

TinyMCE 8 资源与调用方以一个迁移单元回退。任一存量 HTML、核心插件或粘贴契约失败都阻止合并，不通过保留 TinyMCE 4 运行时绕过。
