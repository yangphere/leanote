# 发布浏览器矩阵契约（F，v1）

## Canonical 输入

唯一受跟踪输入文件为 `docs/modernization/browser-smoke/release-matrix.json`。该文件由
`08-25-frontend-libs` 协调任务在真实 smoke 完成后生成和维护；F 不复制浏览器配置，也不创建缺证据
的占位文件。文件必须随被验证的提交一起存在，发布 job 只接受 tag 指向 commit 的版本。

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

## 主版本判定

每条记录的 `executed_at` 是版本判定时间。`current_major` 指该时间点浏览器产品官方 stable
channel 已发布的最高主版本；`previous_major` 指紧邻的上一官方 stable 主版本，不是上一 patch、
beta、dev 或企业自定义构建。执行者必须在真实产品环境中探测完整 `browser_version`，并以该版本的
主版本号与 slot 交叉校验；Safari 使用 Apple 官方 Safari/macOS stable 版本。版本探测命令或厂商
版本页只保留在外部脱敏审计记录中，不能用占位文本写入矩阵。
