# Docker 登录与功能运行验收

## Goal

启动 Leanote Docker 环境，由用户人工完成登录后，通过浏览器对核心页面与功能进行真实运行验证，记录可复现证据与未运行的验收项。

## Background and confirmed facts

- `docker-compose.yml` 编排 `mongo`、一次性 `mongo-seed` 和 `leanote` 三个服务；Leanote 通过宿主机 `LEANOTE_HTTP_PORT` 暴露，当前 `.env` 配置为 `9000`。
- `.env` 已提供非占位的管理员邮箱、初始密码和应用密钥；密码与密钥只用于本机登录，不写入任务材料、日志摘要或最终回复。
- `/healthz` 是就绪探针；Compose 使用 named volumes 保存 MongoDB 与 Leanote 数据，验证过程中不得执行 `docker compose down -v`。
- 仓库文档明确指出构建、单元测试不能替代真实 Docker、MongoDB、浏览器和跨进程验收，因此本任务只把实际运行结果作为运行证据。
- 浏览器 smoke 文档覆盖登录、笔记/搜索、笔记本、附件/相册、博客、admin/member、分享和 `leaui_image` 等区域；本次先覆盖用户可通过主 Web 界面完成且风险可控的核心路径，无法安全完成的区域单独标为未验证。

## Requirements

1. 按仓库 Compose 约定校验配置并启动或重建当前源码对应的 Docker 堆栈，保留现有 named volumes。
2. 通过 HTTP 检查 `/healthz` 返回 `200` 与 `{"status":"ready"}`，并记录 `docker compose ps` 和必要的应用日志摘要。
3. 由用户在本机浏览器中使用 `.env` 中的管理员账号人工完成登录（凭据不经由工具输入）；不得在截图、任务文件或回复中暴露密码、Cookie、认证头或用户数据。
4. 在真实浏览器会话中验证以下核心流程：
   - 登录后工作区/笔记列表可加载，退出后回到登录页；
   - 新建笔记、编辑标题和正文、保存后刷新仍可见；
   - 笔记本或标签至少完成一次可观察的创建/修改或筛选操作；
   - 搜索能找到刚创建的笔记；
   - 将测试笔记发布为博客或打开预览/公开页面，并确认页面可访问；
   - 对测试笔记触发 PDF 导出并确认浏览器下载或响应成功；
   - 若界面提供分享入口，验证生成分享结果或明确记录入口受阻原因；
   - 清理本次创建的测试笔记、博客发布和分享数据（若清理入口可用），避免污染持久卷。
5. 将每个流程记录为通过、失败或未运行，包含时间、URL/操作、可观察结果和证据位置；失败必须保留足以定位根因的服务日志或页面错误。

## Out of scope

- 不修改产品代码、Compose 配置、`.env` 或数据库 schema。
- 不执行破坏 named volumes 的命令，不重置或删除已有用户/笔记数据。
- 不把一次 Chrome/Computer Use 会话外推为 Firefox、Safari、Edge、多版本矩阵或发布验收通过。
- 不把 API、邮件、后台管理、移动端、跨进程故障注入、Mongo 复制集、PDF 安全边界等未实际运行项目标记为通过。

## Acceptance Criteria

- [x] `docker compose config --quiet` 成功；Compose 服务启动后 `mongo` healthy、`mongo-seed` 成功结束、`leanote` running。
- [x] `GET http://127.0.0.1:9000/healthz` 返回 HTTP 200 和 `{"status":"ready"}`。
- [x] 人工登录成功，认证后工作区可用，且退出登录回到 `/login`。
- [x] 笔记创建/编辑/保存/刷新持久化、搜索、笔记本或标签筛选至少各有真实浏览器证据。
- [x] 博客预览或公开页面、PDF 导出、分享入口（可用时）各有明确结果；受阻项标记为 `blocked` 或 `unrun` 并说明原因。
- [x] 测试数据清理结果已记录；任何未执行的真实环境门禁仍保持 `unrun`/`partial`，不以 Go/Node 检查替代。

## Open questions

- None blocking: 本次采用“核心 Web 用户旅程 + 明确列出未覆盖区域”的验收边界；“各项功能”不被解释为所有浏览器、API、邮件和故障注入矩阵。
