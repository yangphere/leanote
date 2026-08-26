# Go 工具链审核修复 - 技术设计

## 边界与不变量

本子任务只修复 CI 工具入口和审计证据。保持 Go 业务实现、模块版本、Golden/USN、MongoDB 5.0 约束与 `sh` 入口命令不变；任何失败必须保留非零退出与原始诊断。

## CI CLI 构建

Travis 的 install 阶段从仓库根目录执行：

1. 将 `$HOME/gopath/bin` 建目录并加入 PATH。
2. 设置 `GOTOOLCHAIN=local`，避免自动下载另一套 Go。
3. 执行 `go build -o "$HOME/gopath/bin/revel" github.com/revel/cmd/revel`，让主模块的 MVS 选择 `x/tools v0.49.0`。
4. 用 `go version -m` + 固定文本匹配审计二进制依赖，然后运行 `revel version`。

`sh/run.sh`/`sh/package.sh` 不需要第二套包装器；它们从同一 PATH 找到该二进制，因此 run/package 使用的实现与审计对象相同。错误不被 `|| true` 或 fallback 吞掉。

## Vet 证据

以 `git archive HEAD` 临时 checkout 隔离当前修复，分别使用 Go 1.26.7 与 1.27.0 清缓存后执行 `go vet ./app/...`。原始输出完整保存；摘要只统计已知类别，不修改输出顺序。两个 Go 版本的诊断顺序可能不同，因此两份原始快照分别保留，并以排序规范化后的行集合比较为零差异。

## 任务记录

将父任务执行计划中的错误数字和“无任何测试跳过”改成当前真实边界：默认 harness 对缺失 ExportPdf golden/wkhtmltopdf 和未显式开启 HTTP integration 的条件跳过；Linux canonical smoke 与默认套件分开记录；record workflow_dispatch 保持 pending。

## 回滚

- Travis 改动可单独回滚，不触碰 Go 模块或生产源码。
- vet 快照为证据文件；若复核发现运行环境不满足版本要求，保留失败输出并回到规划，不用当前工作区输出冒充 HEAD 基线。
- 文档修改可独立回滚，但不能恢复已被证明不完整的 36 行快照或“无跳过”陈述。
