# IDP

## 30 秒结论

IDP 是外部身份源辅助模块。它管理外部身份提供方所需的应用配置、密钥、access token 获取与外部身份声明，供 AuthN 消费。

IDP 不创建 IAM 登录态，不签发 IAM Token，不拥有 User。

## 文档结构

| 文档 | 作用 |
| --- | --- |
| [00-模块总览.md](00-模块总览.md) | IDP 职责和边界 |
| [01-领域模型-WechatApp-Credentials-AppToken-ExternalIdentity.md](01-领域模型-WechatApp-Credentials-AppToken-ExternalIdentity.md) | 当前 IDP 模型 |
| [02-领域模型图.md](02-领域模型图.md) | 模型图 |
| [03-关键链路-微信应用配置与密钥轮换.md](03-关键链路-微信应用配置与密钥轮换.md) | WechatApp 配置和密钥治理 |
| [04-关键链路-微信AccessToken获取与缓存.md](04-关键链路-微信AccessToken获取与缓存.md) | access token 获取与缓存 |
| [05-关键链路-外部身份解析与AuthN协作.md](05-关键链路-外部身份解析与AuthN协作.md) | 外部身份声明如何进入 AuthN |
| [06-模块边界-IDP与AuthN.md](06-模块边界-IDP与AuthN.md) | 与 AuthN 的边界 |
| [07-分层架构与代码索引.md](07-分层架构与代码索引.md) | 代码事实源 |
