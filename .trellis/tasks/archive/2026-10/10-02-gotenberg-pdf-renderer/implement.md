# 执行计划：Gotenberg PDF 渲染容器

## 有序步骤

1. **事实核验（初步门禁已完成）**：已记录版本/digest、`/health`、镜像 `curl`、路由禁用、deny-list、`waitForExpression`、内联 `data:`、PDF 响应与网络失败证据。R1 尚未闭合，因为探针输出还没有经过 Leanote 的 `validatePDFArtifact`；该校验必须在后端接入后完成，失败则停止并回到设计。
2. **配置与唯一交接**：在 `app/httpserver` 实现一个供 prod/dev/test 共用的 `pdf.renderer`/`pdf.gotenberg.url` 解析器；扩展 `ProductionConfig` 为单一 `service.ContentRuntimeConfig`，其中组合 `ContentRoots` 和 `PDFRendererConfig`，并覆盖缺省进程模式、非法 URL、缺 URL、未知值测试。
3. **后端**：新增 `app/service/contentpdf/gotenberg.go` 与测试（使用 `httptest.Server` 或可注入 `http.RoundTripper`），覆盖 multipart、等待表达式、纸张/边距、超时/取消、非 200、响应体超限、非法 PDF、描述符变化和仅访问配置地址。
4. **装配与 R1 集成门禁**：`content_runtime.go` 按 `ContentRuntimeConfig.PDFRenderer` 选择后端；补选择逻辑测试，并让真实 Gotenberg 输出经 `application.PDFRenderer.Render` 完整通过 `validatePDFArtifact`。在该门禁通过前不得切换 Compose/CI。
5. **管理后台**：`ExportPdf` action 与 `export_pdf.html` 区分渲染器模式；补 controller 测试。
6. **部署**：固定已探测的 Gotenberg `8.37.0` 多架构 manifest index digest `sha256:f29984bd1e226bf1b93ba90af06000afa8b315853e99d27b9aaa41b93f15c769`；`docker-compose.yml` 增加 `gotenberg` 服务与 `pdf` 内部网络、健康检查和 Leanote 健康依赖；Dockerfile 移除 wkhtmltopdf；`conf/app.conf-docker` 启用 Gotenberg。
7. **Smoke/CI**：`scripts/container-smoke.sh` 与 `quality-gate.yml` 的 `container-smoke` 作业启动 Gotenberg，用真实导出生成 PDF 并校验 `%PDF-`；失败时输出 Gotenberg 日志。
8. **真实运行验收**：`docker compose up -d --build`（不加 `-v`）；检查健康状态、无宿主机端口、无外网出口；用户人工登录后，在浏览器中导出普通笔记与 Markdown 笔记各一份 PDF 并检查中文。证据写入 `research/runtime-evidence.md`。
9. **规范同步**：更新 `.trellis/spec/backend/quality-guidelines.md` 中 PDF 渲染相关契约（渲染器选择、Gotenberg 隔离、测试要求），并记录本任务新增的非显然边界。

## 验证命令

- `GOTOOLCHAIN=local go test ./app/httpserver ./app/service/... ./app/application/content ./app/controllers/...`
- `gofmt -l app cmd`、`GOTOOLCHAIN=local go build ./...`、`GOTOOLCHAIN=local go vet ./...`
- `npm test`
- `docker compose config --quiet`、`docker compose up -d --build`、`docker compose ps`
- `docker compose exec gotenberg curl -fsS --max-time 5 https://example.com`（预期失败）
- `curl http://127.0.0.1:9000/healthz`
- 真实 Gotenberg 输出必须通过 Leanote `PDFRenderer.Render`；仅 `qpdf --check` 或 `%PDF-` 检查不计为 R1 通过。
- `git diff --check`、`python ./.trellis/scripts/task.py validate .trellis/tasks/10-02-gotenberg-pdf-renderer`

## 评审门禁与回滚点

- 门禁 A（第 1 步后）：R2/R3 已由 2026-10-03 的真实容器探针确认，初步事实允许继续配置、后端和装配实现；R1 必须在第 4 步完成，R1 未通过前不得改部署文件。
- 门禁 B（第 5 步后）：Go 与 Node 检查全部通过后才改部署文件。
- 回滚点：第 6 步前代码对 Docker 部署无影响（缺省进程模式）；第 6 步后回滚只需恢复 3 个部署文件。

## 停止条件

- Chromium 输出不能通过现有 PDF 校验。
- Gotenberg 无法在无外网出口的网络中渲染自包含文档。
- 需要修改 `.env`、Mongo schema 或删除 named volume。
- 任何真实 HTTP、浏览器、CI、镜像内容或 tar 包 smoke 证据未执行时，不得将对应 acceptance criterion 标记为通过。
