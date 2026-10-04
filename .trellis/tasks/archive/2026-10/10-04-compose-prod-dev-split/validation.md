# 验证记录

日期：2026-10-04。任务状态：in_progress，用户已批准规划并开始实施；本轮不提交、归档或推送。

## 依赖与版本同步

- `task.json.meta.depends_on=["10-04-ghcr-tag-image-publish"]`；前置归档任务状态 completed。
- 前置任务最终发布无前缀 `2.0.1` 及同摘要 latest。消费端采用精确 `2.0.1`，同步了原 PRD 的过期 `v1.0.0` 和“尚无镜像”条件。
- 当前分支 dev，任务 base_branch 修正为现有 main；保留用户已有 `CONTEXT.md` 的 8 行改动。

## 真实 Compose 配置验证

环境：Docker Compose v5.5.1、Node v24.21.0、Docker Desktop Linux/amd64。通过临时 fixture 提供配置，清除子进程中相关继承变量，不读取或修改现有 `.env` 凭据。

| 检查 | 结果 | 断言 |
|---|---|---|
| 仅基础文件 `config --quiet`/JSON 渲染 | passed | image 为 `ghcr.io/yangphere/leanote:2.0.1`，无 build |
| 基础 + dev override 渲染 | passed | image 为 `leanote:local`，build VERSION 为 `0.0.0` |
| 共享拓扑比较 | passed | Mongo/seed/Gotenberg、网络、卷在两种组合的渲染结果相同 |
| 平台与 PDF 网络 | passed | 两种组合均 linux/amd64；pdf.internal=true |
| 缺少/空 `LEANOTE_IMAGE_TAG` | passed | 两种组合均失败且明确 `LEANOTE_IMAGE_TAG must be set` |
| 生产缺少 `LEANOTE_VERSION` | passed | 生产渲染成功 |
| dev 缺少 `LEANOTE_VERSION` | passed | 明确 `LEANOTE_VERSION must be set` |

可复现命令（fixture 含 `.env.example` 同名字段及测试凭据）：

```powershell
docker compose --env-file <fixture> -p <isolated-project> -f docker-compose.yml config --quiet
docker compose --env-file <fixture> -p <isolated-project> -f docker-compose.yml config --format json
docker compose --env-file <fixture> -p <isolated-project> -f docker-compose.yml -f docker-compose.dev.yml config --quiet
docker compose --env-file <fixture> -p <isolated-project> -f docker-compose.yml -f docker-compose.dev.yml config --format json
```

## 真实 GHCR 与生产运行验证

使用空的独立 Docker auth config，显式连接当前 Desktop Linux engine。匿名 `docker pull --platform linux/amd64 ghcr.io/yangphere/leanote:2.0.1` 成功，已缓存层复用；不是重新构建的候选镜像。

- manifest digest：`sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`。
- config/实际容器 Image Id：`sha256:99b11b4586b2ea0549b2264bd80bc736fd57216ea2b0a7a4e7cca5661b9ee6a8`。
- OCI version：`2.0.1`；revision：`dea2306c32f27e648d9438cbc8b67cec6151b6cc`；Linux/amd64。
- 测试项目：`leanote-compose-split-check-01a1055b`；仅监听 `127.0.0.1:62818`；新建该项目专属 mongo-data/leanote-data 卷。

| 检查 | 结果 | 实际证据 |
|---|---|---|
| 整个生产组合匿名 `compose pull` | passed | 应用、Mongo、Gotenberg 均 pull 成功 |
| 基础 Compose `up -d --no-build --wait --wait-timeout 60` | passed | Mongo/Gotenberg healthy、seed exit 0、应用启动 |
| HTTP `/healthz` | passed | 200，`{"status":"ready"}` |
| 非 root 用户 | passed | 实际 `id` 返回 uid/gid 10001 |
| 全堆栈 `--force-recreate` | passed | 重建后重新请求健康端点仍 200/ready |
| 应用卷持久化 | passed | `/var/lib/leanote/tmp/compose-split-check` 标记重建后保留 |
| Mongo 卷与 seed 不覆盖 | passed | 独立测试集合标记重建后 count=1，重建 seed exit 0 |
| PDF 内网 | passed | 实际 network inspect 的 Internal=true |
| 原开发堆栈保持 | passed | 容器仍 `leanote:local`，Id `bbeb109202166a924d754a7db44e1e237181028c16d47f9c4db3931f274d27d7`，9000 `/healthz` 仍 200/ready |

## 自动测试和复核

- focused release-contract：passed，31/31；Windows 子进程 PATH 补充 Git sh 后执行。
- 完整 `npm test`：passed，exit 0；242 tests，241 passed，0 failed，1 skipped，405754.4504ms。Windows 子进程 PATH 包含 Git sh。实现代理曾误启动重复实例，已按其精确 PID 树终止；以上结果来自随后唯一完整结束的单实例，不能把被中断的运行视为通过。
- Trellis check 全范围复核：实现/测试/规格/任务元数据四层通过，无代码 findings；新增 Compose contract 1/1、`node --check`、`git diff --check` 与 task.py validate 均通过，无代码修复。最终评审见 `check-review.md`。
- 任务上下文校验：passed（初次 Compose 阶段 implement/check 各 4 个有效引用；追加 GHCR research 后各 5 个）。
- Spec 已同步生产/dev 消费契约及合并前插值约束。
- `git diff --check`：passed。

## 证据边界与清理

- 新 dev override 的实际源码 build/up：unrun；已有开发堆栈健康不等于新配置构建验收。dev 配置已真实渲染验证。
- 浏览器业务、PDF 实际导出和完整 container-smoke：unrun；本任务未修改应用、镜像或 PDF 配置。
- 当前开发堆栈、当前 `.env` 与 Dockerfile 未修改。初次 Compose 阶段不修改发布工作流；用户追加授权后仅修复 `.github/workflows/docker-image.yml`，未触发远端运行。
- 独立测试容器/网络：已通过独立项目的 `compose down` 清理；没有执行 `down -v`。
- 组合清理命令包含 Docker 卷删除和递归临时目录删除，执行前被自动审批拒绝，原始理由 `blocked by policy`，未发生删除。随后缩小为仅清理容器和网络，保留两个本次生成的测试卷 `leanote-compose-split-check-01a1055b_mongo-data` / `leanote-compose-split-check-01a1055b_leanote-data` 及临时 fixture 目录；不再请求扩大清理权限。

## 追加：GHCR 三项审查修复

用户于 2026-10-04 明确要求修复 GHCR 审查问题，授权扩展现有任务；本节为追加变更证据，不以先前发布成功代替新版工作流验收。

- latest：两个发布入口共用 `check-latest-promotion.mjs`，复用 `version.mjs` 的严格格式与 BigInt 比较；旧版/同版跳过 latest，成功且身份绑定的 listing 无 latest 才可初始化。
- 并发：官方文档确认 `queue: max` 支持最多 100 pending，并禁止与 cancel-in-progress:true 同用；保留现有共享锁和 false。来源摘录见 `research/ghcr-review.md`。
- 建包：删除工作流 `ALLOW_INITIAL_PACKAGE_CREATE:true`；底层 helper 默认 false 及其显式首发测试不变。
- 历史边界：GitHub 官方文档明确每次运行使用事件 SHA/ref 内的工作流，push 可触发未合入默认分支的工作流。tag push 不会自动采用已修复的 main 工作流；本次规则仅保护包含修复的工作流及已修复 main 的 recovery。禁止向不含修复的历史提交补推版本 tag；未修改远端规则或旧 tag，也不宣称已经强制阻止历史工作流发布。来源摘录见 `research/ghcr-review.md`。

| 检查 | 结果 | 证据 |
|---|---|---|
| 新 GHCR focused 测试 | passed | docker-image-workflow 19/19；组合 release-contract + docker-image-workflow 50/50，exit 0；初始 red 19 项中 7 项失败，修复后 green |
| 真实 Skopeo list-tags | passed | 固定摘要工具匿名读取成功；Repository=ghcr.io/yangphere/leanote、Tags=[2.0.1,latest] |
| 真实 Skopeo inspect --config | passed | config.Labels 的 version=2.0.1，Linux/amd64，uid=10001:10001 |
| 新文件 CLI + 真实 registry JSON | passed | has-latest=true；候选 1.9.9=false、2.0.1=false、2.0.2=true，仅判断无远端写入 |
| YAML 1.2 解析 | passed | 使用项目已有 Playwright utilsBundle.yaml；concurrency={group:docker-image-latest,queue:max,cancel-in-progress:false} |
| 所有 workflow shell block 语法 | passed | 从解析后的 jobs 提取 12 个 run block，逐个 Git sh -n 通过（仅解析，不执行） |
| actionlint | unrun | 未安装；未以 YAML/语法检查推断 actionlint 通过 |
| 追加后的完整 npm test | passed | 唯一实例 session 79854，exit 0；244 tests、243 passed、0 failed、1 skipped；323100.2458ms |
| 追加后的 Trellis check | passed | 实现/测试/规格/任务元数据四层无代码 finding；复核发现的示例和任务证据旧文案已同步，最终记录见 check-review-ghcr.md |
| 真实 GitHub queue:max 排队 | unrun | 未为验证触发 Actions；依据官方当前文档及静态配置检查 |
| 新 guard 的真实远端晋升/缺 latest 初始化 | unrun | 未改写 2.0.1/latest 或发布新版本；纯逻辑/CLI 回归和真实只读数据验证不能代替远端写入证据 |

可复现只读验证（无 credential 文件挂载）：

```powershell
docker run --rm --network host quay.io/skopeo/stable@sha256:249b92db7297e5c801e19172dbb3b56fde88094a49740a5ededac8c2958bf2c0 list-tags docker://ghcr.io/yangphere/leanote
docker run --rm --network host quay.io/skopeo/stable@sha256:249b92db7297e5c801e19172dbb3b56fde88094a49740a5ededac8c2958bf2c0 inspect --config docker://ghcr.io/yangphere/leanote:latest
node scripts/check-latest-promotion.mjs --has-latest <tags.json> ghcr.io/yangphere/leanote
node scripts/check-latest-promotion.mjs --should-promote <tags.json> <config.json> ghcr.io/yangphere/leanote <candidate-version>
```

ADR/spec/交付文档已同步到最高成功晋升版本语义、100 pending 上限、包缺失拒绝策略。旧 Compose 变更与用户 CONTEXT.md 保留，不修改 `.env`、Dockerfile、其他工作流或归档任务历史。

## 本地收尾授权

2026-10-04 用户要求“提交并归档”，授权按工作提交 → 任务归档 → session journal 顺序完成本地收尾，不推送。前文“不提交、归档”是实施/检查阶段的边界。本轮复核已有四层检查和完整 npm test 证据，代码未新增变动；`git diff --check` 和 implement/check 各 5 个上下文引用校验再次通过。`trellis-update-spec` 核对确认 Compose 消费、版本晋升、队列上限、默认禁建包和历史 tag 工作流边界均已同步，无需新增规格改动。用户 `CONTEXT.md` 的 8 行新增排除在提交之外。

真实 GitHub 排队、新 guard 远端晋升、actionlint、新 dev override build/up 等未运行项继续保留，不以本地提交或归档推断通过。

工作提交：`9322cfeb37fde8c2055d039603ada6d618339285`，24 个任务相关文件，提交后仅 `CONTEXT.md` 保持未提交。归档目标为 `.trellis/tasks/archive/2026-10/10-04-compose-prod-dev-split`；implement/check 中任务内 research 引用同步到归档目标，归档后重新执行 `task.py validate` 核对有效性。
