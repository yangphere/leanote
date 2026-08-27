# jQuery 3.7 规格审核整改 - 执行计划

## Preconditions

- 本任务保持 `planning`；不得执行 `task.py start`、修改业务实现、测试实现或 CI workflow。
- 保留用户现有的 `08-25-jquery-upgrade` 规划改动，仅在审核发现的契约缺口处做最小文档更新。

## 1. Establish Evidence

- [ ] 核对 `conf/app.conf [test]`、harness 的 `leanote_test` 约束、build smoke 的变量验证与当前 workflow 的 E2E 调用范围。
- [ ] 核对根、协调、jQuery 和 CI/CD 任务的浏览器矩阵声明，定位全部冲突表述。

## 2. Repair Upstream Planning Contract

- [ ] 在 jQuery PRD/design/implement 中定义 run token、test-only identity、预检时点、权限和清理失败行为。
- [ ] 定义 `npm run test:e2e` 的 PR/push workflow 所有权、共享 harness 生命周期、独立脱敏摘要和无条件 cleanup。
- [ ] 定义真实四浏览器当前/前一版本 smoke 的记录字段和阻断条件。

## 3. Synchronize Parent Contracts

- [ ] 更新 E 协调任务和 F CI/CD 设计，使它们不再将 Chromium 当作 Chrome/Edge 证据。
- [ ] 将 workflow、harness、test configuration 和 browser-smoke record 纳入 E-jQ 的文件范围与上下文清单。

## 4. Planning Validation

- [ ] 运行 `task.py validate` 检查两个任务的 JSONL。
- [ ] 解析修改的 `task.json`，运行 `git diff --check`，并复审整个规划 diff 是否未包含实现代码。
- [ ] 完成 PRD convergence pass；将结果提交给用户审批，但不激活任务。
