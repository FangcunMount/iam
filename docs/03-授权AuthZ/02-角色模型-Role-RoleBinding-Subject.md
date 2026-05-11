# 02-角色模型：Role、RoleBinding、Subject

## 1. 本文定位

本文是 `03-授权AuthZ/` 文档组中关于 **授权主体、角色与角色绑定** 的模型文档。

前两篇文档已经建立了：

```text
00-AuthZ模型总览：Subject -> RoleBinding -> Role -> Permission -> Resource / Action / Scope
01-资源模型：ResourceKey / ResourcePattern / Action / ActionPattern / Scope
```

本文聚焦主线中的这一段：

```text
Subject
  -> RoleBinding
  -> Role
```

也就是回答：

```text
谁是被授权主体？
Role 是什么？
RoleName 为什么要建模？
Subject 如何获得 Role？
RoleBinding 与 Assignment 的边界是什么？
为什么 RoleBinding 必须带 Tenant / Authorization Domain？
为什么当前 group/service 是模型预留但写入侧主要开放 user？
RoleBinding 写入为什么必须进入 PolicyChange / PolicyChangeCommitter？
```

本文不展开 Resource / Action / Scope 细节，也不展开 Permission 写入事务、PolicyVersion、Outbox、RuntimeReload、Casbin p/g facts 的运行时细节。

这些内容分别放在：

```text
01-资源模型-ResourceKey-ResourcePattern-Action-Scope.md
03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md
04-授权版本与事件传播链路-PolicyVersion-Outbox-RuntimeReload.md
05-权限检查链路-Check-Snapshot.md
06-Casbin运行时模型-pgFacts与四段Matcher.md
07-AuthZ分层架构与事实源索引.md
```

---

## 2. 30 秒结论

AuthZ 中角色与绑定模型的核心是：

```text
Subject      被授权主体，当前模型支持 user / group / service
Role         权限聚合点，承载一组 Permission
RoleName     Role 的稳定业务标识
Tenant       授权域边界，同一 Subject 在不同 Tenant 下可以持有不同 Role
RoleBinding  Subject 在某个 Tenant 下持有某个 Role 的授权事实
Binding      管理面记录，用于查询、撤销、审计
Assignment   REST / proto / SDK 对外 wire term，内部标准术语仍是 RoleBinding
```

核心关系是：

```text
Subject
  -> RoleBinding(Tenant)
  -> Role
  -> Permission
```

一句话：

> Subject 不直接拥有 Permission，而是在某个 Tenant 下通过 RoleBinding 持有 Role；Role 是权限聚合点，RoleBinding 是 Subject 与 Role 在 Tenant 维度上的绑定事实。

---

## 3. 为什么需要 Role

如果不引入 Role，最直接的模型是：

```text
Subject -> Permission
```

也就是把权限直接挂在用户或主体上。

这种模型短期简单，但很快会失控：

| 问题 | 说明 |
| --- | --- |
| 权限碎片化 | 每个用户身上都有一堆 Permission，难以统一维护 |
| 业务语义弱 | 看不出这个用户是管理员、运营、测评师还是访客 |
| 审计困难 | 很难解释“为什么这个用户有这组权限” |
| 变更成本高 | 岗位权限变更时需要批量修改大量 Subject |
| 复用困难 | 多个用户共享同一组权限时无法沉淀稳定权限包 |
| 接入困难 | 下游系统更希望理解角色，而不是逐条权限 |
| 多租户困难 | 同一个 Subject 在不同 Tenant 下角色不同，直接挂 Permission 很难管理 |

Role 的价值是把权限聚合成稳定的业务身份或访问职责：

```text
Role = 一组 Permission 的业务命名
```

例如：

```text
iam:admin
iam:viewer
qs:evaluator
qs:operator
```

Role 让系统可以表达：

```text
某个 Subject 在某个 Tenant 下是 iam:admin
某个 Role 拥有哪些 Resource / Action / Scope 能力
```

而不是直接问：

```text
这个 Subject 身上有多少条 Permission？
```

---

## 4. Role：权限聚合点

### 4.1 Role 是什么

`Role` 是权限聚合点。

它回答：

```text
一组权限应该以什么业务角色被管理？
```

Role 本身不等于 Permission。

Role 是权限集合的业务名称。

Permission 是具体能力声明。

例如：

```text
Role: qs:evaluator

Permissions:
  qs:evaluation:report:* read|export origin:<evaluator-id>
  qs:survey:questionnaire:* read all:*
```

Role 让这些 Permission 形成一个可理解的业务职责：

```text
测评师
```

---

### 4.2 Role 的核心字段

在当前模型中，Role 至少包含：

```text
ID
Name
DisplayName
TenantID
Description
```

其中：

| 字段 | 语义 |
| --- | --- |
| ID | 数据库 / 管理面身份标识 |
| Name | 稳定业务角色名 |
| DisplayName | 展示名称 |
| TenantID | 角色所属授权域 |
| Description | 角色说明 |

Role 的关键不是 ID，而是：

```text
RoleName + TenantID
```

因为在授权事实中，Permission 和 RoleBinding 更关心：

```text
哪个 tenant 下的哪个 role？
```

---

### 4.3 Role 不应该是 User 表字段

不要把 Role 理解成：

```text
users.role = admin
```

这种设计的问题是：

```text
一个用户只能有一个角色；
无法表达 tenant 维度；
无法表达多个业务系统的角色；
无法表达 role 与 permission 的独立生命周期；
无法审计授权动作；
无法支持 group / service subject；
无法通过 PolicyVersion / Outbox 感知授权变更。
```

当前模型是：

```text
User / Subject 与 Role 分离
Role 与 Permission 分离
Subject 通过 RoleBinding 获得 Role
```

也就是：

```text
Subject -> RoleBinding -> Role -> Permission
```

---

## 5. RoleName：稳定业务角色标识

### 5.1 RoleName 是什么

`RoleName` 是 Role 的稳定业务标识。

它回答：

```text
这个角色在业务语义上叫什么？
```

例如：

```text
iam:admin
iam:viewer
qs:evaluator
qs:operator
```

RoleName 应该比 Role.ID 更适合出现在授权事实中。

原因是：

```text
Role.ID 是数据库记录身份；
RoleName 是授权事实中的业务身份。
```

在运行时授权事实中，通常更希望表达：

```text
role:iam:admin
```

而不是：

```text
role_id:123456
```

因为 RoleName 更稳定、更可读、更适合跨服务传播和排查。

---

### 5.2 RoleName 与 app namespace

推荐 RoleName 带 app 前缀：

```text
iam:admin
qs:evaluator
profile:operator
```

这样做的好处是：

```text
不同 app 的角色不会冲突；
AuthorizationSnapshot 可以按 app 投影角色；
SDK / 下游系统更容易理解角色归属；
角色名具备自解释能力。
```

例如：

```text
iam:admin
qs:admin
```

这两个都叫 admin，但归属不同 app，不应该混淆。

---

### 5.3 RoleName 与 DisplayName 的区别

| 概念 | 作用 | 示例 |
| --- | --- | --- |
| RoleName | 稳定业务标识，进入授权事实 | `iam:admin` |
| DisplayName | 展示名称，可变化 | `IAM 管理员` |

RoleName 应该稳定。

DisplayName 可以调整。

例如：

```text
RoleName: iam:admin
DisplayName: IAM 管理员
```

后续可以把 DisplayName 改为：

```text
平台管理员
```

但 RoleName 不应该随意变化。

因为 RoleName 会被这些链路引用：

```text
Permission
RoleBinding
PolicyChange
Casbin p/g facts
AuthorizationSnapshot
Token / SDK 展示上下文
审计记录
```

---

## 6. Subject：被授权主体

### 6.1 Subject 是什么

`Subject` 是被授权主体。

它回答：

```text
谁在请求访问资源？
```

当前模型支持：

```text
user
group
service
```

例如：

```text
user:1001
group:2001
service:3001
```

Subject 是 AuthZ 的主体引用，不等于完整 User 聚合。

AuthZ 不关心用户手机号、昵称、登录身份、密码、第三方 IDP 信息。

这些属于 AuthN / Identity。

AuthZ 只关心：

```text
这个被授权主体是谁？
这个主体在 tenant 下持有哪些 role？
```

---

### 6.2 user / group / service 的语义

| Subject Type | 语义 | 当前状态 |
| --- | --- | --- |
| user | IAM User 主体 | 当前主要开放写入 |
| group | 用户组主体 | 模型预留，后续扩展 |
| service | 服务账号 / 机器主体 | 模型预留，后续扩展 |

当前阶段需要明确：

```text
模型支持 user / group / service；
SubjectResolver 架构支持扩展；
写入侧当前主要开放 user。
```

这不是矛盾。

这是分阶段演进：

```text
先把模型边界设计好；
再按业务需要逐步开放 group / service 写入能力。
```

---

### 6.3 Subject 与 User 的区别

User 是 Identity 模块中的稳定主体。

Subject 是 AuthZ 模块中的授权主体引用。

两者关系可以理解为：

```text
User 是身份模型。
user:<id> 是授权模型中的 SubjectRef。
```

AuthZ 不应该直接持有完整 User 模型。

它只需要知道：

```text
subject type
subject id
```

以及通过 SubjectResolver 判断这个 subject 是否存在、是否允许被授权。

---

## 7. Tenant：RoleBinding 的授权域边界

### 7.1 为什么 RoleBinding 必须带 Tenant

RoleBinding 不是全局事实，而是 tenant/domain 下的授权事实。

它回答：

```text
Subject 在哪个授权域下持有哪个 Role？
```

例如：

```text
user:1001 在 tenant-a 下持有 iam:admin
user:1001 在 tenant-b 下持有 iam:viewer
```

如果 RoleBinding 不带 Tenant，就无法表达：

```text
同一个用户在不同组织 / 租户 / 业务域中拥有不同角色。
```

因此，RoleBinding 的核心不是：

```text
subject -> role
```

而是：

```text
subject -> role within tenant
```

---

### 7.2 Tenant 与 Casbin domain 的关系

领域模型中叫：

```text
Tenant
Authorization Domain
```

Casbin 运行时中通常映射为：

```text
dom / domain
```

例如：

```text
RoleBinding: user:1001 holds iam:admin in tenant-a
Casbin g fact: g, user:1001, role:iam:admin, tenant-a
```

注意：

```text
Tenant 是领域概念；
Casbin domain 是 infra runtime 映射。
```

不要在领域文档中把 `dom` 当成业务模型。

---

### 7.3 Tenant 与 RoleName 的组合

RoleName 不应该脱离 Tenant 单独理解。

在多租户场景中，真正的角色语义通常是：

```text
TenantID + RoleName
```

例如：

```text
tenant-a / iam:admin
tenant-b / iam:admin
```

它们可以是同名角色，但授权域不同。

这也解释了为什么 Permission 和 RoleBinding 都需要 Tenant 维度。

---

## 8. RoleBinding：Subject 与 Role 的绑定事实

### 8.1 RoleBinding 是什么

`RoleBinding` 表示 Subject 在 Tenant 下持有某个 Role。

它回答：

```text
谁被授予了什么角色？在哪个 tenant 下？
```

核心结构是：

```text
Subject
RoleName
TenantID
GrantedBy
```

例如：

```text
Subject: user:1001
Tenant: tenant-a
RoleName: iam:admin
GrantedBy: user:9001
```

这表示：

```text
user:1001 在 tenant-a 下被 user:9001 授予 iam:admin 角色。
```

---

### 8.2 RoleBinding 为什么不是 Permission

RoleBinding 只表达：

```text
Subject 持有 Role
```

它不表达：

```text
Role 能访问哪些 Resource；
Role 能执行哪些 Action；
Role 的 Scope 是什么。
```

这些属于 Permission。

因此：

```text
RoleBinding 决定 Subject 拥有哪些 Role；
Permission 决定 Role 拥有哪些能力。
```

二者组合后才形成完整授权结果。

---

### 8.3 RoleBinding 的两种存在形态

在 IAM 中，RoleBinding 有两种形态：

```text
管理面 Binding 记录
运行时 g fact
```

| 形态 | 用途 | 示例 |
| --- | --- | --- |
| Management Binding | 查询、按 ID 撤销、审计、展示 | `authz_role_bindings` 记录 |
| Runtime g fact | 运行时判定 subject 是否持有 role | `g, user:1001, role:iam:admin, tenant-a` |

为什么需要两种？

因为运行时授权事实服务于判定：

```text
这个 subject 是否持有这个 role？
```

管理面记录服务于后台管理：

```text
这条授权是谁授予的？
什么时候授予的？
如何按 ID 撤销？
如何展示授权列表？
```

它们必须通过同一个写入链路保持一致。

不能只写管理面 Binding，也不能只写 Casbin g fact。

---

## 9. Assignment 与 RoleBinding 的边界

### 9.1 Assignment 是什么

`Assignment` 是对外接口术语。

它主要出现在：

```text
REST DTO
proto message
SDK method
```

例如：

```text
GrantAssignment
RevokeAssignment
```

对外使用 assignment 的原因是：

```text
业务调用方更容易理解“分配角色”；
外部契约不一定需要暴露内部 rolebinding 术语。
```

---

### 9.2 RoleBinding 是什么

`RoleBinding` 是内部领域术语。

它表达：

```text
Subject 在 Tenant 下持有 Role
```

这是 RBAC 语义下更准确的内部模型。

因此，当前约定是：

```text
assignment = REST / proto / SDK 对外 wire term
rolebinding = application / domain 内部标准术语
```

---

### 9.3 为什么不恢复 assignment 包

不要在内部恢复：

```text
domain/authz/assignment
application/authz/assignment
infra/mysql/assignment
```

原因是：

```text
assignment 更像对外接口说法；
rolebinding 才是内部授权模型；
恢复 assignment 包会让内部术语混乱。
```

正确分层是：

| 层次 | 推荐术语 | 说明 |
| --- | --- | --- |
| REST / proto / SDK | Assignment | 对外表示“角色分配” |
| Application | rolebinding command / service | 写入和查询角色绑定用例 |
| Domain | RoleBinding / Binding / Fact | 内部授权领域模型 |
| Infra Casbin | g fact | 运行时 subject-role-domain 事实 |

---

## 10. SubjectResolver：Subject 存在性与扩展点

### 10.1 为什么需要 SubjectResolver

RoleBinding 写入时，系统不能只检查 subject_id 是否非空。

还需要确认：

```text
这个 subject type 是否支持？
这个 subject 是否存在？
这个 subject 是否允许被授予 role？
```

如果直接在 rolebinding validator 中写死 user repository，会导致：

```text
group/service 后续难以扩展；
rolebinding 领域逻辑被 User 仓储污染；
subject type 越多，validator 越臃肿。
```

因此需要 SubjectResolver。

---

### 10.2 SubjectResolver 的职责

SubjectResolver 的职责是：

```text
判断某类 Subject 是否被当前系统支持；
解析或校验某个 Subject 是否存在。
```

可以抽象为：

```text
Supports(subjectType)
Resolve(subjectRef, tenantID)
```

当前可以有：

```text
UserSubjectResolver
```

未来可以扩展：

```text
GroupSubjectResolver
ServiceSubjectResolver
```

这样，RoleBinding 写入逻辑不需要知道每种主体背后的仓储细节。

---

### 10.3 当前 user-only 的阶段性边界

当前实现上，写入侧主要支持：

```text
user
```

而 group/service 属于模型预留和后续扩展。

这应该在文档中明确。

不要误解为：

```text
group/service 已经完整可写、可管理、可判定。
```

更准确的说法是：

```text
Subject 模型支持 user/group/service。
SubjectResolver 架构为 group/service 预留扩展点。
当前 RoleBinding 管理写入主要开放 user。
```

---

## 11. RoleBinding 写入的应用模型

RoleBinding 写入不是直接 insert 一条绑定记录。

它通常需要同时产生：

```text
管理面 Binding 记录；
运行时 g fact；
PolicyVersion 递增；
Outbox version_changed event；
Runtime policy reload。
```

因此，RoleBinding 写入应该走：

```text
Application Command
  -> PolicyAdministration
  -> AuthorizationPolicy
  -> PolicyChange
  -> PolicyChangeCommitter
```

常见命令包括：

```text
GrantCommand
RevokeCommand
GrantByRoleNameCommand
RevokeByRoleNameCommand
```

其中：

```text
REST 通常以 role_id 写入；
gRPC / SDK 可以以 role_name 写入。
```

这是接入方式差异，不改变内部 RoleBinding 语义。

完整写入链路见：

```text
03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md
```

---

## 12. RoleBinding 撤销为什么需要 actor 和 reason

授权撤销不是普通删除。

它是一次安全敏感操作。

撤销时至少需要知道：

```text
谁撤销的？
为什么撤销？
撤销了哪个 Subject 的哪个 Role？
在哪个 Tenant 下撤销？
```

因此，Revoke 相关 command 应该携带：

```text
ChangedBy / RevokedBy
Reason
```

这样后续可以支持：

```text
审计日志；
安全追踪；
误操作排查；
权限变更记录。
```

不要把撤销 actor 硬编码成 `system`，除非确实是系统自动动作。

---

## 13. 与 Casbin g fact 的关系

RoleBinding 在运行时会被映射为 Casbin `g` fact。

领域模型：

```text
Subject: user:1001
RoleName: iam:admin
Tenant: tenant-a
```

运行时 fact：

```text
g, user:1001, role:iam:admin, tenant-a
```

这表示：

```text
user:1001 在 tenant-a 下持有 role:iam:admin
```

但是要注意：

```text
RoleBinding 是领域授权事实；
g fact 是 infra runtime 表达。
```

两者不能混为一谈。

文档中可以解释映射，但不要把 `g` 当成领域模型。

具体 p/g facts 的结构和 matcher 细节放在：

```text
06-Casbin运行时模型-pgFacts与四段Matcher.md
```

---

## 14. 与 Check / Snapshot 的关系

RoleBinding 不是只服务写入管理，它还会影响读链路。

### 14.1 Check

一次 Check 会验证：

```text
Subject 在 Tenant 下是否持有某个 Role；
该 Role 是否拥有匹配 Resource / Action / Scope 的 Permission。
```

因此 RoleBinding 最终会参与：

```text
g(r.sub, p.sub, r.dom)
```

也就是：

```text
请求主体是否在当前 domain 下持有 policy 中的 role。
```

### 14.2 Snapshot

AuthorizationSnapshot 可以展示：

```text
某个 Subject 当前持有哪些 Role；
这些 Role 来自哪个 Tenant；
这些 Role 带来了哪些 Permission；
当前快照对应哪个 PolicyVersion。
```

因此 RoleBinding 也必须能被管理面查询，而不是只存在于 Casbin runtime 中。

完整读链路见：

```text
05-权限检查链路-Check-Snapshot.md
```

---

## 15. 建模示例

### 15.1 IAM 管理员

Subject：

```text
user:1001
```

Tenant：

```text
default
```

Role：

```text
iam:admin
```

RoleBinding：

```text
user:1001 在 default 下持有 iam:admin
```

如果 `iam:admin` 拥有用户管理权限，那么该 Subject 可以通过 Check 获得对应访问能力。

---

### 15.2 QS 测评师

Subject：

```text
user:2001
```

Tenant：

```text
tenant-a
```

Role：

```text
qs:evaluator
```

RoleBinding：

```text
user:2001 在 tenant-a 下持有 qs:evaluator
```

这个绑定本身不说明测评师能访问哪些报告。

具体能力由 `qs:evaluator` 关联的 Permission 决定。

---

### 15.3 服务账号预留模型

Subject：

```text
service:3001
```

Tenant：

```text
tenant-a
```

Role：

```text
qs:worker
```

RoleBinding：

```text
service:3001 在 tenant-a 下持有 qs:worker
```

这类模型适合后台 worker、服务间调用、机器身份授权。

但当前写入侧是否开放 service，需要以当前代码和接口为准。

---

## 16. 常见误区

### 16.1 Role 就是 User.role 字段

错误。

Role 是独立领域对象。

User / Subject 通过 RoleBinding 持有 Role。

---

### 16.2 Subject 只能是 User

不准确。

当前主要开放 user 写入，但模型预留了 group / service。

Subject 是授权主体引用，不等于完整 User 聚合。

---

### 16.3 RoleBinding 就是 Permission

错误。

RoleBinding 只表达 Subject 持有 Role。

Permission 才表达 Role 拥有哪些资源能力。

---

### 16.4 Assignment 和 RoleBinding 完全一样

不准确。

Assignment 是对外 wire term。

RoleBinding 是内部领域术语。

---

### 16.5 RoleBinding 不需要 Tenant

错误。

没有 Tenant，就无法表达同一 Subject 在不同授权域下持有不同 Role。

---

### 16.6 只写 Binding 记录就完成授权

错误。

运行时判定还依赖授权事实。

RoleBinding 写入需要同时保证管理面记录、运行时 g fact、PolicyVersion、Outbox、RuntimeReload 的一致性。

---

### 16.7 直接操作 Casbin g fact 就可以管理角色绑定

错误。

直接操作 g fact 会绕过：

```text
Subject 校验；
Role 校验；
Binding 管理面记录；
PolicyVersion；
Outbox event；
Runtime reload；
审计信息。
```

必须通过标准写入链路。

---

### 16.8 RoleName 可以随便改

错误。

RoleName 会进入 Permission、RoleBinding、运行时 p/g facts、Snapshot、审计和 SDK 语义。

如果需要改名，应把它看作一次授权事实迁移，而不是普通展示字段更新。

展示名称应该使用 DisplayName。

---

## 17. 代码事实源

本文只列角色模型相关入口，更完整的事实源索引见：

```text
07-AuthZ分层架构与事实源索引.md
```

主要代码事实源：

```text
internal/apiserver/domain/authz
internal/apiserver/application/authz
internal/apiserver/infra/casbin
configs/casbin_model.conf
```

重点关注：

| 主题 | 事实源 |
| --- | --- |
| Subject / SubjectRef | `domain/authz` |
| Tenant / Authorization Domain | `domain/authz` |
| Role / RoleName | `domain/authz` |
| RoleBinding / Binding / Fact | `domain/authz` |
| SubjectResolver | `domain/authz` 或 `application/authz` 相关 resolver |
| Role command | `application/authz` |
| RoleBinding command | `application/authz` |
| PolicyAdministration bind/unbind | `application/authz` |
| Casbin g fact 映射 | `infra/casbin` |
| Runtime matcher | `configs/casbin_model.conf` |

如果本文与代码不一致，以代码事实源为准，并同步更新本文档。

---

## 18. 后续文档入口

本文说明 Role / RoleBinding / Subject 模型。

后续继续阅读：

```text
03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md
04-授权版本与事件传播链路-PolicyVersion-Outbox-RuntimeReload.md
05-权限检查链路-Check-Snapshot.md
06-Casbin运行时模型-pgFacts与四段Matcher.md
07-AuthZ分层架构与事实源索引.md
```

其中：

```text
授权写入链路说明 RoleBinding 如何通过 PolicyChange / PolicyChangeCommitter 写入；
版本传播链路说明 RoleBinding 变更如何触发 PolicyVersion / Outbox / RuntimeReload；
权限检查链路说明 RoleBinding 如何参与 Check / Snapshot；
Casbin Runtime 文档说明 RoleBinding 如何映射为 g fact；
分层架构与事实源索引统一收口代码路径、表结构和维护原则。
```

---

## 19. 本文总结

本文讲的是 AuthZ 中 Subject、Role、RoleBinding 的模型边界。

核心关系是：

```text
Subject
  -> RoleBinding(Tenant)
  -> Role
  -> Permission
```

其中：

```text
Subject      表示被授权主体
Role         表示权限聚合点
RoleName     表示稳定业务角色标识
Tenant       表示授权域边界
RoleBinding  表示 Subject 在 Tenant 下持有 Role
Assignment   表示对外接口中的角色分配术语
```

如果只记住一句话：

> Role 是权限聚合点，Subject 不直接拥有 Permission，而是在某个 Tenant 下通过 RoleBinding 持有 Role；Assignment 是外部接口术语，RoleBinding 才是内部授权领域术语。
