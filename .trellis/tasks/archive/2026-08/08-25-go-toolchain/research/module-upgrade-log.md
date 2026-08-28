# Module Upgrade Log — Task 08-25-go-toolchain (Phase 3)

> 逐事务记录 Phase 3 剩余模块升级：current→target、执行命令、结果、GoVersion/许可证、
> go.mod/go.sum diff 归因。环境：Windows，系统 Go 1.27.0（构建/单测），replay 强制
> `LEANOTE_TEST_GO=C:\Users\rog\sdk\go1.26.7\bin\go.exe` + `LEANOTE_GOLDEN=replay`。
> MongoDB：Docker `mongo:5.0` 容器 `leanote-test-mongo`（`env up` 启动，验证 fixture 恢复成功）。
> 注意：integration 测试的 `t.Cleanup(environment.Down())` 会在每次 replay 结束时按设计移除该容器
> （`--rm` 创建），容器消失属夹具正常生命周期，非异常。
>
> 前序已完成（不在本日志重复验证）：bootstrap 批次（`go 1.26` directive + x/tools v0.49.0）、
> x/crypto → v0.55.0（bcrypt 冻结 hash 契约全绿）。

## Golden 基线

- 回放前对 `app/tests/golden/` 全部 132 个文件计算 SHA256 作为基线；每次 replay 后
  `git status --short app/tests/golden` 必须为空且全量哈希与基线零差异。

---

## Transaction 1: github.com/PuerkitoBio/goquery v1.6.1 → v1.12.0

**日期**: 2026-08-26　**结果**: ✅ PASS

### 执行

| 步骤 | 命令 | 结果 |
|---|---|---|
| 升级 | `go get github.com/PuerkitoBio/goquery@v1.12.0` | exit 0；输出 `upgraded goquery v1.6.1 => v1.12.0`、`upgraded cascadia v1.1.0 => v1.3.3` |
| 调用方验证 | `go test ./app/lea -run TestSubStringHTML -count=1 -v` | exit 0；`TestSubStringHTMLFrozenContract` 13 子用例 + `TestSubStringHTMLToRawBasics` 全部 PASS（覆盖截断、多字节计数、不完整标签丢弃、实体、void 标签、嵌套标签补全） |
| 全量构建 | `go build ./app/...` | exit 0 |
| G replay | `$env:LEANOTE_TEST_GO=…go1.26.7…; $env:LEANOTE_GOLDEN='replay'; go test -p 1 ./app/tests/... -count=1 -timeout 30m` | exit 0（app/tests ok 0.299s，harness ok 38.190s）；golden 目录 git status 为空，132 文件 SHA256 与基线零 diff |

### 模块元数据

| 模块 | 版本 | GoVersion | 许可证 | 判定 |
|---|---|---|---|---|
| github.com/PuerkitoBio/goquery | v1.12.0 | **1.25.0** | BSD-3-Clause（LICENSE: "Copyright (c) 2012-2021, Martin Angers & Contributors"） | ≤1.26 ✓，许可证无变化 ✓ |
| github.com/andybalholm/cascadia（传递） | v1.3.3 | 1.16 | MIT 风格（LICENSE: "Copyright (c) 2011 Andy Balholm", redistribution 条款） | ≤1.26 ✓ |

### Diff 归因

- go.mod: `github.com/PuerkitoBio/goquery v1.6.1 → v1.12.0`（直接依赖，R-A2 已批准目标版本）。
- go.mod: `github.com/andybalholm/cascadia v1.1.0 → v1.3.3`（传递）：`go mod graph` 证明
  `github.com/PuerkitoBio/goquery@v1.12.0 github.com/andybalholm/cascadia@v1.3.3`，
  即 goquery 新版自身要求 cascadia v1.3.3。cascadia 无其他 require 路径提升它。
- x/net：goquery@v1.12.0 要求 `golang.org/x/net@v0.52.0`，主模块已解析至 v0.58.0
  （前序 x/tools/x/crypto 批次引入）≥ v0.52.0 ⇒ 本事务对 x/net 零变化。
- Revel/mgo 版本零变化。
- 本事务未运行 tidy；go.sum 中旧版 goquery v1.6.1 / cascadia v1.1.0 行保留属正常
  （模块图仍被引用），将在 Transaction 4 的 `go mod tidy` 中统一收敛并复核。

---

## Transaction 2: github.com/jessevdk/go-flags v1.4.0 → v1.6.1

**日期**: 2026-08-26　**结果**: ✅ PASS

### 执行

| 步骤 | 命令 | 结果 |
|---|---|---|
| 升级 | `go get github.com/jessevdk/go-flags@v1.6.1` | exit 0；仅 `upgraded go-flags v1.4.0 => v1.6.1`，零传递升级 |
| 调用方验证（flags 契约） | `go test ./app/cmd -count=1 -v` | exit 0；`TestParseBuildInvocation`、`TestUpdateBuildConfigDefaults`、`TestGocolorizePlainContract` 全部 PASS（CLI 参数解析 + build 配置默认值 + gocolorize 纯文本契约） |
| 真实生成入口 smoke | `$env:LEANOTE_TEST_GO='…go1.26.7…'; go test ./app/tests/harness -run TestGenerateLegacyEntrypointAndBinary -count=1 -v` | exit 0，4.21s PASS：真实 routes/tmp 生成无 panic，生成后二进制可构建 |
| 全量构建 | `go build ./app/...` | exit 0 |
| G replay | 同 replay 协议（Go 1.26.7 + LEANOTE_GOLDEN=replay） | exit 0（app/tests ok 0.046s，harness ok 36.083s）；golden git status 为空，132 文件 SHA256 与基线零 diff |

### 模块元数据

| 模块 | 版本 | GoVersion | 许可证 | 判定 |
|---|---|---|---|---|
| github.com/jessevdk/go-flags | v1.6.1 | **1.20** | BSD-3-Clause（LICENSE: "Copyright (c) 2012 Jesse van den Kieboom", redistribution 条款） | ≤1.26 ✓ |

### Diff 归因

- go.mod: `github.com/jessevdk/go-flags v1.4.0 → v1.6.1`（直接依赖，R-A2 已批准目标版本）。
- 零传递变化：`go mod graph` 中 go-flags@v1.6.1 仅要求 `golang.org/x/sys@v0.21.0`，
  主模块已解析至 x/sys v0.47.0 ≥ 0.21.0 ⇒ 不触发任何 bump。Revel/mgo 版本零变化。

---

## Transaction 3: keep-module 验证（无升级）

**日期**: 2026-08-26　**结果**: ✅ PASS

### 3a. github.com/robfig/config（保持 v0.0.0-20141207224736-0f78529c8c7e）

| 步骤 | 命令 | 结果 |
|---|---|---|
| 消息解析契约 | `go test ./app/lea/i18n -count=1 -v` | exit 0；`TestMessageContract` PASS，覆盖全部 7 个 locale 目录（de-de、en-us、es-co、fr-fr、pt-pt、zh-cn、zh-hk）及默认 messages 合并：文件发现、section/key、Unicode、插值与缺失键行为全部锁定 |
| 上游可用版本 | `go list -m -u github.com/robfig/config` | 输出当前伪版本且无 `[vX.Y.Z]` 升级提示 ⇒ 上游无可升级版本，与 PRD R-A2 记录一致 |

### 3b. MOD-003 延期决策可链接记录

- 注册位置：`docs/modernization-backlog.md` **第 23–31 行**（"## MOD-003 替换运行时消息配置解析器"）。
- 该条目完整记录：状态（延期，不阻断）、问题、本轮处理（A 保持 robfig/config 当前版本 +
  七语言契约作为替换前基线）、延期原因、启动条件、Owner（独立后端维护任务）与建议验收。
- 关联引用一致：`.trellis/tasks/08-25-go-toolchain/prd.md` R-A2/A-AC2、`design.md` 第 3 节、
  第 9 节延期设计、`app/lea/i18n/i18n_contract_test.go` 注释均指向同一 MOD-003 决策。

### 3c. github.com/agtorre/gocolorize（保持 v1.0.0）

| 步骤 | 命令 | 结果 |
|---|---|---|
| 上游无更新确认 | `go list -m -u github.com/agtorre/gocolorize` | 仅输出 `github.com/agtorre/gocolorize v1.0.0`，无 `[vX.Y.Z]` 后缀 ⇒ 上游自 v1.0.0 后无任何发布，与 R-A2 "保持；C-b 删除 app/cmd 时一并消失" 的处置一致 |

### 本事务 diff 归因

- 零模块图变化：本事务只运行测试与查询命令，未执行任何 `go get`/tidy。

---

## Transaction 4: 最终模块卫生（tidy / verify / 全量 diff 复核）

**日期**: 2026-08-26　**结果**: ✅ PASS

### 执行

| 步骤 | 命令 | 结果 |
|---|---|---|
| tidy #1 | `go mod tidy` | exit 0，无输出 |
| tidy #2 + 双跑零 diff | 快照后再次 `go mod tidy`，对 go.mod/go.sum 与快照逐行比对 | exit 0；go.mod、go.sum 均 **SECOND TIDY ZERO DIFF** |
| 校验 | `go mod verify` | exit 0，`all modules verified` |
| 构建复核 | `go build ./app/...` | exit 0 |

### 升级模块 GoVersion 下限核对（均 ≤ go 1.26）

| 模块 | 解析版本 | GoVersion |
|---|---|---|
| github.com/PuerkitoBio/goquery | v1.12.0 | 1.25.0 ✓ |
| github.com/andybalholm/cascadia | v1.3.3 | 1.16 ✓ |
| github.com/jessevdk/go-flags | v1.6.1 | 1.20 ✓ |
| golang.org/x/crypto | v0.55.0 | 1.25.0 ✓ |
| golang.org/x/tools | v0.49.0 | 1.25.0 ✓ |

### 最终 `git diff go.mod` 全量归因（vs HEAD）

每一行变化均可归因于已批准批次；无未知模块、无第二套框架/日志/配置实现：

1. `go 1.15 → go 1.26` —— bootstrap 批次（A-AC1）。
2. goquery v1.6.1 → v1.12.0 —— Transaction 1。
3. go-flags v1.4.0 → v1.6.1 —— Transaction 2。
4. x/crypto 2020-pseudo → v0.55.0 —— 前序事务（本日志范围前已完成）。
5. x/tools 2020-pseudo → v0.49.0 —— bootstrap 批次。
6. 直接依赖区块重组（gomemcache/redigo/go-cache/revel/cmd/revel/modules/revel/revel/
   robfig/config/mgo.v2 从带 `// indirect` 注释移入直接 require 区块）：**版本值全部不变**，
   纯属 `go mod tidy` 在 `go ≥ 1.17` 剪枝语义下的注释归位——这些模块本就被 app 源码直接 import，
   旧 go.mod 的 `// indirect` 标记是陈旧的。Revel 与 mgo 的**解析版本零变化**
   （revel/cmd v1.0.3、revel/modules v1.0.0、revel/revel v1.0.0、mgo.v2 v2.0.0-20190816093944）。
7. 新增 indirect 行 mattn/go-isatty v0.0.12、pkg/errors v0.9.1：tidy 补全剪枝模块图必需条目
   （分别是 go-colorable 与 revel 的既有传递依赖），版本值与 HEAD go.sum 中已有记录一致。
8. x/mod v0.39.0、x/net v0.58.0、x/sync v0.22.0、x/sys v0.47.0：x/tools v0.49.0 +
   x/crypto v0.55.0（及 goquery→cascadia 链）的传递要求。

### 最终 `git diff go.sum` 归因

- 版本升级哈希替换：goquery/cascadia/go-flags/x/crypto/x/tools 及 x/net、x/sys、x/sync、
  x/mod 的新旧 h1:/go.mod 哈希——全部对应上述已批准升级链。
- 图完整性新增（`/go.mod` 哈希或测试依赖 h1:）：BurntSushi/toml、davecgh/go-spew v1.1.1、
  google/go-cmp、myesui/uuid、pmezard/go-difflib、stretchr/testify v1.4.0、gopkg.in/check.v1、
  gopkg.in/stretchr/testify.v1、gopkg.in/yaml.v2、yuin/goldmark、x/telemetry、x/term 链、
  x/text 链、x/xerrors——均为 `go directive ≥ 1.17` 后剪枝模块图要求 go.sum 记录完整校验和的
  机械结果（bootstrap 批次的既知效应），不引入任何新代码路径。
- 陈旧条目清理：旧 mgo v2.0.0-20180705113604 h1:（2019 已被 a6b53ec6cb22 取代）、
  patrickmn/go-cache v1.0.0 h1:（实际用 v2.1.0+incompatible）、部分过期 x/net/x/crypto /go.mod 行。
- **关键不变量**：diff 中不存在任何 `github.com/revel/*` 或 `gopkg.in/mgo.v2` 的版本值变化；
  mgo 当前版本的 h1:/go.mod 两行原样保留。
