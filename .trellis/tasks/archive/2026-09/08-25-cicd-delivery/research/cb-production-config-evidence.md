# C-b 生产配置正向证据矩阵（F 消费，v1）

## 目的与状态

本材料把 Q-F4 的“实现证据”定义为可重放、可定位、来自当前 C-b checkout 的正向材料。规格文字、
`task.json.status=completed` 或父任务计数都不是实现证据；没有原始命令、测试结果、静态检查结果和
artifact 绑定信息，不得把条目标记为通过。

截至 2026-08-31，矩阵状态仍为 **BLOCKED**。生产入口、固定 `/healthz` 响应、URI/secret 校验和
脱敏主路径已实现并有局部测试，但依赖任务归档、完整进程级错误矩阵以及受控 Linux/Mongo/Docker
运行证据仍缺失。下表中的“当前结果”区分局部正向证据与尚未闭合的证据门，不得解释为完整生产通过。

## 证据矩阵

| ID | 契约不变量 | 必须提供的正向证据 | 复核命令/入口 | 当前结果 |
| --- | --- | --- | --- | --- |
| E1 | prod 只能显式使用 `-conf /etc/leanote/app.conf -runMode prod` | C-b 入口/部署文档、进程启动测试；缺失或非 canonical 参数分别得到 `CONFIG_PATH_INVALID`/`CONFIG_RUN_MODE_INVALID`，退出 `78`，不读取其他配置 | `rg -n "flag\.String\(\"conf\"|flag\.String\(\"runMode\"|/etc/leanote/app\.conf" cmd app docs`; Linux process test captures exit/status/log | **PARTIAL**：`cmd/leanote/main.go:24-54`、`main_test.go` 和二进制缺参检查已证明显式参数与 `78`；合法 canonical 启动仍待 Linux runner |
| E2 | 配置路径是只读 regular file，权限固定 `0440` | Linux/container test proves regular-file type, mode `0440`, unreadable/writable mode rejected as `CONFIG_FILE_UNREADABLE`, exit `78` | `stat -c '%F %a' /etc/leanote/app.conf`; process test with mode variants | **PARTIAL**：`ValidateProductionConfig` 已使用 `Lstat`、regular-file 和 `0440` 校验；Linux 权限变体与进程证据未运行 |
| E3 | Mongo/secret 只接受 `MONGODB_URL`/`LEANOTE_APP_SECRET` 占位引用，运行时值先解析但冲突不覆盖 | C-b config unit tests cover exact prod fixture, both placeholders, missing/empty env, literal/duplicate/alias conflict and precedence | `go test ./... -run 'Config.*Prod|ProductionConfig' -count=1`; test report must list every case | **PARTIAL**：`app/httpserver/production_config.go` 已实现占位、重复敏感来源和别名冲突校验，核心单测通过；每个错误码的完整用例仍待补齐 |
| E4 | URI scheme/host/path/dbname 与 secret 约束完整 | Unit tests cover `mongodb://`/`mongodb+srv://`, non-empty path, exact `db.dbname` match, forbidden localhost/loopback/test DB, printable ASCII secret >=32 bytes and public-default rejection | `go test ./... -run 'ProductionConfig.*(URI|DB|Secret)' -count=1` | **PASS（局部）**：`production_config.go` 已实现并由 `production_config_test.go` 覆盖核心 URI/数据库名/secret 边界；进程级错误交叉验证仍待补齐 |
| E5 | 配置错误在 bind/listen、Mongo dial/ping、`/healthz` 前 fail closed，稳定退出 `78` | Process tests assert each error code, exit `78`, no listening socket, no Mongo dial, and one redacted log line | Linux process harness + `ss -ltn`/Mongo dial spy; output bound to current commit/run | **PARTIAL**：缺参二进制退出 `78`、配置错误日志脱敏已验证；完整错误矩阵、端口/Mongo 探针尚未在 Linux runner 执行 |
| E6 | 不存在 `db.url`、host/port、localhost、公开默认或未声明环境别名 fallback；不记录完整 URI | Static scan and runtime log test prove forbidden keys/aliases and URI logging are absent | `rg -n "db\.(url|host|port|username|password)|MONGO_URL|MONGODB_URI|Log\(url\)|localhost|leanote_test" cmd app conf`; redacted log assertion | **BLOCKED**：生产 plain entry 与 Mongo 初始化已移除 URL/host-port fallback 和完整 URI 日志，但 `app/init.go`、管理备份等 legacy Revel 路径仍有命中，依赖任务需先收口 |
| E7 | 合法配置先完成校验；Mongo ready 才 `/healthz=200`，未 ready `/healthz=503` | Package/container smoke uses canonical file/env injection, probes unauthenticated `/healthz`, records HTTP+Mongo result in allowlisted summary | `curl -fsS -o /dev/null -w '%{http_code}' http://127.0.0.1:9000/healthz`; summary `service.health_path=/healthz` | **PARTIAL**：固定 `/healthz` 单测、package/container smoke 和摘要字段已实现；本机 Docker Desktop 将 0440 映射为不可验证权限，真实 Mongo/Docker smoke 需 Linux runner |
| E8 | 证据必须绑定 C-b 当前实现、测试和 workflow，不能由归档状态替代 | C-b PRD/design/implement/check 全部闭合，workflow run/artifact、commit SHA 和命令输出可复核 | `python ./.trellis/scripts/task.py validate <C-b-task>` + archive/task evidence inspection | **BLOCKED**：依赖任务仍未完成归档，真实 workflow/artifact provenance 缺失 |

## 已运行的部分验证

以下命令证明当前 checkout 的局部实现可运行，但不证明 Q-F4 全部正向合规：

```text
go test ./cmd/leanote ./app/httpserver -count=1
  PASS: cmd/leanote, app/httpserver (including health/config unit tests)
node --test tests/js/release-contract.test.js
  PASS: 8 tests (release metadata, browser matrix, smoke contracts and summary schema)
python ./.trellis/scripts/task.py validate .trellis/tasks/08-25-cicd-delivery
  PASS: implement.jsonl/check.jsonl schema validation
```

本机 Docker Desktop 的容器 smoke 因 bind mount 无法保持 `0440` 而阻断；真实 Mongo 8.0、进程错误矩阵、
受保护四浏览器矩阵和 release/GHCR run provenance 仍不能由本地结果替代。

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
