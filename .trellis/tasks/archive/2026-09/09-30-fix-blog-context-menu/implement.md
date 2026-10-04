# 执行计划

1. 在 `app/controllers/httpserver_notes.go` 统一让 `DeleteNote`、`MoveNote`、`CopyNote`、`CopySharedNote` 和 `SetNote2Blog` 使用 `noteParameterStrings(c.Params, "noteIds")`；发布在无列表时兼容单数 `noteId`，逐项调用 `ToBlog` 并按全成功汇总。
2. 在 `app/controllers/httpserver_notes_test.go` 增加 `noteIds[]`、重复 `noteIds`、索引形式和单数回退的绑定回归；通过共享的 `noteParameterStrings` seam 固定删除/移动/复制/复制共享使用的列表解析，并覆盖发布空列表、数组优先、单数回退、全成功、部分失败且继续处理剩余目标的汇总语义，避免测试依赖 Mongo。
3. 检查 `public/js/app/note.js` 的分享、PDF、删除、移动、复制和复制共享动作及对应路由/失败处理；更新 `public/js/app.min.js` 时仅通过 `npm run build` 生成。
4. 运行 `gofmt`、聚焦 Go 测试、`npm test`、必要的前端静态契约测试、`npm run build`、`git diff --check` 与 `task.py validate`。
5. 记录 Mongo、真实 HTTP、浏览器和 PDF 工具是否运行；未运行则在验收证据中保留 `unrun`。

## 风险与回滚点

- 列表参数可能来自 `noteIds[]`、重复 `noteIds` 或索引字段；复用现有 `noteParameterStrings`，不要新增解析器。
- 批量逐项操作可能部分成功；控制器继续处理剩余目标，响应按全成功规则报告，保留服务层 receipt 供后续对账，不尝试控制器回滚；前端收到 false 时暂不刷新已成功项目的样式，属于已知兼容限制。
- 生成产物改动范围应限制为 manifest 声明的 `public/js/app.min.js`（及构建产生的同步输出）；发现额外生成物时先停止并核对。

## 实现与验证记录（2026-09-30）

- 已实现：`DeleteNote`、`MoveNote`、`CopyNote`、`CopySharedNote` 统一复用
  `noteParameterStrings`；`SetNote2Blog` 支持列表参数和单数 `noteId` 回退，空目标返回
  `false`，逐项调用 `NoteService.ToBlog`，部分失败继续处理且只有全成功才返回 `true`。
- 已新增回归测试：重复参数、`noteIds[]`、索引参数，发布目标的数组优先/单数回退/缺失目标，
  以及空目标、全成功、部分失败和失败后继续调用。删除、移动、复制和复制共享共用的
  `noteParameterStrings` 解析 seam 已由解析 helper 测试固定；未对具体 service 分派做 Mongo 集成测试。
- 已核查 `public/js/app/note.js` 与 `public/js/app.min.js` 的分享、PDF、删除、移动、复制和
  复制共享路由/失败处理；源码契约未要求变更，因此构建未产生前端 diff。
- 本次审核修复后已通过：`gofmt`、`GOTOOLCHAIN=local go test ./app/controllers/... -count=1`、
  `GOTOOLCHAIN=local go vet ./app/controllers/...`、`git diff --check` 和
  `task.py validate`。任务此前记录的 `npm run build`、`npm test`（209 passed，1 skipped）
  仍保留；本次未重跑 npm test。
- 未运行：Mongo、真实 HTTP、浏览器和 PDF 工具验证；这些证据保持 `unrun`，不由本地测试替代。
