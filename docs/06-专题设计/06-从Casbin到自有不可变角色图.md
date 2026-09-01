# 为什么从 Casbin 迁移到自有不可变角色图

> 状态：已实现 · IAM 已完成 Casbin 运行时退役；Assignment 与 RoleInheritance 由自有不可变角色图解析，PermissionGrant 由领域 `Evaluator` 求值。

## 1. 最终分工

| 层 | 负责内容 |
| --- | --- |
| AuthZ domain | Role、Assignment、RoleInheritance、PermissionGrant、ConstraintSet、Resource Schema 的不变量 |
| Authorization Evaluator | 资源/动作匹配、属性校验、条件求值和决策解释 |
| Immutable role graph | 装载 Subject→Role 的 Assignment 与 Role→Role 的继承边，计算有效 Role |
| Native runtime | 编译并发布不可变快照，维护版本、reload 和运行健康 |
| transport/application | 建立可信 Subject/Tenant/Resource/Action/ObjectAttributes |

Assignment 与角色继承组成一个成熟、封闭的图计算问题。当前实现直接使用 `subject.Ref`、`tenant.ID` 和 `role.Name` 构建只读图，保留直接/有效角色、Tenant 隔离、稳定排序、菱形去重与 32 层边界。
PermissionGrant 条件是 IAM 自己的版本化契约，由领域类型、Schema、错误码、审计标识和快照模式显式表达。

## 2. 为什么最终退役 Casbin

- 字符串 tuple 无法自然表达类型化属性和 Schema 校验。
- 任意 matcher/正则会扩大管理接口的策略语言和审计面。
- 管理事实与执行投影并存会引入双写、对账和漂移。
- 对象属性缺失、类型错误、matched Grant 和版本需要稳定公开语义。
- 列表/批量与对象级候选必须在快照中显式区分。
- 当 PermissionGrant、ConstraintSet、Evaluator、Snapshot 和 PolicyVersion 都已原生化后，Casbin 只剩四个角色图调用，却继续引入字符串编码和第三方依赖。

因此权限事实只有 PermissionGrant 一份，角色关系只有 Assignment 与 RoleInheritance 两类事实；角色图和 Grant 索引都是可重建快照投影。

## 3. 安全边界

不可变角色图的结果只是候选 Role，不是 allow。最终仍需：

```text
role closure
  -> candidate PermissionGrant
  -> Resource/Action match
  -> ObjectAttribute schema validation
  -> ConstraintSet evaluation
```

任何 reload 异常都会阻止整份新快照发布。系统不会切回退役路径，也不会因为 IAM 网络错误自动允许业务请求。

## 4. 演进约束

当前只支持 allow-only、对象属性、`EQ` 和 Grant 内 AND。若将来增加运算符或属性类型，应扩展版本化 ConstraintSet 和兼容测试，而不是把条件塞回字符串 matcher。主体属性、环境属性和业务关系是否进入
IAM，必须分别评估事实所有权、新鲜度、可重放性和故障语义。

Casbin 退役不改变 API、数据库事实或 reload 协议。历史 migration 和 `casbin_rule` 缺失性门禁继续保留；它们证明迁移历史和防止旧路径回归，不代表当前运行时仍依赖 Casbin。
