# Go 工具链审核修复 - 执行计划

## Phase 1: 规划与证据

- [x] 阅读父任务 PRD/design/implement、Travis 与 harness 实现、stock CLI panic 证据。
- [x] 从未修改 `HEAD` 的临时归档清缓存生成 Go 1.26.7/1.27.0 vet 快照，确认两版各 237 条；保留原始排序，排序规范化后的行集合一致。

## Phase 2: 实现

- [x] 修改 `.travis.yml`：模块图 `go build` 产出 PATH 中的 Revel CLI，设置 `GOTOOLCHAIN=local`，审计 x/tools v0.49.0。
- [x] 将完整 vet 快照和摘要写入父任务 research，并修正父任务 `implement.md` 的 36 条基线描述。
- [x] 修正父任务 `implement.md` 的跳过记录，明确 ExportPdf、wkhtmltopdf、HTTP integration 和 record workflow_dispatch pending。

## Phase 3: 验证

- [x] 运行与 Travis 等价的 `go build -o <temp>/revel github.com/revel/cmd/revel`、`go version -m`、`revel version`，确认二进制依赖 x/tools v0.49.0。
- [x] 静态检查 `.travis.yml` 无旧 `go install ...@v1.0.3`、无 `go get -u`，`sh/run.sh`/`sh/package.sh` 仍调用 PATH 的 `revel`。
- [x] 复核两份 baseline 的行数、分类计数，以及排序规范化后的行集合一致性；运行 `git diff --check`。
- [x] 运行针对性文档/文本断言；不在本地运行 record 流程，不生成 ExportPdf golden。

## 完成门禁

- [x] `trellis-check` 复核通过，未改变父任务既有 Go 业务实现或模块版本。
- [x] 最终报告明确 record-export-pdf 仍需后续 workflow_dispatch 取证，不能将父任务标记为已完成。
