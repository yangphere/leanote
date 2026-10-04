# 本轮运行验证

## 2026-10-02 修复前基线

- 当前 Compose 的 Mongo 为 healthy，Leanote 在 9000 端口运行；`/healthz` 返回 HTTP 200。
- 真实 HTTP 请求 `/blog/admin` 与 `/blog/post/admin/12da3e968ef6` 均返回 HTTP 500，正文为 `Internal Server Error`。
- 共同根因：`conf/app.conf-docker` 没有 `site.url`，`ConfigService.SetAppConfigSource` 得到空默认域名；`resolveBlogDomain` 在 `CanonicalizeBlogHost` 校验阶段返回 500。公开文章路径中的 12 位字符串是 URL title，并非 ObjectID。
- 用户已在 Codex 内置浏览器登录。当前笔记菜单的“分享给好友”未显示弹窗；控制台记录 `SelectorEngine.findOne -> Modal._showElement` 的 `Illegal invocation`。前端 `showDialogRemote` 期望 HTML，但 `ShareHTTPServer.dispatch` 缺少两个共享信息 action，返回 validation JSON。非 HTML 内容缺少 `.modal-dialog`，造成 Bootstrap 实例结构不完整。
- 菜单“导出PDF”在 20 秒内没有 download 事件；直接访问同一笔记的 `/note/exportPdf` 在已登录会话中显示 `error`，10 秒内没有 download 事件。

## PDF 部署限制

- Mongo 的 keyed configuration 中没有 `exportPdfBinPath`。当前 PDF backend 读取该管理配置，空值在执行前被拒绝。这是尚未配置的部署依赖，不据此改写 renderer 或降低安全校验。
- 容器安装了 `wkhtmltopdf 0.12.6`；`/usr/bin/wkhtmltopdf` 为普通可执行文件，`/usr/local/bin/wkhtmltopdf` 是符号链接，不符合现有可执行路径校验。
- 使用与 renderer 一致的净化环境和参数，对合成的非敏感 HTML 运行 `/usr/bin/wkhtmltopdf`：退出码 0，输出 7811 字节，文件头 `%PDF-`。只生成了临时探针文件，验证后已清理。
- 当前真实笔记导出：**FAIL（缺少管理配置）**。工具探针：**PASS**；不能等同于真实笔记下载通过。未更改现有管理配置、`.env`、密码或 schema。

## 修复后验收

- Compose 配置：`docker compose config --quiet` **PASS**。
- 镜像和服务：`docker compose build leanote` **PASS**；随后使用
  `docker compose up -d --no-deps --force-recreate leanote` 重建应用容器，未执行
  `down -v`。`docker compose ps` 显示 `leanote` 与 Mongo 均运行，Mongo 为
  `healthy`；`GET /healthz` 返回 `200` 与 `{"status":"ready"}`。
- 博客 HTTP：重建后 `GET /blog/admin` 返回 `200`（正文 7266 字节），已发布文章
  `GET /blog/post/admin/about-leanote` 返回 `200`（正文 15954 字节）。原始基线中
  的伪造 title `/blog/post/admin/12da3e968ef6` 不再返回 500，而是明确 `404`。
- 博客浏览器：已登录内置浏览器访问管理页和公开文章页，页面标题与文章正文均可观察。
- 笔记分享：刷新登录笔记页后，从当前笔记右键菜单选择“分享给好友”，显示完整
  Bootstrap modal（Email、权限、分享按钮和“添加分享”）；点击关闭后
  `#leanoteDialogRemote` 不可见且页面无 dialog。刷新后的新浏览器日志没有新增
  `Illegal invocation`；日志中唯一同类错误时间为修复前 `2026-10-02T00:49:45Z`。
- 笔记本分享：对 `Work` 笔记本执行相同打开/关闭流程，完整 modal 可见且关闭后
  `#leanoteDialogRemote` 不可见；同样没有修复后新增的 `Illegal invocation`。
- 空邮箱校验：在笔记分享 modal 中直接点击“分享”（未输入任何收件人）显示
  `请输入好友邮箱`，未建立共享数据；未执行真实收件人授权或清理，以避免在未获
  得明确收件人确认时扩大笔记访问权限。
- 实际共享建立及清理：**UNRUN**；本轮没有向任何账户授予笔记访问权。
- PDF 下载：**FAIL（部署配置限制）**。实际笔记导出仍因 Mongo 管理配置缺少
  `exportPdfBinPath` 被拒绝；未修改该配置、`.env`、密码或 schema。独立的
  `/usr/bin/wkhtmltopdf` 探针仍可退出 0 并生成 `%PDF-`，不能替代真实笔记下载证据。
  重建后的已登录浏览器再次从当前笔记选择“导出PDF”，10 秒内没有 download
  事件，结果保持 FAIL。

## 修复后自动化检查

- `npm run build`：**PASS**。
- `npm test`：**214 passed / 0 failed / 1 skipped**（完整测试集，2026-10-02）。
- `npm run test:e2e:build -- --list`：**PASS**，枚举 1 个 build-smoke 用例。
- `GOTOOLCHAIN=local go test ./app/controllers ./app/service ./app/httpserver`：**PASS**。
- `GOTOOLCHAIN=local go vet ./app/controllers ./app/service`：**PASS**。
- `GOTOOLCHAIN=local go build ./...`：**PASS**。
- `git diff --check`：**PASS**。
- `python ./.trellis/scripts/task.py validate 10-02-docker-runtime-bug-fixes`：**PASS**。

## 评论区缓存修复后的真实浏览器验证（2026-10-02）

- 针对公开文章 `http://127.0.0.1:9000/blog/post/admin/f353155fddab` 刷新页面；页面实际加载
  `/public/blog/css/share_comment.css?v=0.0.0`，确认已绕过旧 CSS 缓存。
- 读取页面计算样式：`.comments-loading.hide` 与 `.hide.comments-more` 均为
  `display: none`，加载图区域为 0×0；评论区不再持续显示 spinner。
- 同页能观察到登录提示和 `0 comments`，刷新后的控制台没有错误。普通与 Markdown 两篇测试博客的 `.comments-loading` 和 `.comments-more` 均为 `display: none`，加载图区域为 0×0。
- Markdown 笔记 `6abf0de87787f94a7d000001`：通过真实编辑器保存后刷新，正文仍在，编辑器图片为
  `http://127.0.0.1:9000/images/blog/default_avatar.png`，自然尺寸 160×160；公开博客
  `/blog/post/admin/f353155fddab` 刷新后正文仍在，图片自然尺寸 160×160。
- 普通笔记 `6abf0a897787f94a7d000000`：通过图片上传工具上传测试图并插入，真实保存后刷新，正文仍在，
  `/file/outputImage?fileId=6abf27437b72268a52750eaa` 自然尺寸 160×100；公开博客
  `/blog/post/admin/c7f9a417190c-2` 刷新后正文仍在，图片自然尺寸 160×100。
- 普通与 Markdown 两条公开博客都加载 `/public/blog/css/share_comment.css?v=0.0.0`，评论 spinner
  缓存失效和图片/正文验收均为 **PASS**。

## localhost 公开博客回归验证（2026-10-02）

- 复现：`GET http://localhost:9000/blog/post/admin/6abf36b29882235fe7000001` 在修复前返回
  `404 Not Found`；同一 URL 使用 `127.0.0.1` 主机返回 `200`。
- 根因：Docker 的 `site.url` 默认主机是 `127.0.0.1`，`resolveBlogDomain` 将 `localhost`
  误判为未配置的自定义域名。
- 修复：默认博客主机匹配仅把 `localhost` 与 `127.0.0.1` 视为 loopback 别名；普通域名及
  `127.0.0.2` 仍要求严格匹配。
- Docker 重建并重建应用容器后，`localhost` 与 `127.0.0.1` 两个请求均返回 `200`；内置浏览器
  打开 `http://localhost:9000/blog/post/admin/6abf36b29882235fe7000001` 可观察到文章正文，
  评论加载层仍为 `display:none`。
- 回归测试：`go test ./app/domain ./app/service ./app/controllers ./app/httpserver`、`go vet`
  对应包、Docker `npm run build`、`git diff --check` 和 Trellis validate 均 **PASS**。

本文件不记录密码、Cookie、认证头、用户笔记正文、页面全文或截图。未执行 `down -v`，未删除已有数据。

## 普通笔记剪贴板图片重复插入验证（2026-10-02）

- 修复前基线：普通笔记粘贴剪贴板图片时，TinyMCE `paste_data_images: true` 先插入一个
  `blob:` 图片；Leanote 的 `editor_drop_paste` 同时把同一剪贴板文件交给 `/file/pasteImage`
  上传器并再插入服务端图片，因此一次粘贴可观察到两个相同图片节点，并可能留下上传占位节点。
- 根因边界：同一 `paste` 事件存在两个图片处理所有者。TinyMCE 的 data-image handler 和
  Leanote 上传器都处理了图片，导致同一用户操作产生两条插入路径。
- 修复：`public/js/tinymce-config-source.js` 将普通笔记配置的 `paste_data_images` 设为
  `false`；`public/js/plugins/editor_drop_paste.js` 在 `#editorContent` 的捕获阶段识别图片
  剪贴板项，阻止 TinyMCE 后续处理，并将文件显式交给现有 `fileupload('add')` 上传器。
  `npm run build` 已同步 `public/js/tinymce-config.js` 与 `public/js/plugins/main.min.js`。
- 修复后真实浏览器证据：在 Docker 重建后的已登录浏览器中重复粘贴测试图，编辑器中没有
  `blob:` 图片，只出现一个服务端图片 URL；Mongo 文件记录每次测试粘贴只增加一个新图片；
  保存并刷新后正文只保留一张图片，公开博客中的对应正文也只显示一张。浏览器控制台没有
  新增错误。
- 自动化回归：TinyMCE 配置测试确认 `paste_data_images` 关闭；粘贴插件契约测试确认捕获
  `paste`、阻止默认/后续处理并调用唯一上传器；完整 `npm test` 与 `npm run build` 均通过。
- 测试数据恢复：验证过程中产生的临时图片和占位内容已通过界面恢复；普通测试笔记恢复为
  原正文和原图片。未删除用户已有数据，未执行 `docker compose down -v`。
