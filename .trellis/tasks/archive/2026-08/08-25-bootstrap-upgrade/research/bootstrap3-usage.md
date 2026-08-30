# Bootstrap 3 使用与迁移清单

审计日期：2026-08-30。该清单是 Bootstrap 5.3.8 实施和验收的事实入口；搜索结果本身不等于行为验收，动态调用必须通过运行时测试确认。

## 核心资源逐文件指纹与处置

以下指纹是在 2026-08-30 对当前工作树逐文件计算的 SHA-256。无版本头的压缩副本不以文件名推断版本，实施时必须以指纹和引用证据为准。

表中“owner / 当前证据”同时记录来源与所有权；列出活动页面或 iframe 的条目视为 runtime，写明“目录存在，运行时引用待证明”的条目视为 non-runtime candidate，删除前必须完成引用证明。每一行的“回归”列是该项必须落到的验收证据。

| 资源 | 版本/文件头 | SHA-256 | owner / 当前证据 | 处置 | 回归 |
|---|---|---|---|---|---|
| `public/css/bootstrap.css` | Bootstrap 3.0.2 | `7b5a8202e9fad05f9aef35e93344053c5917b731af86cf0bf5f48278f88aab36` | home/admin/pdf、博客、TinyMCE image dialog | 由 npm 5.3.8 生成 `/css/bootstrap.css` | AC-BS1/3/6 |
| `public/css/bootstrap-min.css` | 无版本头；3.0.2 压缩变体 | `69f2a716d88c20afc20800e0842d6aaa726f1ad76d5d895832d49154b3e60735` | note、album | 由 npm 5.3.8 生成 `/css/bootstrap-min.css` | AC-BS1/3 |
| `public/css/bootstrap.min.css` | 无版本头；历史重复副本 | `2b6e456a929be044f4509cee63019530d2a9578482498ee684453b137fdb6cd2` | 目录存在，运行时引用待证明 | 无引用证明后删除 | AC-BS2/3 |
| `public/css/bootstrap-theme.css` | Bootstrap 3.0.x theme | `8ae677a4ab009b2b570be4d872554a27d069cec4faca496be23ee37b247658a0` | 目录存在，运行时引用待证明 | 无引用证明后删除 | AC-BS2/3 |
| `public/css/bootstrap-theme.min.css` | Bootstrap 3.0.x theme 压缩副本 | `6eb248394cbe8e0fef9a14a32928dc99a1902781d2363fd714dbc3437eb80d40` | 目录存在，运行时引用待证明 | 无引用证明后删除 | AC-BS2/3 |
| `public/js/bootstrap.js` | Bootstrap 3.0.2 | `5b7cce68c88301327b974be0bd2a147c2561695b7a57a230d92c0cd1ca4cd4f7` | home、admin/member、TinyMCE image dialog | 由 npm bundle 生成 `/js/bootstrap.js` | AC-BS1/3/4/5 |
| `public/js/bootstrap-min.js` | 无版本头；3.0.2 压缩变体 | `73293fc9ab6052f322a45d46921988edd1459be574f31be8af6d34776a0b99c6` | note、album、博客、RequireJS alias、dep/album 输入 | 由 npm bundle 生成 `/js/bootstrap-min.js`；dep/album 直接用 npm 输入 | AC-BS1/3/4/6 |
| `public/js/bootstrap.min.js` | 无版本头；Bootstrap 3.0.2 压缩副本 | `e76c76a35589d5617d58c02be0d9bff127ba1fce76c71f6c17e38c9e6ddedda9` | 目录存在，运行时引用待证明 | 无引用证明后删除 | AC-BS2/3 |
| `public/admin/css/bootstrap.3.2.0.min.css` | Bootstrap 3.2.0 | `326ffedb17cf069bdc342759a21bf78461179b48fe9047d0e4636e3c6115ad9d` | `app/views/admin/top.html`、`member/top.html` | 改用 `/css/bootstrap.css` 后删除 | AC-BS1/2/3 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/css/bootstrap.css` | Bootstrap 3.0.3 | `f8d88307123a036c02d6fe3d3702a468cce7da4880ec463e881d6c46c28b0229` | `leaui_image/index.html` 相对 URL | 删除；iframe 用共享 CSS | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/css/bootstrap.min.css` | Bootstrap 3.0.3 | `81e40cfd9268d77c245692bfe869d56836f557c91b494785b0cf068e875b9892` | iframe 历史副本 | 删除 | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/css/bootstrap-theme.css` | Bootstrap 3.0.3 | `8d137115b6b16bf3f7d88d959b01c3fc963d608074d50793f7362e110b3fe70c` | iframe 历史副本 | 删除 | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/css/bootstrap-theme.min.css` | Bootstrap 3.0.3 | `8c2ce94d9e23ed70b5eea5de66eb3e1875a80213d728eb51c40263b6ff9cc338` | iframe 历史副本 | 删除 | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/js/bootstrap.js` | Bootstrap 3.0.3 | `35b0887d34c681aebbeef4ed06c05839766c1118d89808b2934e3d1bc5c68438` | iframe 历史副本 | 删除；iframe 用共享 JS | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/js/bootstrap.min.js` | Bootstrap 3.0.3 | `46ed2dfb732a01dbc80515ce6a48bcb24dea4bcab8522c71868231812000b58d` | iframe 当前相对脚本 | 删除；iframe 用 `/js/bootstrap-min.js` | AC-BS5 |

### iframe 字体副本

这些字体只服务于被删除的 Glyphicons CSS，不属于新 Bootstrap 输入；删除必须与 iframe CSS/JS 同批完成，并由 AC-BS5 的资源 404/无第二核心检查覆盖。

| 资源 | SHA-256 | owner / 来源 | 运行时处置 | 回归 |
|---|---|---|---|---|
| `public/tinymce/plugins/leaui_image/public/bootstrap3/fonts/glyphicons-halflings-regular.eot` | `62fcbc4796f99217282f30c654764f572d9bfd9df7de9ce1e37922fa3caf8124` | leaui_image Bootstrap 3 theme 字体 | 随 `public/bootstrap3/` 删除 | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/fonts/glyphicons-halflings-regular.svg` | `0b12aba182a43da34d79361168a665a0ec78a6a79169a730ca96b7cf696cd7c4` | leaui_image Bootstrap 3 theme 字体 | 随 `public/bootstrap3/` 删除 | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/fonts/glyphicons-halflings-regular.ttf` | `e27b969ef04fed3b39000b7b977e602d6e6a2b1c8c0d618bebf6dd875243ea3c` | leaui_image Bootstrap 3 theme 字体 | 随 `public/bootstrap3/` 删除 | AC-BS5 |
| `public/tinymce/plugins/leaui_image/public/bootstrap3/fonts/glyphicons-halflings-regular.woff` | `63faf0af44a428f182686f0d924bb30e369a9549630c7b98a969394f58431067` | leaui_image Bootstrap 3 theme 字体 | 随 `public/bootstrap3/` 删除 | AC-BS5 |

## 运行时 URL 与页面

| URL/入口 | 当前引用位置 | 迁移约束 |
|---|---|---|
| `/css/bootstrap.css` | `app/views/home/header.html:12`、`home/header_box.html`、`admin/header.html:12`、`file/pdf.html:8`、`BlogController.go:183`、`public/tinymce/plugins/image/dialog.htm:6` | 继续 200；内容为 npm 5.3.8 |
| `/css/bootstrap-min.css` | `app/views/note/note-dev.html:13`、`album/index.html:9` | 继续 200；manifest 直接生成 |
| `/js/bootstrap.js` | home login/register/index/find-password 页面、admin/member footer、`public/tinymce/plugins/image/dialog.htm:120` | 继续 200；由 bundle 非压缩输入生成 |
| `/js/bootstrap-min.js` | note-dev/note、album、BlogController.go:184、RequireJS alias `public/js/main.js:17` | 继续 200；由 bundle min 输入生成 |
| `/public/admin/css/bootstrap.3.2.0.min.css` | `app/views/admin/top.html`、`member/top.html` | 不再请求，改到 canonical CSS |
| `public/bootstrap3/{css,js}` | `public/tinymce/plugins/leaui_image/index.html:10,170` | 不再请求，使用 `/css/bootstrap.css` 与 `/js/bootstrap-min.js` |

`public/tinymce/plugins/image/dialog.htm` 及其直接 `js/dialog.js` handler 是 TinyMCE 图片对话框的直接 Bootstrap 消费者：本任务迁移 tab 标记和 handler 的 Bootstrap 5 `Tab` API（例如 `data-toggle` 到 `data-bs-toggle`），同时保持 TinyMCE core、bridge、插件公开 API 和数据协议不变。它们不能从资产/运行时 URL 盘点中排除。

`note.html` 是由 `note-dev.html` 生成的运行时模板，不能手工独立迁移。内置博客三套主题从 `BlogController` 注入 Bootstrap URL；用户上传主题内容不改写。

## Bootstrap API 与动态 HTML

| Owner | 证据/调用 | 必须保持的行为 |
|---|---|---|
| `public/js/common.js` | `:565-618` dialog、remote modal；`:798-805` button loading/reset | modal 打开/隐藏、远程内容加载及错误、loading 恢复 |
| `public/js/plugins/tips.js`, `history.js` | `$tpl.modal({show:true})`、`$tpl.modal('hide')` | tips/history modal、焦点和销毁 |
| `public/md/main-v2.js` | `:16969`, `:16983` | markdown 链接/图片 dialog 打开 |
| `public/album/js/main.js` | `:511`, `:518` tab API；动态 alert close | album tab、上传错误和提示 |
| admin/member/blog 内联脚本 | 多处 `$(t).button('loading'/'reset')` | 成功、业务失败、HTTP 失败均恢复按钮 |
| `app/views/user/account.html:165` | `.tab('show')` | 账户设置 tab 初始定位 |
| `public/js/app/blog/*.js`, `public/blog/js/share_comment.js` | BootstrapDialog `show/confirm/getModalBody/close` | 登录提示、评论删除、举报和二维码 dialog |

### 插件指纹与所有权

| 资源 | SHA-256 | 当前 owner / 来源 | 处置与回归 |
|---|---|---|---|
| `public/js/bootstrap-hover-dropdown.js` | `65524441340daf2b1c8b52ee49834c568ee2f98e6dd56a64afceb4bfb0bd9362` | 第一方可读源码；内置主题 footer 加载 `/js/bootstrap-hover-dropdown.js` | 保留唯一 canonical 文件并改用 Bootstrap 5 `Dropdown`；AC-BS6 |
| `public/blog/js/bootstrap-hover-dropdown.js` | `65524441340daf2b1c8b52ee49834c568ee2f98e6dd56a64afceb4bfb0bd9362` | 与上行字节相同的重复副本 | 无直接引用证明后删除；AC-BS2/6 |
| `public/js/bootstrap-dialog.min.js` | `0a1ceb7bb59da119eee02bca5adbfe4056edeabc54719cf49a4d6f5d3f8d1fd3` | min-only；app/blog 调用全局 `BootstrapDialog` | 删除旧副本；由受跟踪的第一方 `public/js/bootstrap-dialog-source.js` 生成 `/js/bootstrap-dialog.js`，按原公开 API 等价替换；AC-BS6 |
| `public/blog/js/bootstrap-dialog.min.js` | `eebf5d27db1d63707f23a76118e8583de942bb190d9c4dc8f2a6517b9a9886f0` | min-only；内置主题 post 模板直接加载 | 删除重复副本；三套主题改加载 `/js/bootstrap-dialog.js`，来源与生成入口同上；AC-BS6 |

Bootstrap 5 不再提供 jQuery plugin 或 modal `remote` option；实现必须使用原生实例 API 和显式请求，不得添加 Bootstrap 3 shim。

## Markup 与属性分类

Bootstrap 归属、必须迁移：`data-toggle="collapse|dropdown|tab|pill|modal|button"`、对应 `data-target`/`data-dismiss`，以及 `pull-left/right`、`hidden-*`/`visible-*`、`form-group`、`input-group-addon`、`btn-default`、`panel`/`well`、`navbar-inner`、`.close`/Glyphicons。

非 Bootstrap、必须登记后保留：member/admin 的 `data-toggle="fullscreen"` 和 `data-toggle="class:*"`，member 的进度示例选择器 `[data-toggle^="progress"]`（仅由 `member.js` 自定义脚本消费，不是 Bootstrap button API），home 的 `data-target="body"`，博客 hover 插件的 `data-hover="dropdown"`。验收不能使用“仓库没有任何 `data-toggle`”这一过宽条件。

## 插件与主题所有权

- `public/js/bootstrap-hover-dropdown.js` 与 `public/blog/js/bootstrap-hover-dropdown.js` 内容相同；内置主题通过 `/js/bootstrap-hover-dropdown.js` 加载。保留唯一 canonical URL，并以 Bootstrap 5 `Dropdown` 实例实现 hover/触摸/键盘/子菜单行为；无引用副本可删除。
- `public/js/bootstrap-dialog.min.js` 与 `public/blog/js/bootstrap-dialog.min.js` 是不同字节的 min-only 副本，内置博客和 `public/js/app/blog` 使用全局 `BootstrapDialog`。必须记录上游版本/provenance，或以第一方等价实现替换；不得直接 patch minified bytes 或保留依赖 `$.fn.modal` 的旧实现。
- 内置主题 `public/blog/themes/{default,elegant,nav_fixed}` 均含 navbar/collapse/dropdown 和评论分享标记，全部纳入 E2E。用户上传主题只验证路径、注入和资源加载，不改存量文件。

## leaui_image 数据边界

实际 HTML 入口是 `public/tinymce/plugins/leaui_image/index.html`，不是 `.../public/index.html`。必须保持 `top.LEAUI_DATAS`、`parent.GlobalConfigs`、`parent.getMsg`、`mdGetImgSrc`、fileupload、分页、URL 图片、图片属性及跨 iframe 插入；TinyMCE core、bridge 与 mindmap 不归本任务。

### 非运行时配置记录

`public/admin/config.codekit` 和 `public/css/config.codekit` 仍包含旧 CSS 输入名。它们是历史 CodeKit 工具配置，不是 manifest 或运行时加载入口；owner 是历史构建工具，来源是 CodeKit 配置文件，运行时状态为 non-runtime。实施时必须在静态扫描中单独标注为 non-runtime，不能把它们当作 Bootstrap 3 已删除的运行时引用，也不能让它们绕过删除证明；回归为 AC-BS2 的 non-runtime 分类检查。

`public/md/main.js` 仍保留旧 Markdown 编辑器实现中的 jQuery `modal`/`tooltip` 调用和 Bootstrap 3 关闭标记。当前 manifest、note 模板和其他应用入口只加载 `public/md/main-v2.min.js`，全仓库源码未发现 `/md/main.js` 或 `public/md/main.js` 的运行时引用；该文件因此标记为 non-runtime historical candidate，不纳入本次运行时迁移。若未来重新启用该入口，必须先单独规划并迁移其 Bootstrap 组件 API，不能把本清单的 non-runtime 分类当作永久兼容保证；回归为 AC-BS2/3 的引用证明和旧用法分类检查。

## 验收映射

1. 静态：`npm ls bootstrap` 精确为 5.3.8；manifest 输出/输入/URL/Git 跟踪唯一；旧核心和未登记旧 API 无残留。
2. 交互：Chromium business 覆盖页面组件、按钮 loading/error、remote modal、内置博客、真实 album 上传和 leaui iframe；所有错误可见且无残留 backdrop/事件。
3. 构建：`npm ci && npm run build && npm test`、开发工作树两次 build 的输出快照相同、干净 CI checkout 的 `git diff --exit-code` 与未跟踪检查均为零、资源无 404，Golden/USN/page smoke 通过。
4. 发布：真实 Chrome、Edge、Firefox、Safari 当前及前一主版本按共享脱敏记录契约完成；任何缺失或失败阻断验收。
