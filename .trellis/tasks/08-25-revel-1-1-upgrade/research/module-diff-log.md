# C-a Task 2：模块 diff 逐条归因（v1.0 → v1.1，2026-08-28）

对照：`module-snapshot-v1.0.txt` vs `module-snapshot-v1.1.txt`（`go list -m all` 全量）。
tidy 连续两次 go.mod/go.sum 零 diff；`go mod verify` all modules verified；无 replace/exclude/vendor。

## 目标三件套（C-a 拥有）

| 模块 | v1.0 | v1.1 |
|---|---|---|
| github.com/revel/revel | v1.0.0 | **v1.1.0** |
| github.com/revel/cmd | v1.0.3 | **v1.1.2** |
| github.com/revel/modules | v1.0.0 | **v1.1.0** |
| github.com/revel/config | v1.0.0 | **v1.1.0**（cmd v1.1.2 图经 MVS 抬升，与预测一致） |

不变：revel/log15 v2.11.20+incompatible、revel/pathtree 2014 伪版本、revel/cron v0.21.0（图内既有）。

## A 拥有的直接依赖（要求零变化，实测零变化）

golang.org/x/tools v0.49.0、github.com/jessevdk/go-flags v1.6.1、golang.org/x/crypto v0.55.0、
github.com/PuerkitoBio/goquery v1.12.0、robfig/config 2014 伪版本、agtorre/gocolorize v1.0.0 —— 全部保持。

## Revel 图传递要求的 MVS 抬升（每条归因）

| 模块 | 变化 | 归因 |
|---|---|---|
| github.com/gomodule/redigo | 新增 v1.8.8 | revel v1.1.0 直接要求（v1.0 用 garyburd/redigo；garyburd 经 tools.go 钉住仍在图内，两者共存） |
| github.com/google/uuid | 新增 v1.3.0 | revel v1.1.0 直接要求 |
| github.com/bradfitz/gomemcache | 2019→2022 伪版本 | revel v1.1.0 要求新伪版本 |
| github.com/fsnotify/fsnotify | v1.4.9→v1.5.1 | revel v1.1.0 要求 |
| github.com/go-stack/stack | v1.8.0→v1.8.1 | revel v1.1.0 要求 |
| github.com/mattn/go-colorable / go-isatty | v0.1.8→v0.1.12 / v0.0.12→v0.0.14 | revel v1.1.0 / cmd v1.1.2 要求 |
| github.com/BurntSushi/toml | v0.3.1→v1.1.0 | revel v1.1.0 indirect 要求 |
| github.com/stretchr/testify | v1.4.0→v1.7.1；stretchr/objx 移除 | revel/cmd v1.1.2 要求 v1.7.0+（testify 簇 MVS 抬升；objx 随旧 testify 图退出） |
| gopkg.in/yaml.v2 v2.2.2→v2.4.0；yaml.v3 新增 | testify 簇传递 |
| golang.org/x/xerrors | 2019→2020 伪版本 | 上述簇的传递要求 |
| valyala/fasthttp v1.12.0→v1.34.0、tcplisten →v1.0.0、klauspost/compress v1.10.4→v1.15.0、andybalholm/brotli 新增 | fasthttp 簇随 Revel 族图的更高要求整体抬升（fasthttp 系 v1.0 代图内既有） |

无未知模块、无第二套框架/日志/配置实现、无法归因条目：无。
`tools.go` 未改动（gomemcache/garyburd-redigo/go-cache/revel-modules-static 钉住保持）。
