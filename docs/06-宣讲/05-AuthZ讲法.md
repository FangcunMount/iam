# AuthZ 讲法

AuthZ 负责判断 Subject 是否能对 Resource 执行 Action。

讲解主线：

```text
Subject
  -> RoleBinding
  -> Role
  -> Permission
  -> Resource / Action / Scope
  -> AuthorizationDecision
```

重点区分：

- AuthZ 不是 `user.role == admin`。
- AuthZ 不是 Casbin CRUD。
- Casbin 是 infra runtime，不是领域模型。

事实层入口：[../02-业务模块/03-AuthZ/README.md](../02-业务模块/03-AuthZ/README.md)。
