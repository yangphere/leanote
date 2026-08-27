# jQuery 3.7 升级（E-jQ）— 执行计划

## Global Constraints

- 目标固定 3.7.1，不升级 4.0。
- migrate 只用于开发诊断，生产零依赖、零 warning。
- 不升级 Bootstrap/TinyMCE，不维护双 jQuery。

### Task 1：建立 jQuery 行为与插件清单

**Files:**
- Create: `tests/js/jquery-compat.test.js`
- Create: `tests/e2e/business/jquery-flows.spec.js`
- Read: `public/js/jquery-1.9.0.min.js`、`public/js/app/`、`public/js/plugins/`、`public/admin/js/`、`public/member/js/`、`public/blog/js/`、`public/album/js/`

- [ ] 用源码搜索记录废弃 API 和所有 jQuery 插件注册点，按主应用/admin/member/blog/album/leaui_image 分类。
- [ ] 写 node/jsdom 可覆盖的工具函数测试和 Chromium 用户流测试。
- [ ] 在仍使用 1.9 的基线上运行并保存结果；把仅 3.7 会失败的断言先置于目标版本测试入口，确认红灯。

### Task 2：切换单一 jQuery 3.7.1

**Files:**
- Modify: `package.json`、`package-lock.json`、`scripts/build/manifest.mjs`
- Modify: templates that load `public/js/jquery-1.9.0.min.js`
- Delete/Replace: `public/js/jquery-1.9.0.min.js`、`public/tinymce/plugins/leaui_image/public/js/jquery.js`

- [ ] 安装锁定 jQuery 3.7.1，并由构建链生成统一静态路径。
- [ ] 更新所有页面与 iframe 加载点，断言同一页面不会加载两次或加载两个版本。
- [ ] 运行构建与静态资源 smoke，先收集 migrate warning 和运行异常。

### Task 3：逐区域消除兼容问题

**Files:** `public/js/common.js`、`public/js/app/*.js`、`public/js/plugins/*.js`、`public/admin/js/*.js`、`public/member/js/*.js`、`public/blog/js/*.js`、`public/album/js/main.js`、`public/tinymce/plugins/leaui_image/public/js/*.js`

- [ ] 先修第一方废弃 API，每类修复后运行对应单测/E2E。
- [ ] 对 validation、fileupload、zTree、contextmenu、slimScroll、artDialog 逐个验证初始化、事件与销毁；必要时升级到兼容版本并锁定。
- [ ] 验证跨 iframe 的 `parent.getMsg`、上传、相册选择和插件 jQuery 对象边界。
- [ ] 验证 AJAX 成功/失败、Deferred 和表单序列化，错误路径必须可见。

### Task 4：移除 migrate 与完整验收

- [ ] 从生产 dependency、manifest 和模板移除 migrate；构建测试扫描 bundle 确认无残留。
- [ ] 在无 migrate 环境运行 `npm run build && npm test` 和 Chromium E2E。
- [ ] 启动真实服务，验证控制台零 warning/error、静态资源零 404。
- [ ] 回放 Golden、USN、页面 smoke，连续两次构建后零 diff。
- [ ] 复核 diff 未提前升级 Bootstrap/TinyMCE 或引入永久兼容层。

## Rollback Point

可整体回退到构建链中的 jQuery 1.9 输入。若只能靠生产 migrate 或双实例才能运行，任务不满足验收，不合并。
