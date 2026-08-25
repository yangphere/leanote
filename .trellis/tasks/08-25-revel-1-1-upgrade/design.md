# Revel 1.1 基线升级（C-a）— 技术设计

## 1. 策略

这是严格的依赖版本阶段，不建立兼容 shim，也不改变 controller API。先升级 runtime，再升级 CLI/生成器；所有必要适配限制在编译错误直接指向的位置。

## 2. 三条运行路径

| 路径 | 入口 | 验证 |
|---|---|---|
| 测试二进制 | `app/cmd` 生成器 + `go build github.com/leanote/leanote/app/tmp` | G 的 HTTP harness 与 Golden |
| 开发 | `revel run -a .` / `sh/run.sh` | 隔离端口真实请求 smoke |
| 生产包 | `revel package --run-mode=prod` / `sh/package.sh` | 解包、启动、访问 `/login` 与 `/api/auth/login` |

三条路径必须同时通过，避免只证明 runtime 可编译却遗漏生成器或打包器。

## 3. 不变量

- 启动顺序仍为 `db.Init` → 邮件/验证初始化 → services → controller service aliases → global config → API service。
- `conf/routes` 优先级、catch-all 与 `RouterFilter` 改写保持原样。
- Session Cookie 仍由 Revel 处理；一次性重新登录决策只适用于 C-b。
- `results.pretty=false` 的测试模式继续作为字节级契约环境。

## 4. 回滚

该阶段应形成单一可回滚版本提交。若 v1.1 产生无法用必要兼容适配解决的行为变化，回退到 v1.0 并在 C-b 设计中记录，不通过改 Golden 强行接受。
