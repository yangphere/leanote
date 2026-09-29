# Docker 管理员环境密码初始化与强制改密

## 目标

Docker Compose 启动的 Leanote 使用 `.env` 配置管理员邮箱和初始密码。初始密码只用于 Docker 首次初始化或显式、一次性的恢复入口；管理员完成改密后，正常登录只以 MongoDB 中保存的密码哈希为准。

## 需求

- `.env` 提供 `LEANOTE_ADMIN_EMAIL`、`LEANOTE_ADMIN_INITIAL_PASSWORD` 和可选的 `LEANOTE_ADMIN_FORCE_ENV_PASSWORD`。
- 管理员用户名默认取邮箱 `@` 前的字符串，并按现有用户名清洗规则归一化；邮箱前缀不足 4 个字符时必须拒绝配置，不能静默生成 `user-<id>`。
- Docker 模式以 `LEANOTE_ADMIN_EMAIL` 对应的数据库用户为管理员身份事实，使用该用户实际的 `UserId`/`Username` 初始化 `ConfigService`，不再依赖 `adminUsername=admin`。
- 首次 Docker 初始化使用 `configs` 集合中的一次性 `adminBootstrapCompleted` 标记：若配置邮箱用户不存在，则接管种子管理员 `Username=admin` 的 UserId，更新其邮箱、用户名、初始密码哈希和管理员状态，而不是新建普通用户；身份冲突、种子管理员缺失或不唯一时启动失败。
- Docker bootstrap 禁用种子 `demo` 账号；认证遇到禁用账号必须失败。测试数据库既有 fixture 不由生产 bootstrap 改写。
- 首次 bootstrap 保存初始密码哈希、`AdminPasswordSetupRequired=true` 和初始密码 HMAC 指纹。明文不得进入 MongoDB、日志、响应、镜像或可序列化生产配置。
- 已完成 bootstrap 的重启不得用 `.env` 覆盖 `Pwd`、邮箱、用户名或强制改密状态。
- `AdminPasswordSetupRequired=true` 是数据库持久化门禁。Web 和 API 只允许登录后改密页面、改密接口、静态资源和登出，其他请求返回 `admin_password_change_required`。
- 管理员成功改密必须以条件更新同时写入新密码哈希并清除门禁；失败时保留旧密码和门禁状态。强制改密不校验旧数据库密码。
- `LEANOTE_ADMIN_FORCE_ENV_PASSWORD` 默认关闭且严格解析。启用后，管理员可用当前 `.env` 初始密码恢复，但成功登录会把门禁写入数据库，Web/API 和并行会话统一受限。
- 恢复为一次性：用户文档保存已消费的 ENV 密码 HMAC 指纹；同一指纹再次恢复必须拒绝。再次恢复必须更换 `.env` 初始密码；开关开启且已消费时启动日志发出不含凭据的告警。
- 恢复改密成功后，应用不修改 `.env`，但清除数据库门禁并记录本次指纹；运维必须移除/关闭开关。开关仍开着时同一 ENV 密码也不能再次恢复。
- 页面注册继续创建普通用户，不得覆盖管理员或绕过门禁。

## 验收标准

- 缺少/非法管理员配置、邮箱前缀不足 4 个字符、密码强度不足、恢复开关非法或管理员身份不唯一时生产启动 fail closed。
- 首次 seed 后配置邮箱成为管理员，种子 UserId 被保留并接管，`demo` 不可登录，不产生第二管理员。
- 用户名等于邮箱 `@` 前缀归一化结果，数据库只保存密码哈希和 HMAC 指纹。
- 初始密码登录后 Web、API、并存会话均受门禁；改密成功后新密码可用，旧密码和初始密码失效。
- 已完成 bootstrap 的重启不覆盖数据库密码；密码和门禁状态跨重启持久化。
- 忘记密码时使用未消费的新 ENV 密码恢复并强制改密；同一 ENV 密码再次失败，关闭开关后 ENV 密码失败。
- 恢复改密不依赖旧数据库密码，条件更新避免并发请求错误清除状态。
- 普通注册、找回密码、管理员重置密码等所有密码写入口都有明确的门禁处理。
- 配置、初始化、登录、Web/API 门禁、恢复一次性消费、失败回滚、重启和注册隔离均有确定性测试；真实 Docker/Mongo/浏览器证据单独记录。

## 约束与范围

- 复用现有 `GenPwd`/`ComparePwd` 和服务边界；所有新密码写入使用现有哈希生成器。
- HMAC 使用已校验的应用 secret，只保存指纹；不提供隐式默认恢复密码。
- 不改变测试 fixture 用户数量和凭据；生产 Docker seed 的 `demo` 禁用属于本任务范围。
- 不实现新的邮件找回流程，不向宿主机暴露 MongoDB。
