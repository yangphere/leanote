# 生产会话 cookie：Secure 推导与有效期可配置

## Goal

1. HTTPS（含反向代理终止 TLS）部署下，服务端签发的会话 cookie 带 `Secure`，防止会话
   cookie 经明文 HTTP 泄露；HTTP 直连部署（如 `http://localhost:9000`）保持可登录。
   不新增变量：`LEANOTE_SITE_URL` 的协议是唯一来源。
2. 生产会话有效期可通过 `.env` 的 `LEANOTE_SESSION_EXPIRES` 配置，不填默认 7 天，
   避免当前 3 小时绝对时长导致使用中被登出。

## Background (repository evidence)

- 服务端只签发一个 cookie：会话 cookie `<cookie.prefix>_SESSION`，`HttpOnly`、
  `SameSite=Lax`；`Secure` 取自 `cookie.secure`（默认 false），有效期取自
  `session.expires`（`time.ParseDuration`，默认 `3h`，非法值静默回退 `3h`），同时决定 cookie
  `Expires`/`MaxAge` 与签名载荷 `exp`（`app/httpserver/session.go:58-75`、`:98`、`:111-126`）。
  由 `cmd/leanote/main.go:166` `httpserver.NewSessionCodec(cfg)` 装配。
- 会话 cookie 只在 action 写入会话键时重新签发（`app/httpserver/registry.go:522-539`
  `applySessionCookie`），所以有效期是"自上次写会话（通常即登录）起的绝对时长"。
- `conf/app.conf-docker` 未声明 `cookie.secure` / `session.expires`；Docker 生产当前实际为
  `Secure=false`、`3h`。参考模板 `conf/app.conf-default` 写 `cookie.secure=false`；
  `conf/app.conf`（dev/test）写 `cookie.secure=false`、`session.expires=3h`（提交 `ae291196`）。
- 生产 `site.url` 必须在 `[prod]` 声明，可为 `${LEANOTE_SITE_URL}` 或字面量（打包 / 容器 smoke
  用字面量 `http://127.0.0.1:...`），经 `validateSiteURL` 校验为小写 `http://` / `https://`
  加主机（`app/httpserver/production_config.go:147-172`）。
- 生产已有禁止键机制 `forbiddenProductionKey`（`production_config.go:276`），`[prod]` 或
  DEFAULT 出现即 `CONFIG_KEY_INVALID`。
- 配置 `${VAR}` 展开遇未设置/空值视为键不存在（`app/httpserver/config.go:169-180`）；
  Compose 可选变量已有 `:-` 默认值先例（`LEANOTE_ADMIN_FORCE_ENV_PASSWORD`，`docker-compose.yml`）。
- 语言 cookie `LEANOTE_LANG` 由前端 JS 写入（`public/js/home/index.js:25-34`），服务端只读。
- 服务端不感知 TLS（无 `r.TLS` / `X-Forwarded-Proto` 处理）。
- 项目尚未正式部署（`10-05-docker-site-url-env` design 的 Compatibility 节），收紧生产配置
  契约无存量迁移。

## Requirements

### Secure

- R1 生产模式下，会话 cookie `Secure` 当且仅当已校验的 `site.url` 以 `https://` 开头时为
  `true`；env 与字面量来源一致生效。
- R2 生产模式下 `cookie.secure` 不可配置：出现在 `[prod]` 或 DEFAULT 时启动失败，
  `ConfigError{Code: "CONFIG_KEY_INVALID", Key: "cookie.secure"}`，退出码 78，不输出值。

### 有效期

- R3 生产 `session.expires` 可选：
  - 未声明，或声明为 `${LEANOTE_SESSION_EXPIRES}` 而该变量未设置/为空 → `168h`。
  - 值（字面量或 env）须能被 `time.ParseDuration` 解析，且在 `[5m, 8760h]` 闭区间内；
    否则启动失败 `CONFIG_SESSION_EXPIRES_INVALID`，Key 为 `LEANOTE_SESSION_EXPIRES`（env
    来源）或 `session.expires`（字面量），不输出值。
  - 引用其他 `${...}` 变量 → `CONFIG_SOURCE_CONFLICT` / `session.expires`。
- R4 Docker 交付：`conf/app.conf-docker` 增加 `session.expires=${LEANOTE_SESSION_EXPIRES}`；
  `docker-compose.yml` 以 `${LEANOTE_SESSION_EXPIRES:-168h}` 传入（可选，不加 `:?`）；
  `.env.example` 增加带说明的 `LEANOTE_SESSION_EXPIRES=168h`。

### 通用

- R5 dev/test 行为不变：仍读 `conf/app.conf` 的 `cookie.secure` / `session.expires`
  （默认值与非法值回退保持 `false` / `3h`）。
- R6 文档：`.env.example` 的 `LEANOTE_SITE_URL` 注释与 README Compose/反向代理段说明
  `https://` 使登录 cookie 仅经 HTTPS 发送、绕过代理 `http://` 直连无法保持登录；
  `LEANOTE_SESSION_EXPIRES` 说明格式（Go 时长，单位 `m`/`h`，无 `d`）、范围、默认值、
  绝对时长语义与修改后需重建容器。

## Acceptance Criteria

- [x] AC1 (R1) 单测：生产运行时配置在 `site.url=https://note.example.com`（env 与字面量）
      下 `CookieSecure=true`，`http://127.0.0.1:9000` 下 `false`；main 装配的会话 cookie 随之。
- [x] AC2 (R2) 单测：`[prod]` 或 DEFAULT 含 `cookie.secure` → `CONFIG_KEY_INVALID` /
      `cookie.secure`，错误串不含值。
- [x] AC3 (R3) 单测表：未声明 / env 未设置 / env 为空 → `168h`；`5m`、`8760h`、`24h` 接受；
      `4m59s`、`8761h`、`0s`、`-1h`、`7d`、`abc` 拒绝，Key 按来源区分；其他 `${X}` →
      `CONFIG_SOURCE_CONFLICT`；运行时 `SessionTTL` 驱动 cookie `MaxAge` 与载荷 `exp`。
- [x] AC4 (R4/R6) `tests/js/release-contract.test.js` 断言：`app.conf-docker` 含
      `session.expires=${LEANOTE_SESSION_EXPIRES}` 且不含 `cookie.secure`；Compose 以
      `:-168h` 传入；`.env.example` 含该变量。README / `.env.example` 已更新。
- [x] AC5 (R5) 现有 dev 路径会话测试（`session_codec_test.go`、`session_test.go`）保持通过。
- [x] AC6 本次改动的 Go 文件 `gofmt -l` 为空；`GOTOOLCHAIN=local go build ./...`、
      `go vet ./...`、相关 Go 测试、`npm test` 通过；`git diff --check` 干净。
      全仓库 `gofmt -l app cmd` 的 5 个既有问题已记录在 `validation.md`，按 AGENTS.md
      只格式化本次改动的 Go 文件，不扩大到无关文件。
- [x] AC7 真实容器 / 浏览器 HTTPS 验证明确记为 `unrun`（见 `validation.md`）。

## Out of Scope

- 滑动续期（活跃即续期）。
- 前端 JS 写入的 `LEANOTE_LANG`、`mongoMachineId` 等客户端 cookie。
- `cookie.domain` 等其他 cookie 配置的 `.env` 化。
- 服务端基于请求（`r.TLS` / `X-Forwarded-Proto`）动态判断 HTTPS。
- 已发布镜像 `2.0.1`：变更随下一次构建/发布生效。

## Decisions

- D1 Secure 只按生产 `site.url` 协议推导，不新增变量、不提供覆盖（用户 2026-10-06）。
- D2 生产配置中的 `cookie.secure` 拒绝而非忽略（沿用 `forbiddenProductionKey` 惯例）。
- D3 有效期保持绝对时长语义，只做可配置（用户 2026-10-06）。
- D4 `LEANOTE_SESSION_EXPIRES` 可选，默认 `168h`（用户 2026-10-06）。
- D5 生产非法有效期 fail-closed，范围 `[5m, 8760h]`（沿用生产配置 fail-closed 惯例）。
