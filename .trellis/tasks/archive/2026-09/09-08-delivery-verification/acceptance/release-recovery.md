# Q-DV1 — 受控补建 Release 验收

决策：2026-09-29 用户确认“允许”。唯一目标合同为 PRD R9 和 design §5；本表仅定义 AC-DV9/10、E-D13 的可观察验收，不声明已实现。全部场景当前 `unrun`。

输入包括仓库、严格 tag、完整 candidate SHA、source run/attempt、可信原制品定位与摘要、独立 recovery run/attempt。原发布前置门禁必须全通过；整体 run 可因 publish 失败而失败。恢复不得改写原 provenance，也不得重建 tar/image 或移动 tag。只允许创建明确不存在的 Release；existing 完整匹配为只读 no-op，不完整 existing 保持阻断。

| ID | 场景 | 预期结果及写入断言 | 状态 |
| --- | --- | --- | --- |
| RR-01 | 原门禁通过、原制品完整、tag/SHA/image digest 全匹配、Release 明确不存在 | 仅 create Release 并上传原允许资产；远端读回 hash/身份一致后 created-complete；build/push/delete/update-tag/覆盖次数均为 0 | unrun |
| RR-02 | 原 run 因 publish 失败而整体 failed，但全部发布前置已通过 | 不因整体 failed 错拒；逐前置核验后按 RR-01。任一前置 fail/skip/unknown 则零写入 | unrun |
| RR-03 | 原 artifacts 缺失/过期/不可获取或来源无法认证 | blocked，零写入、不重建、不从另一 run 或同名本地文件补齐 | unrun |
| RR-04 | repo/ref/SHA/tag/version/platform/epoch/digest/hash/allowlist 任一不匹配或混用 source attempt | conflict，零写入；普通 final 跨 run/attempt 仍拒绝 | unrun |
| RR-05 | tag/image/Release 查询网络或权限错误、push 超时结果未知 | unknown，零写入；不得当作 404/不存在；只读重查获得确定结果后重新核验 | unrun |
| RR-06 | 镜像缺失、digest 不符或 tag 已被移动 | blocked/conflict，零写入；不重推镜像、不恢复/移动 tag | unrun |
| RR-07 | GHCR 与 Release 已完整一致，包括原资产实际 hash | already-complete/no-op，零写入；重复 recovery 仍如此，不冒充本次新建 | unrun |
| RR-08 | Release 已存在但缺资产、hash/候选不符或不是目标正式发布形态 | partial/conflict，零写入；不补传、不覆盖、不删后重建 | unrun |
| RR-09 | 同 tag 两个 recovery 或 ordinary publish 与 recovery 并发，含独立 recovery workflow 和从不同分支 dispatch | 断言普通发布的 `release-${{ github.ref }}` 与 recovery 从目标 strict tag 派生的 group 均为同仓库 `release-refs/tags/<tag>`，且 `cancel-in-progress: false`；不得以 dispatch 分支 ref/workflow/run 区分锁。锁覆盖重查/create/最终读回，创建前重查；竞争者已完成则 RR-07，不一致则 RR-08；最多一个创建者，无 clobber | unrun |
| RR-10 | 创建成功但响应丢失，或 create 在部分资产写入后失败 | 只读查 Release/资产；完整为 already-complete，缺失为 partial，查询未知为 unknown；不盲目再建/补传 | unrun |
| RR-11 | 普通重复发布遇到既有镜像 | 原 duplicate guard 继续拒绝；不会自动转换 recovery，也不会使用 recovery 规则绕过 final | unrun |
| RR-12 | 跨 run 恢复成功，或后续审计回查 | 原 source run/attempt/artifact IDs 与本次 recovery run/attempt 分别可追溯，原 provenance/字节不变；最终 tag/image/assets 再核对，日志不含凭据或授权 URL | unrun |

每个记录沿用 `evidence-matrix.md` 字段，再添加 source/recovery 关联、远端读取结果、create/no-op/blocked 分类和写操作计数。seam/本地 contract 证明边界控制，不替代真实 GitHub/GHCR 实证；真实执行必须在具体发布授权范围内。原制品不可验证或已有 Release 不完整时记录人工处理条件，不能用本决策自动扩大修复权限。
