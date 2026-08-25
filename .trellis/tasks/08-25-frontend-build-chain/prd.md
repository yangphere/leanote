# 前端构建链现代化（D）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

用 Node 24 LTS、esbuild 和普通 Node 脚本替换不可安装的 Gulp 3 流水线，确定性生成 Leanote 现有生产资源，使源码成为唯一事实来源，同时保留生成资源入库和直接检出可运行的交付方式。

## Dependencies

- 依赖 `08-25-regression-baseline`；完成后可与后端链并行。
- 是 `frontend-libs` 及其三个升级子任务的硬前置。

## Requirements

### R-D1 工具与锁定

- 构建环境固定 Node 24.x LTS，提交 `package-lock.json`，CI 使用 `npm ci`。
- 采用 esbuild 0.28.2 基线与 `node:fs/path` 脚本；不引入 Vite、Webpack、前端框架或 ESM 化业务源码。
- 删除 Gulp 3、`gulp-util` 等旧构建依赖和不再使用的 `Gulpfile.js`。

### R-D2 产物覆盖

等价生成并保持现有加载路径：

- `public/js/dep.min.js`、`app.min.js`、`markdown-v2.min.js`。
- `public/js/plugins/main.min.js`。
- `public/album/js/main.min.js`、`main.all.js` 与相应 CSS。
- `public/md/themes/all.css`、主 CSS、Font Awesome、zTree、contextmenu、markdown/theme 压缩资源。
- `getMsg('key')` 抓取产生的 `public/js/i18n/*.js` 与 TinyMCE 语言资源。
- `app/views/note/note-dev.html` → `note.html` 的开发块删除和生产 script 替换。

### R-D3 生成资源契约

- 源输入、顺序和输出清单由一个机器可读 manifest 管理，不在多个脚本重复。
- 生成过程不得写时间戳、绝对路径或随机值；连续两次构建必须得到零 diff。
- 生成资源继续提交 Git，但禁止手工修改；CI 执行 `npm run build` 后必须 `git diff --exit-code`。

### R-D4 行为兼容

- 本任务不升级 jQuery、Bootstrap、TinyMCE 或其他运行时库，只替换构建机制。
- `note.html`、bundle 加载顺序、全局变量、RequireJS 模块名、i18n 内容与页面行为保持兼容。
- 新构建失败必须非零退出并指出输入/输出，不保留旧产物冒充成功。

## Acceptance Criteria

- [ ] 干净环境 Node 24 下 `npm ci && npm run build` 成功。
- [ ] `npm run build && npm run build` 后第二次 `git diff --exit-code` 为零。
- [ ] manifest 列出的每个现有生产产物均生成，缺少任一输入或输出时构建失败。
- [ ] 删除 Gulp 依赖与 `Gulpfile.js` 后源码搜索无构建入口引用。
- [ ] `npm test` 通过；新增构建测试覆盖输入顺序、i18n、note HTML 和漂移检查。
- [ ] `/note`、markdown、相册、博客、admin、member 页面加载无 404/JS error。
- [ ] G 的 Golden 与页面 smoke 通过，证明只换构建机制未改变服务行为。
- [ ] 修改一个源码资源后只运行 `npm run build` 即可更新所有对应生产副本。

## Out of Scope

- 升级或重写任何前端运行时库。
- 把历史全局脚本改成 ESM、SPA 或组件框架。
- 把生成资源移出 Git。
- UI、CSS 视觉重设计。
