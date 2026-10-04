# B-E5 技术设计 — 真实八槽位矩阵执行

依据：`prd.md`（环境门禁与需求）、`research/spec-audit-2026-09-02.md`（环境硬证据）；契约字段级来源为 F `release-matrix-contract.md` 与 B-E4 归档实现。本任务**不改仓库代码**，设计只定义执行序列、runner 命令契约与证据处理。

## 1. 改动面

零仓库代码改动（发现 B-E4 产物缺陷只登记返回 owner）。产出为：环境配置（runner/命令，仓库外）、一次预检运行与两文件 artifact、validator 输出、blocked/通过登记。

## 2. 执行序列（环境就绪后）

```text
候选 SHA（dev 推送头，当前 016cf5ae 谱系）
  → 用户创建严格 tag vX.Y.Z 指向该 SHA（Q-E5-2 定版本号）
  → GitHub Actions: workflow_dispatch "Protected browser release evidence"（输入 tag）
      ↳ 校验 tag 格式 → checkout tag → 剥壳 ^{} 得 RELEASE_COMMIT
      ↳ 8× BROWSER_SMOKE_COMMAND_<PRODUCT>_<SLOT> 真实执行（§3 契约）
      ↳ producer 生成 release-matrix.json + provenance.json（四 ID、JCS、八槽摘要）
      ↳ 上传 browser-release-matrix-v1（retention 7）
  → 下载 artifact 至 checkout 外
  → node scripts/validate-browser-artifact.mjs <dir> --phase precheck --expected-commit <候选 SHA>
      env: GITHUB_REF=refs/tags/vX.Y.Z（重算 raw bytes/JCS/八槽/相邻主版本/tag 绑定）
  → 证据（脱敏摘要 + run/attempt URL）登记进任务材料，交付 E 与 B-E6
```

## 3. runner 命令契约（`BROWSER_SMOKE_COMMAND_<PRODUCT>_<SLOT>`）

每条命令在受保护 runner 上真实启动该产品/版本并执行四套件，stdout 必须依次包含（B-E4 producer fail-closed 解析）：

```text
LEANOTE_BROWSER_VERSION=<完整版本，如 152.0.7977.64>
LEANOTE_BROWSER_OS=<os 标识>
LEANOTE_AUTH_GATE=passed
LEANOTE_ERROR_GATE=passed
LEANOTE_RESOURCE_GATE=passed
LEANOTE_COVERAGE_business_flows=discovered=N;executed=M
LEANOTE_ENTRYPOINTS_business_flows=note,blog          （≥1 项，标识符 ^[a-z0-9][a-z0-9._/-]{0,79}$，无前导斜杠）
LEANOTE_IFRAMES_business_flows=                        （可为空）
  … editor_flows / bootstrap_components / leaui_image_iframe 同构四行组
```

实现载体建议（非强制）：Chrome/Edge/Firefox 用 Playwright channel（`LEANOTE_SMOKE_BROWSER=chrome|msedge|firefox` + `npm run test:e2e:smoke`，套件已含四文件与全部门禁），命令脚本解析结果转 markers；**Safari 无 Playwright channel**，需 safaridriver（macOS）驱动四套件等价流程并输出同构 markers——此为 Q-E5-1(a) 的主要工作量。命令脚本属 runner 环境资产，不入本仓库（workflow 注释约定）；如需版本管理可在用户批准后另立受保护位置。

## 4. 证据处理与失败语义

- 失败槽位：producer 即刻失败、不产出矩阵；重跑 = 新 attempt，旧失败记录保留（run URL 登记）。
- validator 失败：保留原始错误类别与 owner，不降级、不局部重跑冒充全绿。
- 脱敏：仅记录版本/OS/计数/摘要 digest/run URL；禁凭据、正文、截图、trace、原始日志。
- 部分矩阵（Q-E5-1(b) 分步路径）：登记 6/8 blocked + 缺口清单（Safari 两槽），不得进入 E 验收（AC-E6 要求恰好 8）。

## 5. 兼容与回退

- 预检入口/producer/validator 全部为 B-E4 已验证产物（CI node-build 契约测试 26/26），本任务零改动即用。
- 若执行暴露 B-E4 产物缺陷：登记 owner=B-E4 归档任务，按"归档后重开/扩域授权"流程（E design §6），不在本任务内修。
- 回退：无仓库改动，天然可回退；环境资产由用户侧管理。
