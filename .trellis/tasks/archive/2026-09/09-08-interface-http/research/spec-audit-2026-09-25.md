# interface-http 规格审核 — 2026-09-25

审核范围：本任务 `prd.md`/`design.md`/`implement.md`/`implement.jsonl`/`check.jsonl`，父任务
`09-08-business-layer-architecture`，已归档上游 `domain-contracts`、`infrastructure-persistence`、
`application-identity/notes/content/publishing/admin` 的交接条目，下游 `presentation-frontend`、
`delivery-verification`，以及当前仓库实现。审核阶段未修改任何业务实现。

## 1. Ready 判定

| 任务 | 状态 | depends_on | 判定 |
|------|------|------------|------|
| 09-08-interface-http | planning → 本轮 start | domain、identity、notes、content、publishing、admin、persistence（全部已归档） | **ready，按父任务 design §4 迁移顺序第 4 位，选中** |
| 09-08-presentation-frontend | planning | domain、notes（已归档） | ready，但迁移顺序第 5 位 |
| 09-08-delivery-verification | planning | 含 interface-http、presentation-frontend | 未 ready |

## 2. 仓库现状事实（审核时实测）

| # | 事实 | 证据 |
|---|------|------|
| S-1 | Revel controller action 约 247 个：主站 119、api 30、admin 50、member 48（按 `func (c X) Y(...) revel.Result` 统计，含 BaseController 2 个非路由方法） | `app/controllers/**` |
| S-2 | 第一方 registry 当前仅注册 9 个 action：`TestE2e.Identity`、Note PDF 1 个、`ApiAuth` 4 个、`ApiTag` 3 个 | `app/controllers/httpserver.go`、`app/controllers/api/httpserver.go` |
| S-3 | `sh/package.sh` 与 `Dockerfile` **已**构建 `cmd/leanote`；因此当前 tarball/镜像对除 S-2 外的全部页面与 API 返回 404 `No matching action`。交付物目前不可用，这是本任务的起点而非回归 | `sh/package.sh:25-27`、`Dockerfile:16`、`app/httpserver/registry.go` dispatch |
| S-4 | `cmd/leanote` 只接受 `-runMode prod` 与 canonical `/etc/leanote/app.conf`（0440、非 localhost Mongo、非 `leanote_test`） | `cmd/leanote/main.go` `validateCLIOptions`、`app/httpserver/production_config.go` |
| S-5 | Golden/USN harness 仍构建并运行 Revel 生成的 `app/tmp` 二进制（`serverRunMode` 依赖 `github.com/revel/revel` 模块路径），运行模式为 test；`/_test/e2e/identity` 仅在 `RunMode=="test"` 可达 | `app/tests/harness/server.go:23,115-133,274` |
| S-6 | `sh/run.sh` 仍是 `revel run -a .`（watch 模式） | `sh/run.sh` |
| S-7 | `cmd/leanote` 未装配 `LocaleResolver`、`SessionReaderFactory`、`SessionWriterFactory`；Locale 永远为空，消息回落默认语言 | `cmd/leanote/main.go` App 字面量 |
| S-8 | `initializeContentRuntime` 仍用 application-base 派生的兼容映射（`<base>/files`、`<base>/.content-private-quarantine` 等）；production-config 没有任何 content root 键 | `cmd/leanote/main.go:initializeContentRuntime`、`production_config.go` |
| S-9 | 交付文档规定容器卷为 `/app/files`、`/app/public/upload` 两个独立挂载；而 content 合同要求 data 与 quarantine same-filesystem 且 no-overlap。若 quarantine 位于卷外（容器 rootfs），校验必然 cross-device fail closed；若位于卷内同目录则 overlap。**现行卷布局与 ContentRoots 合同不兼容** | `docs/modernization/cicd-delivery.md`、`09-08-application-content` contract、delivery PRD 要求 “data/quarantine paired volume” |
| S-10 | `conf/routes`：95 条（`*` 31、GET 57+`Get` 3、POST 4），Static 9 条，catch-all 3 条；`RouteTable.Match` 方法不匹配时返回 404（沿用 Revel），HEAD 映射 GET | `conf/routes`、`app/httpserver/registry.go:34-57` |
| S-11 | Revel 残留（非测试源码）：controllers 37 个文件、httpserver 4（仅注释）、db 2、service 3、lea 7、`app/init.go`、`app/cmd/**`、`app/tests/harness`；`conf/app.conf(-default)` 含 `module.static=github.com/revel/...`、`results.*` 等 Revel 键；`scripts/ci/update-build-summary.mjs` 含 `revel-cli` 步骤名 | `rg` 统计 |
| S-12 | `CommentPost` 已以 Revel binder 读取 `submissionId` 并用 `c.Has` 判缺失；notes 的 `OperationId`/`ExpectedUsn` 在 controllers 中 14 处引用 | `BlogController.go:1156-1164` |
| S-13 | `/healthz`、exit 78、`ConfigError` 稳定码已实现并有测试 | `app/httpserver/registry.go` dispatch、`health_test.go`、`production_config_test.go` |
| S-14 | admin 已定义 `CredentialProviderRef`（owner = interface/infrastructure）但生产入口无 producer；admin implement 将 “CredentialProviderRef 的 interface seam 接入” 列为未关闭门禁 | `app/service/BackupSecurity.go:117-128`、admin `implement.md:62` |

## 3. 发现与处置

| ID | 类别 | 发现 | 处置 |
|----|------|------|------|
| F-01 | 遗漏 | PRD 未陈述 S-2/S-3 基线，读者会误以为只剩清扫 | PRD 增加“现状基线”节 |
| F-02 | 范围 | 单叶承载 ~238 个 action 迁移 + Revel 删除 + 配置 + harness，超出父任务“可独立验收”粒度 | implement.md 改为 B0–B9 批次，每批独立门禁与回滚点；是否拆子任务列为 **D-H1** |
| F-03 | 冲突 | PRD 要求 harness/dev 迁到 `cmd/leanote`，但入口只允许 prod（S-4/S-5/S-6）；test 模式必需 `leanote_test` 与 localhost | 列为 **D-H2**（非 prod 配置来源）与 **D-H3**（dev watch）；PRD 增加“非 prod 来源不得成为 prod fallback”硬约束 |
| F-04 | 冲突/遗漏 | ContentRoots 无配置键、无稳定错误码表、与现行卷布局冲突（S-8/S-9） | 列为 **D-H4**；PRD 冻结校验顺序与错误码命名规则，键名/挂载路径待确认 |
| F-05 | 遗漏 | admin 需要的 `ConfiguredDatabaseIdentity`、`CredentialProviderRef`、backup root 未进入本任务 production-config 输出（S-14） | PRD 纳入前两项（有明确 owner 声明）；backup root 来源并入 **D-H4** |
| F-06 | 歧义 | 非 identity action 的 HTTP 方法策略未定义；多个上游把 method 冻结委托给本任务 | 列为 **D-H5**；推荐保持观察值，仅 identity 矩阵返回 405 |
| F-07 | 歧义 | AC “未注册 → 404/405” 未区分：未注册 action=404、方法不符的显式路由现状=404、identity 矩阵=405 | PRD AC 拆分为三条确定语义 |
| F-08 | 遗漏 | content 委托：`GetImages`/`GetAlbums` DB 错误 wire | 列为 **D-H6** |
| F-09 | 遗漏 | content 委托：`CopyHttpImage` operation identity | 列为 **D-H7**；推荐不扩 API、登记 backlog |
| F-10 | 可推导 | notes `WN-10 /note/deleteTrash` deprecated alias “由 interface-http 决定移除” | 本任务 Out of scope 已规定不改 URL → 保留 alias，不需用户决策 |
| F-11 | 遗漏 | 上游交接的 binder 失败语义未进入 PRD：domain（非法 hex/整数溢出/缺失/重复/`Tags` 字符串与数组）、publishing Q-P9 `submissionId` 必填与 Q-P5 callback、notes `OperationId`/`ExpectedUsn`、identity AC-I5/I6/I8/I11/I12/I13/I16/I17 | PRD 增加“上游交接承接清单”与对应 AC |
| F-12 | 遗漏 | S-7：i18n LocaleResolver、SessionReader/Writer 未装配到生产入口 | PRD 要求显式装配并有 contract 测试 |
| F-13 | 歧义 | Revel 零命中 AC 的范围与注释/测试/文档口径不明 | PRD 冻结扫描命令、范围与允许例外 |
| F-14 | 失效引用 | `implement.jsonl`/`check.jsonl` 仍指向 `.trellis/tasks/09-08-application-identity/*`、`09-08-application-content/*`，二者已归档 | 更正为 `archive/2026-09/...`，并补 notes/publishing/admin/domain 交接材料 |
| F-15 | 验收 | 未界定本任务自身须运行的真实环境与 delivery 分界 | PRD 规定最低证据：Mongo 8 standalone + 真实 HTTP replay；其余拓扑/浏览器/PDF/容器归 delivery；缺环境记 blocked |
| F-16 | 遗漏 | `conf/app.conf(-default)` 的 Revel 专用键去留未定义 | PRD：移除 Revel 专用键，第一方解析器对遗留键忽略且不报错（兼容旧部署文件）；prod 校验规则不变 |

## 4. 待确认事项（不得擅自假设）

| ID | 问题 | 推荐 | 不确认的影响 |
|----|------|------|--------------|
| D-H1 | 本任务是否拆成子任务（例如 runtime/config、主站、api、admin+member、harness+Revel 清扫）？ | 保持单叶，按 implement.md B0–B9 批次独立提交、独立门禁；若用户希望 PR 级独立归档再拆 | 仅影响管理粒度；不阻塞 B0–B2 |
| D-H2 | 非 prod（dev/test）运行的配置来源 | `cmd/leanote` 增加 `-runMode dev|test`，只从仓库 `conf/app.conf` 对应 section 读取，允许 localhost；`test` 强制 db=`leanote_test`；`prod` 分支保持现有 canonical 校验且不得读取仓库 conf；三者代码路径互斥并有负例测试 | 阻塞 B8（harness 切换）与 Revel 删除；不阻塞 B0–B7 的 handler 迁移（可用 in-process `httptest` 验证） |
| D-H3 | dev 模式是否需要 Revel 式 watch/自动重建 | 不提供 watch；`sh/run.sh` 改为 `go run ./cmd/leanote -runMode dev`，文档说明手动重启；如需 watch 另立任务 | 阻塞 `sh/run.sh` 定稿 |
| D-H4 | ContentRoots 与 backup root 的配置键、容器挂载布局和旧卷迁移 | 新增非敏感键 `content.private.data`/`content.private.quarantine`/`content.public.data`/`content.public.quarantine`/`content.temporary`/`admin.backup.root`；容器改为两个 paired 卷（如 `/var/lib/leanote/private/{files,quarantine}`、`/var/lib/leanote/public/{upload,quarantine}`），`public/upload` 的 URL 前缀不变，静态服务从配置的 public data root 提供；旧 `/app/files` 布局迁移由运维手册一次性移动，不做运行时 fallback | 阻塞 B1 production-config 定稿、delivery 容器证据；在确认前 `cmd/leanote` 的兼容映射保持但不得算完成 |
| D-H5 | 非 identity action 的 HTTP 方法 | 保持 `conf/routes` 观察值：`*` 与 catch-all 接受任意方法，显式 GET/POST 路由方法不符沿用 404；只有 identity 已决 U-05 矩阵返回 405 | 阻塞各批 route negative 的期望值 |
| D-H6 | `GetImages`/`GetAlbums` 依赖失败的 wire | `500` + `Content-Type: application/json` + 保留 legacy body 形状（空 Page/空数组），不再 200；前端按非 2xx 提示 | 阻塞 B3 content 批该两个 action 的 AC |
| D-H7 | `CopyHttpImage` 调用方 operation identity | 不在本任务新增参数；保持“每次请求新导入”，登记 `docs/modernization-backlog.md` 新 MOD 条目 | 若选择新增参数，需同步 presentation-frontend 与 Golden |

## 5. 结论（决策前）

规格经本轮修订后，除 D-H2～D-H7 外均可直接指导实现与验收。B0（盘点与 registry 骨架）和 B2–B7 中不涉及上述决策的 handler 迁移可在确认前开始；B1 的 ContentRoots 部分、B8 harness 切换、B9 Revel 删除必须等待对应决策。

## 6. 决策记录（2026-09-25，用户确认“D-H1～D-H7 全部采用推荐”）

| ID | 冻结结论 |
|----|----------|
| D-H1 | 不拆子任务；保持单叶，按 implement.md B0–B9 批次独立提交、独立门禁、独立回滚。 |
| D-H2 | `cmd/leanote -runMode dev|test` 只读仓库 `conf/app.conf` 的 `[dev]`/`[test]` section（可用 `-conf` 指向同格式的仓库内文件），允许 localhost；`test` 强制 `db.dbname=leanote_test`，否则 exit 78；`prod` 保持现有 canonical 校验且绝不读取仓库 conf。三条路径先按 `-runMode` 分派，互不回退，负例测试覆盖。 |
| D-H3 | 不提供 watch/自动重建；`sh/run.sh` 改为 `go run ./cmd/leanote -runMode dev`，文档说明修改后手动重启；如需 watch 另立任务。 |
| D-H4 | 新增非敏感 `[prod]` 键：`content.private.data`、`content.private.quarantine`、`content.public.data`、`content.public.quarantine`、`content.temporary`、`admin.backup.root`，全部必填、绝对路径、不得来自 `DEFAULT` 继承之外的第二来源。容器布局：`/var/lib/leanote/private/{files,quarantine}`（一个卷）、`/var/lib/leanote/public/{upload,quarantine}`（一个卷）、`/var/lib/leanote/backup`（一个卷）、`/var/lib/leanote/tmp`（镜像内非 root 可写目录，可不持久）。`/upload/*` 与 `/public/upload/*` 两个 URL 前缀都从已验证的 `content.public.data` 提供，URL 不变；`ServedRoots` 为 `public` 静态树与 public data root，quarantine 不得位于其中。旧 `/app/files`、`/app/public/upload` 卷由运维手册一次性移动到新布局，运行时不做任何 fallback 或自动迁移。dev/test 模式的对应键在仓库 `conf/app.conf` 中以仓库相对目录给出默认值（如 `files`、`.content-private-quarantine`），启动时解析为绝对路径后走同一 validator。 |
| D-H5 | 非 identity action 保持 `conf/routes` 观察值：`*` 与 catch-all 接受任意方法；显式 GET/POST 路由方法不符沿用现状 404（Revel 基线）；只有 identity U-05 矩阵外方法返回 405 + `Allow`。HEAD 继续映射 GET。 |
| D-H6 | `GetImages`/`GetAlbums` 依赖失败返回 HTTP 500、`Content-Type: application/json; charset=utf-8`，body 保持 legacy 形状（`GetImages` 为空 Page，`GetAlbums` 为 `[]`）；成功响应不变。这是受控 wire 例外，Golden 需新增失败 variant 并标注来源 D-H6。前端对非 2xx 的提示归 `presentation-frontend`。 |
| D-H7 | `CopyHttpImage` 不新增参数，维持“每次请求新导入”语义；在 `docs/modernization-backlog.md` 新增 MOD 条目记录调用方 operation identity 缺口（B3 实施）。 |

影响面：D-H4 改变容器挂载契约，`docs/modernization/cicd-delivery.md`、`Dockerfile`、`sh/package.sh` 的卷说明在 B1 同步；`delivery-verification` PRD 已要求 “data/quarantine paired volume”，与本决策一致，无需改动其规格。
