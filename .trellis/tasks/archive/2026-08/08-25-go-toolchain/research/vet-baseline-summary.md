# Go vet 完整基线复核

日期：2026-08-26

## 复现边界

- 输入：未修改的 `HEAD`（当前提交 `a59e6ea`），通过 `git archive HEAD` 解包到临时 checkout；不使用工作区已修复源码。
- 每个版本执行 `go clean -cache` 后运行 `GOTOOLCHAIN=local go vet ./app/...`。
- 输出文件：`vet-baseline-go1.26.7.txt`、`vet-baseline-go1.27.0.txt`。

## 结果

两版输出均为 237 行、exit 1。Go 版本改变了原始诊断顺序；保留两份原始输出后，对两者逐行排序规范化，比较结果为 0 行差异：

| 类别 | 数量 |
|---|---:|
| invalid struct tag | 205 |
| unkeyed literal | 21 |
| unreachable | 6 |
| self-assignment | 3 |
| printf misuse | 1 |
| signal channel | 1 |
| 合计 | 237 |

这两份快照是完整前置基线；此前 36 行文件不能代表清缓存后的完整发现集。
