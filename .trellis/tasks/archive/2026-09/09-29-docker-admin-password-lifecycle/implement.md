# 实施计划

1. 配置边界：扩展 `.env.example`、Compose、Docker 生产配置和解析，加入邮箱/初始密码/恢复开关、严格校验、前缀规则和脱敏。
2. 用户模型与数据库 helper：增加 `AdminPasswordSetupRequired`、`AdminEnvPasswordFingerprint`、`Disabled`；实现 HMAC 指纹、条件更新和统一密码写入状态转换。
3. Docker bootstrap：在 seed 后执行 marker 驱动的管理员邮箱认定、种子 admin UserId 接管、初始哈希写入和 demo 禁用；验证冲突、幂等重启与部分失败。
4. 管理员配置事实源：Docker 模式让 ConfigService 消费 bootstrap 的实际 UserId/Username，保留非 Docker/test 的 `adminUsername` 兼容。
5. 认证收敛：正常登录只验证数据库密码；恢复开关只接受未消费指纹的 ENV 密码，并在签发 Web/API 认证前持久化门禁状态。
6. 统一门禁：在 Web/API 认证边界读取数据库状态，限制所有并行 session/token；覆盖恢复登录、普通登录、静态资源、登出和改密路径。
7. 改密入口：新增不校验旧密码的强制改密服务；让普通改密、管理员重置、找回密码和 admin reset 通过统一 helper 清除门禁，使用条件更新并补失败/并发测试。
8. 注册与 Disabled：验证普通注册不会成为管理员，Disabled demo 在所有认证入口失败，测试 fixture 不被生产 bootstrap 改写。
9. 验证：运行 `gofmt`、针对性 Go 测试、`go vet`、`git diff --check`、Trellis validate 和 `docker compose config`；Docker daemon 可用时顺序执行真实 Compose 初始化、重启、恢复和浏览器验收。
10. 最终复核：扫描密码/指纹泄露，记录未运行的真实环境证据，并完成实现阶段收口。

## 2026-09-29 配置说明与本地同步

- `.env.example` 的全部 8 个配置项已补充中文用途、可选值或格式、约束和生效方式；约束依据 Compose、Dockerfile、生产配置校验和管理员密码服务。
- 本地 `.env` 同步模板注释，原有 5 项配置原样保留；补入示例管理员邮箱 `admin@example.com`、安全随机生成的 48 位十六进制初始密码及 `false` 恢复开关。真实凭据不写入任务材料；邮箱尚需由部署者确认。
- `docker compose --env-file .env config --quiet` 通过；配置键完整性、已有赋值保留和写后读取检查通过。
- `GOTOOLCHAIN=local go test ./app/service ./cmd/leanote -run 'Test(ParseAdminForceEnvPasswordIsStrict|AdminPasswordFingerprintUsesSecretAndDoesNotExposePassword|DockerAdminUsernameNormalizesEmailPrefix|ConfigureDockerAdminPasswordRejectsInvalidInput|ProductionConfigSatisfiesAppConfigSeam)$' -count=1 -timeout=60s` 通过。
- 本次仅同步配置和现有行为说明，无新增运行时契约，未改业务代码或新增全局规范；未启动或重建容器，真实 Docker/Mongo/浏览器验收仍未运行，不代表整个任务完成。
