# Go 工具链与通用依赖现代化（A）— 技术设计

## 1. 变更边界

本任务把工具链升级、通用依赖升级和 vet 清零作为一个低风险阶段，但把 Revel 与 MongoDB 留给各自的宽面迁移。依赖版本变更必须逐个落地，使失败能够归因到一个模块。

## 2. Go 兼容政策

`go 1.26` 是源码最低版本；1.27 是前向兼容门禁。实现与测试均以 1.26 可编译为约束，不使用 1.27 独有 API。CI 两个版本执行同一命令，避免“旧版本只 build、新版本才 test”的假矩阵。

## 3. Vet 问题分组

| 类别 | 主要文件 | 处理与保护 |
|---|---|---|
| 无效 struct tag | `app/info/AlbumInfo.go`、`Api.go`、`AttachInfo.go`、`BlogInfo.go`、`Configinfo.go`、`EmailLogInfo.go`、`FileInfo.go`、`GroupInfo.go`、`NoteInfo.go`、`NotebookInfo.go`、`ReportInfo.go`、`SessionInfo.go`、`ShareNotebookNoteInfo.go`、`SuggestionInfo.go`、`TagInfo.go`、`ThemeInfo.go`、`TokenInfo.go`、`UserInfo.go` | 改为合法 `bson`/`json` tag；先用序列化测试钉住现有字段名与省略行为 |
| unreachable code | `app/lea/captcha/Captcha.go`、`app/lea/File.go`、`app/controllers/AuthController.go`、`app/cmd/build.go`、`app/cmd/harness/build.go` | 删除不可达语句，不调整可达路径 |
| unkeyed literal | `app/lea/blog/Template.go`、`app/service/BlogService.go`、`EmailService.go`、`FileService.go`、`NoteService.go`、`ShareService.go`、`UserService.go` | 改为具名字段；保持字段值和顺序语义 |
| self-assignment | `app/service/NoteService.go`、`app/controllers/member/MemberBlogController.go` | 删除无效赋值并用相邻测试/Golden 证明行为未变 |
| API 误用 | `app/controllers/BaseController.go`、`app/cmd/harness/harness.go` | 修正格式化调用与带缓冲 signal channel，不改变返回内容和关停语义 |

## 4. 依赖升级策略

先用 `go list -m -u -json all` 建立候选表，再只处理 `go.mod` 中非 Revel/MongoDB 的直接依赖。每个直接依赖单独升级、构建和测试；若出现破坏性变化，选择该主版本内仍受维护的兼容版本或把替换记录到现代化 backlog，不在同一提交中增加适配框架。

## 5. 回滚

- Go directive、通用依赖、struct tag/vet 修复分别形成可独立评审的提交批次。
- 任一依赖升级导致 Golden 差异时回退该依赖版本，不更新 Golden。
- 本任务不做数据迁移，因此整体回滚只需回退代码与模块文件。
