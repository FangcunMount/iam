# 关键链路：Linking 登录身份绑定

## 链路目标

已认证 User 绑定或解绑新的 LoginIdentity。

## 链路

```text
authenticated Principal
  -> verify linking proof
  -> build ProviderKey / LoginIdentity key
  -> validate conflict
  -> bind or unbind LoginIdentity
```

## 关键边界

- Linking 需要已有 Principal。
- 绑定登录身份不是创建 ProfileLink。
- 解绑登录身份不等于删除 User。
