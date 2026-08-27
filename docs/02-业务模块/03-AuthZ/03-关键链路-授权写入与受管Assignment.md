# 关键链路：授权写入与受管 Assignment

> 状态：已实现 · 本文覆盖当前全部 AuthZ 写模型、事务边界和已知并发边界。

## 结论

AuthZ 写入遵循同一条提交链：校验领域不变量，在 Unit of Work 内写业务事实、递增策略版本并写 Outbox，提交后 reload 当前实例。Assignment 同时提供增量 Grant/Revoke
与“只替换受管集合”的批量接口。

一次“写成功”要分成三个时刻理解：

1. **事实已提交**：MySQL 中事实、PolicyVersion 与 Outbox 已原子落库。
2. **当前实例已切换**：提交后 reload 成功，该进程使用新快照。
3. **全部实例已收敛**：其他进程收到版本事件并完成 reload。

命令成功返回可以直接证明第 1 层，不能单独证明第 3 层。

## 通用写事务

```text
校验调用身份与输入
  -> 加载并校验领域事实
  -> BEGIN Unit of Work
  -> 写 Role / Assignment / Inheritance / Grant / Resource
  -> bump authz_policy_versions
  -> append policy-version Outbox event
  -> COMMIT
  -> reload 当前实例（失败重试）
  -> 其他实例消费事件后 reload
```

事实写入、版本递增和 Outbox 入队必须同事务提交。若事务回滚，三者都不能残留。

### 为什么 PolicyVersion 和 Outbox 必须同事务

若事实先提交、版本后写，版本写失败会让其他实例永远不知道需要 reload。若事件先发布、事实后提交，消费方可能提前建立旧快照并不再重试。

当前的事务内 Outbox 设计把“该 Tenant 有新版本”变成可靠事实；事务外 relay 失败只会延迟通知，不会丢失通知意图。

### 为什么 reload 不在事务内

reload 需要重读完整 AuthZ 数据集并构建快照。将它放在写事务内会延长锁持有时间，并且新事实在提交前对独立读连接不可见。所以当前边界是“先提交，后重建”，并用健康和版本滞后暴露窗口。

## 写操作分类

| 操作 | 核心不变量 |
| --- | --- |
| Create/Update/Delete Role | Tenant 边界、被引用关系与版本一致性 |
| Grant/Revoke Assignment | Subject→Role 直接关系；约束授权器决定可管理范围 |
| ReplaceManagedAssignments | 只替换约束策略定义的受管角色集合 |
| Grant/Revoke Permission | Resource/Action/ConstraintSet 合法 |
| Grant/Revoke Inheritance | 同 Tenant、禁止自继承与成环 |
| Register/Update Resource | attribute schema 可校验、可供运行时求值 |

### 删除是否安全由引用决定

- Role 有活跃 Assignment、PermissionGrant 或 RoleInheritance 引用时不能删除。
- Resource 有活跃 Grant 时不能删除。
- Resource schema/action 变更要重新校验依赖 Grant，不能把已有策略留在无效状态。
- 继承边与 Grant 使用 revoke 保留历史，Assignment 使用软删除并依靠 active guard 允许重新授予。

这些约束的目的不是阻止清理，而是保证每个可见事实都能被 reload 解释。

## 增量 Assignment

`GrantAssignment` 增加一条直接 Assignment；`RevokeAssignment` 删除一条直接 Assignment。它们不是角色继承操作，也不接受 effective roles 作为输入。

调用方必须提交目标 Tenant、Subject 和 Role。服务间调用还会经过 Assignment 约束授权器，限制调用方可以管理哪些角色。

增量写入的具体防线为：

```text
gRPC service identity + method ACL
  -> assignment constraints(caller, operation, domain, subject, role, delegated actor)
  -> parse subject / role name / actor
  -> transaction-local subject and role validation
  -> lock role row
  -> create or soft-delete Assignment
  -> database active uniqueness guard
  -> increment PolicyVersion + stage Outbox
```

方法 ACL 只能回答“该服务能否调用 GrantAssignment”，不能回答“它能否在任意 Tenant 给任意 Subject 授任意 Role”。Assignment constraints 是内容级的第二道授权，
两者不可互相替代。

## `ReplaceManagedAssignments`

该 RPC 用于把一个 Subject 在某个 Tenant 下的“受管 Assignment 集合”替换为目标集合。

算法语义：

1. 约束授权器根据调用服务、Tenant 和候选角色给出可管理的 Role 集合。
2. 目标 `role_ids` 必须全部属于该受管集合。
3. 读取 Subject 当前直接 Assignment。
4. 删除受管集合中但不在目标集合内的 Assignment。
5. 新增目标集合中当前尚不存在的 Assignment。
6. 受管集合之外的 Assignment 原样保留。
7. 目标集合为空表示清空全部受管 Assignment，不影响非受管 Assignment。

示例：

```text
当前直接角色: [tenant_admin, class_teacher, class_observer]
受管集合:     [class_teacher, class_observer]
目标集合:     [class_observer]

提交后实际直接角色: [tenant_admin, class_observer]
当前响应 direct_roles: [class_observer]
```

这里的响应 `direct_roles` 当前表示“目标受管角色子集”，不是提交后数据库中的全部直接角色。需要完整视图时，应重新调用 `GetAuthorizationSnapshot`。

### “受管集合”与“目标集合”

批量替换包含两个不同集合：

```text
M = constraints 认定调用方可管理的全部角色
T = 本次请求期望保留的目标角色

必须 T ⊆ M
实际新直接角色 = (当前直接角色 - M) ∪ T
```

所以空 `T` 是合法请求，表示清空所有受管 Assignment；空 `M` 则不合法，因为命令不知道调用方可覆盖的边界。目标中任一角色不在 M 内，整个请求在开始写之前被拒绝。

### 事务内的精确算法

1. 再次校验 Subject 存在。
2. 按 name 加载 M 中的 Role，确认均属于目标 Tenant。
3. 按 Role ID 排序，再依次 `FOR UPDATE` 锁定，降低多角色并发时的死锁顺序不一致。
4. 读取 Subject 当前全部直接 Assignment，只投影出属于 M 的部分。
5. 软删除 `currentManaged - T`。
6. 创建 `T - currentManaged`，由 active unique guard 阻止重复活跃关系。
7. 若没有变化，读取当前 PolicyVersion 后结束；若有变化，只递增一次版本并写一条事件。

整个删除与新增过程在一个 Unit of Work 内。中间任意一个 Create/Delete/Version/Outbox 失败都会回滚，不会留下“删了旧角色但没加新角色”的部分状态。

## 幂等、版本与回滚

- 如果目标受管集合与当前状态一致，操作是 no-op：不递增策略版本，也不写 Outbox。
- 如果发生新增或删除，只递增一次策略版本并写一次版本事件。
- 任一步骤失败时整个事务回滚，不能留下部分替换结果。
- SQLite 串行测试已经覆盖原子性、幂等、非受管角色保留和回滚。

no-op 不递增版本非常重要。如果一个调用方周期性上报相同状态，每次都 bump version 会制造无业务变化的 Outbox 流量和全实例 reload。当前 `Changed=false` 同时表示事实、版本和事件均未改变。

## 当前并发边界

现有实现会锁定相关角色，但读取 Subject 当前 Assignment 的查询本身不是显式锁定读。在 MySQL 默认 `REPEATABLE READ` 下，两个请求并发替换同一 Subject 时，
等待锁之后是否一定观察到对方已提交结果，目前没有专门的 MySQL 并发测试证明。

因此当前可以声称：

- 单事务原子；
- 串行调用幂等；
- 非受管 Assignment 被保留；

但不能声称：

- 任意并发替换都严格线性化；
- 最后完成的请求一定等价于最后写入者获胜。

在补充 MySQL 并发测试和必要的锁定读/隔离设计前，上游应避免并发替换同一 Tenant+Subject。

### 风险不在单条 Assignment 唯一性

active unique guard 能阻止两个事务同时创建完全相同的活跃 Assignment，但 Replace 要保护的是“多条记录组成的集合”。

当前代码先锁定 M 中 Role，再使用普通 `ListBySubject` 读取当前 Assignment。在 MySQL `REPEATABLE READ` 下，该普通读的 read view 时机与锁等待后的可见性需要用真实
MySQL 并发测试确认。只看到“锁了 Role”不足以推导整个替换已线性化。

如要修复，应先定义所需语义（例如按 Tenant+Subject 串行、最后提交者获胜或使用 expected version 的乐观并发），再选择主体级锁、锁定读或版本检查。不应只在代码中多加一个锁就宣布问题解决。

## Assignment 约束配置

Assignment 写入依赖约束授权器。当前缺失实现时的行为不完全对称：

- 增量 Grant/Revoke 可沿已有认证服务身份路径执行；
- `ReplaceManagedAssignments` 会返回内部错误。

生产和开发配置已经显式提供约束文件，但默认空配置仍可能触发上述差异。部署检查应把约束实现或文件视为必填项，不能依赖默认值。

constraints 不只列出角色，还将 caller service、allowed methods、Tenant/domain、Subject 类型/范围、Role 集合与 delegated actor 规则绑在一起。配置与
`grpc_acl.yaml` 必须做覆盖对齐：

- ACL 有方法但 constraints 无 caller 规则：实际调用将被内容级拒绝或失败。
- constraints 允许 caller 但 ACL 没有方法：请求根本到不了应用层。
- 两者都漏配某个新 RPC：可能形成不一致默认行为，所以架构测试要锁定完整 RPC 列表。

`admin` 之类 allow-all 管理规则可以用于增量 Grant/Revoke，但 Replace 必须返回明确受管 Role 集合，因此不接受无边界 allow-all 作为替换权。

## Actor 与审计

所有管理写入都应记录可信 actor：用户管理请求来自 AuthN Principal，服务间请求来自已认证服务身份。actor 不得由请求体自由指定。写命令的错误映射应区分输入错误、无权限、事实冲突和基础设施错误。

当前 gRPC 契约仍包含 `granted_by` / `revoked_by` / `changed_by`，它们是 delegated actor 信息，不能覆盖传输层得到的 caller service。Assignment
constraints 必须决定调用服务是否允许代表该 actor 写入。Revoke 未提供 delegated actor 时，transport 会回退为 `service:<caller>`。

## 提交后 reload 的错误语义

普通 AuthZ command 在提交后调用 `ReloadRuntimePolicy`，它会重试 3 次，每次间隔 100ms，但外层命令当前不把最终 reload 错误返回给调用方。这是有意区分“事实提交失败”与“快照收敛延迟”，
但也带来明确的撤权窗口。

事件消费者则调用可返错的 reload 版本，最终失败会交给消息机制重试。因此运维上必须监控 reload failure 和 policy version lag，不能把命令 200/gRPC OK 解读为所有实例已生效。

## 主要代码与测试

- `internal/apiserver/application/authz/rolebinding/command_service.go`：Assignment 增量写入与批量替换。
- `internal/apiserver/application/authz/rolebinding/types.go`：批量替换命令及输入不变量。
- `internal/apiserver/domain/authz/service`：领域校验与约束端口。
- `internal/apiserver/infra/mysql/uow/authz`：仓储 Unit of Work 与语义测试。
- `internal/apiserver/infra/mysql/uow/authz/replace_managed_assignments_test.go`：原子性、幂等、保留和回滚测试。

## 证据矩阵

| 结论 | 当前证据 | 尚未证明 |
| --- | --- | --- |
| 单次 Replace 原子 | SQLite UoW 回滚测试 | MySQL 异常注入的所有分支 |
| 串行请求幂等 | no-op 测试，版本/事件不变 | 并发相同/不同目标的线性化 |
| 非受管关系保留 | managed/unmanaged fixture | 配置误把角色纳入 M 时的业务安全性 |
| 单条 Assignment 唯一 | migration active guard + repository concurrency test | 整个 Assignment 集合的事务序列 |
| 提交后可通知 | PolicyVersion + Outbox 同事务 | 全部实例在固定时间内收敛 |

## 安全演进清单

修改任何 AuthZ 写链时，至少要回答：

1. 哪些表记录是同一个领域变更？
2. 是否仍在同一 UoW 中 bump version 并 stage Outbox？
3. no-op 是否会被错误变成一次 reload 风暴？
4. 并发唯一性依赖应用预查还是数据库约束？
5. REST 的 ID 写入与 gRPC 的 role-name 写入是否仍经过同一不变量？
6. caller identity、delegated actor、reason 和变更事实是否能完整追溯？
7. 新的 RPC 是否同时进入 ACL、constraints、proto/SDK 和文档门禁？
