# AuthZ 讲法

> 状态：设计目标 · 宣讲第一版，已按金字塔结构重写；后续需要继续结合 `internal/apiserver/domain/authz`、`application/authz`、Casbin adapter、PolicyVersion、Outbox、REST/gRPC 契约、AuthZ 模块文档和测试逐项核对。

---

## 1. 本文目标

本文用于回答：

```text
AuthZ 模块在 IAM 中负责什么？
```

它是宣讲稿，不是完整领域模型文档，适用于：

```text
面试讲解 AuthZ；
解释 Subject / Resource / Action / Scope；
解释 Role / Permission / RoleBinding；
解释 AuthZ Check 链路；
解释 AuthN 与 AuthZ 的边界；
解释 Casbin 为什么只是 runtime；
解释 PolicyVersion / Outbox / RuntimeReload。
```

本文采用金字塔表达：

```text
先一句话定位；
再讲授权主链路；
再讲核心对象；
再讲 Casbin 与 PolicyVersion；
再讲 AuthZ 与 Identity/AuthN/Suggest 的边界；
最后讲常见追问。
```

---

## 2. 一句话定位

AuthZ 是 IAM 的授权中心，负责判断某个 Subject 在某个 Scope 下，能不能对某个 Resource 执行某个 Action。

更短一点：

```text
AuthZ 管“能不能做”，不管“怎么证明你是谁”。
```

---

## 3. 30 秒版本

```text
AuthZ 模块负责资源访问决策。它的核心不是简单判断 user.role == admin，而是把授权问题拆成 Subject、Resource、Action、Scope、Role、Permission、RoleBinding 和 AuthorizationDecision。请求进来后，先由 AuthN 得到 Principal，再映射成 AuthZ 的 Subject，然后构造 Resource、Action、Scope，最后通过 Check 返回 allow 或 deny。这里 Casbin 只是 infra 层的运行时判定引擎，不是 AuthZ 的领域模型。
```

---

## 4. 1 分钟版本

```text
AuthZ 是 IAM 的授权中心，核心问题是“某个主体能不能对某个资源执行某个动作”。我把授权模型拆成 Subject、Resource、Action、Scope、Role、Permission、RoleBinding 几个核心对象。Subject 表示授权主体，Resource 表示被访问资源，Action 表示要执行的动作，Scope 表示授权域；RoleBinding 表示某个 Subject 在某个 Scope 下绑定了某个 Role，Permission 表示 Role 能对哪些 Resource 执行哪些 Action。

Check 链路上，AuthN 只负责认证出 Principal，AuthZ 会把 Principal 映射为 Subject，再结合 Resource、Action、Scope 做授权判断。Casbin 在这里不是领域模型，而是 infra 层的 runtime engine，用来把 RoleBinding 和 Permission 编译成可执行的 p/g/r rules。授权策略写入后，会通过 PolicyVersion、Outbox 和 RuntimeReload 让运行时策略最终一致地刷新。
```

---

## 5. 3 分钟版本

```text
AuthZ 是 IAM 里的授权中心。它解决的不是“用户怎么登录”，而是“已经认证过的主体，能不能访问某个资源”。所以 AuthZ 的核心问题可以抽象成一句话：Subject 在某个 Scope 下，能不能对某个 Resource 执行某个 Action。

我把 AuthZ 的模型拆成几组对象。

第一组是授权请求对象：Subject、Resource、Action、Scope。Subject 是授权主体，它通常由 AuthN 的 Principal 映射而来；Resource 是被访问的资源，比如某个 Profile、某个后台配置、某个机构数据；Action 是动作，比如 read、write、delete、export、manage；Scope 是授权域，比如家庭、机构、项目、租户或业务域。

第二组是授权策略对象：Role、Permission、RoleBinding。Permission 表示某个 Role 能对什么 Resource 执行什么 Action；RoleBinding 表示某个 Subject 在某个 Scope 下绑定了某个 Role。这样设计比简单的 user.role == admin 更清晰，因为它能表达“谁、在哪个范围、对什么资源、做什么动作”。

第三组是授权决策对象：AuthorizationRequest 和 AuthorizationDecision。Check 用例会根据 Subject、Resource、Action、Scope 构造请求，结合当前策略和必要事实返回 allow 或 deny。这里要注意，AuthZ Check 是资源访问决策，不能被 Token 验签、ProfileLink 或 JWT claims 替代。

第四组是运行时和传播机制。项目里可以使用 Casbin 作为授权 runtime，但 Casbin 只是 infra 层的判定引擎。AuthZ domain 仍然使用 Subject、Role、Permission、RoleBinding 这些业务语言，infra/authz adapter 再把它们映射成 Casbin 的 p/g/r rules。授权事实变更后，需要 bump PolicyVersion，并通过 Transactional Outbox 发布版本变化事件，RuntimeReload 再刷新 Casbin runtime snapshot。

所以 AuthZ 不是简单的角色字段，也不是 Casbin CRUD。它是围绕授权主体、资源、动作、范围和策略版本建立起来的一套访问决策模型。
```

---

## 6. 金字塔结构

### 6.1 顶层结论

```text
AuthZ 负责资源访问决策。
```

---

### 6.2 一条主链路

```text
Principal
  -> Subject
  -> Resource / Action / Scope
  -> RoleBinding / Role / Permission
  -> Check
  -> AuthorizationDecision
```

---

### 6.3 七个核心对象

| 对象 | 一句话 | 不是什么 |
| --- | --- | --- |
| `Subject` | 授权主体 | 不是 Principal 本体，不是 User 本体 |
| `Resource` | 被访问资源 | 不是数据库表名的简单替代，不是 Profile 本体的全部语义 |
| `Action` | 对资源执行的动作 | 不是 HTTP method 的简单等价 |
| `Scope` | 授权域或范围 | 不是 ProfileLink，不是 Suggest ProfileAccessScope 本体 |
| `Role` | 权限集合的命名载体 | 不是用户身份事实 |
| `Permission` | 资源动作访问声明 | 不是 ProfileLink，不是认证结果 |
| `RoleBinding` | Subject 在 Scope 下绑定 Role | 不是 ProfileLink，不是 Casbin g rule 本体 |

---

### 6.4 三条核心边界

| 边界 | 说明 |
| --- | --- |
| AuthZ vs AuthN | AuthN 证明是谁，AuthZ 判断能做什么 |
| AuthZ vs Identity | Identity 提供身份关系事实，AuthZ 做访问决策 |
| AuthZ vs Casbin | AuthZ domain 是业务模型，Casbin 是 infra runtime engine |

---

## 7. AuthZ 对象讲法

### 7.1 Subject

讲法：

```text
Subject 是授权主体。它通常由 AuthN 的 Principal 映射而来，表达“谁在请求访问资源”。
```

重点：

```text
Subject 不是 Principal 本体；
Subject 不是 User 的简单别名；
一个 Principal 在不同授权域下可能映射成不同 Subject 表达；
不要把 openid、unionid、手机号直接当 Subject。
```

---

### 7.2 Resource

讲法：

```text
Resource 是被访问的业务资源，例如某个 Profile、某个测评结果、某个后台配置、某个机构数据。它要表达资源类型和资源实例，不能只靠接口路径隐式表达。
```

重点：

```text
Resource 要有业务语义；
Resource 不等于数据库表；
Resource 不等于 URL；
同一个 URL 下也可能涉及不同 Resource；
高敏资源要单独建模。
```

---

### 7.3 Action

讲法：

```text
Action 表示对资源执行的动作，例如 read、write、delete、export、manage。Action 的粒度决定了权限控制的精细程度。
```

重点：

```text
Action 不应只等同 HTTP method；
read、export、share、delete 这些动作风险不同；
高风险操作必须单独建模 Action；
没有 Action 维度就很难解释为什么某个操作被允许。
```

---

### 7.4 Scope

讲法：

```text
Scope 表示授权生效的范围，例如家庭、机构、项目、租户或业务域。它用于回答“这个角色或权限在哪个范围内有效”。
```

重点：

```text
Scope 是授权域；
Scope 不是 ProfileLink；
Scope 不是 Suggest 的 ProfileAccessScope 本体；
同一个 Subject 在不同 Scope 下权限可以不同。
```

---

### 7.5 Role

讲法：

```text
Role 是权限集合的命名载体，比如 parent、doctor、admin、operator 等。Role 本身不代表身份事实，它要通过 RoleBinding 绑定到 Subject 才对某个主体生效。
```

重点：

```text
Role 是授权概念；
Role 不等于 User 类型；
Role 不等于 ProfileLink 关系类型；
Role 要结合 Scope 才能避免全局过度授权。
```

---

### 7.6 Permission

讲法：

```text
Permission 表示某个 Role 或策略允许对某类 Resource 执行某个 Action。它是访问权声明，不是身份关系事实。
```

重点：

```text
Permission 必须表达 Resource / Action；
Permission 通常需要 Scope 或 domain 语义；
Permission 不是 ProfileLink；
Permission 不应该反向定义亲属关系、档案关系或登录身份。
```

---

### 7.7 RoleBinding

讲法：

```text
RoleBinding 表示某个 Subject 在某个 Scope 下绑定了某个 Role。它回答“这个主体在这个授权域下是什么角色”。
```

重点：

```text
RoleBinding 是授权绑定事实；
RoleBinding 不是 ProfileLink；
RoleBinding 可以映射成 Casbin g rule，但不等于 g rule 本体；
RoleBinding 变化通常会影响 PolicyVersion。
```

---

## 8. AuthZ Check 链路讲法

标准链路：

```text
AccessToken verified
  -> Principal
  -> SubjectMapper
  -> Subject
  -> build Resource / Action / Scope
  -> AuthZ Check
  -> AuthorizationDecision allow / deny
```

讲解重点：

```text
Token 验签只说明请求者身份可信；
Subject 是授权主体；
Resource / Action / Scope 是授权请求的核心三元组；
RoleBinding / Permission 是授权策略事实；
Check 返回 AuthorizationDecision；
deny 不应该被业务系统当成需要 refresh token。
```

边界：

```text
AuthZ Check 不应该散落在业务系统 if/else；
transport 不应该直接调用 Casbin；
业务系统不应该复制 IAM 的 policy；
JWT claims 不应该承载完整权限系统。
```

---

## 9. 授权写入链路讲法

标准链路：

```text
Grant / Revoke / Bind / Unbind
  -> validate authorization command
  -> write Role / Permission / RoleBinding facts
  -> bump PolicyVersion
  -> write OutboxEvent(policy.version.changed)
  -> commit
  -> Relay publish
  -> RuntimeReload
  -> AuthZ runtime reload snapshot
```

讲解重点：

```text
授权写入和 PolicyVersion 变化要一致；
Outbox 解决业务事实写入和事件发布的双写问题；
RuntimeReload 可以最终一致；
重复事件要通过 version 幂等处理；
Casbin runtime snapshot 不是授权事实源。
```

边界：

```text
授权写入不是直接改 Casbin 内存；
Casbin policy reload 不是授权事实本身；
Outbox 不是 MQ；
Outbox 不承诺 exactly-once。
```

---

## 10. Casbin 讲法

一句话：

```text
Casbin 是 AuthZ 的 infra runtime engine，不是 AuthZ 的领域模型。
```

正确关系：

```text
AuthZ domain
  -> Subject / Role / Permission / RoleBinding
  -> AuthZRuntime port
  -> infra/authz Casbin adapter
  -> p / g / r rules
  -> enforcer allow / deny
```

讲解重点：

```text
domain 不 import Casbin；
application 通过 AuthZRuntime port 调用；
infra adapter 负责映射 p/g/r；
transport 不直接调用 enforcer；
业务系统不读取 Casbin policy。
```

常见误解：

```text
不是用了 Casbin 就不用建 AuthZ 领域模型；
不是把 p/g/r 存进库就完成授权设计；
不是把 ProfileLink 写成 g rule 就等于有权限。
```

---

## 11. AuthZ 与 AuthN 的边界

AuthN 回答：

```text
你是谁？
```

AuthZ 回答：

```text
你能不能做这个操作？
```

正确关系：

```text
AuthN Principal
  -> AuthZ Subject
  -> Resource / Action / Scope
  -> Check
```

禁止混用：

```text
Token 验签成功直接放行；
JWT claims 当完整权限；
RefreshToken 当访问凭证；
AuthN 直接写 RoleBinding；
AuthZ 处理密码/验证码校验。
```

讲解句：

```text
认证给出可信身份上下文，授权给出资源访问决策。
```

---

## 12. AuthZ 与 Identity 的边界

Identity 回答：

```text
User 与 Profile 是什么关系？
```

AuthZ 回答：

```text
Subject 能不能对 Resource 执行 Action？
```

正确关系：

```text
Identity ProfileLink
  -> identity fact
  -> AuthZ Check may use it
  -> AuthorizationDecision
```

禁止混用：

```text
ProfileLink 即 Permission；
guardian 自动拥有所有权限；
User 直接等于 Subject；
Profile 直接等于所有资源权限；
Identity 表承载 RoleBinding / Permission。
```

讲解句：

```text
Identity 提供关系事实，AuthZ 决定这些事实能不能推出某个具体操作的访问权。
```

---

## 13. AuthZ 与 Suggest 的边界

Suggest 回答：

```text
当前请求者能搜索到哪些 Profile 候选项？
```

AuthZ 回答：

```text
当前 Subject 能不能对某个资源执行某个动作？
```

正确关系：

```text
SuggestProfile
  -> candidate match
  -> visibility filter may call AuthZ / Identity facts
  -> masked ProfileSuggestItem
  -> later detail API still needs AuthZ Check
```

禁止混用：

```text
ProfileSuggestItem 当授权凭证；
搜索到候选就允许读取详情；
索引命中直接返回；
可见性过滤替代所有资源权限判断；
手机号搜索绕过 AuthZ/Identity 可见性过滤。
```

讲解句：

```text
Suggest 控制候选是否可被搜索到，AuthZ 控制资源操作是否被允许。
```

---

## 14. 典型业务场景讲法

### 14.1 家长查看儿童档案

```text
Principal(user)
  -> Subject(user:U1)
  -> Resource(profile:P1)
  -> Action(profile.read)
  -> Scope(family:F1)
  -> Check
  -> allow / deny
```

重点：

```text
ProfileLink 可以作为事实输入；
但读取档案仍然是一个 Resource/Action/Scope 的授权问题；
不能因为有监护关系就自动允许所有操作。
```

---

### 14.2 医生导出报告

```text
Subject(staff:S1)
  -> Resource(report:R1)
  -> Action(report.export)
  -> Scope(org:O1 / project:P1)
  -> Check
```

重点：

```text
read 和 export 风险不同；
导出应单独建模 Action；
医生有服务关系不代表可以导出所有数据。
```

---

### 14.3 管理员授权角色

```text
Subject(admin:A1)
  -> Action(role_binding.create)
  -> Resource(role_binding)
  -> Scope(org:O1)
  -> Check
  -> if allow: create RoleBinding
  -> bump PolicyVersion
  -> OutboxEvent
```

重点：

```text
授权写入本身也要授权；
写入后要更新 PolicyVersion；
RuntimeReload 最终一致刷新策略。
```

---

## 15. 面试追问展开点

| 追问 | 回答要点 |
| --- | --- |
| 为什么不是 user.role == admin？ | 角色要和 Resource、Action、Scope 结合，否则无法表达范围和动作差异 |
| Subject 和 User 有什么区别？ | User 是身份事实，Subject 是授权主体，一个 User 不一定等同于所有授权场景的 Subject |
| Scope 为什么重要？ | 同一个角色在不同组织/项目/家庭下权限不同，Scope 防止全局过度授权 |
| Permission 和 RoleBinding 有什么区别？ | Permission 是资源动作声明，RoleBinding 是主体绑定角色的事实 |
| ProfileLink 是不是 Permission？ | 不是。ProfileLink 是身份关系事实，Permission 是访问权声明 |
| Casbin 是不是领域模型？ | 不是。Casbin 是 infra runtime，domain 仍然使用 Subject/Role/Permission 等业务语言 |
| 为什么需要 PolicyVersion？ | 授权事实变化后，runtime/cache/consumer 需要知道新版本并幂等刷新 |
| 为什么需要 Outbox？ | 保证授权事实写入和版本变化事件记录在同一个本地事务中 |

---

## 16. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| `user.role == admin` | 只有全局角色，没有资源/动作/范围 | Subject + Resource + Action + Scope |
| Token 验签成功直接放行 | 认证授权混淆 | 验签后 AuthZ Check |
| JWT claims 存完整权限 | 权限更新困难且易漂移 | 权限由 AuthZ runtime 决策 |
| ProfileLink 当 Permission | 身份关系和授权混淆 | ProfileLink 作为事实输入 |
| RoleBinding 当 ProfileLink | 授权绑定和身份关系混淆 | 两套模型分开 |
| Casbin 当领域模型 | runtime 污染 domain | Casbin 留在 infra adapter |
| transport 直接调用 Casbin | 绕过 application | transport -> application -> runtime port |
| 授权写入后直接改内存 policy | 事实源和 runtime 不一致 | 写事实 + PolicyVersion + Outbox + Reload |
| 没有 Action 粒度 | 无法区分 read/export/delete | 明确 Action 建模 |
| 没有 Scope | 容易全局过度授权 | 权限绑定到明确授权域 |

---

## 17. 推荐表达顺序

讲 AuthZ 时建议按这个顺序：

```text
1. 先说 AuthZ 是授权中心；
2. 说明它回答“你能不能做什么”，不是“你是谁”；
3. 讲 Subject / Resource / Action / Scope；
4. 讲 Role / Permission / RoleBinding；
5. 讲 Check 返回 AuthorizationDecision；
6. 讲 Casbin 只是 runtime；
7. 讲 PolicyVersion / Outbox / RuntimeReload；
8. 用 ProfileLink 不是 Permission 做边界案例。
```

不推荐：

```text
一上来讲 Casbin API；
把 AuthZ 讲成角色字段判断；
把 User、Subject、Principal 混用；
把 ProfileLink 讲成权限；
只讲 read，不讲 write/delete/export；
忽略 Scope。
```

---

## 18. 事实源回链

| 内容 | 事实源 |
| --- | --- |
| AuthZ 模块 | [../02-业务模块/03-AuthZ/README.md](../02-业务模块/03-AuthZ/README.md) |
| AuthZ 领域模型 | [../02-业务模块/03-AuthZ/01-领域模型-Subject-Resource-Action-Scope-Role-Permission-RoleBinding.md](../02-业务模块/03-AuthZ/01-领域模型-Subject-Resource-Action-Scope-Role-Permission-RoleBinding.md) |
| 权限检查链路 | [../02-业务模块/03-AuthZ/02-关键链路-权限检查Check.md](../02-业务模块/03-AuthZ/02-关键链路-权限检查Check.md) |
| 授权写入链路 | [../02-业务模块/03-AuthZ/03-关键链路-授权写入Grant-Revoke-Bind-Unbind.md](../02-业务模块/03-AuthZ/03-关键链路-授权写入Grant-Revoke-Bind-Unbind.md) |
| PolicyVersion / Outbox | [../02-业务模块/03-AuthZ/04-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md](../02-业务模块/03-AuthZ/04-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md) |
| Casbin 专题 | [../05-专题设计/04-Casbin在AuthZ中的定位.md](../05-专题设计/04-Casbin在AuthZ中的定位.md) |
| ProfileLink 专题 | [../05-专题设计/05-ProfileLink为什么不是Permission.md](../05-专题设计/05-ProfileLink为什么不是Permission.md) |
| AuthN 讲法 | [04-AuthN讲法.md](04-AuthN讲法.md) |
| Identity 讲法 | [03-Identity讲法.md](03-Identity讲法.md) |

---

## 19. Verify

修改本文后至少执行：

```bash
make docs-hygiene
```

如果同步修改 AuthZ 相关代码或契约，需要执行：

```bash
go test ./internal/apiserver/domain/authz/...
go test ./internal/apiserver/application/authz/...
go test ./internal/apiserver/infra/...
make api-validate
make proto-gen
go test ./internal/pkg/architecture
```

---

## 20. 本文总结

AuthZ 讲法可以压缩成：

```text
AuthZ 是授权中心；
Subject 是授权主体；
Resource 是被访问资源；
Action 是操作；
Scope 是授权域；
Permission 是资源动作访问声明；
RoleBinding 是主体与角色的授权绑定；
Check 返回 AuthorizationDecision；
Casbin 是 infra runtime，不是领域模型；
PolicyVersion / Outbox / RuntimeReload 保证 runtime 最终感知授权变化。
```

宣讲时最重要的是：

```text
把“你是谁”和“你能做什么”分开；
把 User / Principal / Subject 分开；
把 ProfileLink / RoleBinding / Permission 分开；
用 Resource / Action / Scope 体现授权模型深度；
用 Casbin runtime 与 AuthZ domain 分离体现架构边界。
```
