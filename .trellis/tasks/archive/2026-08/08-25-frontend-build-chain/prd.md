# 前端构建链现代化（D）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在不改变 Leanote 服务端渲染页面、公开静态 URL、现有前端库版本、全局脚本语义或 i18n 回退语义的前提下，用 Node 24、锁定的 esbuild 与普通 Node 脚本替换无法在现代 Node 安装的 Gulp 3。新链必须从受控源码确定性生成当前运行时资源以及明确保留的旧 Gulp 产物，并让一次构建即可恢复所有该任务拥有的生成物。

用户价值是消除手工同步可读源码和预构建文件的风险，同时仍支持直接检出运行和可审计发布。

## Confirmed Baseline

- `08-25-frontend-build-chain` 是前端轨道首个叶任务；其唯一 `meta.depends_on` 为已完成归档的 `08-25-regression-baseline`。它完成后才允许 `08-25-frontend-libs` 与 jQuery 子任务开始。
- 当前 `package.json` 仅有 `npm test`，依赖为 Gulp 3 系列，既没有 `build` script，也没有 `package-lock.json`。本机 Node 24.19.0 下 `npm test` 已发现并通过 10 个 TinyMCE 粘贴回归用例。
- 旧默认 Gulp 入口仅运行 `concat`、`plugins`、`minifycss`、`i18n`、`concatAlbumJs` 与 `html`。`concatMarkdownJs`、`concatCss` 和 `tinymce` 不是默认路径；其中 TinyMCE task 还硬编码了开发者绝对目录，因而不是可复现的仓库构建输入。
- `note-dev.html` 是编辑器页面模板源，`note.html` 是生成物。实际生产页面加载 `dep.min.js`、`app.min.js`、`markdown-v2.min.js`、`plugins/main.min.js` 和已有 TinyMCE payload；相册页面加载 `album/js/main.all.js`。
- 旧 i18n 扫描覆盖 `public/{admin,blog,md,js,album,libs,member,tinymce}` 与 `app/views` 中的 `.js`/`.html`。审计发现各 locale 对静态候选键有 6--14 个历史缺失，且 `public/js/common.js:1158` 与 `public/md/main-v2.js:17417` 存在动态 key 调用；现有 `getMsg` 对未收录 key 返回原 key。
- `public/js/markdown.min.js` 已受跟踪但没有当前模板引用；`public/md/themes/all.css` 与 `public/album/js/main.min.js` 均不存在，且分别来自已注释/Deprecated 的旧任务。`public/tinymce/langs/en.js`、`zh.js` 与 `readme.md` 是保留的 TinyMCE 上游资源，不是 locale 生成器的输出。

## Dependencies And Boundaries

- 依赖 `08-25-regression-baseline`，可与后端轨道并行。
- D 仅拥有构建工具、manifest、构建测试、生成物、构建相关 CI 与构建说明。D 可以扩展现有 workflow 的 `node-tests` job 为 `npm ci`、build、漂移检查、test；最终 CI/CD 编排仍由 F 拥有。
- D 拥有最小的 Chromium Playwright 构建资源 smoke 入口；E 及其三个子任务复用该入口和锁定的 Playwright 基础设施，扩展为完整业务 E2E，并负责 Firefox/Safari 发布 smoke。D 不升级、删除或重建 TinyMCE 核心/插件 payload。

## Requirements

### R-D1 工具链、依赖和入口

- `package.json` 必须声明 `engines.node: ">=24 <25"`、`build` script 与锁定的 `esbuild` 0.28.2；提交由 Node 24 npm 生成的 `package-lock.json`。干净 checkout 的生产验证必须先执行 `npm ci`；构建本身必须在本地锁定的 `node_modules/esbuild` 缺失或版本不是 0.28.2 时显式失败，不能借助 `npx`、全局 esbuild 或网络下载临时成功。构建不声称能够从进程状态判断某次安装是否由 `npm ci` 完成。
- `index.mjs` 必须在读取 manifest 前解析 `process.versions.node`，对 `<24` 或 `>=25` 以检测到的版本和支持范围非零退出；不得只依赖 npm 的 advisory `engines` 警告。该拒绝路径与 Node 24 成功路径均有单测。
- `npm run build` 是唯一生产生成入口。允许只读/写入临时目录的测试参数，但不得提供绕过 manifest、部分构建或复用旧产物的成功路径。
- `package.json` 必须以精确版本声明 `@playwright/test` 1.62.1，并提供 `test:e2e:build` 脚本调用仓库内的 `playwright.config.mjs`。该入口只包含 Chromium 构建资源 smoke；浏览器二进制必须由本地锁定的 Playwright CLI 显式安装（`npm exec -- playwright install chromium`），不得通过全局 Playwright 或隐式下载成功。E 的完整业务 E2E 可以另设脚本，但必须复用该依赖和配置基础设施，不得引入第二份 Playwright 版本。
- 删除 `Gulpfile.js`、所有 Gulp 依赖及其文档入口必须是最后一步：只有新链在干净安装、两次连续构建、CI 漂移门禁和页面冒烟均通过后才允许删除。

### R-D2 Manifest、路径和运行时产物

- `scripts/build/manifest.mjs` 是 D 所有生成输出、输入顺序、转换种类、扫描根、运行时 URL 和例外的唯一机器可读事实来源；manifest 必须展开列出全部 33 个默认输出，禁止用未审计的 glob 代替输出清单，其他构建模块不得复制数组或输出路径。
- manifest 的输入与输出均为仓库根相对、规范化的 POSIX 路径；禁止绝对路径、`..`、符号链接逃逸、重复规范化输出和未声明写入。构建在读取前验证所有输入，在发布前验证所有输出唯一且位于 manifest 允许范围。
- 以下 12 个受跟踪构建产物必须由默认 build 生成，且路径、既有加载 URL（对当前未引用的保留产物则保持可服务路径）、相对顺序和全局作用域保持不变；其中 `public/md/themes/default-min.css` 是旧 Gulp 保留产物，当前 `note.html` 仍引用同目录的未压缩 `default.css`，不得把两者混称：
  - `public/js/dep.min.js`、`public/js/app.min.js`、`public/js/plugins/main.min.js`、`public/js/markdown-v2.min.js`；
  - `public/album/js/main.all.js`、`public/album/css/style-min.css`；
  - `public/css/bootstrap-min.css`、`public/css/font-awesome-4.2.0/css/font-awesome-min.css`、`public/css/zTreeStyle/zTreeStyle-min.css`、`public/md/themes/default-min.css`、`public/js/contextmenu/css/contextmenu-min.css`；
  - `app/views/note/note.html`。
- 默认 build 还必须生成 7 个 locale 的 `public/js/i18n/msg.<locale>.js`、7 个 `blog.<locale>.js` 和 7 个对应的 `public/tinymce/langs/<locale>.js`，locale 固定为 `de-de`、`en-us`、`es-co`、`fr-fr`、`pt-pt`、`zh-cn`、`zh-hk`。
- `markdown.min.js` 仅作为已跟踪历史资源保留，`all.css`、相册 `main.min.js` 与外部绝对路径 TinyMCE task 不纳入默认产物；D 不得为了“覆盖旧任务”新增这些不存在的输出，或删除 `tinymce/langs/en.js`、`zh.js`、`readme.md`。

### R-D3 JavaScript、CSS 与模板兼容

- JS 构建按 manifest 中的旧 Gulp**生产**顺序处理：`dep.min.js` 保持旧任务的原样串接（该任务没有 `uglify`），`app`、plugins、markdown-v2 与 album bundle 则对每个输入先分别执行 esbuild transform/minify，再按数组顺序串接。所有 bundle 都禁止 bundle 模式、模块包装、跨文件副作用重排、banner/footer、sourcemap 和运行时库升级；`app.min.js` 以 Gulp 的生产数组为准，不能把 `note-dev.html` 中不同的开发直接加载顺序偷偷写回生产输出。
- CSS 只压缩 manifest 列出的 6 个输入到既有路径；不得改写 URL、字体、主题选择、选择器语义或生成额外 source map。未由旧默认任务生产的主题 CSS 不是 D 的生成目标。
- `note-html` 只允许进行已枚举的转换：移除当前模板中恰好三个成对 `<!-- dev -->` 块；各替换一次四个生产占位符；切换 TinyMCE 与插件的生产脚本路径；移除唯一的 `console.log(o);`。旧 Gulp 的 `console.trace(o);` 替换在当前模板为零次匹配，必须保持允许的 no-op，不能凭空加入或删除其他调试代码。每个 marker 的数量、替换后不存在性和生产 script 顺序均须校验，不匹配即非零退出；除固定换行、去除行尾空白和修正 space-before-tab 外，不得进行宽泛 regex、HTML 美化或修改 Revel 模板表达式。
- 输出均使用 UTF-8 无 BOM 与固定 LF；同一 checkout 在 Windows 与 Linux 上连续两次构建的字节必须相同。

### R-D4 i18n 与 TinyMCE 语言契约

- i18n 扫描根、`.js`/`.html` 后缀、静态 `getMsg` 与 `msg:` 提取规则以及 canonical-source 排除清单必须在 manifest 中显式声明。扫描必须排除 D 的全部 33 个生成输出（包括已有 bundle 和 `note.html`），并额外排除已被 canonical source 覆盖的旧压缩输入 `public/md/main-v2.min.js`；该文件仍可作为 markdown bundle 的构建输入，但不能作为 i18n 源再次扫描。未生成的 TinyMCE 上游资源仍按旧扫描根参与审计。
- 实现前从当前受跟踪的 21 个 locale 输出建立 `tests/js/fixtures/build/i18n-contract.json`：按 locale、`msg`/`blog` namespace 与 TinyMCE 语言对象记录解析后的 key/value 映射、完整静态 key 清单（含 namespace、源文件和行号）、已知动态调用定位，以及每个 namespace 的历史缺失 key 集合。新生成内容必须与此结构语义相等；对象序列化顺序可因稳定排序变化，但键和值不得变化。
- 既有缺失 key 必须继续令浏览器 `getMsg` 返回原 key，不得补造翻译、把所有配置键塞入 bundle，或让历史缺失阻断构建。新增加的静态 key 若未在每个应有 namespace/locale 中定义且未登记于该基线，构建必须以 key、locale、源文件和行号失败。
- 仅 `public/js/common.js:1158` 与 `public/md/main-v2.js:17417` 的历史动态调用可保留，并由基线夹具覆盖其可见结果。任何新增或移位的动态调用都必须显式登记并先回到规划审核；不能静默忽略。字符串、注释和正则字面量中的文本不得被识别为调用。
- D 仅从 `messages/<locale>/{msg,member,markdown,album,blog,tinymce_editor}.conf` 生成上述语言资源；`msg` namespace 的合并顺序固定为 `msg`、`member`、`markdown`、`album`，重复键后者覆盖前者，`blog` 与 `tinymce_editor` 各自独立。解析规则固定为现有 legacy 配置语义（首个 `=` 分隔、保留键和值的既有字符内容、重复键最后写入），非注释且无分隔符的行必须以文件和行号失败。保留 TinyMCE 核心、插件和 `en.js`/`zh.js` 上游语言文件原样，升级它们属于 E-TM。

### R-D5 确定性、失败原子性和 Git 状态

- 构建先在仓库内受控临时 staging 目录完整生成并完成验证，之后才发布 manifest 输出。发布阶段必须在同卷备份旧输出，再逐个原子替换；任一读取、解析、转换、写入或发布失败都要删除新文件并恢复整套旧输出，返回非零、指出输入/输出与原始错误，并清理 staging。不得以“单文件 rename 成功”掩盖多文件半套发布、静默跳过或把旧文件当作成功结果；若恢复本身失败，必须同时报告原始错误和恢复错误并阻断验收。
- 所有默认输出必须已由 Git 跟踪。测试与 CI 除 `git diff --exit-code` 外，还要验证 manifest 每个输出可由 `git ls-files --error-unmatch` 找到，并在干净 checkout 的 build 后以 `git status --porcelain --untracked-files=all` 为空证明没有未跟踪漂移。
- 连续两次 `npm run build` 的第二次必须零 diff；修改任一 manifest 输入后，只运行该命令即可更新其声明的输出，且不得触碰未声明产物。

### R-D6 运行时、文档和交付边界

- 本任务不升级 jQuery、Bootstrap、TinyMCE 或其他运行时库，不把历史全局脚本改为 ESM/SPA，也不进行 UI 或 CSS 视觉重设计。
- 更新 `AGENTS.md` 与 `CLAUDE.md` 中的前端构建命令、生成物来源和 `note-dev.html` 规则，删除“手工同步/运行 Gulp”的建议；不得保留第二个有效构建入口。
- 保持服务端 URL、`/api/*`、USN、认证、页面模板组织、RequireJS 模块名、全局变量及用户上传博客主题不变。构建差异、静态资源缺失或浏览器异常必须暴露为失败，不能增加后端兼容分支或 fallback。

## Acceptance Criteria

- [ ] **D-AC1** Node 24 干净环境中 `npm ci && npm run build && npm test` 成功；无 Gulp、`gulp-util`、`npx gulp`、全局工具或联网 fallback 参与构建。
- [ ] **D-AC2** manifest 单独列出 R-D2 的 12 个运行时输出、21 个 locale 输出、输入顺序、转换和扫描根；所有输入存在、输出唯一、输出受 Git 跟踪，且不存在路径逃逸。
- [ ] **D-AC3** 每个 JS/CSS 输出通过语法/结构回归；`dep`、`app`、plugins、markdown-v2 和 album 的输入顺序与旧 Gulp 生产数组一致，未引入 bundle 包装、source map、运行时库版本变化或未声明输出。
- [ ] **D-AC4** `note.html` 通过模板快照和 marker 契约：三个 dev block 与四个生产 marker 精确处理一次，生产 TinyMCE/插件/四个 bundle 的 URL 与顺序正确，Revel 表达式及非允许转换内容语义保持不变；仅允许固定换行、去除行尾空白和修正 space-before-tab，以通过 `git diff --check`。
- [ ] **D-AC5** 21 个 i18n/TinyMCE locale 输出与 `i18n-contract.json` 的解析结构相等；扫描排除全部 33 个生成输出及 `public/md/main-v2.min.js` 等已声明 derived input，canonical `public/js/common.js:1158` 与 `public/md/main-v2.js:17417` 是登记的动态调用例外；历史缺失与这些动态调用仍按旧运行时语义工作；新缺失或未登记动态调用给出定位明确的非零失败，字符串、注释和正则字面量中的伪调用必须忽略。
- [ ] **D-AC6** 缺失输入、重复/逃逸输出、无效 i18n、写入/发布失败和模板 marker 异常都有聚焦回归测试，且不会把旧产物或半套新产物伪装为成功。
- [ ] **D-AC7** `npm run build && npm run build` 后 `git diff --exit-code` 为零；在干净 CI checkout 中 build 后 `git diff --exit-code` 和 `git status --porcelain --untracked-files=all` 均为零。
- [ ] **D-AC8** 现有 workflow 的 Node job 在 Node 24 上执行 `npm ci`、build、漂移门禁与 `npm test`；G 的 replay/USN/page smoke 继续通过。
- [ ] **D-AC9** `npm ci` 后使用仓库内 `@playwright/test` 1.62.1 的 `npm exec -- playwright install chromium` 安装浏览器，并运行 `npm run test:e2e:build`（显式选择 `build-smoke` project）。调用方必须先由 G 兼容 harness 启动 MongoDB 5.0 fixture 和真实服务，并显式提供 `LEANOTE_BASE_URL`、`LEANOTE_E2E_EMAIL`、`LEANOTE_E2E_PASSWORD`；缺少任一变量或服务不可达必须失败，不得自动启动另一套服务或使用默认凭据。Chromium smoke 登录后只读检查 `/note`（含 markdown 资源）、`/album/index`、`/blog`、`/admin/index`、`/member/index`，按 manifest 的 output→runtime URL 映射逐一 GET 32 个静态资源并确认 `/note` 使用生成的 `note.html`；静态资源全部 200，`console.error`、`pageerror`、未处理 rejection 和 owned-resource 网络 4xx/5xx 均失败。测试只生成不含页面正文、请求/响应头、cookie、认证 token、storage state、截图或视频的脱敏摘要；原始 trace、HTML report 和未脱敏服务日志不得上传，CI artifact 保留期不超过 7 天。测试不得写入笔记、附件、主题或生产输出。E 复用 `business` project 承接完整业务 Chromium E2E，Firefox/Safari smoke 仍由 E/F 的发布门禁负责。
- [ ] **D-AC10** `Gulpfile.js` 与 Gulp 依赖已删除，`AGENTS.md`、`CLAUDE.md` 已只记录 Node 24 构建入口；`git diff --check` 通过，diff 不含运行时库升级、业务模块化或视觉重设计。

## Out Of Scope

- jQuery、Bootstrap、TinyMCE 或其他浏览器运行时版本升级；TinyMCE 核心/插件打包和 `en.js`/`zh.js` 上游语言资源更新。
- 完整业务 E2E、Chrome/Edge/Firefox/Safari 矩阵或 UI 重设计；D 只提供 Chromium 构建资源 smoke 基础设施，完整套件由 E 协调。
- 恢复 Deprecated 的 `all.css`、已注释的 `markdown.min.js`/相册 `main.min.js` 生成，或执行带绝对路径的旧 TinyMCE task。
- SPA、ESM 业务模块化、后端、URL/API、USN、数据库、认证、用户内容和上传主题变更。

## 已确定的执行边界

- Playwright 入口由 D 提供但不拥有服务编排：`test:e2e:build` 只消费 G 兼容 harness 提供的已运行服务和测试账号，变量缺失、Mongo/HTTP 不可用、浏览器未安装或 smoke 失败均阻断验收。该约束消除了“仓库没有现成入口”与“D 是否需要第二套 E2E”的歧义；E 直接复用 `@playwright/test`、配置和 Chromium 安装步骤扩展完整业务流程。
