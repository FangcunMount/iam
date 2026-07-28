# Casbin 运行时模型

> 状态：已实现 · 本文只解释领域授权事实怎样投影成 Casbin 执行模型，以及缓存、并发、匹配和 reload 的真实语义。

## 1. 定位：Casbin 是执行引擎，不是业务事实源

AuthZ 领域关心 Subject、Role、Permission、Resource、Action、Scope 和 PolicyChange；Casbin 关心字符串元组是否匹配。两者通过 `infra/casbin/facts.go` 显式转换：

| 领域概念 | Casbin 表示 |
| --- | --- |
| `subject.Ref{Type:user, ID:42}` | `user:42` |
| Role `operating:editor` | `role:operating:editor` |
| RoleBinding | `g, user:42, role:operating:editor, fangcun` |
| Permission | `p, role:operating:editor, fangcun, app:domain:type:*, read, all:*` |
| AuthorizationRequest | `r, user:42, fangcun, app:domain:type:123, read, origin:x` |

给 Subject 和 Role 使用不同前缀，是为了避免 Casbin 的字符串世界把两个概念误当成同一主体。领域模型仍保留 typed value object，防止这种编码细节向上泄漏。

## 2. model.conf 逐项解释

当前模型：

```ini
[request_definition]
r = sub, dom, obj, act, scope

[policy_definition]
p = sub, dom, obj, act, scope

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && resourceMatch(r.obj, p.obj) && actionMatch(r.act, p.act) && scopeMatch(r.scope, p.scope)
```

逐项含义：

- `g(r.sub, p.sub, r.dom)`：主体在本租户域继承策略角色；
- `r.dom == p.dom`：策略不能跨 domain 生效；
- `resourceMatch`：四段资源逐段匹配，policy 段可为 `*`；
- `actionMatch`：具体 action 对 policy ActionPattern 做锚定正则匹配；
- `scopeMatch`：`all:*` 是通配，其他 scope 精确相等；
- effect 只有 allow 聚合，没有显式 deny 优先规则。

因此当前系统是 default-deny + allow policy，而不是 permit-overrides/deny-overrides 混合策略。若以后增加显式 deny，不能只往表里加一列，必须重新设计 effect、冲突优先级、解释信息和迁移策略。

## 3. Resource 匹配为何不用普通字符串前缀

资源键被约束为四段：

```text
<app>:<domain>:<type>:<name-or-pattern>
```

`resourceMatch` 会先验证 request/policy 都恰好四段，再逐段比较。policy 的 `*` 只匹配一个完整 segment，而不是任意字符子串。

例如：

| request | policy | 结果 | 原因 |
| --- | --- | --- | --- |
| `scale:form:template:42` | `scale:form:template:*` | match | name 段通配 |
| `scale:form:template:42` | `scale:*:template:*` | match | domain/name 分段通配 |
| `scale:form:template:42` | `scale:form:*` | no match | policy 不是四段 |
| `scale:form:template:42` | `scale:for*:template:*` | no match | `*` 不是 segment 内 glob |

分段匹配比普通 `strings.HasPrefix` 更安全：`form` 不会意外匹配 `formal`，缺失 segment 也不会被容错放行。

## 4. ActionPattern 的表达力与风险

request Action 经过 `NewAction`，必须是非空具体操作，不能包含 `|` 或 `*`。policy ActionPattern 只要求非空，运行时用：

```text
^(?:<policy pattern>)$
```

匹配 request action。

这允许一条 policy 表达 `read|list` 等组合，也意味着 policy 是正则而不是普通枚举。风险包括：

- 规则作者误写正则元字符，扩大授权范围；
- 复杂表达式增加 CPU 风险；
- 管理 UI 若把它展示成普通 action，会误导审计者；
- 资源目录的 `HasAction` 在写链只接受 catalog 中的具体 action，正常管理入口会收紧此风险，但历史/直接写表事实仍需 lint。

因此 ActionPattern 的 canonical 解释在 AuthZ 文档和 linter，而不应由各业务系统自行猜测。

## 5. Scope 匹配不是层级继承

当前只有：

```text
all:*       -> 匹配任何 request scope
origin:foo  -> 只匹配 origin:foo
```

不存在 `origin:a` 自动包含 `origin:a/b`，也不存在 org tree、owner、region 的隐式层级。`ScopeFromKey` 遇到非法持久值会回落到 `all:*`，而新写入路径会先由 typed scope 校验阻止非法值。

这个回落是历史兼容策略，安全上必须谨慎：损坏的旧 scope 被归一化为 all 可能扩大权限。当前启动时只会把空 `p.v4` 归一化为 `all:*`。若将来收紧策略，建议把非法 scope 做显式迁移/拒绝，而不是继续宽松解析。

## 6. CachedEnforcer 与锁

`CasbinAdapter` 持有一个 `CachedEnforcer`，外层 `enforcerHolder` 使用 `sync.RWMutex`：

- Check、查询角色/权限使用读锁；
- LoadPolicy、InvalidateCache、直接增删 runtime facts 使用写锁；
- reload 期间新判定会等待，已拿到读锁的判定先完成；
- `LoadPolicy` 前先 invalidate 缓存，成功后内存规则来自 MySQL。

为什么需要外层锁：Casbin 自身提供的缓存并不自动定义“reload 与并发 Enforce 的完整原子可见性”。外层锁把切换临界区明确化，避免读到半加载状态。

当前 reload 不是构建第二个 Enforcer 后原子换指针，而是在同一个 Enforcer 上持写锁执行 `LoadPolicy`。优点是实现简单；代价是大策略集 reload 会暂停 Check。若监控显示 reload 尾延迟影响请求，可以演进为双缓冲 Enforcer：后台完整构建并验证新实例，再原子 swap；但必须同时处理缓存、函数注册、健康状态和旧实例回收。

## 7. 为什么关闭 AutoSave

`NewCasbinAdapter` 执行 `EnableAutoSave(false)`。这条选择十分关键：

- 授权写入必须经过 `PolicyChangeCommitter`；
- 管理表、Casbin facts、PolicyVersion 和 outbox 才能处于同一事务；
- runtime adapter 不能因为一次 `AddPolicy` 就绕过应用事务直接落库。

代码中保留 `addPolicyFacts/removePolicyFacts` 等方法用于适配，但权威写链应通过 MySQL UoW 的 `AuthorizationFacts` 仓储。直接对 Enforcer 开 AutoSave 会产生第二个事务所有者，破坏一致性模型。

## 8. EnforceEx 与 Decision

普通 `Enforce` 只回答 true/false；`EnforceEx` 还返回命中的 policy：

```text
allowed, matchedRule, err
```

adapter 把 matched rule 反投影成 `permission.Permission`，应用返回：

- `Allowed`；
- `Reason`；
- `DenyCode`；
- `MatchedRole`；
- `MatchedPermission`；
- `EvaluatedAt`；
- 数据库当前 `PolicyVersion`。

允许结果有解释上下文，便于审计和排障。拒绝结果当前只有 `policy_not_matched`，不能区分“没有角色”“资源不匹配”“scope 不匹配”；若需要详细拒绝解释，必须谨慎设计，避免向不可信调用方泄露策略结构。

## 9. Runtime reload 的真实保证

`ReloadRuntimePolicy` 最多尝试三次，每次失败之间等待 100ms；每次先尝试清缓存，再 `LoadPolicy`。三次都失败只记录 degraded，不把错误返回给已经提交的写链。

健康状态记录：

```text
lastReloadErr
lastReloadAt
lastEventTenantID
lastEventVersion
lastEventAt
policySyncChannel
reloadLag
```

它没有记录：

- 每租户 loaded version；
- 本次 reload 对应哪一个 event/version；
- 当前 Enforcer 的策略 checksum；
- 多实例追平情况。

因此 health 可以回答“最近 reload 是否报错、最近见到什么事件”，不能严格回答“当前实例对 tenant X 已加载 version N”。

## 10. Snapshot 与 Check 共用 runtime，但目的不同

`SnapshotStore` 从 Enforcer 读取隐式角色和权限，按 app 过滤后返回给调用方，并附 committed version。它适合 UI 权限快照、调用方能力导航和令牌外的权限同步。

Snapshot 不是安全判定：

- 快照可能在使用前过期；
- 调用方可能错误实现 wildcard/scope；
- 不能保证撤权立即被调用方感知；
- Check 的 matcher 才是 canonical 执行语义。

所以服务端保护资源仍应走路由守卫或业务 Check，不能只相信客户端携带的 snapshot。

## 11. 测试重点

| 风险 | 应覆盖的测试 |
| --- | --- |
| 四段资源通配 | exact、单段 wildcard、非法段数、segment 内 wildcard |
| ActionPattern | exact、组合表达式、非法正则、空值 |
| Scope | all、origin exact、非法持久值兼容 |
| 租户隔离 | 同 Subject/Role 在不同 dom 不串权 |
| 角色投影 | subject/role 前缀和 implicit role |
| reload 并发 | Check 与 LoadPolicy 不读半状态 |
| decision explanation | allowed 返回 matched permission；deny 不伪造匹配 |

## 12. 事实源与验证

- 模型：`configs/casbin_model.conf`
- 映射：`internal/apiserver/infra/casbin/facts.go`
- 匹配与 reload：`internal/apiserver/infra/casbin/adapter.go`
- 锁：`internal/apiserver/infra/casbin/enforcer_holder.go`
- 健康：`internal/apiserver/infra/casbin/runtime_health_state.go`
- 领域请求/决策：`internal/apiserver/domain/authz/decision`

```bash
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test -race \
  ./internal/apiserver/infra/casbin/... \
  ./internal/apiserver/domain/authz/decision/... \
  ./internal/apiserver/container/authz/...
```
