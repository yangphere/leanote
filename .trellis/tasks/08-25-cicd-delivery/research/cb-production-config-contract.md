# C-b 生产配置契约（F 消费，v1）

## 契约状态

本材料把 F 所需的生产配置接口收敛为已采纳、可直接实现和验收的 C-b v1 契约。它只适用于
`-runMode prod`；dev/test 继续使用各自的测试配置和隔离数据库规则。C-b 必须在实现任务的
PRD/design/验收材料中引用本契约，F 不复制键名或增加别名。该契约解决 Q-F4 的需求接口决策，
但不替代 C-b 的实现、测试和运行证据门禁。

当前仓库的实现证据尚未满足本契约：`cmd/leanote/main.go` 仍默认读取仓库相对的
`conf/app.conf`，`initDatabase` 仍可回退 `db.host/db.port`，`app/db/Mgo.go` 仍记录完整 URL。
因此本文件固定的是待 C-b 实现的接口和验收证据，不把现有代码误报为生产合规。

## 当前实现证据快照（负面证据）

- `cmd/leanote/main.go:31-43` 将 `-conf` 默认设为 `conf/app.conf`、`-runMode` 默认设为 `dev`，并直接读取
  `app.secret`；这不满足 prod 必须显式传入 canonical path/mode 的要求。
- `cmd/leanote/main.go:149-166` 仍按 `db.url` → `db.urlEnv` → `db.host/db.port/db.username/db.password`
  构造连接串，并为 `db.dbname` 提供 `leanote` 默认值；这不满足唯一 `MONGODB_URL` 和无 host/port fallback。
- `app/db/Mgo.go:61-102` 仍从 Revel 配置按同一顺序回退，并执行 `Log(url)`；该日志会暴露完整 Mongo URI，违反脱敏规则。
- `conf/app.conf:23-38` 与 `conf/app.conf-default:23-38` 仍含 localhost/host-port、公开 `app.secret` 和
  `db.urlEnv` 注释示例；它们只能作为待移除的开发样例，不能成为 prod 来源。
- `app/tests/harness/configuration_test.go:83-135` 只覆盖测试配置的环境变量/空值边界，不能证明上述 prod
  路径、键名、错误码、退出状态或日志契约。

这些快照是当前 checkout 的阻断证据，不是通过记录。C-b 后续必须提供本文件“必须提供的实现证据”所列的正向
材料，并在其 PRD/design/验收材料中引用本契约；F 只在正向证据完整后解除启动门。

## Canonical 运行接口

| 项目 | v1 契约 |
| --- | --- |
| 运行模式 | 必须显式传入 `-runMode prod`；缺失或其他模式以 `CONFIG_RUN_MODE_INVALID` 拒绝，不得进入生产发布 smoke。 |
| 配置文件 | 必须显式传入 `-conf /etc/leanote/app.conf`；缺失或其他路径以 `CONFIG_PATH_INVALID` 拒绝。该路径是唯一生产配置路径，必须是只读 regular file，部署权限固定为 `0440`（无写入/执行位）。不得读取或回退仓库的 `conf/app.conf`、`conf/app.conf-default` 或其他路径。 |
| Mongo URL 环境键 | `MONGODB_URL`，只允许通过 prod 配置中的 `db.urlEnv=${MONGODB_URL}` 注入；不得接受 `MONGO_URL`、`MONGODB_URI` 或其他别名。 |
| secret 环境键 | `LEANOTE_APP_SECRET`，只允许通过 prod 配置中的 `app.secret=${LEANOTE_APP_SECRET}` 注入；不得接受仓库公开默认值或其他别名。 |
| 数据库名键 | `db.dbname` 必须存在于 prod 配置，并且必须与 `MONGODB_URL` 的数据库路径一致；不得为 `leanote_test`。 |
| 生产配置最小片段 | active `[prod]` section 必须包含且仅能以占位引用提供：`db.urlEnv=${MONGODB_URL}`、`db.dbname=<non-test-name>`、`app.secret=${LEANOTE_APP_SECRET}`。 |

配置文件可以包含其他非敏感运行参数，但 `db.url`、`db.host`、`db.port`、`db.username`、
`db.password` 不得在 prod 的任何 section 出现；它们是旧的多来源/回退接口。`db.urlEnv` 和
`app.secret` 的直接字面值也不得出现，避免把 secret 或连接信息写进文件。

## 来源与优先级

1. C-b 先读取唯一的 `/etc/leanote/app.conf`，确认 active `[prod]` section 和契约键形态。
2. 再解析 `MONGODB_URL` 与 `LEANOTE_APP_SECRET`；两者必须已设置且 trim 后非空，环境值填充
   配置中的占位引用。
3. 运行时注入是这两个值的唯一事实来源；挂载文件只提供占位符和非敏感结构。不存在第二个
   文件、CLI 默认路径、host/port 组合或公开默认值 fallback。
4. 若同一语义在文件中出现 literal 值、重复键或另一个环境键，视为来源冲突并失败；“运行时
   注入优先”表示占位引用的解析顺序，不表示静默覆盖冲突值。

`MONGODB_URL` 必须是单行、可解析的 `mongodb://` 或 `mongodb+srv://` URI，含非空数据库路径，
且不得指向 `localhost`、`127.0.0.1`、`::1` 或 `leanote_test`。URI 中的凭据只存在于进程内存，
不得写日志、摘要、artifact 或错误文本。

`LEANOTE_APP_SECRET` trim 后必须非空、至少 32 个可打印 ASCII 字节，且不得等于仓库公开默认 secret。
空白、公开默认值、ASCII 控制字符、非 ASCII 字节或环境变量缺失均不合格。

## 失败语义

所有配置校验必须发生在 HTTP bind/listen、Mongo dial/ping 和 `/healthz` 可达之前。配置失败时：

| 条件 | 稳定错误码 | 进程行为 |
| --- | --- | --- |
| `-runMode` 缺失或不是 `prod` | `CONFIG_RUN_MODE_INVALID` | 写一条脱敏错误，退出状态 `78`，不读取生产配置、不监听端口。 |
| `-conf` 缺失或不是 `/etc/leanote/app.conf` | `CONFIG_PATH_INVALID` | 写一条脱敏错误，退出状态 `78`，不读取其他配置、不监听端口。 |
| 文件缺失 | `CONFIG_FILE_MISSING` | 写一条脱敏错误，退出状态 `78`，不监听端口。 |
| 文件不可读、不是 regular file 或权限不是只读 `0440` | `CONFIG_FILE_UNREADABLE` | 写一条脱敏错误，退出状态 `78`，不监听端口。 |
| `[prod]` 缺失、必需键缺失或键形态不符 | `CONFIG_SECTION_MISSING` 或 `CONFIG_KEY_INVALID` | 退出状态 `78`，不连接 Mongo。 |
| 环境键缺失或 trim 后为空 | `CONFIG_VALUE_MISSING` 或 `CONFIG_VALUE_EMPTY` | 退出状态 `78`，不监听端口。 |
| literal 值、重复键或未声明别名造成冲突 | `CONFIG_SOURCE_CONFLICT` | 退出状态 `78`，不监听端口。 |
| 公开默认 secret、短 secret 或控制字符 | `CONFIG_PUBLIC_DEFAULT` 或 `CONFIG_SECRET_INVALID` | 退出状态 `78`，不监听端口。 |
| Mongo URI scheme/host/path/数据库名不合规 | `CONFIG_MONGO_INVALID` | 退出状态 `78`，不进行 Mongo ping。 |

配置有效但 Mongo ping 失败属于运行时 readiness 失败而不是配置错误：服务按 Q-F2 通过
`GET /healthz` 返回 `503`，不得把它伪装成配置成功。

## 日志与 artifact 脱敏

配置错误日志只能包含稳定错误码、非敏感键名和 `run_mode=prod`，例如
`configuration error code=CONFIG_VALUE_EMPTY key=MONGODB_URL run_mode=prod`。禁止输出配置值、
secret、Mongo URI（包括用户名、密码、host、query）、请求正文、环境 dump 或完整配置文件。
质量门摘要仅记录 `failure.category=setup` 及上述错误码类别；不得上传原始服务日志或独立配置
artifact。

## 必须提供的实现证据

C-b 完成后，F 的 Task 0 必须收集以下可复核证据，缺一仍不得激活 F：

- 运行入口/部署文档明确传入 `-conf /etc/leanote/app.conf -runMode prod`，并证明仓库默认文件
  不在发布 tarball/镜像中；
- 配置单元测试覆盖每个错误码、来源冲突、占位解析、URI/数据库名约束和 secret 约束；
- 进程级测试证明配置失败退出状态为 `78`、不 bind 端口、不 dial Mongo，日志只含脱敏错误码；
- 静态/回归检查证明不存在 host/port/localhost/`leanote_test` fallback、完整 URI 日志和未声明
  的环境别名；
- 合法配置的 package/container smoke 证明配置校验先于 `/healthz`，Mongo ready 才返回 `200`，
  未 ready 返回 `503`，且摘要/artifact 不含配置值。

未取得上述 C-b 实现证据前，F 只能保持 `planning`；本材料不授权修改业务实现或伪造通过记录。
