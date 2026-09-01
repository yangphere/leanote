# C-b 生产配置契约（F 消费，v1）

## 契约状态

本材料把 F 所需的生产配置接口收敛为已采纳、可直接实现和验收的 C-b v1 契约。它只适用于
`-runMode prod`；dev/test 继续使用各自的测试配置和隔离数据库规则。C-b 必须在实现任务的
PRD/design/验收材料中引用本契约，F 不复制键名或增加别名。该契约解决 Q-F4 的需求接口决策，
但不替代 C-b 的实现、测试和运行证据门禁。

当前 checkout 已实现生产入口和校验器的 fail-closed 主路径，但 C-b 的完整正向证据矩阵仍未闭合：
依赖任务尚未全部归档，真实 Mongo/package/container smoke 与进程级错误矩阵尚未在受控 Linux runner
完成。因此本文件固定接口和证据要求，不把本地单元测试或静态检查误报为完整生产合规。

## 当前实现证据快照（负面证据）

- `cmd/leanote/main.go:24-54` 已要求显式 `-conf`/`-runMode prod` 并在 bind 前校验；`main_test.go` 已覆盖
  缺省参数返回的稳定错误码和二进制进程退出 `78`，但 canonical 文件的 Linux 权限/进程级矩阵仍待受控
  runner 证据。
- `app/httpserver/production_config.go:32-111` 已拒绝默认/host-port 来源、重复敏感来源、非 regular
  canonical 文件、非法 URI 和 secret；配置解析单元测试已覆盖 URI/secret 核心边界，但尚未覆盖每个错误码
  的独立进程级“不 bind/不 dial”证据。
- `app/db/Mgo.go` 与 `app/db/mongo_client.go` 的生产入口不再记录完整 URL，并在 Mongo 不可达时由
  `/healthz` 返回 `503`；真实 Mongo 8.0、上传持久化和 PDF smoke 仍需 Linux/Docker runner 复核。
- `conf/app.conf` 与 `conf/app.conf-default` 仍是开发配置参考，包含 localhost/公开示例；打包和镜像
  allowlist 明确排除 `conf/app.conf`，生产入口不读取这些文件。

以上是当前 checkout 的剩余阻断证据，不是通过记录。C-b 后续必须提供本文件“必须提供的实现证据”所列的
正向材料，并在其 PRD/design/验收材料中引用本契约；F 只在正向证据完整后解除启动门。

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
`db.password` 不得出现在 active `[prod]` 的有效配置视图中，包括被全局/root 键继承的值；
它们是旧的多来源/回退接口。`db.urlEnv` 和 `app.secret` 的直接字面值也不得出现，避免把
secret 或连接信息写进文件。解析器必须在合并全局/root 与 `[prod]` 时检测重复键和被禁止键，
不能只检查 section 文本中的直接行。

## 健康端点契约（Q-F2）

C-b 必须提供无需认证的 `GET /healthz`。HTTP 服务已监听且 MongoDB ping 成功时，响应状态为 `200`，
`Content-Type` 固定为 `application/json; charset=utf-8`，正文严格为 `{"status":"ready"}\n`；
任一条件未满足时，响应状态为 `503`，正文严格为 `{"status":"not_ready"}\n`。响应 JSON 只能包含
这个 `status` 字段，不包含版本、配置值、凭据、用户数据或其他动态字段；响应头同样不得泄露敏感信息。
CI、package smoke 和 container smoke 只能用该端点及其状态码证明 readiness，不得用 `/login` 的 `200` 替代。

## 来源与优先级

1. C-b 先读取唯一的 `/etc/leanote/app.conf`，确认 active `[prod]` section 和契约键形态，
   同时形成包含全局/root 继承值的有效配置视图。
2. 再解析 `MONGODB_URL` 与 `LEANOTE_APP_SECRET`；两者必须已设置且 trim 后非空，环境值填充
   配置中的占位引用。
3. 运行时注入是这两个值的唯一事实来源；挂载文件只提供占位符和非敏感结构。不存在第二个
   文件、CLI 默认路径、host/port 组合或公开默认值 fallback。
4. 若同一语义在文件中出现 literal 值、重复键或另一个环境键，视为来源冲突并失败；“运行时
   注入优先”表示占位引用的解析顺序，不表示静默覆盖冲突值。

当多个配置错误同时存在时，错误码优先级固定为：CLI run mode → CLI config path → 文件存在性/类型/权限
→ active `[prod]` section → 必需键/重复键/禁止键 → 环境值缺失或为空 → secret 约束 → Mongo URI/数据库名约束。
同一类别内也必须按固定键顺序检查：环境值先 `MONGODB_URL`、后 `LEANOTE_APP_SECRET`；配置键按
section 名和键名的字典序；secret 约束先公开默认/控制字符，再长度；Mongo 约束先 scheme/host/path，
再数据库名。校验器必须按此顺序报告第一项错误，避免同一输入在不同入口产生不同稳定错误码。

`MONGODB_URL` 必须是单行、可解析的 `mongodb://` 或 `mongodb+srv://` URI，含非空数据库路径；
数据库路径按 URI 解析后移除一个前导 `/` 并进行 percent-decoding，再与 `db.dbname` 做逐字比较，
不得通过 query、fragment 或额外 slash 改变比较结果。
URI 不得指向 `localhost`、`127.0.0.1`、`::1` 或 `leanote_test`。URI 中的凭据只存在于进程内存，
不得写日志、摘要、artifact 或错误文本。

`LEANOTE_APP_SECRET` trim 后必须非空、至少 32 个可打印 ASCII 字节，且不得等于仓库公开默认 secret。
空白、公开默认值、ASCII 控制字符、非 ASCII 字节或环境变量缺失均不合格。

## 失败语义

所有配置校验必须发生在 HTTP bind/listen、Mongo dial/ping 和 `/healthz` 可达之前。配置失败时：

| 条件 | 稳定错误码 | 进程行为 |
| --- | --- | --- |
| `-runMode` 缺失或不是 `prod` | `CONFIG_RUN_MODE_INVALID` | 按固定优先级写一条脱敏错误，退出状态 `78`，不读取生产配置、不监听端口。 |
| `-conf` 缺失或不是 `/etc/leanote/app.conf` | `CONFIG_PATH_INVALID` | 按固定优先级写一条脱敏错误，退出状态 `78`，不读取其他配置、不监听端口。 |
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
