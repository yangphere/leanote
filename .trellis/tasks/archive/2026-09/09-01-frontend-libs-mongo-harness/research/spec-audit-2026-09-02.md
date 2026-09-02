# B-E2 规格审核与根因核实（2026-09-02）

## 结论

B-E2 方向正确，但原 PRD 有两处会直接误导实现的缺陷：**模式选择机制完全未定义**（引用的 `LEANOTE_TEST_MONGO_URL` 在 harness 中不存在），以及**"恢复 fixture 一次"与现行的每测试隔离语义矛盾**（测试实测改写 fixture）。已依据代码与 CI 证据修复 PRD；无需用户裁决的开放问题。审核只改本任务规格与研究材料。

## Ready Selection Evidence

- B-E1 已归档（`archive/2026-09/`），B-E2 `meta.depends_on=[09-01-frontend-libs-build-mode]` 满足，是当前唯一 ready 叶（B-E3 依赖 B-E1+B-E2，其余依序后排）。

## 根因核实（代码级证据链）

CI 证据：[run 33522450969 / mongo-8_0 job `99904829590`](https://github.com/yangphere/leanote/actions/runs/33522450969/job/99904829590)（`99abfab` 谱系，2026-09-01；原始缺陷 run `33477561244` / job `99759909276` 同因）：

1. job 级步骤顺序：service 容器（mongo:8.0@sha256:376f…，health-check ping）→ 宿主安装 mongorestore → `mongorestore --db leanote_test`（**成功**，日志含全部 collection 恢复记录）→ `LEANOTE_GOLDEN=replay LEANOTE_REQUIRE_MONGO=1 go test -p 1 ./app/tests/...`。
2. 失败行：每个 `startBaselineServer` 测试在 `docker run -d --rm --name leanote-test-mongo -p 27017:27017` 处 `exit status 125`，docker 报 `Bind for 0.0.0.0:27017 failed: port is already allocated`（service 已占 27017）。失败测试含 `TestGoldenAPIActions`、`TestUpdateNoteOrContent*` 全系（golden/note-save/usn/smoke 共 11 处 `startBaselineServer` 调用，全仓 `grep` 复核；CI 日志 10 个失败，golden_test.go:138 的 ExportPdf 在缺 wkhtmltopdf 时先跳过未触达 `Up()`）。
3. `app/tests/auth_test.go` **不冲突**：拨号 27017 + `LEANOTE_REQUIRE_MONGO=1` 时失败否则跳过，直连 `mongodb://127.0.0.1:27017/leanote_test`——已是 service 兼容形态并通过。

### 现行架构事实

| 组件 | 行为 | 证据 |
|---|---|---|
| `MongoEnvironment.Up()` | 无条件 `docker rm -f` + `docker run -p 27017:27017` + 容器内 `mongorestore --drop` + users==2 校验 | environment.go:36-84 |
| `startBaselineServer` | 每测试调 `Up()`，`t.Cleanup` 调 `Down()`——**每测试一次全新恢复** | integration_test.go:44-56 |
| e2e supervisor | `Up()` 一次（无端口预检）+ Revel server + 子进程；信号/失败全链路 teardown | app/tests/harness/cmd/e2e/main.go:175-305 |
| `cmd/env` | 本地 up/down 手动 helper | cmd/env/main.go |
| `LEANOTE_GOLDEN` | 空=Replay（本地默认回放），非法值报错 | golden_store.go:61-69 |
| `LEANOTE_REQUIRE_MONGO` | 仅 auth_test 用作"必须有 Mongo 否则 fail"门控 | auth_test.go:16 |
| `LEANOTE_TEST_MONGO_URL` | **harness 中不存在**；仅 configuration_test 作 app.conf `db.urlEnv` 插值 fixture（t.Setenv 作用域） | configuration_test.go:84-101 |

### 隔离语义是硬约束（D2 的证据）

harness 测试**直接改写** fixture：`integration_test.go:381,393` Insert files/attachs；`note_save_contract_test.go:53-54,101-102,125` Delete notes/note_contents；另有 API 写路径（note save、USN 推进、垃圾箱 fixtureTrashNoteID 依赖初始态）。今天每测试的 `--drop` 重恢复是这些测试能按任意顺序独立通过的基础（全仓 11 处调用点）。

## Audit By Requirement Dimension

### D1 模式选择机制未定义（关键缺陷）

原 Req 1 把 `LEANOTE_TEST_MONGO_URL` 写成 service-backed 的选择器之一，但该变量在 harness 不存在；"没有外部 service/URI 时自建"隐含探测式 fallback，与 E design "冲突在启动前失败"的 fail-closed 原则冲突。依据现有语义定死机制（零 DX 变更、零 workflow 变更）：

- **`LEANOTE_REQUIRE_MONGO=1` ⇒ service-backed**：复用 auth_test 已有语义（"外部 Mongo 必须在"）。此模式下禁止任何 docker 调用；消费 `mongodb://127.0.0.1:27017/leanote_test`，或 `LEANOTE_TEST_MONGO_URL` 显式覆盖（覆盖值的数据库名必须恰为 `leanote_test`，否则启动前失败）。CI mongo job 现有命令行已设该变量，**无需改 workflow**。
- **未设 ⇒ 自建**（现状不变）：`startBaselineServer` 照旧 `Up()`，本地开发体验零变化。
- **e2e supervisor 恒为自建**：`Up()` 前必须显式断言 27017 无监听，冲突时以明确错误失败（替代 docker 125 的含糊报错），落实 E design "运行前不得存在外部 Mongo service/URI"。
- 变量名共存说明：`LEANOTE_TEST_MONGO_URL` 同时是 app.conf `db.urlEnv` 插值变量（configuration_test 以 t.Setenv 模拟），harness 读它作 URI 覆盖与配置插值语义一致、进程隔离无冲突。

### D2 "恢复 fixture 一次"与每测试隔离矛盾（关键缺陷）

原 Req 1 "恢复 fixture 一次并设置 LEANOTE_REQUIRE_MONGO=1" 混淆了两个层次：

- **job 级 bootstrap 恢复**（CI 现有步骤，宿主 mongorestore 进 service）：服务 auth_test 等非 harness 测试，保留不动。
- **harness 每测试隔离恢复**：service-backed 模式必须用**宿主 `mongorestore --drop --db leanote_test`** 在每个 `startBaselineServer` 等价点重恢复（CI runner 已安装该二进制；本地 service 模式缺失时 fail closed）。两模式的隔离语义必须等价，否则 11 处调用点的顺序独立性失去保障。

### D3 fail-closed 校验清单具体化

启动前必须非零失败的场景：URI 数据库名 ≠ `leanote_test`；service 模式 ping 不通；service 模式缺宿主 mongorestore；supervisor 自建前 27017 已被占用；`LEANOTE_TEST_MONGO_URL` 含凭据（记录时脱敏为 scheme://host:port/db）。

### D4 验收缺口

原 AC 无法证明"不启动第二个容器"。补两级证据：**单元级**（fake `commandRun` 断言 REQUIRE=1 路径零 docker 调用、URI/端口校验错误路径；environment_test.go 已有 fake 先例）；**CI 级**（mongo-8_0 job 全绿且日志无 `leanote-test-mongo`/`port is already allocated`）。另补隔离等价 AC：两模式下同一套 12 处调用测试通过。

### D5 元数据

task.json `base_branch: master` 与实际集成流矛盾（同 B-E1 缺陷），已改 `dev`。

## 规格修复清单（对 PRD 的改动）

1. Confirmed Defect 更新为上述精确证据（job URL、失败行、12 调用点）。
2. Requirements 重写：模式判定（REQUIRE/未设/supervisor 三态）、service-backed 每测试宿主恢复、supervisor 前置端口断言、fail-closed 清单、脱敏记录。
3. AC 补单元级零 docker 断言、CI 日志无第二容器、隔离等价、CI job 全绿（或保留根因/owner/复验命令）。
4. Out Of Scope 补：chromium job 的 e2e 复验属 B-E3；不新增 workflow 触发器（现有 REQUIRE=1 已足够）。
5. task.json base_branch master→dev。

## 无需用户确认的判断

机制选择（REQUIRE=1 复用）、隔离等价（宿主每测试恢复）、零 workflow 变更均可由代码与 CI 证据唯一确定；本地 DX（未设 REQUIRE 时自动 Up）保持现状，无产品取舍。

### 第二轮修复（2026-09-02，用户指示"修复审核发现的问题"）

- 按 workflow.md:163-167 补齐复杂任务三件套：新增 `design.md`（机制、取舍、兼容与回滚），并将 PRD 中原 "Mode Selection Contract"/"Fixture Isolation Contract" 两节的技术设计内容（env 门控实现、宿主 mongorestore 机制、fake 测试策略）移入 design.md；PRD 重排为行为级 Requirements R1-R5 并指回 design.md。
- implement.md 的回归用例细节改为引用 design.md §5，避免执行清单复述设计。

### 评审后补录（2026-09-02 code-review 双轴）

- 修复调用点计数 12→11（首次 grep 将函数定义行计入；CI 实际 10 失败 + 1 跳过的机制已注明）。
- "进程隔离"表述错误已修正：configuration_test 与 harness 同包同二进制，隔离靠 `t.Setenv` 作用域；据此在 PRD 增补"模式判定按调用点读取、不得包级缓存"约束。
- 补录两处首次重写未登记的删除：原"Dependencies And Order"节已恢复（B-E3 归因次序理由）；原 AC"未提供模式时不启动应用"确认废除——被三态契约取代（未设 REQUIRE=自建是合法模式，"不启动应用"不再成立）。
- e2e supervisor 引用路径/行号修正；jsonl 去除与文档重复的 run/job ID；task.json 补回行尾换行；按 workflow.md 复杂任务护栏补 implement.md（设计依据仍以 PRD 契约 + 本研究文档为准）。

## 审核过程 provenance

- `gh run view 33522450969 --job 99904829590 --log` 提取失败行与恢复日志。
- 通读 `app/tests/harness/{environment,integration_test,golden_store}.go`、`cmd/{e2e,env}/main.go`、`app/tests/auth_test.go`、configuration_test 插值用法、quality-gate.yml mongo-8_0 job。
- `grep startBaselineServer`（11 处调用 + 1 处定义；首次审核误将定义计入，评审后修正）与 Insert/Delete 扫描确立隔离需求。
- E design §3.2、implement.md Task 3、evidence-matrix AC-E5 行交叉核对一致。
- 未修改任何业务实现、测试、workflow；未激活任务（待用户批准）。
