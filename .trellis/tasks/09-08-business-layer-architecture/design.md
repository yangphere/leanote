# 按业务逻辑分层的架构收敛 — 技术设计

## 1. Layered architecture

```text
Browser / external API / CLI clients
                │
        Presentation adapters
  Node/esbuild + templates + editor runtime
                │
        Interface adapters
 HTTP routes + registry + session + middleware
                │
        Application services
 identity | notes/workspace | content/media |
 sharing/publishing | admin/operations
                │
          Domain contracts
 info models + API envelopes + USN + ownership
                │
       Infrastructure boundaries
 MongoDB driver + filesystem + mail + PDF
```

`app/info` 保存跨层数据契约；`app/service` 保存业务编排；`app/db`、文件系统、邮件和 PDF 是基础设施；controller、`app/httpserver` 和前端脚本只做输入输出适配。共享规则必须只有一个事实来源。

生产启动配置的唯一事实来源是 `interface-http` 的 production-config seam（包括来源、secret/Mongo 校验、错误码和 bind/dial/healthz 时序）；application-admin 只消费该契约管理业务设置，delivery 只验证同一契约，不重新定义规则。

## 2. Dependency direction

- Domain 不依赖 application、HTTP、Mongo driver 或前端。
- Application 只通过明确接口/现有 service seam 使用基础设施，不解析 HTTP 或直接写 response。
- Infrastructure 不包含业务授权决策；查询必须接收并强制携带所有权条件。
- Interface adapters 负责绑定、认证、结果转换和路由，不复制服务规则。
- Presentation 只消费稳定 HTTP/模板/资源契约；生成资源只能由 manifest 构建。
- Delivery 只验证上述契约，不通过测试 helper 伪造成功。

## 3. Cross-layer invariants

- `/api/*` 方法、参数、状态码、JSON 信封、字段和 token 生命周期不变。
- 对 D-01 纳入统一同步语义的 notebook/tag delete，必须原子递增 USN、写入 tombstone 并由 sync 返回对应变化；旧差异只作基线对照，不能静默修正为历史通过。note-save 按 D-06 优先同事务，失败不消耗 USN/不返回成功；无事务时显式补偿并暴露 `partial_write`，失败不得伪造成功 envelope。
- 资源查询同时约束 `UserId`；not-found、无权限和 duplicate key 语义保持。
- 未编辑笔记不保存，实际编辑只允许已登记的 HTML 规范化。
- 生成资源稳定且 CI 重建零 diff；浏览器与发布 artifact 必须绑定同一 commit/run/attempt。

## 4. Migration order and rollback

1. `domain-contracts` 建立跨层回归和模型边界。
2. 五个 application 子任务可在领域契约完成后并行实现，但共享服务/数据契约由领域任务冻结；它们的验收只覆盖 service/adapter contract，不提前要求新 HTTP、真实页面或 container smoke。
3. `infrastructure-persistence` 收敛 MongoDB driver 与错误/超时。
4. `interface-http` 在应用服务和持久化边界稳定后迁移全部 controller、harness 和启动入口，并承担 production-config seam、真实 HTTP 路由和页面 adapter smoke。
5. `presentation-frontend` 维护生成链和编辑器/页面适配，可与 HTTP 的纯资源工作并行，但最终由交付层联验。
6. `delivery-verification` 汇合全部层，阻断缺失的真实服务、浏览器、container/PDF 或发布证据；它消费各层 contract，不把 application 任务的局部结果升级为跨层通过。

每个子任务独立提交和回滚；不保留双运行时、隐藏 fallback 或跨任务顺带清理。延期项只链接 backlog。
