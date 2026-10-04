# 为 Leanote 添加 Docker Compose 启动配置

## Goal

在仓库根目录提供一条可复现的 Docker Compose 启动路径，让当前 Leanote 应用与 MongoDB 一起运行。运行变量由本地未跟踪的 `.env` 提供，同时提交 `.env.example` 作为新克隆仓库的配置模板，并确保真实 `.env` 不进入 Git。

## Confirmed facts

- 根目录已有 `Dockerfile`，构建参数 `VERSION` 为必填，最终镜像以生产模式启动：`/app/bin/leanote -conf /etc/leanote/app.conf -runMode prod`。
- 生产配置校验强制要求 `/etc/leanote/app.conf` 是权限为 `0440` 的普通文件，并要求配置包含 `db.urlEnv=${MONGODB_URL}`、`app.secret=${LEANOTE_APP_SECRET}`、`db.dbname`、HTTP 地址/端口和绝对内容目录。
- 生产运行时使用 `/var/lib/leanote` 下的 private/public/backup/tmp 数据目录；镜像已声明对应 volume。
- `validateProductionSecret` 要求应用密钥至少 32 个 ASCII 非控制字符，并拒绝仓库默认公开密钥。
- `validateMongoURL` 要求连接串的数据库名与 `db.dbname` 一致，拒绝 `localhost`、回环地址和 `leanote_test`。
- `.gitignore` 当前没有 `.env` 文件规则；`.dockerignore` 中已有 `.env` 规则，但它不影响 Git 跟踪状态。
- `Dockerfile` 固定以 `linux/amd64` 构建 Go 二进制，因此 Compose 中的 Leanote 服务需要声明 `platform: linux/amd64`，以便在 ARM 主机上通过兼容层运行。

## Requirements

- 在根目录新增 `docker-compose.yml`，定义 MongoDB 和 Leanote 应用服务；应用服务从当前仓库构建镜像，并依赖 MongoDB 健康状态。
- 使用仓库已有的 `mongodb_backup/leanote_install_data` 增加一次性 MongoDB seed 服务：仅在 named volume 尚无集合时恢复安装数据，避免每次重启覆盖已有用户数据；Leanote 等待 seed 成功后再启动。
- 使用 MongoDB 官方 8.0 镜像和内部 Compose 网络；默认不启用 MongoDB root 认证，也不把 MongoDB 的 27017 端口映射到宿主机。Leanote 使用带数据库名的内部连接串 `mongodb://mongo:27017/leanote`，并与生产配置中的 `db.dbname=leanote` 一致。
- 为 MongoDB 和 Leanote 应用使用 named volume，避免宿主机 bind mount 的属主与容器 UID 10001 不一致；应用数据需要覆盖 private、public、backup 和 tmp 的持久化路径。
- Leanote 服务声明 `platform: linux/amd64`，并通过环境变量设置宿主机 HTTP 端口；MongoDB 健康检查使用容器内的 ping 命令。
- 新增不被 `.dockerignore` 排除的 `conf/app.conf-docker` 生产配置模板。调整 `Dockerfile`，把该模板复制到 `/etc/leanote/app.conf`，并使用 `COPY --chown=10001:10001 --chmod=0440`，确保非 root 的 Leanote 进程可读且通过现有生产校验。
- 新增根目录 `.env.example`，只提供变量名、安全的说明性占位文本和生成命令；不得放入真实密钥或可被误认为生产凭据的默认密钥。模板必须说明使用 `openssl rand -base64 48` 生成至少 32 个 ASCII 字符的 `LEANOTE_APP_SECRET`。
- 按用户要求生成根目录 `.env` 作为本地启动配置，但通过在 `.gitignore` 中增加 `/.env` 规则使其保持未跟踪；`.env` 中的 `MONGODB_URL`、`LEANOTE_APP_SECRET`、镜像版本和宿主机端口都可由用户修改。
- Compose 对必需变量使用 `${VAR:?VAR must be set}` 形式；缺少 `.env` 或关键变量时应立即报错，不得静默使用生产默认值。`LEANOTE_APP_SECRET` 必须由本地 `.env` 提供真实的随机值，不把示例占位值用于实际启动。

## Acceptance criteria

- `docker compose config` 在提供本地 `.env` 时成功解析；删除或暂时移走 `.env` 后，Compose 对缺失的必需变量明确失败。
- `docker-compose.yml` 明确构建当前 `Dockerfile`，传入有效的 `VERSION`，声明 Leanote 的 `linux/amd64` 平台、MongoDB 健康检查和一次性 seed 依赖、named volumes、非公开 MongoDB 端口以及带 `/leanote` 数据库名的连接串；首次启动后 `/healthz` 应返回 `200` 与 `{"status":"ready"}`。
- 生产配置模板路径为 `conf/app.conf-docker`，镜像内目标为 `/etc/leanote/app.conf`，权限为 `0440`、属主为 `10001:10001`，并满足现有 fail-closed 校验规则。
- `.env.example` 被 Git 跟踪，`.env` 被 `git check-ignore -q .env` 命中，且 `.env` 不出现在 Git 未跟踪文件列表中。
- 在 Docker daemon、镜像构建和 MongoDB 环境可用时，执行真实 Compose 构建/启动并通过 Leanote 健康检查；若环境不可用，必须在任务记录中明确标记真实容器验证未运行，不得用静态解析替代该证据。

## Out of scope

- 修改应用业务逻辑、数据库 schema、默认开发配置或前端资源。
- 提交真实生产密钥、用户数据或生成的 MongoDB 数据。
- 为 MongoDB 增加外部认证、宿主机端口暴露或 bind mount 数据目录。
- 推送远程仓库或执行不可逆的清理操作。

## Open questions

无。技术约束已由现有生产配置校验、Dockerfile 和用户要求确认，用户目标明确。
