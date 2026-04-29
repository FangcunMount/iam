# IAM Shared Packages

`pkg` 下的事务与事件包是 IAM 当前可跨项目复用的基础能力，但这一组包仍属于 experimental shared packages。后续 qs-server 可以先按这里的边界接入和验证，但不要把这些 API 视为已经冻结的跨仓稳定契约。

| Package | 用途 |
| ---- | ---- |
| `pkg/uow/gorm` | GORM UoW、tx context、Required 传播、`AfterCommit` |
| `pkg/event` | 通用领域事件接口、事件基类和泛型事件 |
| `pkg/eventcatalog` | event type、topic、delivery class 的 YAML catalog parser，支持可选 validation policy |
| `pkg/eventcodec` | legacy payload 编码和事件 metadata 构造，不依赖具体消息中间件 |
| `pkg/eventmessaging` | component-base `messaging.Message` adapter |
| `pkg/eventruntime` | catalog-backed component-base publisher；阻止 durable outbox 事件直发 |
| `pkg/outbox` | outbox relay store/status port |
| `pkg/outboxcore` | outbox record 构造、状态常量、状态快照 |

不在本轮提取的内容：

- IAM MySQL `BaseRepository`、`AuditFields`、`Syncable` 仍依赖 IAM `meta.ID` 和身份上下文，暂不作为跨项目公共能力。
- IAM 事件常量和 `configs/events.yaml` 是 IAM 业务语义，qs-server 应维护自己的事件 catalog。
- 具体 MySQL outbox store 仍在 `internal/apiserver/infra/mysql/eventoutbox`，因为表名、迁移和装配生命周期仍属 IAM 运行时。
