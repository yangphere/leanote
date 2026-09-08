# 应用层：分享、博客与主题 — 执行计划

- [ ] 盘点 Share/Blog/Theme/Group service、主站/member controller、模板和 i18n 调用。
- [ ] 将公开/私有授权、分享生命周期和主题 canonical path/ZIP 安全收敛到 service/专用 boundary。
- [ ] 补齐博客、评论、点赞、统计、群组、分享和预览的成功/失败/过期测试。
- [ ] 验证内置主题与上传主题、模板错误、静态资源和页面 HTML 语义；页面 smoke 入口交由 interface/delivery 任务。
- [ ] 运行 service-level Golden/权限回归，并记录跨层真实页面 smoke 由 interface/delivery 任务验收。

验证：目标 Go 测试、service-level Golden/权限回归和 `git diff --check`；`npm test`、Playwright/真实服务页面 smoke 由 presentation/interface/delivery 任务负责。
