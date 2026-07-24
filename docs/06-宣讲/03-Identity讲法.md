# Identity 讲法

> 状态：设计目标 · 宣讲第一版，已按金字塔结构重写；后续需要继续结合 `internal/apiserver/domain/identity`、`application/identity`、REST/gRPC 契约、Identity 模块文档和测试逐项核对。

---

## 1. 本文目标

本文用于回答：

```text
Identity 模块在 IAM 中负责什么？
```

它是宣讲稿，不是完整领域模型文档，适用于：

```text
面试讲解 Identity；
解释 User / Profile / ProfileLink；
解释 Identity 与 AuthN / AuthZ 的边界；
解释为什么 ProfileLink 不是 Permission；
解释业务系统为什么需要统一身份事实中心。
```

本文采用金字塔表达：

```text
先一句话定位；
再讲核心对象；
再讲关键链路；
再讲边界；
最后讲常见追问。
```

---

## 2. 一句话定位

Identity 是 IAM 的身份事实中心，负责统一管理 User、Profile 和 ProfileLink，回答“用户是谁、档案是什么、用户和档案之间是什么关系”。

更短一点：

```text
Identity 管身份事实，不管登录过程，也不直接决定资源权限。
```

---

## 3. 30 秒版本

```text
Identity 模块负责 IAM 中最基础的身份事实。它把 User、Profile 和 ProfileLink 分开建模：User 是稳定身份主体，Profile 是业务档案或被服务对象，ProfileLink 表示 User 和 Profile 之间的关系。这里最重要的边界是：Identity 只回答“谁是谁、谁和哪个档案有关系”，不负责登录认证，也不直接做权限决策；登录认证交给 AuthN，资源访问判断交给 AuthZ。
```

---

## 4. 1 分钟版本

```text
Identity 是 IAM 的身份事实中心，我主要把它拆成三个核心对象：User、Profile 和 ProfileLink。User 表示系统中的稳定身份主体，例如家长、医生、运营人员等；Profile 表示业务档案或被服务对象，例如儿童档案、患者档案；ProfileLink 表示 User 和 Profile 之间的身份关系，比如监护关系、档案归属关系或服务关系，具体关系类型以当前业务模型为准。

这个模块的重点不是登录，也不是权限，而是把身份事实统一起来。AuthN 会基于登录身份认证出 Principal，AuthZ 会基于 Subject、Resource、Action、Scope 做权限判断，而 Identity 提供的是这些判断所需的基础身份事实。尤其要注意，ProfileLink 只是身份关系事实，不等于 Permission，也不能直接当作授权结果。
```

---

## 5. 3 分钟版本

```text
Identity 是 IAM 里最基础的身份事实模块。它解决的问题是：业务系统里到底有哪些人、有哪些档案、人和档案之间是什么关系。如果这个事实不统一，后面的登录、授权、搜索、外部身份源接入都会变得混乱。

我把 Identity 里最核心的对象拆成三个：User、Profile 和 ProfileLink。

第一，User 是稳定身份主体。它代表系统里一个真实或业务上的使用者，比如家长、医生、运营人员、机构员工等。User 不是登录方式本身，也不是微信 openid。一个 User 可以绑定多个 LoginIdentity，比如手机号、微信小程序、企业微信等，这部分属于 AuthN 的登录身份模型。

第二，Profile 是业务档案或被服务对象。在儿童互联网医疗场景里，Profile 可以理解成儿童档案、患者档案或其他被服务对象。它不是登录账号，也不一定能主动登录系统。业务系统真正操作很多资源时，往往是在操作某个 Profile 相关的数据。

第三，ProfileLink 是 User 和 Profile 之间的身份关系。比如某个家长和某个儿童档案之间存在监护关系，某个工作人员和某个档案之间存在服务关系。ProfileLink 的核心价值是把“人”和“档案”之间的关系建模清楚。

这里最重要的边界是：ProfileLink 不是 Permission。它只能说明 User 和 Profile 有某种关系，但不能直接推出这个用户可以读取、修改、删除、导出某个资源。真正的资源访问要进入 AuthZ，由 Subject、Resource、Action、Scope 和 Permission/RoleBinding 做 Check。

所以 Identity 的定位非常清楚：它是身份事实中心。AuthN 负责认证，回答“当前请求者如何证明自己是谁”；AuthZ 负责授权，回答“这个主体能不能对某个资源执行某个动作”；Suggest 可以读取 Identity facts 派生搜索读模型，但 Suggest 不拥有 Profile 主数据。Identity 保持稳定、清晰，后面的认证、授权和搜索才有可靠基础。
```

---

## 6. 金字塔结构

### 6.1 顶层结论

```text
Identity 是身份事实中心。
```

---

### 6.2 三个核心对象

| 对象 | 一句话 | 不是什么 |
| --- | --- | --- |
| `User` | 稳定身份主体 | 不是登录方式，不是 openid，不是权限主体的完整替代 |
| `Profile` | 业务档案或被服务对象 | 不是登录账号，不是搜索索引，不是权限声明 |
| `ProfileLink` | User 与 Profile 的身份关系 | 不是 Permission，不是 RoleBinding，不是 AuthZ Check 结果 |

---

### 6.3 三条边界

| 边界 | 说明 |
| --- | --- |
| Identity vs AuthN | Identity 管身份事实，AuthN 管登录认证 |
| Identity vs AuthZ | Identity 管关系事实，AuthZ 管资源访问决策 |
| Identity vs Suggest | Identity 管 Profile 主数据，Suggest 管搜索读模型 |

---

### 6.4 一条主链路

```text
User
  -> ProfileLink
  -> Profile
  -> 被 AuthN / AuthZ / Suggest 按需引用
```

---

## 7. Identity 对象讲法

### 7.1 User

讲法：

```text
User 是 IAM 中稳定的身份主体，它代表系统中的一个人或业务身份锚点。User 不等于手机号，也不等于微信 openid，因为手机号、openid、企业微信 userid 都只是登录身份或外部身份标识，最终应该归并到内部统一的 User 上。
```

重点：

```text
User 是内部身份锚点；
LoginIdentity 才表达登录方式；
ExternalIdentity 才表达外部 provider 身份事实；
Subject 才表达授权主体；
不要把 openid/unionid 直接当 UserID。
```

---

### 7.2 Profile

讲法：

```text
Profile 是业务档案或被服务对象。在儿童医疗场景里，它通常对应儿童档案或患者档案。很多业务资源不是直接挂在 User 上，而是围绕 Profile 展开，比如测评、问诊、训练记录、随访记录等。
```

重点：

```text
Profile 是业务档案；
Profile 不等于登录账号；
Profile 不等于搜索索引；
Profile 不等于权限；
Profile 主数据归 Identity 管理。
```

---

### 7.3 ProfileLink

讲法：

```text
ProfileLink 表示 User 和 Profile 之间的关系，例如监护关系、归属关系、服务关系等。它解决的是“这个用户和这个档案是什么关系”，不是“这个用户能做什么操作”。
```

重点：

```text
ProfileLink 是身份关系事实；
ProfileLink 可以作为 AuthZ 判断的事实输入；
ProfileLink 不能直接替代 Permission；
ProfileLink 不能直接替代 RoleBinding；
ProfileLink 不能直接写成 Casbin g rule。
```

---

## 8. Identity 与 AuthN 的边界

AuthN 回答：

```text
当前请求者如何证明自己是谁？
```

Identity 回答：

```text
这个内部 User 是谁？
这个 User 有哪些 Profile 关系？
```

正确关系：

```text
LoginIdentity / Credential
  -> AuthN login
  -> Principal
  -> UserID
  -> Identity User / Profile / ProfileLink facts
```

禁止混用：

```text
把手机号当 User；
把微信 openid 当 User；
把 Credential 放进 Identity；
让 Identity 直接处理 password / otp 校验；
把 Principal 当完整 User entity。
```

讲解句：

```text
Identity 不处理登录过程，它只提供登录成功后可以引用的内部身份事实。
```

---

## 9. Identity 与 AuthZ 的边界

AuthZ 回答：

```text
某个 Subject 能不能对某个 Resource 执行某个 Action？
```

Identity 回答：

```text
User 与 Profile 是否存在某种身份关系？
```

正确关系：

```text
Identity ProfileLink
  -> as identity fact
  -> AuthZ Check may use it
  -> AuthorizationDecision
```

禁止混用：

```text
ProfileLink 即 Permission；
User 即 Subject；
Profile 即 Resource 的完整替代；
有监护关系就允许所有操作；
把 Identity 表当权限表；
把 ProfileLink 直接写成 Casbin g rule。
```

讲解句：

```text
Identity 提供关系事实，AuthZ 才做访问决策。
```

---

## 10. Identity 与 Suggest 的边界

Suggest 回答：

```text
当前请求者在允许范围内能搜索到哪些 Profile 候选项？
```

Identity 回答：

```text
Profile 主数据是什么？User 和 Profile 的关系是什么？
```

正确关系：

```text
Identity Profile facts
  -> Suggest index builder
  -> ProfileSearchTerm / ProfileSuggestionIndex
  -> SuggestProfile query
  -> visibility filter
  -> masked ProfileSuggestItem
```

禁止混用：

```text
Suggest 写 Profile 主数据；
ProfileSuggestionIndex 当 Profile 主数据；
ProfileSuggestItem 当 Profile entity；
索引命中直接返回；
搜索结果绕过可见性过滤；
返回明文手机号或证件号。
```

讲解句：

```text
Identity 是 Profile 主数据源，Suggest 只是从这些事实派生出来的搜索读模型。
```

---

## 11. 典型业务场景讲法

### 11.1 家长查看儿童档案

```text
User 表示家长；
Profile 表示儿童档案；
ProfileLink 表示家长与儿童档案之间的监护关系；
查看档案这个动作还需要 AuthZ 判断 profile.read 是否允许。
```

重点：

```text
监护关系是事实；
读取档案是权限；
两者有关联，但不是同一个东西。
```

---

### 11.2 微信登录后找到内部用户

```text
微信 openid / unionid 先由 IDP 或 AuthN 外部登录链路解析；
AuthN 根据 LoginIdentity 找到内部 User；
Identity 提供这个 User 的 ProfileLink 关系；
业务系统再基于这些事实继续业务流程。
```

重点：

```text
openid 不是 UserID；
外部身份需要映射到内部 LoginIdentity/User；
Identity 不直接相信 provider claims 创建权限。
```

---

### 11.3 搜索儿童档案候选

```text
Identity 维护 Profile 主数据；
Suggest 从 Profile facts 派生搜索索引；
查询时根据 keyword 命中候选；
再结合 ProfileAccessScope 和可见性过滤；
最后返回脱敏 ProfileSuggestItem。
```

重点：

```text
搜索命中不等于可见；
可见不等于有详情读取权限；
详情读取仍要 AuthZ Check。
```

---

## 12. 面试追问展开点

| 追问 | 回答要点 |
| --- | --- |
| 为什么 User 和 Profile 要分开？ | User 是使用者身份，Profile 是业务档案或被服务对象，二者生命周期和业务语义不同 |
| 为什么需要 ProfileLink？ | User 与 Profile 可能是多对多关系，需要显式表达关系类型和状态 |
| ProfileLink 是不是权限？ | 不是。它是身份关系事实，权限要由 AuthZ 的 Resource/Action/Scope/Permission 判断 |
| openid 能不能直接当 UserID？ | 不建议。openid 是外部 provider 标识，应通过 LoginIdentity 映射到内部 User |
| Identity 和 AuthN 怎么协作？ | AuthN 认证登录身份，成功后得到 Principal/UserID，再读取 Identity 身份事实 |
| Identity 和 AuthZ 怎么协作？ | AuthZ Check 可以读取 Identity facts，但最终决策属于 AuthZ |
| Identity 和 Suggest 怎么协作？ | Suggest 从 Identity Profile facts 派生搜索读模型，不反写 Profile 主数据 |

---

## 13. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| User 等于手机号 | 手机号会变，且只是登录标识 | User 作为稳定内部身份 |
| User 等于 openid | 外部 provider 标识污染内部模型 | openid -> LoginIdentity -> User |
| Profile 等于账号 | 被服务对象和登录主体混淆 | Profile 作为业务档案 |
| ProfileLink 等于 Permission | 身份关系和授权策略混淆 | ProfileLink 作为事实输入，AuthZ Check 决策 |
| Identity 处理密码校验 | 模块职责混乱 | Credential 校验归 AuthN |
| Identity 表承载权限 | 授权事实散落 | Role/Permission/RoleBinding 归 AuthZ |
| ProfileSuggestionIndex 当 Profile 主数据 | 读写模型混淆 | Profile 主数据归 Identity |
| 业务系统直接查 Identity DB | 绕过契约和治理 | 通过 REST/gRPC/SDK 接入 |

---

## 14. 推荐表达顺序

讲 Identity 时建议按这个顺序：

```text
1. 先说 Identity 是身份事实中心；
2. 再说三个核心对象：User / Profile / ProfileLink；
3. 解释 User 和 Profile 为什么分开；
4. 解释 ProfileLink 是关系事实；
5. 强调 ProfileLink 不是 Permission；
6. 回到 AuthN / AuthZ / Suggest 的边界；
7. 用家长-儿童档案场景举例。
```

不推荐：

```text
一上来讲数据库表；
一上来讲接口字段；
把 ProfileLink 讲成权限；
把 User 讲成手机号或 openid；
把 Identity 和 AuthN 混成用户登录模块。
```

---

## 15. 事实源回链

| 内容 | 事实源 |
| --- | --- |
| Identity 模块 | [../02-业务模块/01-Identity/README.md](../02-业务模块/01-Identity/README.md) |
| 业务模块总览 | [../02-业务模块/README.md](../02-业务模块/README.md) |
| AuthN 模块 | [../02-业务模块/02-AuthN/README.md](../02-业务模块/02-AuthN/README.md) |
| AuthZ 模块 | [../02-业务模块/03-AuthZ/README.md](../02-业务模块/03-AuthZ/README.md) |
| Suggest 模块 | [../02-业务模块/05-Suggest/README.md](../02-业务模块/05-Suggest/README.md) |
| ProfileLink 专题 | [../05-专题设计/05-ProfileLink为什么不是Permission.md](../05-专题设计/05-ProfileLink为什么不是Permission.md) |
| Suggest 读模型专题 | [../05-专题设计/06-Suggest为什么是读模型.md](../05-专题设计/06-Suggest为什么是读模型.md) |
| 接入契约 | [../03-接入与契约/README.md](../03-接入与契约/README.md) |

---

## 16. Verify

修改本文后至少执行：

```bash
make docs-hygiene
```

如果同步修改 Identity 相关代码或契约，需要执行：

```bash
go test ./internal/apiserver/domain/identity/...
go test ./internal/apiserver/application/identity/...
make api-validate
make proto-gen
go test ./internal/pkg/architecture
```

---

## 17. 本文总结

Identity 讲法可以压缩成：

```text
Identity 是身份事实中心；
User 是稳定身份主体；
Profile 是业务档案或被服务对象；
ProfileLink 是 User 与 Profile 的身份关系；
ProfileLink 不是 Permission；
Identity 不负责登录认证，也不直接做授权决策。
```

宣讲时最重要的是：

```text
把 User、Profile、ProfileLink 三个对象讲清楚；
把 Identity 与 AuthN、AuthZ、Suggest 的边界讲清楚；
用 ProfileLink 不是 Permission 体现建模深度。
```
