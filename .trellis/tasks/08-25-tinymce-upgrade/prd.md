# TinyMCE 8 升级（E-TM）— PRD

父任务：`.trellis/tasks/08-25-frontend-libs`

## Goal

把自托管 TinyMCE 4.1.4 升级到 8.8.2，在保留 Leanote 编辑、粘贴、上传、代码块、目录和思维导图能力的同时，移除旧核心与失效插件副本，并用存量 HTML 兼容契约防止升级改写用户内容。

## Dependencies

- 依赖 `08-25-bootstrap-upgrade`；它已完成 `leaui_image` iframe 的 Bootstrap 5 适配。
- 完成后由协调父任务 `08-25-frontend-libs` 汇总验收，CI/CD 任务等待该父任务完成。

## Requirements

- **R-TM1** TinyMCE 8.8.2 由 npm lockfile 管理，经构建 manifest 生成自托管运行资源；页面不从 CDN 加载编辑器，初始化显式配置 `license_key: 'gpl'`。
- **R-TM2** 迁移并验证四个实际使用的第一方插件：`leaui_image`、`leaui_mindmap`、`leanote_nav`、`leanote_code`。插件只调用 TinyMCE 8 公共 API，不复制核心源码。
- **R-TM3** 把 `public/js/app/page.js`、`public/js/common.js`、`public/js/app/note.js`、博客/会员模板和粘贴处理迁移到 TinyMCE 8 事件、命令、选区、对话框及内容 API。
- **R-TM4** `leaui_mindmap` 是唯一生效的思维导图插件。对 `leaui_mind` 完成引用和行为核验后删除；若发现真实运行入口，先合并唯一功能再删除，不保留两份事实来源。
- **R-TM5** 未发生编辑时不得触发保存，数据库中的笔记 HTML 字节保持不变。发生真实编辑并保存时，允许已记录的空白和属性顺序归一化，但 DOM 语义、文本、链接、图片、代码块和第一方插件标记必须等价。
- **R-TM6** 粘贴仅清理既有安全规则明确禁止的内容；图片 data URL、远程图片、富文本、纯文本、代码片段和 Leanote 内部内容的处理必须有固定夹具。错误不得静默降级成空内容或伪成功。
- **R-TM7** 删除 TinyMCE 4 核心、废弃桥接文件和确认无引用的插件副本；不保留双 TinyMCE、永久兼容层或隐藏回退。
- **R-TM8** 本任务不改变笔记存储 Schema、不批量重写历史 HTML、不引入云端高级插件，也不重新设计编辑器 UI。

## Acceptance Criteria

- [ ] `npm ls tinymce` 只显示 8.8.2，生产页面仅加载构建生成的自托管 TinyMCE 8 资源，且无 CDN、TinyMCE 4 core 或重复 runtime。
- [ ] 四个第一方插件均可初始化、执行主要动作、保存并重新载入；`leaui_mind` 经无引用证明后删除。
- [ ] 现有 `tests/js/paste-plugin.test.js` 迁移并扩充，粘贴夹具覆盖富文本、纯文本、图片、代码和失败路径。
- [ ] 存量 HTML 夹具证明“打开后未编辑不保存且 DB 字节不变”；实际编辑保存后通过 DOM 归一比较，语义字段和插件标记不丢失。
- [ ] Chromium 阻塞 E2E 覆盖打开、编辑、撤销/重做、粘贴、上传图片、代码块、目录、思维导图、自动保存和重新载入。
- [ ] 当前及前一主版本 Chrome/Edge/Firefox/Safari 的发布前 smoke 通过；非 Chromium smoke 结果被记录但不冒充自动化 Safari 等价验证。
- [ ] `npm run build && npm test`、Chromium E2E、Golden/USN 回归及连续两次构建零 diff 全部通过。
- [ ] 浏览器控制台无未处理异常、TinyMCE deprecation warning、许可提示或编辑器资源 404。
- [ ] diff 不包含历史笔记批量迁移、内容 Schema 变化或编辑器视觉重设计。

## Out of Scope

- 批量规范化或重写数据库中的历史 HTML。
- TinyMCE 云服务、付费插件和协同编辑。
- 用另一款编辑器替换 TinyMCE。
- Bootstrap、jQuery 的版本升级；它们由前序子任务负责。
