# 外部事实核验记录（2026-08-25）

本文件记录规划期已**实际核验**的外部事实，供子任务直接引用，避免重复查证。
每条都标注了核验方式。规划期之后若时间跨度较大，标注 ⏳ 的条目值得重新核验。

## 1. mgo 无法连接受支持的 MongoDB（本项目的硬性阻塞项）

**核验方式**：直接扫描本机 Go module cache 中的 mgo 源码。

```
$GOMODCACHE/gopkg.in/mgo.v2@v2.0.0-20190816093944-a6b53ec6cb22/socket.go
  :402  addHeader(buf, 2001)   OP_UPDATE
  :418  addHeader(buf, 2002)   OP_INSERT
  :430  addHeader(buf, 2004)   OP_QUERY
  :448  addHeader(buf, 2005)   OP_GET_MORE
  :456  addHeader(buf, 2006)   OP_DELETE
  :467  addHeader(buf, 2007)   OP_KILL_CURSORS
```

`grep "2013\|OP_MSG\|opMsg"` 在整个 mgo 包中**零命中** ⇒ 该驱动完全不支持 OP_MSG。

**MongoDB 侧**（https://www.mongodb.com/docs/manual/legacy-opcodes/）：
- legacy opcodes 在 MongoDB **5.0 弃用、5.1 起彻底移除**。
- 5.1 起 `OP_MSG` 与 `OP_COMPRESSED` 是唯一受支持的请求 opcode。
- 唯一例外：`OP_QUERY` 仍可用于连接握手阶段的 `hello` / `isMaster`
  ⇒ **旧驱动能"连上"到足以识别服务器版本，但无法执行任何 CRUD**。

**结论**：本项目被钉死在 MongoDB ≤ 5.0，而 MongoDB 5.0 已于 2024-10 EOL。
当前代码无法运行在任何仍受支持的 MongoDB 版本上。

## 2. mgo ObjectId 的 JSON 形态（外部契约保真的关键）

**核验方式**：读 mgo 源码 `bson/bson.go`。

```go
// :166
type ObjectId string          // 内部存原始 12 字节，不是 hex 字符串

// :171-177
func ObjectIdHex(s string) ObjectId {
    d, err := hex.DecodeString(s)
    if err != nil || len(d) != 12 {
        panic(...)             // ← 非法输入 panic，替换时须保持行为等价
    }
    return ObjectId(d)
}

// :268-270
func (id ObjectId) MarshalJSON() ([]byte, error) {
    return []byte(fmt.Sprintf(`"%x"`, string(id))), nil   // ⇒ 24 字符小写 hex
}
```

`UnmarshalJSON`（`:275+`）额外接受 `{"$oid": ...}` 与 `ObjectId("...")` 形式 —— 若客户端会发这类形态，
替换实现时需一并兼容。

**mongo-driver/v2 侧**：`ObjectID` 为 `[12]byte`（v2 中已从 `primitive` 包移入 `bson` 包），
序列化亦为 hex 字符串 ⇒ **两者 JSON 形态很可能一致，但必须由 golden snapshot 钉住，不可假设**。

## 3. 上游依赖可用版本

**核验方式**：`go list -m -versions`（2026-08-25 实测）。

| 模块 | 当前项目 | 上游最新 | 备注 |
|---|---|---|---|
| `go.mongodb.org/mongo-driver/v2` | — | **v2.8.1** | 选定目标 |
| `go.mongodb.org/mongo-driver`（v1 线） | — | v1.17.9 | 不选，避免二次迁移 |
| `github.com/revel/revel` | v1.0.0 | **v1.1.0** | 上游确认止于 v1.1.0 ⇒ 框架停滞 |
| `github.com/revel/cmd` | v1.0.3 | **v1.1.2** | — |
| `gopkg.in/mgo.v2` | v2.0.0-20190816093944 | 同（2018 起停维护） | globalsign fork 亦已归档 |

Revel v1.1.0 release note 内容：修复 log 递归调用、支持 SIGTERM 优雅关停。

## 3.1 采用的支持边界（用户确认，2026-08-25）

| 项 | 官方状态或实测版本 | 本任务采用值 |
|---|---|---|
| Go | 1.27.0 于 2026-08-19 发布；Go 仅支持最新两个主版本 | `go 1.26`；CI 覆盖 1.26 与 1.27 |
| Node.js | 24.19.0 为 LTS；26.7.0 仍为 Current | 构建与阻断 CI 固定 Node 24.x LTS |
| MongoDB | 6.0 已于 2025-07-31 EOL；7.0 支持至 2027-08-31；8.0 支持至 2029-10-31 | 支持 7.0–8.0；CI 与本地基线固定 8.0 |
| jQuery | npm latest 为 4.0.0；3.x 最新为 3.7.1 | 目标 3.7.1，避免把 1.9→3 与 3→4 两次破坏性迁移合并 |
| Bootstrap | 5.3 线最新为 5.3.8 | 目标 5.3.8 |
| TinyMCE | npm latest 为 8.8.2 | 目标 8.8.2 |
| esbuild | npm latest为 0.28.2 | 规划基线 0.28.2；实现前按锁文件评审补丁版本 |

官方来源：

- Go 发布策略与版本记录：https://go.dev/doc/devel/release
- Node.js 版本状态：https://nodejs.org/en/about/previous-releases
- MongoDB 生命周期：https://www.mongodb.com/legal/support-policy/lifecycles
- TinyMCE Community changelog：https://www.tiny.cloud/docs/tinymce/latest/changelog/

版本采用值是本任务的可重复基线，不使用浮动的 `latest` 作为构建输入。

## 4. 本机工具链（可编译性硬证据）

**核验方式**：实测执行。

| 项 | 版本 | 结果 |
|---|---|---|
| Go | `go1.27.0 windows/amd64` | `go build ./app/...` **exit=0** —— 当前代码在 Go 1.27 下可直接编译 |
| Go vet | 同上 | `go vet ./app/service/...` **exit=1**，20+ 处 unkeyed struct literal、`NoteService.go:972` self-assignment |
| Node | v24.19.0 | `npm test` 可用（node:test）；gulp 3 无法安装，仓库无 `node_modules` |
| npm | 11.17.0 | — |
| mongod | **未安装** | 本机无 MongoDB ⇒ `go test ./app/tests/...` 目前无法运行 |
| Docker | 29.7.2 | daemon 可用 |
| Docker Compose | v5.4.0 | 可用于本地 MongoDB 5.0 测试环境 |

## 4a. Revel 1.0 响应 header 行为（golden 归一化规则依据）

**核验方式**：读本机 Go module cache 中 `$GOMODCACHE/github.com/revel/revel@v1.0.0` 源码（2026-08-25）。

- `results.go:399-406`：`BinaryResult.Apply` 在 Delivery 非 NoDisposition 时写
  `Content-Disposition: <attachment|inline>; filename="<Name>"`（disposition 常量 :386-388）。
- `server_adapter_go.go:460-468`：`WriteStream` 对 `io.ReadSeeker` 走标准库
  `http.ServeContent`，因此二进制响应带 `Last-Modified`（= ModTime）与
  `Accept-Ranges: bytes`（标准库 `net/http fs.go` 的 `ServeContent` 无条件写入）。
- `controller.go:311-324`：`RenderFile` 以 `filepath.Base(file.Name())` 作 filename、
  以文件 mtime（fallback `time.Now()`）作 ModTime ⇒ filename **不含** `revel.BasePath`。
- Leanote 侧调用：`ApiFile.GetImage` → `RenderFile(Inline)`；`ApiFile.GetAttach` →
  `RenderBinary(file, attach.Title, Attachment, time.Now())`；`ApiFile.GetAllAttachs` →
  `RenderBinary(fw, note.Title+".tar.gz", Attachment, time.Now())`；
  `ApiNote.ExportPdf` → `RenderBinary(file, FixFilename(note.Title)+".pdf", Attachment, time.Now())`
  （`ApiNoteController.go:634-641`；guid 仅用于磁盘路径 :599，不进 header）。
- `revel.go:242-244`：runMode 在 app.conf 无同名 section 时 `Fatal("Run mode not found")`
  ⇒ `-runMode=test` 必须先存在 `[test]` section。

## 4b. wkhtmltopdf 可用安装源（ExportPdf record job 用）

**核验方式**：GitHub `wkhtmltopdf/packaging` releases 页（2026-08-25）。
仓库已归档（2023-08），最终 release **0.12.6.1-3**（2023-05），资产含
`wkhtmltox_0.12.6.1-3.jammy_amd64.deb` / `...bookworm_amd64.deb` 等 21 个包。
wkhtmltox deb 安装到 `/usr/local`（与 Leanote `exportPdfBinPath` 缺省
`/usr/local/bin/wkhtmltopdf` 精确一致）。
URL：`https://github.com/wkhtmltopdf/packaging/releases/download/0.12.6.1-3/wkhtmltox_0.12.6.1-3.jammy_amd64.deb`

## 5. TinyMCE 目标版本与许可证 ⏳

**核验方式**：npm registry + GitHub LICENSE.md。

- npm `tinymce` 最新发布版本：**8.8.2**（`registry.npmjs.org/tinymce/latest`，
  注意该端点未返回 dist-tags，理论上可能已有更新版本）。
- 许可证：`LICENSE.md`（main 分支）为 **GNU General Public License Version 2 or later**，
  copyright 2024 Ephox Corporation DBA Tiny Technologies ⇒ 与 Leanote 的 GPL v2 **兼容**。

**升级路径上的破坏性事实**：`paste` 插件自 TinyMCE **6** 起并入 core，不再是独立插件。
Leanote 的粘贴修复目前分散在 5 份被 `tests/js/paste-plugin.test.js` 守护的副本中
（`plugins/paste/classes/Clipboard.js`、`plugins/paste/plugin.js`、`plugin.min.js`、
`tinymce.full.js`、`tinymce.full.min.js`），升级后必须重新表达为 core 粘贴事件处理。

当前 `public/tinymce/plugins/` 下 46 个插件中，以下在 5→8 演进中已被移除 / 合并 / 改名，需逐个确认替代：
`contextmenu`、`textcolor`、`colorpicker`、`hr`、`print`、`paste`、`paste_raw`、`noneditable`、
`spellchecker`、`bbcode`、`fullpage`、`layer`、`compat3x`、`legacyoutput`、`textpattern`。

另：目录中同时存在 `leaui_mind` 与 `leaui_mindmap`，需确认哪个在用（疑似历史遗留）。
