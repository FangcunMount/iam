# 事件目录、发布语义与 Transactional Outbox

> 状态：已实现 · 已与 event catalog、publisher、MySQL outbox、relay、AuthZ policy version 链路和测试核对。

## 1. 本文回答

- IAM 为什么区分 `best_effort` 与 `durable_outbox` 事件？
- Outbox 真正保证了什么，又没有保证什么？
- AuthZ 多实例策略同步的完整链路是什么？
- MQ 不可用、发布成功但标记失败、消费者重复消费时会怎样？
- 为什么不在数据库事务中直接 publish？

## 2. 30 秒结论

IAM 的事件系统不是“所有事件都发 MQ”，而是由 `configs/events.yaml` 为每个事件声明 topic、delivery class、aggregate、domain 和 handler。

```text
best_effort
  -> application publisher
  -> MQ 可用则发布；无 MQ 时只记录日志

durable_outbox
  -> 必须在 MySQL 业务事务中 Stage
  -> domain_event_outbox
  -> relay claim/publish/mark
  -> MQ subscriber
```

当前 `iam.authz.version_changed` 使用 durable outbox；`iam.login_otp_sms` 使用 best effort。代码会拒绝把 durable event 直接交给普通
publisher，也会拒绝把 best-effort event 写入 outbox，从机制上避免调用者选错通道。

## 3. 为什么需要事件目录

没有目录时，topic 名、payload、投递级别和 owner 容易散落在代码中。`eventcatalog.Catalog` 把以下事实集中为机器可读配置：

- event type 到 topic 的映射；
- delivery 是 `durable_outbox` 还是 `best_effort`；
- aggregate 类型和负责 handler；
- topic 实际名称。

启动时 `platform.InitEventing` 加载目录。目录缺失或事件引用不存在 topic 会初始化失败；`docs-facts` 也会静态校验配置内部引用。

事件目录不是 schema registry：当前 payload 仍由 Go struct/map + JSON codec 定义，没有独立版本化 schema。修改 payload 时需要同时检查 producer、consumer、
兼容窗口和重放数据。

## 4. 为什么数据库事务里不能直接发 MQ

下面两个顺序都不能同时保证数据库和 MQ：

### 4.1 先提交数据库，再发消息

```text
DB commit 成功
进程在 publish 前崩溃
=> 业务事实存在，事件永久丢失
```

### 4.2 先发消息，再提交数据库

```text
publish 成功
DB commit 失败
=> 消费者看到了一个从未成立的业务事实
```

Transactional Outbox 把“需要发布”也写成同一个数据库事务中的事实：

```text
BEGIN
  write business facts
  increment policy version
  insert domain_event_outbox
COMMIT
```

提交后 relay 可以在任意时间恢复发布，不要求业务请求继续存活。

## 5. Outbox 写入链路

`eventoutbox.Store.Stage` 不自行开事务，而是要求 `dbmysql.RequireTx(ctx)` 成功。这个 fail-fast 约束避免调用者在普通 DB handle 上写出“伪 Outbox”。

`outboxcore.BuildRecords`：

1. 从 catalog 解析 topic；
2. 检查 delivery 必须是 `durable_outbox`；
3. 编码 payload；
4. 生成 `pending` row，`next_attempt_at=now`；
5. 使用 event ID 唯一索引防止同一事件重复入箱。

Outbox row 保存 event/aggregate/topic 元数据和 JSON payload，但不会替代业务表。

## 6. Relay 的并发与状态机

状态：

```text
pending ─claim─> publishing ─publish success─> published
                         └─publish failed──> failed ─retry─┐
publishing 超过 stale window ──────────────────────────────┘
```

### 6.1 Claim

`ClaimDueEvents` 在 MySQL transaction 中选择：

- 到期 `pending`；
- 到期 `failed`；
- 超过 stale window 的 `publishing`。

非 SQLite 使用 `FOR UPDATE SKIP LOCKED`，多个 relay 实例可以并行 claim 不同批次。选中后统一改为 `publishing`。

### 6.2 Publish

Relay 为消息设置：

- message ID = event ID；
- metadata: `event_type`、`aggregate_type`、`aggregate_id`、`source`；
- topic = Outbox row 中解析后的 topic。

### 6.3 Mark

- publish error：`failed`，attempt +1，写 `last_error` 和下次重试时间；
- publish success：`published` 和 `published_at`；
- mark 自身失败：保留可恢复状态并记录错误。

## 7. 它为什么不是 exactly-once

最典型窗口：MQ 已接收消息，但 relay 在 `MarkEventPublished` 前崩溃。row 仍是 `publishing`，超过 stale window 后会再次发布。

因此当前保证应表述为：

- 业务提交与发布意图原子；
- relay 可恢复、可重试；
- 投递倾向 at-least-once；
- 不保证消费者只收到一次；
- 消费者必须以 event ID 或业务版本实现幂等/去重。

声称 exactly-once 会掩盖真实的重复窗口。

## 8. AuthZ 多实例策略同步

完整链路：

```text
AuthZ command service
  -> MySQL tx:
       write role/resource/assignment/inheritance/permission_grant
       increment policy_versions
       Stage iam.authz.version_changed
  -> commit
  -> 当前实例 Reload immutable-snapshot runtime

Outbox relay
  -> topic iam.authz.version
  -> 每个实例使用独立 ephemeral channel 订阅
  -> PolicyPublication.Service decode tenant/version
  -> record observed version event
  -> Build immutable snapshot from DB
```

当前实例在提交后直接 reload，降低本请求之后的本地陈旧窗口；其他实例依靠 durable event reload。

Subscriber channel 包含 hostname + pid + `#ephemeral`，目的是广播到每个实例，而不是让实例组成只消费一次的工作队列。否则只有一个实例 reload，其余实例会继续使用旧快照。

## 9. AuthZ authorization runtime 与事件的关系

数据库中的 Assignment、RoleInheritance、PermissionGrant 和 Resource Schema 是授权事实源，不可变 runtime snapshot 是进程内判定快照。事件不是授权事实，
只是“请重新加载”的协调信号。

这带来两个重要结论：

- 消息重复只会触发重复 reload，业务上应是幂等的；
- 消息暂时丢失/延迟时，实例会陈旧，因此需要 runtime health、版本观测和重试；不能把 MQ 当真相源。

## 10. Best effort 的边界

`eventruntime.RoutingPublisher` 在有 EventBus 时直接发布；没有 EventBus 时进入 logging mode。适合允许降级的通知/集成信号，不适合决定业务事实是否成立。

当前 SMS MQ 发送可选择 publisher。若业务要求“短信任务绝不因进程崩溃丢失”，它就不再是 best effort 问题，需要改为 durable job/outbox 并定义去重、过期和补偿；不能只换一个 topic 名称。

## 11. 故障语义

| 故障 | 当前行为 | 数据是否可恢复 |
| --- | --- | --- |
| DB transaction 回滚 | 业务事实和 Outbox 一起回滚 | 不需要发布 |
| EventBus 启动不可用 | Outbox store 仍可写，relay 不启动 | 可在 MQ 恢复并重启/恢复 relay 后发布 |
| relay claim 后崩溃 | row 留在 publishing | stale window 后重新 claim |
| publish 失败 | row 变 failed，延迟重试 | 可恢复 |
| publish 成功、mark 失败 | 可能重复发布 | 消费者必须幂等 |
| consumer reload 失败 | handler 返回 error；具体重投依赖消息组件 | DB 仍是事实源，可人工/进程重启 reload |

## 12. 为什么不采用其他方案

### 12.1 数据库事务内直接 MQ publish

未采用。数据库和 MQ 没有共同原子提交，存在不可关闭的双写窗口。

### 12.2 XA/两阶段提交

未采用。基础设施支持、延迟和可用性成本高，且不能消除消费者幂等需求。

### 12.3 CDC 直接监听业务表

可选但当前未采用。CDC 能减少应用 Outbox relay，但需要额外平台、schema 演进和事件意图推导；Outbox 显式记录业务事件，当前规模更可控。

### 12.4 所有事件都 durable

未采用。低价值通知也进入 Outbox 会扩大表、重试和运维成本。delivery class 让业务按丢失代价选择。

### 12.5 所有事件都 best effort

未采用。AuthZ 多实例策略传播若永久丢失，会造成授权判定长期漂移，风险不可接受。

## 13. 当前代价与风险

- 仍需消费者幂等，Outbox 不能提供端到端 exactly-once；
- 当前 payload 没有独立 schema/version registry；
- 一般 Outbox 的 backlog 尚未作为 `/readyz` required component，主要通过状态快照/日志观察；
- relay 是进程内周期任务，进程全部停止时不会发布；
- EventBus 无法初始化时启动会降级记录，但 release 运维必须能看到 relay 未运行；
- `last_error` 保存外部错误文本，日志/数据治理需避免敏感信息进入错误消息。

## 14. 证据入口

| 关注点 | 代码/配置 |
| --- | --- |
| event types | `internal/apiserver/eventing/eventing.go` |
| catalog | `configs/events.yaml`、`pkg/eventcatalog` |
| direct publisher | `pkg/eventruntime/publisher.go` |
| Outbox record/state | `pkg/outboxcore/core.go` |
| MySQL store | `internal/apiserver/infra/mysql/eventoutbox/store.go` |
| relay | `internal/apiserver/infra/messaging/outbox_relay.go` |
| composition | `internal/apiserver/container/platform/eventing.go` |
| AuthZ command services | `internal/apiserver/application/authz` |
| AuthZ consumer | `internal/apiserver/application/authz/policypublication/service.go` |

## 15. Verify

```bash
make docs-facts
go test ./pkg/eventcatalog ./pkg/eventcodec ./pkg/eventmessaging ./pkg/eventruntime ./pkg/outboxcore
go test ./internal/apiserver/infra/mysql/eventoutbox ./internal/apiserver/infra/messaging
go test ./internal/apiserver/application/authz/... ./internal/apiserver/infra/authz/runtime/...
```

## 16. 面试追问

**Outbox 解决的是消息重复还是消息丢失？**

核心解决“业务提交成功但发布意图丢失”的双写问题。它通过重试引入/接受重复，重复要由消费者幂等处理。

**为什么 AuthZ 订阅要每实例独立 channel？**

策略快照存在每个进程内；一次策略变更必须让每个实例都 reload。共享竞争消费 channel 只会通知其中一个实例。

**DB 才是真相源，为什么还需要 policy version？**

版本提供变化顺序和可观测性，便于判断运行时是否看到了最新事实；它不替代 PermissionGrant 等管理事实，也不直接完成授权判定。
