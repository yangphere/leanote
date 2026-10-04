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

## container-smoke 整脚本（已通过，Linux 容器替代 CI）
- 审核发现首版脚本生成非法 `app.conf`（字面量 `
`，`CONFIG_KEY_INVALID`），已在 `5014a008` 修复，并新增会真实执行配置生成块的契约测试
  （对旧脚本失败：`invalid generated config line: n`）。
- 在 `docker:cli`（Alpine，挂载 Docker socket、`--network host`）中完整运行 `scripts/container-smoke.sh`，退出码 0：
  健康就绪、旧 `/note/toPdf` 仍为存根、经 Gotenberg 的真实 `/api/note/exportPdf`、Gotenberg 无外网出口、重启后卷数据保留，且无残留容器。

## 未运行
- GitHub Actions 的 `container-smoke` 作业（Linux runner）：本机只用 Linux 容器替代，CI 本身仍为 `unrun`。
- 用户真实账号在浏览器中导出普通笔记与 Markdown 笔记（中文）的人工验收：未执行；用户要求先提交并归档。
