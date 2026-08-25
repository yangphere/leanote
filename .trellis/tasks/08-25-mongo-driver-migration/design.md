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

## 2. Query 语义映射

| mgo 形状 | 官方驱动实现 |
|---|---|
| `Find(q).One(out)` | `FindOne` + decode，保留 no-document 语义 |
| `Find(q).Sort(...).Skip(n).Limit(n).All(out)` | `FindOptions` + cursor 全量 decode + 始终关闭 cursor |
| `Select(fields)` | projection 文档 |
| `Insert(v...)` | 单个 `InsertOne` 或多个 `InsertMany`，错误完整返回到包装层 |
| `Update` / `Remove` | `UpdateOne` / `DeleteOne` |
| `UpdateAll` / `RemoveAll` | `UpdateMany` / `DeleteMany`，保留匹配数量日志 |
| `Upsert` | `UpdateOne` + `SetUpsert(true)` |
| `Count` / `Distinct` | `CountDocuments` / `Distinct` |

排序字符串 `-Count` / `Usn` 由一个解析函数转换为有序 BSON 文档，禁止用 map 表示排序以免丢顺序。

## 3. BSON 与 ObjectId

模型字段统一改为官方 `bson.ObjectID`，查询文档统一使用官方 `bson.M`/`bson.D`。凡原来调用 `bson.ObjectIdHex` 的路径必须改用 `db.MustObjectIDFromHex` 或返回 error 的显式解析函数；选择依据是原调用点当前失败语义，禁止把无效 ID 变成零 ObjectId。

Golden 负责 HTTP JSON 形态，模型序列化测试负责 BSON 字段名。任何字段名变化都视为迁移失败，而不是新驱动的正常差异。

## 4. Context 与超时

兼容方法内部以 `context.WithTimeout(context.Background(), operationTimeout)` 包住单次调用，连接阶段使用独立 `connectTimeout`。默认值写入 `conf/app.conf-default` 和测试配置；无界 `context.Background()` 不直接传给驱动。

完整请求取消传播不在本阶段实现，原因与后续验收见 MOD-001。

## 5. 错误与资源

- 每个 cursor 在 decode 成功或失败时都关闭；close error 进入日志。
- duplicate key、no documents、timeout、network error 分别识别；现有 API 需要 bool 时只在真正成功时返回 true。
- 初始化连接失败保持启动失败，不降级成无数据库运行。
- `CheckMongoSessionLost` 改为受超时约束的 ping/连接健康检查，不在每个请求上创建新 client。

## 6. 回滚

迁移以独立分支完成，合并前同时保留 G 的旧基线，不保留运行时双驱动开关。由于 Schema 不变，回滚只需回退代码；无需反向数据迁移。
