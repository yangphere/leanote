# GitHub / GHCR / actionlint 真实补充验证

2026-10-04 用户要求真实执行原 `unrun` 检查，并明确“不新建，作为已归档任务的补充验证”。本记录补充实现阶段证据，不重新激活或归档任务。用户随后要求“提交并归档”，本次收尾仅提交补充证据并记录日志，沿用原归档目录，不推送远端。

## 范围与来源

- 生产 `main` 当前 SHA：`d525be93d7a393a40d3328ca737d50d977994092`。其工作流仍无新 guard/queue，且保留旧首发许可。本轮不合并或推送本地工作提交到 main。
- 实际验证分支：[`codex/ghcr-live-verification-20261004-84f31d`](https://github.com/yangphere/leanote/tree/codex/ghcr-live-verification-20261004-84f31d)。从当前 main 创建，只添加隔离验证工作流/脚本及已审查的共享版本/晋升 helper；后两者来自工作提交 `9322cfeb37fde8c2055d039603ada6d618339285`，运行侧 blob SHA 与该提交逐一相同。
- helper blob：`scripts/version.mjs=59786447308cff881e237b0903f229ca4d1bc854`；`scripts/check-latest-promotion.mjs=74a5984e4e961d0620003ced1ecc5b5515ae859d`。
- 真正写入的独立 GHCR 包：`ghcr.io/yangphere/leanote-ghcr-live-verification-20261004-84f31d`。镜像是有真实 OCI 版本标签的 scratch fixture，标签 `2.0.1/2.0.2/1.9.9` 属于测试包，不是 Leanote 新版本发布。
- 当前工作区产品代码、main、原有版本 tag 和生产镜像均未修改；实际远端写入只包括上述验证分支和测试包。验证资源保留以便复核。

## 真实 GitHub 排队：passed

使用与发布工作流相同的 `queue: max`、`cancel-in-progress: false`，并为实验单独设置一个固定共享 concurrency group，避免与真实发布共锁。连续触发三个 push；实际 API 快照证明 run 1 为 `in_progress` 时 run 2 和 run 3 同时为 `pending`。最终三个运行都 `success`，无任何取消。

下表使用 job 实际开始时间，而非 queued 时已设置的 `run_started_at`；相邻运行没有执行重叠。时间均为 UTC。

| 运行 | 实际开始 | 最后 job 完成 | 结果 |
|---|---|---|---|
| [37184573081](https://github.com/yangphere/leanote/actions/runs/37184573081) | 07:02:04 | 07:04:38 | success，含真实 registry 验证 |
| [37184614263](https://github.com/yangphere/leanote/actions/runs/37184614263) | 07:04:42 | 07:04:50 | success |
| [37184617541](https://github.com/yangphere/leanote/actions/runs/37184617541) | 07:04:53 | 07:05:01 | success |

证据：`runtime/queue-snapshot-pending.json`、`runtime/queue-snapshot-after-promotion.json`、`runtime/jobs-*.json`。这证明该仓库实际支持多个 pending 的保留和顺序执行，不证明 100 个 pending 的满队列溢出行为。

## 真实 GHCR 晋升：passed（隔离包与原 helper）

GitHub hosted runner 使用 `GITHUB_TOKEN` 的 job 级 `packages: write` 权限和原固定摘要 Skopeo。先证明原 helper 的默认策略拒绝创建缺失的测试包，再为首次 fixture 建包显式开启独立 bootstrap；普通发布工作流的许可未重新启用。三个 fixture 通过 Buildx 单平台 OCI 导出，并使用 `skopeo copy --preserve-digests` 真实发布。每个版本 raw manifest/config descriptor 与 Buildx metadata 匹配。

晋升判断消费真实 Skopeo `list-tags` 与 `inspect --config`，Repository、版本标签和平台均复核；没有 stub 或伪造 registry 响应。

| 场景 | 候选 | 实际判断 | 实际 latest |
|---|---|---|---|
| 成功 listing 确认无 latest | 2.0.1 | true，真实复制并读回 | 2.0.1 |
| 更高版本 | 2.0.2 | true，真实复制并读回 | 2.0.2 |
| 同版 | 2.0.2 | false，无 latest 写入 | 2.0.2，raw bytes 不变 |
| 已发布的较旧版本 | 1.9.9 | false，无 latest 写入 | 2.0.2，raw bytes 不变 |

初始化 latest manifest：`sha256:1ec14b40e35310bdd1bac8c95fb33f988ef4494dc395262f2dd86dd84b57b34c`。

晋升后 latest manifest：`sha256:37407ce9cf50a0c4e2e457ada29f8ddabda183e696d63b66a33f091e677e4f62`；config descriptor：`sha256:8b9c46c7439897caf41a2060a245b3b82c425f80d7432533e50fc99b4a90d7a8`。

生产 `ghcr.io/yangphere/leanote:2.0.1` 与 `:latest` 前后 raw bytes 分别完全相同，均仍为 `sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`。

源 artifact：run 37184573081 的 `ghcr-live-verification`，artifact id `11296073048`，GitHub digest `sha256:6a6b4a55f0b68b4c4da905ea3964bece6e211d4ad7f0dd119507c782f395a683`。原始 listing/config/raw manifest/Buildx metadata/decision 已保存到 `runtime/registry-evidence/`。

## actionlint：真实执行，发行版兼容性失败，上游 PR 版本通过

官方最新发行版 `v1.7.12` 的 Windows amd64 zip 通过 GitHub Release asset digest 校验：`sha256:6e7241b51e6817ea6a047693d8e6fed13b31819c9a0dd6c5a726e1592d22f6e9`。对未修改的 `.github/workflows/docker-image.yml` 执行结果为 exit 1，唯一诊断：

```text
.github/workflows/docker-image.yml:41:3: unexpected key "queue" for "concurrency" section. expected one of "cancel-in-progress", "group" [syntax-check]
```

官方 GitHub workflow syntax 已描述 `queue: max`，真实仓库执行也接受该键；actionlint 上游存在相同的 [issue #657](https://github.com/rhysd/actionlint/issues/657) 和未合并的 [PR #654](https://github.com/rhysd/actionlint/pull/654)。这属于工具 schema 未跟进，不能将官方发行版记录为通过，也不删除有效的 queue 配置来消除错误。

逐文件审查 PR 的 8 个文件后，从其固定提交 `644076a59742c2d1540ebd4686eab3c308f0e562` 构建 actionlint；`go version -m` 确认 vcs.revision 与该 SHA 相同且 vcs.modified=false。该二进制 SHA256：`7c386c3f03b6efa1a2093470fb8ef6b46120cc41feffeeace225c77e282ce810`。它对原生产工作流及本次验证工作流均 exit 0，无 ignore、删键或本地 parser 补丁。PR 工具不代表官方发行版已支持该键。可重现构建与调用：

```powershell
git clone --depth 1 --branch concurrency-queue-max https://github.com/vvoland/actionlint.git <tool-source>
git -C <tool-source> rev-parse HEAD # 必须为上述固定 SHA，否则停止
$env:GOTOOLCHAIN='local'
go build -C <tool-source> -o <actionlint-pr654.exe> ./cmd/actionlint
& <actionlint-pr654.exe> .github/workflows/docker-image.yml <validation-workflow.yml>
```

本机未安装独立 ShellCheck/Pyflakes，未声称其通过；验证脚本 `bash -n` 通过，并已在真实 hosted runner 执行成功。

## 本地复核与保留边界

下载真实 artifact 后，以项目原 `verifyImageManifest` 再次核对版本 manifest/config descriptor、所有晋升决策、最新版本标签、skip 前后字节相等、生产镜像不变及三个运行串行执行。复核脚本 exit 0，汇总见 `runtime/verified-summary.json`。`runtime/.gitattributes` 对 `registry-evidence/**` 禁用换行转换，保证 Windows checkout 后原始字节和摘要仍可复核。可在仓库根目录执行：

```powershell
node .trellis/tasks/archive/2026-10/10-04-compose-prod-dev-split/research/runtime/verify-evidence.mjs
```

本次证明真实 GitHub 排队能力和共享 guard 的真实 GHCR 读写行为。未执行加固后的完整生产 `docker-image.yml`、新 Leanote 版本发布、其完整 quality/smoke/source-run 链、满队列溢出或历史 tag 强制阻断；这些不能由隔离 fixture 推断为通过。main 仍需要另行合入修复后才获得加固工作流。
