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
| `public/tinymce/tinymce.jquery*.js`、`jquery.tinymce*.js` | TinyMCE 4 内核/适配器 | 已从自托管闭包移除 | E-TM 所有 | 删除并由 TinyMCE 8 `tinymce.js`/`tinymce.min.js` 替代 |
| `leaui_mindmap/mindmap/main.js` | 脑图子应用 | 独立文档内运行 | E-TM 所有 | 同上 |

## 2. 引用 `/js/jquery-1.9.0.min.js` 的模板（URL 保持不变，仅内容升级）

`app/views/note/note-dev.html:894`（dev 块；生产经 `dep.min.js`）、`home/index.html:72`、`home/login.html:71`、`home/register.html:50`、`home/find_password.html:34`、`find_password2.html:43`、`find_password2_timeout.html:28`、`admin/footer.html:9`、`member/footer.html:12`、`file/pdf.html:91`、`album/index.html:157`（注释 dev 块，实载 `main.all.js:165`）、`BlogController.go:171`（`jQueryUrl` 变量 → 三个 repo 主题 footer.html:56）。`public/js/main.js:4` 为注释掉的 RequireJS path。处置：全部 `无需改动`（URL 与脚本顺序保持，内容由 manifest 替换）；回归 = AC-jQ3 的单核心断言 + D 资源 smoke。

## 3. 第一方代码高风险调用

### 3.1 jQuery 3 已移除（升级即坏）— 已适配 ✅

| 位置 | 调用 | 行为 | 处置与回归 |
|---|---|---|---|
| `public/js/app/page.js` | `navs.size()` / `target.size()` | 导航省略号计算抛 TypeError | 已改 `.length`；note 页导航 smoke 待 E2E |
| `public/js/app/blog/view.js` | 同上（复制代码） | 博客视图导航 | 已改 `.length`；博客 E2E |
| `public/blog/js/common.js` | 同上（复制代码） | 博客前台 | 已改 `.length`；博客 E2E |

### 3.2 语义变化（attr/prop、:visible）

| 位置 | 调用 | 风险 | 处置与回归 |
|---|---|---|---|
| `public/js/app/page.js` | `.attr("checked", true)` | 3.x 设 attribute 不设 property，radio 预选失效 | 已改 `.prop()` |
| `public/album/js/main.js` | `.attr("disabled",…)` ×8 | 布尔状态读回为字符串 | 已统一 `.prop()`；album E2E |
| 各处 `.is(":hidden"/":visible")`（page.js:699+、share.js、view.js、blog common.js、share_comment.js、album main.js） | 对话框/表单 display:none 显隐切换 | 3.x 语义变化仅影响"有布局盒但不可见"的元素；以上用法全部是 display:none 切换，语义不变 | 已核验无需改动；对话框 E2E 覆盖 |

### 3.3 弃用但未移除（诊断警告项）— 已适配 ✅

第一方代码的 jQuery 3 弃用 API 已全量清理（2026-08-28 诊断 E2E 驱动）：

- 全部事件 shorthand（`.click(fn)`/`.mousedown(fn)`/`.scroll(reNav)` 等 89+ 处，含命名处理器引用形态）改 `.on(event, fn)`；`.hover(fn,fn)` 三处改 `.on('mouseenter'...).on('mouseleave'...)`（page.js/share.js/notebook.js）。
- `$.trim` 全部第一方调用（common/note/tag/page/album/leaui/blog/view/share_comment/blog-common）改原生 `String.prototype.trim`（保留 null 语义）。
- `$.isFunction`（contextmenu）改 `typeof`；contextmenu 的 `.bind/.unbind/.hover` 改 `.on/.off` 并以 esbuild 再生 `jquery.contextmenu-min.js`（readable 源与 min 同步）。
- `jquery.pagination.js` 的 `.bind("click",…)` 改 `.on("click",…)`。
- markdown 主源（可读源 + esbuild 再生的 `main-v2.min.js`）三处 `.andSelf()→.addBack()`、两处 `.bind()→.on()`、五处 shorthand、两处 `$.trim` 全部清理；文件中剩余 `.bind(` 为 Underscore `_.bind`（非 jQuery）。
- `.attr("contenteditable", <bool>)` 改字符串字面量（migrate 的非布尔属性告警）。
- 静态契约：`tests/js/jquery-asset-contract.test.js` 的 first-party 扫描（24 个第一方源文件 × 全部弃用模式，含命名处理器引用形态）保证不回潮；运行时由诊断 E2E（下文）以 migrate 栈帧文件级归因强制归零。
- 诊断 E2E（`tests/e2e/business/jquery-diagnostics.spec.mjs`）在 `/login`、`/note`、`leaui_image` 文档内注入锁定版 jquery-migrate 3.6.0（紧随核心、先于任何插件），断言零"第一方归属"的 `JQMIGRATE:` 警告；第三方归属按下表精确排除（每条豁免都必须实际命中，否则诊断失败）。

### 3.4 AJAX 失败语义（R-jQ3 契约修复）— 已适配 ✅

`public/js/common.js`：`_ajax`/`ajaxPostJson` 的 `error` 回调改经 `_ajaxFailure` 路由——HTTP 4xx/5xx 必触发 `failureFunc`，无回调时保留 `alert("error!")` 可见提示（原实现把 jqXHR 对象误判为成功）；修正 `datatype:` → `dataType:`（响应恢复自动 JSON 解析）；`async` 不再反转（显式传值按文档语义生效，仓库内无调用方传值，行为无实际变化）。回归：`tests/js/ajax-wrapper-contract.test.js` 6 项（成功/失败路由、NOTLOGIN、可见提示、async 三态、dataType）。

直接 `$.get/$.post` 调用已全部补 `.fail()` 可见失败分支（2026-08-28）：album `main.js` ×7（addAlbum/updateAlbum/deleteAlbum/getAlbums/getImages/deleteImage/updateImageTitle）、`leaui_image/main.js` 同构 ×7、`note.js` searchNote（失败时收起 loading 并 alert；**`textStatus === "abort"` 的主动取消旧请求不告警**）、`member.js`/`admin.js` 的 `openDialog(config.url)`（失败时对话框内容置为 error）、博客 wrapper `blog/js/common.js` 与 `app/blog/common.js` 的 `ajaxGet/ajaxPost`（`.fail` → `alert("error!")`，与 `_ajaxFailure` 兜底一致）。`page.js` 的 `$.post("/suggestion")` 位于注释死代码内（854-879 行），不改。回归：business E2E 的 4xx/5xx 门禁覆盖 wrapper 与搜索路径。

## 4. 第三方插件

| 插件 | 版本 | 位置 | 加载方 | 风险 | 处置 |
|---|---|---|---|---|---|
| blueimp fileupload | **10.32.0**（npm 锁定，MIT） | bundle 输入 `node_modules/blueimp-file-upload/js/jquery.fileupload.js`；普通脚本副本 `plugins/libs-min/` 与 `leaui_image/public/js/`（逐字节同步） | note 插件 bundle、album bundle、member 头像/主题导入、leaui iframe | 5.26 的 `.pipe()` 已随版本消除（10.x 经 `_promisePipe` 在 jQuery ≥1.8 选 `then`；另注：jQuery 3.7.1 的 Deferred 仍保留 `pipe` 别名，旧清单"3.0 已移除"表述有误） | **已升级**（R-jQ5），版本/来源/许可证见 `docs/modernization/fileupload-provenance.md`；真实上传回归 = business E2E 上传用例 |
| jquery.ui.widget | **1.12.1**（fileupload 上游捆绑） | 同上三处分发 | 同上 | 旧 1.10.1 的 `.bind/.delegate` 系内部调用已随上游消除 | **已随 fileupload 升级** |
| jquery.iframe-transport | **10.32.0 同包版本** | 同上三处分发 | 同上 | 低（现代浏览器不触发 iframe 分支） | **已对齐上游** |
| RequireJS 加载契约 | — | note-dev.html dev 块、avatar.html、theme.html | dev/普通页面 | 上游 UMD 在 RequireJS 环境下会因 `jquery` 模块缺失挂起 | 普通脚本在 `require.js` 之前加载（全局分支）；bundle 输入由构建层 `amdGuard` 影子包装；第一方模块改空依赖（详见 provenance 文档） |
| zTree | 3.5.17-beta.2 | `jquery.ztree.all-3.5{,-min}.js` | note-dev.html:895、dep.min.js | `bindTree/unbindTree` 内部 `.bind/.unbind`（诊断实测） | 无零告警上游；按 §4.1 所有权排除，诊断 E2E 实测笔记本树正常渲染 |
| slimScroll | 1.3.0 | `jQuery-slimScroll-1.3.0/` | note-dev.html:904、dep.min.js | `.hover/.bind`（诊断实测） | 上游无维护；按 §4.1 排除，笔记本树滚动 E2E 实测正常 |
| contextmenu | 无 banner（第一方封装 LEA.cmroot） | `contextmenu/jquery.contextmenu{,-min}.js` | note-dev.html:905、dep.min.js | 第一方，按 3.x 检查 | **已适配**（bind/unbind/hover/mousedown/isFunction 清理 + esbuild 再生 min），右键菜单诊断实测零告警 |
| jquery-cookie | 1.4.0 | `jquery-cookie{,-min}.js` | 三个博客主题 post.html:101 | 低（$.extend/trim 内部） | 诊断范围外（博客主题页） |
| pagination | 1.2 | `jquery.pagination.js` | album（main.all.js）、leaui iframe | 低 | **第一方维护，已按 3.x 适配**（`.bind`→`.on`） |
| HTML5 sortable | 2012 Ali Farhadi | `member/js/jquery.sortable.js` | member/blog/cate、single | `.mousedown(fn)` 等 shorthand | 诊断范围外（member 页）；随 member 区验收处理，记录于此 |
| artDialog | 4.1.7 | `admin/js/artDialog/` | admin/member footer | `.bind/.unbind` 系 | 诊断范围外（admin 页）；无零告警上游，随 admin 区验收处理 |
| qrcode | 无 banner（压缩） | `jquery.qrcode.min.js` | 博客主题 post.html:103 | `$.browser` 类旧 API 风险 | 诊断范围外（博客主题页）；无源可审，记录 provenance 决策 |
| jquery-validation | 1.13.0 | `admin/js/jquery-validation-1.13.0/` | 14 个 admin 视图 | `$.trim/$.isArray` | 诊断范围外（admin 页）；升级 1.19+ 可消除 |
| jsrender | vendored | `blog/js/jsrender.js` | 博客前台 | `$.trim` | 诊断范围外（博客主题页） |
| bootstrap-hover-dropdown | vendored | `blog/js/bootstrap-hover-dropdown.js` | 博客主题页 | `.hover(fn)` | E-BS 所有（随 Bootstrap 升级处置） |

### 4.1 诊断排除归属表（AC-jQ7）

诊断 E2E 对"第一方归属"警告强制为零；以下第三方来源按所有权精确排除，每条豁免必须在运行中实际命中（`excluded.length > 0` 断言），且任何未登记来源都会使诊断失败：

| 来源（栈帧归属） | 所有权 | 依据 |
|---|---|---|
| `dep.min.js` 内 `jquery.ztree.all-3.5-min.js`、`jquery.slimscroll-min.js`、`bootstrap-min.js`（诊断 bundle 按行区间归因到输入文件） | 后续前端任务 / E-BS | 无零告警上游可升级；bootstrap 升级归 08-25-bootstrap-upgrade |
| `main.all.js` 内 `bootstrap-min.js`（album 页） | E-BS | 当前加载路径未观察到告警；映射保留（出现即豁免并记录），非必达断言项 |
| `dep.min.js`/`main.all.js` 内 verbatim 上传栈输入（`node_modules/blueimp-file-upload/...` 的 `$.isArray/$.isFunction`） | R-jQ5 逐字节同步的 npm dist | 上游字节不可补丁；first-party 扫描断言第一方源无同签名 |
| `public/md/main-v2.js` 内嵌 `waitForImages` 1.4.2 的 `$.isFunction`（约 13758 行；由 `public/md/main-v2.min.js` 派生加载） | 第三方 waitForImages 1.4.2，Markdown 资产维护边界 | 上游源码逐字节保留；静态契约仅允许该精确归属，诊断 warning 必须命中登记来源，未登记来源 fail-closed |
| `/public/admin/js/artDialog/jquery.artDialog.js` | vendored artDialog | 诊断范围外（admin 区验收处理） |
| `tinymce.min.js` | E-TM 所有 | TinyMCE 8 唯一生产 core |
| `bootstrap.js`（login）、leaui `bootstrap3/js/bootstrap.min.js` | E-BS 所有 | 同上 |

必达类别：`dep-lib`、`verbatim-input`、`verbatim-url`、`lib-url`、`upstream-signature`、`markdown-waitforimages`（每次诊断运行必须全部命中，缺失即失败）；未登记来源一律 fail-closed。

## 5. iframe 边界

- `leaui_image` 真实运行入口：`plugin.min.js` 开 `<iframe src="/album/index…">` → 加载 `album/js/main.all.js`（内含 npm jQuery 3.7.1 核心）——随 `album` bundle 切换自动升级；独立 `index.html`（开发/直开路径）删除私有 1.9.1 改根 URL。
- 跨文档访问：`leaui main.js` 用 `top.UrlPrefix`、`parent.GlobalConfigs`；note 页 iframe 通信用原生 `postMessage`——无跨 iframe jQuery 对象传递，`无需改动`（登记验证结论）。
- RequireJS：`public/js/main.js` jquery path 已注释，md 模块自载；`jquery.ui.widget` amd 分支在 3.7 下由升级版覆盖。

## 6. 排除项（E-TM 所有）

`leaui_mindmap/**`：TinyMCE 8 升级任务（08-25-tinymce-upgrade）所有。本任务不修改、不将其计入兼容结论。

## 7. 回归测试映射

- 静态：`tests/js/jquery-asset-contract.test.js`（34 输出、唯一 3.7.1、无 migrate 入生产、无私有 1.9.1、URL 保持；fileupload 10.32.0 溯源与逐字节同步、旧副本删除、bundle amdGuard 输入、markdown 源/min 无 andSelf/bind、24 个第一方源文件的弃用 API 扫描）。
- 运行时：`npm run test:e2e:build`（资源与只读页面健康 + 身份预检）+ `npm run test:e2e` business 套件（身份预检与登录后 admin/member 权限门禁、写入前 fresh 身份确认、错误密码负向回归、登录、笔记列表/搜索、笔记+标签写入与清理、笔记本写入与清理、对话框、附件与相册真实上传及清理（上传前登记 cleanup、响应异常按唯一文件名兜底）、相册、博客、admin/member、leaui_image iframe 零缺失资源门禁；清理断言业务 `Ok:true` 并复查列表/回收站/tombstone，cleanup 内步骤聚合错误不短路；零 console.error/pageerror/unhandled rejection/自有资源 4xx-5xx）。两者仅可在 `go run ./app/tests/harness/cmd/e2e -- <command>` 提供的 test-mode 隔离环境内运行（随机 run token、`e2e_runs` marker、轮换后的 admin 凭据、启动前安装信号处理器 + 无条件幂等 teardown + Windows Job Object 整树回收；CI 门禁见 `.github/workflows/ci.yml` 的 reusable `quality-gate`）。
- 诊断 E2E：`tests/e2e/business/jquery-diagnostics.spec.mjs`（migrate 3.6.0 注入 + 栈帧文件级归因，第一方零警告；§4.1 排除表必须在运行中命中）。
- 浏览器 smoke：`docs/modernization/browser-smoke/jquery-3.7.md`（Chrome/Edge 真实二进制 + Firefox 已记录；Safari 与前一主版本为发布前阻断项）。
- 身份 fail-closed：`app/controllers/TestE2eController_test.go`（非 test mode/非 loopback 404；marker 缺失/重复/过期/摘要不匹配/错误 run kind/错误数据库 503）+ business 套件的 token 篡改断言。
- AJAX 失败：`tests/js/ajax-wrapper-contract.test.js`（wrapper 6 项）+ `tests/e2e/business/ajax-failure.spec.mjs`（五类直接调用各注入一次 4xx/5xx：album getAlbums、leaui getImages、note searchNote、admin openDialog、blog ajaxGet wrapper，断言可观察失败行为——alert 文本、dialog 错误内容）。
