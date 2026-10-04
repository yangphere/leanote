# B-E6 技术设计 — 三门禁修复与发布收口

依据：`prd.md`、`research/spec-audit-2026-09-02.md`（行级根因）。

## 1. D1：package.sh tag 判定（sh/package.sh:7-9）

```sh
# set -eu 安全：无 GITHUB_REF（本地执行）时两侧默认空串，不触发 unbound variable
TAG=${RELEASE_TAG:-}
case "${GITHUB_REF:-}" in
  refs/tags/*) TAG=${TAG:-"${GITHUB_REF:-}"}; TAG=${TAG#refs/tags/} ;;
esac
if [ -n "$TAG" ]; then node "$ROOT/scripts/version.mjs" "$TAG" >/dev/null; fi
```

- 分支 push：`GITHUB_REF=refs/heads/dev` → 不进入断言；tag push / release.yml 显式 `RELEASE_TAG`：行为与现状一致。
- 回归：tests 直接断言脚本两态（正则扫描 shell 文本 + 以子进程跑 `RELEASE_TAG=v0.0.0`/空两分支——沿用 release-contract 的文本断言风格，避免为 sh 搭 harness）。

## 2. D2：整型 epoch 与 OCI 标签拆分

- quality-gate.yml 与 release.yml 的构建步骤（两处同型缺陷，release.yml:102 同样把 RFC3339 传给 `SOURCE_DATE_EPOCH`）：统一改为 `--build-arg SOURCE_DATE_EPOCH="$epoch" --build-arg OCI_CREATED="$created"`——这是参数类型缺陷修复，不改变任一 workflow 的流程/门禁语义。
- Dockerfile：`ARG SOURCE_DATE_EPOCH=0`（语义不变，buildkit 整型）；新增 `ARG OCI_CREATED=`，标签行改 `org.opencontainers.image.created="$OCI_CREATED"`；两条 workflow 路径（quality-gate/release）均恒传 `OCI_CREATED`，无空标签路径。
- package-smoke 同链路核对：`sh/package.sh:11-12` 已强制整型校验（`*[!0-9]*` 拒绝非数字），不动。
- 回归：release-contract 断言 workflow 传双 arg 且 Dockerfile 标签引用 `OCI_CREATED`、`SOURCE_DATE_EPOCH` 不再出现在标签行。

## 3. D3：write-summary stage 语义（scripts/ci/write-summary.mjs:104）

```js
stage: forcedFallback ? 'job_not_started' : (process.env.CI_STAGE || 'complete'),
```

- 依据：write-summary 能成功执行即证明 job 已启动并到达总结步骤——生命周期事实是 `complete`；成功/失败由 `status` + `failure` 块表达，`CI_STAGE` 仍可显式覆盖。
- forcedFallback（write-summary 自身失败重试）保持 `job_not_started`：该路径表示"无法确认后续阶段"，语义不变。
- 既有测试核对：`summary writer rejects missing provenance...` 用 `CI_FORCE_FALLBACK`（不受影响）；`summary validator rejects placeholder provenance` 的 fixture 显式写 stage（不受影响）；新增正/反例：失败+非 fallback → `complete`；fallback → `job_not_started`。

## 4. D4/D5：登记（零代码）

- E evidence matrix：AC-E8 的 F/summary 行补记 D3 根因修复与三门禁全绿 run；AC-E6 行维持 blocked 并引用 B-E5 缺口台账；Downstream reconciliation 段登记 F 冲突的最终对账（completed 状态为历史生命周期事实、notes 阻断主张与 D5 一致成立——冲突实质是"任务生命周期完成 ≠ 发布获批"，登记后发布结论以 D5 为准）。
- 父任务 notes：由 E 收口阶段或经批准的父材料最小更新登记（不在本任务越权改父材料；本任务产出登记文本放入自身材料供 E/父引用）。

## 5. 执行序与验证

1. D1/D2/D3 修复 + 锁步测试（单提交或按仓库习惯拆分，每提交单一目的）。
2. 本地：`npm test` 全绿（含新契约测试）；`sh sh/package.sh` 本地（无 tag 上下文）可执行至产物生成（若本地依赖不齐则记录并依赖 CI 验证）。
3. push → CI：package/container/summary + 既有绿门禁全部 success；下载三 summary artifact 复核 stage/status 与 job 事实一致（D3 验证）。
4. E matrix/登记文本产出 → 汇报终局结论（发布保持 blocked）。

## 6. 回退

三处修复相互独立，revert 单提交即回现状；无数据/流程迁移。
