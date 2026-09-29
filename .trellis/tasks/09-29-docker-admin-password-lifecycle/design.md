# 技术设计

## 管理员身份与 Docker seed 接管

Docker 生产配置新增 `LEANOTE_ADMIN_EMAIL`、`LEANOTE_ADMIN_INITIAL_PASSWORD`、`LEANOTE_ADMIN_FORCE_ENV_PASSWORD`。`ConfigService` 在 Docker 模式消费初始化服务返回的管理员 `UserId` 和实际 `Username`，不再依赖 `adminUsername=admin` 找人；非 Docker/test 模式保留现有配置行为。管理员用户名由邮箱 `@` 前缀生成，清洗后不足 4 个字符时配置失败。

`mongo-seed` 仍恢复安装数据，但应用在同一数据库内使用 `configs` 集合的 `adminBootstrapCompleted` 标记执行一次 bootstrap：

1. 标记不存在时先查配置邮箱。若对应用户存在且唯一，使用其 UserId；若不存在，查找种子管理员 `Username=admin`，保留其 UserId 并接管邮箱/用户名。重复邮箱、缺少种子管理员或身份不唯一均失败。
2. 对选定用户原子写入规范化邮箱、前缀用户名、`GenPwd(initial)`、生命周期字段和初始 ENV 指纹，不创建第二管理员。
3. 写入脱敏的 bootstrap 标记，不保存密码、哈希或指纹原文。
4. 将种子 `demo` 用户标记为 `Disabled=true`；认证服务统一拒绝 Disabled 用户。该步骤只由 Docker 生产 bootstrap 执行，不改变 `leanote_test` fixture。

标记存在时只读取并校验选定管理员，禁止环境变量覆盖数据库 `Pwd`、邮箱、用户名或状态。bootstrap 使用事务或幂等补偿，部分失败不得报告成功。

## 持久化状态

在 `users` 文档增加兼容缺失字段：`AdminPasswordSetupRequired bool`、`AdminEnvPasswordFingerprint string`（应用 secret HMAC-SHA256）和 `Disabled bool`（缺失按 false）。

首次 bootstrap 即设置 `AdminPasswordSetupRequired=true`，避免首次登录成功但门禁尚未落库。旧部署没有这些字段的既有管理员按已完成改密处理；只有 Docker bootstrap 标记不存在时才执行 seed 接管。所有密码写入入口——`UserService.UpdatePwd`、`UserService.ResetPwd`、`PwdService.UpdatePwd` 和 admin reset action——成功写入新哈希时清除门禁；强制改密路径不校验旧密码，并统一走同一 helper。

## 登录与恢复

认证逻辑集中在 `AuthService`：

- 普通登录始终比较数据库 `Pwd`；ENV 初始密码不参与。
- 管理员处于 `AdminPasswordSetupRequired=true` 且输入 ENV 初始密码时允许首次流程，数据库门禁已存在，Web/API 使用同一状态。
- 管理员状态为 false、恢复开关开启且 ENV 指纹不同于 `AdminEnvPasswordFingerprint` 时，允许一次恢复登录；成功后条件更新 `AdminPasswordSetupRequired=true` 和当前指纹，再签发 Web session/API token。更新失败不得签发认证成功。
- 指纹相同表示 ENV 密码已消费，拒绝恢复并记录不含凭据的告警；更换 ENV 密码后指纹不同才可再次恢复。
- Disabled 用户在 Web、API 和其他认证入口统一返回无效凭据。

因此 API 与 Web 共享数据库门禁；所有并行会话/token 后续请求都经过统一门禁，不能依赖只存在 session 的标记。

## 强制改密门禁与改密

认证后的统一 Web/API 身份边界读取当前管理员文档状态。`AdminPasswordSetupRequired=true` 时只放行改密页面、改密 POST、API 改密、静态资源和登出；其他请求返回 `admin_password_change_required`。数据库状态优先于 token/session 缓存。

强制改密调用独立服务方法，不要求 `oldPwd`：校验新密码后执行类似 `{_id: adminID, AdminPasswordSetupRequired: true}` 的条件更新，同时写入新 `Pwd` 并清除门禁。匹配数为 0 返回状态冲突/已完成；写失败保留原密码和 true 状态。普通改密继续校验旧数据库密码。管理员重置和找回密码成功写新密码时也清除门禁，并通过同一 helper。

应用不改写宿主机 `.env`，运维需关闭恢复开关；即使忘记关闭，同一 ENV 密码也因指纹匹配而失效。

## 安全与验证

- 明文只存在于进程输入生命周期；日志、错误、配置投影、健康检查、Mongo 文档和镜像不得包含明文。
- HMAC 使用已校验应用 secret；secret 变化导致旧指纹不匹配，恢复需更换 ENV 密码。
- 普通注册不能修改 `ConfigService` 的管理员 UserId；既有非 Docker 模式保留 `adminUsername` 兼容。
- 测试覆盖配置、seed 接管、marker 幂等、demo 禁用、数据库优先、恢复新/重复指纹、Disabled、强制改密无旧密码、条件并发、所有密码写入口和 Web/API 门禁；真实 Docker/Mongo/浏览器证据单独记录。
