# B-E4 技术设计 — 四项 coverage 与 artifact 契约升级

依据：`prd.md`、`research/spec-audit-2026-09-02.md`；字段级以 F `release-matrix-contract.md` JSON Schema 为准，本文只写映射与机制。

## 1. 改动面

| 文件 | 改动 |
|---|---|
| `tests/e2e/business/leaui-image-iframe.spec.mjs` | 新文件：迁移 business-flows.spec.mjs 唯一的独立 leaui 契约用例（:84）；大测试 :187 内嵌 leaui 段不拆（见 §2） |
| `playwright.config.mjs` | browser-smoke testMatch 扩为四文件 |
| `scripts/browser-release-evidence.mjs` | 四 ID、marker 协议扩展、coverage_summaries、JCS、每行 digest |
| `scripts/validate-browser-artifact.mjs` | schema 升级、JCS 重算、双相位 |
| `.github/workflows/browser-release-evidence.yml` | 增加 workflow_dispatch 预检入口 |
| `tests/js/release-contract.test.js` | 四用例升级 + 新增契约/JCS/结构断言 |

## 2. 套件迁移的边界

`leaui_image preserves the real parent iframe boundary…`（business-flows:84）与 `leaui_image preserves parent iframe data…`（bootstrap-components:315）是**独立用例**，迁移前者至新文件；大业务流测试（:187）内嵌的 leaui iframe 段是流程覆盖的一部分，**不拆**（保持 22 用例语义与既有断言原样）。迁移后：business = 6 文件/22 用例；browser-smoke = 4 文件四 ID。

## 3. marker 协议扩展（producer ↔ 受保护命令）

现有 marker：`LEANOTE_BROWSER_VERSION`/`LEANOTE_BROWSER_OS`/`LEANOTE_AUTH|ERROR|RESOURCE_GATE`。新增每 coverage 一段（顺序固定）：

```text
LEANOTE_COVERAGE_business_flows=discovered=N;executed=M
LEANOTE_ENTRYPOINTS_business_flows=note,blog
LEANOTE_IFRAMES_business_flows=      (可为空)
```

键名转下划线（`business-flows`→`business_flows`）；entrypoints 为逗号分隔的小写稳定标识符且**至少一项**，iframes 同格式但**可为空**（≤40 项，均匹配 `^[a-z0-9][a-z0-9._/-]{0,79}$`）——与 F schema 的 minItems/maxItems 区分一致。producer 解析失败/缺 marker ⇒ fail-closed。B-E5 执行期命令按此协议输出；B-E4 以单测用合成 stdout 锁定解析。

## 4. JCS 最小规范化器

- 递归序列化：对象键按 UTF-16 码元升序；数组保持原序；字符串用 JSON 转义（载荷域 ASCII，无转义歧义）；数字仅安全整数（`Number.isSafeInteger` 且 ≥1，ECMAScript `String(n)` 即 JCS 规范表示）；无空白、无尾随换行；UTF-8 字节流 SHA-256。
- 单测：RFC 8785 附录向量子集（含键排序与整数字符串化样例）+ 契约载荷正例 + 篡改反例（计数变、序变、空白注入）。
- 摘要输入严格为去掉 `coverage_summary_sha256` 的 `{browser_product, release_slot, items}`。

## 5. validator 双相位

`node scripts/validate-browser-artifact.mjs <dir> [--phase final|precheck] [--expected-commit <sha>]`：

- **final（默认）**：现行为 + 新 schema；`release_run` 必须等于当前 `GITHUB_RUN_ID/ATTEMPT`。
- **precheck**：`--expected-commit`（候选 SHA）必填且须等于载荷 `commit`；ref 为严格 tag 且 commit 为其剥壳 SHA（由调用方解析传入）；**不**校验 run/attempt 等于校验进程；其余规则（八槽位、四 ID、JCS 重算、每行 digest、相邻主版本、原始字节 matrix_sha256）全量执行。
- 两相位均拒绝旧六字段 provenance（无 `coverage_summaries`）与通用 scope。

## 6. workflow_dispatch 预检入口

```yaml
on:
  workflow_call:
  workflow_dispatch:
    inputs:
      tag: { description: strict vX.Y.Z tag, required: true }
```

步骤约束：正则断言 `^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`；`git fetch --force origin refs/tags/<tag>` + `git rev-parse refs/tags/<tag>^{}` 得 RELEASE_COMMIT（同 release.yml 既有剥壳模式，release-contract:210-214 已锁定该模式）；checkout 该 SHA；producer 运行；上传两文件 artifact（retention 7）。`permissions: contents: read` 保持；runner 标签不变。**不新增任何 publish/Release/GHCR 步骤**——结构断言测试扫描 workflow 文本证明（无 `create-release`/`ghcr`/publish job）。

## 7. 兼容与不变量

- release.yml 最终发布流复用升级后的 producer/validator（final 相位），发布门语义不变。
- 旧 artifact（六字段）在升级后自然失效——契约要求拒绝，无兼容期。
- business/browser-smoke 用例内容零修改（仅迁移文件归属）；chromium job 无文件数断言（已核实）。
- 回滚：单 commit revert 即回到旧契约（旧测试同 revert）。
