# 浏览器发布证据来源图

本任务的浏览器证据契约由以下仓库入口共同定义，执行者必须直接读取并运行对应入口，不能只依据 PRD 摘要：

- `scripts/browser-release-evidence.mjs`：四浏览器产品、current/previous 两个槽位、四个 coverage ID、八条 matrix record、provenance 和 JCS digest 校验。
- `scripts/validate-browser-artifact.mjs`：发布 artifact 的 schema、commit/run/attempt 绑定和跨文件校验入口。
- `tests/js/release-contract.test.js`：matrix、coverage summary、provenance 和失败路径的 Node contract 回归。
- `.github/workflows/browser-release-evidence.yml`：受保护真实浏览器 workflow、artifact 生成和失败阻断条件。
- `.trellis/spec/frontend/type-safety.md` 与 `.trellis/spec/frontend/directory-structure.md`：validator/test 约定和四个稳定 coverage ID 的项目规范。

如果上述脚本、测试或 workflow 与任务 PRD 的描述不一致，以脚本的实际校验和同一候选 commit 的 contract test 为准；缺失、未运行或 artifact 不完整都保持 blocked。
