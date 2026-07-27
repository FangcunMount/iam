# IDP 与第三方登录讲法

> 状态：规划改造 · 已完成当前事实盘点；正文仍含待实现或尚未收敛的设计内容，不得作为现有能力承诺。

---

## 1. 本文目标

本文用于回答：

```text
IDP 模块和第三方登录在 IAM 中怎么讲？
```

它是宣讲稿，不是完整领域模型文档，适用于：

```text
面试讲解微信登录；
解释 IDP 与 AuthN 的边界；
解释 ExternalIdentity / LoginIdentity；
解释 provider access_token 与 IAM AccessToken；
解释为什么 openid / unionid / wecom userid 不是内部 User；
解释个人项目或业务系统如何接入第三方身份源。
```

本文采用金字塔表达：

```text
先一句话定位；
再讲第三方登录主链路；
再讲核心对象；
再讲 IDP 与 AuthN/Identity 的边界；
最后讲常见追问。
```

---

## 2. 一句话定位

IDP 是 IAM 的外部身份源基础设施模块，负责管理 provider app、credential、provider access token，并把微信、企业微信等外部身份声明解析成 IAM 可使用的 ExternalIdentity。

更短一点：

```text
IDP 负责解析外部身份事实，AuthN 负责把这些事实变成内部登录态。
```

---

## 3. 30 秒版本

```text
IDP 模块负责第三方身份源接入，比如微信小程序、公众号、企业微信等。它主要管理 provider app、credentials、provider access token，并把外部 code、openid、unionid、wecom userid 等 provider 侧身份声明解析成 ExternalIdentity。这里最重要的边界是：IDP 不直接创建 User，不签发 IAM Token，也不负责登录态；AuthN 会拿 ExternalIdentity 去匹配或创建 LoginIdentity，再生成 Principal、Session、AccessToken 和 RefreshToken。
```

---

## 4. 1 分钟版本

```text
IDP 是 IAM 里负责外部身份源接入的模块。比如微信登录时，客户端拿到 code 后，业务系统或 IAM 会把这个 code 交给 IDP。IDP 根据对应的 WechatApp、Credentials 和 provider access token 去调用微信接口，解析出 openid、unionid 或企业微信 userid 等外部身份声明，并把它们统一包装成 ExternalIdentity。

但 ExternalIdentity 还不是 IAM 的登录结果。真正的登录态由 AuthN 负责：AuthN 会根据 ExternalIdentity 找到或绑定内部 LoginIdentity，再构造 Principal，创建 Session，签发 IAM 的 AccessToken 和 RefreshToken。所以 IDP 和 AuthN 的边界很重要：IDP 只回答“外部 provider 说了什么”，AuthN 才回答“这个外部身份如何登录到 IAM”。
```

---

## 5. 3 分钟版本

```text
IDP 是 IAM 中的外部身份源基础设施模块。它存在的原因是：业务系统接入微信小程序、微信公众号、企业微信等第三方登录时，会遇到很多 provider 侧概念，比如 appid、secret、access_token、code、openid、unionid、session_key、wecom userid。如果这些概念直接散落在各个业务系统里，很容易把外部身份和内部用户混在一起，也很难做密钥管理、token 缓存和 provider 差异适配。

所以我把第三方身份源接入单独抽成 IDP 模块。IDP 主要负责四类事情。

第一是 ProviderApp，比如 WechatApp。它描述当前系统接入了哪个外部应用，比如微信小程序 app、公众号 app、企业微信 corp/app 等。

第二是 Credentials。它管理 provider app 对应的 app secret、corp secret 等外部凭据。这里要注意，IDP 的 Credentials 是 provider 凭据，不是 AuthN 的用户登录 Credential。

第三是 AppToken 或 provider access token。它用于调用 provider API，比如调用微信接口换取用户 openid。它不是 IAM 的 AccessToken，不能拿去访问 IAM 业务 API。

第四是 ExternalIdentity。它是 IDP 解析 provider 返回结果后得到的外部身份事实，比如 openid、unionid、wecom userid、provider、app_id 等。

第三方登录的完整链路应该是：客户端拿到 external code；IDP 用 provider app 和 credentials 解析 external identity；AuthN 根据 ExternalIdentity 匹配或绑定内部 LoginIdentity；然后 AuthN 构造 Principal，创建 Session，并签发 IAM AccessToken 和 RefreshToken。

这里的关键边界是：openid 不是 User，provider access_token 不是 IAM AccessToken，IDP 不签发 IAM Token，IDP 也不直接创建 User。外部身份源只提供外部事实，内部身份归 Identity 管，登录态归 AuthN 管，权限归 AuthZ 管。
```

---

## 6. 金字塔结构

### 6.1 顶层结论

```text
IDP 负责外部身份源事实解析，不负责内部登录态。
```

---

### 6.2 一条主链路

```text
external code / provider proof
  -> IDP resolve ExternalIdentity
  -> AuthN map LoginIdentity
  -> AuthN build Principal
  -> AuthN create Session
  -> AuthN issue AccessToken / RefreshToken
```

---

### 6.3 四个核心对象

| 对象 | 一句话 | 不是什么 |
| --- | --- | --- |
| `ProviderApp / WechatApp` | 外部 provider 应用配置 | 不是 LoginIdentity，不是 User |
| `Credentials` | provider app 的外部凭据 | 不是用户 Credential，不是 IAM Token |
| `AppToken / ProviderAccessToken` | 调用外部 provider API 的 token | 不是 IAM AccessToken，不进业务 API |
| `ExternalIdentity` | provider 返回的外部身份事实 | 不是 User，不是 Principal，不是 Subject |

---

### 6.4 三条核心边界

| 边界 | 说明 |
| --- | --- |
| IDP vs AuthN | IDP 解析外部身份事实，AuthN 建立内部登录态 |
| IDP vs Identity | IDP 不拥有 User/Profile 主数据，Identity 才是内部身份事实源 |
| IDP vs AuthZ | IDP 不做资源授权，AuthZ 才返回 AuthorizationDecision |

---

## 7. IDP 对象讲法

### 7.1 ProviderApp / WechatApp

讲法：

```text
ProviderApp 表示 IAM 接入的外部应用，例如微信小程序、微信公众号、企业微信应用等。它描述的是 provider 侧应用配置，而不是内部用户或登录身份。
```

重点：

```text
WechatApp 不是 User；
WechatApp 不是 LoginIdentity；
一个 provider app 下可以解析出很多 ExternalIdentity；
provider app 配置变化不等于用户身份变化。
```

---

### 7.2 Credentials

讲法：

```text
IDP 的 Credentials 是外部 provider 应用凭据，例如 app secret、corp secret 等，用来换取 provider access token 或调用 provider API。
```

重点：

```text
IDP Credentials 不是 AuthN Credential；
IDP Credentials 不属于用户登录材料；
secret 不进日志、不进响应、不进普通文档示例；
Credentials 需要支持轮换和安全存储，具体以实现为准。
```

---

### 7.3 AppToken / ProviderAccessToken

讲法：

```text
ProviderAccessToken 是调用外部 provider API 的访问令牌，比如微信 access_token。它的作用是让 IAM 调用微信接口，不是让业务系统访问 IAM API。
```

重点：

```text
微信 access_token 不是 IAM AccessToken；
provider access_token 不应该给普通业务 API 使用；
provider access_token 不应该进入 AuthZ；
provider access_token 生命周期和 IAM Token 生命周期不同。
```

---

### 7.4 ExternalIdentity

讲法：

```text
ExternalIdentity 是 IDP 从 provider 返回结果中解析出来的外部身份事实，比如 openid、unionid、wecom userid，以及 provider、app_id 等上下文。
```

重点：

```text
ExternalIdentity 不是 User；
ExternalIdentity 不是 Principal；
ExternalIdentity 不是 AuthZ Subject；
ExternalIdentity 需要交给 AuthN 做登录、绑定或开通；
openid/unionid/wecom userid 不应直接当内部 UserID。
```

---

## 8. 第三方登录链路讲法

标准链路：

```text
Client gets external code
  -> submit code to IAM / business backend
  -> IDP loads ProviderApp and Credentials
  -> IDP calls provider API
  -> IDP resolves ExternalIdentity
  -> AuthN finds or creates LoginIdentity，按当前用例决定
  -> AuthN builds Principal
  -> AuthN creates Session
  -> AuthN issues AccessToken / RefreshToken
```

讲解重点：

```text
code 是 provider proof；
ExternalIdentity 是外部身份事实；
LoginIdentity 是内部登录身份；
Principal 是认证结果；
AccessToken / RefreshToken 是 IAM 登录态凭证；
provider access_token 和 IAM AccessToken 完全不是一类 token。
```

边界：

```text
IDP 不直接签发 IAM Token；
IDP 不直接创建 User；
IDP 不做 AuthZ Check；
业务系统不直接把 openid 当 UserID。
```

---

## 9. 微信小程序登录讲法

可以这样讲：

```text
微信小程序登录时，客户端先拿到微信 code。IDP 根据这个 code 和对应的 WechatApp 配置调用微信接口，解析出 openid、unionid、session_key 等 provider 侧结果。然后 IDP 把这些信息封装成 ExternalIdentity。接下来进入 AuthN，AuthN 根据 ExternalIdentity 查找或绑定内部 LoginIdentity，认证成功后生成 Principal、Session、AccessToken 和 RefreshToken。
```

重点区分：

```text
code 不是登录态；
openid 不是 UserID；
unionid 不是 UserID；
session_key 不是 IAM Session；
微信 access_token 不是 IAM AccessToken；
ExternalIdentity 不是 Principal；
AuthN 产出的 AccessToken 才是访问 IAM 业务 API 的凭证。
```

---

## 10. 企业微信登录讲法

可以这样讲：

```text
企业微信登录和微信小程序登录类似，只是 provider 侧身份标识可能是 corp_id、agent_id、userid、external_userid 等。IDP 负责调用企业微信接口并解析这些外部身份事实，AuthN 再决定它们如何映射到内部 LoginIdentity 和 User。
```

重点区分：

```text
wecom userid 不是 IAM UserID；
corp access_token 不是 IAM AccessToken；
企业微信组织关系不等于 IAM RoleBinding；
企业微信身份解析不等于 IAM 授权通过。
```

---

## 11. IDP 与 AuthN 的边界

IDP 回答：

```text
外部 provider 说了什么？
这些 provider 结果如何被规范化成 ExternalIdentity？
```

AuthN 回答：

```text
这个 ExternalIdentity 能不能登录？
应该绑定到哪个 LoginIdentity？
是否需要 onboarding？
认证成功后如何产出 Principal / Session / Token？
```

正确关系：

```text
IDP ExternalIdentity
  -> AuthN LoginIdentity
  -> AuthN Principal
  -> AuthN Session / Token
```

禁止混用：

```text
IDP 直接签发 IAM AccessToken；
IDP 直接创建 Session；
IDP 直接返回 Principal；
AuthN 把 provider access_token 当 IAM AccessToken；
业务系统跳过 AuthN，直接信任 openid。
```

讲解句：

```text
IDP 是外部身份事实解析器，AuthN 是内部登录态生成器。
```

---

## 12. IDP 与 Identity 的边界

Identity 回答：

```text
内部 User 是谁？Profile 是什么？User 与 Profile 有什么关系？
```

IDP 回答：

```text
外部 provider 返回了哪个 openid / unionid / userid？
```

正确关系：

```text
ExternalIdentity
  -> AuthN login/link/onboarding use case
  -> LoginIdentity
  -> User
  -> Identity facts
```

禁止混用：

```text
ExternalIdentity 直接等于 User；
openid 直接作为 UserID；
IDP provider claims 直接写 ProfileLink；
IDP 直接修改 Profile 主数据；
provider 组织关系直接当 IAM 身份关系事实。
```

讲解句：

```text
IDP 提供外部身份事实，Identity 才维护内部身份事实。
```

---

## 13. IDP 与 AuthZ 的边界

AuthZ 回答：

```text
Subject 能不能对 Resource 执行 Action？
```

IDP 回答：

```text
外部身份源返回了什么身份声明？
```

正确关系：

```text
ExternalIdentity
  -> AuthN Principal
  -> AuthZ Subject
  -> AuthZ Check
```

禁止混用：

```text
openid 命中就直接授权；
企业微信部门关系直接当 RoleBinding；
provider claims 直接生成 Permission；
IDP 直接调用 Casbin；
IDP 直接返回 AuthorizationDecision。
```

讲解句：

```text
外部身份可信，不代表有内部资源访问权；资源访问仍要 AuthZ Check。
```

---

## 14. IDP 与 Provider Token 的边界

provider token 与 IAM token 是两类完全不同的 token。

| Token | 来源 | 用途 | 是否访问 IAM 业务 API |
| --- | --- | --- | --- |
| provider access_token | 微信/企业微信等 provider | 调用 provider API | 否 |
| session_key | 微信小程序等 provider | provider 侧会话/解密能力，具体以 provider 语义为准 | 否 |
| IAM AccessToken | IAM AuthN | 访问 IAM / 业务系统 API | 是 |
| IAM RefreshToken | IAM AuthN | 换取新的 IAM AccessToken | 否，不能进普通业务 API |

核心句：

```text
微信 access_token 是 IAM 调微信用的，IAM AccessToken 是业务系统调 IAM/业务 API 用的。
```

---

## 15. 典型业务场景讲法

### 15.1 微信小程序一键登录

```text
小程序拿到 code；
IDP 调微信接口解析 openid/unionid；
AuthN 根据 openid/unionid 查 LoginIdentity；
如果已绑定，生成 Principal 并签发 Token；
如果未绑定，进入注册/绑定/onboarding 流程，具体以当前实现为准。
```

重点：

```text
openid 是外部身份；
LoginIdentity 是内部登录身份；
User 是内部身份事实；
Token 由 AuthN 签发。
```

---

### 15.2 企业微信扫码登录

```text
用户通过企业微信完成 provider 侧认证；
IDP 解析企业微信 userid/corp 信息；
AuthN 查找或绑定内部 LoginIdentity；
生成 Principal；
签发 IAM Token；
后续业务访问继续走 AuthZ。
```

重点：

```text
企业微信认证成功不等于 IAM 授权成功；
企业微信身份要映射为内部 LoginIdentity；
资源访问仍然需要 AuthZ Check。
```

---

### 15.3 多端统一身份

```text
同一个内部 User 可以绑定手机号、微信小程序、企业微信等多个 LoginIdentity；
不同 provider 返回的 ExternalIdentity 最终通过 AuthN 归并到内部 User；
Identity 维护 User 与 Profile 的关系；
AuthZ 基于 Subject 做权限判断。
```

重点：

```text
多个外部身份可以归并到一个内部 User；
外部身份不应该直接成为内部主键；
登录方式扩展不应该污染 Identity 主模型。
```

---

## 16. 面试追问展开点

| 追问 | 回答要点 |
| --- | --- |
| IDP 和 AuthN 有什么区别？ | IDP 解析外部身份事实，AuthN 生成内部登录态和 Token |
| openid 是不是 UserID？ | 不是。openid 是 provider 侧标识，需要映射到 LoginIdentity/User |
| 微信 access_token 是不是 IAM AccessToken？ | 不是。微信 access_token 用来调微信 API，IAM AccessToken 用来访问 IAM/业务 API |
| ExternalIdentity 是不是 Principal？ | 不是。ExternalIdentity 是外部身份事实，Principal 是认证成功后的内部上下文 |
| IDP 能不能直接创建 User？ | 不建议。应通过 AuthN onboarding/linking use case 明确编排 |
| 企业微信组织关系能不能直接当权限？ | 不能。外部组织事实可以作为输入，但内部授权仍要 AuthZ RoleBinding/Permission |
| 为什么要单独做 IDP 模块？ | 避免 provider 差异、密钥、token、外部身份解析散落在业务系统和 AuthN 主流程中 |

---

## 17. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| openid 直接当 UserID | 外部标识污染内部身份 | ExternalIdentity -> LoginIdentity -> User |
| 微信 access_token 当 IAM AccessToken | token 语义混淆 | provider token 只用于 provider API |
| IDP 直接签发 IAM Token | 模块职责混乱 | Token 签发归 AuthN |
| IDP 直接创建 User | 外部事实直接污染内部事实 | 通过 AuthN onboarding/linking 编排 |
| ExternalIdentity 当 Principal | 外部身份事实和认证结果混淆 | AuthN 生成 Principal |
| 企业微信部门当 RoleBinding | 外部组织关系和内部授权混淆 | AuthZ 显式建 RoleBinding/Permission |
| provider secret 打日志 | 凭据泄露 | secret 脱敏和安全存储 |
| 业务系统直接调 provider 后自建用户 | 身份归并混乱 | 统一通过 IDP/AuthN 接入 |
| IDP 直接做权限判断 | 认证授权混淆 | 资源访问走 AuthZ Check |
| session_key 当 IAM Session | provider 会话和 IAM Session 混淆 | IAM Session 由 AuthN 管理 |

---

## 18. 推荐表达顺序

讲 IDP 时建议按这个顺序：

```text
1. 先说 IDP 是外部身份源基础设施；
2. 说明它回答“provider 说了什么”，不是“内部登录态是什么”；
3. 讲 ProviderApp / Credentials / ProviderAccessToken / ExternalIdentity；
4. 讲第三方登录主链路；
5. 重点区分 openid、ExternalIdentity、LoginIdentity、User、Principal；
6. 重点区分 provider access_token 和 IAM AccessToken；
7. 回到 AuthN 生成 Token、AuthZ 做授权。
```

不推荐：

```text
一上来只讲微信接口；
把 openid 讲成用户；
把微信 access_token 讲成系统 token；
把 IDP 讲成登录态模块；
把企业微信组织关系直接讲成权限；
忽略 AuthN 和 AuthZ 的后续链路。
```

---

## 19. 事实源回链

| 内容 | 事实源 |
| --- | --- |
| IDP 模块 | [../02-业务模块/04-IDP/README.md](../02-业务模块/04-IDP/README.md) |
| 外部身份解析与 AuthN 协作 | [../02-业务模块/04-IDP/04-关键链路-外部身份解析与AuthN协作.md](../02-业务模块/04-IDP/04-关键链路-外部身份解析与AuthN协作.md) |
| IDP 与 AuthN 边界 | [../02-业务模块/04-IDP/05-模块边界-IDP与AuthN.md](../02-业务模块/04-IDP/05-模块边界-IDP与AuthN.md) |
| AuthN 讲法 | [04-AuthN讲法.md](04-AuthN讲法.md) |
| AuthN 模块 | [../02-业务模块/02-AuthN/README.md](../02-业务模块/02-AuthN/README.md) |
| Identity 讲法 | [03-Identity讲法.md](03-Identity讲法.md) |
| AuthZ 讲法 | [05-AuthZ讲法.md](05-AuthZ讲法.md) |
| 接入契约 | [../03-接入与契约/README.md](../03-接入与契约/README.md) |

---

## 20. Verify

修改本文后至少执行：

```bash
make docs-hygiene
```

如果同步修改 IDP / AuthN 相关代码或契约，需要执行：

```bash
go test ./internal/apiserver/domain/idp/...
go test ./internal/apiserver/application/idp/...
go test ./internal/apiserver/application/authn/...
make api-validate
make proto-gen
go test ./internal/pkg/architecture
```

---

## 21. 本文总结

IDP 与第三方登录讲法可以压缩成：

```text
IDP 管外部身份源；
ProviderApp 表示外部应用配置；
Credentials 表示 provider 凭据；
ProviderAccessToken 用来调用 provider API；
ExternalIdentity 表示外部身份事实；
AuthN 把 ExternalIdentity 映射成 LoginIdentity / Principal / Session / Token；
Identity 管内部 User / Profile；
AuthZ 管资源访问权限。
```

宣讲时最重要的是：

```text
把 provider 侧概念和 IAM 内部概念分开；
把 ExternalIdentity 和 LoginIdentity 分开；
把 provider access_token 和 IAM AccessToken 分开；
把外部身份认证和内部资源授权分开。
```
