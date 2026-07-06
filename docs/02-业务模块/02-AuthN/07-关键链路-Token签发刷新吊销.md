# 关键链路：Token 签发、刷新、吊销

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

把 Principal 转换为可携带的访问凭证，并维护服务端认证上下文。

## 链路

```text
Principal
  -> create Session
  -> issue AccessToken
  -> issue RefreshToken
  -> refresh AccessToken
  -> revoke Session / RefreshToken
```

## 关键边界

- AccessToken 是短期访问凭证。
- RefreshToken 只用于向 IAM 换取新 AccessToken。
- Session 是服务端认证状态。
- 设计取舍见 [../../05-专题设计/02-Session-AccessToken-RefreshToken边界.md](../../05-专题设计/02-Session-AccessToken-RefreshToken边界.md)。
