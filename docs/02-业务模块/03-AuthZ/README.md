# AuthZ：RBAC 与对象属性条件授权

> 状态：已实现 · 本目录以当前仓库代码为准。历史生产切换证据只证明当时指定 SHA 与数据库状态，不自动证明当前 HEAD 已发布或通过生产验收。

AuthZ 回答一个问题：可信 Subject 在某个 Tenant 中，是否可以对 Resource 执行 Action；对象级动作还可以根据业务服务提交的受信对象属性求值。业务归属关系仍由拥有事实的业务模块判断。

## 30 秒结论

```text
建权与赋权
  -> Role / Assignment / RoleInheritance / PermissionGrant / Resource
验权与决策
  -> AuthorizationRequest + RuntimeSnapshot -> DecisionService / Evaluator -> Decision
施权与执行
  -> Middleware / 业务服务消费 Decision -> continue / deny / error
```

- `PolicyVersion + Outbox + reload` 是建权与赋权后的发布收敛机制，不是独立授权子域。
- 不可变角色图与 Grant 索引是验权运行时的可重建投影，不是第二份权限事实。
- 施权边界不重新解释 Role 或 Grant；它只把 Decision 落实为执行、拒绝或系统错误。
- REST v3 是管理接口；授权判定 `Check` 由 gRPC v3 提供。
- `roles` 表示包含继承结果的有效角色，`direct_roles` 表示直接 Assignment。
- `ReplaceManagedAssignments` 只替换约束策略认定的“受管角色集合”，并保留集合外 Assignment。
- 自有不可变角色图承担 Subject→Role 和 Role→Role 闭包计算；权限匹配与 `ConstraintSet` 求值由领域 `Evaluator` 完成，运行时不再依赖 Casbin。
- 授权写入、策略版本递增和 Outbox 入队处于同一事务；实例通过本地 reload 和版本事件收敛。

## 阅读路径

### 一、模块总览

1. [模块总览](00-模块总览.md)：建立事实、投影、控制面、执行面、信任和失败边界的整体模型。

### 二、领域模型设计

2. [领域模型设计](01-领域模型设计.md)：深入 Subject、Tenant、Role、Assignment、RoleInheritance、PermissionGrant、Resource、
   ConstraintSet 和 ObjectAttributes 的责任与不变量。

### 三、关键链路分析

3. [授权判定与不可变快照](02-关键链路-授权判定与不可变快照.md)：深入 Dataset、`BuildSnapshot`、`Check`、不可变角色图、Decision 与可观测性。
4. [授权写入与受管 Assignment](03-关键链路-授权写入与受管Assignment.md)：深入 UoW、增量写入、受管集合替换、幂等与并发边界。
5. [多实例策略收敛](04-关键链路-多实例策略收敛.md)：深入 PolicyVersion、Outbox、ephemeral subscriber、reload、健康和维护证据。
6. [REST 管理与路由授权](05-关键链路-REST管理与路由授权.md)：深入 REST 控制面、JWT Principal、Resource/Action 路由矩阵与 current/platform Tenant 判定。
7. [gRPC 服务间授权与 SDK](06-关键链路-gRPC服务间授权与SDK.md)：深入 `Check`、Snapshot、Assignment RPC、服务身份、ACL、属性白名单和 SDK。
8. [模块边界](07-模块边界-AuthZ与AuthN-Identity-Suggest.md)：深入 AuthZ 与 AuthN、Identity、IDP、Suggest 及业务服务的事实所有权。

### 四、落地导航

9. [分层架构与代码索引](08-分层架构与代码索引.md)：定位组合根、修改落点、故障链、验证证据与文档维护规则。

## 本目录要回答的设计问题

本目录不是 API 清单，而是要把以下问题闭合：

| 问题 | 当前答案 | 深入位置 |
| --- | --- | --- |
| 权限事实由谁拥有 | MySQL 中的 AuthZ v3 表；业务对象事实留在业务模块 | 00、01 |
| Subject 为什么能得到某个 Role | 直接 Assignment，加 RoleInheritance 闭包 | 01、02 |
| Role 为什么能执行动作 | 命中 PermissionGrant 的 Resource/Action，且条件满足 | 01、02 |
| 对象属性为什么可信 | gRPC transport 按调用服务与资源白名单接收，业务服务负责加载对象 | 00、02、06、07 |
| 请求期为什么不查权限表 | 启动/reload 构建不可变快照，请求只读原子指针 | 02、04 |
| 写成功后何时生效 | 本实例尽力立即 reload，其他实例通过 durable version event 最终收敛 | 03、04 |
| 服务能不能随意改用户角色 | gRPC ACL 控制方法，Assignment constraints 控制 domain/subject/role/actor | 03、06 |
| 角色闭包怎样计算 | Snapshot 内的自有不可变角色图计算 Assignment 与 RoleInheritance | 00、02 |
| 如何证明文档没有漂移 | proto/OpenAPI/route/architecture/docs facts 与聚焦测试分层证明 | 08 |

## 设计核心：把五类变化拆开

AuthZ 当前结构的关键不是“用了 RBAC”，而是把五类变化隔离：

1. **主体变化**：User 是否存在由 Identity 负责；AuthZ 只持有 Subject 引用。
2. **组织变化**：谁被直接授予什么角色由 Assignment 负责。
3. **岗位复用变化**：角色之间如何继承由 RoleInheritance 负责。
4. **能力变化**：角色能对什么资源做什么由 PermissionGrant 负责。
5. **对象状态变化**：具体对象的 origin/status/owner 等由业务模块加载，并以受信属性进入 Check。

如果把这些事实压回一条字符串 policy，会产生三类耦合：用户关系与权限事实共用生命周期、业务对象属性被迫进入 IAM、管理写模型与执行 matcher 同时变化。当前模型接受更多显式对象与表，换取可审计的责任边界和可独立演进的运行时。

## 控制面与数据面

```text
控制面（低频写）
REST v3 / Assignment gRPC
  -> Application Command
  -> Domain invariant
  -> MySQL + PolicyVersion + Outbox

数据面（高频读）
可信服务 / IAM route middleware
  -> DecisionService.Check / RouteDecisionService.CheckRoutePermission
  -> immutable snapshot
  -> Decision
```

控制面可以承担事务、锁与依赖校验；数据面必须避免每次请求 join 多张表。二者通过策略版本和完整快照重建连接，而不是共享一套可变缓存对象。

## 当前模型选择与代价

| 选择 | 解决的问题 | 接受的代价 |
| --- | --- | --- |
| allow-only、默认拒绝 | 决策组合简单，避免 allow/deny 优先级歧义 | 复杂例外需重新设计角色或条件 |
| 四段 Resource | 跨应用命名稳定，通配规则可审计 | 资源命名必须前置治理 |
| 类型化 ConstraintSet v1 | 条件可校验、可版本化、可解释 | 当前只有 `eq`、`all_of` 和最多 8 个谓词 |
| 不可变全量快照 | 请求期无锁读，失败不发布半快照 | reload 成本与多实例滞后需要运维治理 |
| 自有不可变角色图 | 类型化表达继承闭包，权限语义和运行时实现均归 IAM | 需要自行保护深度边界、去重与 Tenant 隔离 |
| durable version event | 数据提交与通知记录同事务 | 不是跨实例同步 barrier，仍是最终一致 |
| 受管 Assignment 替换 | 一个服务只能覆盖自己的角色集合 | constraints 配置和并发语义更复杂 |

## 最小不可破坏不变量

任何重构至少必须保持：

```text
默认拒绝；
Tenant 隔离；
Assignment 与 RoleInheritance 不混淆；
继承图无环；
direct roles 与 effective roles 不混淆；
条件属性必须有 Resource schema 且来自受信服务；
列表/搜索/批量动作不能由条件 Grant 放行；
权限事实、PolicyVersion 与 Outbox 同事务；
新快照完整构建成功后才原子发布；
REST 管理与 gRPC Check 边界不倒置。
```

这些是不变量，不是实现偏好。若要改变其中一项，应先修改领域合同与威胁模型，再改实现和文档。

## 如何阅读 Decision

`Check` 不只返回布尔值：

- 允许时给出 matched Grant、matched Role 与策略版本；
- 未命中时返回 `policy_not_matched`；
- 条件所需属性缺失时返回 `attribute_missing` 和缺失 key；
- 输入属性未注册、类型不符或来源不可信属于合同错误，不伪装成普通 deny。

“普通 deny”与“调用方违反属性合同”是不同问题：前者是权限事实结论，后者是接入错误。监控与业务日志不应把二者混成一个失败率。

## 本目录不承诺什么

- 不承诺历史生产验收自动覆盖当前 HEAD。
- 不承诺 Outbox publish 成功即所有实例加载完成。
- 不承诺一次条件 MySQL 并发测试可以覆盖全部生产调度、隔离配置和受管集合组合。
- 不承诺快照 permission 列表等价于对象级 allow；`OBJECT_CHECK_REQUIRED` 仍要加载对象并 Check。
- 不承诺 role name 是权限；任何放行最终都必须命中 PermissionGrant。
- 不承诺文档门禁能理解所有自然语言；它只保护已编码的结构和关键事实。

## 当前必须保留的边界

- MySQL 是权限事实源；不可变角色图、Grant 索引和完整快照都是可重建投影。
- `RequirePermissionOrGlobal` 先检查当前 Tenant，再检查平台域；“平台域只有通配 Grant 才能全局放行”是当前数据基线，不是代码强制不变量。
- `ReplaceManagedAssignments` 的返回值当前是目标受管角色子集，不等同于持久化后的全部直接角色。
- MySQL `REPEATABLE READ` 下使用锁定当前读重算 Subject Assignment；专门并发测试需要 `MYSQL_HOST`，被跳过时不能声明已有 MySQL 运行证明。
- Assignment 约束授权器缺失时，增量 Grant/Revoke 与批量 Replace 的失败行为并不对称，部署配置必须显式提供实现。

## 事实来源与证据边界

发生冲突时按以下优先级校对：组合根与运行时代码 > protobuf/OpenAPI/数据库 migration > 测试与门禁 > 本目录说明 > 历史专题和演讲材料。

历史切换、发布与健康检查证据保留在[生产验收记录](../../01-运行时/08-IAM重构最终验收记录.md)中。它们用于追溯，不作为当前 HEAD 已发布的证明。

## 快速验证

```bash
go test ./internal/apiserver/domain/authz/... \
  ./internal/apiserver/application/authz/... \
  ./internal/apiserver/infra/authz/...
python3 scripts/check-docs-links.py
python3 scripts/check-docs-facts.py
python3 scripts/check-route-contracts.py
python3 scripts/check-openapi-contracts.py
```
