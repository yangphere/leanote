# 前端构建链现代化（D）— 技术设计

## 1. 文件结构

- `scripts/build/manifest.mjs`：所有 bundle/CSS/i18n/note HTML 的输入顺序与输出路径。
- `scripts/build/js.mjs`：按 manifest 顺序读取全局脚本并用 esbuild transform/minify，保持非 ESM 形态。
- `scripts/build/css.mjs`：拼接和压缩 CSS。
- `scripts/build/i18n.mjs`：复现 `getMsg('key')` 扫描与语言文件生成。
- `scripts/build/note-html.mjs`：从 `note-dev.html` 确定性生成 `note.html`。
- `scripts/build/index.mjs`：统一入口、缺失文件检查、错误汇总和非零退出。
- `tests/js/build-pipeline.test.js`：manifest、顺序、转换与幂等回归。

`package.json` 只暴露 `build` 与必要的分项调试命令；CI 和交付统一调用 `npm run build`。

## 2. JavaScript 策略

现有业务代码依赖 script 顺序、全局变量和 RequireJS，不做模块化改造。脚本按 manifest 顺序读取、拼接，再通过 esbuild `transform` 的目标浏览器配置做语法处理和压缩；禁止 esbuild 自动重排跨文件副作用。

每个 output 的输入数组只定义一次。`dep.min.js` 等历史名称与 URL 保持不变，避免模板和插件同时迁移。

## 3. i18n 与 note HTML

i18n 扫描只接受静态 `getMsg('literal')`，动态 key 记录明确错误或由 manifest 的显式 key 列表补充，不静默遗漏。输出按 locale 和 key 稳定排序。

`note-html.mjs` 复现 Gulp 的 dev block 删除、生产 bundle 替换、TinyMCE 路径和插件路径替换，但不再靠连续多次空行 replace。换行符固定为仓库约定，结果由快照测试保护。

## 4. 漂移门禁

普通 `npm run build` 写入受跟踪产物。CI 从干净 checkout 执行构建后运行 `git diff --exit-code`；旧产物不能作为失败 fallback。两次连续构建验证无时间戳、随机顺序和平台路径漂移。

## 5. 回滚

新旧流水线在任务分支内可短暂并存用于产物对照，任务完成时必须删除 Gulp。运行时库版本不变，因此回滚只涉及 package/lock、构建脚本与重新生成的资源。
