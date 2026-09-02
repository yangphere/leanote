# B-E5 实施计划

## Task 0: Environment Readiness Gate

- [x] 规格审核完成（`research/spec-audit-2026-09-02.md`）；design/implement 就位。
- [x] **Q-E5-1** 已答复（2026-09-02"跳过macOS验证"）：Safari 两槽跳过；本机真实产品执行工程证据（Chrome/Edge/Firefox current 三槽 16/16 全绿，见 acceptance/engineering-evidence.md）；8 槽预检 artifact 因 producer 硬性要求含 Safari 而挂起，E AC-E6 保持 blocked。
- [x] **Q-E5-2** 随预检路径挂起（无 8 槽 artifact 则无 tag 需求）；恢复预检时再议版本号。
- [x] 任务收口形态（blocked 交付，AC-4）经用户确认归档（2026-09-02"归档"）；全程未运行 `task.py start`（无 8 槽执行可激活）。

## Task 1-4: 8 槽执行路径（SUSPENDED——用户裁决跳过 Safari，producer 硬性 8 槽无法满足；恢复条件见缺口台账）

## Task 1: 环境准备（依 Q-E5-1 答复，仓库外）

- [ ] 注册受保护 runner（标签 `[self-hosted, protected-browser-matrix]`），按答复范围安装产品×版本。
- [ ] 配置 8 条（或分步路径的 6 条）`BROWSER_SMOKE_COMMAND_*`，满足 design §3 marker 契约；每条命令单槽位试跑通过。
- [ ] Safari（若 (a)）：macOS 硬件 + safaridriver 驱动方案调试至四套件等价输出。

## Task 2: 预检执行

- [ ] 用户创建严格 tag 指向候选 SHA（Q-E5-2 版本号）。
- [ ] workflow_dispatch 触发预检；监控 8 槽位执行；失败槽位登记并重跑（新 attempt）。
- [ ] 下载 `browser-release-matrix-v1` 两文件至 checkout 外。

## Task 3: 校验与登记

- [ ] `validate-browser-artifact.mjs --phase precheck --expected-commit <候选>`（含 GITHUB_REF 严格 tag）全绿。
- [ ] 脱敏摘要 + run/attempt URL + validator 输出登记进任务材料；PRD AC 勾选。

## Task 4: 交接

- [ ] 交付 E（AC-E6 retest 输入）与 B-E6（八槽位矩阵 + 预检 run ID）；归档需用户确认。

## Completion Gate

- [ ] 8 槽位真实产品全过 + validator 全绿 + 无发布副作用；或环境 blocked 登记完整（AC-4 形态）。
