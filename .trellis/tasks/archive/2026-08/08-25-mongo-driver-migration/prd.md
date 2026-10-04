# MongoDB 官方驱动迁移（B）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

用 `go.mongodb.org/mongo-driver/v2` v2.8.1 替换不可维护且无法对现代 MongoDB 执行 CRUD 的 `mgo.v2`，使 Leanote 无数据迁移地支持 MongoDB 7.0–8.0，同时保持 API、USN、BSON、用户所有权和错误可见性。

## Dependencies

- 依赖 `08-25-revel-1-1-upgrade`；G 的 Golden/USN/权限测试必须为绿。
- 与 C-b 不并行修改后端；驱动迁移完成并稳定后才迁出 Revel。

## Requirements

### R-B1 驱动与服务器支持

- 删除代码库（含生产代码与测试基础设施）对 `gopkg.in/mgo.v2` 及其 `bson` 包的依赖，使用 `go.mongodb.org/mongo-driver/v2` v2.8.1。
- 支持 MongoDB 7.0–8.0；CI 与默认本地 fixture 固定 MongoDB 8.0，并至少保留一次 MongoDB 7.0 完整回归证据。
- G 测试基础设施 `app/tests/harness/**`（含 `cmd/e2e`）直接以 `mgo.v2` 连接 MongoDB（`mgo.DialWithTimeout`、`mgo.ParseURL`、`FindId`、`UpdateId`），而 mgo 无法在 MongoDB 7.0/8.0 上执行命令；harness 对 MongoDB 的访问必须随本任务一并迁移到官方驱动。Golden 存储、归一化规则与 HTTP 断言保持不变，仅替换其数据库访问实现（含 `configuration_test.go` 中引用 `mgo.ParseURL` 的错误文案断言）。

### R-B2 数据访问兼容边界

- 保留 `app/db/Mgo.go` 暴露的 collection 全局变量名；在 `app/db` 内实现 `Collection`、`Query` 与连接生命周期包装。
- 兼容实际使用的 `Find`、`FindId`、`Sort`、`Skip`、`Limit`、`Select`、`One`、`All`、`Insert`、`Update`、`UpdateAll`、`Upsert`、`Remove`、`RemoveAll`、`Count`、`Distinct` 形状；`UpgradeService` 的 `DropIndex("UserId", "ToUserId", "NoteId")` 依赖 mgo 由键列表拼接索引名的语义，包装层需复制该推导并以集成测试核验目标索引名。
- 包装层是唯一驱动适配位置；不得在 service 中重复实现 BSON 转换、超时或错误分类。

### R-B3 数据与序列化契约

- collection 名、字段名、BSON tag、索引语义与现有数据保持兼容，不提供数据迁移脚本。
- ObjectId 继续输出 24 位小写 hex；非法 hex 在原本会失败的位置仍明确失败，不静默产生零值。零值 ObjectId 的 JSON 形态保持 mgo 的 `""`（非 `"000000000000000000000000"`），由 `lea.ObjectID` 的 MarshalJSON 保证。
- 已批准的驱动差异（登记于 `app/info/model_contract_generated_test.go` 头注）：mgo 对零值 ObjectId 字段的 marshal 会报错（17 个非 omitempty 字段从 `zeroMarshalError` 改为 `zeroPresent`）；驱动的非 inline 匿名结构字段恒用类型名做键（`UserAndBlogUrl.User` 键 `user`→`User`）。两者均无生产写入/读取路径依赖旧行为，模型契约测试已按新语义冻结。
- 用作数据库编码/解码目标的结构体必须为全部字段写显式 `bson` tag：官方驱动对无 tag 字段按小写 Go 字段名精确匹配，mgo 则原样保留 Go 字段名并有大小写不敏感回退。该规则由模型契约静态检查守护；纯 JSON 响应结构（如 `ApiNote`、`ApiUser`，不经数据库）不在此约束内。
- `/api/*` 归一化后的响应字节、键序、时间形态与信封保持不变。

### R-B4 所有权与 USN

- 所有 `...ByIdAndUserId`、Share 双用户路径和被重写的查询必须保留 `UserId` 条件。
- 所有 mutation 继续递增 USN，并出现在对应 `GetSync*` delta。

### R-B5 超时与错误

- 连接和每次数据库操作使用 `conf/app.conf` 中显式、可配置的超时；不允许无限等待。
- 本任务不传播 `context.Context` 到所有 controller/service 方法，也不创建第二套 service API；后续工作记录在 `docs/modernization-backlog.md#mod-001-请求上下文贯穿数据访问链`。
- 包装层必须记录包含操作名、collection 和原始 error 的失败；现有 bool/结果 API 可保持，但不得吞掉驱动错误或伪造成功。not-found 语义映射保持现状：`mongo.ErrNoDocuments` 对应 mgo 的 "not found"，在 `Err()` 与 `Get`/`GetByQ` 系列继续按既有 bool 语义处理（含单条 `Update` 无匹配返回 true 的幂等行为），同时以日志可定位；duplicate key、超时、网络错误按类型单独识别，不得与 no-documents 混淆或伪造成成功。

## Acceptance Criteria

- [ ] `rg 'gopkg.in/mgo.v2' app go.mod` 零命中（app 全树，含 `app/tests/harness`）；`go.sum` 允许且仅允许保留一条 mgo 的 `/go.mod` 图校验哈希（来自 `revel/modules → pongo2` 的上游 go.mod require，代码零依赖，包不可达）。
- [ ] MongoDB 8.0 下 `go build`、`go vet`、全部 Go 测试、Golden、USN 和双用户权限用例通过。
- [ ] MongoDB 7.0 下同一套测试至少完整通过一次并把版本与结果记录进任务检查产物。
- [ ] 旧 fixture 无转换即可恢复、登录、读取、写入和同步；仓库没有数据迁移脚本。
- [ ] `ApiNoteContent.NoteId/UserId` 仍匹配 `^[0-9a-f]{24}$`，不得变成 `$oid` 对象或 base64。
- [ ] 数据库不可达、操作超时、duplicate key 和无结果均产生可定位日志/错误，测试不出现“失败但返回成功”。
- [ ] 数据库编解码目标结构体全部字段带显式 `bson` tag，模型契约检查覆盖该规则并通过。
- [ ] CI go-replay 腿 fixture 从 mongo:5.0 切换为 mongo:8.0，且存在可追踪机制（workflow 输入或等价）复现 MongoDB 7.0 完整回归。
- [ ] 所有权负例证明用户不能读取或修改另一用户的笔记、分享、附件、文件或相册。
- [ ] `docs/modernization-backlog.md` 保留 MOD-001，且本任务没有新增全链路 context API。

## Out of Scope

- MongoDB Schema、collection、字段或索引设计优化。
- `context.Context` 全链路传播。
- 迁出 Revel、前端升级或业务功能变化。
- 为兼容 mgo 暂时保留双驱动运行模式。
