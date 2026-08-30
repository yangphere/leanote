# Bootstrap 5.3 升级（E-BS）— PRD

父任务：`.trellis/tasks/08-25-frontend-libs`

## Goal

将仓库内所有由应用拥有、且当前仍被运行时引用的 Bootstrap 3 资源和用法迁移到 npm 锁定的 Bootstrap **5.3.8**，保持服务端渲染页面、已引用公开静态 URL、URL/API、文案、信息架构、博客主题加载和图片流程不变。`public/tinymce/plugins/image/dialog.htm` 及其直接 UI handler `js/dialog.js` 只纳入 Bootstrap 兼容迁移；TinyMCE core、bridge、插件公开 API 和数据协议不升级。该任务只处理 Bootstrap 及其直接依赖的第一方/可审计插件适配，不进行视觉重设计。

## Confirmed Baseline And Dependencies

- `08-25-jquery-upgrade` 已归档完成；本任务是前端轨道当前唯一满足依赖的 ready 叶。完成后才可启动 `08-25-tinymce-upgrade`。
- 代码中的 Bootstrap 基线不是单一 3.2：主站 `public/css/bootstrap.css` 与 `public/js/bootstrap.js` 标注为 3.0.2；`public/tinymce/plugins/leaui_image/public/bootstrap3/{css,js}` 标注为 3.0.3；`public/admin/css/bootstrap.3.2.0.min.css` 标注为 3.2.0。另有对应的 min/theme 重复副本，必须以文件头和 SHA-256 清单为准，不得把它们统称为一个版本。
- 当前引用的 Bootstrap URL 包括 `/css/bootstrap.css`、`/css/bootstrap-min.css`、`/js/bootstrap.js`、`/js/bootstrap-min.js`、`/public/admin/css/bootstrap.3.2.0.min.css`、`public/tinymce/plugins/image/dialog.htm` 的共享 URL，以及 `leaui_image/index.html` 中的相对 `public/bootstrap3/{css,js}`。内置博客主题通过 `BlogController` 的 `bootstrapCssUrl`/`bootstrapJsUrl` 使用前两类主站 URL。
- `scripts/build/manifest.mjs` 是受跟踪生成物的唯一事实来源；当前默认输出为 34 项。Bootstrap 任务必须在同一 manifest 中增加/调整 Bootstrap 输出，并同步其输出计数、输入存在性、Git 跟踪、构建漂移和资源 smoke 契约。

## Requirements

### R-BS1: Single Bootstrap 5.3.8 Asset Contract

- `package.json` 与 `package-lock.json` 精确锁定 `bootstrap` **5.3.8**；构建不得使用 CDN、全局包、未锁定下载或第二个 Bootstrap 版本。
- 以 npm `bootstrap/dist/css/bootstrap.css` 和 `bootstrap/dist/css/bootstrap.min.css` 以及包含 Popper 的 `bootstrap/dist/js/bootstrap.bundle.js`/`.min.js` 作为唯一核心输入。manifest 必须生成所有当前仍被引用的兼容 URL：`public/css/bootstrap.css`、`public/css/bootstrap-min.css`、`public/js/bootstrap.js`、`public/js/bootstrap-min.js`；这些 URL 保持 200，内容来自 5.3.8。
- `dep`、`album` 与其他生成 bundle 直接消费同一 npm Bootstrap 输入，不把生成输出再次作为输入，不加载 Bootstrap 3 或第二份 Bootstrap 核心。
- 只有在全仓库、构建 manifest、博客主题和测试清单均证明无运行时/文档引用后，才删除 `public/css/bootstrap.min.css`、`public/css/bootstrap-theme*.css`、`public/js/bootstrap.min.js` 等历史重复副本；删除列表及证明写入研究清单。

### R-BS2: Complete Surface And Ownership Inventory

- 覆盖 `app/views/home`、`note`、`album`、`admin`、`member`、`user`、`share`、`file/pdf`，生成源 `note-dev.html`（由 build 产生 `note.html`），内置博客主题 `public/blog/themes/{default,elegant,nav_fixed}`，以及 `public/tinymce/plugins/leaui_image/index.html` 和其 `public/` JS/CSS。
- 覆盖第一方调用和动态 HTML：`public/js/common.js`、`public/js/plugins/{tips,history}.js`、`public/js/app/{note,page,blog/*.js,notebook,share}.js`、`public/md/main-v2.js`、`public/album/js/main.js`、admin/member/blog 模板内联脚本。
- 覆盖 `public/js/main.js` 的 RequireJS Bootstrap alias、`public/blog/js/bootstrap-hover-dropdown.js`、`public/js/bootstrap-hover-dropdown.js`、两份 `bootstrap-dialog.min.js`，并在清单中记录版本、来源、加载页面、所有权和迁移/替换决定。min-only 插件不得直接手工改压缩文件；没有 Bootstrap 5 可审计上游时，必须以等价的第一方实现替换并保留调用方行为。
- `public/tinymce/plugins/image/dialog.htm` 及 `js/dialog.js` 仅迁移其 Bootstrap tab/class/API 适配并纳入资源验收；TinyMCE core、`jquery.tinymce` bridge、`leaui_mindmap` 独立应用和插件公开 API 不属于本任务。`leaui_image` 的 Bootstrap UI 属于本任务。用户上传博客主题的字节和路径不修改，只保证主题注入/加载契约；内置三套主题的源码迁移并纳入验收。

### R-BS3: Markup And Data API Migration

- 仅迁移 Bootstrap 所有的 `data-toggle`/`data-target`/`data-dismiss`/`data-slide*`/`data-spy` 等属性为 `data-bs-*`；`data-toggle="fullscreen"`、`data-toggle="class:*"`、`data-target="body"`、`data-hover="dropdown"` 等第一方或 hover 插件属性必须单独登记并保持其既有语义，不能以全局搜索替换误伤。
- `pull-left/right`、`hidden-*`/`visible-*`、`form-group`、`input-group-addon`、`btn-default`、`panel`/`well`、`navbar-inner`、旧 close markup、旧 grid 等逐项映射到 Bootstrap 5 或明确的最小第一方 CSS；不以双框架或隐藏适配器掩盖遗漏。
- `.close`/`&times;` 关闭控件迁移为可访问的 `.btn-close` 与 `data-bs-dismiss`；Glyphicons 不再作为 Bootstrap 资源依赖，图标使用现有 Font Awesome 或明确的文本等价物。
- 保持表单字段名/序列化、DOM id、URL、重定向、文案、博客主题注入和存量 HTML 内容不变；无语义等价的组件必须在实现前回到规划，不得自行改变产品行为。

### R-BS4: JavaScript Component Contracts

- Bootstrap 5 原生实例 API 是唯一组件 API：`Modal`、`Tab`、`Dropdown`、`Tooltip`、`Alert` 等调用必须迁移到 `bootstrap.*.getOrCreateInstance` 或声明的 data API；生产中不得提供 Bootstrap 3 jQuery plugin shim。
- 所有现有 `modal`/`tab`/`dropdown`/`tooltip`/`alert` 事件的显示、隐藏、焦点、键盘 Esc、backdrop、一次性触发和销毁语义保持不变；重复初始化不得重复事件。
- 旧 `.button("loading")`/`reset` 不属于 Bootstrap 5 API。为 `common.js`、note/album/blog/admin/member 的每个调用点定义同一 loading 合约：保存原始文本/属性，设置 disabled 和可观察 loading 状态，在成功、业务失败、HTTP 失败及异常路径均恢复；不吞错。
- `showDialogRemote` 不得继续使用已删除的 `remote` modal option。必须对原 URL 和参数进行正确编码的 fetch/AJAX，成功后注入既有 modal 容器并打开，网络/非 2xx/内容错误时显示现有可见错误并清理 loading/backdrop；不得改后端接口。

### R-BS5: leaui_image And Data Boundary

- `public/tinymce/plugins/leaui_image/index.html` 的内部 tab、alert、form、progress、close、布局和图标迁移到 Bootstrap 5；删除其 `public/bootstrap3` CSS/JS/fonts 副本，使用共享 Bootstrap/jQuery 公共 URL。
- 保持 `fileupload`、相册列表/分页、真实上传回调、URL 图片、图片尺寸/标题/约束、`top.LEAUI_DATAS`、`parent.GlobalConfigs`、`parent.getMsg`、`mdGetImgSrc` 及跨 iframe 插入语义；不得因升级引入第二个 jQuery/Bootstrap 核心或修改 TinyMCE core。

### R-BS6: Blog And Third-Party Compatibility

- 内置博客主题的 navbar/collapse/dropdown、评论、登录提示、举报、二维码和分享流程在 Bootstrap 5 下保持可用；`BootstrapDialog` 的现有 `show`、`confirm`、`getModalBody`、`close` 行为必须有可观察回归。
- `bootstrap-hover-dropdown` 必须保留桌面 hover 延迟、触摸设备不误触、键盘/点击关闭和子菜单行为；若继续保留文件，必须明确其 Bootstrap 5 原生 API 实现和唯一 URL。无引用的重复副本可删除并记录证据。
- 用户上传主题不被自动重写或注入 Bootstrap 3 adapter；其加载失败、资源 404 和主题路径变化均为验收失败。未知自定义 class 不纳入静态迁移清单。

### R-BS7: Browser, Build And Backend Compatibility

- 复用 D/E 的 Node 24、Playwright 1.62.1、`build-smoke`/`business` project、test-mode harness、身份预检、脱敏报告与零 diff 门禁；不得创建第二套 Playwright 配置或锁文件。
- PR/push Chromium E2E 至少覆盖 login/register、note（富文本和 markdown）、modal/tab/dropdown/alert/tooltip、表单 loading/error、album 真实上传、admin/member、博客内置主题/评论/分享和 `leaui_image` iframe；发布前真实 Chrome、Edge、Firefox、Safari 当前及前一主版本 smoke 复用共享记录契约。
- 必须同步收紧现有 `tests/e2e/business/jquery-diagnostics.spec.mjs` 的 E-BS URL 豁免：旧 `/js/bootstrap.js`、`public/bootstrap3` 和 Bootstrap 3 签名请求必须进入失败路径，不能作为允许的第三方告警。
- 运行 `npm ci && npm run build && npm test`、两次连续 build 的零 diff、资源 200/无 404、无 console error/pageerror/unhandled rejection；回放 Golden、USN、页面 smoke。不得改后端 API、HTTP 语义、数据库、认证或 TinyMCE core。

## Acceptance Criteria

- [ ] **AC-BS1** `npm ls bootstrap` 只有 5.3.8；manifest 从 npm 5.3.8 生成 `/css/bootstrap.css`、`/css/bootstrap-min.css`、`/js/bootstrap.js`、`/js/bootstrap-min.js`，`dep`/`album` 不含 Bootstrap 3 或第二核心；构建输出计数、Git 跟踪和输入声明一致。
- [ ] **AC-BS2** 受跟踪 `research/bootstrap3-usage.md` 逐文件覆盖所有旧核心副本（含 SHA-256、版本头和字体）、直接 URL、页面/iframe、内联 API、博客插件、内置主题、RequireJS alias、`image/dialog.htm` 和允许保留的第一方 `data-*` 属性；每项有 owner、来源、运行时/非运行时判定、替换方式和回归用例。
- [ ] **AC-BS3** Bootstrap 归属的旧 `data-*`、class 和 `.modal/.tab/.dropdown/.tooltip/.alert/.button` 调用（含 `public/tinymce/plugins/image/dialog.htm`）全部迁移；仅清单中登记的 `fullscreen`/`class:*`/`data-target="body"`/`data-hover` 等非 Bootstrap 属性可保留。模板源码、生成 `note.html` 与运行时 DOM 均无未登记旧用法。
- [ ] **AC-BS4** modal/tab/dropdown/tooltip/alert 的显示隐藏、事件一次性、焦点/键盘/backdrop 和销毁行为通过 Chromium E2E；所有 button loading/reset 调用在成功、业务失败、HTTP 失败和异常路径恢复；远程 modal 成功和非 2xx/网络失败均可观察且不遗留遮罩。
- [ ] **AC-BS5** `leaui_image/index.html` 及其资源不再请求 `public/bootstrap3`，真实上传/选择/分页/图片属性/跨 iframe 插入通过；`top.LEAUI_DATAS`、`GlobalConfigs`、`getMsg` 和 `mdGetImgSrc` 契约保持。
- [ ] **AC-BS6** 三套内置博客主题和 BootstrapDialog/hover-dropdown 的登录提示、评论删除/举报、二维码、navbar/collapse/dropdown/share 通过；用户上传主题字节和路径未改，页面无主题资源 404。
- [ ] **AC-BS7** `npm ci && npm run build && npm test`、D 的 build smoke、E 的 business E2E、Golden/USN/page smoke 通过。开发工作树中第一次 build 后保存声明输出的哈希/二进制快照，第二次 build 只与该快照比较且不得变化；在干净 CI checkout 中另行执行 `git diff --exit-code` 与 `git status --porcelain --untracked-files=all`，两者均为零。控制台/网络无阻断错误。
- [ ] **AC-BS8** 真实 Chrome、Edge、Firefox、Safari 当前及前一主版本的脱敏 smoke 记录写入受跟踪的 `docs/modernization/browser-smoke/bootstrap-5.3.md`，包含提交 SHA、日期、产品/完整版本、操作系统、覆盖页面/iframe、身份预检、错误门禁和结果；缺失版本、认证/错误门禁失败或任一产品失败均阻断验收。最终 diff 不含 TinyMCE core、后端兼容分支、永久 Bootstrap 3 adapter、双框架加载或视觉重设计。

## Out Of Scope

- Bootstrap 6、jQuery 4、TinyMCE core/bridge、`leaui_mindmap`、SPA/ESM 重构、后端/API/数据库/认证改动、历史笔记 HTML 重写。
- 用户上传博客主题内容改写；只保证加载 URL/注入契约和不引入 Bootstrap 资源 404。
- 新设计系统、页面信息架构改变、无验收依据的视觉重设计，以及用永久 Bootstrap 3 adapter、双框架或静默 fallback 掩盖不兼容。

## Open Questions

无产品范围阻塞项。`bootstrap-dialog.min.js` 仅有压缩运行时文件，实施前必须在研究材料中给出 Bootstrap 5 可审计上游来源，或选择行为等价的第一方替换；两者都不可证实时任务退回规划，不得以未审计补丁进入实现。
