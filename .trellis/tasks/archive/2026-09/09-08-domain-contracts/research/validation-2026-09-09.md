# 领域契约实现验证（2026-09-09）

## 本轮实现

- `app/domain.ObjectID` 保持零值/hex/JSON（含 `null -> zero`）契约；未知 JSON 输入返回 `ErrInvalidObjectID`。
- `app/info` 的持久化模型改用中立 `domain.ObjectID`，生产包和默认测试不再依赖 `app/lea`、Revel 或 Mongo driver。
- `Re` 与 `Theme` 的动态 JSON 字段在 `MarshalJSON` 边界统一调用集中校验；非法值返回带字段路径的可观察错误，nil/空集合 wire shape 不变。
- `app/db` 继续通过迁移期 `lea.CodecRegistry` 适配 `domain.ObjectID` 的 BSON ObjectId，并回归标准 ObjectId、历史空字符串/hex 和未知 BSON 类型失败。

## 可复现命令与结果

| 命令 | 结果 |
|---|---|
| `go test ./app/info -count=1` | 通过 |
| `go test -tags mongo_contract ./app/info -count=1` | 通过 |
| `go test ./app/domain ./app/info ./app/db ./app/lea -count=1` | 通过 |
| `go test ./... -count=1` | 通过 |
| `go vet ./...` | 通过 |
| `pwsh -NoProfile -File research/validate_info_dependencies.ps1` | 通过；`app/info` 生产/测试依赖图无 Revel/Mongo driver |
| `python research/validate_model_catalog.py research/model-catalog.json research/model-catalog.schema.json` | 通过；69 active 类型、29 集合 |
| `python research/validate_input_contracts.py research/input-contracts.json research/input-contracts.schema.json` | 通过；4 个输入结构 |
| `python .trellis/scripts/task.py validate 09-08-domain-contracts` | 通过 |
| `git diff --check` | 通过 |

## 未闭合边界

- notebook/tag 删除的新 USN tombstone/sync（D-01）由 `application-notes` 实现与回放。
- 原子 USN、note-save 事务/补偿和 `partial_write`（D-06）由 `application-notes` 与 `infrastructure-persistence` 实现与回放。
- 真实 Mongo 7/8、HTTP、浏览器、PDF、容器及发布证据仍未运行；本机缺少相应运行环境时保持 `partial`/`unknown`，不以纯单元测试替代。
- 完整 69 类型 JSON full golden 及 driver-dependent BSON 探针的基础设施归属仍按验收矩阵交接，不在领域任务中伪造完成。

## 规格复审补充（本轮仅改任务材料）

- `python research/validate_model_catalog.py research/model-catalog.json research/model-catalog.schema.json`：通过；API 目录仍为 29 个 action，`getSyncTags` 修正为 `[]NoteTag`，冲突项写入 `compatibility_notes`，API DTO、`write_only` 更新载荷和 `EachHistory` 嵌套持久化值的责任已分离（更新载荷完整 BSON fixture 仍为 unknown）。
- `python research/validate_input_contracts.py research/input-contracts.json research/input-contracts.schema.json`：通过；输入结构由 4 个扩展为 7 个，新增 3 个 member-blog 部分更新载荷。
- `python .trellis/scripts/task.py validate 09-08-domain-contracts`：通过；上下文清单包含本轮复审材料。
- `git diff --check`：通过；本轮未修改 `app/`、`cmd/`、`conf/`、`sh/` 或生成资源。

## 差异复核修复（本轮仅改任务材料）

- `ApiTag.GetSyncTags` 的旧文档 `[type.Tag]` 与实际 `[]NoteTag` 对象数组差异已写入 `compatibility_notes`；模型目录校验器现对 9 个已知冲突 action 强制要求非空备注。
- `NoteOrContent` 消费者已移除无证据的 `app/service`；GET/POST 备注改为准确引用 `conf/routes` 的通配方法。
- `python research/validate_model_catalog.py research/model-catalog.json research/model-catalog.schema.json`：通过；29 个 API action 和兼容备注约束均通过。
