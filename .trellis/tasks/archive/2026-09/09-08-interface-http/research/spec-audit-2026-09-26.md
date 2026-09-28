# interface-http 规格复核（B2 编码前）— 2026-09-26

审核范围：本任务 `prd.md`/`design.md`/`implement.md`/`acceptance/evidence-matrix.md`/`research/*`，
父任务 `09-08-business-layer-architecture`，已归档 `application-identity/content/admin` 的相关交接，
以及 B1 提交 `5c665ba8` 之后的实际代码。审核阶段只修改任务规格与验收材料，未触碰业务实现。

## 1. Ready 叶选择

| 任务 | 状态 | depends_on | 父任务轨道 | 判定 |
|------|------|------------|------------|------|
| 09-08-interface-http | in_progress（B0/B1 已提交） | domain、identity、notes、content、publishing、admin、persistence，全部已归档 | Phase 3 | **选中**：唯一 Phase 3 未完成叶，已激活，无需再 `task.py start` |
| 09-08-presentation-frontend | planning | domain、notes（已归档） | Phase 4 | ready，但轨道顺序靠后 |
| 09-08-delivery-verification | planning | 含 interface-http、presentation-frontend | Phase 4 | 未 ready |

不创建新任务；下一编码批次为 B2（identity/API 身份批）。

## 2. B1 之后的实测事实

| # | 事实 | 证据 |
|---|------|------|
| T-1 | PRD“现状基线”已过期：LocaleResolver、SessionReader/Writer 已在 B1 装配，content roots 已改为 D-H4 键；registry 实际是 8 个 action（有 PDF config 时），不是 9 个 | `cmd/leanote/main.go:97-111`、`research/action-inventory.md` Census |
| T-2 | **新入口中 `revel.Config == nil`**。`ConfigService.InitGlobalConfigsWithError` 在第一行因 `revel.Config == nil` 直接返回错误，`cmd/leanote` 只记录日志后继续运行。结果是 `adminUserId`、`siteUrl`、`openRegister`、`demoUserId/demoUsername` 等全局配置为空快照：admin 角色永远无法判定，注册永远关闭，头像 URL 缺少站点前缀，`PrincipalPolicyFromConfig` 把“配置未加载”当成“未配置 demo”，demo 保护失效。B1 的单元测试没有覆盖到这一点 | `app/service/ConfigService.go:53-61`、`cmd/leanote/main.go:84-86`、`app/controllers/api/httpserver.go` `PrincipalPolicyFromConfig` |
| T-3 | 新入口中 `revel.BasePath = appBase`（应用树），但 D-H4 已把私有/公开数据移到 `/var/lib/leanote/*`。以下 Revel 全局使用点在 prod 下会读写错误位置，implement.md 只登记了 ThemeService（B4）和 AdminData（B6）：`ApiUserController.go:150`（头像写入）、`AttachService.go:637/812/825/945/965`（附件读取/打包）、`BlogController.go:116,168` 与 `lea/blog/Template.go:104,115`（主题渲染读取、`results.chunked`）、`NoteController.go:152`（`mode.dev`）、`ConfigService.go:551/657-664/726-738/794-813/876/964`（备份根、db.*、site.url）、`db/Mgo.go`/`mongo_client.go`（dev 连接）、`lea/i18n/i18n.go`（默认语言、cookie 名、messages 目录） | `rg 'revel\.(BasePath|Config|RunMode|DevMode)' app` |
| T-4 | `ApiUser.UpdateLogo` 在 controller 内直接 `ioutil.WriteFile` 到 `revel.BasePath/public/upload/<userId>/images/logo`，绕过 content 的单一 `ContentRoots` resolver。Web 头像走 `FileService.UploadImage(ImageUploadAvatar)`，路径多一个 `Digest3` 段，并写入 `Files` 行。上游 identity F-14 只规定“先校验 multipart、检查落库结果、文件清理归 interface/infrastructure”，content 没有接手这条路径 | `ApiUserController.go:100-190`、`service/image_actions.go:78-81`、identity `spec-audit-2026-09-09.md` F-14、content PRD L54 |
| T-5 | identity U-05 方法矩阵只写在已归档 identity PRD 中，本任务 PRD 没有展开。矩阵包括 `ApiUser` 五个 action（`info` GET，其余 POST）以及 `ApiFile.getImage/getAttach/getAllAttachs` GET-only；显式 `userId` 与 principal 不一致时，已认证返回 `forbidden`，未认证返回 `not_authenticated` | identity `prd.md:59-73` |
| T-6 | `Index.Suggestion`（B2 范围，业务 owner 为 admin）的 `submissionId` 是**可选**字段：登录请求可带，匿名请求带 ID 时返回 `validation`，匿名请求不带 ID 时仍按旧流程提交。PRD 只写了 `CommentPost` 的“必填”规则，实现者可能把两者混用 | admin `prd.md:67,90,93`、`IndexController.go:37` |
| T-7 | `httpserver.Params` 只有 `Get/String/Int/FormFile`：`Int` 解析失败时静默返回默认值，也没有 `Bool`（Revel `Atob`）、`Has`、嵌套键（`Tags[0]`、`Files[0][LocalFileId]`）或 `[]string`。PRD R2 要求“整数溢出、非法 hex 显式失败”并保留 Revel 可观察语义，但没有规定哪些参数严格、哪些保留 Revel 的宽松默认 | `app/httpserver/request.go` |
| T-8 | 迁移方式是**双实现**：旧 Revel controller 保留供 harness 使用，新 `*Server` 复制 action 逻辑（例如 `ApiAuthServer`）。design 没有规定两份实现的同步规则，迁移后仍可能有人修改旧版本 | `app/controllers/api/httpserver.go`、`ApiAuthController.go` |
| T-9 | 旧 harness 目前根本无法启动：Revel 代码生成器（`app/cmd`）会扫描仓库内被 git 忽略的嵌套 worktree `.claude/worktrees/diff-review-four-layer-c1d5e2`，生成的 `app/tmp/run/run.go` 因而导入了 `github.com/leanote/leanote/...` 和该 worktree 的模块路径。B1 证据中“expired module paths”的归因不准确，这是本地环境污染，与代码回归无关 | `go test ./app/tests/harness -count=1` 输出、`git worktree list` |
| T-10 | 本机 `127.0.0.1:27017` 拒绝连接，Mongo 8 standalone 当前不可用 | TCP 探测 |
| T-11 | 新入口目前只有 prod 模式（需要 `/etc/leanote/app.conf` 和非 localhost 的 Mongo），`-runMode test` 要到 B8 才实现。因此 B2–B6 在新 runtime 上没有可用的真实 HTTP replay 环境，但 AC-H5/H6/H9 又要求“真实 HTTP replay” | `cmd/leanote/main.go` `validateCLIOptions`、implement.md B8 |

## 3. 发现与处置

| ID | 类别 | 发现 | 处置 |
|----|------|------|------|
| G-01 | 过期 | PRD 现状基线仍是 B1 之前的状态（T-1） | 重写 PRD 基线节 |
| G-02 | 缺陷/遗漏 | 新 runtime 的全局配置快照永远为空（T-2），直接影响 B2 的 admin/demo/register/头像行为 | 可推导部分写入 PRD R4/R3：first-party config seam 必须向 `ConfigService` 提供 `adminUsername`、`site.url`，所有 run mode 都要在注册 registry 前完成；implement 新增 B2.0。快照不可用时的运行策略无法从上游推出，列为 **D-H9** |
| G-03 | 遗漏 | Revel 全局使用点没有逐项分配到批次（T-3） | design 新增 §7 消费点 → 批次表；implement 在 B2/B3/B4/B6/B7/B8 分别登记 |
| G-04 | 冲突 | API 头像写入绕过 `ContentRoots`，且与 Web 头像路径不一致（T-4） | 列为 **D-H10**；确认前 B2 不实现 `UpdateLogo` 的存储部分 |
| G-05 | 歧义 | U-05 矩阵没有展开（T-5） | PRD R3 内联完整矩阵和 `userId` 不一致时的错误映射；`ApiFile` 三个 GET 归入 B3 |
| G-06 | 歧义 | Suggestion 与 CommentPost 的 `submissionId` 规则不同（T-6） | PRD R2 分别写明；Suggestion 的校验在 service 内完成，binder 不做必填检查 |
| G-07 | 遗漏 | binder 严格/宽松边界不明确（T-7） | PRD R2 冻结规则：只有上游合同点名的字段严格；其余保持 Revel 的零值语义；每个严格字段必须在 inventory 中列出来源；`Params` 需补 `Has/Bool/Strings/嵌套键` |
| G-08 | 遗漏 | 双实现没有同步规则（T-8） | design §2 新增冻结规则：action 注册到 registry 后，其 Revel 版本冻结到 B9，只允许为同步新 handler 的缺陷修复而改动；业务判断优先提取为共享 core |
| G-09 | 证据/环境 | 旧 harness 被嵌套 worktree 污染（T-9），且 Mongo 不可用（T-10） | 记为环境阻塞 **E-1/E-2**；修正 evidence matrix 中 B1 的失败归因。审核阶段不删除 worktree |
| G-10 | 顺序冲突 | B2–B6 的“真实 HTTP replay”门禁在 B8 之前无法满足（T-11） | 列为 **D-H8** |

## 4. 待确认事项（不得擅自假设）

| ID | 问题 | 推荐 | 其他选项 | 不确认的影响 |
|----|------|------|----------|--------------|
| D-H8 | B2–B6 各批的真实 HTTP replay 何时、在哪个 runtime 上运行？ | **A**：把 D-H2 的 `-runMode test`（只读仓库 `[test]`、强制 `leanote_test`、localhost）和 harness 的“新入口目标”前移为 B1.5。之后每批用新入口 replay 本批 action 的 Golden 子集，旧 Revel harness 继续提供全量基线直到 B8。B8 只剩 dev 模式、`sh/run.sh`、CI 和全量切换 | **B**：保持原顺序，B2–B6 的批次门禁只要求 in-process `httptest` contract；AC-H5/H6/H9 的 replay 统一在 B8 之后补跑，期间保持 `unrun` | B2 的完成门禁无法定义。选 B 时，B2–B6 的回归要到 B8 才能暴露，届时需要一次性回滚多个批次 |
| D-H9 | 新 runtime 在全局配置快照不可用时（启动时 Mongo 不可达，或 `InitGlobalConfigsWithError` 失败）应如何运行？ | **A**：视为未就绪。`/healthz` 返回 503 `not_ready`；静态资源照常服务；其余请求返回 503，body 与 healthz 相同；后台带退避重试加载，成功后自动就绪。任何请求都不会在空快照上执行 | **B**：直接失败，exit 非 0，由编排系统重启。**C**：保持 Revel 行为，用空快照继续服务（admin 角色缺失、demo 保护失效、注册关闭） | 决定 B2 的 principal、demo 和 register 行为，也决定 healthz 合同是否扩展；C 存在安全风险 |
| D-H10 | `ApiUser.UpdateLogo` 的存储方式 | **A**：wire 不变。保留逻辑路径 `public/upload/<userId>/images/logo/<guid><ext>`、扩展名白名单、5MB 上限和响应 `Logo` URL；改为通过 content resolver/publish primitive 写入 `ContentRoots.PublicUpload`；不新增 `Files` 行；`UpdateAvatar` 失败时回收已发布文件并返回 `storage` | **B**：复用 Web 头像的 `FileService.UploadImage(ImageUploadAvatar)`。路径会多出 `Digest3` 段、新增 `Files` 行，校验规则变为 content 的规则，客户端可观察到差异 | B2 中 `UpdateLogo` 无法完成；D-H4 的 prod 布局下，头像目前会写进应用树 |
| E-1 | 环境：嵌套 worktree `.claude/worktrees/diff-review-four-layer-c1d5e2` 导致旧 harness 生成失败 | 由用户确认后执行 `git worktree remove`（或移出仓库目录）；另一种做法是让 `app/cmd` 生成器跳过点目录（属于代码改动，放入 B1.5/B8） | — | 在处理之前，旧 Revel 的全量 Golden 基线无法运行 |
| E-2 | 环境：本机没有 Mongo 8 standalone | 启动一个本地 Mongo 8（仅 `leanote_test` 库） | — | 所有真实 HTTP/Golden 证据保持 `blocked` |

## 5. 结论

G-01、G-03、G-05～G-08 属于可以直接推导的补充，已回写到 PRD/design/implement/evidence matrix。
下列内容在 D-H8～D-H10 确认之前不得开始编码：B2 中依赖全局配置的部分（principal、demo、register、头像 URL），
以及 `UpdateLogo` 的存储部分。以下内容不依赖这三项决策，可以在确认后直接开始：B2.0 的 config seam 前置项、
`ApiUser` 其余 action 和 Web `Auth/Captcha/Index/User` 的 binder/adapter 本体，以及 in-process contract 测试。

## 6. 决策记录（2026-09-26，用户确认“D-H8～D-H10 全部采用推荐”）

| ID | 冻结结论 |
|----|----------|
| D-H8 | 采用 A。D-H2 的 `-runMode test`（只读仓库 `conf/app.conf` `[test]`、强制 `db.dbname=leanote_test`、允许 localhost、与 prod 互斥）和 harness 的“新入口目标”前移为 B1.5。之后 B2–B6 每批在 `cmd/leanote -runMode test` 上 replay 本批已注册 action 的 Golden 子集，作为该批真实 HTTP 证据；旧 Revel harness 继续提供全量基线直到 B8。B8 只剩 dev run mode、`sh/run.sh`、CI 和全量切换。执行顺序为 B2.0 → B1.5 → B2：先交付与 run mode 无关的应用配置 seam，test 模式直接消费它。 |
| D-H9 | 采用 A。新入口在全局配置快照未成功加载前（启动时 Mongo 不可达，或 `InitGlobalConfigsWithError` 失败）视为未就绪：`/healthz` 返回 503 `{"status":"not_ready"}`；静态资源照常服务；其余请求返回 503，body 与 healthz 未就绪响应相同；后台按退避重试加载，成功后自动就绪。任何 action 都不得在空快照上执行。该策略覆盖 prod/dev/test 三种 run mode；Revel 入口在 B9 删除前保持原行为。 |
| D-H10 | 采用 A。`ApiUser.UpdateLogo` 的 wire 不变：逻辑路径 `public/upload/<userId>/images/logo/<guid><ext>`、扩展名白名单（gif/jpg/png/bmp/jpeg）、5MB 上限和响应 `{"Logo": "<siteUrl>/public/upload/..."}` 都保持原样。写入改走 content resolver/publish primitive，落到 `ContentRoots.PublicUpload`；不新增 `Files` 行；`UpdateAvatar` 失败时回收已发布文件并返回 `storage`。 |

E-1、E-2 仍是环境阻塞，由用户处理；处理前相关证据保持 `blocked`。
