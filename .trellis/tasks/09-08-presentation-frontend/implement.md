# 呈现层：前端构建与编辑器运行时 — 执行计划

- [ ] 盘点 manifest、Node scripts、模板、语言文件、旧运行时和第一方插件输入/输出。
- [ ] 运行 Node 24 构建并确认 manifest 是唯一事实来源、输入安全和输出零漂移。
- [ ] 分别验证 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 和四个第一方插件。
- [ ] 补齐 editor save-state、未编辑零写入、iframe、上传、模板和 i18n 回归。
- [ ] 接收 notes KD-N6：为 `UpdateNoteOrContent`、copy/shared-copy、delete/move batch 生成 `OperationId`，update 附带 `ExpectedUsn`；用前端 contract 覆盖同一 unknown-result 复用、新意图换代、batch 顺序/子 operation 稳定、stale conflict 以及旧客户端省略字段。
- [ ] 在真实页面执行上述生成/复用与未编辑/编辑流程，把浏览器 artifact 交给 delivery；如发现 server receipt/mapper 缺陷，重开 notes leaf。
- [ ] 运行 `npm ci && npm run build && npm test`、Playwright discovery/business/build smoke，记录真实服务依赖。
