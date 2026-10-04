# Trellis Check Review — GHCR 追加修复

日期：2026-10-04。

## Findings (fixed)

- File: `.trellis/spec/backend/image-publishing.md`
  - Issue: 旧示例仍无条件要求把已存在的 `2.0.1` 复制到 latest，与新增的旧版/同版跳过契约不一致。
  - Fix: 示例改为先执行版本 guard，仅在候选推进或初始化 latest 时复制。
- File: `.trellis/tasks/10-04-compose-prod-dev-split/validation.md`
  - Issue: Compose 阶段的旧证据仍称发布工作流未修改，追加 GHCR 授权后已不再准确。
  - Fix: 区分初始 Compose 阶段与追加修复，明确仅修改 `docker-image.yml` 且未触发远端运行。

未发现需要修改的 GHCR 实现或测试缺陷。

## Findings (not fixed)

- File: `.github/workflows/docker-image.yml` 的历史版本
  - Issue: GitHub tag push 使用事件 tag 所指提交中的 workflow；本次工作区修改无法追溯加固旧提交。向未含修复的历史提交补推版本 tag，仍可能执行旧的无 guard/默认队列/首包许可逻辑。
  - Reason: 这是历史工作流与远端 tag 管理边界，无法由当前 workflow 自身修复。ADR、交付文档、spec、PRD/design/validation 和研究记录已明确禁止此类补推；新 tag 必须包含加固，recovery 必须从已修复 main 执行。强制阻断需要另行评审远端 tag 规则或可信固定发布入口。本项不阻塞当前本地实现，但不能宣称 guard 对全部历史 workflow 生效。

## 四层复核

- 实现：共享 `docker-image-latest` 锁使用 `queue: max` 与 `cancel-in-progress: false`；普通发布移除 `ALLOW_INITIAL_PACKAGE_CREATE`。两条 latest 路径都在候选/不可变版本校验、smoke 和来源复查后，通过带认证的 Skopeo `list-tags` 及必要的 `inspect --config` 读取当前状态，并在 copy 前调用同一 guard。读取、JSON、Repository、Tags、config、Labels 或版本异常均失败关闭。
- 版本规则：`compareImageVersions` 复用 `version.mjs` 的严格 `X.Y.Z` 格式，逐段使用 `BigInt`；覆盖多位数、超大分量、相等和旧版。候选仅在严格大于当前 latest 时晋升；成功且身份绑定的 listing 确认无 latest 时允许初始化。
- 回归保护：不可变版本、单次构建、OCI manifest/config 摘要、candidate/source、smoke、最小权限和 pinned Skopeo 规则未退化。旧版 fresh publish 仍可完成缺失不可变版本写入和 read-back，然后成功跳过 latest；`update_latest` 同样先验证/拉取/smoke 固定 digest，再执行 guard。
- 规格与元数据：ADR-0005、交付文档、backend spec、PRD/design/research/validation/implement/task metadata 已同步最高成功晋升版本、100 pending 上限、默认禁建包、历史 workflow 边界和证据限制。原 Compose 拆分、生产运行证据及用户 `CONTEXT.md` 改动保持不变。

## Verification

- Lint: N/A（仓库无独立 lint 脚本；`git diff --check` 通过）
- TypeCheck: N/A（无类型检查脚本；变更的三个 JavaScript 文件 `node --check` 通过）
- Tests: pass（初始 red：GHCR focused 19 项中 7 项失败；修复后 GHCR 19/19、组合 focused 50/50；完整单实例 `npm test` exit 0，244 tests、243 passed、0 failed、1 skipped）
- Workflow syntax: pass（真实 YAML 1.2 解析通过；解析出的 12 个 `run` shell block 均通过 Git `sh -n`）
- Registry read-only: pass（真实匿名 Skopeo listing/config 与 helper CLI：`1.9.9=false`、`2.0.1=false`、`2.0.2=true`；未写 registry）
- Task context: pass（implement/check 各 5 个引用；最终 `git diff --check` 通过）
- Actionlint: unrun（未安装，未用其它检查替代其证据）
- Runtime boundaries: unrun（真实 GitHub `queue: max` 排队、新版远端晋升/缺 latest 初始化、历史 workflow 强制阻断均未执行或配置）

结论：当前代码、测试、规格与任务元数据无阻塞缺陷，可以完成追加修复的实现与检查阶段；历史 workflow 和远端运行边界须按上述限制保留为未验证、未强制。
