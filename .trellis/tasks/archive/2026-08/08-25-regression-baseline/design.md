# 回归基线（G）— 技术设计

## 1. 核心设计约束：基线必须活过 C-b

C-b 会替换整个 HTTP 层。若基线夹具依赖 Revel（直接调 controller 方法、用 Revel testrunner、
或断言 `revel.Result`），则 C-b 一动，夹具自身失效 —— 基线在最需要它的时刻消失。

**因此：基线只允许通过真实 HTTP 请求与服务器交互，对被测进程内部零假设。**

夹具唯一与框架相关的部分，是"如何构建并启动服务器二进制"。把它收敛到**一个** helper，
且全部用 Go 代码直接 `exec` Go 工具链实现（本开发机是 Windows/PowerShell，禁止依赖 bash 脚本；
`app/cmd/gen_tmp.sh` 只作为命令语义参考）：

```
app/tests/harness/server.go
  StartServer(t) -> baseURL, cleanup
    └─ buildServerBinary()   ← C-b 之后只改这一个函数
```

**固定端口，不用随机端口**：`ExportPdf` 以 `site.url` 拼回调 URL（`ApiNoteController.go:614`），
随机端口会使 wkhtmltopdf 回调到错误地址。基线服务器监听**固定测试端口 28017**
（harness 常量与 `[test]` section 各自引用同一值；`http://127.0.0.1:28017`），
启动前探测端口占用，被占用则**显式失败**并提示，不得回退随机端口。

`buildServerBinary()` 当前实现（C-b 之前，依据 `app/cmd/gen_tmp.sh` 与 `CLAUDE.md`）：

```bash
cd app/cmd && go run . build -v ../../ ./tmptmp && rm -rf ./tmptmp   # 生成 routes.go + tmp/main.go
go build -o "$tempDir/leanote" github.com/leanote/leanote/app/tmp
"$tempDir/leanote" -importPath=github.com/leanote/leanote -runMode=test -port="$port"
```

C-b 之后退化为 `go build -o "$tempDir/leanote" .` —— **只改 `buildServerBinary()` 一处**。

不使用 `sh/run.sh`（它跑 `revel run -a .`，watch 模式、端口固定、不适合测试），
也不启用 `conf/app.conf` 里已注释的 Revel testrunner 模块。

## 2. `results.pretty` 是字节级契约的隐藏变量（关键发现）

`conf/app.conf`：

```
[dev]   results.pretty=true    ← RenderJSON 走 MarshalIndent，输出带缩进
[prod]  results.pretty=false   ← 紧凑 JSON
```

⇒ **同一份代码在 dev 与 prod 下的 API 响应字节不同。** 真实客户端面对的是 prod 语义，
基线必须在 `results.pretty=false` 下录制，否则钉住的是一个没人见过的形态。

**设计：新增 `[test]` 运行模式**（在 `conf/app.conf` 追加一个 section，ini 格式兼容，
不改动任何既有键，因此不破坏既有部署）：

**这是硬前提，不是可选优化**：Revel v1.0.0 在 runMode 无同名 section 时直接 Fatal
（`revel@v1.0.0/revel.go:242-244` 的 `HasSection` 检查）——当前 `conf/app.conf` 只有
`[dev]`/`[prod]`，`-runMode=test` 现状下根本起不来。追加 `[test]` 后 `-runMode=test` 才可用，
且 `Config.SetSection("test")` 使 section 内键覆盖 DEFAULT，正是我们要的覆盖语义。

```
[test]
mode.dev=false            # 与 prod 一致，避免 dev 专属渲染
results.pretty=false      # ← 字节级契约的关键
watch=false
db.dbname=leanote_test    # 与开发库隔离
site.url=http://127.0.0.1:28017        # ExportPdf 回调依赖（DEFAULT 段是 :9000，app.conf:17）
log.*.output=stderr       # 便于排查，与 prod 不同但不影响响应字节
```

`db.Init` 的解析顺序为 `db.url` → `db.urlEnv` → `db.host`/`db.port`/`db.username`/`db.password`
（`app/db/Mgo.go:61-96`），且 `dbname` 可被 `db.dbname` 覆盖 ⇒ 只需在 `[test]` 里设 `db.dbname`
即可把测试指向独立库，无需改动 `db.host`/`db.port`。

注意 `db.Init` 在 dial 失败时 **panic**（`Mgo.go:107-109`）—— 这是实测 `go test` 崩溃的直接原因，
R-G4 的跳过逻辑必须在**调用 `db.Init` 之前**判断可达性，而不是试图 recover。

## 3. 归一化：保序是硬要求

响应含易变字段（新建文档的 ObjectId、`CreatedTime` / `UpdatedTime` / `PublicTime`、`Token`、`Usn`），
不能直接字面比对。但归一化实现有个陷阱：

> `encoding/json` 反序列化到 `map[string]interface{}` **丢失键顺序**，再序列化会按字母序重排。
> 而 JSON 键顺序由 Go struct 字段声明顺序决定，**是契约的一部分**（客户端可能依赖，
> 且键序变化本身就是"输出变了"的信号）。

**设计：在原始字节上按 JSON 字段名定位替换，不做 unmarshal→marshal 往返。**

```
原始响应字节 ─┬─→ 按 ("字段名":"值") 形态正则定位易变字段并替换为占位符 ─→ 与 golden 文件逐字节比对
              └─→ 同一正则的捕获组断言每个易变字段的格式
```

**为什么限定字段路径**：纯形态替换（任意 `"[0-9a-f]{24}"`）会把笔记正文、标题里的合法
业务内容误当 ObjectId/时间戳掩盖掉。因此替换只作用于**枚举出的易变字段名**，正则形如
`"(NoteId|UserId|NotebookId|TagId|FileId|...)":"<值形态>"`；字段清单在 normalize.go 中
集中维护。归一化器必须有 canary 单测：**正文 Content 中出现的 24-hex 字符串不被替换**。

替换规则（录制与回放两侧施加同一套；均为"字段名 + 值形态"的联合匹配）：

| 易变字段（示例） | 值形态 | 占位符 | 同时断言的形态 |
|---|---|---|---|
| `NoteId` / `UserId` / `NotebookId` / `TagId` / `FileId` 等 ObjectId 字段 | 24 位小写 hex | `"OID_TOKEN"` | 恰好 24 位小写 hex —— 若 B 之后变成 `{\"$oid\":…}` 或 base64，正则失配即失败 |
| `CreatedTime` / `UpdatedTime` / `PublicTime` 等时间字段 | RFC3339 带偏移 | `"TIME_TOKEN"` | 形如 `2015-01-20T11:13:41.34+08:00`（`API列表-v0.1.md` 明确记为历史契约） |
| `Token` | 非空字符串 | `"AUTH_TOKEN"` | 非空 |

好处：保序天然成立、golden 文件保持人类可读、正文业务内容不被误掩盖。

`Usn` **不做归一化**：它的值有语义（必须递增），由 R-G2 的成对断言单独覆盖，不进 snapshot 比对。

**HTTP header 规则**（父任务要求"header 严格比较"，但部分 header 天然易变，必须用**闭合集合**）：

| 集合 | header | 原因 |
|---|---|---|
| 比较集（严格逐字节比较） | `Content-Type`、`Location` | 迁移不得改变响应类型与重定向目标 |
| 排除集（跳过比较，清单写入 golden 元数据区） | `Date`、`Set-Cookie`、`Content-Length` | `Date` 每秒变化；session cookie 每次登录随机；归一化占位符长度 ≠ 原值，`Content-Length` 必然不同 |
| **未列入任一集合** | —— | **默认失败**：回放时出现的每个响应 header 必须能归入上述两集之一，否则判定失败，强制新增 header 走显式评审 |

两个集合都是穷举的封闭清单，在 normalize.go 中集中维护；新增排除项必须在 PR 中说明理由。
G-AC2 的"两次回放一致"以此规则为前提。

**二进制端点的专用闭合集**（`GetImage` / `GetAttach` / `GetAllAttachs` / `ExportPdf`）：
这些端点经 Revel `RenderFile` / `RenderBinary` 返回，header 形态与 JSON 端点不同——
`BinaryResult.Apply` 写 `Content-Disposition`（`revel@v1.0.0 results.go:399-406`），
`WriteStream` 经 `http.ServeContent` 写 `Last-Modified`（ModTime）与
`Accept-Ranges: bytes`（`revel@v1.0.0 server_adapter_go.go:460-468`；Go 标准库
`net/http fs.go` 的 `ServeContent` 无条件写 `Accept-Ranges: bytes`）。规则：

| 处理 | header / 部分 | 依据 |
|---|---|---|
| 比较 | `Content-Type`、`Accept-Ranges`（固定值 `bytes`） | 迁移不得改变；`Accept-Ranges` 由 `ServeContent` 固定写入 |
| 比较 `Content-Disposition` 的 disposition 类型（inline/attachment） | `attachment` / `inline` 语义即交付方式 | `results.go:386-388` |
| 比较 `Content-Disposition` 的 filename | **四个端点全部严格比较**——实际取值均与机器无关：`GetImage` = `filepath.Base(磁盘文件名)`（`controller.go:324`，不含 `revel.BasePath`）；`GetAttach` = `attach.Title`；`GetAllAttachs` = `note.Title + ".tar.gz"`；`ExportPdf` = `FixFilename(note.Title)+".pdf"`（guid 只用于磁盘路径 `ApiNoteController.go:599`，不进 header，header filename 见 `:634-640`） | 前三者来自 fixture/种子数据，ExportPdf 来自 fixture note 标题 |
| 排除 | `Date`、`Set-Cookie`、`Content-Length`、`Last-Modified` | `GetAttach` / `GetAllAttachs` / `ExportPdf` 的 ModTime 是 `time.Now()`（每次请求变）；`GetImage` 是文件 mtime（随种子写入时间变） |
| 未列入任一集合 | —— | 默认失败（同 JSON 端点规则） |

**二进制成功路径依赖 harness 种子**：仓库不提交物理上传文件（`files/`、`public/upload/`
均被 `.gitignore` 排除），`files.bson` / `attachs.bson` 记录指向的路径在裸检出上不可读。
因此 harness 必须在**每个成功用例前**用固定字节创建 `files/test_seed/` 下的种子文件，
再向 `leanote_test` 的 `files` / `attachs` 集合插入对应记录（`UserId`=admin、`Path` 为
**仓库内相对路径**——`GetImage` 按 `revel.BasePath + "/" + path` 解析（`ApiFileController.go:68`），
attach 另带 fixture 笔记的 `NoteId`/`Title`）。种子内容由 harness 内嵌的固定字节生成，
不依赖被 `.gitignore` 忽略的预置文件；用例结束必须删除 `files/test_seed/` 并清理相应
数据库记录。未种子的错误/空路径（如 `GetAttach` 的 "No Such File"）按现状钉住。

## 4. 状态确定性：fixture 与用例分层

mutation 会改库，后续读操作的 snapshot 会漂移。设计：

```
每次 suite 运行
  ├─ drop leanote_test
  ├─ mongorestore ./mongodb_backup/leanote_install_data/ → leanote_test
  ├─ 阶段 1：只读用例（对 pristine fixture 录制，可并行）
  └─ 阶段 2：mutation 成对用例（串行；破坏性用例各自前置一次 restore）
```

破坏性用例（`DeleteTrash` / `DeleteNotebook` / `DeleteTag` / `UpdatePwd`）
必须各自独立 restore —— 尤其 `UpdatePwd` 会让后续登录失效。
（`DeleteImage` 是 API 层僵尸方法，不在此清单；图片删除在 web 层 smoke 覆盖。）

认证：`/api/auth/login` 用 fixture 里的 `admin` / `abc123`（`users.bson` 中
`Pwd = e99a18c428cb38d5f260853678922e03 = md5("abc123")`，已实算核实）取 token，
其余请求带 `?token=`。web 层 smoke 用 `POST /doLogin` 的 session cookie，与 API token 独立。
fixture 另有 `demo` 用户，可用于共享/权限用例的第二身份 ——
`ShareController` 的越权测试**需要两个用户**，这是选它做 golden 的主要原因。

同步边界与冲突用例（R-G2）：`afterUsn=0` 全量、`afterUsn=当前最大 Usn` 空 delta、
`maxEntry` 截断翻页、超大 `afterUsn`；冲突判断的生效位置在 controller 层
（`ApiNoteController.go:372-376`，`NoteService.go:447-454` 的同判断已注释），
`DeleteTrash` / `DeleteNotebook` / `DeleteTag` 的冲突分支一并按现状录制。
USN 配对范围以 PRD R-G2 的映射表为准：`DeleteNotebook` 不 bump、`DeleteTag` 写旧 usn
属已知偏差，按现状钉住并记 issue。

## 5. MongoDB 供给：G 阶段固定 5.0（硬约束）

**为什么不是 8.0**：旧实现用 `mgo.v2`，只发送 MongoDB 5.1 起移除的 legacy opcode
（`research/external-facts.md` §1）。旧代码能完成握手但**无法在 5.1+ 上执行任何 CRUD**，
"旧实现 + MongoDB 8.0 + golden 录制"三者不可能同时成立。
因此 G 的录制/回放环境固定 **MongoDB 5.0**（EOL，仅作旧实现基线的测试环境，显式记录此事实）；
7.0/8.0 的验证由 B 阶段（官方驱动迁移后）承接，父任务 R-G 已同步。

- 本地（Windows/PowerShell，`mongorestore` 在容器内执行，宿主机无需安装 tools；
  `mongo:5.0` 镜像已实测包含 mongorestore 与 mongosh）：

```powershell
docker rm -f leanote-test-mongo              # 幂等：清掉可能残留的旧容器（不存在时忽略报错）
docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0
# env up 必须执行同等逻辑：每 500ms 重试，stdout.Trim() == "1" 才算就绪，60s 到期返回非零
# PowerShell 等价验证：
$deadline = (Get-Date).AddSeconds(60); $ping = ''
do { $ping = docker exec leanote-test-mongo mongosh --quiet --eval 'db.runCommand({ping:1}).ok' 2>$null
     if ($ping.Trim() -eq '1') { break }; Start-Sleep -Milliseconds 500
} while ((Get-Date) -lt $deadline)
if ($ping.Trim() -ne '1') { throw 'Mongo ping timeout after 60s' }
# 注意 docker cp 语义：目标 /dump 不存在时，源目录**内容**会落在 /dump 下（/dump/*.bson）；
# 复制到 / 才会得到 /leanote_install_data
docker cp mongodb_backup/leanote_install_data leanote-test-mongo:/
docker exec leanote-test-mongo mongorestore --db leanote_test --dir /leanote_install_data --drop
# 验证：返回 2（admin + demo）
docker exec leanote-test-mongo mongosh --quiet leanote_test --eval 'db.users.countDocuments()'
```

  `--rm` + 前置 `docker rm -f` 保证命令可重复执行：`docker stop` 即整体清理，
  二次运行不会因同名容器残留而失败。
  以上封装为 harness 的固定入口 `go run ./app/tests/harness/cmd/env up`（`down` 为清理），
  内含 ping 等待与超时失败，满足 G-AC9"一条文档化命令"。
- **CI（Go replay job）**：GitHub Actions 的 `services:` 容器对 runner 没有稳定的
  `docker exec`/`docker cp` 通道，**不用 `services:`**。改为在 step 中直接
  `docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0`（runner 有 docker，
  名称由我们控制），之后与本地**完全相同**的 cp/exec/restore/ping 命令，
  测试经 `localhost:27017` 连接。不安装 mongodb-database-tools（容器内自带
  mongorestore），也就没有 tools 版本钉扎问题：

```yaml
# job steps（可直接执行的 bash 片段；step 末尾始终清理同名容器）：
docker rm -f leanote-test-mongo >/dev/null 2>&1 || true
trap 'docker rm -f leanote-test-mongo >/dev/null 2>&1 || true' EXIT
docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0
deadline=$((SECONDS + 60)); ping=''
while :; do
  ping="$(docker exec leanote-test-mongo mongosh --quiet --eval 'db.runCommand({ping:1}).ok' 2>/dev/null || true)"
  [ "$(printf '%s' "$ping" | tr -d '[:space:]')" = '1' ] && break
  [ "$SECONDS" -ge "$deadline" ] && { echo 'Mongo ping timeout after 60s' >&2; exit 1; }
  sleep 0.5
done
docker cp mongodb_backup/leanote_install_data leanote-test-mongo:/
docker exec leanote-test-mongo mongorestore --db leanote_test --dir /leanote_install_data --drop
LEANOTE_GOLDEN=replay go test ./app/tests/... -count=1
```

  Node 基线 job 固定 24.x。
- **ExportPdf 首录的受控 record job**（解决"Windows 不能录、CI 只 replay"的死锁）：
  单独的 `workflow_dispatch`（仅手动触发的 Linux job）：
  - runner：`ubuntu-22.04`；
  - wkhtmltopdf **必须落在生产代码的默认查找路径** `/usr/local/bin/wkhtmltopdf`
    （`ApiNoteController.go:608-611`，未配置 `exportPdfBinPath` 时的缺省）——
    `wkhtmltox` deb 恰好安装到 `/usr/local`，安装后强制执行
    `test -x /usr/local/bin/wkhtmltopdf || exit 1`（路径不存在 = job 失败，不产生 sysError 假象）：

```bash
wget -q https://github.com/wkhtmltopdf/packaging/releases/download/0.12.6.1-3/wkhtmltox_0.12.6.1-3.jammy_amd64.deb
sudo dpkg -i wkhtmltox_0.12.6.1-3.jammy_amd64.deb || (sudo apt-get -f install -y && sudo dpkg -i wkhtmltox_0.12.6.1-3.jammy_amd64.deb)
test -x /usr/local/bin/wkhtmltopdf
```

  - Mongo 供给与上一步相同（`docker run` 命名容器 + cp/exec restore）；
  - `LEANOTE_GOLDEN=record go test ./app/tests/... -count=1 -run '^TestGoldenExportPdf$'`，golden 以 artifact
    上传供人工审核；审核通过、golden 入库后，PR/push 的 replay job 才把该端点纳入比对。
    PR/push 流水线**永不**运行 record。
  - 若届时 `ubuntu-22.04` runner 已下线：退回 `ubuntu-24.04` + 同一 jammy deb
    （noble 的 `libssl3t64` Provides `libssl3`，依赖可满足）；无论哪种都以
    `test -x` 为硬前置。
- fixture 是 BSON dump（`*.bson` + `*.metadata.json`），统一用 `mongorestore` 恢复。

## 6. golden 文件布局

```
app/tests/golden/
  api/
    auth_login.json          # 请求规格 + 归一化后的响应
    note_getNoteContent.json
    note_getNotes_invalidNotebookId.json   # 失败路径用例：<action>_<场景>.json
    ...
  web/
    share_listShareNotes.json
    ...
```

失败路径用例与成功用例同构，文件名以场景后缀区分（`_noToken` / `_invalidToken` /
`_invalidParam` / `_forbidden` 等），保证 G-AC1 的失败路径覆盖可被文件清单直接审计。

一个用例一个文件，便于 code review 时逐端点看 diff —— B/C-b 阶段一旦某端点变了，
diff 会精确指出是哪个端点的哪个字段，而不是一个巨大的合并文件。

文件内含请求规格（method / path / query / body / 是否需要 token / 用哪个身份），
使夹具是**数据驱动**的：新增端点只加文件，不改代码。

### record / replay 模式（防假绿）

- 模式经**环境变量** `LEANOTE_GOLDEN`（`record` / `replay`）传入，缺省 `replay`；
  不用 go test flag（自定义 flag 需 `-args` 且跨 Go 版本解析有差异）。
- **replay 是默认**：golden 缺失或失配 → 测试失败；任何情况下不得写 golden 文件。
- **record 仅人工显式调用**：用于首次录制、经评审的 golden 更新（PR 里展示 golden diff）、
  以及 R-G5 的受控 record job（ExportPdf 首录）。
- CI 的 PR/push 流水线只跑 replay；record/replay 运行前都执行一次 fixture restore（确定性前提）。
- 归一化器单测额外覆盖：replay 模式下对失配 golden 的写保护（断言文件未被改动）。

## 7. 与后续单元的契约

- B、C-b、E 各自的 check 阶段直接回放 G 的 golden，无需重录。
- 若某阶段**确实需要**改变某端点输出（本任务范围内不应发生），必须显式更新 golden 文件，
  并在该子任务的 PR 描述里说明为何这不是契约破坏 —— 让"输出变了"永远是一个需要解释的显式动作。

## 8. 明确接受的局限

- golden 钉住的是**当前行为**，不是**正确行为**。录制中若发现既有 bug，按现状钉住并单独记 issue，
  不在 G 里修 —— 否则 B/C-b 无法区分"我改坏了"与"基线本来就错"。
- 二进制端点（`GetImage` / `GetAttach` / `GetAllAttachs` / `ExportPdf`）只钉 Content-Type 与非空，
  header 按 §3 的二进制专用闭合集处理。
  - `ExportPdf` 的**平台边界**：实现硬编码 `/bin/sh`（`ApiNoteController.go:623`）并依赖
    `site.url` 回调 + 外部 wkhtmltopdf（`exportPdfBinPath`，缺省 `/usr/local/bin/wkhtmltopdf`）。
    其首份 golden 由 §5 的受控 record job（workflow_dispatch Linux job）生成并经人工审核入库，
    之后 replay 在 Linux CI 上执行；Windows 本地只产出显式"平台不支持"记录，
    不计入 G-AC1 对该端点的完成。
  - 文件系统副作用每次录制后清理：`files/export_pdf/*.pdf` 与
    `GetAllAttachs` 的 `files/<note.Title>.tar.gz`（`ApiFileController.go:107-113`），
    并以 `.gitignore` 或等价机制保证不入库（R-G7）。
- admin / member 只有 smoke，无字节级保护（父任务 D5 已定）。
