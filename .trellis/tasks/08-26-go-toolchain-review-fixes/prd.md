# 修复 Go 工具链审核发现的问题

## Goal

修复 2026-08-26 对 `08-25-go-toolchain` 未提交改动的复核发现，使 CI 入口、vet 前置证据和验收记录都与实际行为一致；不改变 Leanote 业务/API、Golden/USN、MongoDB Schema 或既有 Go 实现语义。

## Background / confirmed facts

- `.travis.yml:11` 当前使用 `go install github.com/revel/cmd/revel@v1.0.3`。该隔离安装路径把 Revel CLI 的 `golang.org/x/tools` 固定在 2020 pseudo-version；在 Go 1.26.7 执行 `revel run`/`revel package` 会触发 `go/types.(*StdSizes).Sizeof` nil-pointer panic。`sh/run.sh:8` 和 `sh/package.sh:5` 都从 PATH 调用该 CLI。
- 主模块 `go.mod` 已选择 `golang.org/x/tools v0.49.0`。在主模块依赖图中构建 `github.com/revel/cmd/revel` 可得到可运行 CLI；`go version -m` 可审计其实际依赖。
- 从未修改的 `HEAD` 归档并在两版 Go 清缓存后生成的 vet 输出各 237 条。原始诊断顺序随 Go 版本不同；排序规范化后的行集合一致：205 tag、21 unkeyed、6 unreachable、3 self-assignment、1 printf、1 signal。原 research 快照各 36 条，缺少 205 条 tag 发现。
- 默认 harness 当前会跳过 `TestGoldenExportPdf`（reviewed golden 或 wkhtmltopdf 缺失）以及 `TestServerServesLoginOverRealHTTP`（未设置 `LEANOTE_HTTP_INTEGRATION=1`）；受控 Linux canonical smoke 是单独证据，不等于默认 harness 零跳过。`record-export-pdf` workflow_dispatch 真实取证仍 pending。

## Requirements

### R1 — Travis 使用主模块依赖图的 CLI

- 将 `.travis.yml` 的 Revel CLI 安装改为显式 `go build` 到已加入 PATH 的目录，使用当前主模块的 `go.mod` 选择 `github.com/revel/cmd v1.0.3` 与 `golang.org/x/tools v0.49.0`。
- 固定 `GOTOOLCHAIN=local`，并用 `go version -m` 检查产物包含 `golang.org/x/tools v0.49.0`；任何构建或检查失败都返回非零。
- `sh/run.sh` 与 `sh/package.sh` 继续调用同一 PATH 二进制；不得保留 `go install ...@v1.0.3` 的隔离依赖路径。

### R2 — 完整 vet 基线证据

- 用未修改 `HEAD`、`go clean -cache` 和 Go 1.26.7/1.27.0 重新生成两份快照，保留完整 237 条输出及逐版本一致性证据。
- 更新 `08-25-go-toolchain/implement.md` 中把 36 条标为完整基线的文字；不得改写为“当前实现仍有 237 条”。

### R3 — 如实记录测试跳过

- 更新 `08-25-go-toolchain/implement.md`，明确记录 ExportPdf golden、wkhtmltopdf 和 HTTP integration smoke 的跳过条件，以及受控 Linux smoke 与默认 harness 的边界。
- 保留 `record-export-pdf` workflow_dispatch pending 状态；没有真实 artifact 前不得声称 ExportPdf 已验收或任务“无任何测试跳过”。

## Acceptance criteria

- [ ] **AC1** `.travis.yml` 不再包含 `go install github.com/revel/cmd/revel@v1.0.3`；安装命令从主模块构建 CLI，固定 `GOTOOLCHAIN=local`，并审计到 x/tools v0.49.0。
- [ ] **AC2** 两份 vet baseline 均为 237 行、分类合计 237；保留版本各自的原始诊断顺序，排序规范化后的行集合一致，且证据文件和任务文档相互一致。
- [ ] **AC3** 任务记录准确说明 ExportPdf 与 HTTP smoke 的跳过条件及 record workflow pending，不再声称无任何测试跳过。
- [ ] **AC4** 运行 YAML/文本静态检查、模块 CLI 构建验证、`git diff --check`；不引入业务源代码变化、依赖版本漂移或隐藏 fallback。

## Out of scope

- 不在本任务直接运行 record workflow_dispatch，不在工作区录制或刷新 ExportPdf golden。
- 不升级 Revel/Mongo 依赖，不删除 Travis，不修改 `sh/run.sh`/`sh/package.sh` 的业务命令，不重做 Go vet 实现修复。

## Risks / deferred

- Travis 仍是遗留非阻断入口；最终删除/替换由 CI/CD 任务负责。
- `record-export-pdf` 需要可推送分支并由 GitHub Actions 生成 artifact；本任务只修正记录，不伪造该外部证据。

## Open questions

无。用户已批准创建任务并要求修复全部三条审核发现。
