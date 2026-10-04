# E 协调验收执行证据（2026-09-01）

## 范围与结论

本证据段记录的是规格审核阶段：只执行 E 的候选冻结、依赖盘点、Playwright discovery、静态契约和现有
GitHub Actions 证据核对。当时没有修改生产代码、测试、构建脚本、workflow、生成资源、F/父任务材料或
任何任务状态，也没有启动 Mongo、Chromium harness、真实浏览器或发布流程。随后用户批准“启动功能实现”，
主流程才激活 E；当前 `task.json` 为 `in_progress`，不将该生命周期切换误记为验收运行证据。

候选为 `fcc979bb9f0fe35d1771b00665017e470e2182d4`（branch `dev`）。候选业务树在排除
`.trellis/tasks` 后无未提交路径；当时工作树中 18 条变更均为既有 Trellis 任务材料。当前 E 状态为
`in_progress`。因此下面本地命令均可明确绑定候选源码，但不能替代干净 Linux checkout、CI 运行或
真实浏览器证据。

结论仍为 **blocked**。本轮没有遇到超时；所有单条本地检查均在 30 秒内返回。

## 依赖与归档盘点

`task.json.meta.depends_on` 仍只含 `08-25-frontend-build-chain`。下列归档 task.json 均为
`status=completed`，但只作为生命周期/所有权证据，不替代当前候选通过证明：

| 任务 | 归档路径 | 完成日期 | 与 E 的关系 |
|---|---|---|---|
| `08-25-frontend-build-chain` | `archive/2026-08/08-25-frontend-build-chain/task.json` | 2026-08-27 | E 的唯一显式依赖 |
| `08-25-jquery-upgrade` | `archive/2026-08/08-25-jquery-upgrade/task.json` | 2026-08-30 | jQuery 契约/owner |
| `08-25-bootstrap-upgrade` | `archive/2026-08/08-25-bootstrap-upgrade/task.json` | 2026-08-30 | Bootstrap 契约/owner |
| `08-25-tinymce-upgrade` | `archive/2026-08/08-25-tinymce-upgrade/task.json` | 2026-08-31 | TinyMCE 契约/owner |

验证命令：`python ./.trellis/scripts/task.py validate 08-25-frontend-libs`（通过；
`implement.jsonl=7`、`check.jsonl=10`）。

## 本地依赖、静态资源与测试发现

环境：Node `v24.20.0`、npm `11.19.0`。以下命令均在候选 SHA 上运行：

```powershell
npm ls --all --json > (Join-Path $env:TEMP 'leanote-npm-tree.json')
& .\node_modules\.bin\playwright.cmd test --config=playwright.config.mjs --project=build-smoke --list
& .\node_modules\.bin\playwright.cmd test --config=playwright.config.mjs --project=business --list
& .\node_modules\.bin\playwright.cmd test --config=playwright.config.mjs --project=browser-smoke --list
```

`npm ls --all --json` 返回 0；`package-lock.json` 的完整 `packages` 树与安装树都只发现以下各一条路径：

| 包 | 版本 | 路径 | 数量 |
|---|---|---|---:|
| jquery | 3.7.1 | `node_modules/jquery` | 1 |
| bootstrap | 5.3.8 | `node_modules/bootstrap` | 1 |
| tinymce | 8.8.2 | `node_modules/tinymce` | 1 |
| jquery-migrate | 3.6.0 | `node_modules/jquery-migrate` | 1 |
| playwright | 1.62.1 | `node_modules/playwright` | 1 |
| esbuild | 0.28.2 | `node_modules/esbuild` | 1 |

`jquery-migrate` 不出现在 `MANIFEST` inputs（0 条），`MANIFEST` 中 jquery、Bootstrap、TinyMCE
输入分别为 3、6、35 条，且 manifest 序列化内容没有 `http(s)://`、cdnjs、unpkg 或 jsdelivr。
兼容 URL `/js/jquery-1.9.0.min.js` 的受跟踪字节与
`node_modules/jquery/dist/jquery.min.js` 的 SHA-256 相同：
`fc9a93dd241f6b045cbff0481cf4e1901becd0e12fb45166a8f17f95823f0b1a`（各 87533 bytes）。
对 `public/js`、`public/album/js`、`public/admin`、`public/member`、`public/blog` 的静态扫描没有发现
`jQuery v1/v2`、`JQMIGRATE:` 或 `jQuery Migrate`。这是当前源码的辅助静态结果；尚未覆盖干净 Linux
重建后的所有输出，不能关闭 AC-E2/AC-E3。

Playwright discovery 均以 0 退出：

| project | 发现 | 文件 | 结论 |
|---|---:|---:|---|
| `build-smoke` | 1 | `tests/e2e/build/build-resource-smoke.spec.mjs` | 指定资源 smoke 文件被发现 |
| `business` | 22 | 5 | 发现 `ajax-failure`、`bootstrap-components`、`business-flows`、`editor-flows`、`jquery-diagnostics` |
| `browser-smoke` | 6 | 2 | 仅 `business-flows`、`editor-flows`；未选中 `bootstrap-components`，也没有独立的 `leaui-image-iframe` coverage ID |

`--list` 不执行用例主体，故上述结果只证明 discovery；不能替代 E2E、清理、服务端契约或真实浏览器成功。

## GitHub Actions 与 artifact 核对

候选的 CI 是 [run 33477561244](https://github.com/yangphere/leanote/actions/runs/33477561244)：
event=`push`、ref=`refs/heads/dev`、attempt=1、结论=`failure`。job 级事实如下：

| job | job ID | 结论 | 可用计数/备注 |
|---|---:|---|---|
| `go-1_26_7` | 99759909498 | passed | summary 记录 discovered/executed=100/100 |
| `go-1_27_0` | 99759909101 | passed | summary 记录 discovered/executed=100/100 |
| `node-build` | 99759909194 | failed | `npm run build` 后 `git diff --exit-code` 失败；后续 untracked/Node test step skipped |
| `mongo-8_0` | 99759909276 | failed | integration test step 失败；CI Mongo service 已启动 |
| `chromium-e2e` | 99759999476 | failed | Chromium E2E step 失败 |
| `package-smoke` | 99760156757 | failed | `sh sh/package.sh` 失败；下游 E/F 证据 |
| `container-smoke` | 99760157037 | failed | deterministic container build 失败；下游 E/F 证据 |
| `summary` | 99762299589 | failed | 7 个质量摘要验证失败 |

run 上传的 8 个 artifact 全部命名为 `ci-summary-*`；没有 `browser-release-matrix-v1`。下载并解析这些
脱敏 JSON 后还发现一致性缺陷：所有失败 job 的 summary 均写作 `stage=job_not_started`、
`failure=job_not_started`、计数为 null，而 GitHub job step 明确显示它们已经启动并在后续步骤失败。
因此这些失败 summary 不能作为发现/执行数量或根因的可靠载体；以 job URL/step 事实保留失败，summary
writer/quality-gate owner 需在重跑前修复其 failure-path 记录。E 不修改该实现。

## tag 预检与浏览器 artifact 阻断

Q-E1 选择等待 tag artifact。当前候选既没有本地 `git tag --points-at` 输出，也没有
`git ls-remote --tags origin` 指向该 SHA 的输出；候选 push run 也没有浏览器 artifact。

静态核对还表明当前实现不能产出规格要求的“仅供 E 验收的预检”路径：

- `.github/workflows/browser-release-evidence.yml` 只接受 `workflow_call`；
  `.github/workflows/release.yml` 在 tag 流程中先等待 `quality-gate`，再调用 browser evidence，随后由
  `publish` 消费同一 artifact 并执行 GHCR push / GitHub Release。没有独立的受保护 tag 预检入口来保证
  artifact 生成后不发布、不改变 F 状态。
- `scripts/browser-release-evidence.mjs` 当前写入通用 coverage
  `build-smoke/auth-gate/error-gate/resource-gate`，允许 1--40 项；记录没有
  `coverage_summary_sha256`，provenance 没有 `coverage_summaries`。
- `scripts/validate-browser-artifact.mjs` 仍要求旧的六个 provenance 顶层字段，未校验四个固定
  coverage ID、RFC 8785 JCS digest 或八槽位脱敏摘要。

这三项由 F/release-evidence owner 处理。完成条件是：严格 `vX.Y.Z` tag 指向候选 SHA，由独立受保护
预检 producer 生成两文件 `browser-release-matrix-v1`，不执行发布；其 8 个槽位必须绑定相同
run/attempt、四个固定 coverage ID 与 coverage 摘要 digest。E 归档后 F 再执行最终发布 run。

## 未运行项目

当前环境是 Windows，且本轮任务边界不允许改 harness 或 workflow。为了避免与已知 27017 冲突混用，未运行
Mongo/Go integration 或 `go run ./app/tests/harness/cmd/e2e`；它们保持由 CI 失败证据阻断。没有可用的
受保护 Chrome、Edge、Firefox、Safari current/previous stable 环境，也没有 tag artifact，真实 8-slot
矩阵保持 missing，而不是用 Playwright Chromium 或 WebKit 替代。
