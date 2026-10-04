# jQuery 3.7 规格审核整改 - 执行计划

## Preconditions

- 本任务保持 `planning`；不得执行 `task.py start`、修改业务实现、测试实现或 CI workflow。
- 保留用户现有的 `08-25-jquery-upgrade` 规划改动，仅在审核发现的契约缺口处做最小文档更新。

## 1. Establish Evidence

- [x] 核对 `conf/app.conf [test]`、harness 的 `leanote_test` 约束、build smoke 的变量验证与当前 workflow 的 E2E 调用范围。（2026-08-27 复核完成：workflow 为 dev mode + Mongo 5.0 且仅 build smoke；README/env up 恢复 `leanote_test`。）
- [x] 核对根、协调、jQuery 和 CI/CD 任务的浏览器矩阵声明，定位全部冲突表述。（复核完成：四处契约一致；仅 RD-4 表述滞后记录在案。）

## 2. Repair Upstream Planning Contract

主体条款已随 `6c299d0` 写入上游工件，以下为复核定性的剩余编辑：

- [x] 在 jQuery PRD/design/implement 中定义 run token、test-only identity、预检时点、权限和清理失败行为。（R-jQ3 / design 第 3 节 / implement 第 1 步已满足。）
- [x] 定义 `npm run test:e2e` 的 PR/push workflow 所有权、共享 harness 生命周期、独立脱敏摘要和无条件 cleanup。（R-jQ6 / design 第 5 节已满足。）
- [x] 定义真实四浏览器当前/前一版本 smoke 的记录字段和阻断条件。（R-jQ7 / AC-jQ9 已与根、E、F 对齐。）
- [x] 按 RD-1 确认值在上游 PRD/design 补写 marker 有效期显式阈值，并在上游 implement 红灯清单中登记过期分支断言。（2026-08-27 确认 `createdAt` + 2 小时，已补写三处。）
- [x] 按 RD-5 决策在上游 PRD/design 明确 member 流程执行账号与凭据契约。（2026-08-27 确认选项 (a)，已补写上游 PRD R-jQ3 与 design §3。）

## 3. Synchronize Parent Contracts

- [x] 更新 E 协调任务和 F CI/CD 设计，使它们不再将 Chromium 当作 Chrome/Edge 证据。（`6c299d0` 已同步，复核通过。）
- [x] 将 workflow、harness、test configuration 和 browser-smoke record 纳入 E-jQ 的文件范围与上下文清单。（2026-08-27 二次审查修正：relatedFiles 改为 canonical `tests/e2e/e2e-environment.mjs` 并补登 build smoke 与 reporter；两份 JSONL 补登已存在的 build smoke、reporter、playwright 配置与 task.json（helper 等 未来文件因 JSONL 存在性硬校验须在创建时登记）；notes 显式引用 R-jQ3/R-jQ6。）
- [x] 记录 RD-3 判定标准与 RD-4 滞后（已写入本任务 PRD：RD-3 判定标准并入 AC-1，RD-4 记录在案），无需改动 E/F 文件。

## 4. Planning Validation

- [x] 运行 `task.py validate` 检查两个任务的 JSONL。（2026-08-27 修复后复跑通过：8/5 与 11/11 条目。）
- [x] 解析修改的 `task.json`，运行 `git diff --check`，并复审整个规划 diff 是否未包含实现代码。（仅本任务三个规划文档变更，`git diff --check` 干净。）
- [x] 完成 PRD convergence pass；将结果连同 RD-1/RD-4/RD-5 待确认事项提交给用户审批，但不激活任务。（2026-08-27 用户确认三项采纳建议；RD-1/RD-5 已回填上游，RD-4 确认延后至 frontend-libs 启动时处理；上下游工件一致性复核通过。）
