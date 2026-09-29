# 候选证据与发布门禁完整性

2026-09-29 收尾说明：本地受控 seam 与 workflow 合同检查结果见 evidence-matrix.md“实现阶段工具验证”；下列场景的真实环境证据仍为 unrun，按用户最新决定排除本次归档条件。后文“本轮均不执行”及“实现差距”是规格审核时快照，不代表仓库工具尚未实现；实际实现与剩余缺口见 ../research/implementation-2026-09-29.md。

对应 PRD R1/R2/R8/R9、AC-DV1/8/9/10 和 E-D13。本文是既有要求的验收细化，不是新业务合同或已实现能力。全部场景 `unrun`；产品测试、CI 与远端操作本轮均不执行。

## 场景

| ID | 输入/触发 | 必须观察的结果 | 状态 |
| --- | --- | --- | --- |
| GI-01 | 所有 summary 彼此一致，但均来自另一 SHA/ref/workflow/run/attempt；或错误仓库的同名 artifact | 与可信执行上下文不匹配即拒绝，publish 写操作为 0；不能仅以首条记录作身份基准 | unrun |
| GI-02 | 原七个 job 均通过，任一扩展必需场景缺失/重复抵数/未映射；或只完成 discovery、执行零项 | 集合对账失败，不能宣布九轨联验通过；每个上游条款均有对应 job/场景，计数不是覆盖证明 | unrun |
| GI-03 | status=passed 却退出非零/未知、清理失败/未确认、必需项 skip、partial、blocked 或 cancelled | 汇总失败并保留原失败与清理结果；不得因 job 主命令成功而覆盖失败，不向现有 schema 塞入未知状态 | unrun |
| GI-04 | checkout/setup/runner 失败或取消，summary/artifact 未生成；篡改 workflow 使某必需门禁不再位于 publish 前置链 | 缺失明确阻断；workflow contract 检查必需依赖和失败传播，不以可跳过的附属 job 代替门禁 | unrun |
| GI-05 | 查询 Release/image 的认证/权限/限流/网络/解析错误，正文包含 not found；错误仓库的 404 | unknown、零写入；目标身份和读取权限已核验且结构化资源结果明确 absent 才可进入创建分支 | unrun |
| GI-06 | recovery 从版本已前进的分支 dispatch，目标是较早 strict tag；原 candidate 版本文件/epoch 缺失或与原件不符 | 合法旧候选从原 SHA 的版本文件验证；不读取当前分支版本作期望，不改原文件；原材料缺失/冲突阻断 | unrun |
| GI-07 | 普通 create 退出 0，但 Release 为 draft/prerelease、tag 不符、三项资产缺失/多余或实际字节 hash 不符；读回失败/响应丢失 | 不宣告完成，按 partial/conflict/unknown 保留身份；三项为 tarball、其 checksum、build-metadata，平台自动生成的源码归档不计上传资产；禁止盲重建/补传/覆盖 | unrun |
| GI-08 | 原 source run 前一 attempt 门禁通过，指定 attempt 失败；相同 workflow 显示名但不同执行来源；同名异 ID artifact | 核验指定 source attempt 的实际门禁和 artifact 来源，不能借另一 attempt 或显示名冒充；零写入 | unrun |
| GI-09 | 所有必需门禁通过并且来源相符，但没有目标仓库/tag/SHA 的具体发布授权 | 不执行 tag/image/Release 写操作；规格中的 Q-DV1 规则批准和 task in_progress 均不是发布执行授权 | unrun |

每项沿用 `evidence-matrix.md` 的候选、run/attempt、命令/步骤、环境、退出与清理、artifact/hash、owner 字段；发布负例补充远端读取分类与写操作计数。先以受控 seam 验证失败传播和零写入，再在明确授权的真实环境证明对应能力，二者证据分开。本轮不要求运行服务或浏览器。

## 已核实的实现差距（2026-09-29，HEAD e1dfd907）

- `scripts/ci/validate-summaries.mjs` 固定七个 job，末尾只比较 records 之间的身份；`write-summary.mjs` 与 quality workflow 也有独立固定集合/计数。须同步扩展现有边界并添加当前执行身份核对。
- `release.yml` 的重复检查采用 stderr 文本正则判断远端不存在；Release create 后没有最终资产字节读回。这些属于既有 R9/AC-DV10 的实现待办。
- `validate-release-artifact.mjs` 调用 `readProjectVersion()`，`scripts/version.mjs` 默认读取 cwd；原候选与 recovery 执行器分离是待实现合同。不能通过伪造当前版本、run 或 attempt 消除差异。
