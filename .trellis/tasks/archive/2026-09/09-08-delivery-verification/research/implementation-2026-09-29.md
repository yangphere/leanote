# 2026-09-29 实现进度与阻断

## 当前边界

最新决定：2026-09-29 用户要求“需要真实环境验证的功能都忽略掉. 直接提交并归档”。已完成的仓库工具及本地验证作为本次交付；下表剩余实现和真实环境场景全部从本次关闭范围排除，保留未完成/unrun/blocked 事实。用户授权本地提交、归档和日志；以下早先“没有 commit、archive”是收尾前状态，不再是授权限制。归档不代表完整场景执行器已部署或综合验收通过，发布仍须满足工具现有门禁及具体授权。

2026-09-29 用户进一步确认：“尚未部署，先完成仓库内工具并明确保留真实验收阻断”。本轮按此范围完成门禁/校验/发布恢复工具收尾；不把未部署的 runner 或未完成场景 adapters 记为通过，也不运行真实发布。

用户“批准激活进入实现”已生效。本叶原为 in_progress，未创建任务或重复激活。工作基于 dirty dev / e1dfd907；本地测试属于 working-tree 工具证据，不是正式 candidate 或发布实证。没有 commit、archive、push 或真实 GitHub/GHCR 写入。

已落实：质量 summary 唯一 job 集合与可信当前身份/退出码；独立交付目录与受保护执行协议；扩展门禁进入 publish 依赖；普通发布/恢复共用远端校验、具体授权、同 tag 锁；指定原 run/attempt/artifact ID、原版本/epoch/目录、三公开资产 hash 读回；响应丢失只读对账；脱敏阶段/失败身份保留。操作合同见 docs/modernization/cicd-delivery.md。

目录当前 203 项（包括 38 notes action 和 14 项 frontend 手工清单），不是 203 项已实现或已运行。解析矩阵只能冻结验收范围，不能证明条款内部的边界断言都已由 runner 覆盖。

## 未完成的实现与真实条件

| 缺口 | owner | 状态与恢复条件 |
| --- | --- | --- |
| 完整受保护 scenario executable、逐项命令/断言/环境/fixture 映射 | delivery | 未提供；须复用真实上游 harness 编写和审查 adapters，不得以手工 passed/echo marker 替代 |
| 38 action 新候选 HTTP/DB/owner/USN/history/receipt replay | delivery + notes/interface 合同 | 已索引，历史缺 17 Web；本轮未补齐这 17 项真实 replay，更未产生 38 项实证 |
| Mongo 7 standalone、8 standalone/replica-set、kill/restart 五阶段、failpoint、跨主机/卷故障 | delivery + persistence/content 合同 | 缺专属受控环境及场景执行映射；Docker 可响应不代表条件满足 |
| SMTP handoff、backup/restore、upgrade restart | delivery + admin/publishing 合同 | 真实服务未提供、未运行；局部 fake 测试不是实证 |
| Linux 非 root/volumes、production-config、完整 PDF corpus/出站观测/cleanup | delivery + interface/content 合同 | 本机 Windows，未执行受控 Linux 场景；package/container smoke 不替代完整合同 |
| 四产品八槽和 frontend/publishing 手工业务清单 | protected runner owner + delivery | 未提供 runner 配置，不启动用户 GUI；操作和预期见原手工清单 |
| 干净候选、tag、受保护 GitHub environment、具体发布审批、原 artifact ID | release operator | 未冻结/提供；无真实远端发布授权，恢复 workflow 仅本地合同测试 |

可复用入口：app/tests/harness/cmd/env 的 up/down、LEANOTE_REQUIRE_MONGO=1 外部 fixture 模式、LEANOTE_GOLDEN=replay 与 go test -p 1 ./app/tests/... -count=1 -timeout 30m、native cmd/leanote -runMode test、app/tests/harness/cmd/e2e supervisor、现有 package/container smoke。它们不能直接代表全部目录项；固定 leanote-test-mongo 与 27017/28017 必须串行。app/tests/README.md 的 Mongo 5.0 句子与当前实现/质量规格的 pinned Mongo 8.0 不一致，以实现为准；本次不扩大修改旧 owner 文档。

## API 事实与验证边界

本轮只读获取 GitHub 官方 github/rest-api-description 的 api.github.com.json：job.run_attempt 是可选字段，attempt-specific URL 为权威；workflow-run.path 可含仓库前缀和 @ref；artifact.workflow_run 不提供 attempt。代码额外核验 workflow ID/path/state，并从原 artifact 内容验证 attempt；不允许缺失字段触发任意降级。未进行真实账户/registry 请求或写入验证。

GHCR /v2/ 可读不证明目标 package 权限。对 MANIFEST_UNKNOWN 增加 exact-package tags/list 成功及身份检查；NAME_UNKNOWN 不判空。首次包 provisioning 留给明确授权的管理员。

## 检查限制

原生 implement/check 代理遭 encrypted agent_message 接口错误，项目 channel 实现 worker 也未成功，不计代理实现或独立 check 通过。可用 context_explorer 只读定位真实 harness 缺口，随后安排关键发布边界交叉检查；主流程负责最终 diff 和测试复核。

jbcontext search 报无索引，未建索引，转已知路径定向读取。未升级 Trellis。测试结果追加到 acceptance/evidence-matrix.md；真实 E-D01～13 保持 unrun/blocked。

## 交叉检查裁定

- 接受 runner 与真实资源 cleanup 尚未实现/核实的发现，列为交付阻断；增加另一份自报成功 receipt 不能解决可信执行缺失。
- Mongo 拓扑由同一个受保护 delivery artifact 汇总是当前设计，不能仅因没有独立 job 判为错误。完整映射及真实执行尚缺，不能把现有聚合门禁宣称完成。
- GitHub 默认 GITHUB_* 变量是执行器上下文；源身份已使用独立 SOURCE_* 和 RECOVERY_COMMIT。不得通过覆盖 GITHUB_* 冒充源 attempt；缺失时正确阻断，不引入 fallback。
- 代码层互斥依赖工作流同 tag 锁，create 冲突/响应丢失走只读对账，已有受控 seam 测试；无真实 GitHub 并发实证。
- 原整体 run failed 但全部必需前置成功是批准的恢复情况，不额外要求整体 success。普通流程已有 image 时拒绝并移交显式 recovery，也是批准行为。
- 主流程追加修复 checksum 换行规范化掩盖原始字节篡改：回归首次在远端写后才发现，现于 release-inputs 阶段拒绝，零写入。
- 本机配置探测：LEANOTE_DELIVERY_RUNNER_CONFIG 未设置，BROWSER_SMOKE_COMMAND_* 共 0 项。只能证明当前环境未提供，不能断言远端 runner 不存在。
