# jQuery 3.7 升级（E-jQ）— 技术设计

## 1. Asset Data Flow And URL Compatibility

`package-lock.json` 锁定 `jquery@3.7.1`。manifest 的 `jquery-runtime` 从 `node_modules/jquery/dist/jquery.min.js` 发布到现有的 `public/js/jquery-1.9.0.min.js`，并将该输出加入 canonical output 集合（33 -> 34）。文件名是历史公开 URL，不是运行时版本声明。

`dep` 和 `album` 使用同一 npm 输入构建，不能以 `jquery-runtime` 输出回读，避免生成物成为另一个源。普通页面、博客主题变量与 TinyMCE image dialog 继续请求旧 URL；其请求语义不变而内容升级为 3.7.1。`leaui_image` 删除私有 1.9.1，并改为请求根路径的 canonical URL。每个页面/iframe 的脚本序列在清单中记录并由测试检查只执行一个核心。

manifest 扩展同时更新 `BUILD_OUTPUTS`、输出数量、i18n exclusion、原子发布/恢复、Git 跟踪和 build-tree fixture。测试 copy tree 必须复制声明的 `node_modules/jquery` 输入，不能偷偷读取原仓库外的 node_modules。

## 2. Compatibility Ownership And Adaptation

实现前先把全量扫描和运行时诊断固化到受跟踪清单。清单不是第二份资源清单，而是每个兼容决策的审计证据：页面/iframe、调用或插件、所有者、警告/风险、预期行为、适配或上游版本、测试与排除理由。

第一方代码直接替换废弃 API。第三方代码优先使用有来源、许可证和版本证据的兼容上游版本；生产输入必须来自该可读来源，并由 manifest 生成最终输出。只有多个稳定第一方调用拥有同一已证明语义时才提取 helper；禁止为旧 API 建全局 shim。

TinyMCE 内部 bridge 与脑图子应用只登记到清单的 E-TM 排除区。它们不是未审计遗漏，也不允许在 E-jQ 中修改以伪造“无旧 jQuery”。

## 3. Failure Semantics

`_ajax` 及其公开包装保留当前调用约定：成功与 HTTP/解析失败都进入 `_ajaxCallback`，后者调用 `failureFunc`，或显示现有明确失败提示。适配后不能把 jqXHR、Deferred rejection 或同步异常转换为成功、空回调或仅日志。

业务 E2E 对清单选定的真实请求使用 Playwright route 注入受控失败响应，断言页面可见失败状态/既有 alert 或 failure callback、无 pageerror，且测试不会修改生产数据。直接 `$.get`/`$.post` 没有错误处理的业务路径必须在清单中得到明确适配和回归，不得靠未覆盖来保留静默失败。

E2E 使用单一 test-mode harness。恢复 `leanote_test` 后，harness 生成随机 run token 与 admin 密码，将 `{kind: "jquery-upgrade", tokenSha256, createdAt}` 作为唯一 marker 写入 `e2e_runs`，并用现有密码哈希逻辑轮换 fixture `admin` 的密码。账号名、密码和 token 只经本次运行子进程环境传递；CI 在写入临时 job 环境前 mask 值，不能读取 GitHub Secrets，也不能在日志、摘要或 artifact 输出它们。

`GET /_test/e2e/identity` 仅在 test mode 且请求来源为 loopback 时可达。它通过应用已连接的 MongoDB session 查询该唯一 marker，以常量时间比较 marker 的 `tokenSha256` 与环境 token 的摘要，并从同一 session 取得 database 名；只有 marker 有且仅有一条、未过期、摘要匹配且 database 为 `leanote_test` 时才返回 `{runToken, database}`。非 test/非 loopback 返回 404；marker、数据库或连接校验失败返回无敏感细节的 503。该 handler 只读且不记录 token。

共享 `tests/e2e/e2e-environment.mjs` 同时供 build-smoke 与 business project 使用，在**任何登录**前完成身份预检；business 流程在任何 route 注入和写入前断言该预检结果仍有效，随后验证 admin/member 页面权限。测试数据在各用例 finally 清理，harness 最后删除 marker 和数据库容器；任何预检、清理或报告失败都终止运行。

## 4. Test-Only Migrate Diagnostics

`jquery-migrate@3.6.0` 只在 Playwright diagnostics helper 读取。helper 为直接 runtime URL 和 manifest bundle 各生成内存/临时诊断响应，把 Migrate 放在 jQuery 核心之后、业务/插件之前；临时内容位于测试目录或 OS 临时目录，测试结束删除，永不进入 `public/`、manifest 或服务输出。

同一业务流程运行两次：diagnostic 模式收集并断言零 `JQMIGRATE:`；production 模式不注入 Migrate，断言应用拥有资源的网络失败、`console.error`、`pageerror`、未处理 rejection 均为空。摘要遵循 D 的脱敏规则，不写入 cookie、token、storage、页面正文、trace、截图或视频。

## 5. CI And Browser Evidence

`regression-baseline.yml` 在同一 test-mode harness 中依次运行 build smoke 与 business E2E；两者共享恢复 fixture、随机 token 和随机化 admin 凭据，但分别写入脱敏摘要。workflow 必须对同仓与 fork PR 使用相同的 harness 供应路径，不能以 secrets 是否可用决定是否执行；只在两个命令完成后执行服务/容器 cleanup，任何步骤失败均保持失败状态并上传 allowlisted 摘要，绝不以 build smoke 代替 business E2E。

Chromium 是 PR/push 阻断项目。发布前使用真实 Chrome、Edge、Firefox、Safari 的当前与前一主版本，在同一 test-mode harness 中运行 AC-jQ6 的登录、笔记/笔记本/标签、对话框、上传清理、相册、博客、admin/member 和 `leaui_image` iframe smoke，并检查认证、console/page/unhandled-rejection/owned-resource 门禁。将产品、完整版本、OS、commit、覆盖路由/iframe、认证结果、错误门禁和结果写入受跟踪记录；Safari 只接受真实 Safari 结果，Chromium 不能充当 Chrome/Edge 结果。

## 6. Rollback And Stop Conditions

运行时输入、manifest 输出和生成资源作为一个原子变更回滚。若任一插件只能使用生产 Migrate、双 jQuery、未审计 fork，或被迫改变公开 URL/API/UI，停止该分支并带着清单和复现测试回到规划；不以局部 fallback 继续交付。
