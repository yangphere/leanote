# 前端库现代化（E）— 协调执行计划

## Global Constraints

- 本协调任务不直接修改生产代码。
- 子任务严格按 jQuery → Bootstrap → TinyMCE 执行。
- 每个子任务均回放 G 的契约套件与 D 的生成资源门禁。

### Task 1：启动前核对

- [ ] 确认 `frontend-build-chain` 已完成，Node 24 下构建可重复、Gulp 已删除。
- [ ] 确认三个子任务的 PRD/design/implement 与上下文 manifest 均通过 Trellis 校验。
- [ ] 确认父任务和 task metadata 中的依赖顺序一致。

### Task 2：验收 jQuery 子任务

- [ ] 检查 jQuery 3.7.1、开发期 migrate 诊断结果和生产 bundle 无 migrate。
- [ ] 运行构建、JS 测试、Chromium E2E、Golden 与资源漂移检查。
- [ ] 只有全部通过后允许 Bootstrap 子任务进入实现。

### Task 3：验收 Bootstrap 子任务

- [ ] 检查模板 class/data attribute、事件 API、admin/member/blog/album 和 leaui_image 边界。
- [ ] 运行视觉/交互 smoke、Chromium E2E、Golden 与资源漂移检查。
- [ ] 只有全部通过后允许 TinyMCE 子任务进入实现。

### Task 4：验收 TinyMCE 子任务

- [ ] 检查 npm 自托管、第一方插件、粘贴、markdown 切换与 HTML 语义 fixture。
- [ ] 运行 Chromium E2E、全部 JS 测试、Golden 与资源漂移检查。

### Task 5：组合发布前门禁

- [ ] 从干净 checkout 执行 `npm ci && npm run build && npm test`，构建后零 diff。
- [ ] 运行 Go/Golden/USN/所有权套件，确认前端变更没有后端契约差异。
- [ ] 在 Chromium 运行完整 E2E，在 Firefox/Safari 运行发布 smoke 并记录版本与结果。
- [ ] 确认控制台无未处理异常、静态资源 404、migrate 警告。
- [ ] 确认所有生产代码变更都能归属到三个实现子任务之一。

## Completion Gate

三个子任务均完成且组合门禁全绿后，协调父任务才能归档并解除 `cicd-delivery` 的前端依赖。
