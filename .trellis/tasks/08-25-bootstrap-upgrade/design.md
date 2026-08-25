# Bootstrap 5.3 升级（E-BS）— 技术设计

## 1. 资源边界

Bootstrap 作为 npm 依赖，由 D 的构建 manifest 产生主站 CSS/JS。仓库不再维护 `public/css/bootstrap*.css/js`、`public/admin/css/bootstrap.3.2.0.min.css` 与 `leaui_image/public/bootstrap3` 的独立核心副本。需要不同入口时，它们从同一 5.3.8 依赖生成。

## 2. 模板迁移规则

建立一份可搜索的映射并机械迁移，再逐页视觉复核：

- `data-toggle/target/dismiss` → `data-bs-*`。
- `pull-left/right` → `float-start/end`；旧 clearfix/visible/hidden 工具改为 5.3 等价类。
- `col-xs-*`、offset、form-group、input-group-addon、btn-default、panel/well 等改为明确的 5.3 结构或小范围第一方样式。
- close button 使用 `.btn-close`，不保留依赖旧 × markup 的 JS。

若 Bootstrap 5 无语义等价组件，用当前页面的第一方 CSS 实现最小等价，不引入新 UI 功能。

## 3. JavaScript

优先在共享 dialog/tab/dropdown 入口使用 Bootstrap 5 原生类 API，避免每个调用点各写一套。事件仍使用 `show/shown/hide/hidden.bs.*`，测试防止重复绑定。由于 jQuery 仍存在，业务可继续用 jQuery 做 DOM 查询，但不依赖 Bootstrap 3 jQuery plugin shim。

## 4. leaui_image

该 iframe UI 在本任务完成 Bootstrap 布局与组件迁移，保留其 jQuery fileupload、pagination、相册与 `top.LEAUI_DATAS` 接口。TinyMCE 任务随后只迁移编辑器插件壳，不再次修改其 Bootstrap 结构。

## 5. 视觉与回滚

对 `/login`、`/note`、`/blog`、admin、member、album、leaui_image 在桌面/窄屏保存前后截图。可见差异只有框架必要差异，不能改变业务布局。任务可独立回退到 Bootstrap 3 生成入口与模板提交。
