# blueimp jQuery-File-Upload 升级溯源（R-jQ5）

## 版本与来源

| 项目 | 值 |
|---|---|
| 上游 | blueimp/jQuery-File-Upload（Sebastian Tschan） |
| 版本 | **10.32.0**（npm 包 `blueimp-file-upload`，10.x 线最终版） |
| 安装方式 | `package.json` devDependencies 精确锁定；`package-lock.json` 记录 integrity |
| 许可证 | MIT（`node_modules/blueimp-file-upload/LICENSE.txt`） |
| 组件 | `js/vendor/jquery.ui.widget.js`（jQuery UI widget 1.12.1）、`js/jquery.iframe-transport.js`、`js/jquery.fileupload.js` |

## 分发位置（全部为 npm dist 的逐字节副本）

1. `public/js/plugins/libs-min/jquery.ui.widget.js`、`jquery.iframe-transport.js`、`jquery.fileupload.js`
   —— member 头像（`app/views/member/user/avatar.html`）、博客主题导入（`app/views/member/blog/theme.html`）
   以普通 `<script>` 在 `require.js` 之前加载（无 `define`，走 browser-globals 分支）。
2. `public/tinymce/plugins/leaui_image/public/js/` 同名三件
   —— `leaui_image/index.html` 开发路径普通加载；生产真实入口是 `/album/index`（album bundle）。
3. `plugins` 与 `album` bundle 的 manifest 输入直接引用
   `node_modules/blueimp-file-upload/js/...`，经 esbuild 压缩进 `public/js/plugins/main.min.js`
   与 `public/album/js/main.all.js`。

## AMD 防护（构建层）

上游三件均带 UMD 工厂。note 页面 RequireJS 存在（markdown bundle 内嵌 require.js），
若 bundle 内执行 AMD 分支会产生匿名 define 冲突且 `jquery` 模块永不注册。因此
`scripts/build/js.mjs` 对 manifest 声明的 `amdGuard` 输入追加
`(function(define){…})(void 0);` 影子包装，强制其走 browser-globals 分支；
上游文件字节本身不做任何修补。RequireJS 场景（member 页）改为在 `require.js`
之前普通加载，第一方模块（`attachment_upload`/`editor_drop_paste`/`avatar`/
`import_theme`）已改为空依赖，不再通过 RequireJS 引用 `fileupload`。

## 同步契约（回归）

`tests/js/jquery-asset-contract.test.js` 断言：

- `package.json` 锁定 `blueimp-file-upload@10.32.0` 且许可为 MIT；
- 上文两处分发副本与 `node_modules/blueimp-file-upload` dist **逐字节一致**；
- manifest 的 `plugins`/`album` 输入为 npm 路径且 `amdGuard` 覆盖这三件；
- 生成 bundle 不再包含 5.26 旧版横幅。

副本漂移时 `npm test` 失败；升级时执行
`npm install --save-dev blueimp-file-upload@<version>` 后重新复制三件即可。

## 行为差异备忘（5.26 → 10.32）

- Deferred 调用改为 `_promisePipe`（jQuery ≥1.8 选 `then`），不再无条件 `.pipe()`。
- `maxFileSize`/`acceptFileTypes` 校验在 5.26 core 与 10.x core 中同样不生效
  （历史即如此，album 的 `add` 回调自做大小检查），无需引入 validate 插件。
- 首选 `data.submit()` 的 `add` 回调、`done`/`fail` 回调、`formData` 函数形式、
  `pasteZone: ''`/`dropZone: ''` 禁用语义在 10.x 全部保持。
