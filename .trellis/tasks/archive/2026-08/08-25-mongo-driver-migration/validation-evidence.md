# B 驱动迁移验证证据（2026-08-29）

## MongoDB 8.0（主基线）

- 容器：`mongo:8.0`（8.0.29），fixture `mongodb_backup/leanote_install_data` → `leanote_test`，499 docs / 2 users。
- 全套 Go/Golden/USN 回放（`LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/... -count=1 -timeout 30m`）**连续两次全绿**：
  - Run 1（replay7）：ok app/tests 0.078s / ok harness 53.291s / ok cmd/e2e 0.054s
  - Run 2（replay8）：ok app/tests 0.060s / ok harness 54.161s / ok cmd/e2e 0.088s
- `LEANOTE_REQUIRE_MONGO=1` legacy auth（TestAuth）通过。
- `go build ./app/...`、`go vet ./app/...`、`go test ./app/{db,info,lea,controllers,cmd}`、`npm test`、`git diff --check` 全部通过。

## MongoDB 7.0（兼容性证据）

- 容器：`mongo:7.0`（**7.0.40**），同一 fixture 恢复（499 docs / 2 users）。
- legacy auth（TestAuth）+ 全套回放**一次完整通过**：
  - ok app/tests（TestAuth + replay 前置）0.095s / ok app/tests 0.080s / ok harness 53.503s / ok cmd/e2e 0.055s，EXIT=0。
- 服务器版本字符串 `7.0.40` 由 mongosh `db.version()` 在恢复前核验。

## 残留说明

- `rg 'gopkg.in/mgo.v2' app go.mod`：零命中。
- `go.sum` 保留 1 条 `gopkg.in/mgo.v2 .../go.mod` 图校验哈希：来自 `revel/modules → flosch/pongo2` 的上游 go.mod require（pongo2 的 mongo loader 未被本项目导入，包级不可达），属不可移除的上游产物，已写入 PRD AC。

## 实现期发现并修复的驱动语义差（详见 design.md §3.1）

1. `lea.ObjectID` 定义类型：零值 JSON 为 `""`、`Hex()` 零值为 `""`（mgo 语义，`GetUserId` 等存在性判断依赖），BSON 经显式注册表编解码为标准 ObjectId。
2. `lea.CodecRegistry` 显式注册：默认注册表对 `[12]byte` 底型定义类型会让数组编解码器抢占 `ValueUnmarshaler`（实测 ObjectId 被解成 binary/array）。
3. `BSONOptions{DefaultDocumentM: true}`：mgo 将无类型文档解为 `bson.M`，驱动默认 `bson.D`；博客主题模板按 map 字段访问（`footer.html` 的 `.Url`）。
4. 解码接受 BSON string 形式的 ID 字段（`ParentNotebookId:""` 等 fixture 数据），镜像 mgo 的 string 底型行为。
5. 单条 `Update` 无匹配：v2 返回 nil+matched=0（mgo 为 "not found" 错误），`Err()` 布尔语义保持 true，补日志。

## 审核修复轮（2026-08-29，code-review 后）

- 修复 4 文件 gofmt 违规、11 文件 lea 重复 import（具名+dot 并存）。
- 超时配置非法值改为启动 fatal（design §4 原文要求；`timeoutConfigValue` 手工解析，缺失/空白回默认）。
- `classifyError` 分类（no-documents/duplicate-key/timeout/network/command-error）接入全部失败日志；cursor close 错误入日志（Query.All、FindInCollection）。
- 新增聚焦测试：duplicate key 分类与不伪成功、1ns 超时分类、不可达 dial 失败。
- `TestAllRegistryFieldsHaveExplicitBsonTags` 通用扫描：抓到 `UserAndBlog.BlogUrls`、`UserAndBlogUrl.User` 两处缺 tag，已补显式 tag；驱动的非 inline 匿名字段恒用类型名键（`user`→`User`），属视图结构惰性漂移，冻结期望已更新并登记头注。
- 修复后重验：build/vet/gofmt 绿、全部单测绿、MongoDB 8.0 legacy TestAuth + 全套回放再次全绿（replay9，EXIT=0）。
