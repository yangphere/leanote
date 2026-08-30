# jQuery 3.7 升级（E-jQ）— 执行计划

## Preconditions

- 仅在本 PRD、设计和本计划获得后续明确批准，并由 `task.py start` 激活后开始实施。
- 使用 Node 24、干净 `npm ci`、仓库 test-mode E2E harness 提供的已恢复 `leanote_test` MongoDB 8.0 fixture、每次运行随机 token 和随机化 fixture admin 凭据；不满足前置条件即失败，不创建替代服务或凭据。其他 MongoDB 版本仅可作为额外兼容性验证，不得替代 8.0 基线。该 harness 供应路径必须在 fork PR 可用，不能读取 E2E GitHub Secrets。
- 先检查 `git status --short`；本任务只拥有 jQuery、相关构建/test/docs 和对应生成物，不吸收 Bootstrap、TinyMCE 或无关工作区改动。

## 1. Establish The Inventory And Red Tests

**Create**
- `docs/modernization/jquery-3.7-compatibility-inventory.md`
- `tests/js/jquery-asset-contract.test.js`
- `tests/e2e/business/business-flows.spec.mjs`
- `tests/e2e/business/jquery-diagnostics.spec.mjs`（路由诊断逻辑与业务流分离）
- `tests/e2e/e2e-environment.mjs`（build/business 共用 identity/权限/cleanup fixture）
- `tests/e2e/build/build-resource-smoke.spec.mjs`
- `tests/e2e/build/sanitized-summary-reporter.mjs`
- `docs/modernization/browser-smoke/jquery-3.7.md`

**Read before changes**
- `scripts/build/manifest.mjs`, `scripts/build/{index,js,note-html}.mjs`, `tests/js/build-pipeline.test.js`
- 所有 `/js/jquery-1.9.0.min.js` 引用、`BlogController` 的 `jQueryUrl`、`leaui_image/index.html`
- `public/js/common.js` AJAX wrapper、第一方 app/admin/member/blog/album 源、现有插件可读源和 D 的 Playwright smoke
- test-mode server/harness、`conf/app.conf [test]`、`.github/workflows/regression-baseline.yml` 和现有脱敏 reporter

- [ ] 记录每个资产、页面/iframe、插件、旧 API、所有者、风险和期望回归；将 E-TM 排除项明确写入清单。
- [ ] 先写静态测试：声明 jquery-runtime、34 输出、唯一 3.7.1、无生产 Migrate/私有 1.9.1、旧公开 URL 仍受 manifest 管理；在当前树确认失败。
- [ ] 先扩展 harness：恢复 fixture 后生成 token/密码，轮换 `admin` 密码并写入唯一 marker（token SHA-256、kind、createdAt）；仅以 mask 后的临时 job 环境传给 build/business，fork PR 不读取 E2E secrets。为 marker 重复/过期（按 `createdAt` + 2 小时阈值，2026-08-27 确认）、未来时间边界精确等价（`validationNow + 60s` 允许）与超界最小增量值（`validationNow + 60s + 1ns`，精度不足时可用 `+1s`，必须拒绝）、摘要不匹配、数据库连接错误、密码轮换失败和 cleanup 失败写红灯。
- [ ] 先写 build 与 business 共用的环境校验：identity handler 必须通过当前应用 DB session 验证 marker 后才返回随机 token 与 `leanote_test`；两类 E2E 均在任何登录前执行预检，business 在写入/route 注入前再次断言。缺变量、非 test mode、非 loopback、错误数据库、错误 token、权限不足、清理失败均为红灯。验证 token 与密码不进入摘要或 artifact；基线不具备服务时记录为前置条件，不伪造通过。

## 2. Switch The Canonical Asset Without Changing The Public URL

**Modify**
- `package.json`, `package-lock.json`
- `scripts/build/manifest.mjs`, `scripts/build` test helpers, `tests/js/build-pipeline.test.js`
- `public/tinymce/plugins/leaui_image/index.html`

**Replace generated asset**
- `public/js/jquery-1.9.0.min.js`（路径保留，内容由 3.7.1 生成）
- manifest 所有受影响的 `dep.min.js`、`album/js/main.all.js` 和其他声明输出

**Delete**
- `public/tinymce/plugins/leaui_image/public/js/jquery.js`

- [ ] 锁定 jquery 3.7.1 和 migrate 3.6.0；验证 Migrate 只作为测试输入出现。
- [ ] 以 npm 的 jQuery 输入生成 direct runtime、dep 和 album；更新 33 -> 34 的所有构建契约与 isolated build-tree fixture。
- [ ] 保留模板、博客主题和 controller 的旧 URL；只改变 `leaui_image` 到 canonical root URL，并验证所有脚本顺序和每文档单核心约束。
- [ ] 运行 `npm run build`，确认没有手工修改任何生成 output，也没有额外的未声明资源。

## 3. Adapt First-Party And Third-Party Compatibility

**Potential files, selected only from the inventory**
- `public/js/common.js`, `public/js/app/**/*.js`, `public/js/plugins/**/*.js`
- `public/{admin,member,blog,album}/**/*.js`
- jQuery 插件的可读源、provenance/license 说明和 manifest 输入

- [ ] 逐项修复清单中第一方 API 和 AJAX/Deferred 失败路径；每项先跑最小静态或 browser regression。
- [ ] 对每个插件验证注册、初始化、事件、销毁、上传和跨 iframe 边界；替换时保留来源/许可证并使可读源、manifest 输入和 production output 同步。
- [ ] 对选入业务流的请求注入 4xx/5xx/解析失败，断言 failure callback 或现有可见失败提示；不接受静默成功、吞异常或日志替代。
- [ ] 发现 stop condition 时停止该插件分支，更新清单和规划，不引入 production Migrate、第二实例或 shim。

## 4. Run Diagnostic And Production Acceptance

- [ ] Playwright diagnostic helper 在 jQuery 之后、应用/插件之前注入 Migrate，覆盖 note/markdown、album/upload、blog、admin/member、`leaui_image` iframe；断言零 `JQMIGRATE:`。
- [ ] 不注入 Migrate 的 `business` project 覆盖登录、笔记列表/搜索、笔记本/标签、对话框、上传、相册、博客、admin/member，验证脚本单实例、无控制台/页面/网络错误和清理测试数据；build smoke 与 business 必须复用同一身份预检 helper，不能让 build project 绕过 token/marker 校验。
- [ ] 在 `.github/workflows/regression-baseline.yml` 的同一 test-mode harness 中依次运行 `npm run test:e2e:build` 与 `npm run test:e2e`；为 business E2E 增加独立脱敏摘要、失败 artifact 和无条件 cleanup。显式检查 workflow 的最小只读 `GITHUB_TOKEN` 权限；任一身份、业务、报告或清理失败必须阻断 PR/push。
- [ ] 运行 `npm ci && npm run build && npm test`、`npm run test:e2e:build`、`npm run test:e2e`、G 的 Golden/USN/page smoke 与定向 Go 测试；连续第二次 build 后运行 `git diff --exit-code` 和 `git diff --check`。
- [ ] 按父任务在真实 Chrome、Edge、Firefox、Safari 的当前及前一主版本完成并记录 release smoke；Chromium business E2E 失败必须阻断合并。

## Rollback Points

- 在第 2 步发布前可丢弃 npm/manifest/测试草案；发布后按 manifest 作为单一原子集回退 runtime、bundle 与模板/iframe 引用。
- 任何必须破坏旧静态 URL、保留 Migrate/双实例或改变 Bootstrap/TinyMCE 的方案均不是可接受 rollback 替代，必须回到规划。
