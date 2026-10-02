# 运行验收证据

> 本文件只记录非敏感运行结果；不写入密码、Cookie、认证头、页面全文、截图或用户私有数据。

## 2026-10-02

### Compose / HTTP

- `docker compose config --quiet`: **PASS**。
- `docker compose up -d --build --force-recreate`: **PASS**；当前源码镜像 `leanote:local` 构建成功。
- `docker compose ps`: **PASS**；`mongo` 为 `Up (healthy)`，`leanote` 为 `Up` 并发布宿主机 `9000`；本轮 `mongo-seed` 已正常结束。
- `GET http://127.0.0.1:9000/healthz`: **PASS**；HTTP `200`，响应 `{"status":"ready"}`。
- Leanote 启动日志：**PASS**；生产模式监听 `0.0.0.0:9000`，未观察到启动失败。

### Computer Use / 登录与核心工作区

- 打开 `http://127.0.0.1:9000/login`: **PASS**；登录表单可见。
- 用户人工完成登录并进入工作区（按修订后的 PRD 要求 3）：**PASS**；工作区页面正常加载。
- 新建普通笔记、编辑标题和正文、保存：**PASS**。
- 刷新后笔记内容仍存在：**PASS**；验证了持久化读取。
- 搜索刚创建的笔记：**PASS**。
- 创建测试笔记本：**PASS**；创建后可见且可进入。
- 删除本轮创建的测试笔记：**PASS**；Work 笔记数量恢复为 3。
- 删除本轮创建的测试笔记本 `CodexSmokeNB20261002`：**PASS**；退出前左侧列表已不再显示。
- 用户菜单退出登录：**PASS**；URL 回到 `/login`。

### 已发现的失败或阻塞

- 博客管理页 `/blog/admin`：**FAIL**；页面返回 `Internal Server Error`。
- 博客公开页 `/blog/post/admin/12da3e968ef6`：**FAIL**；页面返回 `Internal Server Error`。
- 分享入口：**FAIL**；返回 `{"Ok":false,"Msg":"validation"...}`，未建立分享。
- PDF 导出：**BLOCKED/UNRUN**；本轮未观察到浏览器下载，未能确认导出文件。
- 浏览器控制台：**FAILURE SIGNAL**；观察到 `dep.min.js` 中 Bootstrap `classList` TypeError，与页面交互异常同时出现。

### 尚未执行

- API 端到端矩阵、邮件发送、多个浏览器、移动端、故障注入、Mongo 持久化重启、真实 PDF 文件内容校验仍为 **UNRUN**。
- 本轮未修改代码、`.env`、数据库 schema 或现有用户数据；未执行 `docker compose down -v`。

### 后续处理（交叉引用，非本轮运行证据）

- 上述 FAIL/BLOCKED 项已移交 `10-02-docker-runtime-bug-fixes`（提交 `f8cfd6b1`、`7cf21120`），修复后复验记录见
  `.trellis/tasks/archive/2026-10/10-02-docker-runtime-bug-fixes/research/runtime-verification.md`。
- 该任务的结论：博客管理页与公开页 500 已修复（根因是 Docker 配置缺少 `site.url`；`12da3e968ef6` 为 URL title，复验返回 404）；分享弹窗与 Bootstrap `Illegal invocation` 已修复，但实际建立并清理分享仍为 **UNRUN**；PDF 导出仍为 **FAIL**（Mongo 管理配置缺少 `exportPdfBinPath`）。
- 本节只做引用；上方条目保留 2026-10-02 本轮的原始观察结果，不改为 PASS。

### 收尾结论（2026-10-02）

- 验收标准均已有明确结果；登录按修订后的 PRD 由用户人工完成。
- PDF 导出：本任务保持 **BLOCKED**；后续改为 Gotenberg 渲染容器方案，由新任务 `10-02-gotenberg-pdf-renderer` 负责开发、部署与真实下载验收。
- 已知证据缺口：各条目只记录到日期，没有逐项时间与证据位置；清理后的 `/healthz` 与容器状态复查没有单独记录；博客 500 的服务端日志摘要由修复任务的根因分析代替。
