# Casbin 运行时模型

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

Casbin 是 AuthZ 的 infra runtime engine，不是 AuthZ 的领域模型。

## 角色

- AuthZ domain 使用 Subject、Role、Permission、RoleBinding、Resource、Action、Scope。
- Infra 将授权事实映射为 Casbin runtime facts。
- Check 链路通过 DecisionEngine 调用 runtime。

## 边界

- 不把 `p/g/r` facts 写成领域语言。
- 不让 transport 直接访问 Casbin。
- 不让业务文档把 AuthZ 简化成 Casbin CRUD。

设计取舍见 [../../05-专题设计/04-Casbin在AuthZ中的定位.md](../../05-专题设计/04-Casbin在AuthZ中的定位.md)。
