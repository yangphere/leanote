# 运行基线

来源：`.trellis/tasks/10-02-docker-login-functional-validation/research/runtime-evidence.md`。

- Docker、Mongo、Leanote 和 `/healthz` 已通过。
- 登录、笔记创建/编辑/刷新、搜索、笔记本创建/删除、退出登录已通过。
- `/blog/admin` 与 `/blog/post/admin/12da3e968ef6` 返回 `Internal Server Error`。
- 分享入口返回 validation 错误。
- 浏览器观察到 `dep.min.js` 的 Bootstrap `classList` TypeError。
- PDF 未观察到下载，状态为 `BLOCKED/UNRUN`。

该基线只用于复现和回归，不等同于完整浏览器/API/邮件发布矩阵。
