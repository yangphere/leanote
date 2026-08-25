# MongoDB 官方驱动迁移（B）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

用 `go.mongodb.org/mongo-driver/v2` v2.8.1 替换不可维护且无法对现代 MongoDB 执行 CRUD 的 `mgo.v2`，使 Leanote 无数据迁移地支持 MongoDB 7.0–8.0，同时保持 API、USN、BSON、用户所有权和错误可见性。

## Dependencies

- 依赖 `08-25-revel-1-1-upgrade`；G 的 Golden/USN/权限测试必须为绿。
- 与 C-b 不并行修改后端；驱动迁移完成并稳定后才迁出 Revel。

## Requirements

### R-B1 驱动与服务器支持

- 删除生产代码对 `gopkg.in/mgo.v2` 及其 `bson` 包的依赖，使用 `go.mongodb.org/mongo-driver/v2` v2.8.1。
- 支持 MongoDB 7.0–8.0；CI 与默认本地 fixture 固定 MongoDB 8.0，并至少保留一次 MongoDB 7.0 完整回归证据。

### R-B2 数据访问兼容边界

- 保留 `app/db/Mgo.go` 暴露的 collection 全局变量名；在 `app/db` 内实现 `Collection`、`Query` 与连接生命周期包装。
- 兼容实际使用的 `Find`、`Sort`、`Skip`、`Limit`、`Select`、`One`、`All`、`Insert`、`Update`、`UpdateAll`、`Upsert`、`Remove`、`RemoveAll`、`Count`、`Distinct` 形状。
- 包装层是唯一驱动适配位置；不得在 service 中重复实现 BSON 转换、超时或错误分类。

### R-B3 数据与序列化契约

- collection 名、字段名、BSON tag、索引语义与现有数据保持兼容，不提供数据迁移脚本。
- ObjectId 继续输出 24 位小写 hex；非法 hex 在原本会失败的位置仍明确失败，不静默产生零值。
- `/api/*` 归一化后的响应字节、键序、时间形态与信封保持不变。

### R-B4 所有权与 USN

- 所有 `...ByIdAndUserId`、Share 双用户路径和被重写的查询必须保留 `UserId` 条件。
- 所有 mutation 继续递增 USN，并出现在对应 `GetSync*` delta。

### R-B5 超时与错误

- 连接和每次数据库操作使用 `conf/app.conf` 中显式、可配置的超时；不允许无限等待。
- 本任务不传播 `context.Context` 到所有 controller/service 方法，也不创建第二套 service API；后续工作记录在 `docs/modernization-backlog.md#mod-001-请求上下文贯穿数据访问链`。
- 包装层必须记录包含操作名、collection 和原始 error 的失败；现有 bool/结果 API 可保持，但不得吞掉驱动错误或伪造成功。

## Acceptance Criteria

- [ ] `rg 'gopkg.in/mgo.v2' app go.mod go.sum` 零生产命中。
- [ ] MongoDB 8.0 下 `go build`、`go vet`、全部 Go 测试、Golden、USN 和双用户权限用例通过。
- [ ] MongoDB 7.0 下同一套测试至少完整通过一次并把版本与结果记录进任务检查产物。
- [ ] 旧 fixture 无转换即可恢复、登录、读取、写入和同步；仓库没有数据迁移脚本。
- [ ] `ApiNoteContent.NoteId/UserId` 仍匹配 `^[0-9a-f]{24}$`，不得变成 `$oid` 对象或 base64。
- [ ] 数据库不可达、操作超时、duplicate key 和无结果均产生可定位日志/错误，测试不出现“失败但返回成功”。
- [ ] 所有权负例证明用户不能读取或修改另一用户的笔记、分享、附件、文件或相册。
- [ ] `docs/modernization-backlog.md` 保留 MOD-001，且本任务没有新增全链路 context API。

## Out of Scope

- MongoDB Schema、collection、字段或索引设计优化。
- `context.Context` 全链路传播。
- 迁出 Revel、前端升级或业务功能变化。
- 为兼容 mgo 暂时保留双驱动运行模式。
