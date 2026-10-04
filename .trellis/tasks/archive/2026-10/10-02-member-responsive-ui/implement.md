# 实施计划

1. 更新 `app/views/admin/top.html`、`app/views/admin/nav.html` 与 `app/views/member/top.html`：增加页面作用域类，移除会阻断窄屏访问的 `d-none d-sm-block` 外壳导航标记（包括 admin 子模板的 `<nav>`），保持现有 DOM id、href 和脚本契约。
2. 更新 `app/views/member/group/index.html`：同时修改 jsrender `#tGroup` 和服务端 `{{range .groups}}` 两处 `.col-sm-4 each-group`，使它们使用同一个专用网格 class；删除模板内联 `.group-title { width: 200px; }`，将输入的可收缩、最大宽度和 focus 样式移到 member LESS/CSS；保留 `.group-title`、`.add-user-input`、`.delete-group`、`.delete-user` 和 `#groups` 选择器。同步更新 `tests/js/fixtures/build/i18n-contract.json` 中因删除 9 行样式而变化的 7 个行号锚点。
3. 在 `public/admin/css/admin.less`、`public/member/css/member.less` 末尾追加共享 shell、导航、内容容器、表格/表单和卡片的纯 CSS 响应式规则；将每个追加块逐字同步到对应的 `admin.css`、`member.css`，并保持规则放在文件末尾以覆盖旧主题规则。
4. 增加或扩展静态契约测试，断言两份组卡片 class 一致、内联固定宽度已移除、每个 `.less`/`.css` 对的追加响应式块逐字一致；运行静态检查，确认规则不会被 Bootstrap 的 `.d-none`、固定 `aside-md` 或旧表格布局覆盖。
5. 运行 Node 构建/测试和 diff 检查；若具备真实服务和浏览器环境，再执行 375/600/768/1024/1440px 的 `/admin`、`/member` 冒烟验证，对 `#content`、`.scrollable`、`.card` 逐元素断言 `scrollWidth <= clientWidth`（`.table-responsive` 除外），记录运行或未运行证据。

## Risk points

- 移动端头部高度由内容决定，因此同时取消窄屏固定定位，避免固定的 `padding-top: 50px` 覆盖换行后的导航。
- admin 原模板在 `aside` 与 `nav` 同时使用 `d-none d-sm-block`；必须一起调整，否则窄屏只会得到空白内容区。
- `.hbox` 在桌面仍依赖 table-cell；窄屏规则只在 `max-width: 767px` 生效，避免影响既有桌面高度计算。

## 实施与验证记录

- 已完成 admin/member 顶层模板、导航和用户组卡片模板的响应式改造；`#tGroup` 与服务端分支使用同一 `group-grid-item` class，模板内联固定宽度已移除。
- 已在四个 LESS/CSS 文件追加逐字同步的纯 CSS 响应式规则，并移除会掩盖横向溢出的 shell `overflow-x: hidden`；移动端同时清除旧固定头部留下的 `50px` 顶部补偿。
- 复现运行页后确认 Bootstrap 5 的 `.nav` 默认横向 flex 布局覆盖了旧管理导航的纵向假设，导致侧栏菜单挤成截图中的碎片；四个样式文件现对管理外壳一级 `.nav`、`li` 和链接显式使用纵向 block 布局。
- `node --test tests/js/frontend-responsive-contract.test.js`：3/3 通过。
- `npm run build`：通过。
- `npm test`：220 项中 219 通过、1 项按条件跳过、0 失败。
- `npm run test:e2e:build -- --list`：发现 1 个 build-smoke 用例；`git diff --check` 与 `task.py validate` 均通过。
- `docker compose up -d --build --force-recreate leanote`：重建并重启成功；`/healthz` 返回 `200 {"status":"ready"}`，容器内 CSS 与宿主机资源一致。
- 登录后的 Playwright 实际页面验证：`/member/index` 在 375/600/1536px 下一级导航为 `display:block`，`#content`、`.scrollable`、`.card` 无横向溢出；`/member/group/index` 在 375/600/768/1024/1440px 下分别呈单列、两列或三列，所有检查项无横向溢出。
- 用户当前已登录的真实 Chrome 标签页未由本次会话直接操作；本地 Docker Mongo 与登录浏览器冒烟已运行，生产环境和外部浏览器会话仍未验证。

