# Vet 清零证明 — Task 08-25-go-toolchain（Phase 4）

> 记录 Phase 4 按类别清零 `go vet ./app/...` 的全部批次证据。
> 环境：Windows，系统 Go 1.27.0；replay 统一使用
> `$env:LEANOTE_TEST_GO='C:\Users\rog\sdk\go1.26.7\bin\go.exe'; $env:LEANOTE_GOLDEN='replay'`
> + `go test -p 1 ./app/tests/... -count=1 -timeout 30m`；
> MongoDB：Docker `mongo:5.0` 容器 `leanote-test-mongo`（`env up` 启动，结束后保留）。
>
> 完整前置基线：未修改 `HEAD` 清缓存后，`vet-baseline-go1.26.7.txt` 与
> `vet-baseline-go1.27.0.txt` 各 237 条；保留原始诊断排序，排序规范化后的行集合一致。
> 本 Phase 4 的起点在 Phase 2 清除 205 条 tag 后：当前 vet 只剩 36 条非 tag 诊断，
> 两版本的排序规范化后行集合一致。

## 批次顺序与总览

严格按 A → B → B2 → D → C 执行（C 最后，E404 锁定先行取证）；每批完成后执行
gofmt 复核、`go build ./app/...`、聚焦测试、G replay 与 vet delta 对比。

| 批次 | 发现数 | 结果 | 涉及文件 | vet delta |
|---|---:|---|---|---|
| A unkeyed literal | 21 | ✅ PASS | Template.go / BlogService.go / EmailService.go / FileService.go / NoteService.go / ShareService.go / UserService.go | 36 → 15 |
| B unreachable | 6 | ✅ PASS | captcha/Captcha.go / cmd/harness/build.go / cmd/build.go / lea/File.go / AuthController.go | 15 → 9 |
| B2 self-assignment | 3 | ✅ PASS | NoteService.go:972 / MemberBlogController.go:383/483 | 9 → 6 |
| D signal channel | 1 | ✅ PASS | cmd/harness/harness.go:333 | 6 → 5 |
| C printf misuse | 5 | ✅ PASS | BaseController.go / route/Route.go / NoteController.go | 5 → **0** |

无跳过 hunk。无 analyzer 抑制（零 `//nolint`、零 vet flag、零构建标签、零包排除）。

---

## Batch A：21 个 unkeyed literal → 具名字段

**日期**: 2026-08-26　**结果**: ✅ PASS

### 字段映射核对（实施前完成）

- `bson.RegEx{p, o}` → `{Pattern: p, Options: o}`：mgo v2.0.0-20190816093944
  `bson/bson.go:428-431` 确认字段序为 `Pattern`、`Options`；10 个调用点逐一核对，
  第一位置实参均为 pattern（`".*?" + key + ".*"`），第二位置均为 `"i"`。
- `revel.PlaintextErrorResult{err}` → `{Error: err}`：Revel v1.0.0 `results.go`
  确认唯一字段 `Error error`，错误值原样保留。
- 项目模型按 `app/info` 结构体定义逐字段核对：
  - `BlogItem{Note, Abstract, Content, HasMore, User}`（BlogInfo.go:10-16）——4 个调用点
    位置值与字段一一对应（含 BlogService.go:508 的 `Abstract: ""`）；
  - `BlogStat{NoteId, ReadNum, LikeNum, CommentNum}`（BlogInfo.go:68-73）；
  - `ArchiveMonth{Month, Posts}`（BlogCustom.go:42-45）；
  - `NoteAndContent{Note; NoteContent}`（嵌入，NoteInfo.go:68-71）→ `{Note: note, NoteContent: noteContent}`；
  - `NoteAndContentSep{NoteInfo, NoteContentInfo}`（NoteInfo.go:109-112）——注意字段名与变量名不同，
    位置类型核对后命名；
  - `ShareNoteWithPerm{Note; Perm}`（ShareNotebookNoteInfo.go:161-164，嵌入）。

### 验证

| 步骤 | 命令 | 结果 |
|---|---|---|
| gofmt | `gofmt -l <7 文件>` | 列出全部 7 个文件，但 HEAD 版本与未触碰文件同样被列出；`gofmt -d` 显示为整文件 CRLF→LF 重排（仓库 autocrlf=true 的既有现象），非本批引入；`git diff` 确认仅 21 个预期 hunk（21+/21−），无行尾或空白扰动 |
| 构建 | `go build ./app/...` | exit 0 |
| 聚焦测试 | `go test ./app/info/... ./app/cmd/... -count=1`；`go test ./app/lea/... -count=1 -vet=off` | 全部 ok（lea/route 当时仍含 C 类发现，按协议临时 `-vet=off`，最终态已不带该开关复验通过） |
| G replay | 同头部统一命令 | exit 0；`git status --short app/tests/golden` 为空 |
| vet delta | `go vet ./app/... \| Sort-Object` | 21 条 unkeyed 全部消失，无新增（36→15） |

---

## Batch B：6 处不可达代码删除

**日期**: 2026-08-26　**结果**: ✅ PASS

### 关键决策：死区整体删除而非仅删被标记语句

vet 的 unreachable analyzer 按"每个不可达 CFG 块的首语句"上报：
`harness/build.go` 的 `Build()` 在 `return // 改了这里`（:97）之后的整个区域
（原 :99–241，含注释、局部变量、无限 for 循环与尾部 `Fatal+return`）构成两个不可达块，
基线才会在 :100 与 :240 各报一条。只删单条被标记语句会把发现移到下一语句，
无法归零 ⇒ 整个可证明不可达区域一并删除（符合"只删除编译器已证明不可达的语句"）。

| 位置 | 处理 | 连带修正 |
|---|---|---|
| `cmd/harness/build.go:99-241` | 整段死区删除（函数以 `return // 改了这里` 收尾），两处发现同源同灭 | 移除仅被死区使用的 import `path`/`runtime`/`time`（全文件其余活代码经逐一检索确认不再使用；`os/exec`、`regexp`、`fmt` 等由 `getAppVersion`/`newCompileError`/`makePackageAlias` 等继续使用）。`importErrorPattern`、`getAppVersion`、`newCompileError` 成为未被引用的包级声明——Go 合法且非 vet 发现，按最小改动保留 |
| `cmd/build.go:96-108` | 死区删除 | 原 :83 `app, err := harness.Build(...)` 中 `app` 仅被死区消费 ⇒ 改为 `_, err = harness.Build(...)`（同一具名返回 err，赋值目标与控制流不变）。`buildCopyFiles` 等成为未引用包级函数，合法，保留 |
| `captcha/Captcha.go:388` | 删除 `panic("unreachable")`（前接无条件终止的 for） | 无 |
| `lea/File.go:147` | 删除 `return false`（if/else 双分支均已 return） | 无 |
| `AuthController.go:100` | 删除 `return nil`（if/else 双分支均已 return） | 无 |

### 验证

| 步骤 | 命令 | 结果 |
|---|---|---|
| 构建 | `go build ./app/...` | exit 0 |
| 聚焦测试 | info/cmd/lea 同上 | 全部 ok |
| harness 包 vet | `go vet ./app/cmd/harness/` | 仅剩 D 类 :334 一条（预期） |
| G replay（全量未过滤） | 同头部统一命令 | exit 0（含 `generate_contract_test` 真实生成+二进制构建路径，覆盖 harness/build.go 改动）；golden 目录 git status 为空 |
| vet delta | 全量 | 6 条 unreachable 全部消失，无新增（15→9） |

---

## Batch B2：3 处 self-assignment

**日期**: 2026-08-26　**结果**: ✅ PASS

- `NoteService.go:972` `tags = tags`：纯自赋值，下一行 `append(tags[:i], tags[i+1:]...)` 为真实逻辑，删除零影响。
- `MemberBlogController.go:383/:483` `filename = filename`：逐一核对作用域——
  `filename` 在各函数内单一绑定（:373/:474 `filename := handel.Filename`），无 shadowing、
  无闭包重绑；后续 `toPath := dir + "/" + filename` 取值不变 ⇒ 输出字节级等价，可安全删除。
  （实施过程中一次误编辑只删了相邻空行、未删赋值行，随即以精确 hunk 修正；
  最终 `git diff` 确认两处均仅减少 `filename = filename` 一行、周边空格与 HEAD 完全一致。）

验证：build exit 0；聚焦测试 ok；vet delta 9→6 无新增。
全量 replay 由 Batch D 过滤套件与最终门禁全量 replay 共同覆盖（B2 改动点均在 golden/usn/smoke 行内）。

---

## Batch D：signal channel 容量 1

**日期**: 2026-08-26　**结果**: ✅ PASS

`cmd/harness/harness.go:333`：`make(chan os.Signal)` → `make(chan os.Signal, 1)`。
`:334` 订阅集合保持 `signal.Notify(ch, os.Interrupt, os.Kill)` 不变；`<-ch` → `h.app.Kill()`
→ `os.Exit(1)` 流程原样。缓冲 1 不改变语义（Notify 单信号投递后 `<-ch` 即唤醒退出），
不新增 SIGTERM。

验证：build exit 0；`go vet ./app/cmd/...` exit 0；
`go test ./app/tests/harness -run "TestEnvironment|TestConfiguration|TestNormalize|TestClient|TestServer" -count=1` ok；
G replay（golden/usn/smoke 过滤套件，-v）全绿——其中
`TestWebAdminMemberAndControllerSmoke.PASS` 同时充当 **C 批实施前的 E404 基线捕获**；
vet delta 6→5 无新增。

---

## Batch C：5 处 printf misuse（输出字节等价改写）

**日期**: 2026-08-26　**结果**: ✅ PASS

### 等价性依据（Revel v1.0.0 源码，module cache）

```go
// controller.go
func (c *Controller) RenderText(text string, objs ...interface{}) Result {
    finalText := text
    if len(objs) > 0 { finalText = fmt.Sprintf(text, objs...) }   // 无 objs 时原文返回
    ...
}
func (c *Controller) NotFound(msg string, objs ...interface{}) Result {
    finalText := msg
    if len(objs) > 0 { finalText = fmt.Sprintf(msg, objs...) }
    c.Response.Status = http.StatusNotFound                        // 固定 404
    return c.RenderError(&Error{Title: "Not Found", Description: finalText})
}
```

| 调用点 | 改写前 | 改写后 | 字节等价论证 |
|---|---|---|---|
| BaseController.go:163（E404） | `c.NotFound("", nil)` | `c.NotFound("")` | 前：`objs=[nil]` 长度 1 ⇒ `Sprintf("", nil)`=`""`；后：`objs=[]` ⇒ 直接取 msg `""`。两者 `Error{Title:"Not Found", Description:""}` + status 404 完全一致 ⇒ RenderError 渲染管线输入相同 ⇒ HTTP 字节相同 |
| Route.go:24 | `NotFound("No matching route found: "+GetRequestURI())` | `NotFound("No matching route found: %s", GetRequestURI())` | 前：objs 为空 ⇒ Sprintf 从不执行，Description=URI 原样拼接；后：URI 作为 `%s` 实参插入（实参不做格式化再解释）。对任意 URI（含 `%` 字符）输出逐字节一致；status/Title 不变 |
| Route.go:77 | `NotFound(err.Error())` | `NotFound("%s", err.Error())` | 同上：错误文本作为 `%s` 实参插入，任意内容逐字节一致 |
| NoteController.go:490/:494 | `RenderText("export pdf error. "+Sprintf("%v", err))` | `RenderText("export pdf error. %v", err)` | 前：objs 空 ⇒ 文本=`前缀+Sprintf("%v",err)`；后：`Sprintf("export pdf error. %v", err)` ≡ 字面量前缀拼接单个 `%v` 展开。对任意 err 值逐字节一致；status 200 text/plain 不变 |

### E404 锁定证据（改写前后）

- Golden 覆盖核查：`note_getNote_notFound.json` 等所有 `_notFound/_invalid/_none` golden
  钉的是 `/api/*` JSON 信封（如 `{"Ok":false,"Msg":"notExists"}`, status **200**），
  **不经过** `BaseController.E404` 的 HTML 404 管线；`E404` 调用方仅
  BlogController / PreviewController / MemberBlogController（HTML 页面路由）。
- 真实覆盖点：smoke 测试 `assertHTMLPage(t, web, "/preview", http.StatusNotFound)`
  （GET /preview → Preview.Index → themeId 为空 → c.E404()）断言 status=404 且返回 HTML 文档。
- 改写前基线：Batch D 的 replay 中 `TestWebAdminMemberAndControllerSmoke` PASS（404 断言成立）；
  改写后：Batch C 与最终门禁 replay 再次全绿，同一断言 PASS ⇒ 覆盖路径输出未变；
  未覆盖部分由上表 Revel 源码级字节等价论证闭合。

### 验证

| 步骤 | 命令 | 结果 |
|---|---|---|
| 构建 | `go build ./app/...` | exit 0 |
| 全量 vet（系统 Go 1.27.0） | `go vet ./app/...` | **零输出，exit 0** |
| 聚焦测试（无 `-vet=off`） | `go test ./app/info/... ./app/lea/... ./app/cmd/... -count=1` | 全部 ok（route 包首次带默认 vet 通过） |
| G replay | 同头部统一命令 | exit 0；golden 目录 git status 为空 |
| vet delta | 全量 | 5 条 printf 全部消失，无新增（5→0） |

---

## 最终门禁（Final Gate）

**日期**: 2026-08-26　**结果**: ✅ PASS

| 步骤 | 命令 | 结果 |
|---|---|---|
| 清缓存 | `go clean -cache` | exit 0（防陈旧缓存少报） |
| vet @1.27.0 | `go vet ./app/...`（清缓存后） | **零输出，exit 0**（go1.27.0 windows/amd64） |
| vet @1.26.7 | `C:\Users\rog\sdk\go1.26.7\bin\go.exe vet ./app/...`（清缓存后） | **零输出，exit 0**（go1.26.7 windows/amd64） |
| 最终 G replay | 同头部统一命令（全量未过滤） | exit 0（app/tests ok、harness ok 39.393s）；golden 目录 git status 为空 |
| Golden 字节校验 | `git diff HEAD -- app/tests/golden`（0 行）+ `git hash-object` 逐文件对比 HEAD blob（132/132 一致）+ 工作区/HEAD 文件数 132=132 | Golden 零变化（注：SHA256 对 git ls-tree blob SHA 的直接比对属算法错配，不作数；以 git 自身规范化哈希对比为准） |
| gofmt 内容复核 | 各批次 `git diff` hunk 级审查 | 全部 36 处修复仅为预期最小 hunk；`gofmt -l` 对触碰文件的整文件列报为仓库既有 CRLF 现象（HEAD 与未触碰文件同样列报），非本任务引入 |

## 遗留与移交

- 本阶段零跳过 hunk；Phase 4 五项 implement.md 检查项全部勾选并附证据。
- Phase 5（CI 矩阵 / record-export-pdf 迁移 / README / harness server 契约 / quality spec /
  Travis / revel run & package smoke / npm test）与 Phase 6（双版本全量验收）不在本次范围。
