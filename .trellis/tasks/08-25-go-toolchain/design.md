# Go 工具链与通用依赖现代化（A）- 技术设计

## 1. 边界与不变量

A 只改变 Go baseline、已列明的通用直接依赖、为清零既有 vet 所需的源码表达和对应验证资产。Revel/Mongo 版本与语义仍由 C-a/B 拥有。

必须保持：

- 外部 HTTP/API、USN、用户所有权、BSON Schema、模板和生成路由不变；
- G 的 Golden 只 replay，不录制或刷新；
- 依赖升级、vet 类别和回滚单元能一一对应；
- 错误以原始非零状态暴露，不引入隐藏 fallback。

G 的实现资产已提交（`2dc85af`），G-AC8 也已由 PR #3 的真实 `pull_request` workflow 运行
32871393901 完成取证并回填 G 归档。原归档流程冲突已闭合，不再阻塞 A；证据边界见 PRD
"实质启动门禁"。

## 2. 工具链与 CI

`go.mod` 使用最低语言版本 `go 1.26`，不写 `toolchain`。GitHub Actions 使用显式矩阵值 `1.26.7`、`1.27.0`，两个值运行相同 build/vet/tests，避免假矩阵。

G 应先拥有 MongoDB 5.0 replay workflow；A 只扩展 Go 版本矩阵和加入本任务门禁，不创建缩水版替代流程。遗留 `.travis.yml` 仅对齐最低 Go 版本（选择器限定 minor 或 patch、不低于 1.26，如 `1.26.x` 或 `1.26.7`；禁止 1.15 及以下和 `stable`/`latest`/`tip` 等跨 minor 滚动别名）并移除浮动 `go get -u`，不在 A 删除；最终 CI 收口由 F 负责。

Go 1.26 是编译兼容下限。所有选定依赖的 `GoVersion` 必须 <= 1.26；当前候选 goquery v1.12.0、go-flags v1.6.1、x/crypto v0.55.0、x/tools v0.49.0 均满足。

工具链切换存在互相锁死：G 夹具每次 replay 都用 `LEANOTE_TEST_GO` 指定的 Go 1.20.14 运行 legacy 源码生成，README 已记录 Go 1.26/1.27 在旧 x/tools 下 panic，而 Go 1.20.14 无法读取声明 `go 1.26` 的主模块。因此 `go directive = 1.26` 与 `x/tools v0.49.0` 构成一个原子 bootstrap 批次：先行落地，在两个 Go 版本上验证生成入口无 panic 后才继续其他依赖。此后 `LEANOTE_TEST_GO` 从必需降级为可选覆盖（缺省 PATH 中的 go）；workflow 双 job、README、harness 提示/测试与 backend quality spec 中的 Go 1.20.14 契约一并迁移，归档文档不改。

迁移后的生成工具链必须 fail-closed：解析缺省 go 时在启动前校验版本 ≥1.26.7，低于下限显式失败；生成/构建子进程强制 `GOTOOLCHAIN=local`，防止 `GOTOOLCHAIN=auto` 环境下旧 Go 自动下载工具链或低补丁版本绕过下限。record 流程验证只在隔离 checkout 或临时副本进行，A 自身工作区保持 replay-only、Golden 零 diff。

## 3. 直接依赖处置

依赖所有权以源码调用方为准，不以 `// indirect` 注释猜测。每个模块按以下事务执行：

1. 记录 current/target、模块 GoVersion、许可证、`go mod why -m` 和直接调用方；
2. 只更新该模块，保留完整命令错误；
3. 运行调用方聚焦测试、全量 build 和 G replay；
4. 审查 `go.mod`/`go.sum` 传递变化并记录因果；
5. 失败时回退该模块，不叠加下一个升级。

候选与处置由 PRD R-A2 的表唯一定义。gocolorize 当前无更新且只服务 `app/cmd`，保持到 C-b 删除该目录。robfig/config 保持当前版本；A 建立消息解析契约，解析器替换延期为 `MOD-003`。

## 4. Struct Tag 兼容迁移

### 4.1 现有有效语义

旧 mgo 的 `getStructInfo` 先读取 `field.Tag.Get("bson")`；当结果为空且原始 tag 不含 `:` 时，回退使用整段原始 tag，并解析 `omitempty`。因此：

```text
`Title`              -> BSON key "Title"；JSON 使用默认字段名 "Title"
`IsBlog,omitempty`   -> BSON key "IsBlog" 且零值省略；JSON 字段 "IsBlog" 不省略
```

直接改成 `json:"IsBlog,omitempty"` 会改变外部 JSON，直接删除 tag 会把 mgo BSON key 变成全小写字段名，二者都不允许。

### 4.2 迁移规则

迁移只给原有 legacy BSON tag 加命名空间：

```text
`Title`              -> `bson:"Title"`
`IsBlog,omitempty`   -> `bson:"IsBlog,omitempty"`
```

不自动添加 JSON tag。已有合法 `bson`/`json` 组合保持原样。实现前从 18 个文件生成并评审 205 字段清单，迁移后清单必须归零且 `go vet` 不再报告 tag。

### 4.3 测试位置与覆盖

模型测试放在 `app/info`，与集成性质的 `app/tests`（harness 会构建并启动真实服务器、连接 Mongo）保持包职责隔离。测试分两层：

- 结构清单：每个受影响类型/字段的目标 BSON 名、flags 与现有 JSON 策略；
- 行为快照：用 mgo BSON marshal/unmarshal 与 `encoding/json` 覆盖非零值、零值、ObjectId、时间、nil/空 slice/map 和嵌套结构。

旧 tag 上先锁定有效行为，迁移后运行同一断言。快照只描述模型契约，不替代 HTTP Golden。

## 5. Vet 修复设计

实现前基线为 237 条：

| 类别 | 数量 | 设计 |
|---|---:|---|
| invalid struct tag | 205 | 按第 4 节等价命名化 |
| unkeyed literal | 21 | 补字段名，值与顺序语义不变；包括 10 个 `bson.RegEx`、Revel result 和项目模型 |
| unreachable | 6 | 只删除编译器已证明不可达的语句 |
| self-assignment | 3 | 只删除无效赋值，保护相邻输出/文件名逻辑 |
| printf misuse | 1 | `E404`（`app/controllers/BaseController.go:161-163`）当前 `("", nil)` 会进入 Revel 的 `fmt.Sprintf`；先锁定实际 404 正文，再用 vet-clean 表达保持它 |
| signal channel | 1 | `app/cmd/harness/harness.go:333` 的 channel 容量改为 1，保持现有订阅（`os.Interrupt`、`os.Kill`）和 interrupt 后的 kill 流程；不提前增加 SIGTERM |

修复按类别分批，批次之间运行 `go vet ./app/...` 和拥有该行为的测试。外部类型 keyed literal 仍属于 A 的语法/静态检查修复，但不授权改变外部模块版本。

## 6. 调用方验证

| 变更 | 聚焦验证 |
|---|---|
| goquery | `SubStringHTML` 对 Unicode、实体、不完整/嵌套标签和结束标记的确定输出 |
| x/crypto | bcrypt 旧 hash 正确/错误密码、生成后回验 |
| go-flags/gocolorize/x/tools | app/cmd CLI 参数、routes/tmp 生成、生成后二进制 build |
| Go 1.20.14 契约迁移 | workflow 双 job、README、server 提示/测试、quality spec 一致；`rg -n '1\.20\.14' .github app/tests .trellis/spec/backend/quality-guidelines.md` 零命中；record 流程在隔离 checkout 于新工具链可跑且工作区 Golden 零 diff |
| robfig/config | 保持当前版本；验证全 locale 消息解析、section/key、插值和缺失键，作为 `MOD-003` 基线 |
| struct tags | DB-independent BSON/JSON 全类型契约 + HTTP Golden |
| controller/service/lea vet | 对应聚焦单测 + Golden/USN/smoke |
| toolchain | 两版本 build/vet/Go tests；真实 Revel run/package smoke |

命令成功但没有发现目标测试不算通过。Golden/USN 使用 MongoDB 5.0；MongoDB 7/8 只在 B 完成驱动迁移后验证。

## 7. 数据流与兼容性

```text
app/info values
   |-- mgo BSON (legacy raw tags -> explicit bson tags, bytes/keys unchanged)
   `-- encoding/json (no new json tags, API fields unchanged)

Go source -- app/cmd parser2 -- generated routes/tmp -- binary -- Revel HTTP
     |                                              |
     `-- go vet/build on 1.26.7 + 1.27.0            `-- G Golden/USN replay
```

本任务无数据库迁移、配置迁移或双写。任何不可解释的 BSON/JSON/HTTP 差异都回滚到产生差异的最小批次。

## 8. 失败与回滚

- G-AC8 证据链接失效、运行结论不再可核验或对应提交发生漂移：停止 A，按 PRD 门禁重新处置 G；
  不在 A 建简化替代品。
- 依赖不可下载、checksum/许可证/GoVersion 不符合：该模块失败并回到规划。
- bootstrap 批次（go directive + x/tools）任一半失败即整批回退，不保留"directive 已升、x/tools 未升"的中间态；其余单模块调用方或 Golden 失败只回退该模块，保留前序已通过批次。
- tag 契约失败：回退对应 tag 批次，不更新模型或 HTTP 快照。
- vet 修复改变行为：回退该类别批次，不扩大为业务重构。
- Go 1.26 与 1.27 结果不一致：以最低版本兼容为约束修复；不得只让新版本 job 通过。

## 9. 延期设计

robfig/config 是运行时依赖且无可升级版本。A 保留当前实现，不增加双解析器或 fallback；七种语言消息契约作为 `MOD-003` 的启动前基线。后续替换必须独立规划、验收和回滚。
