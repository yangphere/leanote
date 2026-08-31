# CI 失败摘要契约（F，v1）

## 目的与范围

质量门的每个 job（包括 Go 矩阵的每个 leg、Mongo、Node/build、Chromium、package、container 和
summary）都必须产出一条机器可校验的摘要。摘要用于定位失败阶段，不是原始日志归档；成功、失败、
取消和未启动都必须有记录。逻辑输出目录为 `ci-summaries/`，单个文件名为
`<job-id>.json`，服务健康信息只能写入同一摘要的 `service` 字段，不得另传未定义的健康 artifact。

质量门的预期 job 集合固定为 `go-1_26_7`、`go-1_27_0`、`mongo-8_0`、`node-build`、
`chromium-e2e`、`package-smoke`、`container-smoke` 和 `summary`。这些是摘要中的唯一合法 job ID；
Go 矩阵的每个 leg 必须分别映射到对应版本 ID，汇总 job 必须显式列出并检查全部 8 个 ID。

## JSON schema

实现可使用等价的本地 schema 校验器，但字段、枚举和禁止项不得放宽：

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:leanote:ci-failure-summary:v1",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "schema_version", "workflow", "job", "run", "commit", "ref", "status", "stage",
    "toolchain", "failure", "service", "tests", "page_paths", "resource_paths", "status_codes", "generated_at"
  ],
  "properties": {
    "schema_version": { "const": "leanote.ci.failure-summary.v1" },
    "workflow": { "type": "string", "minLength": 1, "maxLength": 120 },
    "job": {
      "enum": [
        "go-1_26_7", "go-1_27_0", "mongo-8_0", "node-build",
        "chromium-e2e", "package-smoke", "container-smoke", "summary"
      ]
    },
    "run": {
      "type": "object", "additionalProperties": false,
      "required": ["id", "attempt"],
      "properties": {
        "id": { "type": "string", "pattern": "^[0-9]+$" },
        "attempt": { "type": "integer", "minimum": 1 }
      }
    },
    "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "ref": { "type": "string", "minLength": 1, "maxLength": 255 },
    "status": { "enum": ["passed", "failed", "cancelled", "not_run"] },
    "stage": { "type": "string", "minLength": 1, "maxLength": 80 },
    "toolchain": {
      "type": "object", "additionalProperties": false,
      "required": ["go", "node", "npm", "mongo", "playwright"],
      "properties": {
        "go": { "type": ["string", "null"], "maxLength": 40 },
        "node": { "type": ["string", "null"], "maxLength": 40 },
        "npm": { "type": ["string", "null"], "maxLength": 40 },
        "mongo": { "type": ["string", "null"], "maxLength": 80 },
        "playwright": { "type": ["string", "null"], "maxLength": 80 }
      }
    },
    "failure": {
      "type": "object", "additionalProperties": false,
      "required": ["category", "message", "exit_code"],
      "properties": {
        "category": {
          "enum": [
            "none", "job_not_started", "checkout", "setup", "dependency", "compile", "lint",
            "test", "discovery_zero", "service_readiness", "drift", "package", "container",
            "pdf", "version", "cleanup", "schema", "permission", "timeout", "unknown"
          ]
        },
        "message": { "type": "string", "maxLength": 500 },
        "exit_code": { "type": ["integer", "null"], "minimum": 0 }
      }
    },
    "service": {
      "type": "object", "additionalProperties": false,
      "required": ["health_path", "readiness", "http_status", "exit_code"],
      "properties": {
        "health_path": {
          "type": ["string", "null"],
          "pattern": "^(/[A-Za-z0-9._~/?=&%-]{0,240})?$"
        },
        "readiness": { "enum": ["passed", "failed", "not_run", "unknown"] },
        "http_status": { "type": ["integer", "null"], "minimum": 100, "maximum": 599 },
        "exit_code": { "type": ["integer", "null"], "minimum": 0 }
      }
    },
    "tests": {
      "type": "object", "additionalProperties": false,
      "required": ["discovery", "discovered_count", "executed_count"],
      "properties": {
        "discovery": { "enum": ["passed", "failed", "not_run"] },
        "discovered_count": { "type": ["integer", "null"], "minimum": 0 },
        "executed_count": { "type": ["integer", "null"], "minimum": 0 }
      }
    },
    "page_paths": {
      "type": "array", "maxItems": 20,
      "items": { "type": "string", "pattern": "^/[A-Za-z0-9._~/?=&%-]{0,240}$" }
    },
    "resource_paths": {
      "type": "array", "maxItems": 20,
      "items": { "type": "string", "pattern": "^/[A-Za-z0-9._~/?=&%-]{0,240}$" }
    },
    "status_codes": {
      "type": "array", "maxItems": 20,
      "items": { "type": "integer", "minimum": 100, "maximum": 599 }
    },
    "generated_at": { "type": "string", "format": "date-time" }
  }
}
```

## 运行规则

- 每个 job 的最后一步使用 `if: always()` 写摘要；摘要步骤本身不能读取或复制原始日志、页面正文、
  Cookie、认证头、storage state、截图、视频或 trace。`failure.message` 只允许脱敏类别和短原因，
  不得包含 token、邮箱、用户数据、绝对本地路径或请求正文。
- checkout、tool setup、依赖安装或服务启动失败时，仍须写最小记录：`status=failed`、
  `failure.category=job_not_started`（若能识别则使用更具体类别）、`service.readiness=not_run`，
  未运行计数为 `null`，并保留实际退出码（未知时为 `null`）。
- 独立 `summary` job 必须以 `needs: [所有质量门 job]` 和 `if: always()` 运行，收集并校验全部 8 个
  固定 job ID 的摘要。缺文件、重复 job id、commit/ref 不一致、schema 失败、预期测试层
  `discovery != passed` 或 `discovered_count == 0`、服务 readiness 未通过、cleanup 失败时，汇总
  job 返回非零；`summary` 自身也必须在 `if: always()` 收尾步骤写入 `summary.json`，其状态和失败
  类别遵守同一 schema。健康摘要只取各记录的 `service.health_path`、`readiness`、`http_status` 和
  `exit_code`，不得上传第二个健康文件。
- `status=passed` 只能与 `failure.category=none` 和成功的服务/测试门禁组合；`status=failed`、
  `cancelled` 或 `not_run` 必须保留非成功类别或明确未运行状态，不能用 `category=none` 伪造成功。
- 只允许上传 `ci-summaries/*.json`；服务健康信息必须来自每条摘要的 `service` 字段，不得上传独立
  健康文件。artifact 保留期最长 7 天。成功运行不上传大型中间物。手工 `record-export-pdf` 是唯一
  例外，只能上传单一不含敏感字段的 Golden JSON，同样最长保留 7 天。
