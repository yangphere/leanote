# 交付层：集成验证与版本发布 — 技术设计

## 1. 责任与复用

交付层只组合现有服务合同、harness、CI、包、browser 和 release validators。九轨入口与 AC 映射集中在 `acceptance/evidence-matrix.md`，notes 38 项清单集中在其上游 inventory；本任务 `notes-action-replay.md` 只是执行索引，不是第二业务合同。

服务/适配缺陷记 owning task+失败场景+影响 AC，阻断相关门禁并回流 owner；环境/fixture/交付脚本缺陷由 delivery 修复。不能为让测试通过而重写 Golden、放宽权限、猜 USN 或在 harness 构造成功 receipt。归档 owner 的历史矩阵不被本任务倒填通过；新证据在本任务追加并反向引用。

## 2. 候选与执行流

1. 冻结干净 candidate SHA、版本、配置/fixture、toolchain 和原始上游合同；记录 SHA 与工作树状态，未提交测试标 working-tree，不能冒充候选实证。
2. 先做确定性质量门，再在隔离资源上执行 38-action、Mongo 三种组合、SMTP、kill/restart/filesystem/failpoint、production/package/container/PDF。默认 CI 并未覆盖全部扩展项，后续交付实现须把缺口加入门禁，不仅添加文档复选框。
3. 受保护 runner 执行八槽四 coverage，并执行 presentation/publishing 等额外业务手工清单；缺浏览器只能阻断，不能用另一个引擎代替。
4. 普通 PR/push、独立 tag-precheck、最终 release 是不同证据阶段。最后一个阶段才允许远端写入，并须通过所有必需门禁和用户授权。

### Provenance 关联

- 既有 `.github/workflows/release.yml` 通过 `workflow_call` 调用 quality-gate 和 browser-evidence；它们属于同一个 release run/attempt。final validator 严格拒绝跨 run/attempt 输入，保留此约束。
- `workflow_dispatch` browser tag-precheck 按 tag peel 后 SHA 校验，仅产生候选验收证据；`validate-browser-artifact.mjs --phase precheck --expected-commit <SHA>` 与 final 不可混用。发布必须重新生成该 run/attempt 的两文件 artifact。
- 上游、本地或其他 run 的历史结果可用于缺口定位，不能改写其 provenance 塞进 final。扩展验证可复用上游测试入口，但最终发布需由同一 release 执行链运行并绑定其结果。后续实现需要保证这些扩展项实际成为 publish 的前置；当前代码尚无完整门禁。
- 普通发布重跑整个 workflow attempt 时重新生成被 final 消费的制品。不能只重跑 publish 并假定前一 attempt 的浏览器/包自动有效。源码变更使旧候选失效；仅规划变更也要在最终发布前冻结实际 tag 指向的完整 SHA。Q-DV1 恢复采用下述原执行关联，不把旧制品标记成本次 final 生成。

## 3. Artifact 与数据约束

| 输出 | 现有 owner / 约束 |
| --- | --- |
| quality summaries | `scripts/ci/validate-summaries.mjs`；stage/commit/ref/workflow/run/attempt 一致，discovery/execution 分开，退出/清理失败显式保留 |
| browser | `browser-release-matrix-v1` 仅 `release-matrix.json`、`provenance.json`；8 records、4 固定 coverage、JCS coverage digest 与 matrix 字节 SHA-256 |
| release inputs | `leanote-release-inputs-v1`；`scripts/release-metadata.mjs` + `scripts/validate-release-artifact.mjs`，tar/checksum/build-metadata/image-build-inputs 全部 hash、strict version/ref/SHA/epoch/linux-amd64/image digest |
| 业务联验 | 本任务 AC/scenario 执行记录引用实际 runner 与脱敏 artifact；schema 扩展只能在既有边界设计，禁止悄悄绕过 allowlist |
| 发布结果 | 远端 tag peel、镜像 digest、Release tag/assets 与原 release-inputs 对账；失败/未知不等于远端为空 |

summary 格式正确不能证明真实浏览器执行或日志安全。受保护命令配置须绑定受审脚本版本、实际浏览器 binary/version 与用例入口，禁止 echo 合成通过 marker。marker schema 保持现契约；需要的新 provenance 先扩展同一 schema/validator，不加绕过校验的旁路文件。

仅使用合成测试账号和数据；秘钥由受保护环境注入，不记录值。公开 artifact 无正文、cookie/token、完整授权 URL、trace/截图/视频/原始服务日志；验证故障诊断输出同样脱敏。仅靠字段 allowlist 不声称已完成泄漏验证。

## 4. 运行隔离、异常与兼容

- native HTTP 使用 `cmd/leanote -runMode test`，数据库强制 `leanote_test`；service focused fixture 若有专用测试库，先核对 allowlist/隔离证明。failpoint、restore、kill/restart 仅作用于明确所属的测试资源，不探测/修改用户库。
- 固定 `leanote-test-mongo`/端口的 harness 必须串行；不能与共享资源测试并发。`npm run build` 与会复制工作树的 Node suite 串行，避免 publication/fixture 竞争。
- 单元测试每批默认 `-timeout 60s`；真实 integration 按仓库独立门禁的显式超时运行。服务启动不等于通过，要有请求/DB/文件断言与清理结果。
- roots/权限由 production-config 与 contentfs 唯一 seam 处理；旧卷迁移只复制到新测试卷，验证后仍保留旧卷，不增加 runtime fallback。backup/restore 用合成库，遵循 admin 的 argv/credential channel/预算。
- PDF 验收分别检查 self-contained 正向可读文件与恶意 HTML；观测 renderer outbound 与 synthetic file sentinel，不能把 app 取授权资源所需的访问误记为 renderer 允许联网。退出/timeout/cancel/cleanup 各有断言。
- Golden 与上游明确的兼容例外是目标；不引入新的 API/Schema 变化。无 CSRF/同源主题等已接受风险继续显式移交。

## 5. 发布状态与恢复

当前实现顺序为 `validate -> quality -> browser -> duplicate preflight -> build/digest compare -> GHCR push/read-back -> Release create`。下表是必须可观察的情况，不表示本轮实现了新状态机。

| 场景 | 必须行为 |
| --- | --- |
| 校验/质量/浏览器/扩展业务证据失败 | publish 不运行；保留原失败及清理结果 |
| 远端 tag/Release/image 查询网络或权限错误 | 标 unknown/blocked，不当成 not-found，不开始覆盖写入 |
| 任一已存在且身份不一致 | conflict，停止；禁止覆盖/delete/移动 tag |
| GHCR push 超时或结果未知 | 只读核对精确 tag/digest；不声称推送未发生 |
| 镜像匹配、Release 明确不存在、原门禁与制品完整 | Q-DV1 已确认允许仅用原制品补建 Release；不 build/push 镜像或修改 tag |
| 镜像匹配但 Release 查询未知 | 阻断写入，继续只读对账，未知不等于不存在 |
| 两端都存在 | 恢复入口核对 SHA/digest/assets，完整匹配则记录 already-complete/no-op；不完整或冲突则阻断。重复普通发布仍拒绝 |
| Release 创建成功但响应丢失 | 只读查资产完整性，不盲重建，不自动删除重复对象 |

发布不是跨 GitHub/GHCR 的原子事务，不能承诺自动回滚为“从未发布”。Q-DV1 已由用户确认，受控恢复按以下步骤设计；这是待实现的目标合同，不是现有 workflow 能力声明。

### Q-DV1 恢复执行合同

1. 显式进入 recovery 模式，指定仓库、strict tag、完整 candidate SHA、原 release run/attempt。普通发布入口不因发现已有镜像而自动转 recovery。具体远端操作沿用已有明确发布授权；本轮规则确认只更新规格。
2. 从可信的原 workflow 执行记录核验 repo/ref/commit/run/attempt，以及发布前 validate、quality、browser、扩展业务门禁均通过。原整体 run 可因 publish 失败而失败，不能要求它整体 success，也不能把 publish 之前的失败当作可恢复。
3. 读取该原 run/attempt 的原始 release inputs、browser 两文件及必要联验证据，验证现有 schema、文件 allowlist、全部 hash、版本、epoch、平台和 image digest。原 artifacts 缺失/过期、来源不能认证或被撤销则阻断，不重建、不改 provenance、不用其他 run 的相同文件名替换。
4. 分别记录 `source_run/source_attempt/source_artifact_ids` 与 `recovery_run/recovery_attempt`、候选身份、原摘要和核验结果；复用唯一 validator 的校验逻辑，增加明确 recovery 关联语义，不能靠覆盖 GITHUB_RUN_ID/ATTEMPT 冒充原执行。普通 final 的同 run/attempt guard 不变。
5. 恢复入口必须实现与普通发布共用的仓库/tag 并发锁。现有 `release.yml:7-9` 仅为普通发布配置 `release-${{ github.ref }}`；tag push 时实际 group 为 `release-refs/tags/<tag>`。recovery 必须从显式目标 strict tag 派生完全相同的 `release-refs/tags/<tag>`，设置 `cancel-in-progress: false`，不能使用 dispatch 分支的 `github.ref`，也不能加入 workflow 名或 run/attempt。该互斥是待实现要求，不能因普通发布已有锁就视为 recovery 已具备。锁须覆盖远端重查、create 和最终读回；在锁内只读重查远端 tag peel、GHCR digest/元数据和 Release，只有身份全部一致且 Release 确认不存在才允许 create；网络/权限错误保持 unknown。镜像缺失不能借 recovery 重推。
6. 创建前再次核对 tag/digest/Release 状态，仅从已验证的原 tarball、checksum、build-metadata 创建 Release。不得以当前 checkout 重建制品、覆盖已有资产、删除对象或移动 tag；create 不使用 clobber/update。若竞争者已创建，重新读回后按完整一致 no-op 或 conflict 分类，不覆盖。
7. 创建成功、超时或响应丢失后均读回远端 tag、镜像 digest、Release 标识/资产清单及实际资产字节 hash。匹配则记录 created-complete 或 already-complete；资产缺失/冲突或查询未知则保持 partial/unknown，保留恢复记录，禁止盲目再建或修改已有 Release。

恢复只补缺失 Release，不扩展为修复已有但不完整的 Release。原证据过期不能靠重跑普通流程覆盖既有 tag/image；须保持阻断并记录缺失材料。恢复正负例及无写入断言见 `acceptance/release-recovery.md`。

## 6. 本轮限制

以上为审核后的目标设计；现有 workflow 的实现差距见 `research/spec-audit-2026-09-29.md`。本轮不改任何 runtime/CI/script，不运行真实环境验证。
