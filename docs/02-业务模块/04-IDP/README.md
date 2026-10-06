# IDP：外部身份源接入

> 状态：已实现 · 当前设计与实现。 本页是 IDP 阅读与维护入口，源码事实与运行验收分别记录。

IDP 管理微信/企微应用配置、敏感凭据、provider AppToken 和外部交换，返回请求内 ExternalIdentity；AuthN 再决定该外部身份在 IAM 中用于开通、登录还是绑定。IDP 不创建 User/Session，不签发 IAM Token，不决定资源授权。

## 主题主文

| 问题 | 维护归属 |
| --- | --- |
| 为什么独立 IDP、能力和对象怎样分组 | [模块总览](00-模块总览.md) |
| WechatApp/Credentials、加密、轮换与 AppToken cache 的精确规则 | [应用凭据与 AppToken 缓存](01-应用凭据与AppToken缓存.md) |
| Resolver来源合同、三provider实际适配、标识/时间交接、state与最终用例错误 | [外部身份解析与 AuthN 协作](02-外部身份解析与AuthN协作.md) |
| 外部键/global比较、三用例归属差异、失效联动、账户合并与OIDC/broker迁移取舍 | [外部身份信任模型与方案演化](03-外部身份信任模型与方案演化.md) |
| 实际capability出口、装配/注册与健康、集合动作/AppID、敏感协议/SDK及修改验收链 | [模块边界与代码索引](04-模块边界与代码索引.md) |

同一行为只在主文讲透，其他页保留必要摘要和回链。完整类型、协议字段以源码、OpenAPI/proto 为准，不能从理想方案推导当前已实现能力。

## 三种材料的最短区别

| 材料 | 当前用途 |
| --- | --- |
| AppSecret / CorpSecret | IAM 代表 provider app 调 API |
| provider AppAccessToken | provider 签发给应用的短期调用凭证 |
| ExternalIdentity | 一次 code exchange 的标准化外部标识 |

它们分别区别于 AuthN 长期 Credential、IAM access/refresh token，以及 IAM UserID。一个对象的加密、缓存或类型有效，不能自动建立其他信任保证。

## 当前限制与证据

IDP 当前是本地 AES-256-GCM、无CAS的整体应用更新，以及 IAM/微信SDK两层缓存。Auth/Msg版本不是master key版本；显式Refresh可命中SDK旧token，外层ExpiresAt为本地两小时估计，loser reread只看非空，短lease未续租/fence。AppSecret与消息AESKey加密，CallbackToken明文；API材料轮换为空实现。gRPC GetWechatApp返回明文AppSecret，token RPC返回provider token；保密与目标AppID准入需按入口核对。具体当前规则与候选合同由[凭据主文](01-应用凭据与AppToken缓存.md)维护，WeCom真实装配及其他proof链由[解析主文](02-外部身份解析与AuthN协作.md)维护。

解析成功与AuthN用例成功也不同：WeCom仅OpenUserID结果可进入登录查询，却不能按当前绑定规则建ProviderKey；VerifiedAt与最终错误码各用例不同。标准企微adapter传nil cache，pinned SDK在请求期构造panic；mini/open的ctx传递也不相同。具体触发条件、源码推论与替身测试边界由[解析主文](02-外部身份解析与AuthN协作.md)维护，初始化成功不证明三个provider都可接受。

登录、注册和绑定并非完全相同的归属查找；canonical只保存一个provider/global锚点，不保存每条附加入口的完整外部关联。应用停用不直接撤销已有Session；OIDC/broker和账户合并仍是候选，具体输入/结果与迁移条件见[信任模型](03-外部身份信任模型与方案演化.md)。

AuthN当前只提取Resolver，但deps仍收到完整模块指针；传输capabilities另导出管理服务、repo/Vault。集合动作或方法ACL未形成逐AppID授权，静态health/注册不证明provider可用；当前装配、启动条件、secret出口与SDK迁移见[边界索引](04-模块边界与代码索引.md)。

代码入口：`internal/apiserver/domain/idp`、`application/idp`、`container/idp`；adapter 为 `infra/mysql/wechatapp`、`infra/cache/redis`、`infra/crypto`、`infra/wechat`与`infra/wechatapi`，前缀均为 `internal/apiserver/`。

```bash
make docs-hygiene docs-facts
go test ./internal/apiserver/domain/idp/... ./internal/apiserver/application/idp/...   ./internal/apiserver/infra/mysql/wechatapp ./internal/apiserver/container/idp
```

这些是验证入口；provider 实际行为、迁移状态和生产消费者证据另行核对。跨模块阅读：[AuthN](../02-AuthN/README.md)、[密码学](../../03-基础设施/04-密码学密钥与令牌.md)、[接口与 SDK](../../04-接口与SDK/README.md)。
