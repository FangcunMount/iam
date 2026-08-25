# AuthZ：RBAC 与对象属性条件授权

> 状态：已实现 · 本目录描述仓库中的 AuthZ v3 最终模型，运行时代码只使用新模型。生产数据库验收证据目前仍到 000025，字符串 Scope、AuthZ v2 与持久化 Casbin 权限事实只有在生产完成离线转换并执行 000027 后，才能记为已完成生产退役。

AuthZ 回答：可信 Subject 在某个 Tenant 中，能否对 Resource 执行 Action；对象级动作还可以依据服务端提交的受信对象属性求值。业务关系仍由拥有事实的业务模块判断。

## 阅读路径

1. [模块总览](00-模块总览.md)：最终事实、运行时与 API 总图。
2. [授权模型与匹配语义](01-授权模型与匹配语义.md)：Role、PermissionGrant、ConstraintSet 的语义。
3. [权限检查与原生运行时](02-权限检查与Casbin运行时.md)：v3 Check 和不可变快照。
4. [授权写入与多实例一致性](03-授权写入与多实例一致性.md)：事务、版本、Outbox 与 reload。
5. [内存角色图与授权索引](04-Casbin运行时模型.md)：Casbin 的唯一剩余职责和原生求值流程。
6. [模块边界与代码索引](05-模块边界与代码索引.md)：跨模块边界和修改落点。

## 最终责任链

```text
AuthN Principal / trusted service identity
  -> AuthZ Subject + Tenant
  -> Assignment + RoleInheritance
  -> PermissionGrant(Resource, Action, ConstraintSet)
  -> AuthorizationRuntimeSnapshot
  -> AuthZ v3 Decision
```

- `authz_assignments` 保存 Subject 到 Role。
- `authz_role_inheritances` 保存 Role 到父 Role，禁止循环。
- `authz_permission_grants` 保存 Role 的资源、动作和对象属性条件。
- `authz_resources.attribute_schema` 注册允许参与授权的对象属性。
- `authz_policy_versions` 与 Outbox 协调运行时 reload。
- Casbin 只在内存中计算角色继承，不加载或保存权限规则。

快照用于路由能力候选判断：`UNCONDITIONAL` 可直接满足通用 capability；`OBJECT_CHECK_REQUIRED` 必须在业务服务加载对象后调用 v3 `Check`。列表、搜索和批量操作不接受条件 Grant。
