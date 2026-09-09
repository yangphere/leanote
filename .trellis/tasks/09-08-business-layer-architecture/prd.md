# 按业务逻辑分层的架构收敛 — PRD

## Goal

把当前 Leanote 的混合旧/新实现整理为可独立验收的业务逻辑分层：领域契约 → 应用服务 → 基础设施与接口适配 → 前端呈现 → 交付验证。迁移过程中保持既有产品行为、公开 URL、`/api/*`、USN、用户所有权、MongoDB Schema 和笔记 HTML 语义兼容。

本父任务只负责任务地图、共享不变量、依赖顺序和跨层验收；生产代码由子任务分别实现。

## Confirmed repository facts

- `CONTEXT.md` 已定义外部 API、USN、用户所有权、Golden、源码资源和生成资源等稳定术语。
- `app/info` 是模型与响应结构，`app/service` 是业务服务，`app/db` 是 MongoDB 边界，`app/controllers` 仍包含完整旧业务入口，`app/httpserver` 目前只有部分新 HTTP 适配。
- Go 1.26、MongoDB 官方驱动、Node 24/esbuild、jQuery 3.7.1、Bootstrap 5.3.8 和 TinyMCE 8.8.2 已有实现或契约测试，但完整 HTTP 切换和发布证据仍未闭合。
- 外部兼容性不变量以现有 Golden、USN 和浏览器测试为准；发现既有缺陷时必须单独建任务，不在分层迁移中隐式改变业务语义。

## Layer map and requirements

### Domain layer

- 以 `app/info` 和现有测试为唯一模型事实来源，冻结 API 信封、ObjectID、BSON/JSON 标签、空值、错误、USN、所有权和 HTML 语义。
- 领域契约不得依赖 Revel、HTTP writer、Mongo collection 或浏览器实现。

### Application layer

- 身份认证：用户、密码、Session、Token、登录/注册和认证授权边界。
- 笔记工作区：笔记、笔记本、标签、回收站、内容历史、建议和同步 USN。
- 内容媒体：文件、附件、图片、相册、PDF 和上传持久化。
- 分享发布：分享、博客、评论、群组、主题和预览。
- 管理运维：管理端、配置、升级、邮件和运维审计。
- 每个领域服务必须保留用户所有权条件、USN mutation/sync 配对和失败语义；按已确认 D-01，notebook/tag 删除分配新 USN、写入 tombstone 并进入 sync，旧差异仅作基线对照。note-save 的原子 USN、同事务/显式补偿、部分写入状态和重试幂等性遵循 D-06；不得在 controller 中复制业务规则。

### Infrastructure and interface layers

- `app/db` 负责 MongoDB driver、查询/更新结果、ObjectID、BSON、超时和错误映射，业务层不直接持有 driver collection。
- HTTP 适配负责路由、参数绑定、Session、模板、i18n、鉴权、中间件和完整 controller registry；完成后不再依赖 Revel。

### Presentation and delivery layers

- Node 24/esbuild manifest 是前端源码到生成资源的唯一事实来源；jQuery/Bootstrap/TinyMCE 版本唯一且生成物零漂移。
- 交付层负责跨层 Golden、USN、真实浏览器、MongoDB、Docker/PDF、tarball 和 GHCR 验收，不把局部测试通过升级为发布批准。

## Acceptance criteria

- [ ] 领域契约任务冻结并通过模型/API/USN/所有权/Schema/HTML 回归测试。
- [ ] 五个应用领域任务各自完成服务边界、调用方迁移、错误和权限回归；无 controller 级重复业务规则。
- [ ] 持久化边界完成 MongoDB 7/8 兼容、查询语义和超时/错误契约；业务服务不直接依赖 driver 类型。
- [ ] HTTP 适配完成全部公开路由、catch-all registry、参数/session/template/middleware 和旧入口清扫；生产源码、harness、启动与打包不再使用 Revel。
- [ ] 前端构建与编辑器资源在 Node 24 下可复现，核心库契约、生成物漂移和业务页面回归通过。
- [ ] 交付层完成质量门、8 槽真实浏览器矩阵、生产配置、容器非 root、外置 Mongo、完整 PDF、tarball 和 GHCR 证据；任一缺失保持 blocked。
- [ ] 所有延期工作写入 `docs/modernization-backlog.md`，不得在子任务中隐藏 fallback 或改变范围。

## Out of scope

- 新产品功能、公开 URL/API 重新设计、SPA 重写和 MongoDB Schema/数据迁移。
- 不在本父任务中自动部署生产或写入真实凭据。
- ARM64/PDF 渲染器替换、全调用链 `context.Context`、消息配置解析器替换继续作为独立延期任务，除非另行批准。

## Planning boundary

本轮只创建和完善 Trellis 任务，不执行 `task.py start`，不修改业务实现。每个子任务在独立评审并获得后续实现批准后才能激活。
