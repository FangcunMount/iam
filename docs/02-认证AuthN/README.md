# 02-认证 AuthN 文档总览

## 1. 模块定位

`02-认证AuthN` 用于说明 IAM 项目中 **认证（Authentication）模块** 的模型、链路、分层实现与代码事实源。

在当前 IAM 上下文中，AuthN 不建模业务账号。

AuthN 关注的是：

```text
谁是主体？
系统如何找到这个主体？
请求者如何证明自己控制某个登录身份？
认证成功后如何表达主体？
认证结果如何转换为访问凭证？
```

因此新版 AuthN 文档不再以旧的 `Account` 语义为中心，而是围绕以下核心对象展开：

```text
User / Principal
  └── LoginIdentity 0..N
        └── Credential 0..N

Challenge 独立承载短期认证挑战。
Session 承载服务端认证上下文。
AccessToken / RefreshToken 承载认证后的访问凭证。
JWT / JWS / JWK / JWKS / KeyRotation 承载 AccessToken 的安全表达、验签与密钥治理。
```

这套模型的核心目标是：

```text
区分 IAM 主体、登录身份、长期认证材料、短期认证挑战、认证结果表达、服务端认证上下文、访问凭证与密钥治理。
```

---

## 2. 核心模型一句话

### 2.1 User

`User` 是 IAM 中的稳定主体。

它回答：

```text
这个主体是谁？
```

---

### 2.2 LoginIdentity

`LoginIdentity` 是 User 绑定的登录身份。

它回答：

```text
系统可以通过什么登录标识找到这个 User？
```

例如：

```text
username + realm + username
phone + global + +E164
wechat_minip + appid + openid
wecom + corp_id + userid
```

---

### 2.3 Credential

`Credential` 是 IAM 需要保存并校验的长期认证材料。

它回答：

```text
系统保存了什么长期材料来校验这个登录身份？
```

例如：

```text
password hash
passkey public key credential
TOTP encrypted secret
recovery code hash
```

不是所有 `LoginIdentity` 都有 `Credential`。

微信、企微、手机号验证码等场景通常没有长期 Credential。

---

### 2.4 Challenge

`Challenge` 是短期认证挑战。

它回答：

```text
系统如何验证一次短期、可过期、可消费的证明？
```

例如：

```text
SMS OTP login
phone link OTP
oauth state
reset password code
```

Challenge 主要支撑：

- Login.phone_otp scene
- Linking.link_phone scene

---

### 2.5 Principal

`Principal` 是认证成功后的运行时主体表达，是 Login 的领域终点。

它回答：

```text
本次请求认证成功后，系统识别出的主体是谁？
通过哪个 LoginIdentity 认证？
使用了什么认证方式？
```

> 注意：
> Principal 是认证结果，不是 JWT，也不是 AccessToken。

---

### 2.6 Session / AccessToken / RefreshToken

- `Session` 是服务端认证上下文。
- `AccessToken` 是短期访问凭证。
- `RefreshToken` 是用于换取新 AccessToken 的续期凭证。

三者边界是：

> Session = 服务端认证状态
> AccessToken = 客户端访问资源时携带的短期凭证
> RefreshToken = 客户端向 IAM Token endpoint 换取新 AccessToken 的凭证

---

### 2.7 JWT / JWS / JWK / JWKS / KeyRotation

JWT / JWS / JWK / JWKS 属于 Token 安全表达和密钥治理相关概念。

简化理解：

```text
JWT = claims 的紧凑表达
JWS = 对 payload 的签名或 MAC 保护结构
JWK = JSON 密钥对象
JWKS = 一组 JWK，公开 JWKS endpoint 只应发布可公开的验签公钥
KeyRotation = 签名密钥 active / grace / retired 生命周期治理
```

---

## 3. 文档目录

当前 AuthN 文档体系如下：

```text
02-认证AuthN/
├── README.md
├── 00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md
├── 01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
├── 02-Linking链路-登录身份绑定解绑与安全边界.md
├── 03-Login链路-从登录请求到Principal.md
├── 04-Token链路-从Principal到AccessToken与RefreshToken.md
├── 05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md
├── 06-JWT-JWS-JWK-JWKS边界与KeyRotation.md
├── 07-第三方登录与IDP协作-WeChat-WeCom.md
└── 08-AuthN分层架构与事实源索引.md
```

目录顺序表达的是 AuthN 的模型生命周期：

```text
模型总览
  -> 首次开通 LoginIdentity
  -> 已认证 User 绑定/解绑 LoginIdentity
  -> 登录认证产出 Principal
  -> Principal 转换为 AccessToken / RefreshToken
  -> Session / Token 状态边界
  -> JWT/JWS/JWK/JWKS 与 KeyRotation
  -> WeChat / WeCom 外部 IDP 协作
  -> 分层架构与事实源索引
```

---

## 4. 阅读路径

### 4.1 第一次理解 AuthN 模块

建议顺序：

```text
00 -> 01 -> 02 -> 03 -> 04 -> 05 -> 06 -> 07 -> 08
```

原因：

```text
先建立模型，再理解 LoginIdentity 如何建立和维护，再理解 Login 如何产出 Principal，再理解 Token / Session / JWT / IDP，最后回到代码事实源。
```

---

### 4.2 只想理解核心模型

阅读：

```text
00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md
08-AuthN分层架构与事实源索引.md
```

重点掌握：

```text
User
LoginIdentity
Credential
Challenge
Principal
Session
AccessToken
RefreshToken
JWT / JWS / JWK / JWKS
```

---

### 4.3 想理解首次注册/开通

阅读：

```text
01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
```

它说明：

```text
requestPreparer
userResolver
loginIdentityEnsurer
credentialEnsurer
UnitOfWork
```

核心问题：

```text
如何首次建立 User + LoginIdentity？
什么时候创建 Credential？
为什么微信/企微通常不创建 Credential？
为什么外部 IDP 调用应在事务外？
```

---

### 4.4 想理解多登录身份绑定

阅读：

```text
02-Linking链路-登录身份绑定解绑与安全边界.md
```

它说明：

```text
已认证 User 如何绑定手机号 / 微信 / 企业微信；
手机号绑定为什么需要 link_phone Challenge；
如何防止 LoginIdentity 被跨 User 绑定；
为什么不能解绑最后一个 active LoginIdentity；
为什么敏感解绑需要 recent authentication。
```

---

### 4.5 想理解登录认证

阅读：

```text
03-Login链路-从登录请求到Principal.md
```

它说明：

```text
Login 如何证明 LoginIdentity；
ProofFactory 如何构造 authentication proof；
Authenticator 如何选择策略；
Password / Phone OTP / WeChat / WeCom 如何认证；
Principal 如何产生；
CredentialRecorder 如何记录 password 等 persisted Credential 的认证结果。
```

注意：

```text
Login 到 Principal 为止；
Token 链路从 Principal 开始。
```

---

### 4.6 想理解 Token 签发、刷新、撤销

阅读：

```text
04-Token链路-从Principal到AccessToken与RefreshToken.md
```

它说明：

```text
Principal 如何转换为 AccessToken / RefreshToken；
AccessToken 与 RefreshToken 的职责；
RefreshToken 如何存储、轮换、撤销；
TokenAudit 如何记录签发、刷新、撤销事件；
Local Verify / Online Verify 的边界。
```

---

### 4.7 想理解 Session / Token 状态边界

阅读：

```text
05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md
```

它说明：

```text
Principal 是认证结果；
Session 是服务端认证上下文；
AccessToken 是短期访问凭证；
RefreshToken 是续期凭证；
Logout / Revoke 与 Session 的关系；
Recent Authentication 与 Session 的关系。
```

---

### 4.8 想理解 JWT / JWKS / KeyRotation

阅读：

```text
06-JWT-JWS-JWK-JWKS边界与KeyRotation.md
```

它说明：

```text
JWT / JWS / JWK / JWKS 分别是什么；
kid / alg / typ 的职责是什么；
公开 JWKS endpoint 为什么只能发布可公开的验签公钥；
KeyRotation 为什么需要 active / grace / retired；
资源服务如何通过 JWKS 验签；
为什么验签方必须使用自己的算法白名单。
```

---

### 4.9 想理解微信 / 企业微信登录

阅读：

```text
07-第三方登录与IDP协作-WeChat-WeCom.md
01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
02-Linking链路-登录身份绑定解绑与安全边界.md
03-Login链路-从登录请求到Principal.md
```

因为第三方身份源会同时出现在：

```text
Onboarding
Linking
Login
```

Token 链路不直接参与 IDP proof。

---

### 4.10 要改 AuthN 代码

先读：

```text
08-AuthN分层架构与事实源索引.md
```

再根据具体能力进入对应文档。

---

## 5. 文档说明

### 5.1 `00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md`

这是模型总览文档。

它回答：

```text
User 是什么？
LoginIdentity 是什么？
Credential 是什么？
Challenge 是什么？
Principal 是什么？
Token / Session 在模型中处于什么位置？
为什么 IAM 不建业务 Account？
```

这是所有 AuthN 文档的基础。

---

### 5.2 `01-Onboarding链路-从身份开通到LoginIdentity与Credential.md`

这是首次开通链路文档。

它回答：

```text
如何从外部请求建立 User + LoginIdentity？
什么时候创建 Credential？
为什么第三方登录不创建 Credential？
为什么外部 IDP 调用应该在事务外？
```

核心流程：

```text
Prepare -> Resolve User -> Ensure LoginIdentity -> Ensure Credential -> OnboardingResult
```

---

### 5.3 `02-Linking链路-登录身份绑定解绑与安全边界.md`

这是登录身份绑定/解绑文档。

它回答：

```text
已认证 User 如何绑定新的 LoginIdentity？
手机号绑定为什么需要 link_phone Challenge？
微信/企微绑定如何证明外部身份？
如何防止 LoginIdentity 被跨 User 绑定？
为什么不能解绑最后一个 active LoginIdentity？
为什么敏感解绑需要 recent authentication？
```

核心边界：

```text
Linking 基于已认证 User；
Linking 不签发 Token；
Linking 维护的是 User 与 LoginIdentity 的关系。
```

---

### 5.4 `03-Login链路-从登录请求到Principal.md`

这是登录认证链路文档。

它回答：

```text
登录请求如何选择认证方式？
ProofFactory 如何构造认证证明？
Authenticator 如何选择策略？
Password / Phone OTP / WeChat / WeCom 如何认证？
Principal 如何产生？
CredentialRecorder 如何记录 Credential 认证结果？
```

核心流程：

```text
LoginCommand -> MethodSelector -> ProofFactory -> Authenticator -> AuthDecision -> CredentialRecorder(optional) -> Principal
```

Token 只是 Login 之后的边界调用：

```text
Principal -> TokenApplicationService -> TokenPair
```

完整 Token 链路见第 04 篇。

---

### 5.5 `04-Token链路-从Principal到AccessToken与RefreshToken.md`

这是 Token 应用链路文档。

它回答：

```text
Principal 如何转换为 AccessToken / RefreshToken？
AccessToken 与 RefreshToken 的边界是什么？
RefreshToken 为什么需要服务端控制点？
Refresh / Revoke / Logout 如何理解？
TokenAudit 如何记录 Token 事件？
Local Verify / Online Verify 的差异是什么？
```

核心流程：

```text
Principal -> Claims -> AccessToken -> RefreshToken -> TokenStore / SessionStore -> TokenPair
```

---

### 5.6 `05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md`

这是认证状态边界文档。

它回答：

```text
Principal 是什么？
Session 是什么？
AccessToken 是什么？
RefreshToken 是什么？
它们之间是什么关系？
Logout / Revoke 与 Session 有什么关系？
Recent Authentication 与 Session 有什么关系？
Session / Token 与 AuthZ 的边界是什么？
```

这篇不重复 Token 签发、刷新、撤销的完整应用链路。

---

### 5.7 `06-JWT-JWS-JWK-JWKS边界与KeyRotation.md`

这是 JWT/JWS/JWK/JWKS 与密钥轮换文档。

它回答：

```text
JWT / JWS / JWK / JWKS 分别是什么？
kid / alg / typ 的职责是什么？
公开 JWKS endpoint 为什么只能发布可公开的验签公钥？
KeyRotation 为什么需要 active / grace / retired？
资源服务如何通过 JWKS 验签？
为什么不能盲目信任 token header.alg？
```

---

### 5.8 `07-第三方登录与IDP协作-WeChat-WeCom.md`

这是第三方登录与外部身份源协作文档。

它回答：

```text
微信 code2session 如何映射为 LoginIdentity？
企业微信 code / auth_code 如何映射为 LoginIdentity？
为什么微信/企微不创建 Credential？
AppSecret、session_key、access_token 与 Credential 的边界是什么？
Onboarding / Linking / Login 中 IDP 的作用有何不同？
```

---

### 5.9 `08-AuthN分层架构与事实源索引.md`

这是分层架构与代码事实源索引文档。

它回答：

```text
Transport / Application / Domain / Infra 分别负责什么？
每条链路的事实源文件在哪里？
Repository / Port / 数据表 / 测试如何定位？
如何避免跨层污染？
AuthN 与 Identity / AuthZ / IDP / 业务系统的边界是什么？
```

---

## 6. 核心代码事实源总览

### 6.1 Domain 层

```text
internal/apiserver/domain/authn/loginidentity
internal/apiserver/domain/authn/credential
internal/apiserver/domain/authn/challenge
internal/apiserver/domain/authn/authentication
internal/apiserver/domain/authn/session
internal/apiserver/domain/idp/wechatapp
internal/apiserver/domain/identity/user
```

---

### 6.2 Application 层

```text
internal/apiserver/application/authn/signup
internal/apiserver/application/authn/linking
internal/apiserver/application/authn/signin
internal/apiserver/application/authn/session
internal/apiserver/application/authn/token
internal/apiserver/application/authn/challenge
internal/apiserver/application/authn/jwks
internal/apiserver/application/authn/credential
internal/apiserver/application/authn/uow
```

文档用语与代码包对照见 `internal/apiserver/application/authn/README.md`（含 RenewSession / RefreshToken 命名说明）。

---

### 6.3 Infra 层

```text
internal/apiserver/infra/mysql/loginidentity
internal/apiserver/infra/mysql/credential
internal/apiserver/infra/cache/redis/challenge_repository.go
internal/apiserver/infra/token
internal/apiserver/infra/token/keyset
internal/apiserver/infra/sms
internal/apiserver/infra/idp
```

---

### 6.4 Transport 层

```text
internal/apiserver/transport/rest/authn
internal/apiserver/transport/grpc
```

---

### 6.5 装配层

```text
internal/apiserver/container/assembler
internal/apiserver/container/assembler/capabilities.go
```

如果想理解某个应用服务使用了哪些仓储、IDP adapter、TokenStore、SessionManager，应优先查看装配层。

---

## 7. 数据表事实源

数据库 schema 事实源：

```text
internal/pkg/migration/migrations/000001_init_schema.up.sql
```

AuthN 关键表：

| 表 | 语义 |
| --- | --- |
| `users` | IAM User 主体，属于 Identity 事实源，但 AuthN 会引用 |
| `auth_login_identities` | User 与 LoginIdentity 的绑定 |
| `auth_credentials` | LoginIdentity 的长期认证材料 |
| `auth_token_audit` | Token 签发、刷新、撤销等审计记录 |
| `jwks_keys` | JWK/JWKS / KeyRotation 密钥记录 |
| `idp_wechat_apps` | 微信/企微应用配置与 secret 密文 |

注意：

```text
authz_* 表和 casbin_rule 属于 AuthZ 事实源，不属于 AuthN 事实源。
AuthN 文档不应把 authz_roles / authz_assignments / casbin_rule 当作认证模块数据表索引。
```

---

## 8. 设计边界总览

### 8.1 AuthN 不建业务 Account

AuthN 不建：

```text
OperatorAccount
CustomerAccount
DoctorAccount
GuardianAccount
```

业务身份应由业务系统、Identity/ProfileLink 或 AuthZ scope/role 表达。

AuthN 只建：

```text
User
LoginIdentity
Credential
Challenge
Principal
Session
AccessToken / RefreshToken
```

---

### 8.2 LoginIdentity 不等于 Credential

```text
LoginIdentity：系统如何找到 User？
Credential：系统保存什么长期材料来验证登录身份？
```

微信、企微、手机号通常只有 LoginIdentity，没有 Credential。

---

### 8.3 Challenge 不等于 Credential

```text
Challenge：短期、可过期、可消费。
Credential：长期、可反复校验、可维护生命周期。
```

SMS OTP 是 Challenge。

Password hash 是 Credential。

---

### 8.4 Principal 不等于 JWT / AccessToken

```text
Principal：领域认证结果。
JWT：Token 层对 Principal 的 claims 表达。
JWS：对 JWT claims 的签名保护。
AccessToken：客户端访问资源时携带的短期凭证。
```

Login 的终点是 Principal。

Token 链路从 Principal 开始。

---

### 8.5 Session 不等于 AccessToken

```text
Session：服务端认证上下文。
AccessToken：客户端携带的短期访问凭证。
RefreshToken：客户端向 IAM 换取新 AccessToken 的续期凭证。
```

Session 是服务端控制点。

AccessToken 可以短期自包含。

---

### 8.6 JWKS 不发布私钥

公开 JWKS endpoint 只发布可公开的验签公钥。

不能发布：

```text
private key material
HMAC / oct symmetric secret
AppSecret
RefreshToken secret
```

JWKS 是内部 KeyStore 的 public verification key set 投影，不是内部 KeyStore 全量导出。

---

## 9. 文档维护原则

### 9.1 文档以代码事实源为准

如果代码和文档不一致：

```text
先检查事实源代码；
再修正文档；
必要时补测试锁定语义。
```

---

### 9.2 修改模型前先修改模型文档

如果准备修改：

```text
LoginIdentity
Credential
Challenge
Principal
Session
AccessToken / RefreshToken
```

应先更新：

```text
00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md
08-AuthN分层架构与事实源索引.md
```

然后再更新对应链路文档。

---

### 9.3 修改链路时同步更新事实源索引

如果新增或重命名代码文件，应同步更新：

```text
对应链路文档的“代码事实源索引”
08-AuthN分层架构与事实源索引.md
README.md
```

---

### 9.4 不把旧目录边界带回新文档

新版目录已经确立：

```text
Login 到 Principal 为止；
Token 从 Principal 开始；
Challenge 是支撑能力，不再单独成篇；
JWT/JWS/JWK/JWKS 属于 Token 安全表达与密钥治理。
```

后续维护文档时，不要再写回：

```text
Login = Principal + Token 主链路
Challenge = 独立主链路文档
JWT/JWKS = Session 文档主内容
AuthZ 表 = AuthN 数据表事实源
```

---

## 10. 推荐讲解顺序

面试或项目讲解时，可以按这个顺序讲：

```text
1. IAM 不建业务 Account，只建 User / LoginIdentity / Credential / Challenge 等认证模型。
2. LoginIdentity 表示登录身份绑定，Credential 表示长期认证材料。
3. Onboarding 负责首次创建 User + LoginIdentity + optional Credential。
4. Linking 负责已认证 User 绑定/解绑更多 LoginIdentity。
5. Login 负责证明 LoginIdentity 并产出 Principal。
6. Token 链路负责把 Principal 转换为 AccessToken / RefreshToken。
7. Session 是服务端认证上下文，用于 refresh、logout、revoke、recent authentication。
8. Challenge 负责 SMS OTP 等短期证明，支撑 phone_otp login 和 link_phone linking。
9. JWT/JWS/JWK/JWKS 负责 AccessToken 的签名、验签、公钥发布和密钥轮换。
10. 分层架构保证模型、用例和技术实现解耦。
```

---

## 11. 外部标准参考

本文档组涉及以下外部标准和架构思想：

```text
NIST SP 800-63B：认证器生命周期、认证会话、OTP、reauthentication 等认证要求
RFC 7519：JSON Web Token (JWT)
RFC 7515：JSON Web Signature (JWS)
RFC 7517：JSON Web Key (JWK)
RFC 8725：JWT Best Current Practices
RFC 6749：OAuth 2.0 Framework
RFC 7009：OAuth 2.0 Token Revocation
Ports & Adapters / Hexagonal Architecture：应用核心与外部适配器解耦
```

这些标准不直接决定 IAM 项目代码结构，但用于校准术语边界和安全原则。

---

## 12. 维护备忘（与实现对齐）

| 主题 | 当前做法 |
| --- | --- |
| 开通 | 仅 `signup` 包；`onboarding/` 已删除 |
| 登录 | `signin` 实现，`session.Login` 门面；`login/` 已删除 |
| 续期命名 | 应用 `RenewSession`，HTTP/token 仍 `refresh_token` / `RefreshToken` |
| session 装配 | assembler 构建 `signin.SignIn` 后注入 session，门面不重复挂 Authenticator |
| 敏感再认证 | linking 用 `UnlinkCommand.AuthenticatedAt`；无独立 Reauthenticate；验票用 `token.VerifyToken` |

---

## 13. 最终总结

新版 AuthN 文档体系的核心是：

```text
用 User / LoginIdentity / Credential / Challenge 建立稳定模型；
用 Onboarding / Linking / Login 描述登录身份的建立、维护和认证；
用 Principal / Token / Session 描述认证结果、访问凭证和服务端状态；
用 JWT / JWS / JWK / JWKS / KeyRotation 描述 AccessToken 的安全表达和密钥治理；
用 IDP 文档说明微信/企微等外部身份源如何参与 AuthN；
用分层架构与事实源索引保证文档和代码一致。
```

一句话：

> AuthN 不再围绕业务 Account 展开，而是围绕 IAM 主体、登录身份、认证材料、短期挑战、认证结果、会话、Token 与密钥治理展开。
