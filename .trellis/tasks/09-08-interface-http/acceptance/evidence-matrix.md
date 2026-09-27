# interface-http 验收证据矩阵

决策 D-H1～D-H7 已确认（见 `research/spec-audit-2026-09-25.md` §6），矩阵中不再有决策类 blocked。

状态取值：`unrun`（未执行）、`blocked`（前置决策或环境缺失）、`partial`、`passed`。每条 `passed` 必须写明命令、发现/执行数量与 commit；mock、controller 直调或历史 artifact 不得记为真实 HTTP 证据。

| AC | 内容 | 证据类型 | 前置 | 状态 |
|----|------|----------|------|------|
| AC-H1 | 全部公开 action/静态前缀可达；registry ↔ inventory 对账 | route-table + 对账测试 | B0 inventory | partial |
| AC-H2 | 未注册 404、导出未注册不可达、identity 405、其他方法策略 | route negative | B1 | partial |
| AC-H3 | 响应/状态/Golden；binder 负例 | contract + Golden replay（Mongo 8 standalone） | B2–B6 | unrun |
| AC-H4 | API token 保持、旧 Web cookie 匿名、cookie 安全属性 | session contract + replay | B1 | partial |
| AC-H5 | identity 方法/principal/`_ID`/P-01/P-03/P-06/P-07/U-04 | contract + 真实 HTTP replay | B2 | unrun |
| AC-H6 | `ApiUser` 可达、API Auth/User Golden（identity AC-I5） | Golden live replay | B2 | unrun |
| AC-H7 | production-config 校验、ContentRoots/BackupRoot、exit 78、healthz、admin consumer | config contract + 进程级 exit code 测试 | B1 | partial |
| AC-H8 | LocaleResolver、27 模板函数、主题渲染/Preview | contract + 页面 smoke | B1、B4 | partial |
| AC-H9 | D-H6 wire、notes OperationId/ExpectedUsn、publishing submissionId/callback | 真实 HTTP regression | B3、B4 | unrun |
| AC-H10 | harness 在 `cmd/leanote -runMode test` 全量运行；run mode 互斥；SIGTERM | harness 全量 + 负例 | B8 | unrun |
| AC-H11 | Revel 零命中、go.mod 无 revel、`app/cmd` 删除 | rg + `go mod tidy` diff | B8 通过 | unrun |
| AC-H12 | build/vet/test/npm/diff-check/validate | 质量门 | 全部批次 | partial |

## B0 evidence (2026-09-25)

- `research/action-inventory.md` records the code-derived baseline: 95 routes (83 action, 9 static, 3 catch-all) and 242 exported controller methods returning `revel.Result`; each row includes routability, owner, method, current BEFORE/commonUrl facts, response shape, and Golden status.
- `go test ./app/httpserver -count=1`: passed, including route parsing and the negative that an exported `BaseController` helper is not reachable through the catch-all.
- `go vet ./app/httpserver`: passed.
- `LEANOTE_HTTP_INVENTORY_STRICT=1 go test ./app/controllers -run '^TestRegistryMatchesB0Inventory$' -count=1`: expected failure, `missing=241, extra=0`; the marker covers the 240 routable exported methods plus nine route aliases, while the current registry has 8 entries. This is the explicit migration gap until B1-B6 registrations land, so AC-H1 remains partial. No real HTTP/Mongo replay was run.
- Default (CI) mode of the same test is a ratchet: it fails on registrations outside the inventory or when registered inventory actions drop below `minRegisteredInventoryActions` (8), and only logs the remaining gap. Raise the floor as B2-B6 land.

## B1 focused evidence (2026-09-26)

- `go test ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/controllers/admin -count=1`: passed. Covers locale precedence, 27 template function names, `ViewArgs`, ContentRoots/BackupRoot validation and error-code mapping, both `public` and `public/upload` static roots, admin typed consumer fixture, and identity 405/`Allow` registration. Existing session evidence is `TestAppWritesSessionCookieBeforeActionResponse`, `TestAppUsesInjectedSessionWriter`, `TestAppRejectsSessionCommitFailure`, and `TestAppPreservesActionFailureWhenSessionCommitAlsoFails`.
- `go vet ./app/httpserver ./cmd/leanote ./app/controllers/api ./app/controllers/admin`: passed.
- `go build ./...`: passed.
- Git Bash `sh sh/package.sh`: passed; tar listing contains `bin/leanote` and no `var/` tree, so absolute `/var/lib/leanote` roots are created by deployment rather than embedded under an `/app` extraction prefix.
- `gofmt -l` over all B1 Go files: no output; `python ./.trellis/scripts/task.py validate 09-08-interface-http`: passed (`implement.jsonl` 16 entries, `check.jsonl` 12 entries); `git diff --check`: passed.
- `go test ./... -count=1`: partial. The B0 registry test still reports the documented `missing=241` migration gap. Harness tests also fail before server startup because the generated Revel entrypoint contains expired module paths and legacy module imports; this is not real HTTP evidence for B1.
- Real listener requests, Mongo/Golden replay, production exit-78 process checks, container volume/non-root/restart checks, browser smoke, and full harness migration remain `unrun`; no focused in-process result is promoted to those gates.

## 下游移交（本任务不关闭）

- Mongo 7 standalone、Mongo 8 replica set、kill/restart、failpoint、真实浏览器 8 槽、容器 paired volume/non-root/restart、PDF、tarball/GHCR → `delivery-verification`。
- 前端 `submissionId`/`OperationId` 生成与复用、`registered_relogin_required` 展示 → `presentation-frontend`。
