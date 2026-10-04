# Bootstrap 5.3 升级（E-BS）— 技术设计

## 1. 资源与公开 URL 边界

Bootstrap 5.3.8 由 npm lockfile 管理，`scripts/build/manifest.mjs` 是唯一生成入口。为兼容当前仍被引用的 URL，manifest 生成四个 canonical 入口：

- CSS：`public/css/bootstrap.css`、`public/css/bootstrap-min.css`。
- JavaScript：`public/js/bootstrap.js`、`public/js/bootstrap-min.js`。

四个入口分别从 `bootstrap/dist/css/{bootstrap.css,bootstrap.min.css}` 和包含 Popper 的 `bootstrap/dist/js/bootstrap.bundle.{js,min.js}` 生成。`dep`、`album` 与任何其他 bundle 直接声明相同 npm 输入，不引用生成输出，避免循环和第二事实来源。当前 manifest 的 34 项输出必须同步更新为新的明确计数，并更新所有 manifest/漂移/资源测试。

主站、admin、member、blog、note、album、PDF 和 iframe 只允许加载上述 canonical URL。`/public/admin/css/bootstrap.3.2.0.min.css` 与 `leaui_image` 的相对 `public/bootstrap3` 目录在模板迁移后不得再被请求。未引用的 `bootstrap.min.css`、`bootstrap-theme*.css`、`bootstrap.min.js` 等历史副本只有在研究清单证明无引用后才删除。

## 2. 页面与所有权

实现范围按 `research/bootstrap3-usage.md` 的资产/调用表执行，至少包括：

- 服务端模板：home、note（源 `note-dev.html`）、album、admin、member、user、share、file/pdf；生成的 `note.html` 只能由 build 更新。`public/tinymce/plugins/image/dialog.htm` 及其直接 `js/dialog.js` handler 作为 TinyMCE 图片 UI 的 Bootstrap 消费者，只迁移其 tab/class/API 适配，不改 TinyMCE core、bridge 或公开插件 API。
- 内置博客主题：`public/blog/themes/default`、`elegant`、`nav_fixed`。用户上传主题的字节和路径不改写，只保留加载/注入契约，不为未知自定义 class 提供静默兼容。
- 图片 iframe：实际入口是 `public/tinymce/plugins/leaui_image/index.html`，其 `public/bootstrap3` CSS/JS/fonts 副本删除，内部 UI 迁移；TinyMCE core、`jquery.tinymce` bridge 和 `leaui_mindmap` 不归本任务。
- 第一方代码：`common.js` 的 dialog/loading/remote 路径、tips/history、note/page/notebook/share/blog、markdown、album 以及 admin/member/blog 的内联脚本。
- 直接依赖插件：`bootstrap-hover-dropdown` 有可读源码；`bootstrap-dialog.min.js` 只有压缩运行时，必须升级到有 provenance 的 Bootstrap 5 版本或按原公开 API 做第一方等价替换，不能直接 patch min 文件或保留 Bootstrap 3 shim。

## 3. Markup 与 data API

先从源码生成映射表，再逐项迁移：

- Bootstrap 属性：`data-toggle/target/dismiss/slide*/spy` → 对应 `data-bs-*`。
- 非 Bootstrap 属性必须保留并在映射表中登记：member/admin 的 `data-toggle="fullscreen"`、`data-toggle="class:*"`，home 的 `data-target="body"`，博客 hover 的 `data-hover="dropdown"`。
- `pull-*` → `float-*`；`hidden-*`/`visible-*` → `d-*` 响应式类；`form-group`、`input-group-addon`、`btn-default`、`panel`/`well`、`navbar-inner`、旧 grid → Bootstrap 5 等价结构或最小第一方 CSS。
- `.close`/`&times;` → `.btn-close` 与 `data-bs-dismiss`；Glyphicons 替换为已存在的 Font Awesome 或明确文本。

模板、动态 HTML 字符串和运行时 DOM 都使用同一映射。扫描器不能以“没有 `data-toggle`”作为唯一通过条件，必须允许并核对上面的非 Bootstrap 属性；同时必须把 `public/tinymce/plugins/image/dialog.htm` 的 `data-toggle="tab"` 纳入 Bootstrap 旧用法清单。

## 4. JavaScript 行为契约

- 统一封装 `Modal`、`Tab`、`Dropdown`、`Tooltip`、`Alert` 的原生实例 API，使用 `getOrCreateInstance` 和 `show/hide`；保留 Bootstrap 事件名、一次性触发、焦点、Esc、backdrop 和销毁行为。
- 将所有 `.button('loading'/'reset')` 调用收敛到一处 loading helper：保存原始文本/HTML、disabled、ARIA 状态，所有 success/business error/HTTP error/异常分支都执行恢复；不得通过 jQuery plugin shim 实现。
- `showDialogRemote` 显式编码参数并请求原 URL；只在成功响应注入 `#leanoteDialogRemote` 的既有容器后打开，非 2xx、网络失败或内容错误显示可见错误并恢复状态。后端路由与响应格式不变。
- `BootstrapDialog` 的 `show`、`confirm`、`getModalBody`、`close` 是 blog 的跨主题公开调用契约；替换实现必须覆盖登录提示、评论删除确认、举报表单和二维码对话框。
- hover dropdown 实现必须保留延迟、触摸设备行为、键盘/点击关闭和子菜单；不得依赖 Bootstrap 3 的 `.open` 或 jQuery plugin API。

## 5. leaui_image 数据流

iframe 继续由现有父页面/编辑器打开，保留 `top.LEAUI_DATAS` 初始选中图片、`parent.GlobalConfigs.uploadImageSize` 限制、`parent.getMsg` 翻译、`mdGetImgSrc` 返回值、真实 fileupload 回调、分页、URL 图片和图片 title/width/height/constrain 写回。只替换 Bootstrap UI 和组件控制方式，不改 TinyMCE core、后端端点或跨 iframe 结构。

## 6. 验证、回滚与失败边界

- 复用 D/E 的 Node 24、Playwright 1.62.1、test-mode harness、身份预检和脱敏 reporter；新增 Bootstrap 专用 Chromium 用例，但不新增 Playwright 配置或 lockfile。
- 先运行静态资产/markup contract（包括旧 Bootstrap URL 禁止清单和 JQMIGRATE allowlist 收紧），再运行组件 E2E 与业务失败注入，最后运行 build、Golden/USN/page smoke 和真实四浏览器发布前记录。测试覆盖所有表中页面和 iframe，且不写入真实数据。
- 资源、模板/API、博客插件、iframe UI 四个批次分别可审查；任一批次不能等价时回到规划。禁止双加载 Bootstrap 3/5、永久 adapter、静默 fallback、半套生成物或未声明 URL。
- 回滚恢复上一提交的单一 Bootstrap 3 状态后，必须重新 build 并执行零 diff、资源和业务门禁；不改变数据库、后端接口或用户上传主题文件。

### 零 diff 协议

- 在存在未提交源码改动的开发工作树中，第一次 `npm run build` 后保存 manifest 声明输出的文件哈希/二进制快照；第二次 `npm run build` 只与该快照比较，要求所有声明输出和未跟踪输出完全相同。该比较不把当前任务相对 `HEAD` 的预期源码/生成物 diff 当作失败。
- 在干净 CI checkout 中执行 `npm ci && npm run build`，再执行 `git diff --exit-code` 与 `git status --porcelain --untracked-files=all`；这两个命令均为零才代表提交产物无漂移。

### 发布 smoke 记录

真实浏览器结果写入受跟踪的 `docs/modernization/browser-smoke/bootstrap-5.3.md`，每条记录包含提交 SHA、执行日期、产品与完整版本、操作系统、页面/iframe 覆盖、身份预检、错误/网络门禁和结果；不得记录账号、Cookie、请求头、页面正文或用户数据。
