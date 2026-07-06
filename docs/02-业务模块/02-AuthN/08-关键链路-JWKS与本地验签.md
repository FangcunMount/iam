# 关键链路：JWKS 与本地验签

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

让资源服务可以通过 JWKS 本地验证 AccessToken 签名。

## 链路

```text
KeyStore
  -> active signing key
  -> sign AccessToken
  -> publish public JWK in JWKS
  -> resource service fetches JWKS
  -> verify token by kid + alg allowlist
```

## 关键边界

- JWKS 只发布可公开的验签公钥。
- 私钥不进入 JWKS。
- 资源服务不能盲信 token header 中的 `alg`。
- 设计取舍见 [../../05-专题设计/01-JWT-JWS-JWK-JWKS-KeyRotation.md](../../05-专题设计/01-JWT-JWS-JWK-JWKS-KeyRotation.md)。
