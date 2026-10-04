# Gotenberg 事实记录

## 2026-10-02 官方资料核对

- Gotenberg 主版本为 8。
- HTML 转换使用 `POST /forms/chromium/convert/html`，上传文件字段为 `files`，文件名必须为 `index.html`。
- 相关表单字段包括 `waitForExpression`、`paperWidth`、`paperHeight`、`margin*` 和 `printBackground`；纸张与边距支持 `in`/`pt`/`cm` 单位，默认纸张为 Letter；响应包含 `Gotenberg-Trace`。
- 相关启动参数包括 `--api-timeout`、`--chromium-deny-list`、`--libreoffice-disable-routes`、`--pdfengines-disable-routes` 和 `--webhook-disable`。

## 2026-10-03 本机 Docker 探针

探针使用 Docker `29.8.1`、Linux x86_64，未停止或重建现有 Leanote/Mongo 容器。

- 镜像 `docker.io/gotenberg/gotenberg:8.37.0` 解析为版本 `8.37.0`；通过 `docker buildx imagetools inspect docker.io/gotenberg/gotenberg:8.37.0` 取得的多架构 manifest index digest 为 `sha256:f29984bd1e226bf1b93ba90af06000afa8b315853e99d27b9aaa41b93f15c769`。该 index 包含 `linux/amd64` 子 manifest `sha256:3fdee07e0dcd5005c3db4d81d1600d081fc6fb424da87e409e1c1f9624a249b8`；Compose 应固定 index digest，不应使用 `docker inspect .Id` 作为镜像引用。
- 镜像入口为 `/usr/bin/tini --`、命令为 `gotenberg`，运行用户为 `gotenberg`（UID/GID 1001）。镜像没有内置 Docker `HEALTHCHECK`，但包含 `/usr/bin/curl`。
- `/health` 返回 HTTP 200 和 `{"status":"up"}`；禁用 LibreOffice、PDF engines 后对应路由返回 404。
- 以下参数启动成功，并用于后续转换探针：

  ```text
  --api-timeout=40s
  --libreoffice-disable-routes=true
  --pdfengines-disable-routes=true
  --webhook-disable=true
  --chromium-deny-list=^(?!file:///tmp/|data:).*
  ```

- `POST /forms/chromium/convert/html` 使用 `files=index.html` 成功返回 200；`waitForExpression=window.status === "done"` 生效，页面延迟约 2 秒设置状态时总耗时约 5.3 秒。
- 自包含 HTML 中的内联 `data:` SVG 成功渲染。输出为 31,238 字节，`Content-Type` 为 `application/pdf`，以 `%PDF-1.4` 开头并包含 `Gotenberg-Trace` 响应头。
- 对同一输出执行 `qpdf --check` 返回 0，未报告语法或流错误。
- 含 `https://example.com/...` 的 HTML 仍可生成 PDF，但 Gotenberg 日志明确记录该 URL 被 deny-list 拦截；内部网络容器中直接 `curl https://example.com` 以退出码 6 失败。

## 门禁状态

- **R1 仍未闭合**：上述输出尚未通过 Leanote 现有 `PDFRenderer` 调用链中的 `validatePDFArtifact`。`qpdf --check` 和 `%PDF-` 检查不能替代该校验；实现 `GotenbergBackend` 并接入 `PDFRenderer` 后，必须用真实 Gotenberg 输出走完整调用链，失败则回到设计，禁止放宽校验。
- 已确认的探针事实不能替代最终 Compose 拓扑、健康依赖、真实 Leanote 鉴权导出、浏览器普通/Markdown 笔记下载、中文正文可见性、后台只读/拒绝 POST、CI `container-smoke`、镜像无 `wkhtmltopdf` 和 tar 包 smoke 验收；这些仍标记为 `unrun`。
