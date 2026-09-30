# 修复博客发布及文章右键菜单

## Goal

修复文章右键点击“公开为博客”时报错，使单篇和批量发布/取消发布都按旧接口契约完成；核查并保留文章右键菜单的分享、导出 PDF、删除、移动和复制动作。

## Background and confirmed facts

- `public/js/app/note.js:1663-1693` 的发布菜单发送 `noteIds` 数组（jQuery 会编码为 `noteIds[]`），并在批量模式复用同一入口；该请求不发送 `isTop`，后端默认值仍为 `false`。
- `app/controllers/httpserver_notes.go:543-559` 的删除、移动、复制和复制共享分派也只读取 `noteIds`；生产 `$.param` 会发送 `noteIds[]`，因此这些动作同样会得到空列表。`app/controllers/httpserver_notes.go:573-578` 的发布分派只读取单数 `noteId`，导致发布请求中的 ID 为空。
- 已冻结的发布合同 `PB-BLOG-01`（`.trellis/tasks/archive/2026-09/09-08-application-publishing/research/implementation-decision-brief.md:9`）要求逐项确认，全部成功才返回 `true`；本任务不依赖历史实现的返回行为。
- `app/service/NoteService.go:931` 的 `ToBlog` 已提供 owner-scoped、USN/receipt 保护的单笔发布操作，不应在控制器重复实现业务规则。
- 右键菜单动作定义于 `public/js/app/note.js:2048-2068`：分享读取 `noteId`，PDF 使用 `/note/exportPdf?noteId=...`，删除/移动/复制（含共享复制）使用 `Note.submitMutation` 对应后端批量动作；现有回归测试覆盖 mutation 失败和保存队列行为，但未覆盖生产 `noteIds[]` 到后端的绑定。
- 真实浏览器、Mongo 和 PDF 工具运行证据本任务不假设已具备；静态审查与可运行的聚焦测试分别记录。

## Requirements

1. `Note.SetNote2Blog` 同时接受 `noteIds[]`（兼容 `noteIds` 重复参数）和单数 `noteId`，逐笔调用 `NoteService.ToBlog`，空目标或任一失败返回原始 JSON `false`，全部确认成功才返回 `true`。
2. 发布/取消发布的请求绑定不改变既有 `isBlog`、`isTop` 和认证/owner 边界，也不新增第二套发布逻辑。
3. 统一修复删除、移动、复制和复制共享的 `noteIds[]` 参数绑定，复用 `noteParameterStrings`；分享和 PDF 继续使用单数 `noteId`。核查五个动作的路由和失败处理，补最小回归测试，不改变菜单文案或业务服务边界。

## Acceptance Criteria

- [x] 单篇右键“公开为博客”发送的 ID 能被后端解析，服务成功后返回 `true`，失败返回 `false`，列表状态按响应更新（代码与聚焦测试确认；未进行真实浏览器/Mongo 端到端验证）。
- [x] 批量发布/取消发布逐项执行；空目标、非法/无权限 ID、部分失败均不返回成功，并继续处理剩余目标以保留逐项诊断（控制器汇总与聚焦测试确认；未进行真实 HTTP/Mongo 验证）。
- [x] 右键菜单分享、导出 PDF、删除、移动、复制和复制共享均能解析生产请求参数；PDF 下载仍走流式导出入口，mutation 动作仍保留失败提示（静态契约与代码复核确认；未运行浏览器/PDF 工具）。
- [x] 相关 Go/JavaScript 聚焦测试、格式检查和任务校验通过；Mongo、真实 HTTP、浏览器和 PDF 工具仍明确标注为 `unrun`。

## Out of scope

- 不重写博客发布服务、USN/receipt 或公开博客查询逻辑。
- 不新增右键菜单项目、改变菜单文案/交互，不进行真实浏览器自动化或生产数据操作。

## Open questions

无。实现按上述兼容单篇/批量发布和 mutation 参数绑定合同进行。
