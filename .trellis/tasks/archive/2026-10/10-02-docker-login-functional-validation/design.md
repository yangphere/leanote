# 技术设计：Docker 登录与功能运行验收

## 边界与数据流

1. 先以仓库根目录的 `docker-compose.yml` 和现有 `.env` 为唯一配置来源运行 `docker compose config --quiet`。
2. 使用 `docker compose up -d --build --force-recreate` 启动当前源码，等待 `mongo` 健康、`mongo-seed` 完成和 `leanote` 进入运行态；不删除 named volumes。
3. 通过宿主机 `http://127.0.0.1:9000` 访问服务：命令行只负责健康探针、容器状态和必要日志；页面行为由 Computer Use 的浏览器会话验证。
4. 登录由用户在浏览器中人工完成，工具不读取或填写凭据；凭据不写入证据文件。若持久化数据库中的密码与初始密码不一致，记录登录失败和原因，不擅自开启环境密码恢复或修改现有数据。
5. 测试数据使用带时间标记的标题，便于搜索与清理；所有写入操作都在同一登录会话中完成，并在结束前通过页面入口清理。

## 证据契约

- 运行证据保存到本任务目录的 `research/runtime-evidence.md`，只记录命令、状态、URL、非敏感结果和失败诊断；不保存密码、Cookie、认证头、页面全文、截图或用户私有数据。
- 每项验收使用 `PASS`、`FAIL`、`BLOCKED` 或 `UNRUN`；`PASS` 必须有实际 Docker/HTTP/浏览器观察结果。
- Computer Use 仅证明当前浏览器和当前镜像运行结果，不替代发布所需的多浏览器矩阵或完整 E2E harness。

## 回滚与清理

- 运行异常时先收集 `docker compose ps`、`docker compose logs --tail 100 leanote` 和 `/healthz`，再停止服务；不使用 `down -v`。
- 任务结束保留 named volumes，除非用户另行要求清理；删除本次创建的笔记、博客发布和分享数据。
- 若浏览器操作无法完成，保持证据为 `BLOCKED`/`UNRUN`，不通过 API 或数据库直接伪造成功路径。
