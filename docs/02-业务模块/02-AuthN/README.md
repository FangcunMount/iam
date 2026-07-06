# AuthN

## 30 秒结论

AuthN 是认证域，回答“如何证明当前请求者是某个系统用户”。

核心链路是：

```text
LoginIdentity / Credential / Challenge
  -> Principal
  -> Session
  -> AccessToken / RefreshToken
```

## 文档结构

| 文档 | 作用 |
| --- | --- |
| [00-模块总览.md](00-模块总览.md) | AuthN 职责和边界 |
| [01-领域模型-LoginIdentity-Credential-Challenge-Principal-Session-Token.md](01-领域模型-LoginIdentity-Credential-Challenge-Principal-Session-Token.md) | 认证核心模型 |
| [02-领域模型图.md](02-领域模型图.md) | 领域模型图 |
| [03-核心对象生命周期.md](03-核心对象生命周期.md) | 登录身份、凭证、挑战、会话和 Token 生命周期 |
| [04-关键链路-Onboarding身份开通.md](04-关键链路-Onboarding身份开通.md) | 首次身份开通 |
| [05-关键链路-Linking登录身份绑定.md](05-关键链路-Linking登录身份绑定.md) | 已认证用户绑定登录身份 |
| [06-关键链路-Login登录认证.md](06-关键链路-Login登录认证.md) | 登录到 Principal |
| [07-关键链路-Token签发刷新吊销.md](07-关键链路-Token签发刷新吊销.md) | Token 生命周期 |
| [08-关键链路-JWKS与本地验签.md](08-关键链路-JWKS与本地验签.md) | JWKS 发布和资源服务验签 |
| [09-模块边界-AuthN与Identity-IDP-AuthZ.md](09-模块边界-AuthN与Identity-IDP-AuthZ.md) | 跨模块边界 |
| [10-分层架构与代码索引.md](10-分层架构与代码索引.md) | 代码事实源 |
