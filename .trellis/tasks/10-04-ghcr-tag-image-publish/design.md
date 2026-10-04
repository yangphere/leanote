# 设计：docker-image.yml

## 作业结构

```
validate ──► quality-gate (uses ./.github/workflows/quality-gate.yml) ──► publish
```

- **validate**（`ubuntu-22.04`，只读）：checkout `github.sha`、`fetch-depth: 0`；`npm ci`；使用 `scripts/version.mjs` 严格校验无前缀 `X.Y.Z` tag 与 package/lock 一致；拒绝 `github.event.forced == true`；校验 `refs/tags/<tag>^{}` 等于 `GITHUB_SHA`；校验 `git merge-base --is-ancestor $GITHUB_SHA origin/main`。
- **quality-gate**：复用 `quality-gate.yml`（`workflow_call`）。其并发组含 `github.workflow`，与 `release.yml` 的调用互不冲突。
- **publish**（`packages: write`、`contents: read`）：
  1. checkout + `docker/setup-buildx-action`（固定 SHA）。
  2. 以 `release.yml` 相同参数构建：`--platform linux/amd64 --load --provenance=false --sbom=false`，`VERSION=$tag`，`REVISION=$GITHUB_SHA`，`SOURCE_DATE_EPOCH=$(git show -s --format=%ct)`，`OCI_CREATED` 由 epoch 推导，镜像标签 `ghcr.io/yangphere/leanote:$tag`。首发 Git tag 与镜像 tag 都是 `2.0.1`。
  3. `scripts/container-smoke.sh <image>`（带与 quality-gate 相同的 `CONTAINER_SMOKE_PDF_URL`）。
  4. 通过可单测 helper 做认证 registry 查询：现有包要求 `MANIFEST_UNKNOWN` + 精确包 listing；首次创建仅在本工作流显式启用且要求精确 manifest/listing 端点均返回 `NAME_UNKNOWN`，可选 `detail.name` 若存在必须匹配。查询拒绝重定向，其他状态失败。
  5. `docker login ghcr.io`（`GITHUB_TOKEN`），推送前确认该标签在 registry 不存在（存在则失败，保持不可变）。
  6. `docker push`，再用 `docker buildx imagetools inspect --format '{{.Manifest.Digest}}'` 与 Buildx metadata 的 `containerimage.digest` 比对。

## 关键取舍

- 不复用 `release-runner.mjs`：它绑定审批身份、交接产物与 GitHub Release 创建，轻量路径不具备这些输入。
- 不使用 `docker/build-push-action`：沿用现有 `docker build/push` 脚本式写法，减少新增固定依赖。
- 标签不可变靠"推送前存在性检查 + 推送后 digest 比对"，不依赖 GHCR 配置。
- 轻量路径使用 `docker-image-${{ github.ref }}` 自身锁；不与可能长期等待 protected runner 的 `release.yml` 共锁，也不宣称两条发布路径有跨 workflow 原子性。
- `main` 是唯一版本来源；validate 与 publish 都显式刷新 `origin/main` 后检查祖先关系。普通 CI 的 push 分支同步为 `dev` 与 `main`。
- registry 不存在性检查和实际 push 使用同一个无前缀版本标签。protected Release 保持 `v*.*.*`，本路径只响应无前缀版本，避免同时触发；quality-gate 的七个质量作业与汇总完整保留，仅 protected handoff 作业按原规则 skipped。
- `sh/package.sh` 的版本检查必须在真实 tag 上分别验证严格 `X.Y.Z` 与严格 `vX.Y.Z`；branch push 不作 tag 校验。两种 tag 都必须与 package/lock 版本一致，不能仅修轻量 validate 而让质量门禁打包入口拒绝无前缀 tag。
- 修复已由远端 CI 复现的四个门禁阻塞（见 `remote-ci-preflight.md`）：隔离 release fixture workflow 环境、恢复共享接口兼容 golden 并断言不泄漏数据、精确断言 preview 文本 404、把 TinyMCE manifest 运行 URL 映射到 canonical route。保留生产授权逻辑和完整门禁。

## 回滚

删除或禁用 `docker-image.yml` 与 ADR-0005 即可；已发布的镜像不会被删除（需要用户在 GitHub 包页面手工处理）。
