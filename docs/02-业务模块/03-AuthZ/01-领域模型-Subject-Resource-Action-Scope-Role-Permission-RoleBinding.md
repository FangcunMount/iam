# 领域模型：Subject / Resource / Action / Scope / Role / Permission / RoleBinding

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

| 模型 | 定义 |
| --- | --- |
| Subject | 授权主体引用，不是 User 本体 |
| Resource | 受保护资源 |
| Action | 对资源执行的动作 |
| Scope | 授权范围约束 |
| Role | 权限集合 |
| Permission | Role 对 Resource/Action/Scope 的声明 |
| RoleBinding | Subject 持有 Role 的事实 |
| PolicyVersion | 授权事实版本 |

## 边界

- Assignment 是对外 wire term，不是领域模型主线。
- Casbin 是 infra runtime，不是业务模型。
- ProfileLink 不能替代 RoleBinding。
