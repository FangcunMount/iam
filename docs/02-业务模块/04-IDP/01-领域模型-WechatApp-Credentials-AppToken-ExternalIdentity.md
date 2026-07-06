# 领域模型：WechatApp / Credentials / AppToken / ExternalIdentity

## WechatApp

`WechatApp` 是外部微信应用配置聚合，包含 appID、名称、类型、状态和凭据集合。

## Credentials

`Credentials` 表达外部应用凭据，包括认证 secret、消息回调 secret、API 安全通道密钥等。

## AppAccessToken

`AppAccessToken` 是外部微信 API 的应用 access token。它不是 IAM AccessToken。

## ExternalIdentity

外部身份是 IDP 通过外部 proof 解析出的 openid、unionid、userid 等声明。AuthN 使用这些声明映射 `LoginIdentity`。

## 边界

- IDP app secret 不进入 AuthN Credential。
- 外部 access token 不进入 IAM Token。
- 外部身份声明不直接等于 User。
