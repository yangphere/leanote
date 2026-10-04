# 设计：docker-image.yml

## 作业结构

```
validate ──► quality-gate (uses ./.github/workflows/quality-gate.yml) ──► publish
```

- **validate**（`ubuntu-22.04`，只读）：checkout 执行器 `github.sha`、`fetch-depth: 0`；写出 identity 前复用 `assertImageTagFormat` 并校验非零 40 位 SHA。另行 checkout 候选，在候选目录用执行器 `check-version.mjs` 严格校验 tag 与候选 package/lock 一致；拒绝 `github.event.forced == true`；校验远端 peeled tag 等于候选 SHA，且候选是刷新后的 `origin/main` 祖先。普通 push 的候选为 `github.sha`；恢复 dispatch 的候选为显式 `expected_commit`。
- **quality-gate**：复用 `quality-gate.yml`（`workflow_call`）。其并发组含 `github.workflow`，与 `release.yml` 的调用互不冲突。
- **publish**（`packages: write`、`contents: read`）：
  1. checkout + `docker/setup-buildx-action`（固定 SHA）。
  2. checkout 候选与执行器到两个目录。以 `release.yml` 相同参数在候选目录一次构建：`--platform linux/amd64 --load --output type=oci,dest=<candidate.oci.tar> --provenance=false --sbom=false`，`VERSION=$tag`，`REVISION=$CANDIDATE_SHA`，`SOURCE_DATE_EPOCH=$(git show -s --format=%ct "$CANDIDATE_SHA")`，`OCI_CREATED` 由 epoch 推导。loaded 候选和 OCI archive 来自同一次 Buildx invocation；archive 的 raw manifest SHA/config 与 metadata 一致，config 还必须匹配 loaded image Id。首发 Git tag 与镜像 tag 都是 `2.0.1`。
  3. `scripts/container-smoke.sh <image>`（带与 quality-gate 相同的 `CONTAINER_SMOKE_PDF_URL`）。
  4. 使用执行器的可单测 helper 做认证 registry 查询：现有包要求 `MANIFEST_UNKNOWN` + 精确包 listing 200；首次创建仅在显式启用时接受精确端点的双 `NAME_UNKNOWN` 或 manifest `MANIFEST_UNKNOWN` + listing `NAME_UNKNOWN`，均为结构化 404，可选 `detail.name` 若存在必须匹配。查询拒绝重定向，其他状态失败。
  5. `docker login ghcr.io`（`GITHUB_TOKEN`），Skopeo 复用该认证；推送前确认该版本标签在 registry 不存在（存在则失败，保持不可变）。
  6. 用 `skopeo copy --preserve-digests oci-archive:<candidate.oci.tar> docker://<version>` 发布同一个 artifact，再用执行器 `verify-image-manifest.mjs` 对 raw registry manifest 做原始字节 SHA 与 config 比对。
  7. 版本确认后，复制相同 archive 到 `latest` 并验证相同 manifest/config digest。latest 是用户后续明确要求的可更新别名；版本保持不可覆盖。

## 关键取舍

- 不复用 `release-runner.mjs`：它绑定审批身份、交接产物与 GitHub Release 创建，轻量路径不具备这些输入。
- 不使用 `docker/build-push-action`：沿用现有 `docker build/push` 脚本式写法，减少新增固定依赖。
- 标签不可变靠"推送前存在性检查 + 推送后 digest 比对"，不依赖 GHCR 配置。
- 轻量路径的所有版本 push 与 dispatch 使用 `docker-image-latest` 锁，串行化共享 latest 更新；不与可能长期等待 protected runner 的 `release.yml` 共锁，也不宣称两条发布路径有跨 workflow 原子性。
- `main` 是唯一版本来源；validate 与 publish 都显式刷新 `origin/main` 后检查祖先关系。普通 CI 的 push 分支同步为 `dev` 与 `main`。
- registry 不存在性检查和实际 push 使用同一个无前缀版本标签。protected Release 保持 `v*.*.*`，本路径只响应无前缀版本，避免同时触发；quality-gate 的七个质量作业与汇总完整保留，仅 protected handoff 作业按原规则 skipped。
- `sh/package.sh` 的版本检查必须在真实 tag 上分别验证严格 `X.Y.Z` 与严格 `vX.Y.Z`；branch push 不作 tag 校验。两种 tag 都必须与 package/lock 版本一致，不能仅修轻量 validate 而让质量门禁打包入口拒绝无前缀 tag。
- 修复已由远端 CI 复现的四个门禁阻塞（见 `remote-ci-preflight.md`）：隔离 release fixture workflow 环境、恢复共享接口兼容 golden 并断言不泄漏数据、精确断言 preview 文本 404、把 TinyMCE manifest 运行 URL 映射到 canonical route。保留生产授权逻辑和完整门禁。
- 实际 main CI 又证明正确前缀下的 `index.html` 仍会被 Go 静态服务重定向；因此此类 output 的运行 URL 必须是带尾斜杠的目录。保留 `index.html` 磁盘输出和 direct-200 浏览器门禁，本地真实请求已核实两个目录 URL 直接 200。

## 回滚

删除或禁用 `docker-image.yml` 与 ADR-0005 即可；已发布的镜像不会被删除（需要用户在 GitHub 包页面手工处理）。

## 已存在 tag 的恢复

- 真实 source run `37173559882` attempt `1` 在 registry preflight 失败，未执行 push；候选 `dea2306c32f27e648d9438cbc8b67cec6151b6cc` 已通过七个质量作业、summary、镜像构建和候选 smoke。原整体 conclusion 为 failure。
- `workflow_dispatch` 仅从 `refs/heads/main` 执行，输入 `tag`、`expected_commit`、`source_run_id`、`source_run_attempt`。`2.0.1` tag 始终保留原指向。
- 来源验证通过 GitHub API 绑定仓库、Docker image 工作流路径/名称、push event、数字 tag、SHA、固定 attempt。要求该 attempt 仍是 artifact 所属最新 attempt；来源必需作业成功，publish 构建/smoke 成功、push 步骤失败。
- 下载来源 run 的 `ci-summary-*`，复用 `scripts/ci/validate-summaries.mjs` 的 schema/质量规则，显式校验来源 execution identity（含 summary）。身份冲突、缺失、重复、失败或 attempt 漂移一律阻断。
- main 执行器仍完整调用 quality-gate；候选的质量证据和执行器的质量证据分别验证。实际 Docker 构建、metadata 与 smoke 使用候选 checkout，修复后的 registry helper 从执行器目录运行。推送前再核对远端 tag 和 main 祖先。

## Manifest 传输与 latest 晋升

- 实际恢复 run 已推送 2.0.1，但旧比较误将 BuildKit 导出 manifest 与 Docker Engine 重序列化的 manifest 当成相同字节。真实本地 registry 复现了相同 config、不同 manifest SHA；强制 `oci-mediatypes=false` 也不能消除 Engine 序列化差异。单次 dual export + Skopeo preserve-digests 的原始 artifact 校验已真实通过。
- dispatch `operation` 为 `recover_version` 或 `update_latest`；push 使用内部 `publish`。前者禁止已有版本覆盖；后者必填 `expected_registry_digest` 和 `expected_config_digest`，不构建、不调用版本不存在性检查、不写版本。
- `update_latest` 先核对远端 Git tag/main 祖先和原候选质量证据，验证版本 raw manifest hash/config 与显式 expected digests；拉取 `version@digest`，校验 Id、Linux/amd64、version/revision/source 标签。对这个拉取镜像执行完整 candidate smoke（保留必需 PDF URL），然后只从该 registry digest 复制到 latest，最后验证 latest 相同 SHA/config。
- 本次 latest 输入绑定 version `2.0.1`、registry `sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`、config `sha256:99b11b4586b2ea0549b2264bd80bc736fd57216ea2b0a7a4e7cca5661b9ee6a8`。这些值已通过真实匿名 registry 字节 hash、header、pull 和镜像 metadata 交叉验证。
- Skopeo 使用实测 1.22.3 的固定容器 digest `quay.io/skopeo/stable@sha256:249b92db7297e5c801e19172dbb3b56fde88094a49740a5ededac8c2958bf2c0`，显式 authfile 来自只读挂载的 Docker GHCR 登录配置。Ubuntu 22.04 发行版 Skopeo 1.4.1 实测不支持 preserve-digests，不能用安装旧工具或移除参数的 fallback。
- 多标签更新不是原子事务。版本成功而 latest 失败时保留版本，后续只经显式晋升重试 latest；不能重推版本来使旧运行变绿。
