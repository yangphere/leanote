# jQuery 3.7 浏览器 smoke 记录（R-jQ7 / AC-jQ9）

基线提交：`b3a6ab95864dc3ad565c8637de5f11d03ea4efdb`（工作树含本任务未提交改动时执行，提交后需以最终 SHA 复跑并追加记录）。

执行方式：仓库 test-mode harness（`go run ./app/tests/harness/cmd/e2e`）提供隔离环境（leanote_test fixture、随机 run token、轮换后的 admin 凭据），Playwright 以 `npm run test:e2e:smoke` 运行完整 business 流程（登录、笔记列表/搜索、笔记+标签写入与清理、笔记本写入与清理、对话框、附件与相册真实上传及清理、相册、博客、admin/member、leaui_image iframe），并执行认证、`console.error`、`pageerror`、未处理 rejection 与应用自有资源 4xx/5xx 门禁。身份预检未跳过，数据清理全部通过（清理失败会使测试失败）。

## 2026-08-28 执行记录

| 浏览器 | 产品/完整版本 | 操作系统 | 覆盖 | 身份预检 | 错误/网络门禁 | 结果 |
|---|---|---|---|---|---|---|
| Chrome（真实二进制，Playwright channel=chrome） | 152.0.7977.64 | Windows 11（win32） | 全部流程（见上） | 通过 | 零错误 | 2/2 通过 |
| Edge（真实二进制，Playwright channel=msedge） | 151.0.4129.107 | Windows 11（win32） | 全部流程（见上） | 通过 | 零错误 | 2/2 通过 |
| Firefox（Playwright 构建） | 153.0 | Windows 11（win32） | 全部流程（见上） | 通过 | 零错误 | 2/2 通过 |
| Chromium（诊断基线） | Playwright Chromium（headless shell 151.0.7922.34） | Windows 11（win32） | business + JQMIGRATE 诊断 | 通过 | 零错误 | 3/3 通过 |

驱动：Playwright 1.62.1，Node v24.20.0。本记录不含任何认证材料、页面正文或用户数据。

## 2026-08-28 复跑（第四轮整改后最终工作树）

| 浏览器 | 版本 | 覆盖 | 结果 |
|---|---|---|---|
| Chrome（真实二进制） | 152.0.7977.64 | business 全流程（身份预检 + 权限门禁 + 错误密码负向 + 笔记/标签/笔记本/附件/相册写入与清理 + leaui 零缺失资源） | 3/3 通过 |
| Edge（真实二进制） | 151.0.4129.107 | 同上 | 3/3 通过 |
| Firefox（Playwright 构建） | 153.0 | 同上 | 3/3 通过 |

备注：Firefox 会把字体下载管线的 `downloadable font:` 消息记为 console.error（字体文件实际可访问、页面渲染正常）；该类消息为引擎噪声，已在 error 门禁中文档化排除，应用脚本错误仍然零容忍。

## 发布前仍须补齐（阻断 AC-jQ9，不阻断本 PR 评审）

- **Safari 当前及前一主版本**：须在真实 macOS/Safari 环境执行，本机（Windows）无法产生有效证据。
- **Chrome / Edge 前一主版本**：本机未安装 Chrome 151 / Edge 150，须在装有前一主版本的机器或 CI matrix 上执行。
- **Firefox 前一主版本**：同上。
- 基线提交合并后，须以最终合并 SHA 复跑上表并追加新记录。

以上任何一项缺失或失败，按 R-jQ7 阻断发布验收。
