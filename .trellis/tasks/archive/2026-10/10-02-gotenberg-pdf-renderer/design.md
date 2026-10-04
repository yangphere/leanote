# 技术设计：Gotenberg PDF 渲染容器

## 边界

- `app/application/content` 不变：`PDFRenderer` 继续负责文档序列化、超时、输出上限和 `validatePDFArtifact`。
- `app/service/contentpdf` 新增 `GotenbergBackend`，与 `ConfiguredBackend` 并列实现 `application.PDFBackend`。它只负责一次 HTTP 调用，不读取管理配置。
- `app/httpserver` 提供一个共享的渲染器配置解析/校验入口，供生产、dev、test 三种启动路径调用；它把结果放入 `ProductionConfig` 的唯一 `service.ContentRuntimeConfig` 交接，不保留平行的原始 key 或重复默认值。
- `service.ContentRuntimeConfig` 组合 `ContentRoots` 与 `PDFRendererConfig`；`cmd/leanote` 原样把这一个结构传给 `InitContentRuntime`，`content_runtime.go` 按显式配置选择后端，不做运行时探测或回退。
- 管理后台只读取运行时发布的渲染器类型用于展示和拒绝写入，不能读取或写入 Gotenberg 地址；进程模式仍使用既有管理配置。

## 配置契约

```ini
pdf.renderer=gotenberg            # 缺省 = process（现有管理配置 exportPdfBinPath）
pdf.gotenberg.url=http://gotenberg:3000
```

- 类型：`service.PDFRendererConfig{Kind PDFRendererKind; GotenbergURL *url.URL}`，由 `service.ContentRuntimeConfig{ContentRoots, PDFRenderer}` 统一承载。
- 校验（fail closed）：`Kind` 只能是空、`process` 或 `gotenberg`；`gotenberg` 必须配置 `http`/`https`、非空 host、无 userinfo/query/fragment，path 只能为空或 `/`。未知值、非法 URL、缺 URL 均启动失败，错误码沿用 `configError` 风格。
- `dev`/`test` 与 `prod` 共用同一解析规则；未配置 renderer 时均保持进程模式，现有 golden 与 harness 不受影响。只有部署配置能选择 Gotenberg，管理后台和请求参数不参与选择。

## GotenbergBackend

- `Descriptor(ctx)`：返回 `RendererDescriptor{PolicyID: "leanote-pdf-v1", ExecutableID: hex(sha256(policy + "gotenberg" + endpoint))[:32]}`。不在每次导出前访问网络；可用性由 Render 的错误反映。
- `Render(ctx, descriptor, document, max)`：
  1. 校验描述符与本后端一致，否则返回 `renderer_descriptor_changed`；`max <= 0` 返回验证错误。
  2. 调用 `application.ValidateSelfContainedPDFDocumentContext` 再次校验文档，与进程后端一致。
  3. 构造 multipart：`files` 字段的文件名固定为 `index.html`；表单字段固定为 `waitForExpression=window.status === "done"`、`printBackground=true`、A4（`paperWidth=21cm`、`paperHeight=29.7cm`）及四边 `1cm`（即 10mm）边距，以精确保持现有进程模式的页面尺寸基线。Gotenberg 支持 `in`/`pt`/`cm` 单位，本设计使用厘米避免英寸近似值。
  4. `POST {base}/forms/chromium/convert/html`，使用请求 ctx；HTTP client 禁止重定向（`CheckRedirect` 返回错误）、不读取代理环境变量（`Proxy: nil`）。
  5. 非 200：读取至多 32KB 诊断写入错误链（不进入用户响应），返回 `renderer_process_failed`；连接失败返回依赖错误 `renderer_unavailable`；ctx 超时返回 `renderer_process_timeout`。
  6. 200：`io.LimitReader(body, max+1)` 读取，超限返回 `renderer_output_limit`。PDF 结构校验交给上层 `validatePDFArtifact`。
- 真实集成测试必须把探针/真实容器返回的字节交给 `application.PDFRenderer.Render`，确认现有 `validatePDFArtifact` 成功；不能只调用后端或只运行 `qpdf`。
- 测试通过可注入的 `http.RoundTripper` 或 `httptest.Server`，不依赖 Docker。

## Compose 与网络

```yaml
gotenberg:
  image: docker.io/gotenberg/gotenberg:8.37.0@sha256:f29984bd1e226bf1b93ba90af06000afa8b315853e99d27b9aaa41b93f15c769
  command:
    - gotenberg
    - --api-timeout=40s
    - --libreoffice-disable-routes=true
    - --pdfengines-disable-routes=true
    - --webhook-disable=true
    - --chromium-deny-list=^(?!file:///tmp/|data:).*   # 允许 Gotenberg 临时文件与文档内联资源
  networks: [pdf]
  healthcheck: { test: ["CMD", "curl", "-fsS", "http://127.0.0.1:3000/health"] }
networks:
  pdf: { internal: true }
```

- `leanote` 同时接入 `default`（访问 Mongo 和外部资源）与 `pdf`；`gotenberg` 只接入 `pdf`，没有外网出口，也没有宿主机端口。
- `--api-timeout` 大于应用侧 30s 超时，确保由 leanote 的 ctx 先结束请求。
- deny-list 必须放行内联资源使用的 `data:` URL。已在 2026-10-03 的真实镜像探针中确认 `^(?!file:///tmp/|data:).*` 可用，镜像含 `/usr/bin/curl`；最终 Compose 仍需在实现后验证健康依赖和无外网出口。

## 镜像与发布形态

- Dockerfile 运行阶段移除 `wkhtmltopdf` 与符号链接；保留 `fontconfig`/`fonts-dejavu` 不影响（Gotenberg 镜像自带字体，中文可见性以真实下载验收为准）。
- `conf/app.conf-docker` 增加两行配置。tar 包使用的 `conf/app.conf-default` 不变。

## 管理后台

- `export_pdf.html` 根据渲染器类型显示：进程模式保持现有表单；Gotenberg 模式显示“PDF 由 Gotenberg 服务渲染（部署配置）”，不渲染表单。
- `ExportPdf` action 在 Gotenberg 模式返回 `admin.validation`（不写 `exportPdfBinPath`）。

## 兼容与回滚

- API 与 Web 导出路由、响应头、错误映射不变；客户端无感。
- 回滚：删除 `app.conf-docker` 中的 `pdf.renderer` 两行、恢复 Dockerfile 的 wkhtmltopdf 安装与 Compose 服务定义即可回到进程模式；不涉及数据迁移。

## 风险

- R1（blocking）：Chromium（Skia）输出的 PDF 交叉引用格式可能不符合 `validatePDFArtifact`（要求传统 `xref` 表）。探针的 `qpdf --check` 已通过，但完整 Leanote 校验仍未执行；接入后若不通过则暂停并回到设计，禁止放宽校验。
- R2（已验证）：`window.status` 等待表达式在 2026-10-03 的真实容器探针中生效；页面约 2 秒设置状态时总耗时约 5.3 秒。实现仍需保留真实调用链验收，防止装配或请求构造改变等待行为。
- R3（已验证）：Chromium deny-list 必须放行 `file:///tmp/` 与 `data:`；`^(?!file:///tmp/|data:).*` 已在真实镜像探针中验证，外部 URL 会被拦截。最终 Compose 仍需验证健康依赖和无外网出口。
- R4（实施风险）：CI `container-smoke` 需要在同一 Docker 网络启动 Gotenberg，CI 运行时间与镜像拉取会增加。
