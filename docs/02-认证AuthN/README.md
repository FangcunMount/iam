# 02-认证 AuthN 文档总览

## 1. 模块定位

`02-认证AuthN` 用于说明 IAM 项目中 **认证（Authentication）模块** 的模型、链路、分层实现与代码事实源。

在当前 IAM 上下文中，AuthN 不建模业务账号。

AuthN 关注的是：

```text
谁是主体？
系统如何找到这个主体？
请求者如何证明自己控制某个登录身份？
认证成功后如何表达主体并签发访问凭证？
```

因此新版 AuthN 文档不再以旧的 `Account` 语义为中心，而是围绕以下核心对象展开：

```text
User / Principal
  └── LoginIdentity 0..N
        └── Credential 0..N

Challenge 独立承载短期认证挑战。
Session / Token 承载认证后的访问上下文。
JWKS / KeyRotation 承载 JWT 签名密钥治理。
```

这套模型的核心目标是：

```text
区分 IAM 主体、登录身份、长期认证材料、短期认证挑战、认证结果表达。
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

---

### 2.5 Principal

`Principal` 是认证成功后的运行时主体表达。

它回答：

```text
本次请求认证成功后，系统识别出的主体是谁？
通过哪个 LoginIdentity 认证？
使用了什么认证方式？
```

---

### 2.6 Session / Token

`Session` 是服务端认证上下文。

`Access Token` 是短期访问凭证。

`Refresh Token` 是换取新 Access Token 的凭证。

JWT/JWS/JWKS 是 Token 的安全表达和验签基础设施，不是 AuthN 领域模型本身。

---

## 3. 文档目录

当前 AuthN 文档体系如下：

```text
02-认证AuthN/
├── README.md
├── 00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md
├── 01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
├── 02-Login链路-从登录请求到Principal与Token.md
├── 03-Linking链路-登录身份绑定解绑与安全边界.md
├── 04-Challenge链路-短信验证码与短期认证挑战.md
├── 05-Session与Token边界-Principal-Session-JWT-RefreshToken.md
├── 06-JWT-JWS-JWKS与KeyRotation.md
├── 07-第三方登录与IDP协作-WeChat-WeCom.md
└── 08-AuthN分层架构与事实源索引.md
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
先建立模型，再理解链路，再理解 Token/JWKS，最后回到代码事实源。
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
Token
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

---

### 4.4 想理解登录认证

阅读：

```text
02-Login链路-从登录请求到Principal与Token.md
05-Session与Token边界-Principal-Session-JWT-RefreshToken.md
06-JWT-JWS-JWKS与KeyRotation.md
```

它们分别说明：

```text
Login 如何证明 LoginIdentity
Principal 如何进入 Token
JWT/JWS/JWKS 如何完成签名与验签
```

---

### 4.5 想理解多登录身份绑定

阅读：

```text
03-Linking链路-登录身份绑定解绑与安全边界.md
04-Challenge链路-短信验证码与短期认证挑战.md
```

它们说明：

```text
已认证 User 如何绑定手机号 / 微信 / 企业微信
手机号绑定为什么需要 link_phone Challenge
为什么不能解绑最后一个 active LoginIdentity
```

---

### 4.6 想理解微信 / 企业微信登录

阅读：

```text
07-第三方登录与IDP协作-WeChat-WeCom.md
01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
02-Login链路-从登录请求到Principal与Token.md
03-Linking链路-登录身份绑定解绑与安全边界.md
```

因为第三方身份源会同时出现在：

```text
Onboarding
Login
Linking
```

---

### 4.7 要改 AuthN 代码

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

### 5.3 `02-Login链路-从登录请求到Principal与Token.md`

这是登录认证链路文档。

它回答：

```text
登录请求如何选择认证方式？
ProofFactory 如何构造认证证明？
Authenticator 如何选择策略？
Password / Phone OTP / WeChat / WeCom 如何认证？
Principal 如何产生？
Token 如何签发？
```

核心流程：

```text
LoginCommand -> MethodSelector -> ProofFactory -> Authenticator -> Principal -> Token
```

---

### 5.4 `03-Linking链路-登录身份绑定解绑与安全边界.md`

这是登录身份绑定/解绑文档。

它回答：

```text
已认证 User 如何绑定新的 LoginIdentity？
手机号绑定为什么需要 Challenge？
微信/企微绑定如何证明外部身份？
如何防止 LoginIdentity 被跨 User 绑定？
为什么不能解绑最后一个 active LoginIdentity？
```

---

### 5.5 `04-Challenge链路-短信验证码与短期认证挑战.md`

这是短期认证挑战文档。

它回答：

```text
为什么 SMS OTP 不是 Credential？
Challenge 的 scene / target / secret hash / TTL / consumed_at 分别是什么？
如何创建、发送、校验、消费 SMS OTP？
login 与 link_phone scene 如何隔离？
```

---

### 5.6 `05-Session与Token边界-Principal-Session-JWT-RefreshToken.md`

这是认证结果表达文档。

它回答：

```text
Principal 是什么？
Session 是什么？
Access Token 与 Refresh Token 的边界是什么？
TokenStore 与 SessionManager 分别负责什么？
Logout / Revoke / Refresh / ReAuthenticate 如何理解？
```

---

### 5.7 `06-JWT-JWS-JWKS与KeyRotation.md`

这是 JWT/JWS/JWKS 和密钥轮换文档。

它回答：

```text
JWT / JWS / JWK / JWKS 分别是什么？
kid / alg / typ 的职责是什么？
JWKS 为什么只能发布公钥？
KeyRotation 为什么需要 active / grace / retired？
资源服务如何通过 JWKS 验签？
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
Onboarding / Login / Linking 中 IDP 的作用有何不同？
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
internal/apiserver/application/authn/onboarding
internal/apiserver/application/authn/login
internal/apiserver/application/authn/linking
internal/apiserver/application/authn/challenge
internal/apiserver/application/authn/token
internal/apiserver/application/authn/session
internal/apiserver/application/authn/jwks
internal/apiserver/application/authn/uow
```

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
```

如果想理解某个应用服务使用了哪些仓储、IDP adapter、TokenStore、SessionManager，应优先查看装配层。

---

## 7. 数据表事实源

数据库 schema 事实源：

```text
internal/pkg/migration/migrations/000001_init_schema.up.sql
```

核心表：

| 表 | 语义 |
| --- | --- |
| `users` | IAM User 主体 |
| `auth_login_identities` | User 与 LoginIdentity 的绑定 |
| `auth_credentials` | LoginIdentity 的长期认证材料 |
| `auth_token_audit` | Token 签发与撤销审计 |
| `jwks_keys` | JWKS / KeyRotation 密钥记录 |
| `idp_wechat_apps` | 微信/企微应用配置与 secret 密文 |
| `authz_roles` | 授权角色 |
| `authz_assignments` | User / Group 的角色赋权 |
| `casbin_rule` | Casbin 策略规则 |

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

业务身份应由业务系统或 AuthZ 表达。

AuthN 只建：

```text
User
LoginIdentity
Credential
Challenge
Principal
Session
Token
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

### 8.4 Principal 不等于 JWT

```text
Principal：领域认证结果。
JWT：Token 层对 Principal 的 claims 表达。
JWS：对 JWT claims 的签名保护。
```

---

### 8.5 JWKS 不发布私钥

JWKS 只发布公钥。

私钥只能由 Token 签发组件使用。

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

## 10. 推荐讲解顺序

面试或项目讲解时，可以按这个顺序讲：

```text
1. IAM 不建业务 Account，只建 User / LoginIdentity / Credential / Challenge。
2. LoginIdentity 表示登录身份绑定，Credential 表示长期认证材料。
3. Onboarding 负责首次创建 User + LoginIdentity + optional Credential。
4. Login 负责证明 LoginIdentity 并产出 Principal / Token。
5. Linking 负责已认证 User 绑定/解绑更多 LoginIdentity。
6. Challenge 负责 SMS OTP 等短期证明。
7. Session / Token 承载认证成功后的访问上下文。
8. JWT/JWS/JWKS 负责 Token 的签名、验签和密钥轮换。
9. 分层架构保证模型、用例和技术实现解耦。
```

---

## 11. 外部标准参考

本文档组涉及以下外部标准和架构思想：

```text
NIST SP 800-63B：认证器生命周期、认证器绑定、OTP 等认证要求
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

## 12. 最终总结

新版 AuthN 文档体系的核心是：

```text
用 User / LoginIdentity / Credential / Challenge 建立稳定模型；
用 Onboarding / Login / Linking / Challenge 描述核心链路；
用 Principal / Session / Token / JWT / JWKS 描述认证结果表达；
用分层架构与事实源索引保证文档和代码一致。
```

一句话：

> AuthN 不再围绕业务 Account 展开，而是围绕 IAM 主体、登录身份、认证材料、短期挑战、会话与 Token 展开。
