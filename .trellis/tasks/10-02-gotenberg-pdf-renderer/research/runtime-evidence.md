# 运行证据（2026-10-04）

## R1 门禁（已通过）
- 命令：以 `gotenberg/gotenberg@sha256:f29984bd…c769`（8.37.0，deny-list 与 Compose 一致）监听 `127.0.0.1:33001`，
  `LEANOTE_GOTENBERG_URL=http://127.0.0.1:33001 go test ./app/service/contentpdf -run TestGotenbergBackendRealOutputPassesPDFRenderer -v`。
- 结果：`html`/`markdown`/`external-url` 三个子测试全部通过 `application.PDFRenderer.Render`（含 `validatePDFArtifact`），
  输出头为 `%PDF-1.4`（58409 / 47603 / 5638 字节）。

## Compose 运行（真实用户栈，未使用 `-v`）
- `docker compose up -d --build`：gotenberg 先 healthy，leanote 随后启动；`/healthz` 返回 `{"status":"ready"}`。
- gotenberg 仅接入 `leanote_pdf`，`NetworkSettings.Ports` 为 `{"3000/tcp":null}`（无宿主机端口）；leanote 同时在 `leanote_default` 与 `leanote_pdf`。
- `docker compose exec gotenberg curl -fsS --max-time 5 https://example.com` 失败（无外网出口）。

## 端到端导出（一次性项目 `lnsmoke`，夹具数据，已 `down -v`）
- 夹具用户 `demo`（需在一次性库中重新启用，Docker 引导会禁用它）；导出笔记 `540817e099c37b583c000005`（临时写入中文标题/正文/表格）。
- `GET /api/note/exportPdf` → `200`、`Content-Type: application/pdf`、`Content-Disposition: attachment`、`%PDF-1.4`、66362 字节；
  PyMuPDF 抽取出完整中文文本并渲染出可见中文与表格。
- 夹具笔记原文含已失效的远程图片域名，应用资源加载器拒绝并返回 `sysError`——这是既有行为，不属于 Gotenberg 回归。

## 未运行
- `scripts/container-smoke.sh` 整体：Windows/Git Bash 的绑定挂载与路径转换使其无法在本机运行（Linux CI 脚本），
  其新增的登录/导出/出口检查已按相同命令在一次性 Compose 项目上手工执行；整脚本与 `container-smoke` CI 作业仍为 `unrun`。
- 用户真实账号在浏览器中导出普通笔记与 Markdown 笔记（中文）的人工验收：待用户执行。
