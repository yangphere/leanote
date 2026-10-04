# 管理后台响应式样式设计

## Scope

本任务覆盖 `/admin` 和 `/member` 两套管理界面。两者都复用同一套旧版 `vbox/hbox` 结构和 Bootstrap 工具类，但分别加载 `public/admin/css/admin.css` 与 `public/member/css/member.css`；笔记编辑器和公开博客不在范围内。

## Approach

1. 在 admin/member 顶层模板上补充可识别的页面作用域类，并移除会在窄屏直接隐藏导航的桌面专用工具类。保留现有链接、激活逻辑和脚本选择器。
2. 在两个 LESS 源文件的末尾增加同构的 shell 响应式规则：桌面使用固定侧栏与可收缩内容列，窄屏切换为自然文档流，头部允许换行，侧栏全宽显示，内容区设置 `min-width: 0`，表格和媒体内容在自己的容器内滚动或换行。
3. 将用户组页的卡片容器标记为专用网格，并让卡片在窄屏单列、中等宽度双列、大屏三列；`#tGroup` 和服务端渲染分支使用同一 class。移除模板内联的固定 `.group-title { width: 200px; }`，把可收缩输入规则收归样式文件；成员文本和操作控件使用可收缩尺寸。通用管理表格、表单、卡片只通过 CSS 约束宽度和间距，不改变数据结构。
4. 由于仓库当前没有 LESS 编译脚本，维护的 `.less` 与实际服务的 `.css` 在文件末尾追加同一组纯 CSS 规则，要求对应追加块逐字一致；不重排已有生成内容。

## Invariants

- 页面 URL、模板变量、翻译 key、AJAX endpoint、group 模块的 class/id 选择器不变。
- 桌面宽度继续保留侧栏导航和内容并排结构；窄屏只改变布局方向和尺寸，不隐藏用户必须访问的管理链接。
- 表格仍使用现有 `.table-responsive`，不把长文本截断为不可读内容。

## Validation

- 静态检查两个顶层模板包含作用域类、导航仍可见、组页交互选择器未变。
- `npm ci && npm run build && npm test`，再运行 `npm run test:e2e:build -- --list` 与 `git diff --check`。
- 真实 `/admin`、`/member` 页面在 375/600/768/1024/1440px 的浏览器截图和逐元素横向溢出检查需要 Mongo/登录运行环境；`.table-responsive` 允许自身横向滚动，其他 `#content`、`.scrollable`、`.card` 元素必须满足 `scrollWidth <= clientWidth`。若环境不可用，明确记录为未运行。

