# Casbin 为什么只保留为内存角色图

> 状态：已实现 · IAM 已用原生 PermissionGrant runtime 替换通用权限 matcher；Casbin 的唯一职责是根据 Assignment 与 RoleInheritance 计算内存角色图。

## 1. 最终分工

| 层 | 负责内容 |
| --- | --- |
| AuthZ domain | Role、Assignment、RoleInheritance、PermissionGrant、ConstraintSet、Resource Schema 的不变量 |
| Native runtime | 资源/动作匹配、属性校验、条件求值、快照版本和决策解释 |
| Casbin role manager | 装载 Subject→Role 的 Assignment 与 Role→Role 的继承边，计算有效 Role |
| transport/application | 建立可信 Subject/Tenant/Resource/Action/ObjectAttributes |

Assignment 与角色继承组成一个成熟、封闭的图计算问题，保留 Casbin role manager 可以复用其 domain-aware 关系和继承闭包能力。PermissionGrant 条件则是 IAM 自己的版本化契约，需要类型、Schema、错误码、审计标识和快照模式，适合由领域与原生 runtime 显式实现。

## 2. 为什么不再让 Casbin 执行权限规则

- 字符串 tuple 无法自然表达类型化属性和 Schema 校验。
- 任意 matcher/正则会扩大管理接口的策略语言和审计面。
- 管理事实与执行投影并存会引入双写、对账和漂移。
- 对象属性缺失、类型错误、matched Grant 和版本需要稳定公开语义。
- 列表/批量与对象级候选必须在快照中显式区分。

因此权限事实只有 PermissionGrant 一份，runtime 直接编译它；不存在另一份在线权限投影。

## 3. 安全边界

Casbin role manager 的结果只是候选 Role，不是 allow。最终仍需：

```text
role closure
  -> candidate PermissionGrant
  -> Resource/Action match
  -> ObjectAttribute schema validation
  -> ConstraintSet evaluation
```

任何 reload 异常都会阻止整份新快照发布。系统不会切回退役路径，也不会因为 IAM 网络错误自动允许业务请求。

## 4. 演进约束

当前只支持 allow-only、对象属性、`EQ` 和 Grant 内 AND。若将来增加运算符或属性类型，应扩展版本化 ConstraintSet 和兼容测试，而不是把条件塞回字符串 matcher。主体属性、环境属性和业务关系是否进入 IAM，必须分别评估事实所有权、新鲜度、可重放性和故障语义。

若将来删除 Casbin，必须先提供 Assignment 与 RoleInheritance 的等价图实现，并覆盖多层继承、循环拒绝、Tenant 隔离和 reload 原子性；“权限规则已由原生 runtime 执行”本身不是删除依据。
