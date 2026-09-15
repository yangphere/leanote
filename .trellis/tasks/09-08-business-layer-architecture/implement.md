# 按业务逻辑分层的架构收敛 — 执行计划

## Phase 0：规划门

- [ ] 复核本父任务和全部子任务的 PRD/design/implement、依赖和上下文 manifest。
- [ ] 仅在用户明确批准最终规划摘要后激活一个子任务；父任务继续保持 planning。

## Phase 1：领域契约

- [ ] 完成 `domain-contracts`：模型、API envelope、ObjectID/BSON、USN、所有权、HTML 和 Golden/USN 入口。
- [ ] 运行模型契约、静态来源扫描和现有回归；记录既有缺陷，不改变基线。

## Phase 2：应用业务层

可并行但必须分别验收：

- [ ] `application-identity`：用户、密码、Session、Token、认证授权。
- [ ] `application-notes`：笔记、笔记本、标签、回收站、历史、同步，以及 note-specific asset receipt/idempotency adapter；Suggestion/feedback 交给 admin。
- [ ] `application-content`：通用文件根/路径安全/publish/verify primitive、附件、图片、相册、PDF、上传，并验收 notes adapter 的消费合同。
- [ ] `application-publishing`：分享、博客、评论、群组、主题、预览。
- [ ] `application-admin`：管理端、配置、升级、邮件、运维审计。

每个任务都要验证所有权查询、由 D-01 决定的 notebook/tag delete 新 USN tombstone/sync 配对、错误 envelope、幂等/冲突和跨服务调用；旧差异作为基线对照，不把授权或 mutation 逻辑复制到 controller。边界检查针对纯 application contract 和 controller collection/client，不把允许的 `app/service → app/db` 与 BSON map 误报为约 190 处全库迁移门禁。

## Phase 3：基础设施和接口适配

- [ ] `infrastructure-persistence`：完成 MongoDB driver 单一边界、查询/更新语义、超时、错误和 7/8 验证。
- [ ] `interface-http`：迁移完整路由与 controller registry，补齐参数/session/template/i18n/middleware，移植 harness，删除 Revel 依赖和旧启动链。

## Phase 4：呈现与交付

- [ ] `presentation-frontend`：Node 24/esbuild manifest、生成物、核心库、编辑器、模板和页面回归。
- [ ] `delivery-verification`：跨层 Golden/USN、真实服务、浏览器 8 槽、Docker/PDF、生产配置、tarball/GHCR 证据。

## Required validation

```powershell
python ./.trellis/scripts/task.py validate <task>
go test ./...
go vet ./...
go mod verify
npm ci
npm run build
npm test
git diff --check
```

需要 MongoDB、Docker、真实浏览器或 GitHub/GHCR 的检查必须保留原始运行、发现/执行数量、commit、run/attempt 和脱敏失败信息；环境缺失只能标为 blocked/未运行。

## Rollback points

- 每个 application 任务以其领域 Golden/USN/权限回归为边界。
- 持久化任务以 driver contract 和 MongoDB 7/8 回归为边界。
- HTTP 任务只有在完整 registry/harness/路由验证后删除 Revel；失败时回滚 HTTP 适配提交，不恢复静默双栈。
- 发布任务在 artifact 校验通过前不得创建 Release 或推送 GHCR。
