# 关键链路：外部身份解析与 AuthN 协作

## 链路目标

通过外部 proof 得到外部身份声明，并交给 AuthN 映射 LoginIdentity。

## 链路

```text
external code / auth_code
  -> resolve app secret / app config
  -> call external IDP API
  -> obtain external identity
  -> build provider key
  -> AuthN resolves LoginIdentity
  -> AuthN builds Principal
```

## 关键边界

- IDP 只证明外部身份来源。
- AuthN 决定登录是否成功。
- Token 链路不直接依赖 IDP proof。
