# 呈现层：前端构建与编辑器运行时 — PRD

## Goal

维护 Node 24/esbuild manifest 驱动的生成链，并完成前端核心库、编辑器、模板和第一方插件的稳定呈现。

## Requirements

- Node 24.x LTS、lockfile 和 manifest 固定输入/输出；生成资源继续跟踪且 CI 重建零 diff。
- jQuery 3.7.1、Bootstrap 5.3.8、TinyMCE 8.8.2 各保留一个生产运行时；migrate 只用于诊断。
- 保留服务端模板、历史 URL、`leaui_image` iframe、四个第一方 TinyMCE 插件和保存状态语义。
- i18n 扫描、语言文件、CSS/JS 资源和 `note-dev.html`→`note.html` 由脚本唯一生成。
- 业务页面错误、资源 4xx/5xx、console/pageerror、上传与编辑清理必须可观察。

## Acceptance criteria

- [ ] `npm ci && npm run build && npm test` 在 Node 24 通过，重复构建零漂移。
- [ ] 版本唯一性、生产无 migrate、旧 jQuery/旧 TinyMCE 副本清理和插件资源契约通过。
- [ ] 登录、笔记、markdown、modal/tab/dropdown、上传、相册、admin/member/blog 和 iframe smoke 通过。
- [ ] 未编辑不保存，编辑保存只发生允许的 HTML 规范化且 revision/error 状态正确。
- [ ] 真实浏览器矩阵证据交给 `delivery-verification`，本任务不以 Chromium 代替 Safari。

## Out of scope

不改产品视觉设计、不改公开 URL、不引入 SPA/TypeScript、不替换第一方编辑器业务插件。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
