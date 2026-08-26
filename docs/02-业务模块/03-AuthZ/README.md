# AuthZ：RBAC 与对象属性条件授权

> 状态：已实现 · 已完成生产切换。本目录描述 AuthZ v3 最终模型。AuthZ 切换在 000027 闭合；生产数据库现已验收 `version=28, dirty=0`，000028 仅增加 AuthN `global_identifier` 唯一性约束，不改变 AuthZ 模型。AuthZ v2、字符串 Scope、持久化 Casbin 权限事实和一次性切换入口均已退役。

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

## 生产切换证据

- [一次性切换 run `32859067799`](https://github.com/FangcunMount/iam/actions/runs/32859067799) 从 000025 完成 000026 转换与 000027 退役；转换结果为 105 条 Grant、6 条 RoleInheritance，verify/evidence 的规范化 hash 一致。
- [发布后数据库状态 run `32876762969`](https://github.com/FangcunMount/iam/actions/runs/32876762969) 证明 `version=27, dirty=0`，最终 16 张 BASE TABLE 精确匹配，`casbin_rule`、`authz_cutover_state` 与 `scope_kinds` 均不存在。
- [RoleBinding guard run `32876761874`](https://github.com/FangcunMount/iam/actions/runs/32876761874) 证明 active 重复组与额外行均为 0，guard 结构完整。
- [最终 IAM 发布 run `32877019508`](https://github.com/FangcunMount/iam/actions/runs/32877019508) 与[独立健康检查 run `32877567211`](https://github.com/FangcunMount/iam/actions/runs/32877567211) 证明 SHA `d3f58369d8c58dbf50ae15282f5641bc370055a6` 已部署，容器 healthy，`/healthz`、`/readyz` 均为 200，MySQL/Redis 可达。

切换使用的生产备份、证据文件和旧镜像按 30 天恢复保留期保存，但不再作为在线兼容路径。仓库只保留不可改写的历史 migration 与旧对象缺失性门禁，不保留可再次执行转换的命令或 workflow。
