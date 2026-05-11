# 07-PolicyVersion、Outbox 与 RuntimeReload

## 1. 本文定位

本文是 `03-授权AuthZ/` 文档组中关于 **授权版本、事件传播与运行时刷新** 的文档。

前面几篇文档已经解释了：

```text
00-AuthZ模型总览：Subject -> RoleBinding -> Role -> Permission -> Resource / Action / Scope
01-授权资源与动作模型：ResourceKey / ResourcePattern / Action / Scope
02-授权角色与绑定模型：Role / RoleBinding / Subject
03-Check与Snapshot读链路：Check / Snapshot
04-授权写入链路：PolicyAdministration 与 PolicyChange
05-PolicyChangeCommitter 与 AuthZ UoW
06-Casbin运行时模型：p/g Facts 与四段 Matcher
```

上一篇已经说明：PolicyChangeCommitter 会在事务内完成：

```text
AuthorizationFacts mutation
PolicyVersion increment
Outbox event staging
```

事务提交后再执行：

```text
RuntimeReload(best-effort)
```

本文进一步展开：

```text
PolicyVersion 是什么？
为什么授权变更需要版本号？
Outbox event 为什么必须和授权事实同事务？
RuntimeReload 为什么是事务后的 best-effort 动作？
多实例部署下如何让所有实例感知授权版本变化？
runtime health / reload lag 如何帮助排查授权一致性问题？
```

本文重点讲：

```text
PolicyVersion
Transactional Outbox
AuthZ version_changed event
RuntimeReload
RuntimeHealthDetails
reload lag
multi-instance reload
```

PolicyLinter 与授权事实治理会放到下一篇：

```text
08-PolicyLinter与授权事实治理.md
```

---

## 2. 30 秒结论

AuthZ 写入不是只改一条权限事实。

每次有效授权变更都应该带来：

```text
AuthorizationFacts changed
PolicyVersion incremented
version_changed event staged
Runtime policy reloaded eventually
```

核心链路是：

```text
PolicyChangeCommitter
  -> UoW Transaction
  -> mutate Permission / RoleBinding facts
  -> increment PolicyVersion
  -> stage authz.version_changed Outbox event
  -> commit transaction
  -> local RuntimeReload(best-effort)
  -> outbox relay publishes event
  -> other instances receive event
  -> RuntimeReload
  -> RuntimeHealth updated
```

其中：

| 概念 | 职责 |
| --- | --- |
| PolicyVersion | 表示某个 Tenant 下授权事实的版本 |
| Outbox | 保证事实变更与版本事件同事务落库 |
| version_changed event | 通知其他进程或系统授权版本已变化 |
| RuntimeReload | 让本实例重新加载最新授权 facts |
| RuntimeHealth | 暴露 runtime 是否追上最新授权版本 |

一句话：

> PolicyVersion 解决“授权事实现在是第几个版本”，Outbox 解决“事实和事件必须一致”，RuntimeReload 解决“运行时缓存什么时候看到新事实”。

---

## 3. 为什么 AuthZ 需要 PolicyVersion

授权事实会频繁变化：

```text
给 Role 添加 Permission
从 Role 撤销 Permission
给 Subject 绑定 Role
撤销 Subject 的 RoleBinding
未来 PolicyReconciler 修复脏 facts
```

这些变更都会影响：

```text
Check 判定结果
Snapshot 快照内容
SDK 缓存
业务服务本地权限视图
运行时 Casbin policy
```

如果没有版本号，系统很难回答：

```text
这次 Check 基于哪个授权状态？
这份 Snapshot 是否已经过期？
runtime policy 是否追上数据库事实？
某次拒绝发生在授权变更之前还是之后？
其他实例是否已经加载到最新策略？
```

因此 AuthZ 需要 PolicyVersion。

它是授权事实变化的版本锚点。

---

## 4. PolicyVersion 的模型语义

### 4.1 PolicyVersion 是什么

`PolicyVersion` 是某个 Tenant 下授权策略的版本号。

它回答：

```text
这个 Tenant 的授权事实已经变更到第几个版本？
```

例如：

```text
tenant-a policy version = 17
```

表示：

```text
tenant-a 当前授权事实处于第 17 个版本。
```

当一次授权写入成功后：

```text
version 17 -> version 18
```

---

### 4.2 为什么是 Tenant 维度

AuthZ 的授权事实是按 Tenant 隔离的。

例如：

```text
user:1001 在 tenant-a 下是 admin
user:1001 在 tenant-b 下是 viewer
```

因此授权版本也应该是 Tenant 维度。

也就是说：

```text
tenant-a 的权限变更不应该导致 tenant-b 的 version 变化
```

这样可以避免：

```text
无关租户缓存失效
Snapshot 刷新范围过大
runtime reload 追踪不清楚
```

---

### 4.3 PolicyVersion 进入哪些读模型

PolicyVersion 应该进入：

```text
CheckResponse
AuthorizationSnapshot
RuntimeHealthDetails
version_changed event
```

CheckResponse 中的版本号表示：

```text
本次判定基于哪个授权版本。
```

AuthorizationSnapshot 中的版本号表示：

```text
这份快照对应哪个授权版本。
```

RuntimeHealthDetails 中的版本号用于回答：

```text
runtime 是否已经加载到最新事件版本？
```

version_changed event 中的版本号用于通知：

```text
某个 Tenant 的授权事实已经变更到 version N。
```

---

## 5. PolicyVersion 的写入要求

### 5.1 必须与 facts 同事务

PolicyVersion 必须和授权 facts mutation 在同一个事务中完成。

否则会出现：

```text
facts 已变化，但 version 没有变化
version 已变化，但 facts 没有变化
version_changed event 携带的 version 与事实不一致
```

这些都会破坏读链路和缓存失效机制。

正确做法是：

```text
UoW Transaction
  -> mutate Permission / RoleBinding facts
  -> increment PolicyVersion
  -> stage version_changed event
  -> commit
```

---

### 5.2 必须并发安全

同一个 Tenant 下可能同时发生多个授权写入。

例如：

```text
管理员 A 绑定 Role
管理员 B 添加 Permission
管理员 C 撤销 Permission
```

这些操作都要递增同一个 Tenant 的 PolicyVersion。

因此 PolicyVersion 递增需要保证：

```text
单调递增
不重复
并发下不会丢版本
错误不会泄漏为脏事实
```

常见手段是：

```text
唯一索引
行级锁
retry
事务隔离
```

目标不是让版本号连续具备业务含义。

目标是：

```text
每次有效授权事实变化，都能产生一个可追踪的新版本。
```

---

### 5.3 version 是否必须连续

严格连续不是最重要的。

更重要的是：

```text
版本单调递增
同一 Tenant 下版本不重复
读链路能感知新旧
事件能携带准确 version
```

如果因为并发冲突和 retry 造成内部尝试失败，只要最终提交的版本满足单调性和唯一性即可。

业务系统不应该依赖：

```text
version 一定没有 gap
```

而应该依赖：

```text
version 越大越新
```

---

## 6. Transactional Outbox：事实与事件一致性

### 6.1 为什么不能事务提交后直接发消息

授权事实提交后，需要通知其他实例或系统：

```text
某个 Tenant 的授权策略已经变化
```

最直接的做法是：

```text
commit DB
publish message
```

但这有两个经典问题。

第一种失败：

```text
DB commit 成功
publish message 失败
```

结果是：

```text
数据库事实已变更
其他实例不知道要 reload
```

第二种失败：

```text
publish message 成功
DB commit 失败
```

结果是：

```text
其他实例收到事件并 reload
但数据库事实其实没有变
```

这两种状态都不可接受。

---

### 6.2 Outbox 解决什么

Transactional Outbox 的做法是：

```text
在同一个数据库事务中写业务事实和 outbox event
事务提交后，由 outbox relay 异步发布 event
```

也就是：

```text
UoW Transaction
  -> mutate authorization facts
  -> increment PolicyVersion
  -> insert outbox event
  -> commit

Outbox Relay
  -> poll unpublished events
  -> publish event
  -> mark event published
```

这样可以保证：

```text
只要事实提交成功，event 一定存在于 outbox 中
如果事实回滚，event 也回滚
消息发布失败可以重试
```

---

### 6.3 Outbox 不等于消息队列

Outbox 是数据库中的待发布事件表或事件记录。

消息队列是事件发布后的传输通道。

它们关系是：

```text
业务事务写 outbox
outbox relay 发布到 message broker
consumer 消费 message broker 中的事件
```

不要把 Outbox 和消息队列混为一谈。

Outbox 解决的是：

```text
数据库事实与待发布事件的一致性
```

消息队列解决的是：

```text
事件的异步传输和消费
```

---

## 7. AuthZ version_changed event

### 7.1 事件表达什么

AuthZ 写入成功后，会 stage 一个版本变更事件。

可以称为：

```text
authz.policy.version_changed
```

它表达：

```text
某个 Tenant 的授权事实已经变更到某个 PolicyVersion。
```

事件通常包含：

```text
tenant_id
policy_version
change_kind
occurred_at
```

可选包含：

```text
actor
reason
trace_id
```

---

### 7.2 事件不应该携带完整权限明细

version_changed event 不应该变成完整权限快照。

它不是：

```text
这里是所有 p/g facts
```

而是：

```text
某个 tenant 的授权版本变了，请重新读取或 reload。
```

原因是：

```text
权限明细可能很大
事件体不适合承载全量策略
事件重复消费需要幂等
消费者应以数据库 facts 为事实源
```

因此 version_changed event 是一种：

```text
cache invalidation signal
runtime reload signal
```

而不是授权事实本身。

---

### 7.3 change_kind 的作用

`change_kind` 可以帮助排查：

```text
grant_permission
revoke_permission
bind_role
unbind_role
reconcile_policy
```

它不是 matcher 必需字段。

它主要用于：

```text
日志追踪
审计分析
告警上下文
运维排查
```

真正决定 runtime reload 的关键字段仍然是：

```text
tenant_id
policy_version
```

---

## 8. RuntimeReload：让运行时看见新 facts

### 8.1 Runtime policy 是什么

AuthZ 的事实源在数据库中。

运行时判定为了效率，会将 p/g facts 加载到 runtime engine 中。

当前运行时 engine 是 Casbin。

因此 runtime policy 可以理解为：

```text
当前进程内 Casbin enforcer 持有的 p/g facts 视图
```

Check 链路直接消费的是 runtime policy。

如果数据库 facts 更新了，但 runtime policy 没刷新，就会出现：

```text
数据库已经授权
Check 仍然拒绝
```

或：

```text
数据库已经撤权
Check 仍然允许
```

因此授权写入后必须考虑 RuntimeReload。

---

### 8.2 本实例 best-effort reload

PolicyChangeCommitter 在事务提交后，会触发本实例 RuntimeReload。

它的目标是：

```text
写入发生在哪个实例，就尽快让这个实例看到最新策略。
```

为什么叫 best-effort？

因为 reload 发生在事务提交之后。

如果 reload 失败，不能回滚数据库事实。

只能：

```text
记录失败
标记 degraded
等待重试或事件驱动 reload
```

---

### 8.3 为什么不在事务内 reload

不能在数据库事务内执行 RuntimeReload。

原因包括：

```text
reload 是进程内缓存刷新，不是数据库写入
reload 可能很慢，会拉长事务时间
reload 失败不应该导致事实回滚
reload 在多实例场景下本来无法由单个事务覆盖所有实例
```

正确边界是：

```text
事务内保证数据库事实一致
事务后尽力刷新当前实例 runtime
跨实例依赖 outbox event 驱动 reload
```

---

## 9. 多实例 RuntimeReload 闭环

### 9.1 单实例与多实例的差异

单实例部署下，事务后本地 reload 基本可以满足需求。

但多实例部署下，问题会变复杂。

例如：

```text
apiserver-A 处理授权写入
apiserver-A 本地 reload 成功
apiserver-B 没有收到任何信号
用户请求打到 apiserver-B
apiserver-B 仍使用旧 runtime policy
```

这就是多实例 runtime policy 不一致。

---

### 9.2 多实例闭环目标

多实例下目标链路应该是：

```text
PolicyChangeCommitter
  -> stage version_changed outbox event
  -> commit transaction
  -> local reload

Outbox Relay
  -> publish version_changed event

Each apiserver instance
  -> consume version_changed event
  -> RecordPolicyVersionEvent
  -> LoadPolicy
  -> update RuntimeHealthDetails
```

这样每个实例都能感知：

```text
tenant 的授权版本发生变化
自己需要 reload runtime policy
```

---

### 9.3 消费端需要幂等

version_changed event 可能重复投递。

消费者必须幂等。

常见规则：

```text
如果 event.version <= last_event_version，可以忽略或记录重复事件
如果 event.version > last_event_version，记录事件并尝试 reload
reload 成功后更新 last_reload_version / reloaded_at
reload 失败则记录 degraded
```

这样可以避免：

```text
重复 reload 造成过多压力
乱序事件导致 runtime version 回退
重复事件污染 health 状态
```

---

### 9.4 是否要按 Tenant 局部 reload

理想情况下，某个 Tenant 的策略变更只需要 reload 该 Tenant 的 facts。

但实际实现要看 runtime engine 是否支持局部刷新。

如果不支持，可能只能：

```text
LoadPolicy 全量刷新
```

这会带来性能成本。

因此可以分阶段：

```text
第一阶段：全量 LoadPolicy，保证正确性
第二阶段：评估 tenant-level incremental reload
第三阶段：必要时引入分片 runtime 或读模型缓存
```

不要在模型尚未稳定时过早做局部 reload 优化。

---

## 10. RuntimeHealthDetails

### 10.1 为什么需要 RuntimeHealth

RuntimeReload 是最终一致动作。

它可能失败。

多实例下也可能出现某些实例 reload 落后。

因此需要 RuntimeHealthDetails 暴露：

```text
runtime 当前是否健康
最后一次收到的 policy version event 是多少
最后一次成功 reload 的版本是多少
最后一次 event 时间
最后一次 reload 时间
reload lag 是多少
最近一次错误是什么
```

否则授权问题很难排查。

---

### 10.2 RuntimeHealth 应该回答的问题

RuntimeHealth 应该帮助回答：

```text
这个实例是否加载了最新授权策略？
这个实例是否收到过 version_changed event？
最近一次 reload 是否失败？
reload 落后多久？
当前 Check 结果是否可能基于旧策略？
```

这些信息可以进入：

```text
health endpoint
metrics
structured logs
admin diagnostics
```

---

### 10.3 degraded 状态

当 reload 失败时，runtime 不应该假装健康。

它应该进入：

```text
degraded
```

degraded 的含义是：

```text
数据库事实可能已经更新，但当前 runtime policy 可能未追上最新版本。
```

degraded 不一定意味着服务完全不可用。

但它意味着：

```text
授权判定可能存在短暂或持续滞后
需要告警或自动重试
```

---

## 11. reload lag

### 11.1 reload lag 是什么

reload lag 可以理解为：

```text
当前实例 runtime reload 落后于最新授权事件的时间或版本差。
```

常见指标包括：

```text
last_event_version
last_reload_version
last_event_at
last_reload_at
reload_lag_ms
```

如果：

```text
last_event_version > last_reload_version
```

说明 runtime 还没有追上最新事件。

如果：

```text
reload_lag_ms 持续很高
```

说明 reload 可能失败或处理过慢。

---

### 11.2 reload lag 如何用于排查

当用户反馈：

```text
我已经授予权限，但接口仍然 403
```

可以检查：

```text
该 tenant 的 PolicyVersion 是否已经递增
Outbox event 是否已发布
目标实例是否收到 event
目标实例 last_reload_version 是否追上 last_event_version
reload 是否 degraded
```

这比单纯查看数据库表更有效。

因为问题可能不在事实写入，而在 runtime reload。

---

## 12. Check / Snapshot 与 RuntimeReload 的关系

Check 和 Snapshot 读取的是 runtime 或读模型中的授权事实。

如果 runtime policy 没刷新，就可能出现：

```text
Check 结果滞后
Snapshot 结果滞后
```

但 Check / Snapshot 本身不应该主动执行 reload。

原因是：

```text
读请求触发 reload 会导致延迟不可控
高并发下可能引发 reload 风暴
读链路职责会被污染
```

更好的方式是：

```text
写后本地 best-effort reload
outbox-driven reload
health / metrics 监控 reload 状态
必要时后台定时 reconcile reload
```

---

## 13. 与 PolicyChangeCommitter 的关系

PolicyChangeCommitter 是版本和事件的写入入口。

它负责：

```text
在事务内递增 PolicyVersion
在事务内 stage version_changed outbox event
事务提交后触发 local RuntimeReload
```

但它不负责：

```text
发布 outbox event
保证所有实例 reload 成功
长期监控 reload lag
自动修复 degraded runtime
```

这些属于：

```text
Outbox relay
Event consumer
RuntimeReload handler
Observability / health system
```

因此要明确：

```text
PolicyChangeCommitter 保证本次写入产生版本和事件。
Outbox / consumer / health 保证事件传播和 runtime 最终追上。
```

---

## 14. 与 PolicyLinter 的关系

PolicyLinter 是授权事实治理工具。

它通常读取数据库中的 Permission facts，并与 ResourceCatalog 对比。

PolicyLinter 不依赖 runtime policy 是否已 reload。

它关注的是：

```text
事实本身是否合理
```

RuntimeReload 关注的是：

```text
运行时是否加载了事实
```

二者解决不同问题。

例如：

```text
Permission fact 引用了不存在的 Resource
```

这是 PolicyLinter 问题。

```text
Permission fact 已经存在，但 Check 仍然拒绝
```

可能是 RuntimeReload 问题。

---

## 15. 生产化建议

### 15.1 必须有 Outbox relay

如果只有 outbox table，但没有 relay，事件不会被发布。

结果是：

```text
写入实例可能通过 local reload 看到新策略
其他实例永远不知道版本变化
```

因此生产环境必须有：

```text
Outbox relay
```

负责发布 `authz.policy.version_changed` 事件。

---

### 15.2 必须有 RuntimeReload consumer

如果事件发布了，但没有 consumer，其他实例仍然不会 reload。

因此每个 apiserver 实例或授权 runtime 节点都应该具备：

```text
AuthZ version_changed event consumer
RuntimePolicyReloadHandler
```

消费事件后执行：

```text
RecordPolicyVersionEvent
LoadPolicy
Update RuntimeHealthDetails
```

---

### 15.3 必须有 metrics / health

至少要暴露：

```text
last_event_version
last_reload_version
last_event_at
last_reload_at
reload_lag_ms
reload_error_count
runtime_degraded
```

这样才能排查：

```text
授权写入成功但判定未生效
某实例策略长期落后
某 tenant reload 频繁失败
```

---

### 15.4 可以有定时兜底 reload

即使有事件驱动 reload，也可以保留低频定时兜底。

例如：

```text
每隔 N 分钟检查是否有未追上的 version
或定期全量 LoadPolicy
```

这不是主链路。

这是防御性兜底。

---

## 16. 常见误区

### 16.1 PolicyVersion 是全局版本

错误。

PolicyVersion 应该按 Tenant 维度管理。

某个 tenant 的授权变化不应让所有 tenant 缓存失效。

---

### 16.2 只要 DB 写成功，Check 就一定马上生效

错误。

Check 依赖 runtime policy。

DB 写成功后还需要 RuntimeReload。

---

### 16.3 RuntimeReload 失败应该回滚 DB

错误。

RuntimeReload 在事务提交后执行。

失败时应该标记 degraded，并通过重试或事件驱动 reload 修复。

---

### 16.4 直接发消息就等于 Outbox

错误。

Outbox 是事务内写入的事件记录。

直接发消息不能保证与数据库事实一致。

---

### 16.5 version_changed event 要包含所有权限明细

错误。

它是版本变更信号，不是权限快照。

消费者应该根据版本信号重新读取 facts 或 reload runtime。

---

### 16.6 多实例只靠写入实例 local reload 就够了

错误。

其他实例不会自动知道授权事实变化。

必须依赖 Outbox event 和 consumer。

---

### 16.7 Check 请求中发现 version 落后时应该自动 reload

不推荐。

读请求触发 reload 容易造成延迟抖动和 reload 风暴。

应通过写后 reload、事件驱动 reload、健康检查和后台兜底解决。

---

## 17. 代码事实源

本文涉及的主要代码事实源：

```text
internal/apiserver/domain/authz/policy
internal/apiserver/application/authz/policy
internal/apiserver/application/authz/uow
internal/apiserver/infra/mysql/policy
internal/apiserver/infra/mysql/casbinrule
internal/apiserver/infra/casbin
internal/apiserver/shared/authz

# event / outbox 相关事实源
internal/apiserver/infra/mysql/outbox
internal/apiserver/application/events
internal/apiserver/infra/events
```

重点关注：

| 主题 | 事实源 |
| --- | --- |
| PolicyVersion | `domain/authz/policy`、`infra/mysql/policy` |
| PolicyChangeCommitter | `application/authz/policy` |
| AuthZ UoW | `application/authz/uow`、`infra/mysql/uow` |
| Outbox event staging | outbox / event stager 相关实现 |
| RuntimePolicyReloader | `infra/casbin` |
| RuntimeHealthDetails | `infra/casbin` |
| Reload helper | `shared/authz` |
| CheckResponse policy_version | `application/authz/authorization`、`transport/rest/authz`、`transport/grpc/service/authz` |
| Snapshot policy_version | `application/authz/authorization` |

如果本文与代码不一致，以代码事实源为准。

---

## 18. 本文总结

本文讲的是 AuthZ 的版本传播和 runtime 刷新机制。

核心链路是：

```text
PolicyChangeCommitter
  -> mutate facts
  -> increment PolicyVersion
  -> stage version_changed Outbox event
  -> commit transaction
  -> local RuntimeReload(best-effort)
  -> Outbox relay publish event
  -> other instances consume event
  -> RuntimeReload
  -> RuntimeHealth updated
```

其中：

```text
PolicyVersion 负责版本感知
Outbox 负责事实与事件同事务
version_changed event 负责传播缓存失效信号
RuntimeReload 负责让 runtime policy 追上数据库事实
RuntimeHealth / reload lag 负责生产排查
```

如果只记住一句话：

> PolicyVersion 告诉系统“授权事实变到哪个版本”，Outbox 保证“事实和事件一起提交”，RuntimeReload 保证“运行时最终看见新事实”；单实例靠本地 best-effort reload，多实例必须靠 outbox-driven reload 闭环。
