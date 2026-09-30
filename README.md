# Leanote

Leanote 是一个开源的个人知识管理和笔记应用，支持富文本、Markdown、标签、笔记本、分享、博客和 PDF 导出。本仓库包含 Leanote Web 服务端、模板、前端资源和 Docker 部署配置。

## 当前项目结构

- `cmd/leanote`：Go 服务入口。
- `app/httpserver`：HTTP 服务、路由、会话和 `/healthz`。
- `app/controllers`、`app/service`、`app/application`：请求适配器和业务服务。
- `app/views`、`public`、`messages`：模板、浏览器资源和语言包。
- `docker-compose.yml`、`Dockerfile`：本地 Docker Compose 部署。
- `docs/`：CI/CD、生产配置和交付约定。

生产镜像使用 Go 1.26 和 Node.js 24 构建，当前 Compose 部署目标为 `linux/amd64`。MongoDB 使用 Compose 中固定版本的 MongoDB 8.0 镜像。

## 使用 Docker Compose 部署

### 首次部署

在仓库根目录执行：

```powershell
Copy-Item .env.example .env
```

编辑 `.env`，至少替换以下值：

- `LEANOTE_APP_SECRET`：至少 32 字节的 ASCII 密钥，例如 `openssl rand -base64 48` 的输出。
- `LEANOTE_ADMIN_EMAIL`：首次初始化管理员邮箱。
- `LEANOTE_ADMIN_INITIAL_PASSWORD`：管理员初始密码。

其余配置项及允许值见 [`.env.example`](.env.example)。`.env` 包含凭据，不要提交到 Git。

首次启动完整 Compose 堆栈：

```powershell
docker compose config --quiet
docker compose up -d --build --force-recreate
docker compose ps
Invoke-WebRequest http://127.0.0.1:9000/healthz
```

这个项目是从当前源码重新构建本地镜像，镜像名固定为 `leanote:local`。修改程序后，在仓库根目录执行：

```powershell
git pull

docker compose config --quiet
docker compose build leanote
docker compose up -d --no-deps --force-recreate leanote

docker compose ps
Invoke-WebRequest http://127.0.0.1:9000/healthz
```

也可以合并为：

```powershell
docker compose up -d --build --force-recreate leanote
```

如果基础镜像也需要更新：

```powershell
docker compose build --pull leanote
docker compose up -d --no-deps --force-recreate leanote
```

启动后默认通过 <http://127.0.0.1:9000> 访问。`/healthz` 在 HTTP 服务和 MongoDB 都就绪时返回 HTTP 200 及 `{"status":"ready"}`；初始化或 MongoDB 暂不可用时会返回 HTTP 503 及 `{"status":"not_ready"}`。

### Compose 数据和重建规则

当前 Compose 使用 named volumes 保存数据：

- `mongo-data`：MongoDB 数据库、用户和笔记数据。
- `leanote-data`：Leanote 的私有文件、公开上传、备份和临时目录。

重新构建和重建 `leanote` 容器不会删除已有用户和笔记数据。`mongo-seed` 只在内置 MongoDB 还没有集合时恢复 `mongodb_backup/leanote_install_data`，不会在每次启动时覆盖已有数据。

注意：

- 不要执行 `docker compose down -v`，否则会删除 MongoDB 和 Leanote 数据卷。
- 修改 `.env` 后必须重新创建容器，单纯 `restart` 不会更新环境变量。
- 修改 Go、模板或前端代码后必须重新 `build`，旧容器不会自动使用新代码。
- 查看启动日志：

  ```powershell
  docker compose logs -f leanote
  ```

如需停止服务，可以使用 `docker compose stop`；再次执行 `docker compose up -d` 即可启动已有容器和数据卷。

### Compose 服务

| 服务 | 作用 |
| --- | --- |
| `mongo` | MongoDB 8.0，数据写入 `mongo-data`，不向宿主机发布端口。 |
| `mongo-seed` | 等待 MongoDB 健康后执行一次性安装数据恢复。 |
| `leanote` | 从当前 `Dockerfile` 构建 `leanote:local`，容器内监听 9000。 |

宿主机端口由 `LEANOTE_HTTP_PORT` 控制，默认是 `9000`；容器内部端口始终为 `9000`。如果修改了宿主机端口，访问健康端点时也要同步修改 URL。

## 本地开发

### 环境要求

- Go 1.26。
- Node.js `>=24 <25` 和 npm。
- 本地 MongoDB，或先启动 Compose 中的 MongoDB 服务。

安装前端依赖并构建资源：

```powershell
npm ci
npm run build
```

使用仓库默认开发配置启动 Go 服务：

```powershell
$env:GOTOOLCHAIN = "local"
go run ./cmd/leanote -runMode dev
```

开发配置默认连接 `127.0.0.1:27017` 的 `leanote` 数据库，并监听 `9000`。生产模式使用 Docker 提供的 `/etc/leanote/app.conf` 和环境变量接口；不要把 `conf/app.conf` 中的开发默认密钥用于生产。

## 测试和检查

常用检查命令：

```powershell
$env:GOTOOLCHAIN = "local"
go build ./...
go vet ./...
go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote
npm test
```

需要 MongoDB 和独立 `leanote_test` 数据库的集成测试：

```powershell
go test ./app/tests/...
```

构建或单元测试不能替代真实 Docker、MongoDB、浏览器和跨进程验收；需要这些证据时，应单独执行对应的运行环境检查。

## 生产配置和数据路径

生产入口固定为：

```text
/app/bin/leanote -conf /etc/leanote/app.conf -runMode prod
```

生产配置通过 `MONGODB_URL` 和 `LEANOTE_APP_SECRET` 注入。应用数据路径为：

```text
/var/lib/leanote/private/files
/var/lib/leanote/private/quarantine
/var/lib/leanote/public/upload
/var/lib/leanote/public/quarantine
/var/lib/leanote/backup
/var/lib/leanote/tmp
```

完整的生产配置、发布流程、卷迁移和支持矩阵见 [`docs/modernization/cicd-delivery.md`](docs/modernization/cicd-delivery.md)。

## 相关项目和文档

- [Leanote Desktop](https://github.com/leanote/desktop-app)
- [Leanote iOS](https://github.com/leanote/leanote-ios)
- [Leanote Android](https://github.com/leanote/leanote-android)
- [项目 Wiki](https://github.com/yangphere/leanote/wiki)
- [问题反馈](https://github.com/yangphere/leanote/issues)

## 贡献和许可证

欢迎通过 [Pull Request](https://github.com/yangphere/leanote/pulls) 贡献代码或提交问题。项目采用 GPL v2 许可证，具体以仓库现有版权声明为准。
