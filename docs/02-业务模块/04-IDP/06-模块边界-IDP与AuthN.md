# 模块边界：IDP 与 AuthN

## IDP 负责

- 外部应用配置。
- 外部应用密钥。
- 外部 access token 获取。
- 外部身份 proof 解析准备。

## AuthN 负责

- LoginIdentity 映射。
- Credential / Challenge 校验。
- Principal 构造。
- Session / Token 签发。

## 禁止混淆

- `WechatApp` 不是 `LoginIdentity`。
- 微信 access token 不是 IAM AccessToken。
- openid / userid 不是 User 本体。
- 外部 proof 不是长期 Credential。
