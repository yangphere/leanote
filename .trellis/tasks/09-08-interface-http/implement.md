# 接口适配层：标准库 HTTP 与路由 — 执行计划

每个批次独立提交、独立运行门禁、独立回滚（D-H1：不拆子任务）。D-H1～D-H7 已于 2026-09-25 确认，冻结结论见 `research/spec-audit-2026-09-25.md` §6。

## B0 盘点与对账骨架

- [ ] 生成 `research/action-inventory.md`：`conf/routes` 95 条（显式/Static/catch-all）× 约 247 个 controller 方法，逐项标注 routable/非路由辅助、owner application 任务、当前 method、BEFORE/`commonUrl`、响应类型、Golden 覆盖。
- [ ] 读取已归档交接矩阵：identity `acceptance/evidence-matrix.md`、persistence、notes `research/action-inventory.md`、content `research/action-inventory.md`、publishing/admin `research/action-contract-inventory.md`、domain `research/input-contracts.json`。
- [ ] 增加 registry ↔ inventory 对账测试（先失败，随批次转绿）；增加“导出未注册方法不可达”负例。

## B1 runtime 与 production-config

- [ ] 生产 App 装配 `LocaleResolver`、SessionReader/SessionWriter；补 27 个模板函数名集冻结测试与 `ViewArgs` 框架键。
- [ ] 抽取只读 `ProductionConfig`（Addr、ShutdownTimeout、DatabaseName、`ConfiguredDatabaseIdentity`、`CredentialProviderRef`）并提供 admin consumer fixture；删除 admin 侧重复解析（仅接线，不改 admin 业务规则）。
- [ ] D-H4：解析 `content.private.data`/`content.private.quarantine`/`content.public.data`/`content.public.quarantine`/`content.temporary`/`admin.backup.root`，调用 content validator，补齐错误码与 exit 78 负例；`/upload/*` 与 `/public/upload/*` 改从 public data root 提供；删除 `initializeContentRuntime` 兼容映射。
- [ ] D-H4：同步 `Dockerfile`（卷与 `/var/lib/leanote/tmp`）、`sh/package.sh` 与 `docs/modernization/cicd-delivery.md`（新键、三卷布局、旧 `/app/files`/`/app/public/upload` 一次性迁移步骤）。
- [ ] D-H5：identity 方法矩阵 405 与 `Allow` 头；其他路由保持观察值的 route negative。

## B2 identity/API 身份批

- [ ] `ApiUser` 注册；`ApiAuth` 余量；`_token/_userId` 写回时序；显式 invalid token 不 fallback。
- [ ] 主站 `Auth`、`Captcha`、`Index`、`User`；P-01/P-06/P-07/U-04 contract + replay。

## B3 notes/content 批

- [ ] 主站 `Note`、`Notebook`、`Tag`、`NoteContentHistory`；notes `OperationId`/`ExpectedUsn` binder 与 result mapper；`/note/deleteTrash` alias 保留。
- [ ] 主站 `File`、`Attach`、`Album`；api `ApiNote`/`ApiNotebook`/`ApiTag` 余量/`ApiFile`。
- [ ] D-H6：`GetImages`/`GetAlbums` 依赖失败 500 + legacy body 形状，新增 Golden 失败 variant。
- [ ] D-H7：`CopyHttpImage` 保持现有 wire；在 `docs/modernization-backlog.md` 新增 MOD 条目。

## B4 publishing 批

- [ ] 主站 `Blog`（含 8 处 JSONP、Q-P5 callback 过滤、Q-P9 `submissionId` 必填）、`Share`、`Preview`；博客自有 `html/template` 路径与主题 Preview 错误展示保持。

## B5 member 批

- [ ] `member/*` 全部 action，按 `MemberX.Y` 前缀改写与 LoginRequired BEFORE。

## B6 admin 批

- [ ] `admin/*` 全部 action，admin principal 与 `ProductionConfig` consumer。

## B7 汇合

- [ ] 提取 `needValidateAPI` 与 `needValidateWhitelist` 的重复白名单为单一 helper；registry 对账测试全绿。
- [ ] 各批所用 service/lea 的 `revel.Config`/`revel.BasePath`/`revel.AppLog` 调用改走第一方 config/logger seam。

## B8 入口与 harness（D-H2、D-H3）

- [ ] D-H2：`cmd/leanote` 增加 dev/test run mode，读取仓库 `conf/app.conf` `[dev]`/`[test]`，test 强制 `leanote_test`；仓库 conf 补 content/backup 相对默认值；互斥负例；prod 路径不变。
- [ ] harness 改为构建运行 `cmd/leanote -runMode test`；Golden/USN/权限/admin/member/页面 smoke 在新入口全量通过后方可进入 B9。
- [ ] D-H3：`sh/run.sh` 改为 `go run ./cmd/leanote -runMode dev`（无 watch，文档说明手动重启）；CI（含 `scripts/ci/update-build-summary.mjs` 的 `revel-cli` 步骤名）改用新入口。

## B9 Revel 删除

- [ ] 删除 `app/init.go` 旧链、`app/lea/route`、`app/cmd/`、所有 Revel controller 签名与 `revel.Result`；`go mod tidy`。
- [ ] `conf/app.conf(-default)` 移除 Revel 专用键；第一方解析器对旧文件遗留键忽略的测试。
- [ ] 同步 `CLAUDE.md`、`AGENTS.md`、README 与部署文档；延期项登记 `docs/modernization-backlog.md`。

## Validation

```bash
python ./.trellis/scripts/task.py validate 09-08-interface-http
go build ./...
go vet ./...
go test ./app/httpserver ./app/controllers/... ./cmd/leanote -count=1
go test ./...                                   # 需要 MongoDB
LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/... # 需要 MongoDB 8 standalone
npm test
rg -n 'github\.com/revel|revel\.' app cmd go.mod go.sum sh conf scripts .github Dockerfile
git diff --check
```

真实 HTTP/Mongo 证据写入 `acceptance/evidence-matrix.md`，记录命令、发现/执行数量、commit；缺环境记 `blocked`。

## Review gates

- 每批结束运行 `trellis-check`，并更新 evidence matrix 对应行。
- B8 结束前禁止删除任何 Revel 依赖；B9 结束前禁止宣称 AC-H11 通过。
