# interface-http 验收证据矩阵

决策 D-H1～D-H7 已确认（见 `research/spec-audit-2026-09-25.md` §6），矩阵中不再有决策类 blocked。

状态取值：`unrun`（未执行）、`blocked`（前置决策或环境缺失）、`partial`、`passed`。每条 `passed` 必须写明命令、发现/执行数量与 commit；mock、controller 直调或历史 artifact 不得记为真实 HTTP 证据。

| AC | 内容 | 证据类型 | 前置 | 状态 |
|----|------|----------|------|------|
| AC-H1 | 全部公开 action/静态前缀可达；registry ↔ inventory 对账 | route-table + 对账测试 | B0 inventory | unrun |
| AC-H2 | 未注册 404、导出未注册不可达、identity 405、其他方法策略 | route negative | B1 | unrun |
| AC-H3 | 响应/状态/Golden；binder 负例 | contract + Golden replay（Mongo 8 standalone） | B2–B6 | unrun |
| AC-H4 | API token 保持、旧 Web cookie 匿名、cookie 安全属性 | session contract + replay | B1 | unrun |
| AC-H5 | identity 方法/principal/`_ID`/P-01/P-03/P-06/P-07/U-04 | contract + 真实 HTTP replay | B2 | unrun |
| AC-H6 | `ApiUser` 可达、API Auth/User Golden（identity AC-I5） | Golden live replay | B2 | unrun |
| AC-H7 | production-config 校验、ContentRoots/BackupRoot、exit 78、healthz、admin consumer | config contract + 进程级 exit code 测试 | B1 | unrun |
| AC-H8 | LocaleResolver、27 模板函数、主题渲染/Preview | contract + 页面 smoke | B1、B4 | unrun |
| AC-H9 | D-H6 wire、notes OperationId/ExpectedUsn、publishing submissionId/callback | 真实 HTTP regression | B3、B4 | unrun |
| AC-H10 | harness 在 `cmd/leanote -runMode test` 全量运行；run mode 互斥；SIGTERM | harness 全量 + 负例 | B8 | unrun |
| AC-H11 | Revel 零命中、go.mod 无 revel、`app/cmd` 删除 | rg + `go mod tidy` diff | B8 通过 | unrun |
| AC-H12 | build/vet/test/npm/diff-check/validate | 质量门 | 全部批次 | unrun |

## 下游移交（本任务不关闭）

- Mongo 7 standalone、Mongo 8 replica set、kill/restart、failpoint、真实浏览器 8 槽、容器 paired volume/non-root/restart、PDF、tarball/GHCR → `delivery-verification`。
- 前端 `submissionId`/`OperationId` 生成与复用、`registered_relogin_required` 展示 → `presentation-frontend`。
