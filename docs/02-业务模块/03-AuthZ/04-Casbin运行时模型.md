# 内存角色图与原生授权索引

> 状态：已实现 · 文件名为历史链接保留。Casbin 不保存或执行 PermissionGrant，只作为进程内 RoleInheritance 图计算器。

## 1. 角色图

运行时从 Assignment 和 RoleInheritance 构造租户隔离的内存角色图，用于求出 Subject 的直接及继承 Role。角色名和 Subject 使用不同类型/编码边界，避免字符串碰撞。构建时再次验证未知角色和继承循环，任何异常阻止新快照发布。

## 2. PermissionIndex

PermissionGrant 被编译为按 Tenant、Role、Resource、Action 查询的不可变索引。索引只保存已校验的 active Grant；Resource AttributeSchema 与索引位于同一快照，因此不会出现“新条件配旧 Schema”的半加载状态。

Grant 求值：

```text
subject roles
  -> candidate grants
  -> resource/action match
  -> attribute schema validation
  -> all_of predicates
  -> first allow or final deny
```

多个 Grant 是 OR，单个 Grant 内是 AND。空 ConstraintSet 是无条件 allow。

## 3. 并发与 reload

请求只读取当前不可变快照，不持有数据库锁。reload 在新对象上完成读取、校验和编译，然后一次原子交换引用。失败不会部分更新角色图、索引、Schema 或版本。

运行时暴露低基数指标：Check 结果与耗时、reload 成败与耗时、loaded version 和版本滞后。指标不得包含 Subject、对象 ID 或属性值。

## 4. 明确不存在的路径

- 不持久化角色图或权限 tuple。
- 不从退役权限事实加载或 fallback。
- 不执行正则策略语言。
- 不在 matcher 中访问业务数据库或远程服务。
- 不把角色图计算结果当作最终权限；Role 必须继续匹配 PermissionGrant。
