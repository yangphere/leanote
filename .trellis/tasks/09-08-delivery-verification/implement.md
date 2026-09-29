# 交付层：集成验证与版本发布 — 执行计划

## Phase 0 — 本轮规格审核

> 2026-09-29 最新收尾决定：用户要求忽略需要真实环境验证的功能，直接提交并归档。下面未勾选的真实场景/场景执行器/发布步骤保留未完成事实，并从本次关闭范围排除；不将其批量勾选为完成。仓库工具本地检查已完成，证据见 acceptance/evidence-matrix.md。本次按本地工作提交 → 归档 → 日志收尾，不 push。branch/base_branch 同为 dev、无 PR，归档采用仓库支持的 --skip-branch-validation，仅跳过不适用的 PR 分支校验。

- [x] 核对父轨道、9 个 archived completed 依赖及 children=[]；报告选中叶后再激活，不创建新任务。
- [x] 修复上下文 7 个归档失效路径，validate 后激活既有任务；激活不授权本轮业务编码。
- [x] 读取九轨证据、现有 CI/browser/release 与生产交付合同，补齐目标、输入输出、边界/异常/兼容/责任。
- [x] 建立 13 行交付矩阵和 38-action 索引，所有本轮产品验证仍 unrun。
- [x] 2026-09-29 用户确认 Q-DV1：核验后受控补建缺失 Release；已同步 PRD/design/恢复验收矩阵。
- [x] 规格引用、38-action 索引、AC/证据覆盖与仅本叶改动范围校验通过；结果见 acceptance 矩阵。
- [x] 2026-09-29 用户明确“批准激活进入实现”；当前任务已为 in_progress，承接最新审核规格进入 Phase 1，不重复创建/激活。

## 当前实现授权与分工（2026-09-29）

- 本节覆盖本文及 PRD/design 中此前审核阶段的“本轮不实现/不运行产品验证”限制；历史审核记录保留。实现授权不包括具体远端发布、提交、归档或 push。
- 主流程负责规格收敛、任务证据、跨模块集成和最终验证；质量实现代理负责 CI summary、扩展验收门禁/harness 及相关 workflow/tests；发布实现代理负责普通发布/显式 recovery、原制品/候选核验、远端分类/读回及相关 workflow/tests。
- 先做有确定性 seam 的失败用例，再实现现有边界并验证；保留用户已有规划改动，不建立第二业务规则来源。真实环境可用性单独核验，缺失项保持 blocked/unrun，不能用 contract tests 代替。

## Phase 1 — 后续候选与工具门禁

- [x] 先加载本任务上下文与上游明细；用现代码核对历史矩阵，不要求历史 partial 先自行变成 passed 才开始补齐。
- [ ] 冻结干净 candidate/version、允许使用的隔离资源、工具链与浏览器清单；缺资源仅阻断对应工作，不伪造运行。
- [ ] 范围内补齐交付 harness/CI/validator；业务/provider 缺陷记录并回原 owner，不能在测试侧建立第二业务逻辑。
- [x] 把 R2～R8 扩展门禁接入最终 publish 依赖链；workflow/contract 接线已完成，但受保护完整 scenario runner 尚未落实，发布继续阻断，不能代替实证。
- [ ] 按 design §3 和 `acceptance/gate-integrity.md` 收敛现有 summary producer/validator 的必需集合、workflow 依赖和当前执行身份；逐上游条款建立场景映射，定义 schema 状态转换及缺失/取消/清理失败拒绝规则。
- [x] 按已确认 Q-DV1 同步 scripts/tests/workflow：显式 recovery、原制品与执行关联、同 tag 并发锁、只补缺失 Release；普通 duplicate/final provenance guard 不变。本地合同通过，真实 GitHub/GHCR 未运行。
- [ ] 按 design §5 实现并核对锁名：普通 tag 发布与 recovery 均解析为同仓库 `release-refs/tags/<tag>`，`cancel-in-progress: false`；从不同分支 dispatch 或独立 workflow 恢复也必须一致，锁覆盖重查/create/最终读回，并按 RR-09 验收。

## Phase 2 — 跨层验收

- [ ] 依序运行 Go/JS/build/Golden/USN 与干净候选资源零漂移，Node build 与复制工作树的测试串行。
- [ ] 按 `acceptance/notes-action-replay.md` 的 38 个 ID 填完整请求/响应/owner/USN/receipt 映射；补缺 17 Web，并复验原 21 项。
- [ ] 跑 Mongo 7 standalone、8 standalone、8 replica-set 的 notes/identity/persistence 场景；claim/apply/verify/save/commit kill/restart、Mongo failpoint、跨主机/persistent-volume 文件故障。
- [ ] 单列 legacy image/attachment upload、CopyHttpImage 的 partial_write/owner-row Verify/quarantine/late-row restore，服务端修复不冒充客户端 response-loss 幂等；D-H7/MOD-004 不增参数且不新增阻断范围。真实 HTTP 分别核验 D-H6 的 GetImages/GetAlbums 500+原 body 与 Attach.GetAttachs 的 legacy 200 error。
- [ ] 汇合 publishing/admin 的权限/期限/Host/JSONP、主题 ZIP/preview、comment submission receipt/压缩、取消与 SMTP handoff_unknown、真实 backup/restore、升级续跑和反馈/broadcast。
- [ ] 验证 native routes/session/config/未就绪/退出；完成 presentation 未编辑零写入、mutation 未知重试/冲突、move 重读及 copy/delete 队列出口，关联真实 HTTP+DB。
- [ ] Linux/container 执行 production-config 负例、非 root、paired roots/backup volume、旧卷复制与重启，以及完整 PDF 正向和恶意 corpus/zero-outbound/cleanup。
- [ ] 受保护真实八槽四 coverage；另执行上游手工清单，记录实际版本/命令来源；发布 artifact 中禁止原始日志/trace/截图。

### 命令与证据要求

以下是最终候选命令要求。本轮已在工作树运行 Node tests/build、Go build/vet 及工作流静态检查；Golden/USN/真实服务未运行，不在共享本机盲跑：

```text
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go vet ./...
go mod verify
go test <target packages> -count=1 -timeout 60s
LEANOTE_GOLDEN=replay LEANOTE_HTTP_INTEGRATION=1 go test -p 1 ./app/tests/... -count=1 -timeout 30m
npm ci
npm run build
npm test
npm run test:e2e:build -- --list
python ./.trellis/scripts/task.py validate .trellis/tasks/09-08-delivery-verification
git diff --check
```

环境赋值示例为 POSIX 形式；Windows 用 PowerShell `$env:`。`--list` 仅 discovery；不得填 executed。真实浏览器按现有受保护命令执行，Mongo fixture/固定端口串行且完成 cleanup。每个扩展场景须记录实际命令/手工步骤，不能以笼统“全量通过”填表。

## Phase 3 — 发布演练、授权与收尾

- [ ] 验证 strict tag/version、SHA/tag peel、包与 image inputs/digest、同 run/attempt，以及缺文件/篡改/跨 attempt/duplicate/unknown/cleanup 失败矩阵。
- [ ] 独立 browser precheck 仅候选证据；final 必须在 release workflow 内重新生成，部分 job 重跑不得借用旧 attempt artifact。
- [ ] 执行 `acceptance/release-recovery.md` 全部恢复正负例，包括原门禁失败、过期/错来源、冲突/未知、并发/响应丢失、重复 no-op；演练使用受控 seam，不写真实 GitHub/GHCR，也不冒充实际发布实证。
- [ ] 执行 GI-01～GI-09：过期但内部一致的 summary、缺门禁/断接线、查询文本误判空、跨分支原候选版本、指定 source attempt/来源核验、普通发布资产读回及具体授权边界；保持普通与恢复共用校验语义。
- [ ] 全部必需矩阵通过后呈现 candidate/tag/制品/digest/风险供发布授权；当前任务审核不构成该授权。
- [ ] 获授权后发布并读回 GHCR digest、Release tag/资产/校验和；未知/部分成功按已确认策略处理，不自动删除或覆盖。
- [ ] 全范围复核、记录最终证据和缺陷 owner；真实证据未齐不可归档为综合验收完成。commit/archive/journal/push 均按各自授权处理。

## 回退与停止条件

当前工具实现和未完成项详见 research/implementation-2026-09-29.md。两个原生实现代理、独立 check 及 channel workers 没有成功运行，实际代码由主流程完成；只读探索代理仅提供 harness 调查和边界交叉检查，不计完整独立检查通过。完整 runner/17 Web action replay 等实现缺口继续保留，任务不关闭。

- 规格未决、provider 缺陷、资源不可用分别阻断相关阶段；记录恢复条件，不清除失败证据。
- 失败测试和 Golden mismatch 先定位 owner，不通过修改预期隐藏问题。
- 已远端发布的对象不能靠本地回退提交撤销；先只读对账，任何删除/覆盖须单独决策授权。
- `task.py start` 已记录 branch/base_branch=dev 的后续 PR/archive 警告；本轮不据此自行切分支或绕过归档检查。
