# application-admin 验收证据矩阵

审核基线：2026-09-25。`status` 只表示当前证据状态；`task.json.status` 或 focused unit test 不会自动关闭真实环境门禁。每次运行需记录命令、fixture、Go/Mongo/SMTP/OS、发现/执行/通过/失败/跳过、退出码、首个脱敏错误及 commit/run 摘要。

| ID | 可观察结果与负例 | 必备证据 | owner/交接 | 当前状态 |
| --- | --- | --- | --- | --- |
| AC-A1 | admin action 逐项覆盖 anonymous/member/demo/admin；未授权在 DB/file/outbox/SMTP 前拒绝；旧 `Re`、redirect、status、Content-Type 保持 | service/adapter contract；真实 HTTP route/method/status replay；零副作用 query/file counter | admin；interface/delivery | `unrun` |
| AC-A2 | 多键 Config 更新逐键预检、失败不覆盖 cache、成功 strict read-back；production-config 只消费 interface seam；通用模板不泄露 secret | cache/Mongo failure fixtures；consumer mapping；redaction scan；template/JSON snapshot | admin；interface | `unrun` |
| AC-A3 | mongodump/restore 只用 allowlist absolute regular file + argv/timeout；默认非公开 `mongodb_backup` root，最多 30 份或 20 GiB；单次 Download 最多 100,000 文件/10 GiB 源文件/4 GiB 归档；凭据不进 argv/env/log；root/symlink/traversal/预算/保护备份/同一 database identity/未知结果 fail closed；download 无 panic | fake executable/credential channel；filesystem failpoint；safe tar inspection；真实 Mongo backup/restore | admin；infrastructure/delivery | `unrun` |
| AC-A4 | Blog/Beta2/Beta4 upgrade 有 checkpoint、幂等、逐条错误和 resume；重复调用不重复写/回拨 USN/删错索引 | service/DB migration fixtures；restart/partial-write replay；跨版本真实 Mongo | admin；notes/delivery | `unrun` |
| AC-A5 | `/suggestion` 保持公开/匿名兼容；匿名携带 `submissionId` validation 拒绝，匿名无 ID 可提交且 `RetrySafe=false`；`Addr` 空值允许、非空为单个 addr-spec 且不超过 254 字节；`Suggestion` 合法 UTF-8、trim 后非空、不超过 2000 码点/8192 字节；feedback 文档、receipt、outbox 三者 durable success 且逐项 read-back；SMTP failure 不回滚；无 request goroutine | input matrix；receipt BSON/index/30-day GC；transaction/standalone compensation；HTTP `Re` replay；static goroutine scan；SMTP fake | admin；interface/delivery | `unrun` |
| AC-A6 | admin broadcast/feedback/comment 均 durable enqueue；broadcast 新请求 batchId 为 32 位小写 hex，旧无 ID 通过 `Re.Id` 返回 reconciliation ID 且不可重试，batch/body/recipient digest replay/conflict；`feedbackRecipients` 最多 20 个且先校验去重；CR/LF/非法收件人/模板错误；HTTP 200/`Re.Ok=true` 只表示已入队；retry/dead/handoff_unknown；旧账号 event 不退化；日志脱敏 | outbox memory/Mongo tests；batch/receipt matrix；template/header/recipient matrix；worker lease/CAS；legacy event fixtures；真实 SMTP | admin；identity/publishing/delivery | `partial`（comment focused tests；其余未运行） |
| AC-A7 | comment 状态合法/非法迁移、confirm gate、取消与领取竞态、逐 recipient read-back、typed metadata/index、旧 payload 兼容；缺失/冲突 identity 进入 `handoff_unknown`；闸门/发送先赢不报已取消 | publishing/admin integration；Mongo concurrent fixture；SMTP DATA/QUIT/unknown；人工对账记录 | admin + publishing；delivery | `partial`（现有状态机 focused evidence；真实交接未运行） |
| AC-A8 | password/token/Mongo/SMTP secret、完整授权 URL、未转义敏感正文不进入 source/log/template/summary/artifact/env/EmailLog | repository scan；redacted log/error fixture；artifact inspection | admin；identity/content | `unrun` |
| AC-A9 | 每行证据可复核，明确 `unrun`/`partial`/`delegated-unrun`；不以 build/vet/task validate 替代真实边界 | matrix entry + command/run provenance；四层 diff review | admin；delivery | `in-progress` |

## 当前已有但不能升级为通过的证据

Q-A1～Q-A6 已完成产品确认，当前状态不再是 `blocked-by-decision`；`unrun`/`partial` 仅表示实现或真实运行证据尚未完成。

- `app/db` comment outbox memory/Mongo focused tests 和 `app/service` outbox renderer tests：只能证明局部状态/错误分流，不能证明真实 Mongo/SMTP、跨进程 CAS、人工 `handoff_unknown` 对账或 browser/HTTP。
- `go build`/`go vet`/Node tests（若后续运行）：只能证明编译/静态或前端合同，不关闭 backup/restore、SMTP、Mongo topology、filesystem failpoint 或真实页面门禁。
- 已归档 publishing/content/notes/identity 任务的 `completed` 状态：表示各自 Trellis 生命周期完成，不代表 admin 的交接证据已通过。

## 2026-09-25 本地 focused 运行记录

```text
run: application-admin-review-fixes-local-2026-09-25
environment: Go local toolchain; OS Windows; Mongo/SMTP/mongodump/mongorestore unavailable
commands: go test ./app/db ./app/service ./app/controllers; go vet ./app/db ./app/service ./app/controllers; go build ./app/...; git diff --check; python ./.trellis/scripts/task.py validate 09-08-application-admin
result: focused tests/build/vet/task validation passed; diff check passed
scope: outbox legacy metadata, Suggestion controls/error mapping, backup argv/retention helpers, config-array preflight, broadcast receipt CAS, upgrade USN replay guard, broadcast blank-line filtering
boundary: this run does not upgrade AC-A3/A4/A5/A6/A7 to real-environment passed; Mongo concurrency, SMTP delivery, mongodump/mongorestore credential behavior, HTTP and failpoint evidence remain unrun/partial
commit/run summary: working-tree run; no commit created in this review turn
```

## 证据记录模板

```text
AC: AC-Ax
status: planned | in-progress | passed | partial | failed | blocked-by-decision | unrun | delegated-unrun
commit/run: <immutable id>
environment: Go <>, Mongo <topology/version>, SMTP <fixture>, OS <>
command/url/fixture: <exact>
discovered/executed/pass/fail/skip: <counts>
first failure: <脱敏 category + safe identity digest>
owner handoff: <task/action>
```
