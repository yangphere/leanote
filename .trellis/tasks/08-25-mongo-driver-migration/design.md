# MongoDB 官方驱动迁移（B）— 技术设计

## 1. 包边界

`app/db` 继续是唯一数据访问基础设施层。建议文件职责：

- `app/db/Mgo.go`：保留全局 collection 名与现有高层 helper，改用本包类型。
- `app/db/mongo_client.go`：连接、ping、关闭、数据库选择、连接超时。
- `app/db/mongo_collection.go`：写操作、Count、Distinct 与统一错误记录。
- `app/db/mongo_query.go`：`Find` 后的 Sort/Skip/Limit/Select/One/All 链。
- `app/db/object_id.go`：ObjectId 解析、hex 与失败语义收敛。
- `app/db/mongo_compat_test.go`：包装形状与 MongoDB 7/8 集成测试。

service 继续调用 `db.Notes.Find(...).Sort(...).All(...)` 等形状，但具体驱动类型不再泄露。

补充边界：

- `app/controllers/TestE2eController.go` 是唯一在 `app/db` 之外使用 `db.Session` 的调用点（`Session.Copy()` + 参数化库名读 `e2e_runs`）。`app/db` 提供受控的 e2e 运行记录读取入口替换它；迁移后 service/controller 层不再出现 session/copy 语义。
- harness 迁移范围：`app/tests/harness/integration_test.go`（`mgo.DialWithTimeout`、`mgo.ParseURL`、`FindId`）、`app/tests/harness/configuration_test.go`（引用 `mgo.ParseURL` 的错误文案断言随新实现更新）、`app/tests/harness/cmd/e2e/main.go`（`mgo.DialWithTimeout`、`UpdateId`）。Golden 存储与 HTTP 断言零改动。

## 2. Query 语义映射

| mgo 形状 | 官方驱动实现 |
|---|---|
| `Find(q).One(out)` | `FindOne` + decode，保留 no-document 语义 |
| `FindId(id).One(out)` | `FindOne({_id: id})` + decode，保留 no-document 语义 |
| `Find(q).Sort(...).Skip(n).Limit(n).All(out)` | `FindOptions` + cursor 全量 decode + 始终关闭 cursor |
| `Select(fields)` | projection 文档 |
| `Insert(v...)` | 单个 `InsertOne` 或多个 `InsertMany`，错误完整返回到包装层 |
| `Update` / `Remove` | `UpdateOne` / `DeleteOne` |
| `UpdateAll` / `RemoveAll` | `UpdateMany` / `DeleteMany`，保留匹配数量日志 |
| `Upsert` | `UpdateOne` + `SetUpsert(true)` |
| `Count` / `Distinct` | `CountDocuments` / `Distinct` |
| `DropIndex(keys...)` | `Indexes().DropOne(strings.Join(keys, "_"))`，键名拼接规则与 mgo 一致 |

排序字符串 `-Count` / `Usn` 由一个解析函数转换为有序 BSON 文档，禁止用 map 表示排序以免丢顺序。

## 3. BSON 与 ObjectId

模型字段统一改为 `lea.ObjectID`（见 §3.1），查询文档统一使用官方 `bson.M`/`bson.D`。凡原来调用 `bson.ObjectIdHex` 的路径必须改用 `db.MustObjectIDFromHex` 或返回 error 的显式解析函数；选择依据是原调用点当前失败语义，禁止把无效 ID 变成零 ObjectId。

Golden 负责 HTTP JSON 形态，模型序列化测试负责 BSON 字段名。任何字段名变化都视为迁移失败，而不是新驱动的正常差异。

字段名解码差异：官方驱动对无 `bson` tag 的字段使用小写 Go 字段名且精确匹配，mgo 则原样使用 Go 字段名并有大小写不敏感回退。因此凡作为数据库编解码目标的结构体，全部字段必须有显式 `bson` tag，并把该规则加入 `app/info` 模型契约检查（凡被 `db.Insert`/`Update`/`Find` 解码目标使用的类型不得出现缺失 tag 的字段）。纯 JSON 响应结构（`ApiNote`、`ApiUser` 等，已核实不经数据库）不受影响。

## 3.1 ObjectID 类型与无类型文档解码（实现期定稿）

两项驱动差异需要在适配层补齐，均已实现：

- **`lea.ObjectID` 定义类型 + 显式编解码器**（`app/lea/ObjectID.go`）：底层 `[12]byte`，通过 `MarshalJSON` 保持 mgo 遗留 JSON 契约——零值输出 `""` 而非 `"000000000000000000000000"`（golden 已证明该零值出现在 `listNotes.CreatedUserId`、`addNotebook.ParentNotebookId` 等真实响应中）。注意：驱动默认注册表对 `[12]byte` 底型的定义类型会让 kind 级数组编解码器抢占 `ValueUnmarshaler`（实测 ObjectId 被解成 binary/array），因此 `lea.CodecRegistry` 显式注册了该类型的编解码器；client 经 `options.Client().SetRegistry(CodecRegistry)` 挂载，直接使用 `bson.Marshal/Unmarshal` 的测试需走 `Encoder/Decoder.SetRegistry`。`db.MustObjectIDFromHex`/`db.NewObjectID` 返回该类型，非法 hex 仍按 mgo 语义 panic（Revel recover → 500）。
- **`options.BSONOptions{DefaultDocumentM: true}`**（client 级解码器开关，与 SetRegistry 独立生效）：mgo 把无类型文档解为 `bson.M`（map），驱动默认解为 `bson.D`（切片）；博客主题模板在解出的文档上做字段访问（`footer.html` 的 `.Url`），`bson.D` 会直接 500。该开关全局恢复 mgo 解码语义。

CI 用 MongoDB 8.0 fixture（先前为 5.0）；`harness.environment.MongoImage` 与 `environment_test` 的命令断言同步为 8.0。

## 4. Context 与超时

兼容方法内部以 `context.WithTimeout(context.Background(), operationTimeout)` 包住单次调用，连接阶段使用独立 `connectTimeout`。配置键定为 `db.connectTimeoutMs`（默认 10000）与 `db.operationTimeoutMs`（默认 15000），写入 `conf/app.conf` 与 `conf/app.conf-default`；键缺失时取默认值，值非法时启动 fatal（与连接失败同级）。

完整请求取消传播不在本阶段实现，原因与后续验收见 MOD-001。

## 5. 错误与资源

- 每个 cursor 在 decode 成功或失败时都关闭；close error 进入日志。
- 错误映射保持行为兼容：`FindOne` 的 no-documents 返回 `mongo.ErrNoDocuments`（≙ mgo "not found"），单条 `Update` 无匹配在 driver v2 返回 nil 且 matched count 为 0（补日志）——两者在 `Err()` 下均按既有 bool 语义返回 true（幂等行为不变）；duplicate key 用 `mongo.IsDuplicateKeyError` 识别（≙ `mgo.IsDup`）；超时（`context.DeadlineExceeded`/`mongo.IsTimeout`）、取消（`context.Canceled`）与网络错误单独分类。真实失败（超时、取消、网络、duplicate key、命令错误）不得返回 true——"现有 API 需要 bool 时只在真正成功时返回 true" 仅指真实失败路径，不改变 not-found 的既有语义。`classifyError`（mongo_collection.go）输出 no-documents/duplicate-key/timeout/canceled/network/command-error 分类并计入每条失败日志；cursor close 错误同样入日志（Query.All 与 FindInCollection）。
- `Err()` 内的 `fmt.Println` 升级为项目日志，输出 collection、操作名与原始 error；`Get`/`GetByQ` 等无返回错误的 helper 保持签名不变，内部记日志。
- `Session.SetMode(mgo.Monotonic, true)`、`Session.Copy()`、`Session.Refresh()` 在官方驱动无等价物：连接池自动管理，随迁移直接删除。
- 初始化连接失败保持启动失败，不降级成无数据库运行。
- `CheckMongoSessionLost` 改为受超时约束的 ping/连接健康检查，不在每个请求上创建新 client；重连语义交由驱动连接池。

## 6. 回滚

迁移以独立分支完成，合并前同时保留 G 的旧基线，不保留运行时双驱动开关。由于 Schema 不变，回滚只需回退代码；无需反向数据迁移。

CI：`.github/workflows/regression-baseline.yml` 的 go-replay 腿把 mongo:5.0 换为 mongo:8.0（mgo 清除后 5.0 基线失去存在理由），并新增 `workflow_dispatch` 输入 `mongo_version`（默认 8.0，可选 7.0）；MongoDB 7.0 至少一次完整通过以 dispatch run 链接记录进任务检查产物。
