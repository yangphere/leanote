# 执行计划：Gotenberg PDF 渲染容器

## 有序步骤

1. **事实核验（门禁）**：拉取 `gotenberg/gotenberg:8` 并记录 digest；在内部网络中启动它，验证 `/health`、镜像内健康检查命令、deny-list 正则、`waitForExpression`；用一份由 `serializeSelfContainedPDF` 生成的中文样例文档调用 HTML 路由，并用 `validatePDFArtifact` 校验输出。结果写入 `research/gotenberg-facts.md`。R1 不通过则停止，回到设计。
2. **配置**：在 `app/httpserver` 解析 `pdf.renderer`/`pdf.gotenberg.url`，加入 `ProductionConfig` 并写配置测试。
3. **后端**：新增 `app/service/contentpdf/gotenberg.go` 与测试（使用 `httptest.Server`）。
4. **装配**：`content_runtime.go` 按配置选择后端；为选择逻辑补测试。
5. **管理后台**：`ExportPdf` action 与 `export_pdf.html` 区分渲染器模式；补 controller 测试。
6. **部署**：`docker-compose.yml` 增加 `gotenberg` 服务与 `pdf` 内部网络；Dockerfile 移除 wkhtmltopdf；`conf/app.conf-docker` 启用 Gotenberg。
7. **Smoke/CI**：`scripts/container-smoke.sh` 与 `quality-gate.yml` 的 `container-smoke` 作业启动 Gotenberg，用真实导出生成 PDF 并校验 `%PDF-`；失败时输出 Gotenberg 日志。
8. **真实运行验收**：`docker compose up -d --build`（不加 `-v`）；检查健康状态、无宿主机端口、无外网出口；用户人工登录后，在浏览器中导出普通笔记与 Markdown 笔记各一份 PDF 并检查中文。证据写入 `research/runtime-evidence.md`。
9. **规范同步**：更新 `.trellis/spec/backend/quality-guidelines.md` 中 PDF 渲染相关契约（渲染器选择、Gotenberg 隔离、测试要求）。

## 验证命令

- `GOTOOLCHAIN=local go test ./app/httpserver ./app/service/... ./app/application/content ./app/controllers/...`
- `gofmt -l app cmd`、`GOTOOLCHAIN=local go build ./...`、`GOTOOLCHAIN=local go vet ./...`
- `npm test`
- `docker compose config --quiet`、`docker compose up -d --build`、`docker compose ps`
- `docker compose exec gotenberg curl -fsS --max-time 5 https://example.com`（预期失败）
- `curl http://127.0.0.1:9000/healthz`
- `git diff --check`、`python ./.trellis/scripts/task.py validate .trellis/tasks/10-02-gotenberg-pdf-renderer`

## 评审门禁与回滚点

- 门禁 A（第 1 步后）：R1/R2 结论确认后才写生产代码。
- 门禁 B（第 5 步后）：Go 与 Node 检查全部通过后才改部署文件。
- 回滚点：第 6 步前代码对 Docker 部署无影响（缺省进程模式）；第 6 步后回滚只需恢复 3 个部署文件。

## 停止条件

- Chromium 输出不能通过现有 PDF 校验。
- Gotenberg 无法在无外网出口的网络中渲染自包含文档。
- 需要修改 `.env`、Mongo schema 或删除 named volume。
