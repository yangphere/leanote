# Bootstrap 5.3 浏览器 smoke 记录

本文件是 Bootstrap 5.3 任务的受跟踪发布验收记录。每次执行必须追加记录，不得写入账号、密码、Cookie、认证头、页面正文、用户数据、trace、截图或视频。

## 记录字段

每条记录必须包含：

- 基线提交 SHA、执行日期和执行环境。
- 浏览器产品与完整版本，并标明当前版本或前一主版本。
- 操作系统、Node 版本、Playwright 版本和 test-mode harness/run-token 预检结果。
- 覆盖页面与 iframe：login/register、note（富文本与 markdown）、modal/tab/dropdown/alert/tooltip、表单 loading/error、album 真实上传、admin/member、三套内置博客主题、评论/举报/二维码/share 和 `leaui_image`。
- console.error、pageerror、unhandled rejection、资源 4xx/5xx、旧 Bootstrap URL/核心字节和数据边界检查结果。
- 结果与失败原因；任一预检、清理、错误门禁或资源门禁失败均为失败。

## 当前记录

| 浏览器 | 完整版本 | 当前/前一主版本 | 操作系统 | 覆盖与预检 | 错误/网络门禁 | 结果 |
|---|---|---|---|---|---|---|
| 待执行 | - | - | - | - | - | 阻断 |

发布验收必须补齐真实 Chrome、Edge、Firefox、Safari 的当前及前一主版本；Safari 只接受真实 Safari 环境结果，不能用 Chromium 代替。最终合并 SHA 产生后需重新执行并追加记录。
