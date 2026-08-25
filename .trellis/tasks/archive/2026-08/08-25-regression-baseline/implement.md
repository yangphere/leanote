# 回归基线（G）— 执行计划

## 顺序清单

### 阶段 1：环境与夹具骨架（不碰生产代码）

1. **Mongo 供给**（R-G6）：一条命令起 `mongo:5.0` 容器（**不是 8.0**——旧 `mgo.v2` 只支持
   legacy opcode，≥5.1 无法 CRUD，见 design §5）+ 容器内 `mongorestore` 恢复
   `mongodb_backup/leanote_install_data/` 到 `leanote_test`：
   `docker rm -f leanote-test-mongo`（幂等清理残留，不存在时忽略）→
   `docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0` →
   **ping 等待**（Go `env up` 每 500ms 执行 `docker exec ... mongosh --quiet --eval
   'db.runCommand({ping:1}).ok'`，仅 stdout.Trim()==`1` 成功；60s deadline 到期返回非零并
   输出超时错误）→
   `docker cp mongodb_backup/leanote_install_data leanote-test-mongo:/`
   （**复制到 `/` 而非 `/dump`**——目标不存在时 docker cp 会把源目录**内容**摊到 `/dump`，
   得到 `/dump/*.bson` 而非 `/dump/leanote_install_data`）→
   `docker exec leanote-test-mongo mongorestore --db leanote_test --dir /leanote_install_data --drop`。
   `--rm` + 前置 `rm -f` 保证可重复执行（`docker stop` 即整体清理，二次运行不因同名容器失败）。
   封装为固定入口 `go run ./app/tests/harness/cmd/env up`（`down` 清理），写进 README（G-AC9）。
   验证：`docker exec leanote-test-mongo mongosh --quiet leanote_test --eval 'db.users.countDocuments()'` 返回 2（admin + demo）。
2. **`conf/app.conf` 追加 `[test]` section**（`mode.dev=false`、`results.pretty=false`、
   `watch=false`、`db.dbname=leanote_test`、`site.url=http://127.0.0.1:28017`、日志到 stderr）。
   ⚠️ 只追加新 section，**不改任何既有键** —— 既有部署不受影响。
   依据：Revel v1.0.0 `revel.go:242-244` 要求 runMode 有同名 section，否则 Fatal；
   服务端入口 `app/init.go:423` 以 `db.Init("", "")` 启动 ⇒ section 内 `db.dbname` 覆盖生效；
   `site.url` 覆盖是 `ExportPdf` 回调可达的前提（DEFAULT 段为 `:9000`，`app.conf:17`）。
3. **`app/tests/harness/server.go`**：`StartServer(t)` → 构建二进制、**固定端口 28017** 启动
   （与 `[test]` section 的 `site.url` 同值；端口被占用则显式失败，不回退随机端口——
   `ExportPdf` 回调依赖，见 design §1）、轮询就绪、返回 baseURL + cleanup。
   `buildServerBinary()` 独立成函数（C-b 只改此处）。
   验证：`curl $baseURL/login` 返回 200。
4. **`app/tests/harness/normalize.go`**：原始字节上的正则归一化，**按 JSON 字段名定位**
   （`"字段名":"值形态"` 联合匹配，字段清单集中维护），**不做 unmarshal→marshal 往返**（会丢键序）。
   附带格式断言。header 采用闭合集合：比较集 {`Content-Type`、`Location`}、
   排除集 {`Date`、`Set-Cookie`、`Content-Length`}，未列入任一集合的 header 判失败。
   验证：单元测试覆盖归一化器本身 —— 键序保持、枚举字段被替换、
   **正文 Content 中的 24-hex 不被替换**（canary）、`{"$oid":…}` 形态被判失配、
   未知 header 判失败、排除集只影响清单内 header。
5. **`app/tests/harness/client.go` 与模式开关**：登录取 token（admin/abc123）、发请求、
   按用例规格驱动；模式经环境变量 `LEANOTE_GOLDEN=record|replay` 传入（缺省 replay，
   不用 go test flag——需 `-args` 且跨版本有解析差异），replay 写保护（见 R-G8 / design §6）。

### 阶段 2：清理既有测试（R-G4 / D6）

6. 删除 `app/tests/config_test.go`、`note_content_test.go`、`db_test.go`、`tmp.go`、`reg_test.go`。
   （`reg_test.go` 为审核补充：无断言、仅 `t.Log` 的调试脚本，符合 R-G4 删除标准。）
7. 改 `auth_test.go`：包级 `init()` → 可跳过。**在调用 `db.Init` 之前**探测 Mongo 可达性
   （`db.Init` 在 dial 失败时 panic，`Mgo.go:107-109`，无法 recover 成优雅跳过）。
   验证：无 Mongo 时 `go test ./app/tests/ -run TestAuth -count=1` 输出 SKIP 而非 panic（G-AC6）。

### 阶段 3：录制 golden（R-G1 / R-G2；action 清单 = 29 个可分发 action）

8. **只读用例**：`/api/*` 中的 `GetSync*` / `Get*` / `Info` + web 层只读端点。
   对 pristine fixture 录制。清单中**不含** `GetHistories` 与 `ApiFile.CopyImage/GetImages/
   UpdateImageTitle/DeleteImage`（均为块注释内的僵尸方法，见 PRD R-G1）。
9. **失败路径用例**（R-G1）：每个需要 token 的 action 录无 token / 无效 token；
   参数校验失败（如 `GetNotes` 非法 `notebookId` → `notebookIdInvalid`）、资源不存在、
   demo 身份越权读 admin 资源。文件名带场景后缀，可按清单审计 G-AC1 覆盖。
10. **mutation 成对用例（按 R-G2 映射表）**：
    - `GetSyncNotes` ← `AddNote` / `UpdateNote` / `DeleteTrash`：断言 `Usn` 递增且 delta 含变更。
    - `GetSyncNotebooks` ← `AddNotebook` / `UpdateNotebook`：同上；
      **`DeleteNotebook` 不 bump（`NotebookService.go:303-312`）——按现状钉住 + 记 issue**。
    - `GetSyncTags` ← `AddTag`；**`DeleteTag` 写库为旧 usn（`TagService.go:122-123`）——按现状钉住 + 记 issue**。
    - auth/user profile/file mutation 不做 USN 断言（映射表外）。
    - 同步边界用例（afterUsn=0 / 最大 Usn / maxEntry 翻页 / 超大 afterUsn）与
      各冲突分支（生效位置 `ApiNoteController.go:372-376` 等）一并录制。
    - 破坏性用例（`DeleteTrash` / `DeleteNotebook` / `DeleteTag` / `UpdatePwd`）
      各自前置一次 fixture restore；`UpdatePwd` 必须最后跑或独立 restore（会让登录失效）。
    （注：原清单中的 `DeleteImage` 是 API 层僵尸方法，已移出；图片删除在 web 层 smoke 覆盖。）
11. **`ShareController` 双身份用例**：用 fixture 的 admin + demo 两个用户覆盖共享与越权路径 ——
    这是选它进 golden 的主要理由。
12. **二进制端点**（`GetImage` / `GetAttach` / `GetAllAttachs` / `ExportPdf`）：
    只钉 Content-Type + 非空，header 按 design §3 的**二进制专用闭合集**执行
    （`Accept-Ranges: bytes` 严格比较；`Content-Disposition` 的 disposition 类型与
    filename **四个端点全部严格比较**——filename 实际取值均与机器无关，见 design §3 表；
    `Last-Modified` 排除）。
    - 成功路径用例前置 harness 种子：用内嵌固定字节在运行时创建
      `files/test_seed/`，插入对应 `files`/`attachs` 记录；不依赖 gitignored 的预置文件，
      用例后删除种子目录并清理记录。仓库无物理上传文件，不种子则只能钉住空/错误响应。
    - `ExportPdf` 的首份 golden 由阶段 5 的**受控 record job** 生成、人工审核入库；
      Windows 本地只产出显式"平台不支持"记录。
    - 每次录制后清理副作用：`files/export_pdf/*.pdf` 与 `GetAllAttachs` 的
      `files/<note.Title>.tar.gz`（`ApiFileController.go:107-113`）。

### 阶段 4：smoke（R-G3）

13. `admin`、`member` 两包 + `Blog` / `Preview` / `Auth` / `Index` + 页面级
    `/`、`/login`、`/note`、`/blog`、`/demo`。JSON 端点判定：状态码 + JSON 可解析 + 关键字段存在；
    页面端点判定：逐页期望状态码（含未登录重定向语义）+ 关键 HTML 标记。
    web 凭证走 session cookie（`POST /doLogin`），API 走 token。

### 阶段 5：CI（R-G5）

14. GitHub Actions 三个 job（Mongo 供给统一用 `docker run` 命名容器，**不用 `services:`**，
    也就不装 mongodb-database-tools——容器内自带 mongorestore；命令与本地完全一致）：
    - **Go replay job**：先幂等执行 `docker rm -f leanote-test-mongo || true`，再
      `docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0`；按 design §5
      的 60 秒 ping loop 等待，再执行 `docker cp ... :/` 与 `docker exec mongorestore`，最后
      显式运行 `LEANOTE_GOLDEN=replay go test ./app/tests/... -count=1`；job cleanup 必须
      再次 `docker rm -f ... || true`，避免 self-hosted/retry 残留。
    - **ExportPdf 首录 record job**：`workflow_dispatch` 仅手动触发；runner `ubuntu-22.04`；
      安装 `wkhtmltox_0.12.6.1-3.jammy_amd64.deb`（deb 安装到 `/usr/local`，恰好落在生产代码
      默认查找路径 `/usr/local/bin/wkhtmltopdf`），安装后强制 `test -x /usr/local/bin/wkhtmltopdf`
      （失败即 job 失败，防 sysError 假象）；Mongo 同上；`LEANOTE_GOLDEN=record go test ./app/tests/... -count=1
      -run '^TestGoldenExportPdf$'`，golden 以 artifact 上传供人工审核，审核入库后 replay job 才纳入
      该端点。若届时 `ubuntu-22.04` 下线，退回 `ubuntu-24.04` + 同 deb（见 design §5）。
    - **Node 24.x `npm test` job**。
    **不删** `.travis.yml`（属 F 单元）。验收（已裁决）：push 后以真实 workflow 运行记录为证据，
    静态校验不算完成；若暂不能 push，G 保持未完成。

### 阶段 6：自证

15. `git diff --stat` 确认 `app/` 下除 `app/tests/` 外零改动（G-AC10）；
    改动仅限 `app/tests/`、`conf/app.conf`（追加 section）、`.github/`、`sh/` 或 README；
    并确认无 `files/export_pdf/`、`files/*.tar.gz` 副作用文件被暂存。
16. 连续两次以 **replay 模式**回放 golden，结果一致且 golden 文件零改写（G-AC2）
    —— 证明归一化与 header 闭合集已消除抖动，且模式分离排除了"每次重录"假绿。

## 验证命令

（本开发机为 Windows/PowerShell；夹具本体由 Go 代码 `exec` 驱动，下列命令为语义参考，
Linux CI 上等价执行。）

```powershell
# 起测试用 Mongo（本机无 mongod，用容器；mongorestore 在容器内执行，宿主机无需 tools）
docker rm -f leanote-test-mongo      # 幂等清理（不存在时报错可忽略）
docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0
$deadline = (Get-Date).AddSeconds(60); $ping = ''
do { $ping = docker exec leanote-test-mongo mongosh --quiet --eval 'db.runCommand({ping:1}).ok' 2>$null
     if ($ping.Trim() -eq '1') { break }; Start-Sleep -Milliseconds 500
} while ((Get-Date) -lt $deadline)
if ($ping.Trim() -ne '1') { throw 'Mongo ping timeout after 60s' }
docker cp mongodb_backup/leanote_install_data leanote-test-mongo:/    # 复制到 / ⇒ /leanote_install_data
docker exec leanote-test-mongo mongorestore --db leanote_test --dir /leanote_install_data --drop
docker exec leanote-test-mongo mongosh --quiet leanote_test --eval 'db.users.countDocuments()'  # 期望 2

# 全套（显式 replay；-count=1 禁用 go test 缓存）
$env:LEANOTE_GOLDEN = 'replay'; go test ./app/tests/... -count=1 -v; $env:LEANOTE_GOLDEN = $null

# 首次录制/经评审更新 golden（仅人工显式调用；模式走环境变量，不用 go test flag）
$env:LEANOTE_GOLDEN = 'record'; go test ./app/tests/... -count=1 -v; $env:LEANOTE_GOLDEN = $null

# 无 Mongo 时应 SKIP 而非 panic（G-AC6 的关键验证；--rm 使 stop 即整体清理）
docker stop leanote-test-mongo
go test ./app/tests/ -run TestAuth -count=1 -v      # 期望 SKIP

# 归一化器自身的单元测试（不需要 Mongo；含 canary、header 闭合集与二进制集用例）
go test ./app/tests/harness/... -v

# 幂等性（G-AC2）：运行前、第一次后、第二次后分别快照；三者必须一致
$env:LEANOTE_GOLDEN = 'replay'
$h0 = Get-ChildItem -Recurse -File app\tests\golden | Get-FileHash
go test ./app/tests/... -count=1
$h1 = Get-ChildItem -Recurse -File app\tests\golden | Get-FileHash
go test ./app/tests/... -count=1
$h2 = Get-ChildItem -Recurse -File app\tests\golden | Get-FileHash
Compare-Object $h0 $h1    # 期望无输出（第一次 replay 零改写）
Compare-Object $h1 $h2    # 期望无输出（第二次 replay 零改写）
$env:LEANOTE_GOLDEN = $null

# 自证未碰生产代码（G-AC10）
git diff --stat -- app/ ':(exclude)app/tests'    # 期望空
```

## 风险文件与回滚点

| 文件 | 风险 | 说明 |
|---|---|---|
| `conf/app.conf` | **中** | 唯一被触碰的既有配置。只允许**追加** `[test]` section；若误改既有键会破坏所有部署。回滚 = revert 单文件 |
| `app/tests/*` | 低 | 删除 4 个文件（config/note_content/db/reg_test.go）+ `tmp.go`；均为无断言、硬编码路径或真发邮件的调试脚本 |
| `app/cmd/` | 只读 | 夹具**调用** `gen_tmp.sh` 的等价命令，但不修改 `app/cmd/` |
| 生产代码 | 零 | G-AC10 以 `git diff --stat` 强制 |

整个 G 单元可整体 revert 且无副作用（纯新增测试 + CI + 一个 conf section）。

## 启动前检查

- [x] `prd.md` / `design.md` / `implement.md` 齐备（2026-08-25 三轮审核后修订：
      Mongo 5.0 裁决、29 action 清单、USN 映射表、固定端口/site.url、record/replay、
      header 闭合集（含二进制专用集）、ExportPdf 受控 record job、CI restore 机制、
      副作用清理、restore 命令具体化且幂等、LEANOTE_GOLDEN 环境变量模式）
- [x] `implement.jsonl` / `check.jsonl` 已填真实条目（第二轮已修正冲突分支引用并补偏差证据）
- [x] 父任务的 D5（覆盖面）与 D6（旧测试处理）已反映在本子任务 PRD —— 已反映
- [x] PRD 原待确认 2 条已裁决（G-AC8 选 (a) 实跑回填；reg_test.go 删除），见 PRD「已裁决事项」
- [ ] 用户批准本子任务规划后才 `task.py start`（`start` 会把 planning 翻成 in_progress，
      见 `.trellis/scripts/task.py:127-131`）
