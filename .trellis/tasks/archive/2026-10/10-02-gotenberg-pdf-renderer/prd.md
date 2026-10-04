# Gotenberg PDF 渲染容器开发与部署

## Goal

以 Gotenberg 容器替代 Docker 部署中的 wkhtmltopdf 进程渲染：新增实现 `PDFBackend` 的 Gotenberg HTTP 后端、启动配置与 Compose 服务，并完成真实笔记 PDF 下载验收。

## Background and confirmed facts

- 来源：`archive/2026-10/10-02-docker-login-functional-validation` 中 PDF 导出为 BLOCKED；修复任务 `10-02-docker-runtime-bug-fixes` 确认根因是 Mongo 管理配置缺少 `exportPdfBinPath`，不是渲染代码缺陷。
- wkhtmltopdf 上游已归档、内核为旧 QtWebKit；Docker 镜像目前固定安装 `wkhtmltopdf=0.12.6-2+b1`，并创建一个会被路径校验拒绝的 `/usr/local/bin/wkhtmltopdf` 符号链接。
- 渲染边界已存在：`app/application/content/pdf_renderer.go:25` 的 `PDFBackend`（`Descriptor` + `Render`）接收已净化、自包含的 HTML，返回 PDF 字节；超时（30s）、输出上限（64MB）与 `validatePDFArtifact` 由 `PDFRenderer` 统一执行。
- 当前唯一实现是 `app/service/contentpdf.ConfiguredBackend`，从管理配置 `exportPdfBinPath` 读取可执行文件；在 `app/service/content_runtime.go:135` 装配。
- 自包含文档只保留内置脚本 `window.status = "done"`，wkhtmltopdf 以 `--window-status done` 等待它。
- 发布形态有两种：Docker 镜像（Compose）与 Linux tar 包（`sh/package.sh` + `scripts/package-smoke.sh`，CI 安装 wkhtmltopdf）。
- 已确认 Docker 镜像移除 wkhtmltopdf、只使用 Gotenberg；Linux tar 包继续保留 wkhtmltopdf（2026-10-02）。

## Requirements

1. 新增 Gotenberg 渲染后端，实现现有 `PDFBackend` 契约；只用 Go 标准库 `net/http` 与 `mime/multipart`，不引入第三方 Gotenberg 客户端。
2. 渲染器选择由启动配置显式决定（`ProductionConfig` 为唯一交接）：`pdf.renderer=gotenberg` 时必须配置合法的 `pdf.gotenberg.url`；未配置 `pdf.renderer` 时保持现有 wkhtmltopdf 管理配置行为；未知值启动失败。Gotenberg 不可用时返回依赖错误，不回退到 wkhtmltopdf，也不探测选择。
3. Gotenberg 地址只来自部署配置，不允许通过管理后台或请求参数修改，避免 SSRF。
4. Compose 新增按 digest 固定的 `gotenberg` 服务：不发布宿主机端口；只接入 `internal: true` 的内部网络，没有外网出口；禁用 LibreOffice、PDF engines 与 webhook 路由；Chromium 只允许 Gotenberg 自身临时文件（`file:///tmp/`）与文档内联 `data:` 资源，拒绝其他所有 URL；配置健康检查，`leanote` 依赖其健康状态。
5. Docker 镜像移除 wkhtmltopdf 包和符号链接，`conf/app.conf-docker` 启用 Gotenberg 渲染器；tar 包发布形态继续使用 wkhtmltopdf，相关 CI 与 package smoke 不变。
6. 管理后台“Export PDF”页面在 Gotenberg 模式下显示当前渲染器为只读状态，并拒绝提交 wkhtmltopdf 路径；进程模式下保持现有行为。
7. `scripts/container-smoke.sh` 与 CI `container-smoke` 作业改为与 Gotenberg 容器一起运行，并通过真实导出路由证明能生成合法 PDF；保留失败诊断（应用日志、Gotenberg 日志、响应头）。
8. 渲染结果仍必须通过 `validatePDFArtifact`、超时与输出上限；中文正文必须可见（不能缺字或显示为方块）。

## Out of scope

- 不修改 `.env`、Mongo schema、用户数据，不执行 `docker compose down -v`。
- 不替换 tar 包形态的 wkhtmltopdf，不引入 chromedp/rod，不在 leanote 镜像内安装 Chromium。
- 不改变 PDF 文档净化规则、资源内联策略、鉴权或下载响应契约（文件名净化、`Content-Type`）。
- 不为 Gotenberg 启用 Basic Auth（网络隔离已提供边界；如后续需要跨主机部署再单独立项）。

## Acceptance Criteria

- [ ] Gotenberg 后端单元测试覆盖：multipart 请求形状（`index.html`、等待表达式、纸张/边距）、超时/取消、非 200 状态、响应体超限、非法 PDF、描述符变化，且后端只访问配置的地址。
- [ ] 配置测试覆盖：缺省保持进程模式、`gotenberg` 缺 URL 或 URL 非法（含 userinfo、query、非 http/https）启动失败、未知渲染器启动失败。
- [ ] Chromium 实际输出通过 `validatePDFArtifact`；若不通过，停止实现并回到设计，不得放宽校验。
- [ ] `docker compose config --quiet` 成功；`docker compose up -d --build` 后 `mongo`、`gotenberg` healthy，`leanote` running，`/healthz` 为 200 ready；`gotenberg` 没有发布宿主机端口。
- [ ] 证明 Gotenberg 无外网出口：从 `gotenberg` 容器内访问外部地址失败，且渲染包含外部 URL 的文档时不产生出站请求。
- [ ] 真实浏览器中从已登录笔记菜单“导出PDF”触发下载，文件以 `%PDF-` 开头、文件名为净化后的笔记标题，中文正文可见；普通笔记与 Markdown 笔记各一次。
- [ ] 管理后台在 Gotenberg 模式下显示只读渲染器状态，提交路径被拒绝。
- [ ] 镜像中不再包含 wkhtmltopdf；`container-smoke` 通过；tar 包 `package-smoke` 不受影响。
- [ ] `go test`（相关包）、`gofmt -l app cmd`、`go build ./...`、`go vet ./...`、`npm test`、`git diff --check`、Trellis validate 全部通过；未运行的真实环境项明确标为 `unrun`。

## Open questions and blocking gates

- **R1（blocking）**：2026-10-03 的 Gotenberg 探针已经确认 API、等待表达式、内联 `data:` 资源和传统 PDF 输出可用，但真实输出尚未通过 Leanote 现有 `PDFRenderer` 的 `validatePDFArtifact`。实现后必须在切换 Compose/CI 前完成该调用链验证；`qpdf --check` 或 `%PDF-` 不能替代它。
- **R2（已解决）**：`waitForExpression=window.status === "done"` 已在 2026-10-03 的真实容器探针中生效；页面约 2 秒设置状态时总耗时约 5.3 秒，不会一直等待到超时。
- **R3（已解决）**：deny-list 不能只放行 `file:///tmp/`，因为 Leanote 的自包含文档使用 `data:` 资源。采用并保留实测通过的 `^(?!file:///tmp/|data:).*`。
- **R4（实施风险）**：CI `container-smoke` 需要在同一 Docker 网络启动 Gotenberg，CI 运行时间与镜像拉取会增加。
- **最终运行证据（unrun）**：最终 Compose 拓扑/健康依赖、真实已登录普通与 Markdown 笔记下载、中文可见性、后台只读和拒绝 POST、CI `container-smoke`、镜像内容检查及 tar 包 `package-smoke` 尚未执行，不得在规划阶段宣称验收通过。
