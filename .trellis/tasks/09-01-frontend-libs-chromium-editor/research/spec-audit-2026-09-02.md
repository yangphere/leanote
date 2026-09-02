# B-E3 规格审核与根因核实（2026-09-02）

## 结论

三个 CI 失败已全部定性：两个为可本地复现的普遍缺陷（测试 shell 缺事件 API；index.html 写死 `md` 类隐藏属性面板），一个为 CI 环境级联（本地通过）。原 PRD 的方向性假设（"真实运行时兼容错误…而不是仅修测试 mock"）经证据修正：**生产插件的 TinyMCE 8 API 用法兼容**，正确修复是测试 shell 保真 + 恢复对话框的条件类原设计。已重写 PRD；无需用户裁决项。审核只改任务规格与研究材料。

## Ready Selection Evidence

B-E1、B-E2 均已归档；B-E3 `meta.depends_on=[build-mode, mongo-harness]` 满足，是唯一 ready 叶（B-E4 依赖 B-E1+E2+E3）。

## 缺陷证据（CI + 本地双源）

CI：[run 33579336426 / chromium-e2e job `100090357472`](https://github.com/yangphere/leanote/actions/runs/33579336426/job/100090357472)（`d37e3b8` 谱系）：build-smoke 1/1 过；business **19 过 / 3 败**（8.7m）。

本地复现（2026-09-02，经 B-E2 supervisor `go run ./app/tests/harness/cmd/e2e -- npm run test:e2e`，supervisor 预检/容器/teardown 全链路正常）：business **20 过 / 2 败**（5.7m），`leanote-test-mongo` 独占并清理成功。

| # | 失败测试 | 错误 | 本地 | 定性 |
|---|---|---|---|---|
| 1 | `business-flows.spec.mjs:84` leaui_image 契约 | `page.evaluate: TypeError: editor.on is not a function`，栈帧 plugin.js:152 | 复现 | 普遍：测试 shell 缺事件 API |
| 2 | `business-flows.spec.mjs:162` 大业务流 | `locator.fill('#attrTitle')` "element is not visible" 至 300s（spec:470；此前 468/469 的 enabled/value 断言已通过） | 复现（同点位 spec:470） | 普遍：`.md` 类写死隐藏属性面板 |
| 3 | `editor-flows.spec.mjs:69` | 主体通过，`finally` 清理 `deleteNote` POST 处 180s 预算耗尽 | **通过** | CI 环境级联（预算/时序），待 1+2 修复后 CI 复验 |

CI 另有 `[business-e2e] cleanup failed: …Test timeout` 系列——均为失败 2 超时后 Playwright 上下文销毁的次生产物，非独立缺陷。

## 根因 1：测试 shell 缺 TinyMCE 事件 API（失败 1）

- `business-flows.spec.mjs:121-148` 构造受控 editor shell（仅 selection/getContent/dom/insertContent/ui.registry/windowManager），调用真实插件工厂 `window.__pluginFactory(editor)`。
- 插件工厂体 `plugin.js:152` 调用 `editor.on('dragstart', …)`（注册按钮的 `onSetup` 内 `editor.on/off`（:135/:137）在 shell 下不被触发，因 shell 的 addButton 只存配置）。shell 无 `on` → TypeError。
- **生产侧无兼容问题**：本仓锁定的 TinyMCE 8.8.2 typings 中 `Editor implements EditorObservable → Observable` 提供 `on/off/fire`（tinymce.d.ts:2953/:2680）；5-7 为历史同一 API 惯例但未在本仓验证；真实页面证据——editor-flows 用真实 `window.tinymce.get('editorContent')` 加载含 leaui_image 的真实编辑器并跑通全部断言（本地），bootstrap 系列与 [20/22] 亦过。
- 正确修复：shell 补最小事件 API（on/off 存储 + 触发），测试可顺带断言 onSetup 的订阅/退订。

## 根因 2：`index.html` 写死 `md` 类（失败 2，生产回归）

- `public/tinymce/plugins/leaui_image/public/css/style.css:220-227`：`.md { height:350px; overflow:hidden } .md #previewAttrs { display:none }`。
- `index.html:21` `<body class="md" id="body">` 硬编码；同文件注释掉的原始逻辑为 **URL 含 `md=1` 才加类**。
- 既有契约佐证：markdown 编辑器打开的是另一页面 `/album/index?md=1`（public/js/markdown-v2.min.js、public/md/main-v2.js:16981）；TinyMCE 插件 `openAlbum` 打开 `leaui_image/index.html?<ts>` **无 `md=1`**（plugin.js:115-118）。
- 后果：富文本编辑器的图片对话框属性面板（title/width/height/constrain）被隐藏——真实生产回归；测试在 standalone 页正确固化了"非 md 模式显示属性面板"契约（点击 `#preview li` 后 `initAttr`（public/tinymce/plugins/leaui_image/public/js/main.js:675-695）已启用并赋值输入框，故 enabled/value 断言过、fill 因不可见超时）。
- 正确修复：恢复条件类（`md=1` → 加 `md`），standalone 与富文本对话框显示属性面板；markdown 的 `/album/index` 不受影响。
- 溯源：`class="md"` 硬编码来自上古上游提交（git -S 命中 721e375d/68185746），非升级任务引入；E-TM 归档时该验收项 checkbox 本就未勾（implement.md:92），即 B-E3 立项时已知残留。

## 根因 3：CI 级联（失败 3）

editor-flows 本地全绿（含 180s 预算内的全部 poll 与清理 POST）。CI 上失败 2 耗尽 300s 且其清理被上下文销毁波及；editor-flows 随后在 CI 慢机上（business 8.7m vs 本地 5.7m）预算吃紧。判定：非独立缺陷；协议=修复 1+2 后 CI 复验，若仍失败再单独诊断，不得以放宽超时/忽略退出码掩盖。

## Audit By Requirement Dimension

- **方向修正（D1，关键）**：原 Req 1 假设"真实运行时兼容错误"，禁止"仅修测试 mock"。证据表明运行时兼容、失败 1 纯属 shell 保真缺口；正确修复恰是补 shell（并断言事件 API 边界）+ 恢复生产对话框条件类。已按证据重写，避免实现者反向"修插件"或逃避测试修复。
- **范围澄清（D2）**：Req 2 的"会员博客入口"等大覆盖已由既有 22 用例承载（本地 20/2 中的 20 个全过）；本任务不新增覆盖面，聚焦恢复全绿。
- **失败 3 协议（D3）**：新增"修复后 CI 复验，不放宽超时"的明确条款，防掩盖。
- **验收可测性（D4）**：AC 绑定 CI job 的 22/22 + 发现数 + 清理结果，本地运行作工程证据；`editor.on` 零出现 + `#attrTitle` 流程通过作为具体断言。
- **元数据（D5）**：task.json `base_branch: master` → `dev`（同前例）。

### 评审后补录（2026-09-02 code-review 双轴）

- 引用修正：body 标签行号 22→21；style.css/main.js 补全嵌套路径（二者不在 index.html 旁）；"TinyMCE 5-8 均有 on/off"收窄为本仓 8.8.2 typings 证据。
- PRD Req 5 的"md 上下文仍隐藏"断言补入 design §3 与 implement Task 2（以 business 测试承载：`index.html?md=1` 下 `#previewAttrs` 隐藏 + body 类条件成立）。
- 删除无证据依据的可选 Node 静态断言分支（Speculative Generality）；PRD Req 1/4 增补 design §指针；Req 4 收敛为约束+指针。
- 原 PRD AC3（未编辑零写入/revision/HTML 语义）降级为 Req 3 正文系有意留痕：该三项由 22/22 中的 editor-flows 既有断言承载，不再单列 checkbox。

## 无需用户确认的判断

shell 补事件 API vs 换真编辑器（选前者：契约测试本旨是"只实现插件公开用到的 API"的边界验证）；条件类恢复 vs CSS 改动（选前者：忠于原设计、markdown 页面零影响）；失败 3 不预修（本地证据充分）。

## 审核过程 provenance

- `gh run view 33579336426 --job 100090357472 --log`：三失败的测试名、错误行、调用栈、清理产物。
- 本地 supervisor 复现：`go run ./app/tests/harness/cmd/e2e -- npm run test:e2e`（输出含 [n/22] 序列与 2 failed 明细）。
- 通读 `plugin.js` 全文、`index.html` 全文、`style.css` 的 `.md`/媒体查询规则、`main.js` 的 `initAttr`/点击绑定、`business-flows.spec.mjs:84-160/440-500`、`editor-flows.spec.mjs:20-145`。
- `git log -S 'class="md"'` 溯源；归档 E-TM `implement.md:92` 未勾验收项核对。
- 未修改任何业务实现、测试、构建脚本；未激活任务（待用户批准）。
