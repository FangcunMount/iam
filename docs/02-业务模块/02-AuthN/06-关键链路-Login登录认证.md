# 关键链路：Login 登录认证

## 链路目标

把登录请求证明为一个 `Principal`。

## 链路

```text
login request
  -> parse authentication proof
  -> resolve LoginIdentity
  -> verify Credential / Challenge / external proof
  -> evaluate User and LoginIdentity status
  -> build Principal
```

## 关键边界

- Login 的领域终点是 Principal。
- Token 签发是后续链路。
- 授权角色不在 Login 链路中判定。
