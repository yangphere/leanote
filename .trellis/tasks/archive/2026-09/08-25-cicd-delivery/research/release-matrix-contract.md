# 发布浏览器矩阵契约（F，v1）

## Canonical 输入（Q-F5 已确认）

唯一的发布输入是受保护真实浏览器证据 workflow 生成的
`browser-release-matrix-v1` artifact，而不是源码中的 tracked 文件。该 artifact 只在
release workflow 已锁定严格 `vX.Y.Z` tag 的 checkout SHA 后生成一次，载荷文件名固定为
`release-matrix.json`；F 不复制 frontend-libs 的浏览器配置，也不创建缺证据的占位文件。

矩阵载荷中的顶层和每条记录 `commit` 均填写 tag commit。由于载荷在 tag commit 之后的
workflow 运行中产生，它不进入源码提交，也不自引用最终 commit；commit 绑定由同 artifact
中的 provenance 清单和 release workflow 当前 run/attempt 完成。

artifact allowlist 恰好为以下两个文件，禁止附带截图、视频、trace、storage state、Cookie、
认证头或页面正文：

- `release-matrix.json`：本材料下方定义的八行矩阵 schema；
- `provenance.json`：本材料定义的矩阵哈希、tag/ref、commit、producer workflow 和 release run
  绑定信息。

### Provenance schema

`provenance.json` 必须通过以下等价 JSON Schema；实现可使用等价校验器，但不得放宽字段、格式
或 `additionalProperties` 约束：

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:leanote:browser-smoke:release-matrix-provenance:v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "matrix_sha256", "commit", "ref", "producer_workflow", "release_run"],
  "properties": {
    "schema_version": { "const": "leanote.browser-smoke.release-matrix-provenance.v1" },
    "matrix_sha256": { "type": "string", "pattern": "^[0-9a-f]{64}$" },
    "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "ref": { "type": "string", "pattern": "^refs/tags/v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$" },
    "producer_workflow": { "type": "string", "minLength": 1, "maxLength": 120 },
    "release_run": {
      "type": "object", "additionalProperties": false,
      "required": ["id", "attempt"],
      "properties": {
        "id": { "type": "string", "pattern": "^[0-9]+$" },
        "attempt": { "type": "integer", "minimum": 1 }
      }
    }
  }
}
```

release validator 必须重新计算 `release-matrix.json` 的 SHA-256，并确认 provenance 的
`matrix_sha256`、`commit`、`ref` 和 `release_run` 分别等于当前载荷哈希、tag checkout SHA、
当前 tag ref 和当前 release workflow 的 run/attempt；producer workflow 必须是受保护的真实
浏览器证据 workflow。artifact 名称、文件数量或 provenance 绑定任一不符均在创建 Release/GHCR
前失败。每个 release `run.id`/`run.attempt` 只允许一个该名称 artifact；重试产生的新 attempt
不得复用旧 attempt 的矩阵，旧 artifact 会被视为重复/跨 attempt 并阻断。artifact 保留期不超过
7 天，workflow 不覆盖或删除；人工删除、重试和恢复遵守 F 的发布人工边界。

## 结构与完整性

`release-matrix.json` 必须通过以下 JSON Schema（draft 2020-12）；实现可使用等价的本地校验器，
但不得放宽 `const`、`enum`、数量、格式或 `additionalProperties` 约束：

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:leanote:browser-smoke:release-matrix:v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "commit", "records"],
  "properties": {
    "schema_version": { "const": "leanote.browser-smoke.release-matrix.v1" },
    "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "records": {
      "type": "array", "minItems": 8, "maxItems": 8,
      "items": {
        "type": "object", "additionalProperties": false,
        "required": [
          "commit", "browser_product", "release_slot", "browser_version", "os", "environment",
          "coverage", "auth_gate", "error_gate", "resource_gate", "executed_at", "result"
        ],
        "properties": {
          "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
          "browser_product": { "enum": ["chrome", "edge", "firefox", "safari"] },
          "release_slot": { "enum": ["current_major", "previous_major"] },
          "browser_version": { "type": "string", "pattern": "^[0-9]+(?:\\.[0-9]+){1,3}$" },
          "os": { "type": "string", "minLength": 1, "maxLength": 120 },
          "environment": { "const": "real-browser" },
          "coverage": {
            "type": "array", "minItems": 1, "maxItems": 40,
            "items": { "type": "string", "pattern": "^[a-z0-9][a-z0-9._-]{0,79}$" }
          },
          "auth_gate": { "enum": ["passed", "failed", "not_run"] },
          "error_gate": { "enum": ["passed", "failed", "not_run"] },
          "resource_gate": { "enum": ["passed", "failed", "not_run"] },
          "executed_at": { "type": "string", "format": "date-time", "pattern": "Z$" },
          "result": { "enum": ["passed", "failed"] }
        }
      }
    }
  }
}
```

除 JSON Schema 外，发布 validator 还必须执行以下跨记录规则：

- `schema_version` 必须精确为 `leanote.browser-smoke.release-matrix.v1`；顶层 `commit` 和每条记录
  的 `commit` 必须是同一发布 commit 的 40 位小写 SHA-1。
- `records` 必须恰好 8 条，唯一键为 `browser_product + "/" + release_slot`，且完整覆盖
  `chrome`、`edge`、`firefox`、`safari` × `current_major`、`previous_major`。同一产品/slot 不得重复，
  不得添加第九条记录代替缺失项。
- `browser_version` 必须是探测到的完整版本字符串，不得使用 `latest`、`current`、`previous`、
  `x` 或其他占位值；`os`、`coverage`、`executed_at` 均为非空值。`coverage` 使用稳定 scope id，
  不写页面正文或账号信息。
- 所有记录的 `environment` 必须为 `real-browser`。Safari 记录必须来自真实 Safari；WebKit、
  Playwright Chromium、远程模拟器或仅 user-agent 覆盖均不合格。Chrome、Edge、Firefox 同样不能
  以其他产品或引擎代替。
- 发布门要求 8 条记录的 `auth_gate`、`error_gate`、`resource_gate` 和 `result` 全部为 `passed`；
  任一 `failed`、`not_run`、commit/版本不符或文件缺失都阻断发布。该文件本身不携带截图、视频、
  storage state、Cookie、认证头或页面正文，必要的运行链接只能在外部脱敏审计记录中维护。
- 任何使用本契约的校验器必须启用 `format` 校验（尤其是 RFC3339 UTC `executed_at`），不能只
  按 JSON 类型或正则接受本地时区、占位版本或格式错误的时间。

## 主版本判定

每条记录的 `executed_at` 是版本判定时间。`current_major` 指该时间点浏览器产品官方 stable
channel 已发布的最高主版本；`previous_major` 指紧邻的上一官方 stable 主版本，不是上一 patch、
beta、dev 或企业自定义构建。执行者必须在真实产品环境中探测完整 `browser_version`，并以该版本的
主版本号与 slot 交叉校验；Safari 使用 Apple 官方 Safari/macOS stable 版本。版本探测命令或厂商
版本页只保留在外部脱敏审计记录中，不能用占位文本写入矩阵。
