# Casbin 为什么只是授权执行引擎

> 状态：已实现 · 本文以当前 AuthZ 领域模型、Casbin adapter/model、授权写入和 runtime reload 为依据；替代模型只作方案分析。

## 1. 先分清三层语言

IAM 授权同时存在三层语言：

| 层 | 典型对象 | 负责什么 |
| --- | --- | --- |
| 业务授权语言 | Subject、Role、Permission、RoleBinding、Resource、Action、Scope、Tenant | 定义可解释的授权事实和不变量 |
| 运行时策略语言 | Casbin r/p/g、matcher、effect | 把已确定的事实高效求值 |
| 传输/接入语言 | REST path、gRPC method、SDK call | 把请求映射到可信授权元组 |

Casbin 只拥有第二层。如果让 domain 直接暴露 p/g 字符串，业务就失去 RoleBinding 的生命周期、GrantedBy、Tenant 和错误语义；如果让 handler 直接调用 Enforcer，输入的 Subject/Resource/Scope 又无法由 application 统一约束。

当前责任链：

```text
transport/service context
  -> application CheckCommand
  -> domain value objects
  -> PolicyRuntime port
  -> infra/casbin projection + EnforceEx
  -> domain Decision
```

## 2. 当前五元模型如何落到 Casbin

领域问题：

```text
Subject × Tenant × Resource × Action × Scope -> Decision
```

运行时模型：

```ini
r = sub, dom, obj, act, scope
p = sub, dom, obj, act, scope
g = _, _, _
```

主要投影：

```text
RoleBinding(subject, role, tenant)
  -> g(subject-ref, role-name, tenant-id)

Permission(role, tenant, resource-pattern, action-pattern, scope)
  -> p(role-name, tenant-id, resource-pattern, action-pattern, scope)
```

`g` 看起来像 RoleBinding，但不是管理事实的全部；`p` 看起来像 Permission，但不承担业务 ID、审计、创建约束或 UI 查询。它们是执行所需的规范化投影。

## 3. Matcher 是语义编译结果，不是业务流程

当前 matcher 组合：

- tenant/domain 相等；
- Subject 在该 domain 拥有 policy role；
- 四段式 Resource pattern 匹配；
- Action pattern 完整正则匹配；
- policy Scope 为 `all:*` 或与请求 Scope 相等；
- policy effect 是 allow-list/default-deny。

它适合纯判定函数，不适合：

- 查询对象当前所有者并改变状态；
- 调外部服务获取动态风险评分；
- 在 matcher 中写长业务流程；
- 直接解析 HTTP path/body；
- 负责 Grant/Revoke 的业务合法性。

复杂业务事实应先由可信 resolver 变成 Scope/attribute，再进入 Check。把 I/O 放进 matcher 会使延迟、失败和审计不可控。

## 4. 为什么 DB 是事实源

若应用写接口直接调用某个进程的 `AddPolicy`：

1. 只有当前实例立即变化；
2. 进程重启后若无持久化可能丢失；
3. Role/Permission 管理表与 runtime fact 可能分裂；
4. 无法让 PolicyVersion/Outbox 与事实同事务。

当前 canonical 写链以 MySQL 为中心：

```text
management entity + casbin_rule + PolicyVersion + outbox
  --same transaction--> commit
  --post commit--> local LoadPolicy
  --relay/broadcast--> every instance LoadPolicy
```

Casbin adapter 启动时从 `casbin_rule` 加载并关闭 autosave。Enforcer 是可重建投影，不是另一个独立写模型。

## 5. 缓存授权的特殊风险

普通展示缓存陈旧通常影响体验；授权缓存陈旧的方向不对称：

```text
grant 未加载  -> 暂时拒绝，偏可用性
revoke 未加载 -> 暂时继续允许，偏安全性
```

因此需要同时观察：

- DB PolicyVersion；
- outbox backlog/publish；
- 每实例 event delivery；
- reload success/error/time；
- 理想的 per-tenant loaded version。

当前 runtime 有 reload health 和最近事件信息，但没有强 per-tenant loaded-version 状态，也没有请求级 version barrier。Decision 中读取的 DB PolicyVersion 不能证明 Enforcer 正以该版本判定。

## 6. `EnforceEx` 与可解释性

只返回 bool 无法回答“为什么允许”。当前 adapter 使用 `EnforceEx` 取得命中的 policy，并尝试还原 MatchedRole/MatchedPermission。

可解释性有三类用途：

- 用户/管理员理解角色为什么生效；
- 审计记录决策依据；
- 排障区分输入错误、无 policy 和 stale runtime。

但返回命中 policy 也要控制敏感信息：外部错误通常只需要稳定 deny code，完整策略细节适合受控日志/管理接口，避免向调用者泄露其他资源或角色结构。

## 7. 为什么采用 default deny

当前 effect 只有 allow，无匹配即拒绝。显式 deny 能表达“允许大集合、排除少数例外”，但会引入：

- allow/deny 冲突优先级；
- 角色继承后的例外传播；
- policy order 或 priority；
- 更难解释的结果；
- 管理接口误配风险。

对当前 IAM，allow-list/default-deny 更容易审核和 fail closed。若未来业务确需 explicit deny，应先定义冲突语义和 lint/解释模型，而不是只改一行 effect。

## 8. Resource 和 Action 为什么不直接用路由

HTTP path/gRPC method 是传输细节，业务 Resource/Action 是稳定能力语言。二者直接绑定会导致：

- REST 版本升级改路径时权限名漂移；
- 同一能力的 REST/gRPC 产生两套策略；
- HTTP method 无法表达 publish/import/export 等业务动作；
- path parameter 容易被误当对象 Scope。

当前使用四段 Resource key 和显式 Action。路由注册表负责从传输入口选择固定权限元组；对象级 Scope 由业务事实解析。

## 9. RBAC、ABAC 与 ReBAC 的选择

### 纯 RBAC

角色可解释、治理成熟，但角色数量可能随对象范围组合爆炸。当前用 Tenant/Scope 增强，而不是为每个来源创建角色。

### ABAC

可以把部门、时间、风险、对象属性直接写规则，表达力强；代价是属性新鲜度、规则调试和决策重放复杂。当前没有通用 ABAC 引擎，部分对象事实先被解析成有限 Scope。

### ReBAC

适合组织图、分享链和“用户是对象 owner 的 guardian”等图关系。当前 ProfileLink 留在 Identity，并不直接变成授权图。若关系查询规模和深度上升，可评估专门 ReBAC/tuple store，但要定义与现有 Role/Permission 的组合语义。

### 当前选择

domain-scoped RBAC + 受控 Scope 提供足够表达力，Casbin 作为成熟 matcher/role graph 引擎降低自研成本。代价是 pattern、投影和多实例 reload 必须有治理。

## 10. 常见反模式的推理

| 反模式 | 为什么看似方便 | 为什么最终危险 |
| --- | --- | --- |
| domain import Casbin | 少写 port/mapper | 领域语义被字符串/runtime API 污染 |
| handler 直接 Enforce | 调用短 | Subject/Tenant/Scope 来源无法统一审计 |
| 业务服务复制 model.conf/policy | 本地快 | 多份真相、撤销和版本漂移 |
| JWT claim 直接作 Permission | 无远程 Check | 权限冻结到 token TTL |
| ProfileLink 直接写 g | 关系可快速授权 | 关系事实与动作权限生命周期混淆 |
| 手工改 `casbin_rule` | 紧急直接 | 绕过管理实体、version、outbox 和审计 |

## 11. 演进时的检查顺序

修改授权语义时依次检查：

```text
domain value object/invariant
  -> application command/use case
  -> management persistence + casbin facts
  -> matcher/projection
  -> version/outbox/reload
  -> transport registry and callers
  -> policy lint/tests/migrations
```

只改 model.conf 可能让历史策略含义改变；只改 domain 又可能让 Casbin projection 无法执行。授权演进必须把“语言定义”和“执行编译”作为一个兼容性变更审查。

## 12. 面试追问

### Casbin 是 RBAC 数据库吗？

不是。Casbin 是策略求值框架；当前 MySQL 管理表和 `casbin_rule` 保存事实，CachedEnforcer 加载后执行。Role/Permission 的业务生命周期仍由 AuthZ domain/application 负责。

### 为什么有 Casbin 还要 PolicyVersion 和 Outbox？

因为每个实例都有内存策略。Casbin 解决单实例如何判定，不解决数据库提交如何可靠通知所有实例重载。

### 能否把所有业务条件都写进 matcher？

技术上可扩展函数，但包含 I/O、动态流程和大量属性后，延迟、可测试性和解释性会恶化。应先把业务事实转换成受控授权输入，只把纯匹配留在 matcher。

## 13. 事实来源与验证

- [授权模型与匹配语义](../02-业务模块/03-AuthZ/01-授权模型与匹配语义.md)
- [权限检查与 Casbin 运行时](../02-业务模块/03-AuthZ/02-权限检查与Casbin运行时.md)
- [授权写入与多实例一致性](../02-业务模块/03-AuthZ/03-授权写入与多实例一致性.md)
- `configs/casbin_model.conf`
- `internal/apiserver/infra/casbin`
- `internal/apiserver/application/authz`

```bash
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./internal/apiserver/domain/authz/... ./internal/apiserver/application/authz/... ./internal/apiserver/infra/casbin/... ./internal/apiserver/container/authz
```
