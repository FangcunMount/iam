# 01-授权资源与动作模型：ResourceKey、ResourcePattern、Action、Scope

## 1. 本文定位

本文是 `03-授权AuthZ/` 文档组中关于 **资源、动作与作用范围** 的模型文档。

上一篇《00-AuthZ模型总览》已经建立了 AuthZ 的主线：

```text
Subject
  -> RoleBinding
  -> Role
  -> Permission
  -> Resource / Action / Scope
  -> AuthorizationDecision
```

本文聚焦其中这一段：

```text
Permission
  -> Resource / Action / Scope
```

也就是回答：

```text
权限到底作用在哪个资源上？
权限允许执行什么动作？
权限作用范围是什么？
ResourceKey 和 ResourcePattern 为什么要分开？
Action 和 ActionPattern 为什么要分开？
Scope 为什么不能直接拼进 ResourceKey？
```

本文不展开 RoleBinding、PolicyChange、UoW、Outbox、Casbin 运行时细节。

这些内容分别放在：

```text
02-授权角色与绑定模型-Role-RoleBinding-Subject.md
04-授权写入链路-PolicyAdministration与PolicyChange.md
05-PolicyChangeCommitter与AuthZUoW.md
06-Casbin运行时模型-pgFacts与四段Matcher.md
07-PolicyVersion-Outbox与RuntimeReload.md
```

---

## 2. 30 秒结论

AuthZ 中资源与动作模型的核心是：

```text
ResourceKey      表示资源目录中的资源或资源族
ResourcePattern  表示授权事实或判定请求中的资源匹配模式
Action           表示一次请求要执行的具体动作
ActionPattern    表示授权事实中可匹配的动作表达式
Scope            表示权限作用的对象范围
```

它们共同组成 Permission 的能力声明：

```text
Permission = RoleName + TenantID + ResourcePattern + ActionPattern + Scope
```

一次 Check 请求则使用：

```text
AuthorizationRequest = Subject + TenantID + ResourcePattern + Action + ObjectScope
```

注意两个关键分离：

```text
ResourceKey != ResourcePattern
Action != ActionPattern
ResourceKey != Scope
```

一句话：

> Resource 表达“什么资源”，Action 表达“做什么动作”，Scope 表达“作用到哪些对象范围”；ResourceKey 和 Action 是具体语义，ResourcePattern 和 ActionPattern 是授权匹配语义。

---

## 3. 为什么资源模型不能直接使用 HTTP Path

一种常见做法是直接把 HTTP 接口路径当权限资源：

```text
GET /api/v1/users/:id
POST /api/v1/users
DELETE /api/v1/users/:id
```

这种方式短期看起来简单，但在 IAM 中不合适。

原因是：

| 问题 | 说明 |
| --- | --- |
| 协议耦合 | REST path 变化会影响权限模型 |
| 难以复用 | gRPC、SDK、后台任务没有 HTTP path |
| 业务语义弱 | `/api/v1/users/:id` 不如 `iam:identity:user:*` 稳定 |
| 动作混乱 | HTTP method 不能完整表达 approve、export、read_own 等业务动作 |
| 范围表达困难 | path 不适合表达 all/origin 等对象范围 |
| 版本治理困难 | 接口重构容易导致历史权限事实漂移 |

IAM 的做法是把资源抽象成稳定的授权语义：

```text
iam:identity:user:*
iam:authz:role:*
iam:authz:resource:*
qs:survey:questionnaire:*
qs:evaluation:report:*
```

HTTP path、gRPC method、SDK method 都可以映射到同一套 Resource / Action / Scope 模型。

因此：

```text
协议接口是接入层事实
ResourceKey 是授权领域事实
```

两者不能混为一谈。

---

## 4. ResourceKey：资源目录中的资源标识

### 4.1 ResourceKey 是什么

`ResourceKey` 是资源目录中的资源标识。

它回答：

```text
IAM 中有哪些受保护资源？
```

当前 IAM 使用四段结构：

```text
<app>:<domain>:<type>:<name-or-*>
```

例如：

```text
iam:identity:user:*
iam:authz:role:*
iam:authz:resource:*
qs:survey:questionnaire:*
qs:evaluation:report:*
```

四段分别表示：

| 段 | 含义 | 示例 |
| --- | --- | --- |
| app | 应用或系统边界 | `iam`、`qs` |
| domain | 业务域或子域 | `identity`、`authz`、`survey`、`evaluation` |
| type | 资源类型 | `user`、`role`、`questionnaire`、`report` |
| name | 资源名、资源族或资源实例模式 | `*`、`default`、`report-template` |

---

### 4.2 为什么必须是四段

早期如果只用两段：

```text
iam:user
iam:role
```

会遇到几个问题：

```text
资源命名空间不够清楚
跨业务域后容易冲突
无法区分 app / domain / type / name
Snapshot 按 app 投影困难
PolicyLinter 很难判断 resource 是否属于某个资源族
Casbin matcher 很难表达稳定的业务匹配规则
```

四段结构让资源语义更稳定：

```text
iam:identity:user:*
```

可以被拆解为：

```text
app: iam
domain: identity
type: user
name: *
```

这使系统可以明确回答：

```text
这个资源属于哪个 app？
属于哪个业务域？
是什么资源类型？
是单个资源，还是资源族？
```

---

### 4.3 ResourceKey 的约束

在当前设计中，ResourceKey 主要用于资源目录。

因此，它应该是相对具体、可登记、可治理的资源标识。

推荐约束是：

```text
必须是四段
每段不能为空
app/domain/type 不应为 *
name 可以是具体名称，也可以是 * 表示资源族
```

合法示例：

```text
iam:identity:user:*
iam:authz:role:*
qs:survey:questionnaire:*
qs:evaluation:report:default
```

不推荐或非法示例：

```text
iam:user                    # 非四段
iam:identity                # 非四段
*:identity:user:*            # app 不能是 *
iam:*:user:*                 # domain 不应是 *
iam:identity:*:*             # type 不应是 *
*:*:*:*                      # 这是 pattern，不应进入资源目录
```

ResourceKey 的职责是进入 ResourceCatalog，作为可管理的资源事实。

不要把过于宽泛的 pattern 当成资源目录项。

---

## 5. ResourcePattern：授权事实中的资源匹配模式

### 5.1 ResourcePattern 是什么

`ResourcePattern` 是授权事实或判定请求中的资源匹配表达式。

它回答：

```text
这条 Permission 能匹配哪些 Resource？
```

例如：

```text
iam:identity:user:*
iam:authz:*:*
qs:*:*:*
*:*:*:*
```

ResourcePattern 也使用四段结构：

```text
<app>:<domain>:<type>:<name-or-pattern>
```

但它比 ResourceKey 更偏向 matcher 语义。

---

### 5.2 ResourceKey 与 ResourcePattern 的区别

| 概念 | 主要场景 | 是否偏目录事实 | 是否偏匹配语义 | 示例 |
| --- | --- | --- | --- | --- |
| ResourceKey | ResourceCatalog | 是 | 否 | `iam:identity:user:*` |
| ResourcePattern | Permission / Check / Casbin fact | 否 | 是 | `iam:*:*:*`、`*:*:*:*` |

可以这样理解：

```text
ResourceKey 是资源目录语言。
ResourcePattern 是授权匹配语言。
```

例如：

```text
ResourceKey: iam:identity:user:*
ResourcePattern: iam:identity:user:*
```

这两者字符串可能相同，但语义不同。

前者表示：

```text
IAM 中登记了一个 user 资源族。
```

后者表示：

```text
某条 Permission 可以匹配 iam:identity:user:* 这类资源。
```

再看：

```text
ResourcePattern: *:*:*:*
```

这可以作为超级管理员权限的授权模式。

但不应该作为普通 ResourceCatalog 的 ResourceKey。

---

### 5.3 为什么不能只保留一个 ResourceKey

如果不区分 Key 和 Pattern，会出现两个问题。

第一，资源目录会被过宽 pattern 污染：

```text
*:*:*:*
qs:*:*:*
iam:*:*:*
```

这些不是具体资源目录项，而是授权匹配表达式。

第二，权限事实会被限制得过死：

```text
只能精确匹配某个 ResourceKey
无法表达 app 级 / domain 级 / type 级通配授权
```

因此，应该明确分层：

```text
ResourceCatalog 使用 ResourceKey
Permission / AuthorizationRequest 使用 ResourcePattern
Casbin p/r fact 使用 ResourcePattern 字符串
```

---

## 6. Action：请求侧的具体动作

### 6.1 Action 是什么

`Action` 是请求侧要执行的具体动作。

它回答：

```text
Subject 想对 Resource 做什么？
```

例如：

```text
create
read
read_all
read_own
update
delete
approve
export
```

Action 应该尽量使用业务语义，而不是机械照搬 HTTP method。

例如：

```text
GET /reports/:id      -> read
GET /reports          -> list
POST /reports/export  -> export
POST /reports/approve -> approve
```

---

### 6.2 Action 的约束

Action 是请求侧具体动作，所以它应该是一个明确 operation。

它不应该包含 matcher 表达式。

合法示例：

```text
read
list
create
update
delete
export
approve
```

不适合作为 Action 的示例：

```text
read|list
.*
read.*
```

这些应该属于 ActionPattern，而不是 Action。

原因是：

```text
请求侧要问的是“我能不能执行 read？”
授权事实可以回答“我允许 read 或 list”。
```

两者不能混在一起。

---

## 7. ActionPattern：授权事实中的动作匹配表达式

### 7.1 ActionPattern 是什么

`ActionPattern` 是 Permission 中的动作匹配表达式。

它回答：

```text
这条 Permission 可以覆盖哪些 Action？
```

例如：

```text
read
read|list
create|update|delete
.*
```

其中：

```text
read                 表示只允许 read
read|list            表示允许 read 或 list
create|update|delete 表示允许 create/update/delete
.*                   表示匹配全部动作
```

---

### 7.2 Action 与 ActionPattern 的区别

| 概念 | 主要场景 | 示例 | 语义 |
| --- | --- | --- | --- |
| Action | Check 请求 | `read` | 本次请求要执行的具体动作 |
| ActionPattern | Permission fact | `read&list` | 授权事实可匹配的动作集合或模式 |

一次判定可以理解为：

```text
request.action = read
policy.action_pattern = read|list
```

如果 `read` 能被 `read|list` 覆盖，则动作维度匹配成功。

---

### 7.3 为什么 ActionPattern 不能滥用

虽然 ActionPattern 允许表达匹配模式，但不应该滥用。

推荐优先使用明确动作集合：

```text
read|list
create|update|delete
```

谨慎使用：

```text
.*
```

因为 `.*` 很强，会掩盖资源目录和动作治理问题。

如果某个角色确实需要全动作能力，应该在文档和初始化权限中明确说明它是：

```text
super admin / system operator / service owner
```

否则更推荐使用明确动作集合。

---

## 8. Scope：权限作用范围

### 8.1 Scope 是什么

`Scope` 是权限作用范围。

它回答：

```text
这条权限能作用到哪些对象？
```

当前模型支持：

```text
all:*
origin:<value>
```

其中：

| Scope | 含义 |
| --- | --- |
| `all:*` | 全范围 |
| `origin:<value>` | 某个来源、归属或业务边界内的对象 |

例如：

```text
ResourcePattern: qs:evaluation:report:*
ActionPattern: read
Scope: all:*
```

表示：

```text
可以读取所有 evaluation report。
```

而：

```text
ResourcePattern: qs:evaluation:report:*
ActionPattern: read
Scope: origin:1001
```

表示：

```text
只能读取 origin 为 1001 的 evaluation report。
```

---

### 8.2 Scope 与 Resource 的边界

Resource 表达：

```text
什么资源类型？
```

Scope 表达：

```text
这个权限覆盖哪些对象范围？
```

因此，不推荐把对象范围塞进 ResourceKey。

不推荐：

```text
qs:evaluation:report:origin:1001
```

推荐：

```text
ResourcePattern: qs:evaluation:report:*
Action: read
Scope: origin:1001
```

这样做的好处是：

```text
ResourceKey 结构稳定
Scope matcher 独立演进
PolicyLinter 可以独立检查 resource/action/scope
未来扩展 scope hierarchy 时不破坏 resource model
```

---

### 8.3 Scope 的匹配语义

当前基本语义可以理解为：

```text
policy scope = all:*       可以覆盖任意 request scope
policy scope = origin:x    只能覆盖 request scope = origin:x
```

也就是：

| Policy Scope | Request Scope | 是否匹配 |
| --- | --- | --- |
| `all:*` | `all:*` | 是 |
| `all:*` | `origin:1001` | 是 |
| `origin:1001` | `origin:1001` | 是 |
| `origin:1001` | `origin:1002` | 否 |
| `origin:1001` | `all:*` | 否 |

这个语义非常重要。

它表达的是：

```text
all:* 是更宽权限。
origin:x 是更窄权限。
窄权限不能覆盖宽请求。
```

---

## 9. Permission 如何组合 Resource / Action / Scope

Permission 的核心结构是：

```text
RoleName
TenantID
ResourcePattern
ActionPattern
Scope
```

可以写成：

```text
Permission(role, tenant, resource_pattern, action_pattern, scope)
```

例如：

```text
Permission(
  role = iam:admin,
  tenant = default,
  resource_pattern = iam:identity:user:*,
  action_pattern = read|update|delete,
  scope = all:*
)
```

这条 Permission 表示：

```text
iam:admin 在 default tenant 下，
可以对 iam:identity:user:* 执行 read/update/delete，
作用范围是 all:*。
```

而一次请求是：

```text
AuthorizationRequest(
  subject = user:1001,
  tenant = default,
  resource_pattern = iam:identity:user:*,
  action = read,
  object_scope = all:*
)
```

判定时会检查：

```text
subject 是否通过 RoleBinding 持有 iam:admin
request tenant 是否匹配 permission tenant
request resource 是否匹配 permission resource_pattern
request action 是否匹配 permission action_pattern
request scope 是否被 permission scope 覆盖
```

---

## 10. ResourceCatalog 的职责

### 10.1 ResourceCatalog 是什么

`ResourceCatalog` 是资源目录。

它回答：

```text
当前系统有哪些可被授权保护的资源？
这些资源支持哪些动作？
这些资源支持哪些 scope kind？
```

一个资源目录项通常包括：

```text
ResourceKey
DisplayName
AppName
Domain
Type
Actions
ScopeKinds
Description
```

例如：

```text
ResourceKey: iam:identity:user:*
Actions: read, list, create, update, delete
ScopeKinds: all, origin
```

---

### 10.2 ResourceCatalog 的边界

ResourceCatalog 主要用于：

```text
资源登记
授权写入时校验 action 是否被资源支持
授权写入时校验 scope kind 是否被资源支持
PolicyLinter 检查已有 permission facts 是否与资源目录一致
```

但是，ResourceCatalog 变更不会自动删除已有 PermissionFacts。

也就是说：

```text
ResourceCatalog 是 grant-time validation catalog。
已有 PermissionFacts 不会因为 ResourceCatalog 更新而自动失效。
```

例如：

```text
某个 Resource 原本支持 export
后来 ResourceCatalog 移除了 export
```

这不会自动删除旧 Permission。

正确治理方式是：

```text
PolicyLinter 发现 unsupported_action
人工确认或未来 PolicyReconciler 生成修复计划
修复必须通过 PolicyChangeCommitter 进入写入链路
```

不能直接手动删除 `casbin_rule`。

---

## 11. PolicyLinter 在资源模型中的作用

Resource / Action / Scope 模型建立后，还需要治理已有授权事实。

因为现实中可能出现：

```text
ResourceCatalog 被修改
旧 PermissionFacts 仍存在
某个 action 已经不被资源支持
某个 scope kind 已经不被资源支持
某条 casbin_rule 是历史脏数据
```

PolicyLinter 负责做只读诊断。

它可以发现：

```text
missing_resource
unsupported_action
unsupported_scope_kind
invalid_permission_fact
uncheckable_action_pattern
```

注意：PolicyLinter 不负责自动修复。

它只是回答：

```text
当前授权事实是否与 ResourceCatalog 一致？
```

如果未来要自动修复，应该引入 PolicyReconciler，并且必须走：

```text
PolicyReconciler
  -> PolicyChange
  -> PolicyChangeCommitter
  -> PolicyVersion
  -> Outbox
  -> RuntimeReload
```

---

## 12. 与 Casbin matcher 的关系

Resource / Action / Scope 是领域概念。

Casbin matcher 是 infra runtime 实现。

当前 IAM 的 Casbin matcher 需要表达三类匹配：

```text
resourceMatch(r.obj, p.obj)
actionMatch(r.act, p.act)
scopeMatch(r.scope, p.scope)
```

含义是：

```text
request resource 是否匹配 policy resource pattern
request action 是否匹配 policy action pattern
request scope 是否被 policy scope 覆盖
```

但本文只解释领域模型。

具体 p/g facts、model.conf、resourceMatch、actionMatch、scopeMatch 的实现细节，放在：

```text
06-Casbin运行时模型-pgFacts与四段Matcher.md
```

这里需要记住的是：

```text
ResourcePattern / ActionPattern / Scope 是领域授权事实。
Casbin matcher 只是这些授权事实的运行时匹配实现。
```

不要反过来用 Casbin 的 p/g/r 术语污染领域模型。

---

## 13. 建模示例

### 13.1 IAM 用户管理资源

资源目录：

```text
ResourceKey: iam:identity:user:*
Actions: list, read, create, update, disable, enable
ScopeKinds: all, origin
```

管理员权限：

```text
Role: iam:admin
Tenant: default
ResourcePattern: iam:identity:user:*
ActionPattern: list|read|create|update|disable|enable
Scope: all:*
```

普通查看者权限：

```text
Role: iam:viewer
Tenant: default
ResourcePattern: iam:identity:user:*
ActionPattern: list|read
Scope: all:*
```

---

### 13.2 QS 测评报告资源

资源目录：

```text
ResourceKey: qs:evaluation:report:*
Actions: read, read_all, export
ScopeKinds: all, origin
```

测评师权限：

```text
Role: qs:evaluator
Tenant: tenant-a
ResourcePattern: qs:evaluation:report:*
ActionPattern: read|export
Scope: origin:1001
```

系统管理员权限：

```text
Role: qs:admin
Tenant: tenant-a
ResourcePattern: qs:evaluation:report:*
ActionPattern: read|read_all|export
Scope: all:*
```

---

### 13.3 超级管理员资源模式

超级管理员可能拥有：

```text
Role: iam:super_admin
Tenant: default
ResourcePattern: *:*:*:*
ActionPattern: .*
Scope: all:*
```

这表示：

```text
所有 app / domain / type / name 的资源
所有 action
所有 scope
```

这种权限很强，应该谨慎使用，并且不应把 `*:*:*:*` 当成普通 ResourceCatalog 的 ResourceKey。

---

## 14. 常见误区

### 14.1 ResourceKey 可以随便写

错误。

ResourceKey 应该是四段结构：

```text
<app>:<domain>:<type>:<name-or-*>
```

不要写成：

```text
user
iam:user
/api/v1/users
```

---

### 14.2 ResourcePattern 和 ResourceKey 是一回事

不准确。

字符串可能相同，但语义不同。

```text
ResourceKey 是资源目录事实。
ResourcePattern 是授权匹配表达式。
```

---

### 14.3 Action 可以写成 `read|list`

错误。

`read|list` 是 ActionPattern，不是 Action。

Action 应该是本次请求的具体动作。

---

### 14.4 Scope 可以拼进 ResourceKey

错误。

ResourceKey 表达资源类型或资源族。

Scope 表达对象范围。

两者应该分开。

---

### 14.5 ResourceCatalog 更新会自动删除旧权限

错误。

当前 ResourceCatalog 变更不自动 reconcile 旧 PermissionFacts。

应该通过 PolicyLinter 检查，再通过未来 PolicyReconciler 或人工操作走标准授权写入链路。

---

### 14.6 Casbin matcher 就是领域模型

错误。

Matcher 是 infra runtime 机制。

领域模型是 Resource / Action / Scope / Permission。

---

## 15. 代码事实源

本文涉及的主要代码事实源：

```text
internal/apiserver/domain/authz/resource
internal/apiserver/domain/authz/scope
internal/apiserver/domain/authz/permission
internal/apiserver/domain/authz/decision

internal/apiserver/application/authz/resource
internal/apiserver/application/authz/policy
internal/apiserver/application/authz/authorization
internal/apiserver/application/authz/policylint

internal/apiserver/infra/casbin
configs/casbin_model.conf
```

重点关注：

| 主题 | 事实源 |
| --- | --- |
| ResourceKey / ResourcePattern | `domain/authz/resource` |
| Action / ActionPattern | `domain/authz/resource` |
| Scope | `domain/authz/scope` |
| Permission | `domain/authz/permission` |
| AuthorizationRequest | `domain/authz/decision` |
| ResourceCatalog | `application/authz/resource` |
| Permission command | `application/authz/policy` |
| CheckCommand / SnapshotQuery | `application/authz/authorization` |
| PolicyLinter | `application/authz/policylint` |
| runtime matcher | `infra/casbin`、`configs/casbin_model.conf` |

如果本文与代码不一致，以代码事实源为准。

---

## 16. 本文总结

本文讲的是 AuthZ 中最容易混淆的一组模型：

```text
ResourceKey
ResourcePattern
Action
ActionPattern
Scope
```

它们的边界是：

```text
ResourceKey      资源目录事实
ResourcePattern  授权匹配表达式
Action           请求侧具体动作
ActionPattern    授权事实动作模式
Scope            对象作用范围
```

Permission 正是由这些模型组合而成：

```text
Permission = RoleName + TenantID + ResourcePattern + ActionPattern + Scope
```

Check 请求则使用：

```text
AuthorizationRequest = Subject + TenantID + ResourcePattern + Action + ObjectScope
```

如果只记住一句话：

> Resource 表达“什么资源”，Action 表达“做什么动作”，Scope 表达“作用到哪些对象范围”；ResourceKey 和 Action 是具体语义，ResourcePattern 和 ActionPattern 是授权匹配语义。
