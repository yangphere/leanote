# 生产与 dev Compose 拆分设计

## 行为边界与不变量

当前基础文件同时声明本地 build 与 `leanote:local`，部署路径无法区分远端版本和当前源码。将应用镜像选择拆开，服务拓扑和持久化规则继续只有一个事实来源。

- 基础文件只引用 `ghcr.io/yangphere/leanote:${LEANOTE_IMAGE_TAG:?LEANOTE_IMAGE_TAG must be set}`，无 build。
- 显式 `docker-compose.dev.yml` 只覆盖 `services.leanote.image` 为 `leanote:local`，并声明现有 build context、Dockerfile 和必填 `VERSION` 参数。
- Mongo/seed/Gotenberg、`linux/amd64`、PDF 内网、安全参数、健康检查、环境变量校验及 named volumes 保持不变。
- 不修改 Dockerfile、应用实现和当前 `.env` 凭据；GHCR 工作流仅按用户追加审查修复范围修改。

## 版本与插值契约

完成的前置任务发布无 `v` 前缀的 `2.0.1`，生产直接消费该版本。`LEANOTE_IMAGE_TAG` 指定已发布的精确 `X.Y.Z`，不默认到 latest；`.env.example` 提供已发布的 `2.0.1` 作为操作示例。`LEANOTE_VERSION=0.0.0` 只影响 dev 的本地构建元数据。

Compose 对单个文件先插值再合并，所以 dev 仍须提供基础文件的 `LEANOTE_IMAGE_TAG`，但合并结果使用本地镜像。保留必填表达式并明确文档说明，不增加兼容 fallback 或额外版本校验器。严格版本生成与发布仍由现有共享版本契约负责。

## 部署和兼容

生产启动为 config → pull → up，无源码构建；升级修改 `.env` 中的 `LEANOTE_IMAGE_TAG` → pull leanote → recreate leanote。dev 的所有命令显式传两个 `-f` 参数，源码更新时 build → recreate。seed 仍挂载仓库的 `mongodb_backup/leanote_install_data`，生产需要保留此目录。

README、环境模板、交付文档、smoke 注释和 release-contract 同步到同一契约。现有默认 Compose 的用户若希望继续源码构建，需显式添加 dev override。

## 验证与回退

使用非敏感 fixture 环境独立渲染两种组合，覆盖缺少/空 `LEANOTE_IMAGE_TAG` 和 dev 缺少 `LEANOTE_VERSION` 的错误，并检查生产不依赖 dev 版本。执行 focused release-contract 与完整 npm test。

运行验证使用独立项目名和空的新卷、loopback 测试端口，不替换当前开发容器。真实拉取已发布镜像，检查非 root、镜像身份、健康请求及重新创建后数据仍可用。仅清理本次独立项目资源；不执行 `down -v`，不改当前 `.env`。回退消费端改动即可恢复原配置；本任务不修改数据库或远端镜像。

## 追加：GHCR 审查修复设计

这是共享发布状态的结构性修复：两个发布入口共用一个 latest 晋升规则，禁止旧版回拨且不重复定义 SemVer 格式。

- 在现有 `scripts/version.mjs` 的严格格式校验上增加数值比较，使用逐段 BigInt 避免字典序、浮点精度和新版本解析器。
- 新增小型纯逻辑/文件 CLI 晋升 helper，消费 Skopeo 的成功包标签列表和原始 OCI config。先确认 Repository 与固定目标一致、Tags 结构正确；仅不存在 latest 时允许初始化；存在时必须读取合法的 `config.Labels[org.opencontainers.image.version]` 并与候选比较。返回明确 promote/skip；文件、认证、网络和 JSON 异常不能当作缺失。
- 使用已固定摘要的 Skopeo `list-tags` / `inspect --config` 读取 latest 元数据；所有请求保留认证参数。新版本原始 manifest 校验后及 latest-only 的 smoke/来源复查后才执行共用 guard，并且 guard 必须位于 latest copy 前。
- 低于或等于当前 latest 的版本成功完成其既有版本验证后，跳过 latest copy/read-back并输出可识别说明。latest 缺失仅在成功、身份绑定的 listing 确认后初始化。版本 immutable 与 latest 字节/digest 绑定规则保持。
- 共享锁启用 `queue: max`，最多 100 pending，保持运行不取消；这是保留等待运行而非无限队列或按版本排序。完整工作流继续串行，版本 guard 独立保证 latest 不回拨。
- 删除工作流永久建包许可，现有 helper 默认 false 不变；保留显式首次建包的底层能力与测试，但普通发布不启用。
- GitHub 按事件 SHA/ref 选择工作流，tag push 使用候选提交内的版本。本地修改不能追溯加固历史工作流；禁止给不含修复的旧提交补推版本 tag。新 tag 必须包含修复，恢复 dispatch 在已修复的 main 上执行。强制约束旧工作流需另行授权远端 tag 规则或可信固定发布入口，本轮仅记录边界。

官方并发文档来源和摘录记录在 `research/ghcr-review.md`。仅作本地实现/测试和只读远端检查，不触发发布或改写现有版本。
