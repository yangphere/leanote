# Leanote 技术栈现代化（父任务）— 技术设计

## 1. 设计原则

现代化沿稳定契约逐层替换实现：先固定观测，再建立窄兼容边界，最后删除旧依赖。父任务不允许把多个大迁移揉成一个提交或用临时双栈长期运行。

```text
HTTP / browser contract
        │
        ├── BaseController-compatible request/result boundary ── Revel → net/http
        │
        ├── service and domain behavior (preserved)
        │
        └── app/db compatibility boundary ── mgo → mongo-driver/v2

source assets ── manifest + Node/esbuild ── tracked generated assets
                                              │
                                              └── jQuery → Bootstrap → TinyMCE
```

两个轨道共享 G 的 golden/USN/smoke，只在 F 汇合。

## 2. 契约优先基线

G 从进程外 HTTP 和数据库结果观察行为，不依赖 Revel controller 测试 helper，确保 C-b 后同一套测试继续有效。归一化只处理 request id、cookie、时间戳等已枚举动态值；数组顺序、状态码、关键 header、业务字段和错误保持严格。

Mongo fixture 固定 8.0 并从可审计的最小数据恢复。每次测试隔离用户/笔记本/笔记 ID，避免用共享 mutable fixture 制造顺序依赖。

## 3. 后端轨道

### 3.1 Go 与依赖

先把 language/toolchain baseline 提到 Go 1.26，在 1.26/1.27 矩阵中逐个升级非 Revel/非 Mongo 直接依赖。Revel 和 Mongo 由专属任务拥有，防止依赖升级把行为迁移混入工具链任务。

修改 BSON/JSON struct tag 前以序列化测试锁定名称、omitempty、空值与 ObjectID 表现。vet 问题按类别提交，每类都有针对性验证。

### 3.2 Revel 1.1 过渡

C-a 是版本基线，不是兼容层设计阶段。它验证启动、开发/测试和打包三条路径，使后续 Mongo 与 HTTP 迁移不再同时背负框架版本差异。

### 3.3 Mongo wrapper

`app/db` 是唯一驱动语义收敛点：client/collection/query/ObjectID 四类职责分开，业务 service 不直接持有官方 driver collection。wrapper 只实现仓库实用到的查询形态，显式映射 sort/skip/limit/projection、not-found、duplicate key、upsert 和批量更新结果。

每次 DB 操作在 wrapper 内创建有界 context 并立即 cancel；timeout 按连接/普通读写/长查询命名配置。controller→service→db context 传播不伪装为本任务已解决，记录到 `MOD-001`。

### 3.4 标准库 HTTP 层

`app/httpserver` 负责路由、middleware、参数/响应 adapter、session 与静态资源，`cmd/leanote/main.go` 只装配依赖并启动服务器。

`conf/routes` 中的显式路由生成/维护为类型明确的 route table。历史 `/:controller/:action` 与 `/api/:controller/:action` 只能查找受限 registry；未注册组合返回明确 404/405，不反射任意导出方法。middleware 顺序固定为 request id/logging → recovery → security headers → session → CSRF/auth → handler → response logging。

新 session cookie 不兼容旧 Revel 签名，部署说明要求一次性重新登录。API token 的读取与验证保持在现有业务边界。生产启动校验拒绝空或公开默认 secret，cookie 安全属性在一个配置模块内生成。

## 4. 前端轨道

### 4.1 可复现构建

Node 24 + esbuild 只处理旧资产所需能力，不把应用强制改成 ESM。`scripts/build/manifest.mjs` 是源码→产物唯一清单，专用脚本分别处理 JS、CSS、i18n、`note.html` 和索引。生成物保留跟踪，构建测试检查完整性和稳定性。

### 4.2 顺序升级库

三个库严格顺序执行：

1. jQuery 3.7.1：用 migrate 找问题，修完后从生产完全移除。
2. Bootstrap 5.3.8：迁移 markup/plugin API 和 `leaui_image` iframe，保留视觉与交互语义。
3. TinyMCE 8.8.2：自托管 npm runtime，迁移插件、粘贴及持久化契约。

顺序避免同一 DOM/插件故障同时存在三种可能根因。协调父任务只汇总版本唯一性、浏览器矩阵和整体验收，不重复修改生产代码。

### 4.3 编辑器内容

TinyMCE 测试把“只读打开”和“实际编辑”分开。只读路径不得保存，DB 字节严格相同；编辑路径解析 DOM 后只归一空白与属性顺序，文本、链接、图片、代码和 Leanote 插件标记严格比较。`leaui_mindmap` 是实际入口；`leaui_mind` 经引用/行为证明后移除。

## 5. 浏览器与 E2E

`@playwright/test` 的 Chromium 运行在每个 PR 并阻塞合并，覆盖认证、笔记 CRUD、同步、上传、对话框、admin/member/blog/album 和编辑器高风险流。Chrome/Edge/Firefox/Safari 当前及前一主版本做发布前 smoke；真实 Safari 结果单独记录，不把 WebKit 仿真表述为 Safari 验证。

## 6. CI/CD 与发布

PR/push workflow 并行运行 Go 矩阵、Mongo 8 集成、Node/build、Chromium、package 和 container smoke。tag workflow 只匹配 `v*`，复验同一 commit 后生成可复现 tarball/SHA-256 与 GHCR image。

Docker 使用 Node/Go/运行时多阶段构建，最终为非 root Linux/amd64，外接 MongoDB，显式持久化上传/文件路径并携带 PDF 运行依赖。真实 PDF smoke 是阻塞项。流程不持有生产部署职责。

## 7. 决策与技术债

- 任务 DAG 与契约优先策略：`docs/adr/0001-stage-modernization-as-contract-first-dag.md`。
- Mongo/Revel 兼容边界：`docs/adr/0002-replace-mgo-and-revel-behind-compatibility-boundaries.md`。
- 前端生成物与顺序升级：`docs/adr/0003-modernize-frontend-with-generated-asset-contract.md`。
- Release/GHCR、无自动部署：`docs/adr/0004-publish-versioned-artifacts-without-production-deploy.md`。
- 允许延期且必须后续重构的项目只进入 `docs/modernization-backlog.md`；当前为 context 传播 `MOD-001` 与 arm64/PDF `MOD-002`。

## 8. 回滚策略

每个叶子任务独立提交并通过父级契约门。单个任务失败只回退该迁移单元；不回退已通过且与故障无关的前序护栏。B 与 C-b 不并行、三个前端库不并行，避免回滚边界互相污染。
