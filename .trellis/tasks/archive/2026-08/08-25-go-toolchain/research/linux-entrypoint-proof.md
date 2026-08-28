# Linux 入口验证记录（Phase 5，2026-08-26）

任务：`08-25-go-toolchain` Phase 5「在受控 Linux 环境运行 app/cmd 源码生成、生成后二进制
build、revel run 真实 HTTP smoke 和 revel package；检查 tarball 存在、非空、可解包」。

## 环境与隔离

- 容器：`golang:1.26.7-bookworm`（digest `sha256:e8c859f5...`），全程 `GOTOOLCHAIN=local`。
- 仓库以 bind mount 挂到 `/src`（A 阶段按 dispatch 示例直接在挂载上执行）。
- MongoDB：复用 G 的 fixture 容器 `leanote-test-mongo`（`go run ./app/tests/harness/cmd/env up`
  创建，mongo:5.0，已 restore `leanote_test`），并额外 restore 一份到 `leanote` 库
  （dev/prod 模式全局配置 `db.host=127.0.0.1:27017`/`db.dbname=leanote` 所需）。
- **网络方案**：验证容器以 `--network container:leanote-test-mongo` 共享 Mongo 的网络命名空间，
  使容器内 `127.0.0.1:27017` 即为该 Mongo——不修改任何被跟踪的 conf 文件即可满足 `db.Init` 的
  连接目标。先以 `(echo > /dev/tcp/127.0.0.1/27017)` 探针确认可达。
- B 阶段（canonical 脚本）运行于容器内副本 `/work`（tar 复制排除 `.git`），避免 Windows bind
  mount 的 inotify 局限；副本内 `sed -i 's/\r$//' run.sh package.sh` 去除 Windows checkout 的
  CRLF（仅改副本，工作区零接触）。脚本：`linux-entrypoint-validation.sh`（Temp 目录，
  内容见下文命令逐条记录）。

## A. app/cmd 源码生成 + 生成后二进制构建（Go 1.26.7, linux/amd64）

```bash
cd /src/app/cmd && go run . build -v ../../ ./tmptmp     # GENERATION OK，无 panic
# 输出：/src/app/routes/routes.go (61402 bytes)、/src/app/tmp/main.go + tmp/run/
cd /src && go build -o /tmp/leanote-bin github.com/yangphere/leanote/app/tmp
# BINARY BUILD OK，/tmp/leanote-bin 21941825 bytes
rm -rf /src/app/tmp /src/app/routes /src/app/cmd/tmptmp # 三个路径均确认不存在
```

结论：x/tools v0.49.0 bootstrap 后，真实生成入口在 Linux Go 1.26.7 下不再 panic，
生成产物可构建。与 harness `buildServerBinary` 流程等效。

## B. Revel CLI 在 Go 1.26.7 下的两条构建路径

### B2a 上游默认路径：`go install github.com/revel/cmd/revel@v1.0.3`

安装成功，但**执行 `revel run`/`revel package` 必然 panic**（证据：
`research/linux-stock-cli-panic-go1.26.7.log`，取自同日第二次容器会话的完整输出）：

```text
Changed detected, recompiling
Parsing packages, (may require download if not cached)...The following panic happened ...
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
go/types.(*StdSizes).Sizeof(0x0, ...)
golang.org/x/tools/go/packages.(*loader).loadPackage(...)
    /go/pkg/mod/golang.org/x/tools@v0.0.0-20200219054238-753a1d49df85/go/packages/packages.go:862
```

根因：`@version` 安装按 **revel 模块自身依赖图**编译 CLI，其中 `github.com/revel/revel@v1.0.0`
钉死 `golang.org/x/tools v0.0.0-20200219...`；旧 x/tools 与 Go 1.26+ 的 go/types 不兼容，
正是本任务 README 迁移前记载的同类 panic。该依赖图属于上游 Revel，不在 A 的模块所有权内
（Revel 版本由 C-a 拥有，A 禁止变更）。

### B2b 模块图路径：`go build -o /tmp/revel github.com/revel/cmd/revel`（在主模块内）

同一份 `github.com/revel/cmd v1.0.3` 源码（主模块直接依赖，go.mod:12），共享依赖解析为
已批准的 `golang.org/x/tools v0.49.0` ⇒ 编译成功、运行无 panic。这是开发者在本仓库内的
自然构建方式（不带 `@version` 的模块模式安装）。

## B3. revel run 真实 HTTP smoke（canonical sh/run.sh）

```bash
cd /work/sh && PATH="/tmp:$PATH" sh run.sh &   # run.sh: SCRIPTPATH=$(dirname $PWD); cd ..; revel run -a .
```

curl 结果（Mongo fixture 已恢复 leanote 库）：

| 请求 | 结果 | 服务端 request log |
|---|---|---|
| GET /login | **HTTP 200** | Auth.Login status=200 |
| GET / | HTTP 200 | Index.Default status=200 |
| GET /blog | HTTP 200 | Blog.Index status=200 |
| GET /demo | HTTP 302 | Auth.Demo status=302（重定向，符合预期） |

服务由 Revel dev 模式真实启动（watcher "Rebuilt, result error=nil"），非直连 controller。

## B4. revel package 生产打包（canonical sh/package.sh）

```bash
cd /work/sh && PATH="/tmp:$PATH" sh package.sh
# Your application has been built in: /tmp/work1712129466
# Your archive is ready: /work/sh/leanote.tar.gz
ls -l  → 27446759 bytes（存在且非空）
tar -tzf → 2303 entries（含 run.sh/run.bat/src/github.com/yangphere/leanote/app/views/**）
tar -xzf → 解包成功（/tmp/unpack/{run.sh,run.bat,src,work}）
```

注：package.sh 目标路径固定写 `sh/leanote.tar.gz`；本次在容器内 `/work/sh` 执行，
因此工作区未产生 tarball 残留。

## 最终结果

第三次会话完整跑通，容器 exit 0：`LINUX ENTRYPOINT VALIDATION: ALL PASS`
（全文日志：`research/linux-entrypoint-run3-all-pass.log`）。

## 遗留缺口（如实记录）

1. **stock CLI（`@v1.0.3` 安装）在 Go ≥1.26 下不可用**：属上游 Revel 冻结依赖问题，
   A 规格禁止升级 Revel 模块 ⇒ 无法也不应在 A 内修复。可执行的等价入口为模块图构建的
   CLI（B2b）或 harness 直启二进制（replay/record 全部走此路径）。建议随 C-a
   （Revel 1.1 升级）一并消除；若 CI 或文档需要 `revel run/package`，必须采用模块图构建。
2. `record-export-pdf` workflow 的真实 `workflow_dispatch` 实跑未执行：本地无 push 权限触发
   GitHub Actions。workflow diff 本身已完成（setup-go 1.20.14 → 1.26.7）；其实跑取证标记为
   pending-real-CI，见 implement.md Phase 5 对应条目。

## 工作区零残留核验

容器结束后：`app/tmp`、`app/routes`、`app/cmd/tmptmp`、`sh/leanote.tar.gz` 均不存在；
`git status --short app/tests/golden` 为空；`env down` 移除 fixture 容器；
辅助 docker volume（go module/build cache）已删除。
