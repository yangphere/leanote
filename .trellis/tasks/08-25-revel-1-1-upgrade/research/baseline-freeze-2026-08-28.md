# C-a Task 1：v1.0 三路径基线冻结（2026-08-28）

冻结点：`app/ conf/ sh/ go.mod go.sum` 在本轮 C-a 改动前的工作树（= HEAD 提交树）。
注意：HEAD 相对 origin/dev 含已提交的 jquery 任务改动（app/views 模板、TestE2e 控制器、
`/_test/e2e/identity` 路由、go.mod 模块路径改名 yangphere），因此 **dev/package 路径的 v1.0
基线须在本 HEAD 重取**（A 08-26 的 Linux smoke 早于这些模板变化，只作参照不作基线）。

- 测试二进制路径（本 HEAD 实证，2026-08-28 A Phase 6 复验 = 同一代码树）：
  `go clean -cache` 后双版本（go1.27.0 缺省、go1.26.7 运行器+LEANOTE_TEST_GO）build/vet 零输出 exit 0；
  MongoDB 5.0 fixture `go test -p 1 ./app/tests/... -count=1` replay ×2 exit 0，68 个测试函数；
  Golden 132 文件 SHA256 聚合 `f6ec2ec036b91340bbf44c6387282825ac1a94de`（升级后对照基线）。
- 开发/生产包路径 v1.0 基线：见 `linux-v1.0-baseline-run.log`（golang:1.26.7-bookworm 容器，
  --network container:leanote-test-mongo，主模块图构建的 v1.0.3 CLI，覆盖 /、/login、/note、/blog、/demo、
  sh/package.sh 解包启动、SIGTERM 行为观察）。
- 模块图快照：`research/module-snapshot-v1.0.txt`（go.mod sha1 `68a37618…`、go.sum sha1 `907a4d32…`）。
- v1.0 代 Revel 族终态：revel v1.0.0、cmd v1.0.3、modules v1.0.0、config v1.0.0、cron v0.21.0（图内既有）、
  log15 v2.11.20+incompatible、pathtree v0.0.0-20140121。
