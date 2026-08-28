# 执行计划

1. 重新扫描并锁定复审锚点，先补静态回归测试，再修改第一方 JS、Markdown、模板和清单归属。
2. 修复 diagnostics/business 的 cleanup、身份复核、AJAX route 注入和跨导航 rejection 收集；补齐 HTTP 状态与删除后列表断言。
3. 修复 harness server/supervisor、marker 精确删除、Windows Job Object 错误传播。
4. 修正 reporter 生命周期测试和 Firefox 栈正则；清理 implement/check 登记 warning。
5. 验证：针对性 Node/Go 测试；`npm test`；`npm run build` 两次并确认无漂移；`git diff HEAD --check`；`task.py validate 08-28-jquery-upgrade-review-repair`。Mongo/Revel 与真实浏览器仅在环境可用时执行并记录结果。

## 风险控制

- 生成资源只通过 build 管线更新；不直接编辑压缩输出。
- 共享 Mongo 服务/容器检查串行执行，命令超时按未知处理，必须获得正常退出后才报告通过。
- 任一身份、cleanup、supervisor 或 reporter 门禁失败即停止，不以日志或 fallback 掩盖。
