# 修复 Docker 验证发现的博客分享前端问题

## Goal

修复 2026-10-02 Docker + Computer Use 验证中发现的可复现 Web 功能故障，并把同一真实浏览器流程固化为回归验收依据。

## Confirmed evidence

- `/blog/admin` 返回 `Internal Server Error`。
- `/blog/post/admin/12da3e968ef6` 返回 `Internal Server Error`。
- 分享入口返回 `{"Ok":false,"Msg":"validation"...}`，未建立分享。
- 浏览器控制台观察到 `dep.min.js` 中 Bootstrap `classList` TypeError；需要先补齐触发堆栈和调用路径，再决定修复点。
- PDF 导出未观察到浏览器下载，当前只能视为待复现/待验证，不能预先假定是产品 bug。
- 原始运行记录见 `.trellis/tasks/10-02-docker-login-functional-validation/research/runtime-evidence.md`。

## Requirements

1. 定位并修复博客管理页和博客公开页的共同根因，保证认证用户可打开管理页，已发布测试内容的公开 URL 可访问。
2. 定位并修复分享入口的 validation 失败，保证从现有笔记触发分享时返回成功结果或明确、可操作的校验错误。
3. 复现 `classList` TypeError，确认触发页面和 Bootstrap 状态转换；修复后同一操作不再产生该异常，也不破坏菜单交互。
4. 重新验证 PDF 导出：若能稳定复现失败，修复并确认浏览器下载或成功响应；若无法复现，记录环境、操作和结果，不伪造通过。
5. 保留当前认证、笔记、搜索和笔记本等已通过流程；不得删除用户数据或使用 `down -v` 清空 Mongo named volume。
6. 按用户补充要求，在普通笔记和 Markdown 笔记中添加可辨认的测试图片，确认保存、刷新和公开博客中的正文及图片显示正常。
7. 修复公开博客评论区持续显示加载图的问题，评论加载完成后隐藏 spinner，并确保已有浏览器缓存能获取修复后的样式。
8. 修复普通笔记粘贴剪贴板图片时同一图片被插入两次的问题；一次粘贴只能产生一个持久化图片节点和一个服务端图片文件。

## Acceptance Criteria

- [ ] 针对博客管理页和公开页的回归测试或等价可复现验证通过；HTTP 500 消失，页面内容可观察。
- [ ] 分享入口回归验证通过；成功结果可观察，且生成的数据可按既有入口清理。
- [ ] `classList` TypeError 的触发路径有记录；修复后同一 Computer Use 流程无该异常，Bootstrap 菜单可打开和关闭。
- [ ] PDF 导出有明确 `PASS`、`FAIL` 或 `UNRUN` 结果；`PASS` 必须包含实际下载或响应证据。
- [ ] 运行环境验证通过：Compose 服务健康、`/healthz` 返回 200/ready。
- [ ] 不暴露凭据、Cookie、认证头或用户私有内容；未执行破坏 named volumes 的命令。
- [ ] 普通与 Markdown 测试笔记中的正文、图片在保存和刷新后仍存在，公开博客显示正常。
- [ ] 公开博客评论区加载完成后 spinner 不可见，静态资源缓存失效与三套内置主题契约有验证证据。
- [ ] 普通笔记粘贴剪贴板图片后，编辑器、保存后的正文和公开博客均只显示一张图片；浏览器与 Mongo 证据确认没有重复的 `blob:` 节点或第二个上传文件。

## Out of scope

- 不扩展为 Firefox/Safari/Edge、多版本、移动端、邮件、API 全矩阵或故障注入验收。
- 不在没有复现证据时修改 PDF 安全策略或引入新的导出实现。
- 不重置管理员密码，不修改 `.env`、数据库 schema 或无关模块。

## Open questions

- None blocking. PDF 结果以复现后的证据决定是修复项还是保留为 `UNRUN`/环境限制。
