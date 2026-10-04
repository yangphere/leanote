# Leanote 技术栈现代化（父任务）— 执行计划

## Global Constraints

- 本文件只编排子任务；实现必须在对应叶子任务中启动和验收。
- G 完成后，后端/前端轨道可并行；F 等两个轨道完成。
- 每个叶子任务单独提交、单独回滚，不跨任务偷带版本升级。
- 生产代码开始前必须重新获得用户批准并运行项目的开发前指南。

## Phase 0：确认规划与上下文

- [ ] 未获用户批准并运行 `task.py start` 前，父任务及 11 个子/孙任务均保持 `planning`；批准后仅当前 `task.py current` 指向的叶子可为 `in_progress`，父任务和未激活任务仍为 `planning`，依赖图与本文件一致。
- [ ] 每个任务的 PRD、复杂任务 design/implement、implement/check context manifest 可被 Trellis 校验。
- [ ] ADR、外部事实研究和 `docs/modernization-backlog.md` 从相关任务互链。
- [ ] 规划文档不存在模糊版本、占位符、未决产品边界或与父任务冲突的验收。

## Phase 1：G 回归基线

任务：`08-25-regression-baseline`

- [ ] 在旧实现上完成 HTTP golden、USN、MongoDB 5.0 fixture（旧 `mgo.v2` 只支持 legacy opcode，
      ≥5.1 无法 CRUD；7.0/8.0 验证由 B 阶段承接）、页面 smoke 和历史测试清理。
- [ ] 证明测试真实发现目标用例；无 Mongo 的聚焦单测跳过而不 panic。
- [ ] 产出后端与前端轨道共同复用的非 Revel 测试入口。

## Phase 2A：后端轨道

依次执行：

1. `08-25-go-toolchain`：Go 1.26 baseline + 1.26/1.27 矩阵、通用直接依赖、vet/序列化契约。
2. `08-25-revel-1-1-upgrade`：Revel 1.1 版本基线和三条执行路径。
3. `08-25-mongo-driver-migration`：官方 driver/v2 wrapper、MongoDB 7/8、零数据迁移；登记 `MOD-001`。
4. `08-25-revel-migration`：标准库 HTTP、显式路由/受限 registry、新 session 安全默认值、删除 Revel。

每步完成后运行 G 的 Golden/USN/smoke；B 与 C-b 不并行。

## Phase 2B：前端轨道

依次执行：

1. `08-25-frontend-build-chain`：Node 24、esbuild/Node scripts、manifest 和稳定生成物。
2. `08-25-frontend-libs` 协调下的 `jquery-upgrade` → `bootstrap-upgrade` → `tinymce-upgrade`。
3. 协调父任务汇总唯一版本、生产资源、Chromium E2E 和浏览器 smoke。

每步都运行 build、JS tests、Chromium 高风险流和 `git diff --exit-code`；三个库不并行。

## Phase 3：F 交付收口

任务：`08-25-cicd-delivery`

- [ ] PR/push 质量门覆盖 Go 1.26/1.27、MongoDB 8.0、Node 24、Chromium、漂移、tarball 与 container smoke。
- [ ] `v*` 测试 tag 产出 GitHub Release tarball/SHA-256 与 GHCR Linux/amd64 image。
- [ ] 非 root、外部 Mongo、上传持久化和完整 PDF 行为通过；登记并链接 `MOD-002`。
- [ ] 证明没有生产部署 job 或长期发布凭据。

## Parent-level Verification

```powershell
go version
go test ./app/...
go vet ./app/...
npm ci
npm run build
npm test
npm run test:e2e
git diff --exit-code
```

Mongo contract 另在 7.0 与 8.0 运行；CI 固定 8.0。最终还需真实服务回放 HTTP golden/USN、浏览器 smoke、tarball 解包启动、Docker 上传持久化和 PDF 输出。命令成功但发现零个目标测试不计为通过。

## Completion Gate

- [ ] 所有叶子任务验收完成并归档，协调父任务只保留汇总证据。
- [ ] 所有不变量有自动化或可审计 smoke 证据，无旧 runtime、双实现和隐藏 fallback。
- [ ] `docs/modernization-backlog.md` 的 owner、触发条件与完成定义完整，正文没有未登记技术债。
- [ ] 最终 diff、生成物、依赖图、许可证、发布权限和敏感数据边界复核通过。
