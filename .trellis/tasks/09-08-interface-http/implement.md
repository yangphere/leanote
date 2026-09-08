# 接口适配层：标准库 HTTP 与路由 — 执行计划

- [ ] 盘点 `conf/routes` 全部路由、旧 controller actions、filters、binders、templates、session 和 i18n。
- [ ] 完善 route table、registry、参数绑定、Session、ViewArgs、模板函数、i18n、gzip、恢复和错误 Result。
- [ ] 按业务应用任务迁移主站、API、admin、member 全部 actions，确保每批有 Golden/权限回归。
- [ ] 把 harness、`cmd/leanote`、`sh/run.sh`、`sh/package.sh` 和 CI 切换到标准库入口。
- [ ] 清扫 `app/init.go`、service/lea 调用方、`app/cmd`、go.mod/go.sum 和文档中的 Revel 生产引用。
- [ ] 运行完整 route negative、Golden/USN、session/cookie、模板/静态资源和 SIGTERM 验证。
