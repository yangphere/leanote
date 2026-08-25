# MongoDB 官方驱动迁移（B）— 执行计划

## Global Constraints

- 驱动固定 `go.mongodb.org/mongo-driver/v2` v2.8.1。
- MongoDB 7.0–8.0 数据兼容；默认测试 8.0。
- 不改变 Schema、API、USN、所有权查询，不传播全链路 context。

### Task 1：为兼容层写失败测试

**Files:**
- Create: `app/db/mongo_compat_test.go`
- Create: `app/db/object_id_test.go`
- Test data: `mongodb_backup/leanote_install_data/`

- [ ] 写集成测试覆盖 Find/Sort/Skip/Limit/Select/One/All、Insert、Update/UpdateAll、Upsert、Remove/RemoveAll、Count、Distinct。
- [ ] 写排序测试证明 `-Count` 与多字段排序顺序不被 map 重排。
- [ ] 写 ObjectId 测试覆盖有效 hex、无效 hex 明确失败和 JSON 24 位小写 hex。
- [ ] 在包装层尚未存在时运行测试，确认因缺少本包 `Collection`/`Query` 实现而失败。

### Task 2：实现 client、collection 与 query

**Files:**
- Create: `app/db/mongo_client.go`
- Create: `app/db/mongo_collection.go`
- Create: `app/db/mongo_query.go`
- Create: `app/db/object_id.go`
- Modify: `app/db/Mgo.go`
- Modify: `conf/app.conf`、`conf/app.conf-default`

- [ ] 实现单例 client 生命周期、MongoDB ping、database/collection 初始化与显式 connect/operation timeout。
- [ ] 实现设计 §2 的全部已用查询/写入形状，确保 cursor 总被关闭。
- [ ] 统一记录 collection、操作、匹配数量和原始 error；保留启动失败为 fatal。
- [ ] 运行 `go test ./app/db/... -v`，先在 MongoDB 8.0 下通过。

### Task 3：迁移 BSON 模型与查询调用点

**Files:**
- Modify: `app/info/AlbumInfo.go`、`Api.go`、`AttachInfo.go`、`BlogInfo.go`、`Configinfo.go`、`EmailLogInfo.go`、`FileInfo.go`、`GroupInfo.go`、`NoteImage.go`、`NoteInfo.go`、`NotebookInfo.go`、`ReportInfo.go`、`SessionInfo.go`、`ShareNotebookNoteInfo.go`、`SuggestionInfo.go`、`TagInfo.go`、`ThemeInfo.go`、`TokenInfo.go`、`UserInfo.go`
- Modify: importing files under `app/controllers/`、`app/service/`、`app/lea/Util.go`

- [ ] 先机械替换 import/type 名，再逐个修复 ObjectId 构造与 hex 转换；不在调用点复制适配逻辑。
- [ ] 把 `app/service/init.go` 的 collection 参数改为 `*db.Collection`，所有全局 collection 名保持不变。
- [ ] 编译每个 package，确保生产代码中 `mgo` import 归零。
- [ ] 运行模型序列化测试和 Golden，确认 BSON/JSON 字段与键序不变。

### Task 4：权限、USN 与错误路径

**Files:** `app/db/Mgo.go`、`app/service/NoteService.go`、`NotebookService.go`、`TagService.go`、`ShareService.go`、`FileService.go`、`AlbumService.go`、`AttachService.go`、G 的 HTTP 用例

- [ ] 对每个 `...ByIdAndUserId` helper 写正/负用例，确认查询文档包含 `UserId`。
- [ ] 用 admin/demo 双用户回放 Share、File、Attach、Album 越权负例。
- [ ] 回放每个 mutation→`GetSync*` 成对用例，断言 USN 递增与 delta。
- [ ] 测试 Mongo 不可达、操作超时、duplicate key 与 no-document；确认日志可定位且结果不伪成功。

### Task 5：MongoDB 8 与 7 兼容验证

- [ ] 在 MongoDB 8.0 恢复原始 fixture，连续两次运行全套 Go/Golden/USN 测试。
- [ ] 在 MongoDB 7.0 容器恢复同一 fixture，完整运行一次并记录服务器版本与结果。
- [ ] 运行 `go build ./app/...`、`go vet ./app/...`、`npm test`、`git diff --check`。
- [ ] 运行 `rg 'gopkg.in/mgo.v2' app go.mod go.sum`，确认零生产命中。
- [ ] 复核仓库没有迁移脚本、双驱动开关或 service context 第二接口。

## Rollback Point

Schema 零变化使代码回滚可直接恢复 mgo 版本。若需要修改 fixture 或生产数据才能通过，设计即失败，停止而不是编写迁移脚本。
