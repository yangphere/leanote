# ADR-0003：以生成资源契约现代化前端

- **状态**：Accepted
- **日期**：2026-08-25

## Context

Gulp 3 已无法在现代 Node 安装，生产资源依赖手工同步；jQuery 1.9、Bootstrap 3.2 与 TinyMCE 4.1.9 均已陈旧。应用仍需要服务端渲染、历史 URL、存量笔记 HTML 和用户博客主题。

## Decision

- Node 构建基线固定 24.x LTS并提交 lockfile。
- 使用 esbuild 与普通 Node 脚本等价生成现有 bundle、CSS、i18n 和 `note.html`。
- 生成资源继续提交到 Git，但只能由构建脚本产生；CI 重建后执行零 diff 门禁。
- `frontend-libs` 作为协调父任务，按 jQuery 3.7.1 → Bootstrap 5.3.8 → TinyMCE 8.8.2 顺序交付。
- `jquery-migrate` 只在迁移开发阶段用于诊断；最终生产 bundle 不包含它且迁移警告清零。
- TinyMCE 通过 npm 自托管；仓库只保留第一方插件和仍需 vendoring 的独立脑图子应用。
- 未编辑即关闭不得触发保存，数据库 HTML 原文保持字节不变。实际编辑并保存时允许明确列出的空白或属性顺序规范化，但 DOM 语义、文本、链接、图片、代码块和第一方插件标记必须等价。
- 支持 Chrome、Edge、Firefox、Safari 的当前及前一主版本；Chromium E2E 是 PR/push 阻断门禁，真实 Chrome、Edge、Firefox、Safari 的两版 smoke 是发布前证据，Safari 必须在真实 Safari 环境验证，Chromium 不能代替 Chrome 或 Edge；不支持 IE。

## Consequences

- 保留现有直接检出与打包方式，同时消除手工维护多份资源的事实来源。
- Git diff 仍包含生成文件，但任何漂移都可由 CI 发现。
- 三个库升级可独立评审和回滚；TinyMCE 仍是前端链最高风险阶段。
- HTML 验收关注数据与语义保真，而不是要求新版编辑器产生与旧版本完全相同的序列化字节。
