# 应用层：身份认证与会话 — 执行计划

本文件是规格审核后的实施顺序；当前审核阶段只修改任务材料，不执行以下业务代码步骤。

## 阶段 0：实施前门禁（规格决策已批准）

- [ ] 读取本任务 `prd.md`、`design.md`、`research/spec-audit-2026-09-09.md` 和领域任务交接材料。
- [x] U-01～U-05 已采用推荐决策：统一找回密码文案、注册事务+outbox、按用途并存 token、`demoUserId` 唯一事实来源、按 action 固化 API 方法和 principal `userId`。
- [ ] 先完成 `09-08-infrastructure-persistence` 的事务/补偿、outbox、TTL/index、token schema/旧文档兼容契约，并确认 owner、fixture 和 Mongo 7/8 证据；在该前置任务未完成前不得开始本任务业务编码。
- [ ] 将已批准决策同步到 `interface-http`/`delivery-verification` 任务；实现前确认 route method matrix、SessionWriter、Golden/replay 和真实环境汇合责任；`_ID` 稳定标识和 `_token/_userId` 回写按已归档迁移设计执行。
- [ ] 由 `task.py validate`、所有 `implement.jsonl/check.jsonl` 路径存在性检查和 `git diff --check` 确认任务材料完整。

## 阶段 1：服务契约与错误收敛

- [ ] 在 `AuthService`、`UserService`、`PwdService`、`SessionService`、`TokenService` 内收敛身份、输入、所有权、错误分类和时钟/存储 seam；service 绑定 `Vd` 精确规则并安全解析 ObjectId。
- [ ] 保留旧 MD5 与当前 hash 的密码验证；修复无条件成功、忽略 DB/副作用结果、无效 token 建 session、token 类型遗漏、并发可变过期时长和错误凭据吞 storage error。
- [ ] 按已决策方案实现注册事务/outbox/幂等补偿、token 独立文档与 `(UserId,Type)` 唯一约束，并为每个失败点增加 focused regression。

## 阶段 2：Web/API adapter contract

- [ ] 将 Web session 投影、API token resolver、demo policy 和 anonymous/member/admin principal 统一接入显式接口；SessionWriter Set/Delete/Encode/Commit 错误不得静默；service 不再依赖 Revel controller/session。
- [ ] Legacy Auth/User 和 first-party API Auth/User 只负责绑定、映射和响应；不复制业务校验或自行信任 `userId`。
- [ ] 保持既有 API envelope、字段顺序、Content-Type 和公开文案；按 action 固化允许方法，未列方法返回 405，显式 `userId` 必须与 principal 一致；所有变化先更新 Golden/fixture/兼容说明。

## 阶段 3：测试和证据

- [ ] 运行 service/httpserver/adapter contract、Golden normalization 和敏感信息扫描；覆盖 anonymous/source=none、有效、过期边界、action-token 类型错误、数据库错误、权限失败、跨用户、demo 配置错误和 SessionWriter 写失败。
- [ ] 运行 `go test ./app/service ./app/httpserver ./app/controllers/api ./app/controllers -count=1`、`go vet ./app/service`，必要时运行受支持的 Mongo fixture 测试；真实 HTTP/browser/release 证据不得在本任务伪造。
- [ ] 更新本任务 acceptance evidence matrix，记录命令、结果、fixture 路径和仍未闭合的下游证据。

## 验证命令

```text
python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-application-identity
git diff --check
go test ./app/service ./app/controllers/api ./app/controllers -count=1
go test ./app/httpserver -count=1
go vet ./app/service
```

Mongo、真实 HTTP、浏览器、邮件和发布验证必须使用各自任务规定的 harness/环境；环境不可用时记录 `partial`/`unknown`，不启用随机配置或静默 fallback。

## 回滚点与危险边界

- Auth/User 服务、Session/Token 存储、controller adapter 三个边界分别可回滚；不得回滚领域模型契约或删除用户/session/token 数据。
- 任何改变 token 主键、token 索引、Cookie 格式、API envelope、公开方法、权限白名单或注册副作用的改动，先更新 Golden/fixture、兼容说明和下游依赖，再实现；U-03 的旧文档兼容必须由 persistence 任务提供。
- 不修改 `conf/routes`、生成资源、前端文件、生产配置或其他 application 叶的业务代码；路由可达性和真实 HTTP 迁移由 `interface-http` 承接。
