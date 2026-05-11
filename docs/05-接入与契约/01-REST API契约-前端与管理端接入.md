# 01-REST API 契约：前端与管理端接入

## 1. 本文定位

本文是 `05-接入与契约/` 文档组中关于 **REST API 接入契约** 的文档。

上一篇《00-接入总览：业务系统如何接入 IAM》已经说明：IAM 对外提供三类接入形态：

```text
REST：前端、管理后台、调试、非 Go 调用方；
gRPC：服务间调用、内部系统集成；
Go SDK：Go 服务端项目的工程化封装。
```

本文聚焦 REST。

REST API 的核心使用者是：

```text
浏览器前端；
小程序；
移动端；
管理后台；
外部系统低门槛接入方；
调试人员；
非 Go 语言调用方。
```

本文回答：

```text
IAM REST API 面向哪些接入场景？
REST API 如何分组？
哪些接口适合前端调用？
哪些接口属于管理端接口？
请求 Header、Token、Tenant、Trace 如何传递？
REST API 的错误结构如何理解？
AuthN / Identity / AuthZ / IDP 在 REST 层如何暴露？
业务系统，例如 qs-server，应该如何使用 IAM REST？
REST 契约的事实源在哪里？
```

本文不试图替代 OpenAPI。

REST 的机器契约事实源应以项目中的 OpenAPI、REST handler 和 REST 测试为准。

本文负责解释：

```text
REST API 的接入方式；
REST API 的分组方式；
REST API 的安全边界；
REST API 与 AuthN / Identity / AuthZ / IDP 的对应关系；
REST API 与 gRPC / SDK 的分工。
```

---

## 2. 30 秒结论

IAM REST API 是面向前端、管理端和外部低门槛接入方的 HTTP 契约。

它不是 IAM 内部领域模型，也不是服务间高频调用的唯一通道。

REST API 的推荐分组是：

```text
AuthN REST：登录、刷新、退出、Token 验证、JWKS、当前 Principal；
Identity REST：User、Profile、ProfileLink；
AuthZ REST：Resource、Role、Permission、RoleBinding/Assignment、Check、Snapshot、PolicyLinter；
IDP REST：WeChat / WeCom app 管理与外部身份源配置；
System REST：health、ready、metrics、debug-only endpoint。
```

REST API 的基本接入链路是：

```text
Client
  -> IAM REST Login
  -> AccessToken / RefreshToken
  -> Client calls IAM REST or business API with Bearer Token
  -> IAM REST middleware verifies token / builds Principal
  -> Application service
  -> response
```

核心原则：

```text
REST 负责协议适配，不承载领域规则；
前端可以使用 REST 登录、刷新 Token、查询当前用户、调用管理端接口；
管理端可以使用 REST 管理用户、角色、资源、权限和角色绑定；
业务服务间高频调用优先使用 gRPC 或 SDK；
REST 文档解释接口语义，字段细节以 OpenAPI / handler / tests 为准。
```

---

## 3. REST API 的职责边界

### 3.1 REST API 适合什么

REST API 适合：

```text
前端登录；
管理后台；
人类调试；
跨语言低门槛接入；
外部系统通过 HTTP 集成；
需要 OpenAPI / Swagger 文档的场景。
```

例如：

```text
用户在前端登录 IAM；
管理后台创建角色；
管理后台给角色授予权限；
管理后台给用户绑定角色；
前端查询当前用户和 ProfileLink；
调试人员通过 curl 检查 AuthZ Check 返回。
```

---

### 3.2 REST API 不适合什么

REST API 不应该成为所有内部调用的默认方案。

以下场景更适合 gRPC 或 SDK：

```text
服务间高频 Token Verify；
服务间高频 AuthZ Check；
批量身份查询；
worker 调用 IAM；
内部服务调用 IAM；
需要统一重试、deadline、metadata 的 Go 服务。
```

REST 当然也能调用这些能力，但不是最高效或最工程化的服务间通道。

---

### 3.3 REST API 与 Application 的关系

REST handler 的职责是：

```text
绑定 HTTP request；
解析 path / query / body / header；
做基础参数校验；
转换为 Application Command / Query；
调用 Application service；
把 Application result 转成 HTTP response。
```

REST handler 不应该：

```text
直接操作 repository；
直接调用 Casbin Enforce；
直接签发 Token；
直接拼接 p/g facts；
直接递增 PolicyVersion；
直接写 Outbox；
承载领域不变量。
```

正确方向：

```text
REST Handler
  -> Application Command / Query
  -> Application Service
  -> Domain / Infra ports
```

---

## 4. REST 契约事实源

REST 契约必须以机器事实源为准。

常见事实源入口：

```text
api/rest
internal/apiserver/transport/rest
internal/apiserver/application
internal/apiserver/domain
REST integration tests
```

推荐事实优先级：

```text
OpenAPI YAML
  -> REST handler
  -> Application command/query
  -> tests
  -> docs/05 文档
```

如果本文与 OpenAPI / handler / tests 冲突，应优先相信代码和机器契约，并同步修正文档。

OpenAPI 的定位是：

```text
用机器可读、人类可读的方式描述 REST API；
支持生成参考文档；
支持生成客户端；
支持接口校验；
支持集成测试和调试工具。
```

REST 文档不应该手抄所有字段成为第二套事实源。

本文只保留：

```text
接口分组；
核心用途；
关键请求字段；
关键响应字段；
安全边界；
典型请求；
错误语义；
事实源入口。
```

---

## 5. 通用 HTTP 契约

### 5.1 Base URL

不同环境可以有不同 Base URL。

示例：

```text
Local:       http://localhost:8080
Staging:     https://iam-staging.example.com
Production:  https://iam.example.com
```

实际地址以部署配置为准。

REST 文档不应该把某个环境地址写死为唯一事实源。

---

### 5.2 API Version

推荐 REST API 使用稳定版本前缀。

示例：

```text
/api/v1/...
/api/v2/...
/api/v3/...
```

如果项目中同时存在多个版本，应明确：

```text
当前推荐版本；
兼容版本；
已废弃版本；
迁移路径；
移除时间。
```

文档中不要混用旧接口和新接口。

如果旧接口仍保留，应标注：

```text
deprecated；
compatibility-only；
do not use for new integration。
```

---

### 5.3 Content-Type

JSON REST API 推荐：

```http
Content-Type: application/json
Accept: application/json
```

错误响应如果采用 Problem Details 风格，可以使用：

```http
Content-Type: application/problem+json
```

具体格式以项目当前 OpenAPI 和 handler 为准。

---

### 5.4 Authentication Header

受保护接口应使用 Bearer Token：

```http
Authorization: Bearer <access_token>
```

Bearer Token 的基本语义是：

```text
任何持有该 Token 的调用方都可以使用它访问相关受保护资源。
```

因此：

```text
Token 必须通过 HTTPS 传输；
Token 不应写入日志；
Token 不应暴露在 URL query；
Token 不应长期保存在不安全存储中；
前端应处理过期和刷新；
服务端应验证签名、过期时间、issuer、audience 和撤销状态边界。
```

---

### 5.5 Tenant Header

多租户或授权域相关接口需要 Tenant 上下文。

常见方式：

```http
X-Tenant-ID: tenant-a
```

也可能通过 path、query 或 token claims 传递。

项目应统一约定优先级：

```text
明确请求参数；
Header；
Token claims；
默认 tenant。
```

如果多个来源同时存在，必须避免冲突。

推荐规则：

```text
管理接口显式传 tenant；
业务接口由业务系统从上下文确定 tenant；
IAM 不应无声地猜测 tenant。
```

---

### 5.6 Trace Header

推荐所有 REST 请求携带请求追踪信息：

```http
X-Request-ID: <request-id>
X-Trace-ID: <trace-id>
```

如果客户端未传，服务端可以生成。

这些字段用于：

```text
链路追踪；
错误排查；
审计关联；
跨 IAM / qs-server / worker 关联请求。
```

---

## 6. 通用响应结构

### 6.1 成功响应

REST 成功响应可以采用项目统一响应结构。

常见结构：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

或者直接返回资源对象。

无论采用哪种形式，应保持：

```text
OpenAPI 与 handler 一致；
REST 文档与 OpenAPI 一致；
SDK / 前端生成代码与 OpenAPI 一致。
```

---

### 6.2 分页响应

列表接口应明确分页模型。

常见字段：

```text
items；
next_cursor；
page；
page_size；
total；
```

推荐避免在同一 API 中混用多种分页风格。

如果使用 cursor pagination，应说明：

```text
cursor 是否稳定；
排序字段是什么；
是否支持反向分页；
游标过期如何处理。
```

---

### 6.3 错误响应

错误响应至少应表达：

```text
HTTP status；
IAM error code；
message；
request_id；
details(optional)。
```

如果采用 Problem Details 风格，典型结构是：

```json
{
  "type": "https://iam.example.com/problems/permission-denied",
  "title": "Permission denied",
  "status": 403,
  "detail": "The subject is not allowed to access this resource.",
  "instance": "/api/v1/authz/check",
  "code": "authz.permission_denied",
  "request_id": "req_xxx"
}
```

Problem Details 的价值是让 HTTP API 能在状态码之外提供机器可读的错误细节。

项目不一定必须完全采用 RFC 7807 格式，但错误响应必须稳定、可解析、可排查。

---

## 7. HTTP 状态码语义

推荐状态码语义：

| 状态码 | 语义 |
| --- | --- |
| `200` | 成功 |
| `201` | 创建成功 |
| `202` | 已接受异步处理 |
| `204` | 成功且无响应体 |
| `400` | 请求参数错误 |
| `401` | 未认证或 Token 无效 |
| `403` | 已认证但无权限 |
| `404` | 资源不存在 |
| `409` | 状态冲突或唯一性冲突 |
| `422` | 语义校验失败 |
| `429` | 触发限流 |
| `500` | 服务端内部错误 |
| `503` | 服务暂不可用 |

重点边界：

```text
401 = 没有通过认证；
403 = 认证通过，但没有权限；
400 = 请求格式或基础参数错误；
422 = 请求格式正确，但业务语义不合法；
409 = 与现有状态冲突。
```

---

## 8. AuthN REST API

AuthN REST API 面向登录、Token、Session、认证主体。

它主要服务：

```text
前端登录；
前端刷新 Token；
前端退出登录；
业务系统调试 Token；
管理端查看当前 Principal。
```

### 8.1 登录

登录接口负责：

```text
接收登录请求；
校验登录证明；
产出 Principal；
签发 AccessToken / RefreshToken；
返回登录结果。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST /api/v3/authn/login
POST /api/v3/auth/login
```

典型请求：

```json
{
  "method": "password",
  "realm": "default",
  "identifier": "alice@example.com",
  "password": "***"
}
```

典型响应：

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 3600,
  "token_type": "Bearer",
  "principal": {
    "user_id": "u_123",
    "subject": "user:u_123",
    "tenant_id": "tenant-a"
  }
}
```

注意：

```text
Login 的领域终点是 Principal；
Token 签发属于 Token 链路；
REST 登录只是 AuthN 应用能力的协议投影。
```

---

### 8.2 刷新 Token

刷新接口负责：

```text
使用 RefreshToken 换取新的 AccessToken；
按策略轮换 RefreshToken；
维护 Session / Token 状态；
记录 Token audit。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST /api/v3/authn/token/refresh
POST /api/v3/auth/refresh
```

典型请求：

```json
{
  "refresh_token": "..."
}
```

典型响应：

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

---

### 8.3 退出登录

退出登录接口负责：

```text
撤销当前 Session；
撤销或失效 RefreshToken；
使后续刷新失败；
根据策略处理 AccessToken 剩余有效期。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST /api/v3/authn/logout
POST /api/v3/auth/logout
```

---

### 8.4 Token 验证

Token 验证接口负责：

```text
校验 Token 是否有效；
返回 Principal / claims / status；
根据策略检查 Session、撤销状态、账号状态。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST /api/v3/authn/token/verify
POST /api/v3/auth/verify
```

注意：

```text
服务间高频 VerifyToken 更推荐 gRPC / SDK；
REST Verify 适合调试、管理端或外部 HTTP 集成。
```

---

### 8.5 JWKS

JWKS endpoint 用于发布公钥集合。

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET /api/v3/authn/jwks
GET /api/v3/auth/jwks
GET /.well-known/jwks.json
```

JWKS 只应该发布可公开的验签公钥。

不应该发布：

```text
private key；
HMAC secret；
RefreshToken secret；
IDP AppSecret。
```

JWKS 用于本地验签，但本地验签不一定等于完整在线认证状态检查。

如果业务需要强状态控制，仍可能需要远程 VerifyToken。

---

### 8.6 当前 Principal / Me

当前主体接口用于返回当前认证主体摘要。

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET /api/v3/authn/me
GET /api/v3/me
```

典型响应：

```json
{
  "user_id": "u_123",
  "subject": "user:u_123",
  "tenant_id": "tenant-a",
  "login_identity_id": "li_123",
  "auth_method": "password"
}
```

---

## 9. Identity REST API

Identity REST API 面向 User、Profile、ProfileLink。

它主要服务：

```text
管理后台；
前端当前用户页面；
业务系统身份关系查询；
Profile 管理；
用户与儿童档案关系管理。
```

### 9.1 User

User 接口负责：

```text
查询用户；
创建用户；
更新用户基本信息；
禁用 / 启用用户；
批量查询用户。
```

典型资源路径以当前 OpenAPI 为准，可能类似：

```http
GET    /api/v3/identity/users/{user_id}
GET    /api/v3/identity/users
POST   /api/v3/identity/users
PATCH  /api/v3/identity/users/{user_id}
```

User 是 Identity 主体。

不要把 LoginIdentity、Credential、RoleBinding 混进 User 接口。

---

### 9.2 Profile

Profile 接口负责：

```text
查询 Profile；
创建 Profile；
更新 Profile；
管理儿童档案或业务身份档案。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET    /api/v3/identity/profiles/{profile_id}
GET    /api/v3/identity/profiles
POST   /api/v3/identity/profiles
PATCH  /api/v3/identity/profiles/{profile_id}
```

---

### 9.3 ProfileLink

ProfileLink 接口负责：

```text
建立 User 与 Profile 的关系；
查询某个 User 关联的 Profile；
查询某个 Profile 关联的 User；
解除关系；
修改关系类型。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET    /api/v3/identity/profile-links
POST   /api/v3/identity/profile-links
DELETE /api/v3/identity/profile-links/{link_id}
```

注意：

```text
ProfileLink 是身份关系，不是资源权限。
```

如果业务操作需要访问控制，仍应进入 AuthZ Check。

---

## 10. AuthZ REST API

AuthZ REST API 面向资源、角色、权限、角色绑定、授权检查和授权快照。

它主要服务：

```text
管理后台；
权限配置页面；
调试授权问题；
外部 HTTP 接入方；
低频 Check / Snapshot 调用。
```

### 10.1 Resource Catalog

Resource Catalog 接口负责：

```text
注册资源；
查询资源；
维护资源支持的 Action；
维护资源支持的 ScopeKind；
为授权写入提供校验依据；
为 PolicyLinter 提供资源目录事实。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET    /api/v3/authz/resources
POST   /api/v3/authz/resources
GET    /api/v3/authz/resources/{resource_id}
PATCH  /api/v3/authz/resources/{resource_id}
```

ResourceKey 应使用四段结构：

```text
<app>:<domain>:<type>:<name-or-*>
```

---

### 10.2 Role

Role 接口负责：

```text
创建角色；
查询角色；
更新角色展示信息；
禁用或删除角色；
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET    /api/v3/authz/roles
POST   /api/v3/authz/roles
GET    /api/v3/authz/roles/{role_id}
PATCH  /api/v3/authz/roles/{role_id}
```

RoleName 是稳定业务角色标识。

DisplayName 是展示名称。

不要把 Role 简化为 User 表上的字符串字段。

---

### 10.3 Permission

Permission 接口负责：

```text
给 Role 授予 Resource / Action / Scope 能力；
从 Role 撤销某条 Permission；
查询 Role 当前权限。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST   /api/v3/authz/roles/{role_id}/permissions
DELETE /api/v3/authz/roles/{role_id}/permissions/{permission_id}
GET    /api/v3/authz/roles/{role_id}/permissions
```

内部链路必须进入：

```text
PolicyAdministration
  -> AuthorizationPolicy
  -> PolicyChange
  -> PolicyChangeCommitter
```

REST handler 不应直接写 casbin_rule。

---

### 10.4 RoleBinding / Assignment

对外接口可以使用 Assignment 术语。

内部领域模型应使用 RoleBinding。

REST 接口负责：

```text
给 Subject 分配 Role；
撤销 Subject 的 Role；
查询某个 Subject 的角色绑定；
查询某个 Role 的绑定对象。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST   /api/v3/authz/assignments
DELETE /api/v3/authz/assignments/{assignment_id}
GET    /api/v3/authz/assignments
```

边界：

```text
assignment = REST wire term；
rolebinding = internal domain term；
binding record = management DB term；
g fact = Casbin runtime term。
```

---

### 10.5 Check

Check 接口负责一次授权判定。

典型路径以当前 OpenAPI 为准，可能类似：

```http
POST /api/v3/authz/check
```

典型请求：

```json
{
  "subject": "user:u_123",
  "tenant_id": "tenant-a",
  "resource": "qs:evaluation:report:*",
  "action": "read",
  "scope": "origin:p_123"
}
```

典型响应：

```json
{
  "allowed": true,
  "reason": "matched",
  "matched_role": "qs:evaluator",
  "matched_permission": "qs:evaluation:report:* read origin:p_123",
  "policy_version": 17
}
```

注意：

```text
Check 是权威判定；
Snapshot 不是 Check 的替代品；
前端不应该直接用 Snapshot 自行决定最终访问权。
```

---

### 10.6 Snapshot

Snapshot 接口负责查询授权视图。

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET /api/v3/authz/snapshot?subject=user:u_123&tenant_id=tenant-a&app=qs
```

典型响应：

```json
{
  "subject": "user:u_123",
  "tenant_id": "tenant-a",
  "roles": ["qs:evaluator"],
  "permissions": [],
  "policy_version": 17
}
```

Snapshot 适合：

```text
管理后台展示；
调试；
前端按钮展示；
SDK 缓存授权视图。
```

---

### 10.7 PolicyLinter

PolicyLinter 接口负责只读诊断授权事实。

典型能力：

```text
检查 missing_resource；
检查 unsupported_action；
检查 unsupported_scope_kind；
检查 invalid_permission_fact；
返回 diagnosis report。
```

PolicyLinter 不负责自动修复。

未来自动修复必须走：

```text
PolicyReconciler
  -> PolicyChange
  -> PolicyChangeCommitter
```

---

## 11. IDP REST API

IDP REST API 面向外部身份源配置。

当前重点是：

```text
WeChat Mini Program；
WeChat Official Account；
WeCom；
其他外部 IDP 扩展。
```

IDP REST 接口通常属于管理端接口。

它负责：

```text
创建 IDP app；
更新 IDP app 配置；
禁用 / 启用 IDP app；
查询 IDP app；
轮换或更新 AppSecret；
```

注意：

```text
IDP AppSecret 属于敏感配置；
REST response 不应返回明文 secret；
secret 更新和轮换必须有审计；
IDP 配置不等于 LoginIdentity；
LoginIdentity 属于 AuthN。
```

典型路径以当前 OpenAPI 为准，可能类似：

```http
GET    /api/v3/idp/wechat-apps
POST   /api/v3/idp/wechat-apps
PATCH  /api/v3/idp/wechat-apps/{app_id}
```

---

## 12. System REST API

System API 用于运行时健康检查、就绪检查和观测。

常见接口：

```http
GET /health
GET /ready
GET /metrics
```

语义建议：

```text
health：进程是否存活；
ready：依赖是否可用，是否可以接流量；
metrics：Prometheus metrics；
```

注意：

```text
metrics 通常不应公开暴露到公网；
debug endpoint 应只在内网或开发环境启用；
ready 应覆盖关键依赖，例如 MySQL、Redis、Casbin runtime、Outbox relay 状态。
```

---

## 13. 前端接入建议

### 13.1 前端登录

前端通过 REST 调用登录接口。

流程：

```text
1. 用户输入登录证明；
2. 前端调用 IAM Login；
3. IAM 返回 AccessToken / RefreshToken；
4. 前端保存 Token；
5. 前端调用业务 API 时携带 Bearer Token。
```

---

### 13.2 前端保存 Token

前端保存 Token 必须考虑安全风险。

建议：

```text
优先使用 HTTPS；
不要把 Token 放在 URL；
不要把 Token 打进日志；
根据应用形态选择 HttpOnly Cookie 或内存存储；
RefreshToken 存储要比 AccessToken 更谨慎；
登出时清理本地 Token。
```

具体策略由前端类型和安全要求决定。

---

### 13.3 前端权限展示

前端可以使用 Snapshot 或后端聚合接口控制按钮展示。

但要明确：

```text
前端展示控制不是最终权限控制。
```

最终访问控制必须由后端调用 IAM AuthZ Check 或由 IAM 管理接口自身执行权限检查。

---

## 14. 管理后台接入建议

管理后台通常需要：

```text
用户管理；
身份关系管理；
登录身份管理；
角色管理；
资源目录管理；
权限管理；
角色绑定管理；
PolicyLinter；
授权快照；
```

管理后台自身也必须经过 AuthZ。

例如：

```text
只有 iam:admin 可以创建角色；
只有 iam:admin 可以给用户绑定角色；
只有 iam:security_admin 可以查看 PolicyLinter；
```

不要认为管理后台登录成功就拥有所有管理权限。

登录属于 AuthN。

管理操作仍应进入 AuthZ。

---

## 15. qs-server 使用 REST 的边界

qs-server 作为 Go 后端服务，推荐优先使用 SDK / gRPC。

但 REST 仍可用于：

```text
本地调试；
管理后台；
非 Go 客户端；
前端直接登录 IAM；
临时运维验证。
```

qs-server 业务运行时不建议大量使用 REST 做高频内部调用。

例如：

```text
高频 VerifyToken；
高频 AuthZ Check；
批量 User 查询；
```

更推荐：

```text
gRPC；
Go SDK；
本地 Token verifier + 远程 Check 组合。
```

完整 qs-server 接入链路见：

```text
04-业务系统接入链路-以qs-server为例.md
```

---

## 16. REST 与 gRPC / SDK 的关系

REST、gRPC、SDK 不是三套业务模型。

它们是同一套 IAM 能力的三种接入投影。

```text
REST -> HTTP/JSON projection
gRPC -> proto service projection
SDK  -> Go client projection
```

它们背后的应用能力应该一致：

```text
AuthN Application；
Identity Application；
AuthZ Application；
IDP Application。
```

如果 REST 和 gRPC 表达同一能力，应保证：

```text
语义一致；
错误语义一致；
权限要求一致；
版本策略一致；
审计行为一致；
```

字段命名可以因协议风格不同而有所差异，但业务语义不能漂移。

---

## 17. 安全边界

### 17.1 公开接口与管理接口

REST API 应区分：

```text
公开接口；
已认证用户接口；
管理员接口；
内部调试接口。
```

示例：

```text
Login：公开接口；
Me：已认证用户接口；
CreateRole：管理员接口；
Metrics：内部观测接口。
```

---

### 17.2 管理接口必须二次授权

管理员登录成功不等于拥有所有权限。

管理接口仍应调用 AuthZ。

例如：

```text
POST /authz/roles
  -> require iam:authz:role:* create all:*
```

---

### 17.3 敏感信息不出现在响应中

REST response 不应返回：

```text
password hash；
Credential material；
RefreshToken secret hash；
private key；
HMAC secret；
IDP AppSecret 明文；
SMS OTP；
Challenge secret；
```

---

### 17.4 日志中不记录敏感字段

REST middleware 和 handler 不应记录：

```text
Authorization header；
password；
refresh_token；
access_token；
AppSecret；
SMS OTP；
private key；
```

可以记录：

```text
request_id；
user_id；
subject；
tenant_id；
endpoint；
status；
latency；
error code。
```

---

## 18. REST 契约维护规则

### 18.1 REST 文档不替代 OpenAPI

本文只解释 REST 契约。

字段级契约必须以 OpenAPI 为准。

如果新增、删除、修改 REST endpoint，应同步更新：

```text
OpenAPI；
REST handler；
REST tests；
docs/05-接入与契约/01-REST API契约-前端与管理端接入.md；
README / 00 接入总览中的相关入口。
```

---

### 18.2 Breaking Change

以下通常属于 breaking change：

```text
删除 endpoint；
修改 path；
修改 HTTP method；
删除响应字段；
修改字段类型；
修改枚举语义；
新增 required 请求字段；
修改错误码语义；
改变认证要求；
改变权限要求。
```

必须明确版本策略和迁移路径。

---

### 18.3 Deprecated endpoint

如果接口废弃，应在 OpenAPI 和文档中标明：

```text
deprecated；
replacement；
remove_after；
migration guide。
```

不要只在代码里保留旧接口，却不在文档中说明。

---

## 19. 验证建议

修改 REST 契约后，建议运行：

```bash
make docs-swagger
make api-validate
```

以及 REST 相关测试：

```bash
go test ./internal/apiserver/transport/rest/...
```

如果变更影响 Application command / query，还应运行：

```bash
go test ./internal/apiserver/application/...
```

如果变更影响 SDK 生成或 SDK 封装，还应运行：

```bash
go test ./pkg/sdk/...
```

具体命令以项目 Makefile 和 CI 为准。

---

## 20. 后续文档入口

本文说明 REST API 契约。

继续阅读：

```text
02-gRPC API契约-服务间调用与内部集成.md
03-SDK接入模型-Go服务端集成.md
04-业务系统接入链路-以qs-server为例.md
05-契约事实源与防漂移机制.md
```

其中：

```text
第 02 篇说明服务间调用契约；
第 03 篇说明 Go 服务端 SDK 封装；
第 04 篇说明 qs-server 如何组合 REST / gRPC / SDK 接入 IAM；
第 05 篇说明 OpenAPI / proto / SDK / docs 如何防漂移。
```

---

## 21. 本文总结

REST API 是 IAM 面向前端、管理后台、调试和外部低门槛接入方的 HTTP 契约。

它承载：

```text
AuthN：登录、刷新、退出、Token 验证、JWKS、Me；
Identity：User、Profile、ProfileLink；
AuthZ：Resource、Role、Permission、Assignment、Check、Snapshot、PolicyLinter；
IDP：WeChat / WeCom app 管理；
System：health、ready、metrics。
```

REST API 的核心边界是：

```text
REST handler 只做协议适配；
Application 承载用例编排；
Domain 承载模型和不变量；
Infra 承载数据库、Token、Casbin、Outbox 等技术实现；
OpenAPI 是 REST 字段级机器契约事实源。
```

如果只记住一句话：

> REST API 是 IAM 面向前端与管理端的 HTTP 接入投影，它解释“如何通过 HTTP 使用 IAM”，但字段级事实源必须以 OpenAPI、REST handler 和测试为准。
