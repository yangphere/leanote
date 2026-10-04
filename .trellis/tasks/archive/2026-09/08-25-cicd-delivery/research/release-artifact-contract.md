# Release 输入制品交接契约（F，v1）

## 目的与范围

本材料固定 quality-gate 到 `release.yml` 的制品交接，补足“同一 run/commit/SHA-256”
要求中缺少的 artifact 名称、文件 allowlist、清单字段和校验顺序。它只约束交付链，
并记录已确认的 tarball 二进制布局与 GHCR tag 映射；不允许 release 从旧 run、分支构建或
registry tag 回补输入。

## Artifact 与 allowlist

quality-gate 汇总成功后只发布一个逻辑 artifact：
`leanote-release-inputs-v1`。其文件 allowlist 必须恰好为以下五个根路径文件，不能
包含目录、隐藏文件、日志或其他测试输出：

1. `release-inputs.json`
2. `leanote-vX.Y.Z-linux-amd64.tar.gz`
3. `leanote-vX.Y.Z-linux-amd64.tar.gz.sha256`
4. `build-metadata.json`
5. `image-build-inputs.json`

其中 `X.Y.Z` 必须来自 `package.json` 顶层 `version`，并与当前 tag、清单和元数据
逐字一致。`.sha256` 必须是单行 `<64 位小写 SHA-256><两个空格><tarball 文件名><换行>`。

tarball 内的 Go 二进制路径固定为 `bin/leanote`，文件名为 `leanote`，权限固定为 `0755`；
`sh/package.sh` 和 container entrypoint 都只调用该路径。release tag `vX.Y.Z` 唯一映射为
`ghcr.io/yangphere/leanote:vX.Y.Z`，重复预检、manifest、推送和独立拉取复验均使用该完整
字符串，不生成或回退到 `latest` 或去掉 `v` 的别名。

## `release-inputs.json` 最小 schema

实现可使用等价的本地校验器，但不得放宽以下字段、枚举或 additionalProperties 约束：

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:leanote:release-inputs:v1",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "schema_version", "artifact_name", "workflow", "run", "ref", "commit",
    "version", "source_date_epoch", "platform", "image_digest", "base_image_digest",
    "provenance", "attestation", "sbom", "files", "image_build_inputs_sha256"
  ],
  "properties": {
    "schema_version": { "const": "leanote.release-inputs.v1" },
    "artifact_name": { "const": "leanote-release-inputs-v1" },
    "workflow": { "type": "string", "minLength": 1, "maxLength": 120 },
    "run": {
      "type": "object", "additionalProperties": false,
      "required": ["id", "attempt"],
      "properties": {
        "id": { "type": "string", "pattern": "^[0-9]+$" },
        "attempt": { "type": "integer", "minimum": 1 }
      }
    },
    "ref": { "type": "string", "pattern": "^refs/tags/v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$" },
    "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "version": { "type": "string", "pattern": "^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$" },
    "source_date_epoch": { "type": "integer", "minimum": 0 },
    "platform": { "const": "linux/amd64" },
    "image_digest": { "type": "string", "pattern": "^sha256:[0-9a-f]{64}$" },
    "base_image_digest": { "type": "string", "pattern": "^sha256:[0-9a-f]{64}$" },
    "provenance": { "enum": ["enabled", "disabled"] },
    "attestation": { "enum": ["enabled", "disabled"] },
    "sbom": { "enum": ["enabled", "disabled"] },
    "files": {
      "type": "array", "minItems": 4, "maxItems": 4,
      "items": {
        "type": "object", "additionalProperties": false,
        "required": ["path", "kind", "sha256"],
        "properties": {
          "path": { "type": "string", "minLength": 1, "maxLength": 180 },
          "kind": { "enum": ["tarball", "checksum", "metadata", "image_build_inputs"] },
          "sha256": { "type": "string", "pattern": "^[0-9a-f]{64}$" }
        }
      }
    },
    "image_build_inputs_sha256": { "type": "string", "pattern": "^[0-9a-f]{64}$" }
  }
}
```

`files` 不包含 `release-inputs.json` 自身，必须各有且只有一个 `tarball`、`checksum`、
`metadata` 和 `image_build_inputs` 条目。跨字段校验必须确认路径、kind、版本和 allowlist
一一对应，`image_build_inputs_sha256` 与对应文件一致。`release-inputs.json`、
`build-metadata.json` 和 `image-build-inputs.json` 中的 `commit`、`version`、`source_date_epoch`、
`platform` 以及镜像 digest/构建开关字段必须指向同一 release 输入；tarball 只通过文件名中的版本/platform 与其
哈希校验，`.sha256` 只通过固定行格式绑定 tarball，不能要求这两个二进制/文本文件携带不存在的
结构化字段。

`source_date_epoch` 必须精确等于 tag checkout SHA 的 Git committer timestamp（等价于
`git show -s --format=%ct <tag-checkout-sha>` 的十进制秒数）；只在文件之间保持同一个任意整数
不满足契约。release validator 必须用当前 tag checkout SHA 重新取得该值并比较。

## 被列 JSON 文件最小 schema

为避免 release 对两份 JSON 元数据各自解释，quality-gate 必须同时满足以下固定字段和
`additionalProperties: false` 约束。两份 JSON 中与 manifest 同名的字段必须逐字一致；
`tarball_sha256` 通过 manifest 的 `kind=tarball` 条目绑定，其他字段不得脱离 manifest 或 gate
输出单独解释。实现可以使用等价校验器，但不得省略字段或接受未知字段。

`build-metadata.json`：

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:leanote:build-metadata:v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "version", "commit", "source_date_epoch", "platform", "tarball_sha256", "image_digest"],
  "properties": {
    "schema_version": { "const": "leanote.build-metadata.v1" },
    "version": { "type": "string", "pattern": "^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$" },
    "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "source_date_epoch": { "type": "integer", "minimum": 0 },
    "platform": { "const": "linux/amd64" },
    "tarball_sha256": { "type": "string", "pattern": "^[0-9a-f]{64}$" },
    "image_digest": { "type": "string", "pattern": "^sha256:[0-9a-f]{64}$" }
  }
}
```

`image-build-inputs.json`：

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:leanote:image-build-inputs:v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "version", "commit", "source_date_epoch", "platform", "base_image_digest", "provenance", "attestation", "sbom"],
  "properties": {
    "schema_version": { "const": "leanote.image-build-inputs.v1" },
    "version": { "type": "string", "pattern": "^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$" },
    "commit": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "source_date_epoch": { "type": "integer", "minimum": 0 },
    "platform": { "const": "linux/amd64" },
    "base_image_digest": { "type": "string", "pattern": "^sha256:[0-9a-f]{64}$" },
    "provenance": { "enum": ["enabled", "disabled"] },
    "attestation": { "enum": ["enabled", "disabled"] },
    "sbom": { "enum": ["enabled", "disabled"] }
  }
}
```

`build-metadata.json.tarball_sha256` 必须等于清单中 `kind=tarball` 的哈希；清单中的
`kind=image_build_inputs` 条目哈希必须等于 `image_build_inputs_sha256`。两份 JSON 的共享字段和
镜像 digest/构建开关字段必须等于 manifest 对应字段；GHCR 最终 tag 必须等于已确认的
`ghcr.io/yangphere/leanote:vX.Y.Z` 映射。

## 生产方与消费方校验顺序

1. release 只从当前 tag release workflow 的当前 quality-gate run/attempt 下载该 artifact，
   并确认 artifact 名称唯一；禁止按名称搜索其他 run。
2. 校验文件数量、根路径 allowlist、`release-inputs.json` schema、workflow/run/attempt、
   `refs/tags/vX.Y.Z`、tag checkout SHA、版本、platform、镜像 digest/构建开关和
   `SOURCE_DATE_EPOCH == git show -s --format=%ct <tag-checkout-sha>`。
3. 重新计算四个被列文件的 SHA-256，校验 `.sha256` 的固定格式和 tarball 哈希，再校验
   两份 JSON 的最小 schema 及其 commit/version/platform/source_date_epoch/base-image/开关字段。
4. 任一文件缺失、重复、额外、跨 run/ref、哈希不符、部分上传或清单不一致都必须非零失败，
   且在创建 GitHub Release 或推送 GHCR 前停止。release 不重建缺失 tarball，不从历史 artifact
   或 registry tag 补偿。
5. release 必须仅使用 `image-build-inputs.json` 交接的固定输入重建或载入候选镜像，先确认本地
   digest 等于 manifest/`build-metadata.json.image_digest`，再按预检通过的 GHCR tag 推送；推送后
   读取 registry 返回的 immutable digest 并再次要求完全一致。不一致时必须失败且不得创建 GitHub
   Release。workflow 不自动删除已经推送的错误 tag，后续处理遵守 Q-F3。

artifact 的保留期不超过 7 天。workflow 不覆盖或删除 artifact；失败重试、错误资产删除和
人工恢复遵守 F 的 Q-F3 边界。GHCR 最终镜像 tag 已固定为
`ghcr.io/yangphere/leanote:vX.Y.Z`，该契约不允许实现引入其他别名。
