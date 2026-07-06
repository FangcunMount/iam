# JWT / JWS / JWK / JWKS / KeyRotation

## 30 秒结论

- JWT 是 claims 的紧凑表达。
- JWS 是签名保护结构。
- JWK 是 JSON 密钥对象。
- JWKS 是公开验签公钥集合。
- KeyRotation 是签名密钥 active / grace / retired 生命周期治理。

## 模块回链

当前实现链路见 [../02-业务模块/02-AuthN/08-关键链路-JWKS与本地验签.md](../02-业务模块/02-AuthN/08-关键链路-JWKS与本地验签.md)。
