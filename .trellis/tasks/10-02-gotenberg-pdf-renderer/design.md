# 技术设计：Gotenberg PDF 渲染容器

## 边界

- `app/application/content` 不变：`PDFRenderer` 继续负责文档序列化、超时、输出上限和 `validatePDFArtifact`。
- `app/service/contentpdf` 新增 `GotenbergBackend`，与 `ConfiguredBackend` 并列实现 `application.PDFBackend`。它只负责一次 HTTP 调用，不读取管理配置。
- `app/httpserver` 解析并校验渲染器配置，放入 `ProductionConfig`，再经 `service.ContentRoots` 旁的类型化字段传给 `InitContentRuntime`；`content_runtime.go` 按配置选择后端，不做运行时探测。
- 管理后台只读取当前渲染器类型用于展示，不能写入 Gotenberg 地址。

## 配置契约

```ini
pdf.renderer=gotenberg            # 缺省 = process（现有管理配置 exportPdfBinPath）
pdf.gotenberg.url=http://gotenberg:3000
```

- 类型：`service.PDFRendererConfig{Kind PDFRendererKind; GotenbergURL *url.URL}`。
- 校验（fail closed）：`Kind` 只能是空/`process`/`gotenberg`；`gotenberg` 要求 `http`/`https`、有 host、无 userinfo/query/fragment，path 为空或 `/`。错误码沿用 `configError` 风格。
- dev/test 段不配置时保持进程模式，现有 golden 与 harness 不受影响。

## GotenbergBackend

- `Descriptor(ctx)`：返回 `RendererDescriptor{PolicyID: "leanote-pdf-v1", ExecutableID: hex(sha256(policy + "gotenberg" + endpoint))[:32]}`。不在每次导出前访问网络；可用性由 Render 的错误反映。
- `Render(ctx, descriptor, document, max)`：
  1. 校验描述符与本后端一致，否则返回 `renderer_descriptor_changed`；`max <= 0` 返回验证错误。
  2. 调用 `application.ValidateSelfContainedPDFDocumentContext` 再次校验文档，与进程后端一致。
  3. 构造 multipart：`files` 字段的文件名固定为 `index.html`；表单字段 `waitForExpression=window.status === 'done'`、`printBackground=true`，纸张与边距显式设置（A4，边距与当前 wkhtmltopdf 默认输出接近，实施时对比确定）。
  4. `POST {base}/forms/chromium/convert/html`，使用请求 ctx；HTTP client 禁止重定向（`CheckRedirect` 返回错误）、不读取代理环境变量（`Proxy: nil`）。
  5. 非 200：读取至多 32KB 诊断写入错误链（不进入用户响应），返回 `renderer_process_failed`；连接失败返回依赖错误 `renderer_unavailable`；ctx 超时返回 `renderer_process_timeout`。
  6. 200：`io.LimitReader(body, max+1)` 读取，超限返回 `renderer_output_limit`。PDF 结构校验交给上层 `validatePDFArtifact`。
- 测试通过可注入的 `http.RoundTripper` 或 `httptest.Server`，不依赖 Docker。

## Compose 与网络

```yaml
gotenberg:
  image: docker.io/gotenberg/gotenberg:8.x@sha256:<实施时固定>
  command:
    - gotenberg
    - --api-timeout=40s
    - --libreoffice-disable-routes=true
    - --pdfengines-disable-routes=true
    - --webhook-disable=true
    - --chromium-deny-list=^(?!file:///tmp/).*   # 只允许 Gotenberg 自己的临时文件
  networks: [pdf]
  healthcheck: { test: ["CMD", "curl", "-fsS", "http://127.0.0.1:3000/health"] }
networks:
  pdf: { internal: true }
```

- `leanote` 同时接入 `default`（访问 Mongo 和外部资源）与 `pdf`；`gotenberg` 只接入 `pdf`，没有外网出口，也没有宿主机端口。
- `--api-timeout` 大于应用侧 30s 超时，确保由 leanote 的 ctx 先结束请求。
- deny-list 必须放行内联资源使用的 `data:` URL；deny-list 的确切正则、健康检查命令是否可用（镜像内是否有 curl）在实施第一步用真实镜像验证；验证结果写入 `research/gotenberg-facts.md`。

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

- R1：Chromium（Skia）输出的 PDF 交叉引用格式可能不符合 `validatePDFArtifact`（要求传统 `xref` 表）。实施第一步用真实 Gotenberg 输出验证；不通过则暂停并回到设计，禁止放宽校验。
- R2：`window.status` 等待表达式在 Chromium 中的行为与 wkhtmltopdf 不同；需在真实容器中确认不会一直等到超时。
- R3：CI `container-smoke` 需要在同一 Docker 网络启动 Gotenberg，CI 运行时间与镜像拉取会增加。
