# jQuery 3.7 兼容性清单（E-jQ）

任务：`.trellis/tasks/08-25-jquery-upgrade`。本清单是 R-jQ2 要求的逐项审计证据：每行记录位置、所有者、风险、预期行为、处置与回归方式。搜索命中不是完成证据；第一方各项在适配后需有运行时诊断（零 `JQMIGRATE:`）或针对性测试结论。

状态图例：`待适配`（本任务处置）、`E-TM 所有`（TinyMCE 升级任务处置，本任务不得修改）、`无需改动`（已验证兼容）。

## 1. jQuery 核心与公开 URL

| 资产 | 位置 | 现状 | 所有者 | 处置 |
|---|---|---|---|---|
| `public/js/jquery-1.9.0.min.js` | 93,034 B，jQuery v1.9.0 | 全站唯一核心；URL 是公开契约不可删 | 第一方 | manifest 新增 `jquery-runtime` 输出，由 `node_modules/jquery/dist/jquery.min.js`（3.7.1）生成到同一路径（R-jQ1） |
| `public/tinymce/plugins/leaui_image/public/js/jquery.js` | 92,633 B，jQuery v1.9.1 | 仅 `leaui_image/index.html:167` 引用 | 第一方 | 删除；iframe 改用根路径公开 URL（R-jQ1） |
| `dep` bundle 输入 | manifest 第 8 行直接输入 1.9.0 文件 | note.html 生产加载 | 第一方 | 改为同一 npm 输入（R-jQ1） |
| `album` bundle 输入 | manifest 第 29 行同上 | album/index 与编辑器内嵌 album iframe 加载 | 第一方 | 同上 |
| `public/tinymce/tinymce.jquery*.js`、`jquery.tinymce*.js` | TinyMCE 4 内核/适配器 | 不被 note 页直接执行（pro 用 tinymce.full.min.js） | E-TM 所有 | 仅登记；本任务不修改、不计入"生产无 1.9 核心"结论 |
| `leaui_mindmap/mindmap/main.js` | 脑图子应用 | 独立文档内运行 | E-TM 所有 | 同上 |

## 2. 引用 `/js/jquery-1.9.0.min.js` 的模板（URL 保持不变，仅内容升级）

`app/views/note/note-dev.html:894`（dev 块；生产经 `dep.min.js`）、`home/index.html:72`、`home/login.html:71`、`home/register.html:50`、`home/find_password.html:34`、`find_password2.html:43`、`find_password2_timeout.html:28`、`admin/footer.html:9`、`member/footer.html:12`、`file/pdf.html:91`、`album/index.html:157`（注释 dev 块，实载 `main.all.js:165`）、`BlogController.go:171`（`jQueryUrl` 变量 → 三个 repo 主题 footer.html:56）。`public/js/main.js:4` 为注释掉的 RequireJS path。处置：全部 `无需改动`（URL 与脚本顺序保持，内容由 manifest 替换）；回归 = AC-jQ3 的单核心断言 + D 资源 smoke。

## 3. 第一方代码高风险调用

### 3.1 jQuery 3 已移除（升级即坏）

| 位置 | 调用 | 行为 | 处置与回归 |
|---|---|---|---|
| `public/js/app/page.js:632,639` | `navs.size()` / `target.size()` | 导航省略号计算抛 TypeError | 改 `.length`；note 页导航 smoke |
| `public/js/app/blog/view.js:8,15` | 同上（复制代码） | 博客视图导航 | 改 `.length`；博客 E2E |
| `public/blog/js/common.js:219,226` | 同上（复制代码） | 博客前台 | 改 `.length`；博客 E2E |

### 3.2 语义变化（attr/prop、:visible）

| 位置 | 调用 | 风险 | 处置与回归 |
|---|---|---|---|
| `public/js/app/page.js:726` | `.attr("checked", true)` | 3.x 设 attribute 不设 property，radio 预选失效 | 改 `.prop()`；主题选择 smoke |
| `public/album/js/main.js:588-591,675-678` | `.attr("disabled",…)` 混用 `.prop("checked")` | 布尔状态读回为字符串 | 统一 `.prop()`；album E2E |
| `page.js:693`、`share.js:138`、`app/blog/view.js:74,324`、`blog/js/common.js:285`、`blog/share_comment.js:245`、`album/js/main.js:90,209` | `.is(":hidden"/":visible")` | 3.x 布局盒（含 0×0）视为 visible | 逐处核对是否依赖"隐藏=无盒"；对话框/树显隐 E2E |

### 3.3 弃用但未移除（诊断警告项，需清零）

`.bind/.unbind`：`page.js:185,195,204,215,749`、`editor_drop_paste.js:242`、`attachment_upload.js:22`、`member/avatar.js:20`、`member/import_theme.js:19`、`album/main.js:825`、`leaui_image/public/js/main.js:822`（`$(document).bind('dragover')`）→ 全部改 `.on/.off`；诊断 E2E 断言零警告。`.hover(fn,fn)`（page.js:127、share.js:127、notebook.js:270）3.7 支持，`无需改动`。

### 3.4 AJAX 失败语义（R-jQ3 契约修复）

`public/js/common.js`：`_ajax`（L230-251）的 `success` 与 `error` 回调都路由到 `_ajaxCallback`（L209-229）；HTTP 4xx/5xx 时 jqXHR 对象被 `typeof=="object"` 判为成功 → **failureFunc 永不触发**。`ajaxPostJson` 另有两处缺陷：`datatype:` 拼写错误（L295，响应不自动解析）与 `async` 反转（L234-238、L286-290，显式传 `true` 也变同步）。处置：`_ajax` 的 error 分支改传失败标记进 `_ajaxCallback` 保证 failureFunc/可见提示触发；修正拼写与 async 判定；第一方业务代码无绕过 wrapper 的直接 `$.get/$.post`（已扫描确认），回归 = AC-jQ7 受控 4xx/5xx/解析失败注入。

## 4. 第三方插件

| 插件 | 版本 | 位置 | 加载方 | 风险 | 处置 |
|---|---|---|---|---|---|
| blueimp fileupload | 5.26 | `plugins/libs/jquery.fileupload.js`（并入 `libs-min/fileupload.js`、`main.min.js`、`album main.all.js`） | member/blog 主题、avatar、album | `.pipe()`（3.0 移除，L800,802,938,1007）、`$.parseJSON` | 升级到兼容 3.7 的上游版本（≥9.x），记录来源/许可证；压缩件不得手补丁 |
| jquery.ui.widget | 1.10.1+amd | `plugins/libs/jquery.ui.widget.js` 同上 bundle | 同上 | `.bind/.delegate` 系、`$.cleanData`、parseJSON/isArray（压缩形式） | 随 fileupload 一并升级（上游捆绑 widget 1.11+/1.12） |
| jquery.iframe-transport | 1.6.1 | 同上 | 同上 | 低（旧式 API 少） | 随 fileupload 上游版本对齐 |
| zTree | 3.5.17-beta.2 | `jquery.ztree.all-3.5{,-min}.js` | note-dev.html:895、dep.min.js | 3.x 前时代，事件/data 用法旧 | 升级 3.5 最新或验证兼容；诊断跑笔记本树 |
| slimScroll | 1.3.0 | `jQuery-slimScroll-1.3.0/` | note-dev.html:904、dep.min.js | 低-中 | 诊断验证；必要时升 1.3.8 |
| contextmenu | 无 banner（第一方封装 LEA.cmroot） | `contextmenu/jquery.contextmenu{,-min}.js` | note-dev.html:905、dep.min.js | 第一方，按 3.x 检查 | 诊断验证 |
| jquery-cookie | 1.4.0 | `jquery-cookie{,-min}.js` | 三个博客主题 post.html:101 | 低（$.extend/trim 内部） | 诊断验证 |
| jquery-validation | 1.13.0 | `admin/js/jquery-validation-1.13.0/` | 14 个 admin 视图 | 低-中 | 诊断验证 admin 表单；必要时升 1.19.x |
| artDialog | 4.1.7 | `admin/js/artDialog/` | admin/member footer | 3.x 前时代（live/bind 时代） | 重点诊断；不兼容则升级或按 R-jQ5 停分支回报 |
| qrcode | 无 banner（压缩） | `jquery.qrcode.min.js` | 博客主题 post.html:103 | `$.browser` 类旧 API 风险 | 诊断验证；无源可审时记录 provenance 决策 |
| pagination | 1.2 | `jquery.pagination.js` | album（main.all.js）、leaui iframe | 低 | 诊断验证 |
| HTML5 sortable | 2012 Ali Farhadi | `member/js/jquery.sortable.js` | member/blog/cate、single | 非 jQuery 插件依赖弱 | 诊断验证 |

## 5. iframe 边界

- `leaui_image` 真实运行入口：`plugin.min.js` 开 `<iframe src="/album/index…">` → 加载 `album/js/main.all.js`（内含 1.9.0 核心）——随 `album` bundle 切换自动升级；独立 `index.html`（开发/直开路径）删除私有 1.9.1 改根 URL。
- 跨文档访问：`leaui main.js` 用 `top.UrlPrefix`（L20）、`parent.GlobalConfigs`（L749）；note 页 iframe 通信用原生 `postMessage`（page.js:1462-1472）——无跨 iframe jQuery 对象传递，`无需改动`（登记验证结论）。
- RequireJS：`public/js/main.js` jquery path 已注释，md 模块自载；`jquery.ui.widget` amd 分支在 3.7 下由升级版覆盖。

## 6. 排除项（E-TM 所有）

`tinymce.jquery*.js`、`jquery.tinymce*.js`、`leaui_mindmap/**`：TinyMCE 8 升级任务（08-25-tinymce-upgrade）所有。本任务不修改、不将其计入兼容结论；诊断 E2E 遇其警告记为 E-TM 排除而非本任务失败。

## 7. 回归测试映射

- 静态：`tests/js/jquery-asset-contract.test.js`（34 输出、唯一 3.7.1、无 migrate 入生产、无私有 1.9.1、URL 保持）。
- 运行时：诊断 E2E（零 `JQMIGRATE:`）+ 生产 business E2E（登录/笔记/笔记本/标签/对话框/上传/相册/博客/admin-member/leaui iframe，零 console.error/pageerror）。
- AJAX 失败：AC-jQ7 的受控 4xx/5xx/解析失败注入（`ajaxGet`/`ajaxPost`/`ajaxPostJson` 各至少一次）。
