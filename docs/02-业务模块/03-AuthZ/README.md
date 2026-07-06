# AuthZ

## 30 秒结论

AuthZ 是授权域，回答：

```text
某个 Subject 在某个授权域下，
能不能对某个 Resource 执行某个 Action，
并满足某个 Scope？
```

## 文档结构

| 文档 | 作用 |
| --- | --- |
| [00-模块总览.md](00-模块总览.md) | AuthZ 职责和边界 |
| [01-领域模型-Subject-Resource-Action-Scope-Role-Permission-RoleBinding.md](01-领域模型-Subject-Resource-Action-Scope-Role-Permission-RoleBinding.md) | 授权领域模型 |
| [02-领域模型图.md](02-领域模型图.md) | 模型图 |
| [03-核心对象生命周期.md](03-核心对象生命周期.md) | 资源、角色、权限、绑定、版本生命周期 |
| [04-关键链路-权限检查Check.md](04-关键链路-权限检查Check.md) | Check 读链路 |
| [05-关键链路-授权写入Grant-Revoke-Bind-Unbind.md](05-关键链路-授权写入Grant-Revoke-Bind-Unbind.md) | 授权写入链路 |
| [06-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md](06-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md) | 版本传播链路 |
| [07-Casbin运行时模型.md](07-Casbin运行时模型.md) | Casbin runtime 边界 |
| [08-模块边界-AuthZ与AuthN-Identity.md](08-模块边界-AuthZ与AuthN-Identity.md) | 跨模块边界 |
| [09-分层架构与代码索引.md](09-分层架构与代码索引.md) | 代码事实源 |
