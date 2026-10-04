# docker-compose 拆分生产与 dev 配置

## 目标

`docker-compose.yml` 当前把 `leanote` 服务的 `build` 与 `image: leanote:local` 写在一起，生产与开发无法区分。拆分后：

- 生产：使用推送 `X.Y.Z` git tag 时由 `docker-image.yml` 发布到 GHCR 的不可变版本镜像（前置任务 `10-04-ghcr-tag-image-publish`）。
- dev：保持现有"从当前源码构建镜像"的方式。

## 已确认决策（grill 结论）

1. **布局**：`docker-compose.yml` 是生产基础文件，引用 `ghcr.io/yangphere/leanote:${LEANOTE_IMAGE_TAG:?LEANOTE_IMAGE_TAG must be set}`；新增 `docker-compose.dev.yml` 作为 override，只给 `leanote` 加 `build`（含 `VERSION` 构建参数）和 `image: leanote:local`。mongo、mongo-seed、gotenberg、网络、卷只在基础文件中写一份。dev 启动命令：`docker compose -f docker-compose.yml -f docker-compose.dev.yml ...`。不使用 `compose.override.yml` 自动加载。
2. **变量语义**：生产使用新变量 `LEANOTE_IMAGE_TAG`，取值是不带 `v` 的三段版本号（如 `2.0.1`，对应 git tag 与镜像标签 `2.0.1`），必填、无默认值、不使用 `latest`。`LEANOTE_VERSION` 只作为 dev 构建参数保留（三段非负整数，`0.0.0` 用于本地构建）。镜像仓库路径固定写在 compose 中。Compose 在合并 override 前插值每个文件，因此 dev 也必须提供 `LEANOTE_IMAGE_TAG`；dev 的最终镜像仍是 `leanote:local`，该值不用于拉取生产镜像。
3. **seed 数据**：`mongo-seed` 保持挂载 `./mongodb_backup/leanote_install_data`，生产部署仍需要仓库中的 `mongodb_backup/` 目录；在 README 中明确说明。"只用一个 compose 文件即可部署"另立任务，不在本任务范围。
4. **范围**：compose 文件、`.env.example`、`README.md`、`scripts/container-smoke.sh` 注释、`tests/js/release-contract.test.js` 的 compose 断言、`docs/modernization/cicd-delivery.md` 的 compose 描述。
5. **镜像发布时机**：前置任务的最终契约是无前缀 `X.Y.Z` 版本与 `latest` 别名，`2.0.1` 已实际发布且匿名拉取通过。生产 Compose 不使用可变 `latest`。用户后续授权修复 GHCR 审查问题，追加范围与验收见下节。

2026-10-04 实施前同步：原规划的 `v` 前缀及“GHCR 尚无镜像”已被完成的前置任务取代。本次用户批准规划并开始实施，按其最终发布契约同步消费端。

## 约束

- 保持 `platform: linux/amd64`、非 root 运行、gotenberg 固定摘要与安全参数、`pdf` 内部网络不变。
- 不放宽现有必填环境变量校验（`:?`）。
- 不修改 `Dockerfile`；工作流改动仅限下节用户授权的 GHCR 修复。

## 验收标准

- `docker compose config --quiet` 在仅生产（带 `LEANOTE_IMAGE_TAG`）和 生产 + dev 两种组合下均通过；缺少 `LEANOTE_IMAGE_TAG` 时生产配置报必填错误。
- 生产组合渲染出的 `leanote` 服务镜像为 `ghcr.io/yangphere/leanote:<tag>`，且无 `build`；dev 组合渲染出 `build` 与 `leanote:local`。
- `npm test` 中 release-contract 测试通过，并新增对生产镜像引用、`LEANOTE_IMAGE_TAG` 必填、dev override 含 `build` 的断言。
- README 与 `.env.example` 分别描述生产与 dev 的启动、升级命令。
- 使用独立 Compose 项目、端口和卷真实拉取并运行已发布的 `2.0.1`，验证 `/healthz`；不替换当前运行中的开发堆栈。若环境阻塞则如实记录 `unrun` 或 `partial`，不能用配置渲染替代运行证据。

## 非目标

- 不增加发布入口或新标签、不做 ARM64（MOD-002）。
- 不把种子数据打进镜像。

## 追加范围：GHCR 审查修复

2026-10-04 用户要求修复三个 GHCR 审查问题，授权在现有任务中追加以下行为，不新建任务或修改归档任务历史：

- `latest` 代表成功晋升的最高严格 `X.Y.Z` 版本。新发布及 `update_latest` 对旧版/同版跳过 latest 写入，仍可发布缺失的旧版不可变版本镜像。只有确认包标签列表中无 latest 时允许初始化别名；读取/认证/JSON/元数据异常明确失败。
- 并发保持共享 `docker-image-latest` 锁及 `cancel-in-progress: false`，启用官方 `queue: max`，避免默认单 pending 的替换取消；100 个 pending 的上限和人工恢复步骤须记录。
- 首发完成后，工作流不再启用 `ALLOW_INITIAL_PACKAGE_CREATE`，包缺失时使用现有默认拒绝规则；显式首发 helper 的旧测试保留。
- 改动限 `.github/workflows/docker-image.yml`、必要的共享版本/晋升 helper、`tests/js/docker-image-workflow.test.js`、ADR-0005、交付文档与适用 spec。已有 Compose 改动、用户 `CONTEXT.md`、本地 `.env`、Dockerfile 保留。
- 新增针对版本数值顺序（包含两位数/超大分量）、相等/旧版跳过、latest 缺失与无效元数据拒绝、两条晋升路径的 guard 顺序、队列配置及建包默认拒绝的回归测试；相关 focused 和完整 npm test 必须通过。
- 不推送或改写现有远端 tag/manifest，不触发 Actions；真实 GitHub 队列行为和远端新版晋升验证仍须单独记录为 unrun。
- 明确历史工作流边界：tag push 执行 tagged commit 中的工作流，本次规则不会追溯加固旧提交；禁止向不含修复的旧提交补推版本 tag，新 tag 和 main recovery 执行器必须包含本次修复。强制远端 tag 约束不在本轮范围。
