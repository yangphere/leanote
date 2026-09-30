# 技术设计

## 边界与不变量

HTTP 适配器只负责把兼容的请求字段绑定为 note ID 列表，并逐项调用现有 `NoteService.ToBlog`。发布的 owner、回收站、已删除、USN、receipt、标签和内容投影规则继续由服务层负责；控制器不直接访问集合或重建发布逻辑。

发布结果保持旧的 raw JSON boolean：非空列表中的每一项都确认成功才返回 `true`，空列表、非法 ID、权限失败或任一项失败返回 `false`。逐项调用顺序保持请求顺序；即使某项失败也继续处理剩余项，以保留每个目标的 receipt/对账结果；不引入无结果后台任务或静默 fallback。部分成功时前端现有回调只收到 `false`，不会更新已成功项目的列表样式，刷新后才与服务端一致；这是本任务记录的已知兼容限制，不在本次扩大为前端状态重构。

## 请求与数据流

1. 浏览器右键菜单 `Note._setBlog` 发送 `noteIds[]` 和 `isBlog`；`isTop` 不在该请求中发送，由后端按 `false` 处理。
2. `noteParameterStrings(c.Params, "noteIds")` 读取重复参数、`noteIds[]` 或索引形式；若兼容旧调用没有列表，则回退读取单数 `noteId`。
3. `Note.SetNote2Blog` 对每个 ID 调用 `noteService.ToBlog(userID, id, isBlog, isTop)`，汇总为 `allNotesToBlog`。
4. 浏览器仅在 `true` 时更新列表缓存；false 继续走现有 AJAX 错误提示。

右键菜单动作统一按生产编码复核：分享只读 `noteId` 并打开分享信息；PDF 通过 `/note/exportPdf` 进入 `ContentPDF.Export` 流式响应；删除/移动/复制/复制共享的后端分派均通过 `noteParameterStrings(c.Params, "noteIds")` 接受 `noteIds[]`、重复 `noteIds` 和索引形式，再交给 mutation service。这样浏览器和 HTTP 适配器共享同一数组字段契约。

## 兼容与回滚

- 发布保留单数 `noteId` 兼容旧客户端，同时优先支持前端当前使用的数组字段；mutation 动作继续使用 `noteIds` 列表并兼容其数组编码形式。
- 不改变路由、响应 MIME 或 JSON 形状；回滚只需恢复控制器分派实现。
- 真实 Mongo/HTTP/浏览器/PDF 证据属于未运行门，不由单元测试替代。
