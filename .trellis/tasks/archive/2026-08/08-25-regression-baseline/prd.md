# 回归基线（G）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`
本子任务是**整棵任务树的前置门禁**：在它交付前，不动任何生产代码。

## Goal

在**未改动的旧代码**上建立可重复执行的回归基线，使后续 A–F（尤其 B 驱动迁移与 C-b 迁出 Revel）
的"没有破坏任何东西"能被**自动证明**，而不是靠人工比对。

## Background：为什么必须先做这个

现状不是"测试薄"，而是**测试当前根本跑不起来**（全部为实测/读源码所得）：

| 事实 | 证据 |
|---|---|
| `go test ./app/tests/...` **FAIL**，`panic: no reachable servers` | 实测 exit=1，11.6s；栈顶 `auth_test.go:13` → `db.Init` → `app/db/Mgo.go:109` |
| 同一 package 有 **3 个 `func init()`** 全部执行 ⇒ `db.Init` 调 3 次、`InitService()` 3 次、`InitGlobalConfigs()` 2 次 | `auth_test.go:12`、`config_test.go:12`、`note_content_test.go:18` |
| `config_test.go:13`、`note_content_test.go:19` 硬编码原作者 macOS 绝对路径 `/Users/life/Documents/Go/package_base/src`，**在任何其他机器上不可能成立** | 读源码 |
| `TestSendMail`、`TestApiFixNoteContent2` **无任何断言**，只有 `t.Log` ⇒ 是调试脚本 | 读源码 |
| `TestSendMail` 真的向 `life@leanote.com` **发邮件** | `config_test.go:21` |
| `TestApiFixNoteContent2` 硬编码原作者库的 ObjectId，不在本仓库 fixture 内 | `note_content_test.go:27-28` |
| `TestAuth` 是**唯一真实有效**的测试；fixture 支持已核实 | `users.bson` 中 admin `Pwd` = `e99a18c428cb38d5f260853678922e03` = `md5("abc123")`（实算） |
| `/api/*` 契约测试**不存在** | 无 |
| 本机无 `mongod`，但 **Docker 29.7.2 + Compose v5.4.0 可用、daemon 正常** | 实测 |

而下游是**无法回滚的已发布客户端**（桌面 / iOS / Android），`/api/*` 与 USN 语义是外部契约。

## Requirements

### R-G1 golden snapshot（HTTP 边界录制）

必须在 **HTTP 边界**录制 —— 起真实服务器、走真实 HTTP 请求、记录原始响应字节。
**不可**直接调用 controller 方法：C-b 会替换整个 HTTP 层，那样夹具自身会失效。

覆盖面（父任务 D5 已定；action 清单以**当前可分发的代码**为准，两轮源码审核修订）：

- **`/api/*` 全部 29 个 action**（34 个导出方法中 5 个处于块注释内，不可分发：
  `ApiNote.GetHistories`（`ApiNoteController.go:553-563`）与 `ApiFile.CopyImage` / `GetImages` /
  `UpdateImageTitle` / `DeleteImage`（`ApiFileController.go:26-57`））：
  - `ApiAuth`：`Login` / `Logout` / `Register`
  - `ApiUser`：`Info` / `UpdateUsername` / `UpdatePwd` / `GetSyncState` / `UpdateLogo`
  - `ApiNotebook`：`GetSyncNotebooks` / `GetNotebooks` / `AddNotebook` / `UpdateNotebook` / `DeleteNotebook`
  - `ApiNote`：`GetSyncNotes` / `GetNotes` / `GetTrashNotes` / `GetNote` / `GetNoteAndContent` /
    `GetNoteContent` / `AddNote` / `UpdateNote` / `DeleteTrash` / `ExportPdf`
  - `ApiTag`：`GetSyncTags` / `AddTag` / `DeleteTag`
  - `ApiFile`：`GetImage` / `GetAttach` / `GetAllAttachs`
  - 被注释掉的 4 个 `ApiFile` 功能在 web 层 `FileController` 有活跃实现
    （`FileController.go:218/223/229/251`），其覆盖归入 R-G1 web 层清单，**不得**伪造 API 端点。
  - **二进制端点**（`GetImage` / `GetAttach` / `GetAllAttachs` / `ExportPdf`，共 4 个）只钉
    Content-Type 与非空，不做字节比对；header 规则按 design §3 的**二进制专用闭合集**执行
    （`Accept-Ranges: bytes` 与 `Content-Disposition`（含 filename，四端点取值均与机器无关）
  严格比较；`Last-Modified` 排除）。仓库无物理上传文件，成功路径由 harness 在用例前
  用固定字节运行时创建 `files/test_seed/` 并插入 `files`/`attachs` 记录，用例后清理，
  不依赖被 `.gitignore` 忽略的预置文件。
    - `ExportPdf` / `GetAllAttachs` 有文件系统副作用（见 R-G7）。
    - **`ExportPdf` 平台边界**：其实现硬编码 `/bin/sh`（`ApiNoteController.go:623`）并以
      `site.url` 拼回调地址（`:614`）——Windows 本地**无法执行**。其首份 golden 由 R-G5 的
      **受控 Linux record job**（workflow_dispatch + 预装 wkhtmltopdf）生成、经人工审核后入库，
      之后 CI 只跑 replay；Windows 本地运行只允许产出显式
      "平台不支持"记录，**该跳过不计入 G-AC1 对该端点的完成**。

- **失败路径与错误信封**（父任务 R-G 明确要求成功/失败路径都录）：
  - 每个需要 token 的 action 至少录一条无 token 与一条无效 token 的失败响应。
  - `ApiFile.GetImage` / `GetAttach` / `GetAllAttachs` 在现有 `commonUrl` 中明确为公开读取端点，
    不属于上述需要 token 的 action；仅录制无 token 的当前访问语义，不把 200 响应误标为鉴权失败。
  - 覆盖参数校验失败（如 `GetNotes` 对非法 `notebookId` 返回 `notebookIdInvalid`）、资源不存在、
    所有权越权读（用 demo 身份访问 admin 的 noteId/notebookId）。
  - `{Ok:false, Msg}` 错误信封的字节形态按现状钉住（G-AC3 的信封规则同样适用于失败响应）。
- **web 层所有权敏感 controller**：`NoteController`、`NotebookController`、`TagController`、
  `ShareController`、`AttachController`、`AlbumController`、`FileController`。

必须包含 `ApiNote.GetNoteContent`（`app/controllers/api/ApiNoteController.go:195-211`），
它把 `info.ApiNoteContent` 的 `bson.ObjectId` 字段直接下发，是 ObjectId JSON 形态的活契约。

### R-G2 USN 语义回归（按映射表验证 sync-visible mutation）

USN 断言**只针对有 `GetSync*` 语义的 mutation**，按下列映射表逐对验证；
映射表之外的 mutation（auth / user profile / file）**明确不纳入** USN 断言
（`UserService.IncrUsn` 的注释也只约束 notebook/note/tag 域，`UserService.go:15-16`）。

| GetSync* 端点 | 配对 mutation（预期 bump） | 已知旧行为偏差（按现状钉住 + 记 issue，不在 G 修） |
|---|---|---|
| `GetSyncNotes` | `AddNote`、`UpdateNote`、`DeleteTrash` | 无已知偏差 |
| `GetSyncNotebooks` | `AddNotebook`、`UpdateNotebook` | **`DeleteNotebook` 不 bump**：API 走 `DeleteNotebookForce`（`NotebookService.go:303-312`），校验 usn 后直接物理删除且不调 `IncrUsn` |
| `GetSyncTags` | `AddTag`（新建/复活时 bump，仅更新 count 不 bump） | **`DeleteTag` 写入旧 usn**：`DeleteTagApi` 调 `IncrUsn` 取 `toUsn` 返回（`TagService.go:122`），但写库用的是传入旧 `usn`（`TagService.go:123`） |

- 冲突分支：`UpdateNote` 的 usn 不匹配判断在 controller 层生效
  （`ApiNoteController.go:372-376`）；`NoteService.go:447-454` 的同判断已被注释。
  `DeleteTrash` / `DeleteNotebook` / `DeleteTag` 亦有各自冲突分支，一并按现状录制。
- 手法：写操作以**成对**形式覆盖 —— 调 mutation → 断言响应 → 再调对应 `GetSync*`
  断言 delta 语义（对上表"无已知偏差"项，断言 `Usn` 已增且 delta 含该变更；
  对已列偏差项，钉住当前实际行为）。
- **同步边界**：`afterUsn=0` 全量、`afterUsn=当前最大 Usn` 空 delta、
  `maxEntry` 截断翻页（两页拼接 = 全量）、超大 `afterUsn` 空 delta
  （`maxEntry=0` 时服务端默认 100，`ApiNoteController.go:96-99`）。三个 `GetSync*` 同理。

### R-G3 smoke（非 golden）

- 包级：`admin`（37 处 `RenderJSON`）、`member`（29 处）。
- controller 级：`Blog`、`Preview`、`Auth`、`Index`。
- 页面级：`/`、`/login`、`/note`、`/blog`、`/demo`。
- 判定分层：
  - admin/member 包级与 controller 级 JSON 端点：HTTP 状态码 + JSON 可解析 + 关键字段存在。
  - 页面级端点返回 HTML：逐页显式列出期望状态码（含未登录重定向语义，如 `/note` 未登录的跳转目标）、
    断言关键 HTML 标记存在；不做 JSON 解析。
- 凭证方式：web 层 smoke 用 session cookie（`POST /doLogin` 登录取得），`/api/*` 用 token；
  两套凭证相互独立，golden 与 smoke 均不得混用。

### R-G4 清理既有 `app/tests/`（父任务 D6 已定）

- 保留 `auth_test.go` 的 `TestAuth`；把包级 `init()` 改为无 Mongo 时可跳过，
  使 `go test -run` 在无 Mongo 环境下可选择性执行。
- 删除 `config_test.go`、`note_content_test.go`、`db_test.go`、`tmp.go`、`reg_test.go`。
  （`reg_test.go` 为审核时发现的遗漏项：与 `TestSendMail` 同类 —— 无断言、仅 `t.Log`
  的调试脚本，含硬编码 `localhost:9000` 与他人库 ObjectId，符合本节"删除调试脚本"的既定标准。）

### R-G5 GitHub Actions

- Go 测试 job：**MongoDB 5.0** 服务 + 从 `mongodb_backup/leanote_install_data/` 恢复 fixture，
  **只跑 replay 模式**。
  （版本依据：旧实现使用 `mgo.v2`，只发送 MongoDB 5.1 起移除的 legacy opcode
  ——见 `research/external-facts.md` §1——**旧代码在 7.0/8.0 上无法执行任何 CRUD**。
  5.0 已 EOL，此处仅作为"旧实现基线"的测试专用环境；7.0/8.0 的验证属于 B 阶段
  驱动迁移的验收，不由 G 承担。）
- **CI restore 机制**：不用 `services:`（其对 runner 无稳定 `docker exec`/`docker cp` 通道）；
  Go job 在 step 中先幂等 `docker rm -f leanote-test-mongo || true`，再
  `docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0`（容器名由我们
  控制），执行 60 秒 ping loop 后与本地**完全相同**的 cp/exec/restore 命令，显式设置
  `LEANOTE_GOLDEN=replay` 运行测试，job cleanup 再次移除容器，测试连 `localhost:27017`。
  不在 runner 安装 mongodb-database-tools（mongorestore 由容器自带）。
- **ExportPdf 首录的受控 record job**：单独的 `workflow_dispatch`（手动触发）Linux job，
  runner `ubuntu-22.04`，安装 `wkhtmltox_0.12.6.1-3.jammy_amd64.deb`（安装到 `/usr/local`，
  恰为生产代码默认查找路径 `/usr/local/bin/wkhtmltopdf`），安装后强制
  `test -x /usr/local/bin/wkhtmltopdf`（失败即 job 失败）；以 `LEANOTE_GOLDEN=record` 生成
  ExportPdf golden 并以 artifact 上传供人工审核；审核入库后 PR/push 的 replay job 才把它
  纳入比对。CI 的 PR/push 流水线**永不**运行 record。
- Node 24.x 执行 `npm test`；这里只建立基线工作流，最终矩阵与发布由 F 接管。
- 本子任务**不删** `.travis.yml`（那是 F 单元的 R-F1）。
- G-AC8 的验收方式（已裁决）：实现完成后 push，由 GitHub Actions **实际运行**并回填证据；
  仅本地静态校验不算完成。

### R-G6 本地可重复执行

提供一键起 MongoDB 并恢复 fixture 的方式（Docker，因本机无 `mongod`），
使 golden snapshot 的录制与回放在开发机上可重复。
该一键方式必须在本开发机的实际平台（Windows / PowerShell）上可执行，实现为固定入口
`go run ./app/tests/harness/cmd/env up`（内含 ping 等待；`down` 清理）；
夹具以 Go 代码直接 `exec` Go 工具链构建服务器，不依赖 bash 脚本（`app/cmd/gen_tmp.sh` 只作参考语义）。
命令必须是**具体可执行**的（含容器名、fixture 拷贝方式、mongorestore 执行位置），
不得留"实现时确定"的占位。

### R-G7 平台、端口与副作用约束

- 夹具的构建、启动、restore 全部由 Go 代码驱动（`exec go` / `docker`），跨 Windows/Linux 可用；
  禁止在测试路径中调用 `sh` 脚本。
- `db.url` / `db.urlEnv` 必须保持单行；续行会产生字面换行并导致旧版 `mgo.Dial` 失败，守卫必须在启动前拒绝。
- URL 没有数据库路径段时，守卫必须复刻 `db.Init` 回落到已验证的 `[test] db.dbname=leanote_test`。
- **端口一致性**：`ExportPdf` 以 `site.url` 拼回调地址（`ApiNoteController.go:614`），
  而 DEFAULT 段 `site.url=http://localhost:9000`（`conf/app.conf:17`）。
  基线服务器必须仅绑定回环地址 `http.addr=127.0.0.1`，使用**固定测试端口 28017**，
  并在 `[test]` section 内以 `site.url=http://127.0.0.1:28017` 覆盖，保证
  wkhtmltopdf 回调可达且 Windows 不向测试可执行文件发起公共/专用网络防火墙放行请求；
  端口被占用时显式失败，不回退随机端口。
- 文件系统副作用（每次录制后清理，不得进入版本库）：
  - `ExportPdf` → `files/export_pdf/*.pdf`；
  - `GetAllAttachs` → `files/<note.Title>.tar.gz`（`ApiFileController.go:107-113`）。
- HTTP 易变 header 处理规则见 design §3：闭合的比较集与排除集，未列入任一集合的 header 默认失败。

### R-G8 录制/回放模式严格分离

- 归一化夹具必须有两种显式模式：`record`（写入 golden）与 `replay`（比对 golden）。
- 模式经**环境变量** `LEANOTE_GOLDEN`（`record` / `replay`）传入，缺省 `replay`
  ——不用 `go test` flag（自定义 flag 需 `-args` 且跨 Go 版本解析有差异）。
- `replay` 是默认模式：golden 文件缺失或失配即**失败**，不得自动生成或覆盖。
- `record` 仅允许人工显式调用（首次录制、经评审的 golden 更新、R-G5 的受控 record job）。
- CI 的 PR/push 流水线只运行 `replay`；每次运行（record 或 replay）前显式 restore fixture。

## Acceptance Criteria

- [ ] **G-AC1** 在**未改动**的生产代码上，`/api/*` 29 个可分发 action 与 7 个 web 所有权敏感 controller
      的 golden snapshot 全部录制完成并入库；其中需要 token 的 API action 均含至少一条失败路径用例
      （无 token / 无效 token / 参数非法 / 越权，按 R-G1 失败路径清单）。
      `ExportPdf` 的 golden 以 Linux CI 录制为准；Windows 的"平台不支持"跳过**不满足**本条对该端点的要求。
- [ ] **G-AC2** 连续两次以 `replay` 模式回放 golden snapshot 结果一致，且无 golden 文件被改写
      （证明归一化——字段路径限定的 body 占位符与闭合 header 集——已消除全部抖动来源；
      前提是 R-G8 的模式分离已实现，排除"每次重录"假绿）。
- [ ] **G-AC3** `ApiNoteContent` 的 `NoteId` / `UserId` 被钉在 `^[0-9a-f]{24}$`；
      时间戳被钉在 `2015-01-20T11:13:41.34+08:00` 形态；`{Ok, Msg}` 信封结构被钉住（成功与失败响应均适用）；
      JSON 键序不被归一化重排。
- [ ] **G-AC4** USN 验证按 R-G2 映射表逐对通过：每个 sync-visible mutation 后对应 `GetSync*`
      delta 语义符合映射表（无偏差项断言 `Usn` 递增且 delta 含变更；已列偏差项钉住当前实际行为，
      各记一条 issue）；同步边界用例（afterUsn=0 / 最大 Usn / maxEntry 翻页 / 超大 afterUsn）与
      各冲突分支均有录制。
- [ ] **G-AC5** `go test ./app/tests/...` 在**有** MongoDB 时全绿。
- [ ] **G-AC6** `go test ./app/tests/ -run TestAuth` 在**无** MongoDB 时不 panic（跳过而非崩溃）。
- [ ] **G-AC7** `config_test.go`、`note_content_test.go`、`db_test.go`、`tmp.go`、`reg_test.go` 已删除；
      仓库内不再有 `/Users/life/` 硬编码路径，也不再有会真实发邮件或无断言的调试测试。
- [x] **G-AC8** GitHub Actions 在 PR 上以 MongoDB 5.0 + fixture 实际跑通 Go 测试（含 golden replay），
      并在 Node 24.x 跑通 `npm test`；证据为真实 workflow 运行记录（已裁决选 (a)，静态校验不算完成）。
      2026-08-26 已由 draft PR [#3](https://github.com/yangphere/leanote/pull/3) 的 `pull_request`
      运行 [32871393901](https://github.com/yangphere/leanote/actions/runs/32871393901) 完成取证
      （head `0fce6e7b933166412142ad8b109edcdef414163a`）：`go-replay` 在 Ubuntu 22.04 上恢复
      MongoDB 5.0 fixture 后通过 `TestAuth` 与 Golden/USN replay；`node-tests` 使用 Node 24.19.0，
      `npm test` 发现并通过 10/10 个测试、失败 0 个。PR/push 事件中的 `record-export-pdf` 按设计跳过。
- [ ] **G-AC9** 一条文档化的本地命令即可起 Mongo + 恢复 fixture + 录制/回放基线。
- [ ] **G-AC10** 本子任务**未修改任何生产代码**（`app/` 下除 `app/tests/` 外零改动），
      可用 `git diff --stat` 证明。

## Out of Scope

- 任何生产代码改动（G 是纯基线，`app/tests/` 与 CI 配置除外）。
- 删除 `.travis.yml`（属 F 单元 R-F1）。
- Dockerfile 作为**交付物**（属 F 单元 R-F2）；G 只用容器跑测试用的 Mongo。
- 为 admin / member 建 golden snapshot（D5 已定只做 smoke）。
- 提升测试覆盖率本身 —— G 的目标是**钉住现有行为**，不是改善它。
- 修复通过基线发现的既有 bug —— 若发现，记录为 issue，按原样钉住当前行为，
  避免把"修 bug"和"证明未破坏"混在一起。

## 已裁决事项（2026-08-25 第二轮审核后）

1. **G-AC8 验收方式**：选 (a) —— 实现完成后 push，由 GitHub Actions 实际运行并回填证据；
   若暂时不能 push，可先交付 workflow YAML 作为阶段性产物，但 G 保持未完成，不得 archive。
2. **`reg_test.go`**：确认删除（无断言、仅 `t.Log`、硬编码 `localhost:9000` 与他人库 ObjectId），
   已列入 R-G4 / G-AC7。
3. **G 阶段 MongoDB 版本**：5.0（旧 `mgo.v2` 仅支持 legacy opcode，无法在 ≥5.1 上执行 CRUD；
   5.0 为 EOL 测试专用环境）。7.0/8.0 验证移交 B 阶段。

## 已知事实备注（无需动作）

- fixture 目录含 `leanote.has_share_notes.bson`、`leanote.ShareNotes.bson` 等非规范命名文件，
  `mongorestore --dir` 会按文件名建立带点号的同名 collection（历史遗留噪音）。
  按"golden 钉住现状"原则不清理；ShareController 用例只需确认读取的是规范集合
  `share_notes` / `has_share_notes` 的行为即可。
- `-runMode=test` 在当前 conf 下会直接 Fatal：Revel v1.0.0 要求 runMode 存在同名 section
  （revel.go:242-244 `HasSection` 检查）。design §2 的 `[test]` section 因此是硬前提而非可选优化。
- 5 个被块注释关闭的"僵尸方法"：`ApiNote.GetHistories`、`ApiFile.CopyImage/GetImages/
  UpdateImageTitle/DeleteImage`。不在 API golden 清单内；若后续任务要恢复，须先走显式的
  spec 变更，不得由基线夹具"顺带"复活。

## 一个必须接受的前提

golden snapshot 钉住的是**当前行为**，不是**正确行为**。若基线录制过程中发现既有 bug，
按现状钉住并单独记录，不在 G 里修 —— 否则 B/C-b 阶段无法区分"我改坏了"与"基线本来就是错的"。
