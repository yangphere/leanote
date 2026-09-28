# Presentation/frontend acceptance matrix

更新：2026-09-28。`local-pass` 只表示当前未提交工作树的自动验证；`partial` 表示本地证据具备、真实验收未齐；`unrun`/`delegated-unrun` 均不能当通过。Q-PF-01/Q-PF-02 已确认，无待决产品问题。

| AC | 已有证据与通过条件 | 剩余证据 / owner | 当前状态 |
| --- | --- | --- | --- |
| AC-PF1 构建 | 当前工作树 npm ci/build/Node；164 个 manifest 输出全部跟踪，连续两次 SHA256 零漂移；构建负向/回滚测试 | 已提交候选干净检出及 CI 零 diff / presentation | partial |
| AC-PF2 资源 | Node build/resource contract 核对锁定库、7 locale、四插件、模板 URL 和生成闭包 | 实际 HTTP 资源及独立 iframe 运行 / presentation | partial |
| AC-PF3 业务页面 | Playwright discovery；无服务或浏览器执行 | 真实隔离 leanote_test、页面/上传/资源/console/清理 / presentation + delivery | unrun |
| AC-PF4 编辑器 | 可执行 Node 覆盖 metadata-only、零变更、dirty/revision、迟到响应、只读、草稿恢复、queue 及加载竞态 | HTML 语义、真实 HTTP/DB 零写入、浏览器交互 / presentation | partial |
| AC-PF5 Web identity | Node 覆盖冻结身份/body、顺序、未知重试、精确 Usn、严格 boolean、copy、冲突、旧选择；Go mapper 序列化；浏览器用例补协议断言 | HTTP+DB receipt/故障回放、实际页面请求、部分成功核对 / delivery | partial |
| AC-PF5 旧客户端 | mapper 单测保持原六字段和失败形状，新建分支不改 | 缺字段旧客户端真实 HTTP Golden / notes + interface + delivery | delegated-unrun |
| AC-PF6 决策 | 用户确认：存量精确 committed Usn、新建不扩展 receipt、仅页面内存恢复；PRD/design/spec/tests 同步 | 无待决项 | passed |
| AC-PF7 发布矩阵 | 明确委托与未运行边界 | Chrome/Edge/Firefox/Safari current/previous 八槽、Mongo 7 standalone / 8 replica set、kill/restart/failpoint / delivery | delegated-unrun |

## Manual operation checklist (unchecked)

下列是将来在隔离测试环境执行的操作与预期；本轮没有启动服务或浏览器，也没有把这些勾选为通过。

- [ ] 打开已有笔记，不编辑直接切换/关闭；预期没有 `/note/updateNoteOrContent` 请求，持久化内容、history、USN 不变。
- [ ] 只改标题或标签并保存；预期请求带新的 `OperationId`、权威 `ExpectedUsn`，不带 `Content`，成功后 cache 与服务端 revision 一致。
- [ ] 编辑富文本、Markdown 和第一方插件内容，保存后撤销/重做；预期 dirty/revision 准确，HTML 只发生非语义规范化，上传与 iframe 资源无错误。
- [ ] 在保存请求未完成时继续编辑、切换笔记或触发第二次保存；预期旧响应不清除新内容，不用新意图越过 pending/unknown 的同 note 保存。
- [ ] 把存量 update 的结果设为未知后重试同一意图；预期 ID、ExpectedUsn、字段 presence 和内容完全相同，receipt 只产生一次提交及 USN/history。
- [ ] 制造 stale USN、权限失败、validation 和网络中断；预期不显示“保存成功”，用户改动仍在，冲突需显式解决。
- [ ] 对 copy/shared-copy/delete/move 各执行单项和多项操作并模拟未知结果；预期有序输入和同一 ID 冻结，重试不重复 destination/USN，失败不永久移除列表或伪造计数。
- [ ] 对 delete/move 分别注入原始 `false` 和带 `Ok:false` 的 `info.Re`（包括 `Msg:"storage"`）返回；预期只有原始 `true` 允许成功清理，其余均显示失败并保留列表、选择和计数。
- [ ] 在“全部笔记”移动当前笔记，同时编辑并触发自动保存；预期主动读取权威 Usn，草稿不被重置，后续保存使用新 notebook 和返回版本，缺版本不提示随机数错误。
- [ ] 模拟 move 成功后读取失败，再按 Ctrl+S（分别有新编辑/无新编辑）；预期重试读取而非重发 move，安全读取成功后解除限制，远端内容冲突仍保留草稿并要求核对。
- [ ] 在 copy/shared-copy/delete 进行中捕获编辑；预期 copy 成功后源笔记继续保存，delete 成功后对应队列移除，不因已删除笔记残留队列而持续触发离开提示；不影响其他笔记草稿。
- [ ] 重新加载带待确认操作的页面；预期离开前提示，重载后核对服务端；不恢复正文/请求体、不自动重放，也不用新 ID 盲重试。
- [ ] 登录、笔记、modal/tab/dropdown、相册/附件上传、admin/member/blog、`leaui_image` iframe；预期主站资源无 4xx/5xx、无应用脚本错误，fixture 清理成功。
- [ ] 在真实 Safari 当前和前一主版本与其他发布槽位执行固定四项 browser smoke；预期每槽 artifact 独立标明产品/完整版本、候选提交及通过/失败/跳过，不以 Chromium 代替。

## Evidence rule

每次执行另记日期、候选 commit、命令、环境/浏览器完整版本、discovered/executed/passed/failed/skipped、失败或清理原因以及脱敏 artifact 路径。未运行保持 `unrun`；环境不可用保持 `blocked`/`delegated-unrun`。本矩阵不含 token、cookie、正文、原始请求体、trace、截图或视频。

## 2026-09-28 implementation evidence (before review fixes)

- 候选基线 HEAD：`5090f340`，branch=dev，未提交工作树；不宣称候选 SHA 已包含这些改动。
- Node v24.21.0、npm 11.19.0；Go go1.27.1 windows/amd64，GOTOOLCHAIN=local。
- npm ci 已在当前工作树成功；最终通过现有 runBuild 连续两次标准构建，164 输出 `drift=[]`、`untracked=[]`。
- app.min.js SHA256：`1df4dbdec56bf265b6f70d89344ef8e6fcc1589df8e66bbf630fdbe018c304f8`。
- 最新保存行为回归 12/12；Go controllers/... 和 httpserver -count=1 -timeout 60s、go build ./...、相关 go vet 通过。
- 最终 npm test：162 项、161 通过、1 项 Windows 条件跳过、0 失败（113.3 秒）。Playwright discovery：build-smoke 1 项、business 22 项；executed=0。
- task validate 通过：implement.jsonl/check.jsonl 各 9 entries；git diff --check 通过，修改的三个 Go 文件 gofmt -l 输出为空。
- 未启动服务、Mongo 或浏览器。真实 HTTP、DB receipt、跨进程、failpoint、Safari 八槽及干净提交候选/CI 均未运行。

## 2026-09-28 review-fix evidence

- 候选仍为 HEAD `5090f340` 上的未提交工作树；本轮没有 commit、archive、journal 或 push，也未重新运行 npm ci。环境为 Node v24.21.0 / Windows。
- 四项审核问题已落实：move 主动读取权威 Usn 并安全恢复队列；copy/shared-copy 释放源队列；delete 清理已删除队列；旧请求签名和 R-PF5 待定文字更新为确认契约。
- 回归先失败后修复：首批 19 项中 7 失败、12 通过，覆盖缺版本文案、move、copy/shared-copy 和 delete 队列出口；另新增“无新编辑时重试读取”用例，先复现 1 失败再修复。不得通过手动回填 cache.Usn 模拟恢复。
- 最终针对性命令 `node --test tests/js/note-save-contract.test.js tests/js/mutation-intents-contract.test.js tests/js/note-mutation-failure-contract.test.js`：45 项通过、0 失败、0 跳过。包含真实生产处理函数及生成 bundle 的 delete/move 行为。
- 标准 `npm run build` 后再次 `runBuild()` 比较所有 `BUILD_OUTPUTS`：164 输出，`drift=[]`、`untracked=[]`。最终 app.min.js SHA256：`8d353e96180c728466ffefd93fd4ee09dfae42e1798406a1c5a8233e8d1b0ec1`。
- `GOTOOLCHAIN=local go test ./app/info -run TestJSONContractFrozen -count=1 -timeout 60s` 通过；确认已有扁平读取 JSON 契约。本轮未改 Go，未重跑历史 controllers/build/vet 门禁。
- 最终串行 `npm test`：177 项、176 通过、1 项 Windows 条件跳过、0 失败（91.1 秒），exit 0。此前与构建并发的一次运行出现夹具缺失输入失败：测试 `copyBuildTree()` 复制实时 public 树时与构建发布重叠，提前触发缺文件错误，未到达 symlink 断言。未改动或放宽该测试；构建完成后串行全量复跑通过，包括该 symlink 用例。后续重建和复制工作树夹具的测试须串行执行。
- `npm run test:e2e:build -- --list` 发现 1 项、执行 0 项；`task.py validate` 两份清单各 9 entries，`git diff --check` 通过。
- implement/check 子代理均在启动阶段因 `encrypted agent_message` 传输错误失败，未写文件；由主会话直接完成实现和复核，不记为独立子代理通过。
- 真实 HTTP/DB/browser、跨进程/failpoint、发布八槽及干净候选/CI 仍为原 `unrun`/`delegated-unrun`；上方新增手工操作均未勾选。

## 2026-09-28 local closeout

- 用户明确授权“提交并归档”；实现归档不等同于未执行 AC 通过。全部未完成实证由 delivery-verification 承接，其 PRD 已登记本矩阵和三类操作恢复场景。
- 提交前重新执行 45 项 Node 针对性回归，全部通过；`GOTOOLCHAIN=local go test ./app/controllers/... ./app/httpserver -count=1 -timeout 60s` 五个包通过；task validate 和 diff 检查通过。前一轮完整 npm test 177 项（176 通过、1 条件跳过）之后没有再改业务代码。
- 本地收尾次序为工作提交 → 实现任务归档 → journal；实际哈希由 task.json 和 journal 记录，不执行远程推送。
