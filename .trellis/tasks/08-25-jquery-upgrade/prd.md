# jQuery 3.7 升级（E-jQ）— PRD

父任务：`.trellis/tasks/08-25-frontend-libs`

## Goal

把应用运行时 jQuery 从 1.9.0 升到 3.7.1，修复第一方代码和仍保留插件的兼容问题；`jquery-migrate` 只作为开发诊断工具，最终生产 bundle 不包含兼容层或迁移警告。

## Dependencies

- 依赖 `08-25-frontend-build-chain`。
- 完成后才允许 `08-25-bootstrap-upgrade` 开始。

## Requirements

- **R-jQ1** jQuery 3.7.1 由 npm lockfile 管理，构建链生成稳定的生产路径；删除 `public/js/jquery-1.9.0.min.js` 及 `leaui_image` 内重复的旧 jQuery 核心副本。
- **R-jQ2** 盘点并适配 `public/js/app`、`public/js/plugins`、admin/member/blog/album 与 `jquery-cookie`、validation、fileupload、zTree、contextmenu、slimScroll、artDialog 等现有插件用法。
- **R-jQ3** 开发诊断构建可以加载与 3.7.1 匹配的 `jquery-migrate`，但生产 manifest、bundle 与页面不得包含它；完成时 migrate 控制台警告为零。
- **R-jQ4** 不升级 Bootstrap/TinyMCE，不引入 jQuery 4，不把旧 API 封装为永久全局兼容层。
- **R-jQ5** AJAX、事件、表单序列化、Deferred、DOM/data 与插件行为保持用户可见兼容；失败必须在控制台或测试中暴露。

## Acceptance Criteria

- [ ] `npm ls jquery` 只显示 3.7.1；生产资源中不存在 jQuery 1.9 核心副本。
- [ ] `jquery-migrate` 不在生产 dependency/manifest/bundle 中，诊断运行零 warning。
- [ ] `npm run build && npm test` 通过且重建后零 diff。
- [ ] Chromium E2E 覆盖登录、笔记/笔记本/标签、搜索、对话框、上传、相册、博客、admin/member 常用交互。
- [ ] AJAX 失败测试证明错误不会被旧回调兼容代码吞掉。
- [ ] 浏览器控制台无未处理异常、deprecated warning 或静态资源 404。
- [ ] G 的 Golden、USN 与页面 smoke 通过。
- [ ] diff 不包含 Bootstrap/TinyMCE 版本升级或视觉重设计。

## Out of Scope

- jQuery 4、移除 jQuery 或把应用改成原生 DOM/SPA。
- Bootstrap、TinyMCE 和第三方插件的功能升级；只做 jQuery 3.7 必需适配。
- 长期保留 `jquery-migrate`。
