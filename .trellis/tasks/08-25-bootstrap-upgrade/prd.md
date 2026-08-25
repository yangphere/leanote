# Bootstrap 5.3 升级（E-BS）— PRD

父任务：`.trellis/tasks/08-25-frontend-libs`

## Goal

把 Bootstrap 3.2 升到 5.3.8，适配服务端模板、第一方 JavaScript/CSS 和 `leaui_image` UI，在不重新设计界面、不改变 URL/信息架构的前提下恢复主要页面和交互的现代浏览器兼容性。

## Dependencies

- 依赖 `08-25-jquery-upgrade` 完成。
- 完成后才允许 `08-25-tinymce-upgrade` 开始。

## Requirements

- **R-BS1** Bootstrap 5.3.8 由 npm lockfile 管理并通过构建链生成；删除主站、admin 与 `leaui_image` 的 Bootstrap 3 核心副本。
- **R-BS2** 更新 `app/views/` 中 Bootstrap 3 class、grid、form、navbar、modal、dropdown、tab、alert、close button 和 `data-*` 属性，保留页面结构与文案。
- **R-BS3** 更新第一方 JS 的 modal/tab/dropdown/tooltip 等调用到 Bootstrap 5 支持的 API；不留下永久 Bootstrap 3 jQuery adapter。
- **R-BS4** 适配 `public/css`、admin/member/blog/album 样式覆盖，避免旧选择器靠偶然优先级生效。
- **R-BS5** `public/tinymce/plugins/leaui_image/public/bootstrap3/` 的 UI 升级到 5.3.8，并保持上传、相册、分页、图片属性流程。
- **R-BS6** 不升级 TinyMCE core，不做视觉重设计，不改变后端接口。

## Acceptance Criteria

- [ ] `npm ls bootstrap` 只显示 5.3.8，仓库无 Bootstrap 3.2 核心副本或 `bootstrap3` 运行目录。
- [ ] 模板源码搜索无 `data-toggle`、`data-target`、`pull-left/right`、`hidden-*` 等已迁移旧用法；允许项必须在兼容清单中说明。
- [ ] Chromium E2E 覆盖 navbar、dropdown、modal、tab、tooltip、alert、表单校验、登录/注册、note、blog、admin、member、album、leaui_image。
- [ ] 关键页面在桌面与窄屏视口无阻断布局回归，并为可见变化保存评审截图。
- [ ] `npm run build && npm test`、Golden、USN、页面 smoke 通过，构建后零 diff。
- [ ] 控制台无 Bootstrap 初始化错误、重复事件或静态资源 404。
- [ ] diff 不包含 TinyMCE core 升级、URL/信息架构变化或 UI 重设计。

## Out of Scope

- 采用新的设计系统、重做视觉、重写模板架构。
- TinyMCE 8 插件 API 迁移；本任务只升级 `leaui_image` 内部 Bootstrap UI。
- IE 兼容和长期 Bootstrap 3 adapter。
