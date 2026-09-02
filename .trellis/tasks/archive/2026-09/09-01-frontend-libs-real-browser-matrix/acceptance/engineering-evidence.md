# B-E5 工程证据台账（真实浏览器，脱敏）

更新时间：2026-09-02　执行环境：开发者 Windows 本机（win11），仓库 test-mode supervisor（`go run ./app/tests/harness/cmd/e2e`，leanote_test 隔离 fixture、随机 run token、轮换 admin），方法学与 `docs/modernization/browser-smoke/jquery-3.7.md` 先例一致。

## 用户裁决记录

- **Q-E5-1**（2026-09-02，原话"跳过macOS验证"）：Safari 两槽位跳过（不因 macOS 阻塞）。
- **契约后果（如实登记）**：B-E4 producer/validator 与 E/F 契约**硬性要求恰好 8 槽位**（含 Safari），Safari 缺席则 8 槽 `browser-release-matrix-v1` 预检 artifact 无法产出——预检路径挂起，E 的 AC-E6 保持 blocked。Q-E5-2（严格 tag）随预检路径一并挂起。
- 本台账为**工程证据**（jQuery 文档同级先例），不冒充 8 槽契约 artifact，不进入 E 验收。

## 已执行槽位（current_major，真实产品，四套件=business-flows/editor-flows/bootstrap-components/leaui-image-iframe）

| 产品 | 完整版本 | 驱动 | OS | 发现/执行 | 结果 | 清理 |
|---|---|---|---|---|---|---|
| Chrome（真实二进制，channel=chrome） | 152.0.7977.65 | Playwright 1.62.1 | win32/win11 | 16/16 | 全绿（32.2s） | 通过（清理失败即失败） |
| Edge（真实二进制，channel=msedge） | 152.0.4191.53 | Playwright 1.62.1 | win32/win11 | 16/16 | 全绿（31.8s） | 通过 |
| Firefox（Playwright Firefox） | 153.0 | Playwright 1.62.1 | win32/win11 | 16/16 | 全绿（35.2s） | 通过 |

每槽位含：身份预检（run token 门禁）、console/pageerror/unhandled-rejection 门禁、owned-resource 4xx/5xx 门禁、真实上传/写入与清理、编辑器语义与 leaui 跨 iframe 契约。命令：`LEANOTE_SMOKE_BROWSER=<chrome|msedge|firefox> go run ./app/tests/harness/cmd/e2e -- npm run test:e2e:smoke`。

## 缺口台账（blocked 项，owner/retest）

| 槽位 | 状态 | 原因 | retest 条件 |
|---|---|---|---|
| Safari current/previous | **用户裁决跳过** | 需 macOS 硬件 + safaridriver（Playwright 无真实 Safari channel） | 用户提供 macOS 环境并裁决恢复；或明确修订 E/F 契约为非 8 槽（属契约变更，需另行授权） |
| Chrome previous_major | 缺二进制 | 本机仅 stable 152 | 下载 Chrome for Testing 151.x 官方构建（executablePath 驱动）后复跑 |
| Edge previous_major | 缺二进制 | 无便捷旧版独立构建渠道 | 提供旧版 Edge 独立安装后复跑 |
| Firefox previous_major | 缺二进制 | Playwright 绑定单一 Firefox 版本 | 提供官方旧版 Firefox + 驱动方案后复跑 |

## 对 E/B-E6 的含义

- E AC-E6（8 槽 tag 预检 artifact）：保持 blocked（Safari 缺席 + 预检路径挂起）。
- B-E6：其 depends_on 含本任务；本任务以"blocked 形态交付"（AC-4）收口后，B-E6 的发布收口范围需在其实际执行时按当时契约状态复核。
