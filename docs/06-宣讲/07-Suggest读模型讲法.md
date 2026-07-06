# Suggest 读模型讲法

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

Suggest 是 Profile 联想搜索读模型，不是核心身份域。

讲解主线：

```text
query
  -> normalize
  -> index match
  -> ProfileAccessScope filter
  -> rank
  -> mobile mask
```

重点区分：

- Suggest 不拥有 Profile 写模型。
- ProfileAccessScope 不是 ProfileLink。
- 手机号搜索必须权限控制、脱敏和限流。

事实层入口：[../02-业务模块/05-Suggest/README.md](../02-业务模块/05-Suggest/README.md)。
