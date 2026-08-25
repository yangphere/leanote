# jQuery 3.7 升级（E-jQ）— 技术设计

## 1. 资源归一

`jquery` 作为 npm 依赖，由 D 的 manifest 复制/压缩到统一运行路径。主应用和 `leaui_image` iframe 使用同一锁定版本，不再在插件目录维护独立核心源码。TinyMCE 的 `jquery.tinymce` 桥接文件属于旧编辑器 core，由后续 TinyMCE 任务处理，不误判成 jQuery 核心副本。

## 2. 诊断流程

开发诊断入口在 jQuery 后加载 migrate，并把 warning 视为测试失败。每条 warning 追到第一方调用或具体第三方插件：第一方直接改；第三方优先升级到与 jQuery 3.7 兼容的同类版本，若升级会改变 UI/API，则在本任务内做最小调用适配而不是复制插件源码。

最终生产入口不加载 migrate，构建测试检查依赖图和 bundle 文本均无 migrate。

## 3. 高风险 API

- `.bind/.unbind/.delegate`、事件 shorthand 与 `event.returnValue`。
- `.size()`、`.andSelf()`、旧 `.load/.error` 事件写法。
- `$.parseJSON`、`$.isArray`、`$.trim`、Deferred/AJAX 回调差异。
- `.data()` 属性解析、`:visible`、表单序列化与跨 iframe jQuery 对象。
- fileupload、validation、zTree、contextmenu、slimScroll、artDialog 的插件注册。

适配优先修改调用者；只有多个稳定调用共享同一语义时才提取第一方 helper。

## 4. 回滚

运行路径与构建 manifest 的 jQuery 版本变更作为独立单元。若某第三方插件无兼容路径，停止并把替代选择带回规划，不通过长期 migrate 或双 jQuery 实例绕过。
