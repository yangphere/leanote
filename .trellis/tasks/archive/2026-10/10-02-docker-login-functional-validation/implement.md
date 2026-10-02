# 执行计划：Docker 登录与功能运行验收

## 有序步骤

1. 读取 `.env` 的非敏感配置形状，运行 `docker compose config --quiet`，确认工作区干净且不覆盖用户已有改动。
2. 运行 `docker compose up -d --build --force-recreate`；轮询 `docker compose ps` 和 `/healthz`，必要时读取 Leanote 最近日志。
3. 打开 `http://127.0.0.1:9000/login`，由用户人工填写管理员账号完成登录；随后用 Computer Use 验证登录后的工作区和导航。
4. 在浏览器中按 PRD 顺序执行笔记持久化、笔记本/标签、搜索、博客/预览、PDF、分享和退出流程；为每项记录结果和证据。
5. 删除本次测试数据，刷新页面确认清理；再次检查 `/healthz` 与容器状态。
6. 把真实运行证据写入 `research/runtime-evidence.md`，复核敏感信息未落盘，运行 `git diff --check` 和 `python ./.trellis/scripts/task.py validate <task-dir>`。

## 验证命令

- `docker compose config --quiet`
- `docker compose up -d --build --force-recreate`
- `docker compose ps`
- `Invoke-WebRequest http://127.0.0.1:9000/healthz`
- `docker compose logs --tail 100 leanote`
- Computer Use 浏览器操作与页面观察
- `git diff --check`
- `python ./.trellis/scripts/task.py validate .trellis/tasks/10-02-docker-login-functional-validation`

## 风险与停止条件

- Docker Desktop 不可用、镜像构建失败、Mongo seed 失败或 `/healthz` 不 ready：停止浏览器验收，收集日志并标记 `BLOCKED`。
- 登录失败：不得开启 `LEANOTE_ADMIN_FORCE_ENV_PASSWORD=true` 或修改数据库；记录当前持久化账号与 `.env` 初始密码不一致的事实。
- 页面写入无法清理：停止继续写入，记录残留数据和清理入口，避免扩大污染。
- PDF、上传、分享等入口出现下载/外部跳转限制：记录为 `BLOCKED` 或 `UNRUN`，不以 HTTP 猜测替代页面证据。
