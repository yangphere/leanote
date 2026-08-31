# C-b 生产配置正向证据矩阵（F 消费，v1）

## 目的与状态

本材料把 Q-F4 的“实现证据”定义为可重放、可定位、来自当前 C-b checkout 的正向材料。规格文字、
`task.json.status=completed` 或父任务计数都不是实现证据；没有原始命令、测试结果、静态检查结果和
artifact 绑定信息，不得把条目标记为通过。

截至 2026-08-31，矩阵状态为 **BLOCKED**。当前实现仍有明确反例：入口默认 `conf/app.conf`/`dev`，
Mongo 连接允许 host/port 回退，`app/db/Mgo.go` 记录完整 URI，仓库没有 `/healthz`，且 C-b 归档验收
材料仍有未勾选项。下表中的“当前结果”是负面或部分证据，不得解释为正向通过。

## 证据矩阵

| ID | 契约不变量 | 必须提供的正向证据 | 复核命令/入口 | 当前结果 |
| --- | --- | --- | --- | --- |
| E1 | prod 只能显式使用 `-conf /etc/leanote/app.conf -runMode prod` | C-b 入口/部署文档、进程启动测试；缺失或非 canonical 参数分别得到 `CONFIG_PATH_INVALID`/`CONFIG_RUN_MODE_INVALID`，退出 `78`，不读取其他配置 | `rg -n "flag\.String\(\"conf\"|flag\.String\(\"runMode\"|/etc/leanote/app\.conf" cmd app docs`; Linux process test captures exit/status/log | **FAIL**：`cmd/leanote/main.go:31-32` 默认 `conf/app.conf`/`dev`，没有 canonical 参数错误码 |
| E2 | 配置路径是只读 regular file，权限固定 `0440` | Linux/container test proves regular-file type, mode `0440`, unreadable/writable mode rejected as `CONFIG_FILE_UNREADABLE`, exit `78` | `stat -c '%F %a' /etc/leanote/app.conf`; process test with mode variants | **MISSING**：当前没有 `/etc/leanote/app.conf` 校验入口 |
| E3 | Mongo/secret 只接受 `MONGODB_URL`/`LEANOTE_APP_SECRET` 占位引用，运行时值先解析但冲突不覆盖 | C-b config unit tests cover exact prod fixture, both placeholders, missing/empty env, literal/duplicate/alias conflict and precedence | `go test ./... -run 'Config.*Prod|ProductionConfig' -count=1`; test report must list every case | **PARTIAL**：`app/httpserver/config_test.go:133-169` 只证明通用 section/插值；`app/tests/harness/configuration_test.go:83-135` 是 test fixture |
| E4 | URI scheme/host/path/dbname 与 secret 约束完整 | Unit tests cover `mongodb://`/`mongodb+srv://`, non-empty path, exact `db.dbname` match, forbidden localhost/loopback/test DB, printable ASCII secret >=32 bytes and public-default rejection | `go test ./... -run 'ProductionConfig.*(URI|DB|Secret)' -count=1` | **FAIL**：当前仅 `validateProdSecret` 检查空值/公开默认，未实现 URI、dbname、长度和字符约束 |
| E5 | 配置错误在 bind/listen、Mongo dial/ping、`/healthz` 前 fail closed，稳定退出 `78` | Process tests assert each error code, exit `78`, no listening socket, no Mongo dial, and one redacted log line | Linux process harness + `ss -ltn`/Mongo dial spy; output bound to current commit/run | **FAIL**：当前使用 `log.Fatalf`，无稳定错误码/退出 `78` 进程契约 |
| E6 | 不存在 `db.url`、host/port、localhost、公开默认或未声明环境别名 fallback；不记录完整 URI | Static scan and runtime log test prove forbidden keys/aliases and URI logging are absent | `rg -n "db\.(url|host|port|username|password)|MONGO_URL|MONGODB_URI|Log\(url\)|localhost|leanote_test" cmd app conf`; redacted log assertion | **FAIL**：`cmd/leanote/main.go:149-166` 与 `app/db/Mgo.go:61-102` 命中回退/完整 URL 日志 |
| E7 | 合法配置先完成校验；Mongo ready 才 `/healthz=200`，未 ready `/healthz=503` | Package/container smoke uses canonical file/env injection, probes unauthenticated `/healthz`, records HTTP+Mongo result in allowlisted summary | `curl -fsS -o /dev/null -w '%{http_code}' http://127.0.0.1:9000/healthz`; summary `service.health_path=/healthz` | **FAIL**：当前仓库无 `/healthz`；现有 smoke 不能作为该契约证据 |
| E8 | 证据必须绑定 C-b 当前实现、测试和 workflow，不能由归档状态替代 | C-b PRD/design/implement/check 全部闭合，workflow run/artifact、commit SHA 和命令输出可复核 | `python ./.trellis/scripts/task.py validate <C-b-task>` + archive/task evidence inspection | **FAIL**：归档 C-b 的验收清单仍未闭合，真实 workflow 证据缺失 |

## 已运行的部分验证

以下命令只证明现有局部测试仍可运行，不证明 Q-F4 正向合规：

```text
go test ./cmd/leanote ./app/httpserver -run 'Test(ValidateProdSecret|Config|LoadConfig|ParseConfig)' -count=1
  PASS: cmd/leanote, app/httpserver
go test ./app/tests/harness -run 'Test(.*Configuration|.*Config)' -count=1
  PASS: app/tests/harness
```

这些测试没有覆盖 E1/E2/E4/E5/E7 的 canonical prod 入口、错误码、退出状态、端口/Mongo 顺序和
`/healthz`，因此只能作为 E3 的部分背景，不能关闭证据门。

## 证据收口规则

C-b 后续实现必须在其材料中引用 `research/cb-production-config-contract.md` 和本矩阵，并上传以下
可审计材料：

1. `cb-config-unit`：逐项覆盖 E1-E4 的单元测试结果和测试发现数；
2. `cb-config-process`：E1/E2/E5 的进程退出、端口/Mongo 探针和脱敏日志；
3. `cb-config-static`：E6 的源码扫描结果，绑定 commit SHA；
4. `cb-config-smoke`：E7 的合法/非法配置 package/container smoke 与 `service` 摘要；
5. `cb-config-provenance.json`：上述材料的 artifact 名称、workflow run/attempt、ref、commit 和 SHA-256。

任何材料缺失、测试发现为零、跨 run/ref、哈希不符、日志包含敏感值、或 E1-E8 任一非 `PASS`，F
都必须保持 `planning`。本矩阵不授权修改业务实现、不创建占位证据，也不允许把当前负面结果改写为成功。
