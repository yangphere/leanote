# 前端构建链现代化（D）— 实施计划

## Global Constraints

- 实现前重新读取本任务 PRD、design、父任务/ADR-0003、`AGENTS.md`、`CLAUDE.md` 和实际 Gulp 输入；Node 版本为 `>=24 <25`，esbuild 精确为 0.28.2。
- 本任务只替换构建链。不得升级 jQuery、Bootstrap、TinyMCE，修改业务源码为 ESM，重建 TinyMCE 核心/插件，或恢复 Deprecated/non-default 产物。
- manifest 是输入/输出唯一事实；构建使用本地 `npm ci` 依赖，所有错误 fail closed。默认 build 生成 12 个运行时产物和 21 个 locale 产物，且只生成它们。D 另外提供复用的 Chromium 构建资源 smoke；E 不得另装一份 Playwright。
- `npm test` 必须不写入真实生产产物；生产构建在完整 staging 验证后才发布。`Gulpfile.js` 和 Gulp 依赖最后删除。

## Task 1: Capture Contracts Before Replacing The Builder

**Files**
- Create: `tests/js/fixtures/build/i18n-contract.json`
- Create: `tests/js/build-pipeline.test.js`
- Read only: `Gulpfile.js`, `app/views/note/note-dev.html`, `app/views/note/note.html`, `messages/**`, current generated assets

- [ ] 从当前受跟踪的 14 个 browser i18n 与 7 个 locale TinyMCE 文件解析并写入 fixture；记录完整静态 key/source/line 清单、`msg`、`blog`、TinyMCE map，历史缺失集合和登记动态调用定位。fixture 不得通过手写翻译或修改 message `.conf` 获得“完整”；字符串和注释中的伪调用必须忽略。
- [ ] 编写失败测试，断言 R-D2 清单、旧 Gulp 每个生产数组顺序、12/21 输出路径、Git 跟踪状态和保留的 `markdown.min.js`/TinyMCE `en.js`/`zh.js`/readme 边界。
- [ ] 编写模板 fixture 测试，断言三个 dev block、四个 marker、TinyMCE/插件 URL、四个 bundle URL 与顺序，以及允许转换以外的片段不变。
- [ ] 编写 i18n fixture 测试，断言完整静态 key/source/line 清单、结构相等、历史缺失回退、登记动态调用；新增静态缺失或动态位置必须失败并定位。
- [ ] 运行 `node --test tests/js/build-pipeline.test.js`，确认在新构建模块不存在前按预期失败；保留该失败证据，不修改运行时代码。

## Task 2: Add Manifest And Pure Generator Interfaces

**Files**
- Create: `scripts/build/manifest.mjs`
- Create: `scripts/build/js.mjs`, `scripts/build/css.mjs`, `scripts/build/i18n.mjs`, `scripts/build/note-html.mjs`, `scripts/build/index.mjs`
- Create: `playwright.config.mjs`, `tests/e2e/build/build-resource-smoke.spec.mjs`
- Modify: `package.json`, `package-lock.json`

- [ ] 在 manifest 中逐项写入设计表的根相对输入、输出、转换、扫描根、7 个 locale、保留路径和登记的动态 key 例外；每一输出路径只出现一次。
- [ ] 各模块导出可由 `build-pipeline.test.js` 调用的纯生成/验证函数，并接受显式 repo root 与 staging root；默认入口只使用仓库 root，测试使用一次性临时输出 root。
- [ ] 在 `index.mjs` 的所有仓库 I/O 前验证 `process.versions.node` 的 major 必须为 24；提取可注入版本字符串的纯 guard，为 23、24、25 写入明确的拒绝/成功单测，不能把 `package.json.engines` warning 当作门禁。
- [ ] 加入路径规范化与 manifest 验证：绝对路径、`..`、符号链接逃逸、重复输出、未声明发布及缺失输入立即失败，并在错误中给出 entry/input/output。
- [ ] 用锁定的本地 esbuild 实现旧 Gulp 的处理粒度：dep 原样串接；app/plugins/markdown/album 各输入先 transform/minify 后串接；禁用 bundle、source map、banner/footer 与模块包装。对 CSS 仅做压缩。
- [ ] 增加 `engines.node`、`build` script 和精确 `esbuild@0.28.2`，生成 lockfile。运行 `npm ci` 后，移除本地依赖或在无网络/无全局工具环境运行应证明 build 不会通过 `npx` 或全局 fallback；不把“是否由 npm ci 安装”作为脚本可检测状态。
- [ ] 以精确 `@playwright/test@1.62.1` 增加 `test:e2e:build` 脚本（显式 `--project=build-smoke`）和包含 `build-smoke`、`business` 两个 Chromium project 的 `playwright.config.mjs`；配置只读取 `LEANOTE_BASE_URL`、`LEANOTE_E2E_EMAIL`、`LEANOTE_E2E_PASSWORD`，不提供 webServer/凭据 fallback。用 `build-resource-smoke.spec.mjs` 从 manifest 驱动 32 个静态 URL 与五个页面的只读检查，并捕获 console/page/unhandled-rejection 错误；只写入 allowlisted 脱敏摘要，禁止 storage state、cookie、认证头、页面正文、截图、视频和原始 trace/report 发布，临时文件完成后清理。
- [ ] 运行构建单测，覆盖每个 JS/CSS 产物的存在、顺序、语法/结构和“测试不触碰 tracked 输出”。

## Task 3: Implement Template And i18n Contracts

**Files**
- Modify: `scripts/build/i18n.mjs`, `scripts/build/note-html.mjs`, `scripts/build/index.mjs`
- Test: `tests/js/build-pipeline.test.js`, `tests/js/fixtures/build/i18n-contract.json`

- [ ] 实现 marker-aware `note.html` 生成，精确执行 design 第 4 节的五步并固定 UTF-8 无 BOM/LF。为三个 dev marker/placeholder 数量错误、残留 marker、路径替换次数错误、script 顺序错误写失败用例。
- [ ] 实现固定扫描根、canonical-source 排除清单、静态 key/`msg:` 提取、message `.conf` 解析和 key 排序输出；排除全部 33 个 D 生成目标以及 `public/md/main-v2.min.js` 等 derived input，保留 TinyMCE 非 locale 上游文件，并固定 `msg -> member -> markdown -> album` 的覆盖顺序。词法扫描必须屏蔽字符串、注释和正则字面量，测试必须证明压缩副本不会产生第三个动态调用。
- [ ] 让输出语义与 i18n fixture 相等。历史缺失仅保留原 key fallback；新缺失报 key、locale、源文件和行号。除两个登记位置外，动态调用报 source locator 并失败。
- [ ] 运行 `node --test tests/js/build-pipeline.test.js` 与现有 `npm test`；两者必须通过，且后者仍发现 10 个 TinyMCE 粘贴测试。

## Task 4: Stage, Publish And Prove Failure Behavior

**Files**
- Modify: `scripts/build/index.mjs` and focused helpers/tests only
- Test: `tests/js/build-pipeline.test.js`

- [ ] 默认 build 先在受控 staging 完整生成和验证，再以同卷备份、逐文件原子替换并在失败时完整恢复旧输出；成功前不改真实目标，失败后也不得留下半套新输出。确保 staging/backup 在成功和失败后清理。
- [ ] 添加缺输入、重复/逃逸路径、i18n 解析/缺失/动态 key、模板 marker、转换、写入、发布和恢复失败的回归。每例断言非零退出、定位信息、没有旧输出 fallback，且旧输出集合被完整恢复。
- [ ] 运行 `npm run build` 生成全部 33 个输出，并用 `git ls-files --error-unmatch` 验证全部 manifest 输出已跟踪；确认没有 `markdown.min.js`、`all.css`、相册 `main.min.js` 或 TinyMCE payload 被误写。资源探针只对 32 个静态输出发 HTTP 请求，`note.html` 通过 `/note` 页面验证。
- [ ] 连续运行两次 `npm run build`；第二次运行 `git diff --exit-code` 为零，并验证 `git status --porcelain --untracked-files=all` 为空。

## Task 5: Switch CI And Documentation, Then Remove Gulp

**Files**
- Modify: `.github/workflows/regression-baseline.yml`, `AGENTS.md`, `CLAUDE.md`, `package.json`, `package-lock.json`
- Delete only after previous gates: `Gulpfile.js` and Gulp dependency entries

- [ ] 将现有 Node job 的固定顺序改为 Node/Go 工具链与 MongoDB 5.0 fixture、Leanote 服务启动和 readiness 等待，随后执行 `npm ci`、`npm run build`、`git diff --exit-code`、空 `git status --porcelain --untracked-files=all`、`npm test`、`npm exec -- playwright install chromium` 与 `npm run test:e2e:build`，最后用 `if: always()` 清理服务和 Mongo；不为 E 再造第二个 Playwright 依赖。
- [ ] 将 `AGENTS.md` 和 `CLAUDE.md` 的 Gulp/手工同步说明替换为 Node 24 `npm ci && npm run build && npm test`，说明 manifest/`note-dev.html` 是唯一来源、产物仍受跟踪且不得手改。
- [ ] 搜索仓库中的构建入口；删除 Gulpfile 与依赖后，不得保留可执行的 Gulp 命令或手工 bundle 同步指令。
- [ ] 从干净 Node 24 install 执行 `npm ci && npm run build && npm test`，并复跑漂移与未跟踪检查；在此之前不得删除旧 Gulp 文件。

## Task 6: Runtime And Regression Acceptance

- [ ] 在可用 MongoDB 5.0 fixture 下由 G 兼容 harness 启动既有服务，显式设置 `LEANOTE_BASE_URL`、`LEANOTE_E2E_EMAIL`、`LEANOTE_E2E_PASSWORD`，执行 `npm exec -- playwright install chromium` 与 `npm run test:e2e:build`；验证 `/note`、markdown 资源、`/album/index`、`/blog`、`/admin/index`、`/member/index`、manifest 32 个静态 URL、生成 `note.html` 和所有错误/资源门禁。只保留脱敏摘要（版本、URL、状态码、资源路径、错误类别），删除原始 trace/HTML report、截图/视频、storage state、cookie、认证头、页面正文和未脱敏服务日志；报告 artifact 保留期不超过 7 天。测试不得修改业务数据或生产输出。随后清理报告目录，再运行 Git 漂移检查。
- [ ] 运行 G 的既定 Golden、USN 与页面 smoke，不刷新 Golden 来吸收构建差异。环境不可用时明确阻塞，不能以未运行宣称通过。
- [ ] 运行 `npm test`、`git diff --check`、两次 build 漂移检查与全量 diff 复核，确认改动只包含构建链、生成物、CI 和文档；没有运行时库升级、业务重构、视觉修改、未声明产物或敏感配置。
- [ ] 完成上述证据后才可请求用户审阅最终规划并在后续明确批准后执行 `task.py start`。本任务此阶段不启动、不提交、不归档。

## Rollback Point

任务提交前任一契约失败都停在该任务分支：修正 manifest/生成器/测试或回到规格审核，不手改 bundle。任务提交后的回滚必须同时回退锁文件、构建脚本、已生成的 33 个声明输出、CI/文档和 Gulp 删除，恢复一个完整、可运行的旧构建链；不得只还原个别 minified 文件。
