# Gotenberg 事实记录

## 2026-10-02 文档核对（gotenberg.dev，未在本机运行）

- 当前主版本为 8。
- HTML 转换：`POST /forms/chromium/convert/html`，上传的文件必须命名为 `index.html`。
- 表单字段：`waitForExpression`（JS 表达式返回 true 后开始渲染）、`waitDelay`、`failOnConsoleExceptions`（409）、`failOnResourceLoadingFailed`（400）、`skipNetworkIdleEvent`、`paperWidth`/`paperHeight`/`margin*`（单位 in/pt/cm，默认 Letter）、`printBackground`。
- 响应头：`Gotenberg-Trace`。
- 启动参数：`--chromium-allow-list`（默认全部）、`--chromium-deny-list`（默认 `^file:(?!//\/tmp/).*`）、`--chromium-disable-javascript`、`--api-port`（3000）、`--api-timeout`（30s）、`--api-body-limit`、`--libreoffice-disable-routes`、`--pdfengines-disable-routes`、`--webhook-disable`、`--api-enable-basic-auth`（`GOTENBERG_API_BASIC_AUTH_USERNAME`/`PASSWORD`）。

## 待实施第 1 步验证（unrun）

- 固定版本与镜像 digest。
- `/health` 路径与镜像内健康检查命令（是否自带 curl）。
- 自定义 deny-list 正则是否按预期拦截 `http(s)`/`file` 外部访问，同时不误拦文档内联资源使用的 `data:` URL。
- `waitForExpression=window.status === 'done'` 是否及时完成。
- Chromium 输出能否通过 `validatePDFArtifact`（R1）。
- 镜像自带字体对中文的覆盖情况。
