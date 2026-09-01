# 修复 release-contract 测试的 CI run 环境泄漏

父任务：`.trellis/tasks/08-25-frontend-libs`
路由依据：B-E1 的 mode 修复（`c903007`）使 CI node-build 首次执行到 npm test，暴露本潜伏缺陷；并入 B-E6 会死锁（B-E6 排序最后，而 node-build 全绿是 B-E1 关闭前提），故按 `08-27-jquery-upgrade-spec-repair` 先例开独立轻量修复任务。用户已批准"采用推荐方案"（2026-09-01）。

## Confirmed Defect

- CI run `33519988846` / node-build job `99896497577`（commit `c903007`）：`release artifact validation rejects unknown metadata schema versions` 与 `release artifact validation binds build metadata to the tarball bytes` 两用例失败（121 用例中仅此 2 失败）；本地（无 `GITHUB_RUN_ID`）120/120 全绿。
- 根因：两用例以 `execFileSync` 运行 `scripts/validate-release-artifact.mjs`，`env` 展开 `process.env` 却未处理 CI 注入的 `GITHUB_RUN_ID`/`GITHUB_RUN_ATTEMPT`。validator 的防重放守卫（`validate-release-artifact.mjs`：`GITHUB_RUN_ID` 与 `releaseInputs.run.id`、`GITHUB_RUN_ATTEMPT` 与 `run.attempt` 必须相等）发现 fixture 固定值 `'12'/1` ≠ 真实 run，先抛 `release inputs run mismatch`，`assert.throws` 期望的 `/schema version|metadata mismatch/i` 与 `/build metadata tarball hash mismatch/` 路径从未到达。
- 该守卫由 F（`08-25-cicd-delivery`，2dd4d18）引入；其后两次 CI（`fcc979b`、`b8a9c68`）均死在更早的 mode 漂移门禁，npm test 在 CI 从未执行，故缺陷潜伏至今。同文件其他子进程用例（如 `summary writer rejects missing provenance`）均显式自设或删除 `GITHUB_*`，唯此两处遗漏。

## Requirements

1. 两个 spawn 的 `env` 在 `process.env` 基础上显式钉 `GITHUB_RUN_ID: '12'`、`GITHUB_RUN_ATTEMPT: '1'`，与 `buildReleaseInputs` fixture 值一致：既消除 CI 泄漏，又让守卫的相等放行路径被真实执行（优于删除变量）。
2. 不修改 `scripts/validate-release-artifact.mjs`——生产防重放守卫语义保持不变。
3. 不触碰其他测试与实现文件。

## Acceptance Criteria

- [x] 本地全量 `npm test` 全绿：120 过 / 0 失败 / 1 跳过（win32 跳过 POSIX 用例）；另以 `GITHUB_RUN_ID=33519988846 GITHUB_RUN_ATTEMPT=1` 注入 CI 同款环境验证该文件 19/19 通过（修复前此环境必现 2 失败）。
- [x] 修复提交 `99abfab` 的 [node-build job](https://github.com/yangphere/leanote/actions/runs/33522450969/job/99904830024) 全绿：零漂移门禁通过，npm test 121/121（含 B-E1 mode 契约用例在 Linux 通过）。run 33522450969 其余失败均为既有阻断（mongo=B-E2、chromium=B-E3、package/container+summary=B-E6/F）。
- [x] 提交 `99abfab` 变更仅限 `tests/js/release-contract.test.js` 两处 env（+注释）。

## Out Of Scope

- 不处理 mongo-8_0（B-E2）、chromium-e2e（B-E3）、package/container-smoke 与 summary（B-E6/F）的既有失败。
- 不修改 E 验收矩阵与 F 任务状态。

## Provenance

- 缺陷证据：run `33519988846`（2026-09-01），job `99896497577`，失败日志含 `AssertionError … 'release inputs run mismatch'`。
