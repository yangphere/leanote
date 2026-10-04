# 修复新建普通与 Markdown 笔记报错

## Goal

在笔记本中点击“新建普通笔记”或“新建 Markdown 笔记”后，笔记列表请求应正常完成，不再因请求地址不匹配而弹出 `error!`。

## Background

- 截图显示新建操作期间 `GET /note/listNotes/?notebookId=...` 返回 404，页面弹出 `error!`。
- 两种新建按钮都调用 `Note.newNote`（`public/js/app/note.js:2773-2780`）；切换到新笔记所属笔记本时共用 `Notebook.changeNotebookForNewNote`（`public/js/app/note.js:1331-1341`）。
- 该函数及常规笔记本切换分别请求 `/note/listNotes/`（`public/js/app/notebook.js:602,689`），但 `conf/routes:32` 只注册 `/note/listNotes`。路由器按路径段匹配，尾斜杠会增加空段（`app/httpserver/registry.go:34-81`）。
- `research/repro-new-note-route.cjs` 已复现调用方地址与注册路由不一致：实际 `/note/listNotes/`，期望 `/note/listNotes`。

## Requirements

- R1：普通笔记与 Markdown 笔记共用的笔记本切换请求必须使用已注册的列表路由。
- R2：常规笔记本切换使用同一正确地址；保留现有笔记本参数、缓存与列表渲染行为。
- R3：保持现有 HTTP 路由精确匹配规则与 AJAX 失败提示，不以全局路径归一化或静默吞错掩盖此错误。

## Acceptance Criteria

- [x] A1：针对性请求地址回归检查通过，两个 `/note/listNotes/` 调用点均与 `conf/routes` 一致。
- [x] A2：Node 前端测试与构建通过，生成的页面资源包含修正后的请求地址。
- [x] A3：若可用真实 MongoDB 和浏览器环境，验证两种新建入口均无列表 404；否则明确记录未运行此项。真实环境验证未运行，见 `research/validation.md`。

## Out of Scope

- 修改新笔记持久化、自动保存或共享笔记流程。
- 放宽整个 HTTP 路由器的尾斜杠匹配规则。
