# TinyMCE 8 升级（E-TM）— 技术设计

## 1. 单一运行资源

`tinymce@8.8.2` 是唯一核心来源。D 的构建 manifest 把 npm 包中必要的 core、theme、icons、model、skin 和 language 资源复制到稳定的 `public/tinymce/` 运行路径；模板 URL 尽量保持稳定，但产物必须能追溯到 lockfile，不能继续手工维护 vendored core。

构建测试校验资源清单、版本和引用闭包，禁止 CDN、第二份 core 与未声明文件。第一方插件源码仍归仓库管理，由同一 manifest 复制或打包。

## 2. 初始化与 API 迁移

以 `public/js/app/page.js` 的编辑器初始化为唯一主入口，抽取可测试的配置生成逻辑，显式配置 GPL 许可、语言、skin、content CSS、插件、toolbar、粘贴和保存事件。`common.js`、`note.js` 及模板只通过该入口或命名明确的 adapter 与编辑器交互。

逐项替换 TinyMCE 4 的全局对象、事件、命令、selection、DOM、windowManager/dialog 和 plugin manager API。适配层只承载 Leanote 自身的稳定语义，不模拟完整 TinyMCE 4 API。

## 3. 第一方插件边界

- `leaui_image`：保留上传、图库选择、替代文本和插入/更新图片语义；其 iframe 已由 Bootstrap 子任务迁移。
- `leaui_mindmap`：保留当前 `page.js` 实际注册的思维导图标记、编辑和重开行为。
- `leanote_nav`：保留目录/锚点生成及再次编辑行为。
- `leanote_code`：保留语言、代码文本和相关 class/data 属性。
- `leaui_mind`：先做全仓引用和运行时注册核验；确认不在资源闭包后删除。若包含 `leaui_mindmap` 缺失的真实能力，只把该能力并入后者再删除源副本。

每个插件都有独立初始化、命令执行、序列化和重新载入测试，避免用一次“编辑器能启动”代替插件验收。

## 4. 内容兼容契约

在 `tests/fixtures/editor-html/` 保存有代表性的历史内容和期望语义，包括普通富文本、链接、图片、表格、代码块、目录、思维导图及混合内容。测试分为两类：

1. **只读路径**：加载后不产生 change/dirty 信号，不调用保存接口，持久化字节完全不变。
2. **真实编辑路径**：执行一个明确编辑后保存；比较解析后的 DOM 语义。白空格及属性顺序按文档化规则归一，文本、链接目标、图片属性、代码内容和插件标记逐项严格比较。

任何未列入允许集合的节点、属性或内容变化都失败，不用宽泛 HTML 清洗掩盖差异。

## 5. 粘贴与错误可见性

把旧 paste 插件依赖的回调迁移到 TinyMCE 8 公开 paste/preprocess 事件，并复用单一清理函数。夹具固定输入与输出，覆盖远程/内联图片、Office 风格富文本、纯文本、代码和非法节点。上传或转换失败保留原可恢复内容并显示明确错误，不吞错、不返回空字符串伪装成功。

## 6. 浏览器验证

`@playwright/test` 提供 Chromium 阻塞 E2E，复用登录和种子数据夹具。当前及前一主版本 Chrome/Edge/Firefox/Safari 做发布前 smoke；Safari 结果明确来自真实 Safari 环境，不以 Playwright WebKit 冒充。浏览器矩阵和证据由父任务及 CI/CD 任务汇总。

## 7. 回滚与发布

TinyMCE 运行资源和调用方迁移作为一个原子单元，不部署双版本选择开关。若内容兼容夹具、第一方插件或粘贴行为未通过，回退整个 TinyMCE 子任务；不得以旧 core 隐藏回退继续发布。
