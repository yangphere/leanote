# 执行计划

## 1. 准备

- [x] 核对用户批准及前置 `meta.depends_on`：归档任务 `10-04-ghcr-tag-image-publish` 为 completed，发布无前缀 `2.0.1`。
- [x] 同步 PRD 过期的版本前缀和运行证据条件，补充设计、执行计划和子代理上下文。
- [x] 验证上下文并激活现有任务。

## 2. 实现

- [x] 先新增生产镜像/必填变量/dev override 的 focused 失败契约测试。
- [x] 拆分 Compose 文件，保持共享服务和安全/持久化规则。
- [x] 同步 `.env.example`、README、smoke 注释和交付文档。
- [x] 运行 focused release-contract（31/31）和 `npm test`（241 passed / 1 skipped / 0 failed）。

## 3. 验证

- [x] 使用非敏感 fixture 渲染生产与 dev，检查 image/build/共享拓扑。
- [x] 检查生产缺少/空镜像版本、dev 缺少构建版本时必填失败；生产无需 `LEANOTE_VERSION`。
- [x] 独立项目真实 pull/up `2.0.1`，请求 `/healthz`，核对非 root 和镜像摘要；重建验证新测试卷保留。
- [x] Trellis check 子代理复核实现、测试、规格与任务元数据，无代码 findings。
- [x] `git diff --check` 与任务上下文校验，写入 `validation.md`。
- [x] 同步 Compose 使用契约到适用 spec。

## 范围与收尾

产品改动包含 PRD 中 Compose 文件及用户追加的 GHCR 审查修复；Trellis 文档记录规划和证据，spec 记录消费/晋升契约。保留用户已有 `CONTEXT.md` 改动，不改本地 `.env`。本轮只实施与检查，不自动提交、归档、推送或部署当前堆栈。

## 4. GHCR 审查修复（用户追加授权）

- [x] 核实实际工作流和 GitHub 官方并发文档，记录研究证据并更新范围。
- [x] 实现代理先写失败回归（7 项 red），再修复共享 latest 晋升规则、队列和默认禁建包。
- [x] focused release-contract/docker-image-workflow 50/50；完整 npm test 单实例 exit 0，243 passed / 1 skipped / 0 failed。
- [x] YAML 1.2 与 12 个 shell block 静态语法检查通过，真实只读 Skopeo/新 CLI 验证通过；actionlint 未安装，标为 unrun。
- [x] 检查代理全范围复核，同步 ADR/spec/任务元数据与证据。
- [x] 核实 GitHub 官方事件 SHA/ref 工作流选择规则，记录历史 tag 工作流无法追溯加固及禁止补推旧提交 tag 的适用边界。
- [x] diff --check/context validate 通过；保留远端队列/写入操作 unrun。
