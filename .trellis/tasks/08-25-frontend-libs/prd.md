# 前端库现代化（E，协调父任务）— PRD

父任务：`.trellis/tasks/08-25-tech-stack-modernization`

## Goal

在保持服务端渲染、现有 URL、页面信息架构、TinyMCE 内核和存量笔记数据兼容的前提下，协调完成 jQuery 3.7.1、Bootstrap 5.3.8 与 TinyMCE 8.8.2 三个独立升级，并以统一浏览器、生成资源和端到端门禁验收整体前端。

本任务是协调父任务，不直接修改生产代码。

## Dependencies and Children

- 依赖 `08-25-frontend-build-chain`。
- 子任务严格串行：`08-25-jquery-upgrade` → `08-25-bootstrap-upgrade` → `08-25-tinymce-upgrade`。

## Shared Requirements

- **R-E1** Node 24 构建链是所有运行时库与生成资源的唯一入口；不得手工修改生产 bundle。
- **R-E2** 支持 Chrome、Edge、Firefox、Safari 当前及前一主版本；不支持 IE。
- **R-E3** `@playwright/test` 的 Chromium E2E 是每个子任务的阻断门禁；Firefox 与 Safari 完成发布前冒烟，Safari 结果必须来自真实 Safari 环境。
- **R-E4** 保留服务端渲染、模板结构、URL、RequireJS/全局脚本契约和用户上传博客主题。
- **R-E5** `/api/*` 与 USN 不变；前端升级不得通过后端兼容分支掩盖错误。
- **R-E6** 每个子任务完成后重新生成所有资源并通过零 diff 漂移门禁。

## Acceptance Criteria

- [ ] 三个子任务均完成各自 PRD 验收并可独立回滚。
- [ ] 最终 lockfile 只含目标 jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2；生产 bundle 不含 `jquery-migrate`。
- [ ] Chromium 自动化覆盖登录、笔记列表、富文本编辑、markdown、图片/附件、博客、admin/member 关键流程。
- [ ] Firefox 与 Safari 当前版本完成发布前冒烟并记录结果。
- [ ] `npm ci && npm run build && npm test` 通过，构建后 `git diff --exit-code` 为零。
- [ ] Golden、USN 与所有权回归通过。
- [ ] 浏览器控制台没有未处理异常、缺失资源或 jQuery Migrate 警告。
- [ ] 未编辑笔记不触发保存；编辑后 HTML 通过语义等价夹具。

## Out of Scope

- SPA、前后端分离、UI 重新设计或编辑器内核替换。
- jQuery 4、Bootstrap 6 或 TinyMCE 之外的新编辑器。
- 由协调父任务直接承载生产代码改动。
