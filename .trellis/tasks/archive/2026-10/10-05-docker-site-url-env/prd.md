# Docker 通过 .env 提供 site.url 配置

## Goal

Docker Compose 部署时，让运维通过 `.env` 的 `LEANOTE_SITE_URL` 声明 Leanote 的对外访问地址
（`site.url`），并在启动时严格校验格式。经 Caddy / Nginx 反向代理、以域名访问时，编辑器插入的
图片与附件、API 返回给客户端的图片/附件链接、博客默认域名解析、邮件链接都使用真实对外地址，
而不是镜像内写死的 `http://127.0.0.1:9000`。

## Background

- 镜像内生产配置 `conf/app.conf-docker:11` 写死 `site.url=http://127.0.0.1:9000`；
  `Dockerfile:44` 以 0440 复制到 `/etc/leanote/app.conf`，`.env` 无法改变它；
  `.env.example` 与 `docker-compose.yml` 的 `leanote.environment` 中没有 site.url 相关变量。
- `site.url` 的消费点：
  - `app/controllers/httpserver_main.go:237` → `app/views/note/note.html:876` `UrlPrefix`；富文本插图对话框
    `public/tinymce/plugins/leaui_image/public/js/main.js:16,281,383` 以 `UrlPrefix` 拼绝对地址，
    TinyMCE `convert_urls: false`（`public/js/tinymce-config-source.js:23`）原样存入笔记。
  - 附件链接 `public/js/app/note.js:2195`。
  - API 内容改写 `app/service/NoteService.go:1692` `FixContent` 以 `site.url` 生成
    `/api/file/getImage`、`getAttach` 绝对链接给桌面/移动客户端。
  - 博客域名解析 `app/controllers/httpserver_blog.go:113` 用 `GetDefaultDomain()`
    （`applySiteURLDomain`，`app/service/ConfigService.go:1127`，仅识别 `http://` / `https://`
    前缀并直接截取剩余部分）匹配请求 Host；反代后 Host 为对外域名，与 `127.0.0.1:9000`
    不匹配 → 进入自定义域名分支 → 404。带路径或末尾 `/` 的 site.url 会得到错误默认域名。
  - 邮件激活/找回密码链接 `app/service/EmailService.go:381,407,501,541,567`。
- 粘贴/拖拽上传图片用相对路径（`public/js/plugins/editor_drop_paste.js:196,439`），不受影响。
- `ValidateProductionConfig`（`app/httpserver/production_config.go:43`）目前不校验 `site.url`；
  `${VAR}` 未设置/为空时 `ParseConfig` 视为键不存在（`app/httpserver/config.go:169`），
  site.url 为空时博客解析返回 500（`.trellis/spec/backend/quality-guidelines.md:598-617`）。
- 启动时配置文件的 `site.url` 覆盖数据库中非空的 `siteUrl`（`ConfigService.go:197-200`）。
- 现有 Compose 约定：除恢复开关外均为 `${VAR:?VAR must be set}` 必填。
- 项目仍在开发中、尚未部署上线：无存量部署、无历史笔记数据，不需要升级兼容或迁移方案（用户确认）。
- `scripts/container-smoke.sh:67`、`scripts/package-smoke.sh:84` 自行生成的 prod 配置不含 `site.url`；
  `tests/js/release-contract.test.js:528` 对 Docker 配置做文本断言。

## Requirements

- R1 `.env.example` 新增必填 `LEANOTE_SITE_URL`，中文注释说明：用户浏览器实际访问的地址、
  格式规则、反代示例 `https://note.example.com`、直连示例 `http://localhost:9000`、
  修改后需重建容器、已写入笔记的旧地址不会自动改写。
- R2 `docker-compose.yml` `leanote.environment` 注入
  `LEANOTE_SITE_URL: ${LEANOTE_SITE_URL:?LEANOTE_SITE_URL must be set}`；缺失或为空时 Compose 拒绝启动。
- R3 `conf/app.conf-docker` 改为 `site.url=${LEANOTE_SITE_URL}`。
- R4 生产启动（`-runMode prod`）必须校验 `site.url`，不合法时以稳定、脱敏的 `ConfigError`
  在绑定监听和数据库之前退出（exit 78）。合法值规则：
  - 无首尾空白；可被 URL 解析；
  - scheme 为小写 `http` 或 `https`；
  - 主机非空，且能通过现有博客主机规范化（`domain.CanonicalizeBlogHost`）；
  - 可带端口，端口必须为 1–65535 的数字；
  - 不含用户信息、路径（包括单独的末尾 `/`）、查询参数或片段。
- R5 `[prod]` 中的 `site.url` 必须显式存在；取值只能是字面 URL 或恰好 `${LEANOTE_SITE_URL}`，
  其他 `${...}` 占位符视为来源冲突。引用 `${LEANOTE_SITE_URL}` 时，环境变量缺失/为空分别报告
  `CONFIG_VALUE_MISSING` / `CONFIG_VALUE_EMPTY`，键名为 `LEANOTE_SITE_URL`。
- R6 两个 smoke 脚本生成的 prod 配置补上合法的字面 `site.url`，保持可运行。
- R7 README Compose 章节说明 `LEANOTE_SITE_URL` 与反向代理要点（转发原始 Host、
  Nginx `client_max_body_size`），并提示修改对外地址后，此前经插图对话框写入笔记的绝对地址不会自动改写。

## Acceptance Criteria

- [x] AC1（R2）`.env` 缺少或置空 `LEANOTE_SITE_URL` 时，`docker compose config` 报错
      `LEANOTE_SITE_URL must be set`。
- [x] AC2（R4/R5）Go 单测覆盖：合法值（http/https、带端口、IP、localhost）通过；
      非法值（缺 scheme、`ftp://`、大写 scheme、末尾 `/`、路径、查询、片段、用户信息、空主机、
      非法端口、首尾空白）、`[prod]` 缺 `site.url`、非 `${LEANOTE_SITE_URL}` 占位符、
      环境变量缺失/为空均返回预期 `ConfigError` 代码与键；错误文本不含配置值。
- [x] AC3（R3/R4）以 `LEANOTE_SITE_URL=https://note.example.com` 启动的生产配置，
      `ParseConfig` 得到的 `site.url` 为 `https://note.example.com`，`GetDefaultDomain()` 为
      `note.example.com`。
- [x] AC4（R1–R3）`release-contract` 测试断言 `.env.example`、Compose、`app.conf-docker`
      中的 `LEANOTE_SITE_URL` 契约并通过。
- [ ] AC5（R6）`scripts/container-smoke.sh`、`scripts/package-smoke.sh` 生成的配置能通过新校验；
      真实 smoke 执行依赖 Docker/Mongo，本地未执行，状态为 `unrun`。
- [x] AC6 Go 检查（`go build ./...`、`go vet ./...`、`go test ./app/httpserver ./app/service ./cmd/leanote`）
      与新增 Node 合约用例通过；`git diff --check` 通过。完整 `npm test` 在 Windows
      受既有 `sh`/CRLF 环境失败影响，需在 Linux/CI 复核。
- [ ] AC7（可选，真实环境）重建容器后反代访问：`/note` 输出的 `UrlPrefix` 为对外地址，
      插图对话框插入的图片以对外地址开头，博客默认路径不再 404；真实容器/反代验证未执行，状态为 `unrun`。

## Out of Scope

- 笔记内容中绝对地址的数据迁移工具（尚未上线，无存量数据）。
- 改变富文本插图对话框/附件链接生成绝对地址的前端行为。
- 后台"站点 URL"设置的持久化语义（启动时仍以配置文件为准）。
- `applySiteURLDomain` 既有的端口推导细节（如 https 无端口时的 `port` 值）。
- 开发/测试模式（`conf/app.conf` 的 `[dev]`、`[test]`）的 site.url 校验。
- 生成 Nginx/Caddy 配置文件；发布新的 GHCR 镜像版本。
- 修改开发者本地未入库的 `.env`（由用户自行补充 `LEANOTE_SITE_URL`）。
