# 交付层：集成验证与版本发布 — 技术设计

## Boundaries

交付层消费各子任务的稳定 artifact 和测试入口，负责跨层矩阵、质量门、容器/PDF、浏览器和发布；不修改业务规则。

## Evidence flow

commit → quality-gate（Go/Mongo/Node/Chromium/package/container）→ 受保护真实浏览器 8 槽 artifact → release validator → tarball/SHA-256/GHCR。每一步绑定 commit、run/attempt 和脱敏摘要。

## Invariants

失败、未运行、清理失败、缺 artifact、跨 run 或 digest 不一致都阻断；不使用模拟 Safari、旧 artifact、`latest` 或自动部署。

## Rollback

发布校验失败时不创建 Release/GHCR；质量门或 browser artifact 问题只重跑对应阶段，不修改已通过的业务层提交。
