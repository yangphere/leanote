# MongoDB 官方驱动迁移（B）— 执行计划

## Global Constraints

- 驱动固定 `go.mongodb.org/mongo-driver/v2` v2.8.1。
- MongoDB 7.0–8.0 数据兼容；默认测试 8.0。
- 不改变 Schema、API、USN、所有权查询，不传播全链路 context。
- `app/tests/harness` 一并迁移到官方驱动；Golden 存储、归一化规则与 HTTP 断言不变。

### Task 1：为兼容层写失败测试

**Files:**
- Create: `app/db/mongo_compat_test.go`
- Create: `app/db/object_id_test.go`
- Test data: `mongodb_backup/leanote_install_data/`

- [x] 写集成测试覆盖 Find/Sort/Skip/Limit/Select/One/All、FindId、Insert、Update/UpdateAll、Upsert、Remove/RemoveAll、Count、Distinct、DropIndex（键名拼接推导的索引名与服务器现存索引一致）。
- [x] 写排序测试证明 `-Count` 与多字段排序顺序不被 map 重排。
- [x] 写 not-found 映射测试：`Find().One`、单条 `Update` 无匹配、`Remove` 无匹配的 bool 语义与现状一致。
- [x] 写 ObjectId 测试覆盖有效 hex、无效 hex 明确失败和 JSON 24 位小写 hex。
- [x] 在包装层尚未存在时运行测试，确认因缺少本包 `Collection`/`Query` 实现而失败。

### Task 2：实现 client、collection 与 query

**Files:**
- Create: `app/db/mongo_client.go`
- Create: `app/db/mongo_collection.go`
- Create: `app/db/mongo_query.go`
- Create: `app/db/object_id.go`
- Modify: `app/db/Mgo.go`
- Modify: `conf/app.conf`、`conf/app.conf-default`

- [x] 实现单例 client 生命周期、MongoDB ping、database/collection 初始化与显式 connect/operation timeout（`db.connectTimeoutMs`/`db.operationTimeoutMs`，缺省用默认值、非法值启动 fatal）。
- [x] 实现设计 §2 的全部已用查询/写入形状，确保 cursor 总被关闭；提供 e2e 运行记录受控读取入口（替换 TestE2eController 的 `Session.Copy()` 语义）。
- [x] 统一记录 collection、操作、匹配数量和原始 error；保留启动失败为 fatal。
- [x] 运行 `go test ./app/db/... -v`，先在 MongoDB 8.0 下通过。

### Task 3：迁移 G 测试基础设施（harness）

**Files:**
- Modify: `app/tests/harness/integration_test.go`
- Modify: `app/tests/harness/configuration_test.go`
- Modify: `app/tests/harness/cmd/e2e/main.go`

- [x] 用官方驱动替换 `mgo.DialWithTimeout`/`mgo.ParseURL`/`FindId`/`UpdateId`；URL→database 解析断言逻辑保持等价（`db.Init` 选择库与 fixture 库一致）。
- [x] 更新 `configuration_test.go` 中引用 `mgo.ParseURL` 的错误文案断言。
- [x] Golden 存储、归一化规则、HTTP 断言零 diff；在 MongoDB 8.0 下跑通 harness 全套。

### Task 4：迁移 BSON 模型与查询调用点

**Files:**
- Modify: `app/info/AlbumInfo.go`、`Api.go`、`AttachInfo.go`、`BlogInfo.go`、`Configinfo.go`、`EmailLogInfo.go`、`FileInfo.go`、`GroupInfo.go`、`NoteImage.go`、`NoteInfo.go`、`NotebookInfo.go`、`ReportInfo.go`、`SessionInfo.go`、`ShareNotebookNoteInfo.go`、`SuggestionInfo.go`、`TagInfo.go`、`ThemeInfo.go`、`TokenInfo.go`、`UserInfo.go`
- Modify: importing files under `app/controllers/`、`app/service/`、`app/lea/Util.go`
- Modify: `app/controllers/TestE2eController.go`、`app/service/UpgradeService.go`、`app/service/init.go`

- [x] 先机械替换 import/type 名（`bson.ObjectId`→`bson.ObjectID` 等），再逐个修复 ObjectId 构造与 hex 转换；不在调用点复制适配逻辑。
- [x] 把 `app/service/init.go` 的 collection 参数改为 `*db.Collection`，所有全局 collection 名保持不变。
- [x] TestE2eController 改用 db 包受控入口；UpgradeService 的 `DropIndex` 走包装层键名拼接语义。
- [x] 核查数据库编解码目标结构体无缺失 `bson` tag 的字段，模型契约静态检查通过。
- [x] 编译每个 package，确保生产代码中 `mgo` import 归零。
- [x] 运行模型序列化测试和 Golden，确认 BSON/JSON 字段与键序不变。

### Task 5：权限、USN 与错误路径

**Files:** `app/db/Mgo.go`、`app/service/NoteService.go`、`NotebookService.go`、`TagService.go`、`ShareService.go`、`FileService.go`、`AlbumService.go`、`AttachService.go`、G 的 HTTP 用例

- [x] 对每个 `...ByIdAndUserId` helper 写正/负用例，确认查询文档包含 `UserId`。
- [x] 用 admin/demo 双用户回放 Share、File、Attach、Album 越权负例。
- [x] 回放每个 mutation→`GetSync*` 成对用例，断言 USN 递增与 delta。
- [x] 测试 Mongo 不可达、操作超时、duplicate key 与 no-document；确认日志可定位且结果不伪成功。

### Task 6：MongoDB 8 与 7 兼容验证

- [x] 修改 `.github/workflows/regression-baseline.yml`：go-replay 腿 fixture 从 mongo:5.0 切换为 mongo:8.0，新增 `workflow_dispatch` 输入 `mongo_version`（默认 8.0，可选 7.0）。
- [x] 在 MongoDB 8.0 恢复原始 fixture，连续两次运行全套 Go/Golden/USN 测试。
- [x] 通过 workflow dispatch（或本地等价流程）在 MongoDB 7.0 容器恢复同一 fixture，完整运行一次并把服务器版本与 run 链接记录进任务检查产物。
- [x] 运行 `go build ./app/...`、`go vet ./app/...`、`npm test`、`git diff --check`。
- [x] 运行 `rg 'gopkg.in/mgo.v2' app go.mod go.sum`，确认零命中（含 `app/tests/harness`）。
- [x] 复核仓库没有迁移脚本、双驱动开关或 service context 第二接口。

## Rollback Point

Schema 零变化使代码回滚可直接恢复 mgo 版本。若需要修改 fixture 或生产数据才能通过，设计即失败，停止而不是编写迁移脚本。

## 完成记录（2026-08-29）

全部任务与验证项完成，证据见 `validation-evidence.md`：MongoDB 8.0.29 连续两次全套回放绿 + 7.0.40 一次全套绿；
`app go.mod` mgo 零命中（go.sum 仅存 pongo2 上游图校验哈希，见 PRD AC 例外说明）。
