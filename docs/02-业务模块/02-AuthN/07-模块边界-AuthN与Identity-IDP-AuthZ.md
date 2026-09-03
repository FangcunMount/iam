# 模块边界：AuthN 与 Identity、IDP、AuthZ

> 状态：已实现 · 本文解释认证主线跨模块时传递什么、不传递什么，以及为什么这些边界能防止身份、证明和权限混成一个模型。

## 1. 结论

```text
IDP 证明外部账号
  -> AuthN 绑定/认证 LoginIdentity
  -> Identity 提供长期 User 状态
  -> AuthN 产生 Principal，并颁发 AuthenticationGrant(Session + UserTokenSet)
  -> AuthZ 对 Subject 做资源判定
```

跨模块传递的应该是最小事实或端口结果，而不是让一个模块直接操作另一个模块的仓储。

## 2. AuthN 与 Identity

### User 与 LoginIdentity

User 是长期业务主体，LoginIdentity 是可登录入口。一对多关系让同一 User 可以有用户名、手机号、微信、企微等入口，并允许单独禁用或解绑其中一个。

AuthN 当前会读取/写入 User，主要发生在：

- SignUp 的 UoW 中创建或复用 User；
- `AdmissionPolicy` 通过 Identity 的 `UserStatusReader` 检查 User active/blocked；
- User 状态变化后，Identity 通过 outbox 驱动按 User 撤销 Session。

这并不意味着 User 属于 AuthN。User 的状态不变量和 Profile 关系仍由 Identity 定义；AuthN 只是通过仓储端口和 UoW 完成跨聚合用例。

### Session 状态不是 User 状态

User blocked 后已有 Session 可能尚在 Redis；Session revoke 是派生收敛动作。在线 Verify 还会重新检查 User/LoginIdentity 状态，因此即使异步批量撤销尚未完成，
也不应仅凭 Session active 放行。

### 禁止的耦合

- 在 User 表增加每种 provider 的 openid/credential；
- 通过删除 User 来表达登录身份解绑；
- 把 ProfileLink 当 LoginIdentity；
- transport 直接跨模块拼多个 repository 写入而绕过 UoW。

## 3. AuthN 与 IDP

IDP 管理 provider 应用配置、AppSecret 和 provider AppToken，并通过窄 `ExternalIdentity Resolver` 完成应用查询、密钥解密和 code exchange。
AuthN 只提交 provider/realm/code、消费标准结果，再决定这个外部身份用于 Login、Linking 还是 SignUp。

| IDP 概念 | AuthN 概念 | 为什么不同 |
| --- | --- | --- |
| WechatApp | LoginIdentity realm 的配置来源 | 应用不是人 |
| IDP Credentials/AppSecret | AuthN Credential | 前者证明 IAM 有权调用 provider，后者证明用户控制登录入口 |
| AppAccessToken | IAM AccessToken | 前者调用微信 API，后者访问业务服务 |
| openid/unionid/userid | LoginIdentity ProviderKey 的组成 | 外部标识还未映射为 IAM User |
| provider code/state | 一次性 proof/challenge | 不能长期保存为 credential |

外部 API 调用必须在本地数据库事务外完成，返回 request-local、不可直接登录的 `ExternalIdentity`。AuthN 把它映射为既有认证输入或 ProviderKey，再在短事务中 ensure 本地事实。

### 信任边界

公共 REST SignUp 只接受 appID + jsCode，由 IDP Resolver 查配置、解密 AppSecret 并调用 provider；它不应直接信任调用方提供的 openid。
应用层内部保留的预解析 openid/unionid 分支标记为请求内 `TrustedLegacyInput`，不会伪装为 provider 已验证结果，也不进入公共协议、数据库 Metadata 或 `VerifiedAt`。

## 4. AuthN 与 AuthZ

AuthN 的身份证明阶段产生 Principal，在线 SignIn 用例还会将其颁发为 `AuthenticationGrant`。AuthZ 不消费 Grant 或 Session 写模型，
而从可信 Principal/token claims 构造 Subject。典型转换只取：

```text
Principal.UserID -> subject.NewUserRef(UserID)
```

JWT middleware 可以把 UserID、tenant domain、org、AMR 等放进请求上下文，但资源访问仍需 AuthZ 路由守卫或 Check。

### 为什么 role snapshot 不等于授权

当前 AuthN Token 不携带完整 Assignment / PermissionGrant / ConstraintSet。即使下游自行缓存 role/permission snapshot，也可能在签发或缓存后过期；
撤权不会修改已经签名的 JWT。这类派生信息可用于界面导航，但不能替代需要最新策略的服务端判定。

### 最近认证不是授权

`auth_time`/AMR 可以证明认证新鲜度或方式，适合敏感操作前置条件；它仍不回答主体是否有执行目标操作的权限。安全流程可能需要：

```text
AuthN recent-auth check
AND AuthZ resource check
```

而不是二选一。

## 5. AuthN 与 Suggest

Suggest 从 JWT 上下文读取 user/tenant/org，构造最小 `visibility.Principal`，再通过 AuthZ facts 与 visibility reader 解析
`visibility.Scope`。它不读取 Credential、Challenge、Session 或 provider secret。

AuthN 只负责让请求上下文拥有可信身份；Suggest 自己负责关键词安全、scope 过滤和手机号脱敏。

## 6. 跨模块协作的三种形式

### 同事务 UoW

仅在同一 MySQL、确实需要原子提交时使用，例如 SignUp 的 User/LoginIdentity/Credential。事务所有权必须在应用服务中明确。

### 同步端口

用于当前请求必须拿到结果的能力，例如 AuthN 调 IDP provider adapter 解析 code、AdmissionPolicy 通过 `UserStatusReader` 读取 User 状态。端口暴露业务结果，
不暴露具体 SDK/GORM。

### Outbox/event

用于事实提交后驱动异步收敛，例如 User 禁用后批量撤销 Session。事件不应被当成权威事实，消费者必须幂等并具备重试/观测。

## 7. 边界漂移检查

修改前询问：

1. 新字段是在描述人、登录入口、外部应用还是权限？
2. 它的生命周期由哪个模块决定？
3. 是权威事实、一次性证明、运行时快照还是派生投影？
4. 需要同事务，还是同步查询/异步事件已足够？
5. 基础设施失败时应 fail-closed、降级为空还是允许继续？
6. 下游会不会把 snapshot/claim 误用成权威事实？

## 8. 事实源

- AuthN/Identity 状态检查：`domain/authn/admission/policy.go`、`application/authn/admission/guard.go`
- SignUp UoW：`application/authn/signup`、`infra/mysql/uow/authn`
- IDP proof：`application/authn/signin/proof`、`application/idp/externalidentity`
- AuthN 组合根：`container/authn/application.go`、`container/authn/infra.go`
- AuthZ Subject：`domain/authz/subject`
- request context：`internal/pkg/requestctx`、`internal/pkg/middleware/authn`
