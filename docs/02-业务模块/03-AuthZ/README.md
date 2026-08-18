# AuthZ：授权事实与运行时判定

> 状态：已实现 · 授权模型、Check、写入事务、Casbin 投影和多实例传播已按当前实现复核。

AuthZ 回答：某个 Subject 在某个 Tenant 中，能否对某个 Resource 执行某个 Action，并满足对象 Scope。AuthN 只负责建立可信身份，不能代替这一步。

## 阅读路径

1. [模块总览](00-模块总览.md)：建立领域事实、Casbin 投影和 committed/published/observed/loaded 状态模型。
2. [授权模型与匹配语义](01-授权模型与匹配语义.md)：理解 domain-scoped RBAC 与五元判定。
3. [权限检查与 Casbin 运行时](02-权限检查与Casbin运行时.md)：理解可信输入、p/g/r 投影、matcher 与缓存边界。
4. [授权写入与多实例一致性](03-授权写入与多实例一致性.md)：理解事务、PolicyVersion、Outbox、广播和 reload。
5. [Casbin 运行时模型](04-Casbin运行时模型.md)：逐项理解 matcher、锁、缓存、反投影和 reload 保证。
6. [模块边界与代码索引](05-模块边界与代码索引.md)：区分 Principal/User/Subject、ProfileLink/RoleBinding，并定位跨层修改面。

## 责任边界

```text
AuthN Principal / service identity
  -> AuthZ Subject
  -> RoleBinding -> Role -> Permission
  -> Resource + Action + Scope + Tenant
  -> Decision
```

- Identity 拥有 User/Profile，不拥有 RoleBinding。
- Casbin 是运行时判定器，不是授权事实的唯一写入口。
- Snapshot 用于能力展示/缓存，不能替代服务端 Check。
- Suggest 可消费 AuthZ 能力计算可见范围，AuthZ 不维护搜索索引。

## 当前实现要特别记住的四点

- `casbin_rule` 与管理实体、PolicyVersion、Outbox 在一个 MySQL UoW 中提交。
- DB 是事实源；`CachedEnforcer` 是每个实例的派生快照。
- 本地 reload 在事务提交后执行，失败不回滚已提交事实。
- durable outbox 把版本事件广播给每个实例，但当前没有请求级 loaded-version barrier。

## 代码入口

- domain：`internal/apiserver/domain/authz`
- application：`internal/apiserver/application/authz`
- runtime：`internal/apiserver/infra/casbin`
- persistence：`internal/apiserver/infra/mysql/{casbinrule,policy,role,rolebinding,resource}`
- composition：`internal/apiserver/container/authz`
- matcher：`configs/casbin_model.conf`

## 验证

```bash
go test ./internal/apiserver/domain/authz/... ./internal/apiserver/application/authz/... ./internal/apiserver/infra/casbin/... ./internal/apiserver/container/authz
```
