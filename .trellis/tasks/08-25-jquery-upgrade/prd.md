# jQuery 3.7 升级（E-jQ）— PRD

父任务：`.trellis/tasks/08-25-frontend-libs`
前置任务：`.trellis/tasks/archive/2026-08/08-25-frontend-build-chain`（D，已完成）

## Goal

将 Leanote 浏览器运行时从 jQuery 1.9.x 升级到 **3.7.1**。用户继续使用同一套服务端渲染页面、公开静态 URL、全局 `$`/`jQuery` 契约、URL/API、笔记内容和博客主题；生产环境不加载 `jquery-migrate`、不保留双 jQuery 实例，也不把过时 API 包装成永久兼容层。

## Confirmed Facts And Dependencies

- 当前 ready 叶是本任务的规划整改子任务 `08-27-jquery-upgrade-spec-repair`。该子任务完成并归档后，本任务才能作为 D 之后首个可启动的 jQuery 实现任务；`task.json.meta.depends_on` 仅为已归档完成的 D。完成后才可解除 `08-25-bootstrap-upgrade` 的依赖。
- 当前 `scripts/build/manifest.mjs` 是 33 项默认生成物的唯一事实来源。`dep.min.js` 与 `album/js/main.all.js` 均直接输入 `public/js/jquery-1.9.0.min.js`；该文件同时被首页、登录、找回密码、admin/member、album、PDF 和 `note-dev.html` 引用。
- 博客主题经 `app/controllers/BlogController.go` 的 `jQueryUrl` 加载同一路径；`public/tinymce/plugins/image/dialog.htm` 也引用它。`leaui_image/index.html` 则加载本地 `public/js/jquery.js`，其内容为 jQuery 1.9.1。
- 父任务的不可变约束是保持公开静态 URL、服务端渲染、RequireJS/全局脚本契约和用户上传博客主题。故本任务不得删除 `/js/jquery-1.9.0.min.js` 这个公开 URL；“淘汰 1.9”指淘汰其内容和插件目录内的私有副本，而不是令该 URL 返回 404。
- `jquery-migrate` 最新 4.0.2 只适用于 jQuery 4；经包元数据验证，**3.6.0** 的 peer dependency 是 `jquery >=3 <4`，因此本任务诊断固定使用 3.6.0。
- `public/tinymce/tinymce.jquery*.js`、`jquery.tinymce*.js` 和 `leaui_mindmap/mindmap/main.js` 属于 TinyMCE 内核/脑图子应用；它们必须出现在资产盘点中，但运行时替换或重建由后续 E-TM 任务拥有，不能借本任务改动。

## Requirements

### R-jQ1: Single Runtime Asset Contract

- `package.json` 与 `package-lock.json` 必须精确锁定 `jquery` **3.7.1** 和仅供诊断/测试使用的 `jquery-migrate` **3.6.0**。两者是 Node 构建/测试输入，不是 Node 服务端运行依赖。
- manifest 必须将 `node_modules/jquery/dist/jquery.min.js` 作为唯一 jQuery 核心输入，并把现有公开 URL `public/js/jquery-1.9.0.min.js` 作为名为 `jquery-runtime` 的**生成输出**。文件名因 URL 兼容而保留，但其内容必须声明 3.7.1；不得新增另一个版本化公开 URL。
- 本任务可把 manifest 默认输出契约由 33 扩展为 34，并同步更新其单元测试、输入存在性检查、回滚检查、i18n derived-output 排除、Git 跟踪检查和 D 的资源 smoke 自动枚举。不得回写已归档 D 的规划工件。
- `dep` 与 `album` bundle 必须直接使用同一个 npm jQuery 输入，而非把生成的 `jquery-runtime` 再作为输入。各页面在一次导航/iframe 文档中只能执行一个 jQuery 核心；不同生成 bundle 中的同版本副本不构成双运行时。
- 删除 `public/tinymce/plugins/leaui_image/public/js/jquery.js` 的 1.9.1 副本，并令该 iframe 使用上述公开运行时 URL。所有现有模板、`BlogController` 变量和 TinyMCE image dialog 保持其既有 URL 字符串，从而继续解析到已替换的 3.7.1 内容。

### R-jQ2: Complete Compatibility Inventory And Ownership

- 在开始适配前，新增一份受跟踪的 jQuery 兼容性清单，逐项记录：资产/调用位置、加载页面或 iframe、所有者（第一方、具体第三方插件或 E-TM）、旧 API/诊断 warning、预期行为、修复方式和回归测试。
- 清单至少覆盖 `public/js/common.js`、`public/js/app/`、`public/js/plugins/`、admin/member/blog/album、所有引用 `/js/jquery-1.9.0.min.js` 的模板、博客主题、`leaui_image` iframe，以及 zTree、contextmenu、slimScroll、jquery-cookie、validation、fileupload、artDialog、qrcode、pagination、sortable。
- 第一方已知高风险调用包括 `.bind/.unbind/.delegate/.undelegate`、`.size()`、`$.parseJSON`、`$.trim`、`$.isArray`、旧 Deferred/AJAX 回调、`.data()` 属性解析、`:visible`、表单序列化和跨 iframe jQuery 对象。搜索结果不是完成证据：每项都必须有运行时诊断或针对性测试结论。
- 不属于本任务的 TinyMCE 内核、`jquery.tinymce` bridge 和 `leaui_mindmap` 必须在清单中标为 E-TM 所有；本任务不得把它们计入“生产无 1.9 核心”的结论，也不得修改它们来通过本任务测试。

### R-jQ3: Behaviour, Error And Data Compatibility

- 保持 `$` 与 `window.jQuery` 的单一全局实例、脚本相对顺序、RequireJS 模块名、AJAX method/URL/参数/响应解释、DOM 选择、事件委派、表单字段序列化、Deferred 成功/失败分支、`.data()` 的可见值和跨 iframe 调用语义。
- 已有公共 AJAX wrapper（`public/js/common.js` 的 `_ajax`/`ajaxGet`/`ajaxPost`/`ajaxPostJson`）在 HTTP 4xx/5xx 或解析失败时必须调用既有 `failureFunc`；没有该回调时保留已有明确可见的失败提示。选入 E2E 的第一方直接 `$.get`/`$.post` 调用也必须有可观察的失败分支，不能因迁移而静默完成、吞掉异常或只写日志。
- 写入型 business E2E 与 build smoke 只能消费仓库启动的 Revel `test` mode harness。harness 恢复 `leanote_test` 后生成每次运行唯一的密码学随机 `LEANOTE_E2E_RUN_TOKEN`，并在该数据库写入唯一、带创建时间的 `e2e_runs` marker（固定 run-kind、token 的 SHA-256 摘要）；再轮换 fixture 中配置为 admin 的账号密码并将账号名和随机密码仅传给同次运行的子进程。测试不得写死、推导或复用 fixture 默认密码，也不得依赖 GitHub Secrets；CI 必须先 mask 值并以临时 job 环境传递，因此 fork PR 与同仓 PR 使用同一隔离路径。
- 仅 test mode 的 loopback 服务可暴露只读 `GET /_test/e2e/identity`。handler 必须通过**当前应用的数据库会话**读取 `e2e_runs` marker，验证 marker 唯一、未过期（有效期为 `createdAt` 后 2 小时，2026-08-27 确认）且其 token 摘要与进程中的 run token 常量时间匹配，并从该会话取得实际 database 名。只有全部成立时才返回 `{runToken, database}`，其中 `database` 为 `leanote_test`；非 test mode 或非 loopback 一律 404，marker/数据库/摘要校验失败一律 503 且不得泄露 token、marker 内容或连接信息。handler 不得创建、刷新或删除 marker，也不得记录原始 token。
- `test:e2e:build` 与 business E2E 都必须经共享 helper `tests/e2e/e2e-environment.mjs`（build/business 复用同一实现）在任何登录前请求并严格比对身份响应；business E2E 还必须在创建、上传、删除或 route 注入前再次确认该预检已完成。缺少 `LEANOTE_BASE_URL`、`LEANOTE_E2E_EMAIL`、`LEANOTE_E2E_PASSWORD` 或 `LEANOTE_E2E_RUN_TOKEN`，服务不可达、接口非 200、字段不匹配、浏览器未安装或账号未认证均须立即失败。测试不得自行启动另一服务、使用默认凭据，或把任意可登录 URL 当作隔离环境。
- 会写入数据的流程只允许在隔离 fixture 和测试专用实体上执行；每个用例在 finally/fixture teardown 删除自身创建的笔记、附件、相册、博客评论和主题，harness 销毁数据库容器前还须删除 `e2e_runs` 标记。admin/member 流程必须在写入前验证登录账号可访问对应页面；member 区流程由同一已轮换的 admin 账号执行（2026-08-27 确认，选项 (a)），fixture 中的非 admin `demo` 账号不纳入轮换或使用。不满足权限或清理失败均失败，不得遗留共享数据库数据。

### R-jQ4: Migrate Is Test-Only Diagnostics

- `jquery-migrate` 不能出现在 manifest、`BUILD_OUTPUTS`、模板、静态目录、生产 bundle 或生产页面请求中。常规 `npm run build` 必须不读取或复制它。
- 诊断 E2E 必须从本地已锁定包读取 3.6.0，并以 Playwright 路由/临时诊断 bundle 在**同一文档**内紧随 jQuery 核心、先于任何应用或第三方插件脚本执行。诊断字节不得写回仓库或服务静态目录。
- 诊断必须断言所有测试页面和 iframe 流程的 `JQMIGRATE:` warning 数为零；生产 E2E 同时断言没有 `console.error`、`pageerror`、未处理 rejection、应用拥有资源的 4xx/5xx 或请求失败。不得用过滤 warning、关闭日志或永久 shim 达成通过。

### R-jQ5: Third-Party Adaptation Boundary

- 第一方调用优先直接现代化。第三方插件优先升级到可验证兼容 3.7 的上游版本，并在 lockfile 或受跟踪 provenance 中精确记录版本、来源和许可证；不得手工修补压缩运行时文件而不同步可读源和生产输入。
- 若某插件只能依赖生产 Migrate、第二个 jQuery、不可审计 fork 或会改变既有 UI/API，停止该插件分支并回到规划，给出可比较的替代方案；该情形不得静默降级或合并。

### R-jQ6: Required Business E2E Gate

- `.github/workflows/regression-baseline.yml` 的 `node-tests` job 必须在同一 test-mode harness、fixture、随机账号凭据和 `LEANOTE_E2E_RUN_TOKEN` 生命周期内，先运行 `npm run test:e2e:build`，再运行 `npm run test:e2e`，最后无条件停止服务并删除 MongoDB fixture。workflow 不得从 GitHub Secrets 读取 E2E 登录凭据；business E2E 必须在包括 fork PR 的 PR/push 上执行并阻断合并，不能以本地人工运行或 build smoke 成功替代。
- build 与 business E2E 必须分别输出 allowlisted 脱敏摘要和共享服务健康摘要；失败 artifact 最长保留 7 天，且不得包含 token、账号、cookie、storage、页面正文、trace、截图、视频、请求/响应头或未脱敏日志。workflow 的服务启动、run token 写入、两条 E2E 命令与 cleanup 属于本任务范围。

### R-jQ7: Browser Matrix Evidence

- Chromium business E2E 是 PR/push 阻断门禁。Chrome、Edge、Firefox、Safari 的当前及前一主版本是发布前 smoke：Chrome 和 Edge 必须使用各自真实浏览器二进制，Safari 必须在真实 Safari 环境；Playwright Chromium 不能替代 Chrome 或 Edge。
- 每个真实浏览器版本至少在同一 test-mode harness 中覆盖登录、笔记列表/搜索、笔记本/标签、对话框、一次上传并清理、相册、博客、admin/member 与 `leaui_image` iframe，并验证认证、`console.error`、`pageerror`、未处理 rejection 和应用拥有资源的网络失败门禁。该 smoke 不得跳过身份预检或数据清理。
- 每次 smoke 在受跟踪的 `docs/modernization/browser-smoke/jquery-3.7.md` 记录提交 SHA、执行日期、浏览器产品/完整版本、操作系统、覆盖页面/iframe、身份验证结果、错误门禁结果和通过/失败；不得记录认证材料、页面正文或用户数据。缺少任一产品或版本、或任一 smoke 失败，阻断本任务发布验收。

### R-jQ8: Scope, Compatibility And Downstream Contract

- 不升级 Bootstrap、TinyMCE、jQuery 4 或其他无关库；不进行视觉重设计、SPA/ESM 改造、后端 fallback、API/USN/数据库变更或历史笔记 HTML 重写。
- 保持公开 URL/API、HTTP 状态与重定向语义、认证、服务端模板结构、博客主题可注入脚本和未编辑笔记不保存的不变量。任何生成物必须只由 `npm run build` 产生；连续构建后的 Git 树必须无漂移。
- 本任务完成记录 jQuery 运行时、诊断和 E2E 契约；Bootstrap 仅在本任务验收全部通过后才开始，E-TM 保留其明确的 TinyMCE 资产所有权。

## Acceptance Criteria

- [ ] **AC-jQ1** `npm ci` 后 `npm ls jquery jquery-migrate` 解析为唯一的 jQuery 3.7.1 与 jquery-migrate 3.6.0，且 migrate 的 peer dependency 没有安装第二个 jQuery。
- [ ] **AC-jQ2** manifest 的 34 个输出均唯一、受 Git 跟踪、无路径逃逸；`jquery-runtime` 和 `dep`/`album` 均由 npm 的 jQuery 3.7.1 输入生成。隔离 build-tree 测试也包含该声明输入，缺失时在发布前显式失败。
- [ ] **AC-jQ3** `/js/jquery-1.9.0.min.js` 仍返回 200 且内容为 3.7.1；`dep.min.js`、`album/js/main.all.js` 和 `leaui_image` iframe 不再执行任何 1.9.x 核心。一个页面或 iframe 不会加载两个 jQuery 核心。
- [ ] **AC-jQ4** 受跟踪的兼容性清单覆盖 R-jQ2 的区域、每个实际 Migrate warning 和所有权排除项；第一方适配与第三方替换均有定位、行为说明和回归用例。
- [ ] **AC-jQ5** Node 静态契约测试证明生产 manifest/output/template 中没有 migrate、私有 1.9.1 iframe 副本或未声明的 jQuery 核心；`npm run build && npm run build && git diff --exit-code` 通过。
- [ ] **AC-jQ6** `npm run test:e2e:build` 与新的 `business` Chromium E2E 仅在 test-mode harness 的匹配 run token、由实际应用 DB 会话验证的唯一 marker、`leanote_test` 身份响应和已认证的随机化 fixture admin 账号下通过；build 与 business 两个 project 的身份预检均在任何登录前经共享 helper `tests/e2e/e2e-environment.mjs` 执行，business 流程在所有写入或 route 注入前再次确认。marker 缺失/重复/过期、摘要不匹配、数据库错误、非 test mode、非 loopback 或错误凭据均有 fail-closed 回归。业务流覆盖登录、笔记列表/搜索、笔记本/标签、对话框、上传、相册、博客、admin/member 以及 `leaui_image` iframe，写入用例无残留数据。
- [ ] **AC-jQ7** 诊断 E2E 的 `JQMIGRATE:` warning 为零；生产 E2E 的错误/网络断言为零。每类选入的第一方 AJAX wrapper 至少有一次受控 4xx/5xx/解析失败回归，证明既有失败回调或可见提示被触发而非静默吞掉。
- [ ] **AC-jQ8** `npm run build && npm test`、D 的资源 smoke、G 的 Golden/USN/page smoke、相关 Go/Node 定向测试均通过；最终 diff 不包含 Bootstrap/TinyMCE 升级、永久兼容层、后端兼容分支或视觉重设计。
- [ ] **AC-jQ9** Chrome、Edge、Firefox 和真实 Safari 的当前及前一主版本 smoke 按 R-jQ7 的受跟踪记录完成；Chromium E2E 是本任务 PR/push 合并阻断门禁。
- [ ] **AC-jQ10** 包括 fork PR 的 PR/push workflow 在同一 test-mode harness 中先后运行 build smoke 与 `npm run test:e2e`；账号与 run token 均由 harness 随机生成、mask 后仅传给本次 job，workflow 不读取 E2E GitHub Secrets。business E2E、身份预检、权限预检、数据清理或 harness cleanup 任一失败均使 workflow 失败，artifact 只含脱敏摘要。

## Out Of Scope

- jQuery 4、删除 jQuery、SPA/原生 DOM 重写、Bootstrap/TinyMCE 升级或 UI 重设计。
- E-TM 所有的 TinyMCE 内核、`jquery.tinymce` bridge 与脑图独立应用的改造。
- 新增生产静态 URL、移除 `/js/jquery-1.9.0.min.js` URL、生产 Migrate/双实例/fallback，或依赖未隔离的生产测试数据。

## Open Questions

无。版本选择、URL 兼容、测试服务前置条件和后续任务所有权均由父任务、D 的 E2E 契约和现有代码证据确定。
