# Casbin 在 AuthZ 中的定位

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

Casbin 是 AuthZ 的 infra runtime engine，不是领域模型。

领域模型应该使用 Subject、Role、Permission、RoleBinding、Resource、Action、Scope。Casbin 的 p/g/r facts 是运行时表达，不应污染 domain 语言。

## 模块回链

当前实现链路见 [../02-业务模块/03-AuthZ/07-Casbin运行时模型.md](../02-业务模块/03-AuthZ/07-Casbin运行时模型.md)。
