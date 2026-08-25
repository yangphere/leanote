# 前端构建链现代化（D）— 执行计划

## Global Constraints

- Node 24.x LTS；`npm ci`；提交 lockfile。
- 不升级 jQuery、Bootstrap、TinyMCE，不把业务源码改为 ESM。
- 生成资源继续入库，失败时禁止复用旧输出伪造成功。

### Task 1：建立产物 manifest 与失败测试

**Files:**
- Create: `scripts/build/manifest.mjs`
- Create: `tests/js/build-pipeline.test.js`
- Read: `Gulpfile.js:22-409`

- [ ] 把 `concatDepJs`、`concatAppJs`、`plugins`、`concatMarkdownJsV2`、`concatAlbumJs`、`concatCss`、`minifycss`、`i18n`、`devToProHtml` 的输入顺序和输出逐项写入 manifest。
- [ ] 写测试断言所有 manifest 输入存在、输出路径唯一、关键历史产物一个不少。
- [ ] 运行 `node --test tests/js/build-pipeline.test.js`，确认因构建模块尚未实现而失败。

### Task 2：实现 JS/CSS 构建

**Files:**
- Create: `scripts/build/js.mjs`、`css.mjs`、`index.mjs`
- Modify: `package.json`
- Create: `package-lock.json`

- [ ] 安装并锁定 esbuild 0.28.2，设置 `engines.node` 为 24.x 支持范围。
- [ ] 实现按 manifest 顺序拼接与压缩；缺失输入、重复输出和写入失败均非零退出。
- [ ] 先生成到临时目录，与当前产物逐项比较文件清单和全局入口，再切换到正式输出。
- [ ] 运行现有 `npm test`，确认粘贴测试仍通过。

### Task 3：实现 i18n 与 note HTML

**Files:**
- Create: `scripts/build/i18n.mjs`、`note-html.mjs`
- Modify/Generate: `public/js/i18n/*.js`、TinyMCE lang、`app/views/note/note.html`
- Source: `app/views/note/note-dev.html`、`messages/*/*.conf`

- [ ] 写 i18n fixture 测试，覆盖静态 key、重复 key、缺失翻译和动态表达式失败。
- [ ] 实现稳定排序的语言输出，验证现有 locale/key 集合未减少。
- [ ] 写 note HTML 快照测试并实现 dev block、bundle、TinyMCE 和插件路径替换。
- [ ] 在 Windows 与 CI Linux 换行策略下验证输出一致。

### Task 4：切换唯一构建入口

**Files:** `package.json`、`Gulpfile.js`、旧 gulp dependency entries、`CLAUDE.md`

- [ ] 让 `npm run build` 生成 manifest 全部产物。
- [ ] 删除 Gulpfile 与 gulp 依赖，文档命令改为 Node 24 + `npm ci` + `npm run build`。
- [ ] 重新生成并提交所有受跟踪产物，确认没有手工编辑生成文件。

### Task 5：幂等、页面与契约验证

- [ ] 从干净依赖状态运行 `npm ci && npm run build && npm test`。
- [ ] 连续运行两次构建，第二次 `git diff --exit-code` 为零。
- [ ] 启动真实服务验证 note/markdown/album/blog/admin/member，无静态 404 和控制台错误。
- [ ] 回放 G 的 Golden 与页面 smoke。
- [ ] 运行 `git diff --check`，确认没有运行时库升级或业务源码模块化改写。

## Rollback Point

切换前保留 Gulp 产物清单作为对照；完成时删除旧链。若任一产物无法由新链可靠生成，停止并补齐 manifest/脚本，不允许继续手工同步。
