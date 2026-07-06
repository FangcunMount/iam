# 领域模型：User / Profile / ProfileLink

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## User

`User` 是 IAM 内部稳定身份主体。它可以被 AuthN 的 `Principal.UserID` 指向，也可以被 AuthZ 表达为 `Subject`。

## Profile

`Profile` 是业务身份资料、业务档案或被服务对象。它不是登录账号，也不是授权主体本身。

## ProfileLink

`ProfileLink` 是 User 与 Profile 之间的关系事实。它表达关系类型、关系状态和撤销语义。

## 边界

- ProfileLink 不是 Permission。
- ProfileLink 不能替代 RoleBinding。
- ProfileLink 不是 Suggest 可见范围的直接替代品。
