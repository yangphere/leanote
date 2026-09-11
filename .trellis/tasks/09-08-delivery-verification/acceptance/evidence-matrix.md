# `09-08-delivery-verification` 身份链路汇合矩阵

| 阶段 | 必须消费的前置证据 | 通过条件 | 缺失处理 |
| --- | --- | --- | --- |
| persistence | `09-08-infrastructure-persistence/acceptance/evidence-matrix.md` AC-P2～P6 | transaction/补偿、outbox、TTL/index、token/唯一身份在 Mongo 7/8 有绑定 run artifact | `blocked` |
| identity | `09-08-application-identity/acceptance/evidence-matrix.md` AC-I1～I7、P-01/P-05/P-06/P-07/P-08 | principal、session/token、错误、安全、session rotation、action-token atomicity、logout/register failure 和 token digest contract 有实现证据 | `blocked` |
| interface | identity 方法矩阵、SessionWriter、ApiUser registry、HTTP replay | 405、`_ID` fallback/rotation、invalid token fail-closed、P-06/P-07 Cookie/envelope、query/form transport 和 Cookie 写回 replay 通过 | `blocked` |
| real convergence | Mongo/HTTP/mail/browser/container/PDF/release matrix | 所有真实环境 artifact 绑定同一 commit/run/attempt，未使用 mock 或历史 artifact | `blocked` |

任何 `partial`、`unknown`、失败或清理失败都不能被下游汇总为通过。
