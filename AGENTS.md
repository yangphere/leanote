# Repository Guidelines

## 项目结构与模块组织

Leanote 是 Go 1.15、Revel 1.0 与 MongoDB 构成的单体应用。`app/controllers/` 处理 Web、管理端、会员端和 `/api/*` 请求，`app/service/` 放业务逻辑，`app/db/` 负责 MongoDB 访问，`app/info/` 定义模型，`app/views/` 保存 Revel 模板。浏览器代码、样式和图片位于 `public/`，翻译位于 `messages/<locale>/`，路由与运行配置位于 `conf/`。Go 测试在 `app/tests/`，Node 测试在 `tests/js/`；`tests/apptest.go` 是未启用的旧 Revel 测试入口。

## 构建、测试与本地开发

本地运行需要 MongoDB 和 Revel CLI：

```bash
go get github.com/revel/cmd/revel@v1.0.3
mongorestore -h localhost -d leanote --dir ./mongodb_backup/leanote_install_data/
cd sh && sh run.sh                 # 在 :9000 启动开发服务器
go test ./app/tests/...            # 需要已运行且已初始化的 MongoDB
npm test                           # 运行 tests/js/*.test.js
cd sh && sh package.sh             # 生成 sh/leanote.tar.gz
```

前端生成统一使用 Node 24 构建链：`npm ci && npm run build && npm test`。`scripts/build/manifest.mjs` 与 `app/views/note/note-dev.html` 是唯一来源，生成的受跟踪资源不得手工修改。

## 编码风格与命名

Go 代码提交前运行 `gofmt`；包名小写，导出标识符用 PascalCase，局部变量用 camelCase，并遵循相邻文件的历史命名。旧前端通常使用四空格缩进，Node 测试使用两空格；不要对 vendored 或压缩文件做无关格式化。`app/views/note/note-dev.html` 是编辑器页面源文件，相关修改需同步生成的 `note.html`。修改 TinyMCE 行为时，同步可读源码和所有生产 bundle。

## 测试准则

Go 测试使用标准 `testing` 包，文件命名为 `*_test.go`、用例命名为 `TestXxx`；JavaScript 使用 `node:test` 和 `*.test.js`。仓库未配置数值覆盖率门槛，但每个修复应包含聚焦的回归用例。UI 或服务改动除自动化测试外，还应通过真实页面或请求验证。

## 提交与 Pull Request

历史提交多为简短祈使句，如 `fix duplicate pasted images`；也接受明确前缀，如 `fix:`、`chore:`。每个提交聚焦一个目的。PR 应说明问题、行为变化和验证命令，关联 issue；涉及 `app/views/` 或 `public/` 的可见变化应附截图，并注明更新过的生成资源。

## 安全与 Agent 说明

不要提交真实数据库、SMTP、Cookie 或 `app.secret` 凭据；以 `conf/app.conf-default` 为配置参考。用户数据、上传文件和本地生成物不得进入版本库。

<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->
