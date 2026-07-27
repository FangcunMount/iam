# AuthN 讲法

> 状态：规划改造 · 已完成当前事实盘点；正文仍含待实现或尚未收敛的设计内容，不得作为现有能力承诺。

---

## 1. 本文目标

本文用于回答：

```text
AuthN 模块在 IAM 中负责什么？
```

它是宣讲稿，不是完整领域模型文档，适用于：

```text
面试讲解 AuthN；
解释登录认证链路；
解释 LoginIdentity / Credential / Challenge / Principal；
解释 Session / AccessToken / RefreshToken / JWKS；
解释认证与授权的边界；
解释为什么 AuthN 不是简单 JWT demo。
```

本文采用金字塔表达：

```text
先一句话定位；
再讲认证主链路；
再讲核心对象；
再讲 Token 与会话边界；
再讲 AuthN 与 Identity/AuthZ/IDP 的边界；
最后讲常见追问。
```

---

## 2. 一句话定位

AuthN 是 IAM 的认证中心，负责验证调用者提供的登录凭据或外部身份事实，产出 Principal、Session、AccessToken 和 RefreshToken，回答“当前请求者如何证明自己是谁”。

更短一点：

```text
AuthN 管认证，不管资源授权。
```

---

## 3. 30 秒版本

```text
AuthN 模块负责登录认证。它把 LoginIdentity、Credential、Challenge、Principal、Session 和 Token 分开建模：LoginIdentity 表达一种登录身份，Credential 表达长期认证材料，Challenge 表达短期认证挑战，Principal 表达认证成功后的主体上下文，Session 表达服务端登录会话状态，AccessToken 和 RefreshToken 分别用于短期 API 访问和续期。这里最重要的边界是：AuthN 只证明“你是谁”，不判断“你能访问什么资源”，资源访问要继续走 AuthZ Check。
```

---

## 4. 1 分钟版本

```text
AuthN 是 IAM 的认证中心，核心问题是“当前请求者如何证明自己是谁”。我把它拆成几类对象：LoginIdentity 表示登录身份，比如手机号、微信小程序 openid、企业微信 userid、后台账号等；Credential 表示长期认证材料，比如密码、OTP 绑定材料、OAuth provider 标识等；Challenge 表示短期认证挑战，比如验证码、二维码状态、一次性登录挑战；认证成功后生成 Principal，表示当前请求者的认证结果。

在会话和 Token 上，AuthN 会创建 Session，并签发 AccessToken 和 RefreshToken。AccessToken 是短期访问凭证，可以通过 JWKS 给业务系统本地验签；RefreshToken 是续期凭证，只能用于 refresh/logout 等 AuthN 接口，不能进入普通业务 API。这里一定要区分认证和授权：Token 验签成功只代表请求者身份可信，后续资源访问仍然需要 AuthZ 根据 Subject、Resource、Action、Scope 做 Check。
```

---

## 5. 3 分钟版本

```text
AuthN 是 IAM 里的认证中心。它解决的不是“用户有哪些权限”，而是“当前请求者如何证明自己是谁”。如果 Identity 是身份事实中心，那么 AuthN 就是把登录方式、认证材料、挑战流程和认证结果组织起来的一层。

我把 AuthN 的主线拆成六类对象。

第一是 LoginIdentity。它表示一个 User 可以通过什么方式登录，比如手机号、微信小程序、企业微信、后台账号等。LoginIdentity 不是 User，它只是 User 的登录入口。这样设计的好处是，一个内部 User 可以绑定多个登录身份，而外部 openid、unionid、wecom userid 不会直接污染内部 User 模型。

第二是 Credential。Credential 表示长期认证材料，比如密码哈希、手机号 OTP 相关材料、OAuth provider 标识等。Credential 不是登录身份本身，而是 LoginIdentity 可用于认证的材料。这样可以支持一个登录身份拥有多种认证方式，也方便后续扩展和安全治理。

第三是 Challenge。Challenge 是短期认证挑战，比如验证码、扫码登录状态、一次性校验流程等。它和 Credential 的区别是：Credential 通常是长期材料，Challenge 是短期、一次性或有时效的认证过程。

第四是 Principal。认证成功后，AuthN 不应该直接把 User entity 或 JWT 当作结果，而是产出 Principal。Principal 表示“这次请求已经被认证为谁”，后续可以映射成 AuthZ 的 Subject，也可以作为业务请求上下文的一部分。

第五是 Session。Session 是服务端认证会话状态，表达一次登录会话是否仍有效、是否已登出、是否还能继续刷新 Token。Session 不是普通 API 的 Bearer Token。

第六是 Token。AccessToken 是短期访问凭证，用于普通业务 API；RefreshToken 是续期凭证，只用于刷新 AccessToken；JWKS 用于发布公钥，让业务系统能够本地验签 AccessToken。这里的关键边界是：AccessToken 验签成功只说明认证通过，不代表授权通过。业务系统拿到 Principal 后，还要进入 AuthZ，用 Resource、Action、Scope 做访问决策。

所以 AuthN 不是简单发一个 JWT，而是把登录身份、认证材料、认证挑战、认证结果、会话状态和 Token 生命周期分开治理的认证中心。
```

---

## 6. 金字塔结构

### 6.1 顶层结论

```text
AuthN 负责证明调用者是谁。
```

---

### 6.2 一条主链路

```text
LoginIdentity / Credential / Challenge
  -> authenticate
  -> Principal
  -> Session
  -> AccessToken / RefreshToken
  -> JWKS local verification，若是 AccessToken
```

---

### 6.3 六个核心对象

| 对象 | 一句话 | 不是什么 |
| --- | --- | --- |
| `LoginIdentity` | 一种登录身份入口 | 不是 User，不是 Credential |
| `Credential` | 长期认证材料 | 不是登录结果，不是 Challenge |
| `Challenge` | 短期认证挑战 | 不是长期凭据，不是 Session |
| `Principal` | 认证成功后的主体上下文 | 不是 JWT，不是完整 User entity，不是 AuthZ Subject 本体 |
| `Session` | 服务端登录会话状态 | 不是普通业务 API Bearer Token |
| `AccessToken / RefreshToken` | 访问凭证与续期凭证 | AccessToken 不等于 RefreshToken，Token 不等于权限 |

---

### 6.4 三条核心边界

| 边界 | 说明 |
| --- | --- |
| AuthN vs Identity | AuthN 管认证，Identity 管身份事实 |
| AuthN vs AuthZ | AuthN 证明是谁，AuthZ 判断能做什么 |
| AuthN vs IDP | IDP 解析外部身份源，AuthN 使用解析结果完成登录/绑定/开通 |

---

## 7. AuthN 对象讲法

### 7.1 LoginIdentity

讲法：

```text
LoginIdentity 表示一个内部 User 可以通过哪种方式登录。比如手机号登录、微信小程序登录、企业微信登录、后台账号登录，本质上都是不同类型的 LoginIdentity。
```

重点：

```text
LoginIdentity 不是 User；
一个 User 可以有多个 LoginIdentity；
openid/unionid/wecom userid 应先进入 LoginIdentity 或 ExternalIdentity 映射；
不要把外部 provider 标识直接当 UserID。
```

---

### 7.2 Credential

讲法：

```text
Credential 表示认证材料，比如密码哈希、OTP 相关材料、OAuth provider 标识等。它回答的是“这个 LoginIdentity 用什么材料证明自己”。
```

重点：

```text
Credential 是长期认证材料；
Credential 不等于 LoginIdentity；
Credential 不等于 Challenge；
Credential 不应泄露原始密码、secret、token；
Credential 校验属于 AuthN，不属于 Identity。
```

---

### 7.3 Challenge

讲法：

```text
Challenge 表示短期认证挑战，比如短信验证码、扫码登录状态、一次性校验流程。它解决的是认证过程中的短期交互问题。
```

重点：

```text
Challenge 有时效；
Challenge 通常只能使用一次或有限次数；
Challenge 不应当作长期凭据；
Challenge 成功后才能继续产出 Principal。
```

---

### 7.4 Principal

讲法：

```text
Principal 是认证成功后的主体上下文，表达“这次请求已经被认证为谁”。它是 AuthN 的输出，不是 JWT 本身，也不是完整 User entity。
```

重点：

```text
Principal 是认证结果；
Principal 可进入请求上下文；
Principal 可映射为 AuthZ Subject；
Principal 不直接表达资源权限；
Principal 不等于 User，也不等于 JWT claims 的全部内容。
```

---

### 7.5 Session

讲法：

```text
Session 是服务端认证会话状态，表示一次登录会话是否仍有效、是否已登出、是否还能继续刷新 Token。
```

重点：

```text
Session 是服务端状态；
Session 不进入普通业务 API；
Session 状态会影响 RefreshToken 是否可继续续期；
是否让已签发 AccessToken 即时失效，需要黑名单、introspection 或短 TTL 策略配合。
```

---

### 7.6 AccessToken / RefreshToken

讲法：

```text
AccessToken 是短期 API 访问凭证，RefreshToken 是续期凭证。普通业务 API 只应该接受 AccessToken，RefreshToken 只能进入 AuthN 的 refresh/logout/revoke 链路。
```

重点：

```text
AccessToken 生命周期短；
RefreshToken 生命周期更长但服务端可吊销；
RefreshToken 不进入普通业务 API；
AccessToken 可通过 JWKS 本地验签；
Token 验签成功不代表授权通过。
```

---

## 8. 登录认证链路讲法

标准链路：

```text
Login request
  -> identify LoginIdentity
  -> verify Credential or Challenge
  -> build Principal
  -> create Session
  -> issue AccessToken
  -> issue RefreshToken
  -> return token response
```

讲解重点：

```text
LoginIdentity 先确定登录入口；
Credential/Challenge 完成认证；
Principal 是认证结果；
Session 记录服务端会话状态；
AccessToken 用于访问普通 API；
RefreshToken 用于续期。
```

边界：

```text
登录成功不代表拥有所有资源权限；
登录成功后访问资源仍要 AuthZ Check；
AuthN 不直接决定业务资源是否可读、可写、可删、可导出。
```

---

## 9. Token 刷新链路讲法

标准链路：

```text
Refresh request
  -> submit RefreshToken
  -> validate RefreshToken
  -> check Session state
  -> rotate RefreshToken，若实现支持
  -> issue new AccessToken
  -> return token response
```

讲解重点：

```text
RefreshToken 不是重新登录；
RefreshToken 必须受 Session 状态控制；
登出或吊销后不能继续刷新；
RefreshToken 建议支持轮换和重放检测，具体以实现为准。
```

边界：

```text
RefreshToken 不应该被业务系统当 Bearer Token；
RefreshToken 不应该进入普通业务 API；
RefreshToken 泄露风险比 AccessToken 更高。
```

---

## 10. JWKS 与本地验签讲法

讲法：

```text
AccessToken 如果采用 JWT/JWS，AuthN 使用私钥签名，业务系统通过 JWKS 获取公钥并本地验签。这样普通业务 API 不需要每次都请求 AuthN 校验 Token，可以降低认证中心运行时压力。
```

标准链路：

```text
AccessToken header contains kid
  -> business service fetches JWKS
  -> find public key by kid
  -> verify signature
  -> validate iss / aud / exp / nbf
  -> build Principal context
  -> call AuthZ Check
```

重点：

```text
JWKS 只发布公钥；
private key 不外泄；
验签必须校验 iss/aud/exp/nbf；
验签通过只代表认证成功；
资源访问仍要 AuthZ Check。
```

---

## 11. AuthN 与 Identity 的边界

Identity 回答：

```text
User 是谁？Profile 是什么？User 和 Profile 是什么关系？
```

AuthN 回答：

```text
当前请求者如何证明自己是谁？
```

正确关系：

```text
LoginIdentity
  -> AuthN authenticate
  -> Principal/UserID
  -> Identity User/Profile/ProfileLink facts
```

禁止混用：

```text
把 Credential 放进 Identity；
让 Identity 处理 password/otp 校验；
把 openid 直接当 UserID；
把 User entity 当 Principal；
把 ProfileLink 当认证结果。
```

讲解句：

```text
Identity 提供身份事实，AuthN 负责认证过程。
```

---

## 12. AuthN 与 AuthZ 的边界

AuthN 回答：

```text
你是谁？
```

AuthZ 回答：

```text
你能做什么？
```

正确关系：

```text
AccessToken verified
  -> Principal
  -> map to Subject
  -> AuthZ Check(Resource, Action, Scope)
  -> AuthorizationDecision
```

禁止混用：

```text
Token 验签成功直接放行；
JWT claims 承载完整权限系统；
Principal 当 Permission；
RefreshToken 当访问权限；
AuthN 直接写 RoleBinding；
AuthN 直接决定资源可读/可写/可删。
```

讲解句：

```text
AuthN 只给出可信身份上下文，AuthZ 才给出资源访问决策。
```

---

## 13. AuthN 与 IDP 的边界

IDP 回答：

```text
外部 provider 说了什么？这些外部身份事实是否可信？
```

AuthN 回答：

```text
这些外部身份事实如何用于登录、绑定或开通 IAM 内部身份？
```

正确关系：

```text
provider proof
  -> IDP resolve ExternalIdentity
  -> AuthN login/link/onboarding
  -> LoginIdentity / Principal / Token
```

禁止混用：

```text
IDP 直接签发 IAM Token；
IDP 直接创建 User；
AuthN 直接把 provider access_token 当 IAM AccessToken；
业务系统直接用 openid 当 Principal。
```

讲解句：

```text
IDP 解析外部身份事实，AuthN 决定这些事实如何进入内部认证链路。
```

---

## 14. 典型业务场景讲法

### 14.1 手机号密码登录

```text
手机号对应一个 LoginIdentity；
密码哈希是 Credential；
AuthN 校验 Credential；
校验通过后产出 Principal；
创建 Session；
签发 AccessToken 和 RefreshToken。
```

重点：

```text
手机号不是 User 本身；
密码校验属于 AuthN；
登录成功后访问资源仍要 AuthZ。
```

---

### 14.2 微信小程序登录

```text
微信小程序 code 先通过 IDP 或 provider adapter 解析成 ExternalIdentity；
AuthN 根据 ExternalIdentity 找到或绑定 LoginIdentity；
认证成功后产出 Principal；
签发 IAM AccessToken / RefreshToken。
```

重点：

```text
openid 不是 IAM UserID；
provider access_token 不是 IAM AccessToken；
外部身份解析和内部认证结果要分开。
```

---

### 14.3 业务 API 访问

```text
业务请求携带 AccessToken；
业务系统通过 JWKS 本地验签；
验签成功得到 Principal；
再映射成 AuthZ Subject；
执行 Resource / Action / Scope 的 AuthZ Check。
```

重点：

```text
AccessToken 只解决认证；
AuthZ Check 才解决授权；
不要把 JWT claims 当完整权限。
```

---

## 15. 面试追问展开点

| 追问 | 回答要点 |
| --- | --- |
| LoginIdentity 和 User 有什么区别？ | LoginIdentity 是登录入口，User 是内部身份锚点，一个 User 可以有多个 LoginIdentity |
| Credential 和 Challenge 有什么区别？ | Credential 是长期认证材料，Challenge 是短期认证挑战 |
| Principal 是不是 JWT？ | 不是。Principal 是认证结果上下文，JWT/AccessToken 是它的一种访问凭证表达 |
| AccessToken 和 RefreshToken 为什么分开？ | AccessToken 用于短期 API 访问，RefreshToken 只用于续期，风险和生命周期不同 |
| RefreshToken 能不能调业务 API？ | 不应该。普通业务 API 只接受 AccessToken |
| JWKS 解决什么问题？ | 让业务系统用公钥本地验签 AccessToken，降低 AuthN 运行时依赖 |
| 验签成功是不是就有权限？ | 不是。验签是认证，资源访问还要 AuthZ Check |
| IDP 和 AuthN 怎么分工？ | IDP 解析外部身份事实，AuthN 使用这些事实完成登录/绑定/开通 |

---

## 16. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| AuthN 只是发 JWT | 忽略登录身份、凭据、会话、刷新、密钥治理 | 拆分 LoginIdentity / Credential / Session / Token |
| openid 直接当 UserID | 外部标识污染内部身份 | ExternalIdentity/LoginIdentity -> User |
| Credential 放进 Identity | 模块职责混乱 | Credential 归 AuthN |
| Principal 当 JWT | 认证结果和凭证表达混淆 | Principal 是上下文，JWT 是表达 |
| RefreshToken 调普通 API | 长期凭证暴露面扩大 | 普通 API 只接受 AccessToken |
| Token 验签后直接授权 | 认证授权混淆 | 验签后 AuthZ Check |
| JWKS 暴露私钥 | 严重安全事故 | JWKS 只发布公钥 |
| JWT claims 存复杂权限 | 权限更新困难且易越权 | 权限由 AuthZ 决策 |
| 登出只删前端 token | 服务端仍可刷新 | revoke Session/RefreshToken |
| 日志打印完整 Token | 凭证泄露 | 日志脱敏或禁止记录 |

---

## 17. 推荐表达顺序

讲 AuthN 时建议按这个顺序：

```text
1. 先说 AuthN 是认证中心；
2. 说明它回答“你是谁”，不是“你能做什么”；
3. 讲 LoginIdentity / Credential / Challenge；
4. 讲 Principal 是认证结果；
5. 讲 Session / AccessToken / RefreshToken；
6. 讲 JWKS 与本地验签；
7. 回到 AuthZ Check 边界；
8. 用手机号登录或微信登录举例。
```

不推荐：

```text
一上来只讲 JWT；
把 AuthN 讲成用户表查询；
把 openid 讲成 UserID；
把 Token claims 讲成权限系统；
把 RefreshToken 和 AccessToken 混用；
把认证和授权一起说成“鉴权”。
```

---

## 18. 事实源回链

| 内容 | 事实源 |
| --- | --- |
| AuthN 模块 | [../02-业务模块/02-AuthN/README.md](../02-业务模块/02-AuthN/README.md) |
| Token 签发刷新吊销 | [../02-业务模块/02-AuthN/05-关键链路-Token签发刷新吊销.md](../02-业务模块/02-AuthN/05-关键链路-Token签发刷新吊销.md) |
| JWKS 与本地验签 | [../02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md](../02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md) |
| JWT/JWKS 专题 | [../05-专题设计/01-JWT-JWS-JWK-JWKS-KeyRotation.md](../05-专题设计/01-JWT-JWS-JWK-JWKS-KeyRotation.md) |
| Session/双 Token 专题 | [../05-专题设计/02-Session-AccessToken-RefreshToken边界.md](../05-专题设计/02-Session-AccessToken-RefreshToken边界.md) |
| Identity 讲法 | [03-Identity讲法.md](03-Identity讲法.md) |
| AuthZ 模块 | [../02-业务模块/03-AuthZ/README.md](../02-业务模块/03-AuthZ/README.md) |
| IDP 模块 | [../02-业务模块/04-IDP/README.md](../02-业务模块/04-IDP/README.md) |

---

## 19. Verify

修改本文后至少执行：

```bash
make docs-hygiene
```

如果同步修改 AuthN 相关代码或契约，需要执行：

```bash
go test ./internal/apiserver/domain/authn/...
go test ./internal/apiserver/application/authn/...
go test ./internal/apiserver/application/authn/token/...
make api-validate
make proto-gen
go test ./internal/pkg/architecture
```

---

## 20. 本文总结

AuthN 讲法可以压缩成：

```text
AuthN 是认证中心；
LoginIdentity 表示登录身份；
Credential 表示长期认证材料；
Challenge 表示短期认证挑战；
Principal 表示认证结果；
Session 表示服务端会话状态；
AccessToken 用于短期 API 访问；
RefreshToken 只用于续期；
JWKS 支持业务系统本地验签；
认证成功不等于授权通过。
```

宣讲时最重要的是：

```text
把“证明你是谁”和“判断你能做什么”分开；
把 LoginIdentity / Credential / Principal / Session / Token 分开；
用 AccessToken / RefreshToken / JWKS 体现认证安全设计深度。
```
