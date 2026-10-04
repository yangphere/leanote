# Leanote 技术栈现代化（父任务）— PRD

## Goal

在不改变 Leanote 核心产品行为、公开 URL、API 数据语义和 MongoDB Schema 的前提下，把后端、前端构建、关键浏览器库与交付链迁移到 2026 年仍受支持、可复现和可持续维护的技术栈。

本父任务只协调契约与完成顺序；实现拆入可独立验收的子任务，避免一次性大爆炸升级。

## Confirmed Baseline

- 当前 `go.mod` 声明 Go 1.15；本机 Go 1.27 可编译，但 `go vet ./app/...` 已暴露 struct tag、不可达代码、未键控字面量、自赋值、格式化调用和 signal channel 等历史问题。
- Revel 1.0 仍能编译，但上游 release 停在 1.1；最终迁出到标准库 `net/http`。
- `mgo.v2` 使用 legacy MongoDB wire opcodes，不能连接当前受支持的 MongoDB；迁移到官方 `mongo-driver/v2` 是硬阻塞项。
- Gulp 3 流水线不能在本机 Node 24.19.0 正常安装；现有产物由 concat/minify、CSS、i18n 抓取和 `note-dev.html` 模板生成组成。
- 前端运行时为 jQuery 1.9、Bootstrap 3.2 和 TinyMCE 4；TinyMCE 实际使用的第一方插件是 `leaui_image`、`leaui_mindmap`、`leanote_nav`、`leanote_code`。
- 当前自动化安全网有限：JS 粘贴测试可运行，Go 集成测试依赖 MongoDB 与全局初始化，缺少现代 CI、容器及可复现发布。

版本与上游证据集中记录在 `research/external-facts.md`，架构选择记录在 `docs/adr/0001` 至 `0004`。

## Non-negotiable Invariants

- 公开页面 URL、`/api/*` 端点、method、query/form 参数、状态码、重定向、响应 JSON/文本语义保持兼容。
- API token 生命周期和认证行为保持；Web session 允许一次性重新登录，不兼容 Revel cookie 字节格式。
- MongoDB collection、字段名、BSON 类型、ObjectID 字符串表现、排序、分页与更新语义保持，不执行数据迁移。
- 用户未编辑笔记时不保存且 DB 中 HTML 字节不变；真实编辑保存后只允许已记录的非语义 HTML 归一化。
- 服务端渲染、现有模板组织和 TinyMCE 编辑器内核保留；本项目不是 SPA 重写。
- 生成前端资产继续纳入版本控制，但源码与 manifest 是唯一事实来源；CI 重建后必须零 diff。
- 错误要显式失败，不用静默 fallback、双运行时、永久兼容层或占位成功路径掩盖迁移问题。

## Task Tree and Dependencies

```text
08-25-tech-stack-modernization
├─ G  08-25-regression-baseline
├─ Backend track
│  └─ A   08-25-go-toolchain                 ← G
│     └─ C-a 08-25-revel-1-1-upgrade         ← A
│        └─ B   08-25-mongo-driver-migration ← C-a
│           └─ C-b 08-25-revel-migration     ← B
├─ Frontend track
│  └─ D 08-25-frontend-build-chain           ← G
│     └─ E 08-25-frontend-libs                ← D（协调收口，planning）
│        ├─ E-jQ 08-25-jquery-upgrade         [archive/completed]
│        ├─ E-BS 08-25-bootstrap-upgrade      [archive/completed]
│        └─ E-TM 08-25-tinymce-upgrade        [archive/completed]
└─ F 08-25-cicd-delivery                      ← C-b + E
```

上图保留历史 children 与原计划依赖，不能单独证明当前生命周期顺序。2026-09-01 的现场状态是：D 及三个 E child 已归档，E 仍为 `planning`；F 也已归档，但 F 的 notes 仍声称上游真实证据阻断。G 必须先完成；随后后端与前端轨道可并行。F “只在两个轨道全部完成后启动”是原计划约束，而不是对当前 F 归档时序的事实背书。Q-E1 等待模式允许受保护 workflow 在 E 归档前生成仅供 E 验收的 tag 预检 artifact，但该运行不得发布或改变 F 状态；F 正式发布仍位于 E 归档之后，不在任务图中增加反向边。父任务收口前必须登记该时序冲突，并分别核验 E 候选提交门禁与 F tag-bound 发布 artifact。

## Requirements

### R-G：回归基线

- 在旧实现上录制 HTTP golden，覆盖公开页面、认证页面和 `/api/*` 成功/失败路径；动态字段归一化但数组顺序、状态码、header 和业务内容严格比较。
- 固定 USN 语义测试，覆盖单调性、冲突、删除和同步边界。
- G 阶段使用 **MongoDB 5.0** fixture（旧 `mgo.v2` 只发送 5.1 起移除的 legacy opcode，
  旧实现无法在 7.0/8.0 上执行 CRUD；5.0 为 EOL，仅作旧实现基线的测试专用环境）；
  7.0/8.0 验证由 B 阶段驱动迁移后承接。本地 Docker 和 CI 执行相同恢复、录制、回放流程。
- 修整历史 `app/tests/`，消除包级初始化导致的无 Mongo panic，并证明目标测试真实被发现。

### R-A：Go 工具链与通用依赖

- `go.mod` 最低版本改为 1.26；CI 支持 Go 1.26 与 1.27。
- 逐一升级非 Revel、非 Mongo 的直接依赖，使用调用方与针对性测试证明兼容，不执行无边界的 `go get -u`。
- 分类别清理已确认的 vet 问题；新增模型 BSON/JSON 序列化契约测试，防止修 tag 时改变存储或 API。

### R-C-a：Revel 1.1 基线

- 只把 Revel runtime 升到 1.1.0、相关 cmd 依赖升到兼容版本，保持路由、filter、session、模板和错误行为。
- 验证项目现有三条执行路径：应用启动、测试/开发入口、打包入口；不在此任务提前迁出框架。

### R-B：MongoDB 官方驱动

- 用 `go.mongodb.org/mongo-driver/v2` 2.8.1 替换 `mgo.v2`，支持 MongoDB 7.0–8.0；本地和 CI 固定 8.0。
- 在 `app/db` 建立单一兼容边界，显式表达 Find/Sort/Skip/Limit/One/All/Count/Insert/Update/UpdateAll/Upsert/Remove/Select 等实际语义。
- ObjectID、BSON 标签、nil/空集合、not-found、duplicate key、排序分页和更新结果保持现有业务契约。
- 仅在 DB wrapper 内使用显式有界 timeout；本任务不贯穿 controller→service→db 的 context，后续工作记录为 `MOD-001`。
- 数据零迁移；Golden、USN 与 driver contract 在 MongoDB 7.0、8.0 均通过。

### R-C-b：迁出 Revel

- 使用标准库 `net/http` 与 `http.ServeMux`，以 `conf/routes` 为输入建立显式路由表。
- 只允许受限 controller/action registry 处理历史 catch-all，禁止任意字符串反射调用。
- 在明确边界重建参数绑定、结果转换、filter 顺序、模板、静态资源、session、CSRF/auth、错误与日志语义，随后删除 Revel 及 vendored `app/cmd`。
- Web session 切换可令用户一次性重新登录；API token 不变。
- 生产配置拒绝空值或公开默认 `app.secret`；新 cookie 默认 `HttpOnly=true`、`SameSite=Lax`，`Secure` 可配置。

### R-D：前端构建链

- 用 esbuild 与普通 Node 24 脚本替换 Gulp 3，lockfile 固定依赖；唯一支持 Node 24.x LTS。
- 完整复现 JS/CSS concat/minify、i18n、TinyMCE language、模板 `note-dev.html`→`note.html` 和所有生产路径。
- 以 manifest 明确源码→产物映射；相同输入连续构建字节稳定，CI 执行 `git diff --exit-code`。

### R-E：前端库协调

- `08-25-frontend-libs` 只对已归档的三个子任务做同一候选提交组合验收，不直接混合生产改动；children 关系用于历史追踪，不表示本轮仍需按顺序启动。
- jQuery 固定 3.7.1；`jquery-migrate` 仅开发诊断，生产无 migrate 且 warning 为零。
- Bootstrap 固定 5.3.8；迁移模板、插件和 `leaui_image` iframe，不做视觉重设计。
- TinyMCE 固定 8.8.2、自托管、显式 GPL；迁移四个实际插件和粘贴行为，核验后删除失效 `leaui_mind` 副本。
- `@playwright/test` 的 Chromium E2E 阻塞合并；当前及前一主版本 Chrome/Edge/Firefox/Safari 做发布前 smoke。

### R-F：CI/CD 与交付

- PR/push 运行 Go 1.26/1.27、MongoDB 8.0、Node 24、JS/build、Chromium E2E、生成物漂移、打包与容器 smoke。
- `v*` tag 创建 GitHub Release tarball + SHA-256，并推送 GHCR Linux/amd64 image；无生产自动部署。
- 镜像非 root、连接外部 MongoDB、声明持久化路径且包含真实可用的完整 PDF 功能。
- arm64/PDF 适配记录为 `MOD-002`，不以删减 PDF 功能换取多架构。

## Acceptance Criteria

- [ ] G 的 Golden、USN、页面 smoke 与测试夹具在旧实现上建立并可重复执行。
- [ ] Go 1.26/1.27 全部质量门通过，`go.mod` 最低 1.26，历史 vet 问题按批准范围清零或有精确、临时、可追踪的基线。
- [ ] 应用在 MongoDB 7.0 和 8.0 上读写相同数据，无迁移脚本且 driver contract/Golden/USN 全绿。
- [ ] Revel 从 module、import、运行入口和 vendored cmd 中清除；所有显式与 catch-all 路由均受 registry 限制并通过契约测试。
- [ ] Web session 安全默认值生效，一次性重新登录被记录；API token 和对外 API 契约不变。
- [ ] Node 24 下 `npm ci && npm run build && npm test` 通过，连续构建及 CI 重建零 diff。
- [ ] jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 各自独立验收，无 migrate、旧 runtime、双版本或编辑器内容损坏。
- [ ] Chromium E2E 阻塞通过，浏览器支持矩阵的发布前 smoke 有可审计记录。
- [ ] PR/push 质量门通过；测试 `v*` tag 产出可复验 tarball 和 GHCR Linux/amd64 镜像，且没有生产部署动作。
- [ ] E 的组合验收与 F 的发布验收分别满足各自 allowlist/provenance；E 不等待 tag 时使用受保护的
  `candidate-browser-matrix-v1`，F 发布使用两文件 `browser-release-matrix-v1`，两者均须校验按固定顺序的四个
  稳定 coverage ID、槽位摘要 digest（RFC 8785 JCS）和 run/attempt；不得把 F 在 E planning 时的归档状态改写为按
  DAG 顺序完成，也不得以 E 候选 SHA 冒充 F 的 tag commit/artifact。
- [ ] Q-E1 等待模式下，E 只消费严格 tag 指向候选 SHA 的受保护预检 artifact；该预检不创建 Release/GHCR，E 归档后 F
  必须在最终 release run 中重新生成并校验正式 `browser-release-matrix-v1`，保持 F 发布位于 E 之后且不形成任务依赖环。
- [ ] Docker 非 root、外置 MongoDB、上传持久化及真实 PDF smoke 通过。
- [ ] 所有延期结构性工作只记录在 `docs/modernization-backlog.md`，并从相关任务与 ADR 链接。

## Out of Scope

- 产品功能、视觉设计、URL/API 重新设计或 SPA/前后端分离重写。
- MongoDB Schema/数据迁移、批量历史 HTML 重写。
- 自动生产部署、生产凭据管理和环境编排。
- 首期 Linux/arm64 image。
- 在 Mongo 任务中完成全调用链 context 传播。
