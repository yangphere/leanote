# B-E3 技术设计 — Chromium 编辑器两处失败修复

依据：`prd.md`（需求与验收）、`research/spec-audit-2026-09-02.md`（双源证据）。本文件只写"怎么做"与取舍。

## 1. 改动面

| 文件 | 改动 | 性质 |
|---|---|---|
| `tests/e2e/business/business-flows.spec.mjs` | 契约测试 shell 补事件 API；补两处断言 | 测试 |
| `public/tinymce/plugins/leaui_image/index.html` | `md` 类改为 URL `md=1` 条件应用 | 生产（对话框） |
生产 `plugin.js` 零改动（其 API 用法经真实编辑器验证兼容）。

## 2. shell 事件 API（失败 1）

契约测试的 shell 宗旨不变："只实现插件公开用到的 API"。事件 API 是插件真实依赖的边界，因此 shell 补：

```text
listeners = {}                                  // 以空格分隔的事件名 → handler 数组
editor.on(names, handler)  → 注册（按名拆分存储）
editor.off(names, handler) → 退订
```

- 断言一：工厂调用后 `dragstart` 已有订阅（插件 :152 真实执行）。
- 断言二：手动调取 `window.__button.onSetup({ setEnabled() {} })`，验证 `NodeChange ModeChange` 已订阅、返回的退订函数调用后订阅清空（覆盖插件 :133-138 的 onSetup 契约）。
- 取舍：不引入完整 EventDispatcher（fire/once 等）——插件未用到，超出 shell 宗旨；不换真编辑器——契约测试本要验证"仅依赖公开 API"的边界。

## 3. 对话框条件类（失败 2）

`index.html` 原始注释逻辑的等价恢复：

```html
<body id="body">
<script>
if (location.href.indexOf('md=1') >= 0) document.body.className = 'md';
</script>
```

- 非 `md=1`（standalone、TinyMCE `openAlbum` 的 `index.html?<ts>`）：`.md` 的 350px/overflow/隐藏规则全部不生效，`#previewAttrs` 正常显示，`initAttr`（main.js:675）点击启用后可编辑——测试 spec:467-479 的交互链成立。
- `md=1`：行为与现状一致（`display:none`）。当前无调用方对本页传 `md=1`（markdown 走 `/album/index?md=1`），条件分支为原设计保留。
- 回归断言（business 测试承载）：非 md 上下文断言 `#attrTitle` 可见可填；随后导航 `index.html?md=1` 断言 body 类为 `md` 且 `#previewAttrs` 隐藏——条件分支双向都被覆盖。
- 风险与回滚：改动独立于插件/编辑器状态机，revert 即回到现状。

## 4. 失败 3 复验协议（执行序）

1. 本地：`go run ./app/tests/harness/cmd/e2e -- sh -c 'npm run test:e2e:build && npm run test:e2e'` → 预期 1+22 全绿（复现基线：本地 20/2 → 修复后 22/22）。
2. 提交推送 → CI chromium-e2e job 全绿（含 build-smoke 先行、发现/执行数、清理摘要）。
3. 若 editor-flows 仍失败：收集该用例的逐步耗时（Playwright trace/step durations）与服务器侧日志定位，作为独立事实登记——禁止调大 `test.setTimeout` 或忽略退出码。

## 5. 兼容与不变量

- 不动 22 用例的任何既有断言与超时；不动身份预检/所有权门禁。
- `.md` 媒体查询（≤700px 隐藏规则）不受影响——CI/本地视口均大于阈值；`plugin.js`/`plugin.min.js` 零改动，无 bundle 同步事项。
