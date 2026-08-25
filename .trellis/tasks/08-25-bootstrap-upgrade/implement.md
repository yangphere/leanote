# Bootstrap 5.3 升级（E-BS）— 执行计划

## Global Constraints

- 目标固定 Bootstrap 5.3.8。
- 保持页面结构、URL、文案和功能；不做视觉重设计。
- 不使用永久 Bootstrap 3 adapter，不升级 TinyMCE core。

### Task 1：建立模板/交互基线

**Files:**
- Create: `tests/e2e/bootstrap-components.spec.js`
- Create: `.trellis/tasks/08-25-bootstrap-upgrade/research/bootstrap3-usage.md`
- Read: `app/views/`、`public/css/`、`public/admin/`、`public/member/`、`public/blog/`、`public/album/`、`public/tinymce/plugins/leaui_image/public/`

- [ ] 用源码搜索生成 Bootstrap 3 class、data attribute、jQuery plugin 调用和自定义覆盖清单，逐项给出 5.3 去向。
- [ ] 为 modal/tab/dropdown/alert/form/navbar 与 leaui_image 写 Chromium 交互测试。
- [ ] 保存关键页面桌面/窄屏基线截图。

### Task 2：切换单一 Bootstrap 5.3.8 资源

**Files:**
- Modify: `package.json`、`package-lock.json`、`scripts/build/manifest.mjs`
- Replace/Delete: `public/css/bootstrap*.css`、`public/js/bootstrap*.js`、`public/admin/css/bootstrap.3.2.0.min.css`、`public/tinymce/plugins/leaui_image/public/bootstrap3/`

- [ ] 安装并锁定 Bootstrap 5.3.8，从同一依赖生成各入口需要的 CSS/JS。
- [ ] 先在测试模板加载 5.3，运行组件测试观察明确失败，再开始模板迁移。
- [ ] 确认页面没有同时加载 Bootstrap 3 与 5。

### Task 3：迁移共享模板和组件 API

**Files:** `app/views/`、`public/js/common.js`、`public/js/app/*.js`、`public/admin/js/*.js`、`public/member/js/*.js`、`public/blog/js/*.js`、`public/album/js/main.js`

- [ ] 按研究映射迁移 data attributes、grid、forms、navbar、modal、tabs、dropdown、alerts 和 close button。
- [ ] 把共享 JS 调用改为 Bootstrap 5 原生实例 API并测试 show/hide 与事件只触发一次。
- [ ] 迁移区域自定义 CSS，只改与 Bootstrap 5 冲突的规则。
- [ ] 每完成一个页面区域就运行对应 E2E 与截图比较。

### Task 4：迁移 leaui_image UI

**Files:** `public/tinymce/plugins/leaui_image/public/index.html`、`public/tinymce/plugins/leaui_image/public/js/*.js`、`public/tinymce/plugins/leaui_image/public/css/*.css`

- [ ] 把内部布局、tab、alert、progress、form 和 close 控件改为 Bootstrap 5.3。
- [ ] 保持 fileupload、相册分页、URL 图片、图片尺寸/标题与 `top.LEAUI_DATAS` 数据接口。
- [ ] 在当前 TinyMCE 4 壳中完成真实上传/选择/插入 smoke，证明任务可在 TinyMCE 升级前独立验收。

### Task 5：范围与发布前验证

- [ ] 运行旧 class/data attribute 搜索，逐项核对允许残留清单。
- [ ] 运行 `npm run build && npm test && npm run test:e2e`，构建后零 diff。
- [ ] 回放 Golden/USN/页面 smoke；检查控制台和网络 404。
- [ ] 在桌面和窄屏复核截图，可见差异附在评审材料中。
- [ ] 确认未升级 TinyMCE core、未改变后端接口、未增加设计系统。

## Rollback Point

资源、模板/API、leaui_image 三个批次可分别复核，但任务整体回滚到 Bootstrap 3。无法等价的组件必须回到规划，不用隐藏 CSS 或双框架加载掩盖。
