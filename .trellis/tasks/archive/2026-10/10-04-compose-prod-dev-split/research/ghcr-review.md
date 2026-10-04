# GHCR 审查核实

日期：2026-10-04。官方来源：

https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency

通过官方 article body API 读取同一页面：

`https://docs.github.com/api/article/body?pathname=/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency`

## 已确认并发规则

官方原文：

> `single` (default): At most one job or workflow run can be `pending` in the concurrency group. When a new job or workflow run is queued, any existing `pending` job or workflow run in the same group is canceled and replaced.

> `max`: Up to 100 jobs or workflow runs can be `pending` in the concurrency group. When the queue is full, any additional jobs or workflow runs are canceled.

> The combination of `queue: max` and `cancel-in-progress: true` is not allowed and will result in a workflow validation error.

因此保留共享 group 与 cancel-in-progress:false，增加 queue:max 直接修复默认 pending 替换取消；不能宣称无限队列，或 FIFO 即按版本顺序。本任务没有在 GitHub 实际制造并发运行，运行侧证据为 unrun。

## 实际代码核实

- `.github/workflows/docker-image.yml` 两条 latest copy 路径没有版本比较；tag 只校验严格格式/候选版本/main 祖先，旧提交可产生旧版晋升。
- 全工作流 `docker-image-latest` group 当前无 queue，因此使用官方 default single。
- fresh/recover publish step 固定 `ALLOW_INITIAL_PACKAGE_CREATE:'true'`；底层 `checkGhcrTagAbsent` 默认 false 已有准确拒绝路径，无需新增第二规则。
- `scripts/version.mjs` 已是严格 X.Y.Z 格式单一来源；晋升比较复用它，不另加正则或数字范围限制。
- 已发布的 2.0.1/latest 与历史真实发布证据保持，不进行 push、tag 或 dispatch。

## 历史工作流适用边界

官方来源：

- https://docs.github.com/en/actions/concepts/workflows-and-actions/workflows
- https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#push

通过对应官方 article body API 读取，确认原文：

> Each workflow run will use the version of the workflow that is present in the associated commit SHA or Git ref of the event.

> Runs your workflow when you push a commit or tag, or when you create a repository from a template. This includes workflows that are not merged into the default branch.

因此 tag push 使用 tagged commit 中的工作流；合入 main 不会追溯加固历史工作流。本次 latest guard、queue:max 和禁建包规则只保护包含修复的工作流，以及从已修复 main 执行的恢复 dispatch。禁止向不含修复的旧提交补推版本 tag。强制约束历史发布须另行设计远端 tag 规则或可信固定入口，本轮不修改远端规则、旧 tag 或 manifest。
