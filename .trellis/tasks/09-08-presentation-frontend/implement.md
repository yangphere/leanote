# 呈现层：前端构建与编辑器运行时 — 执行计划

- [ ] 盘点 manifest、Node scripts、模板、语言文件、旧运行时和第一方插件输入/输出。
- [ ] 运行 Node 24 构建并确认 manifest 是唯一事实来源、输入安全和输出零漂移。
- [ ] 分别验证 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 和四个第一方插件。
- [ ] 补齐 editor save-state、未编辑零写入、iframe、上传、模板和 i18n 回归。
- [ ] 运行 `npm ci && npm run build && npm test`、Playwright discovery/business/build smoke，记录真实服务依赖。
