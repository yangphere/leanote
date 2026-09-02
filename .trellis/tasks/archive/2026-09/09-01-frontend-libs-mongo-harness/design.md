# B-E2 技术设计 — Mongo service 与 harness 生命周期统一

依据：`prd.md`（需求与验收）、`research/spec-audit-2026-09-02.md`（根因与证据）。本文件只写"怎么做"与取舍。

## 1. 边界与数据流

```text
运行环境（CI job / 本地 shell / e2e supervisor）
   │  声明：LEANOTE_REQUIRE_MONGO=1？LEANOTE_TEST_MONGO_URL？
   v
harness 模式判定（每调用点求值，禁止包级缓存）
   ├─ REQUIRE=1 ──> service-backed：校验 URI/ping/工具 → 每测试宿主 mongorestore --drop → 起服务跑测试（零 docker）
   └─ 未设     ──> 自建（现状）：Up() 容器 + 容器内恢复（e2e supervisor 额外前置端口断言）
```

改动面：`app/tests/harness/`（environment.go 及模式门控、integration_test.go 的 `startBaselineServer`）、`cmd/e2e/main.go`（端口预检）、新增回归用例。不改业务代码、Mongo 结构、workflow。

## 2. 模式判定设计

- 判定函数按调用点读取 `LEANOTE_REQUIRE_MONGO` 与 `LEANOTE_TEST_MONGO_URL`，**不做包级缓存**：configuration_test 与 harness 同包同二进制，其 `t.Setenv` 窗口（unsafe fixture 指向 `leanote` 库）要求求值必须即时。
- `LEANOTE_REQUIRE_MONGO=1` 复用 auth_test 现有语义（"外部 Mongo 必须在"），不新增变量：CI 命令行已设该值，实现零 workflow 变更；变量语义与 E design §3.2 的"运行环境声明模式"一致。
- `LEANOTE_TEST_MONGO_URL` 为 service-backed 的 URI 覆盖（默认 `mongodb://127.0.0.1:27017/leanote_test`）；解析后数据库名必须恰为 `leanote_test`，否则启动前失败。它与 app.conf `db.urlEnv` 插值同名同义，属同一事实的两种消费。

## 3. 隔离恢复设计

- 自建模式：维持现状（容器内 `mongorestore --drop` + users==2 校验），本地 DX 不变。
- service-backed 模式：`startBaselineServer` 等价点执行宿主 `mongorestore --drop --db leanote_test --dir mongodb_backup/leanote_install_data`（CI runner 已装该二进制；缺失即 fail closed）。选宿主执行而非 `docker exec` 进 service 容器：GitHub service 容器不承诺可 exec，宿主二进制是 job 已验证存在的事实。
- job 级 bootstrap（quality-gate 现有步骤）服务非 harness 测试，保留不动；harness 每测试恢复保证 11 个调用点顺序无关（测试实测直接 Insert/Delete fixture）。

## 4. fail-closed 校验（启动前，按序）

1. URI 覆盖存在 → 解析库名 ≠ `leanote_test` ⇒ 失败。
2. service-backed：ping 不通 ⇒ 失败（不等待、不降级）。
3. service-backed：宿主 mongorestore 缺失 ⇒ 失败。
4. supervisor 自建：27017 已有监听 ⇒ 明确报错（预检替代 docker exit 125 的含糊信息）。

## 5. 测试设计

- 单元级（fake `commandRun`，environment_test.go 先例）：REQUIRE=1 路径断言零 docker 调用；上述 4 类 fail-closed 错误路径；service 模式每测试恰好一次 `--drop` 恢复的调用序断言。
- CI 级：mongo-8_0 job 全绿且日志无 `leanote-test-mongo`/`port is already allocated`。
- 隔离等价：同一套 11 个调用点测试在两模式下均通过。

## 6. 取舍记录

- **复用 REQUIRE 而非新变量**：避免第三种环境语义；CI 零改动；与 auth_test 语义同源。
- **未设 REQUIRE 保持自建**：本地 `go test` 体验零变化；隐式自建不违反 fail-closed（冲突仅存在于 service 模式与自建并发时，supervisor 预检已封堵）。
- **禁探测式自动 fallback**（探测 27017 有应答就当 service）：探测结果与模式声明脱节，违反 E design"冲突在启动前失败"。

## 7. 兼容与回滚

- 本地 Windows/WSL（未设 REQUIRE）：行为不变。CI mongo job：从"全红"变全绿，无灰度需求。chromium job（B-E3 范围）：supervisor 预检使其在 service 存在时显式失败而非含糊超时。
- 回滚形态：单提交 revert 即恢复现状（自建路径未被删除，只是被门控绕行）。
