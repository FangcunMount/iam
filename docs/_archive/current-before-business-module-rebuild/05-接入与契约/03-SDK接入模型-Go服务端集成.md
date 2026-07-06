# 03-SDK 接入模型：Go 服务端集成

## 1. 本文定位

本文是 `05-接入与契约/` 文档组中关于 **Go SDK 接入模型** 的文档。

前面几篇已经说明：

```text
00-接入总览：业务系统如何接入 IAM；
01-REST API 契约：前端与管理端接入；
02-gRPC API 契约：服务间调用与内部集成。
```

本文聚焦 Go SDK。

Go SDK 的核心使用者是：

```text
Go 业务后端，例如 qs-server；
Go worker，例如 qs-worker；
Go 采集服务，例如 collection-server；
Go gateway；
Go internal service；
需要接入 IAM 的 Go 项目。
```

本文回答：

```text
IAM SDK 的定位是什么？
SDK 和 REST / gRPC 是什么关系？
Go 服务端项目应该如何初始化 SDK client？
SDK 如何处理 Token 验证、Principal 注入、Identity 查询、AuthZ Check？
SDK 是否应该本地缓存权限？
SDK 如何封装 deadline、metadata、retry、错误映射？
qs-server 应该如何通过 SDK 接入 IAM？
SDK 的事实源在哪里？
SDK public API 如何防止漂移？
```

本文不试图替代 SDK 代码和 example。

SDK 字段级、方法级事实源应以：

```text
pkg/sdk
SDK examples
SDK tests
REST / gRPC 机器契约
```

为准。

本文负责解释：

```text
SDK 的工程定位；
SDK 的分层边界；
SDK 的典型使用方式；
SDK 与业务系统的集成模式；
SDK 的错误、超时、重试、缓存、安全边界；
SDK 契约如何防漂移。
```

---

## 2. 30 秒结论

IAM Go SDK 是 Go 服务端接入 IAM 的工程化封装。

它不是 IAM Server 的本地副本，也不是本地授权引擎。

SDK 的职责是：

```text
封装 REST / gRPC 调用；
统一配置；
统一 metadata；
统一 timeout / deadline；
统一错误映射；
提供 TokenVerifier；
提供 AuthN / Identity / AuthZ / IDP client；
提供 middleware / interceptor 集成辅助；
提供测试 fake / mock 便利能力。
```

SDK 不应该：

```text
直接访问 IAM 数据库；
直接读取 casbin_rule；
直接拼接 p/g facts；
本地复制 Role / Permission / RoleBinding；
绕过 IAM AuthZ Check 做最终权限判定；
暴露 internal 包；
隐藏认证、授权、安全边界。
```

推荐 Go 服务端接入链路：

```text
service startup
  -> load IAM SDK config
  -> sdk.NewClient(config)
  -> install auth middleware / interceptor

request path
  -> extract Bearer Token
  -> sdk.AuthN().VerifyToken / TokenVerifier
  -> inject Principal into context
  -> handler builds Resource / Action / Scope
  -> sdk.AuthZ().Check / Allow / AllowScoped
  -> allow / deny business operation
```

一句话：

> SDK 是 Go 服务接入 IAM 的客户端产品层：它负责把 REST/gRPC 契约封装成稳定、易用、可测试的 Go API，但认证、身份、授权事实和最终权限判定仍然由 IAM Server 负责。

---

## 3. SDK 与 REST / gRPC 的关系

REST、gRPC、SDK 是同一套 IAM 能力的三种接入投影。

```text
REST -> HTTP/JSON projection
gRPC -> proto service projection
SDK  -> Go client projection
```

SDK 可以封装 REST，也可以封装 gRPC。

对 Go 服务端来说，推荐优先：

```text
服务间高频调用：SDK -> gRPC；
前端和管理端：REST；
本地调试：REST / SDK example；
跨语言服务：gRPC 或 REST，视语言生态而定。
```

SDK 不应该创造一套新的业务语义。

例如，以下语义必须保持一致：

```text
AuthN Login / Refresh / VerifyToken；
Identity User / Profile / ProfileLink；
AuthZ Check / Snapshot / Assignment；
IDP app 配置；
错误码；
认证与授权要求；
PolicyVersion；
```

如果 SDK 与 REST / gRPC 对同一能力表达不一致，应优先检查：

```text
OpenAPI；
proto；
server implementation；
SDK implementation；
SDK tests。
```

---

## 4. SDK 不是什么

### 4.1 SDK 不是 IAM Server

SDK 不拥有 IAM 的事实源。

它不应该保存：

```text
User 表；
LoginIdentity 表；
Credential 表；
Session 表；
RefreshToken 表；
Role 表；
Permission facts；
RoleBinding facts；
casbin_rule 表。
```

这些都属于 IAM Server 和 IAM 数据库。

---

### 4.2 SDK 不是本地授权引擎

SDK 可以提供：

```text
Authz().Check；
Authz().Allow；
Authz().AllowScoped；
Authz().GetAuthorizationSnapshot。
```

但默认不应该：

```text
把所有 Permission 拉到本地；
在业务服务中重建 Casbin Enforcer；
让业务服务自己维护 PolicyVersion / Outbox / RuntimeReload；
让业务服务直接消费 IAM p/g facts。
```

最终访问控制应由 IAM AuthZ Check 判定。

如果未来确实需要边缘本地判定，也必须明确：

```text
PolicyVersion；
cache invalidation；
TTL；
Outbox event；
一致性等级；
安全边界。
```

这不应成为默认 SDK 模式。

---

### 4.3 SDK 不是业务规则层

SDK 不应该知道 qs-server 的业务规则。

例如 SDK 不应该内置：

```text
测评报告只有创建人能看；
儿童 Profile 的 guardian 可以读取报告；
某个问卷类型只能医生查看；
某个机构角色可以导出数据。
```

这些属于业务系统。

业务系统负责把业务语义转换为 IAM AuthZ 请求：

```text
Resource；
Action；
ObjectScope。
```

然后交给 IAM 判定。

---

## 5. SDK 包结构建议

SDK public API 应稳定、清晰、面向接入方。

推荐结构：

```text
pkg/sdk
├── client.go
├── config.go
├── errors.go
├── authn
├── identity
├── authz
├── idp
├── middleware
├── grpc
├── rest
├── jwt
└── testing
```

实际目录以当前代码为准。

建议边界：

| 包 | 职责 |
| --- | --- |
| `sdk` | Client facade、Config、Option、公共入口 |
| `sdk/authn` | Login、Refresh、VerifyToken、Principal |
| `sdk/identity` | User、Profile、ProfileLink 查询 |
| `sdk/authz` | Check、Allow、Snapshot、Assignment client |
| `sdk/idp` | IDP app 查询或管理封装 |
| `sdk/middleware` | HTTP/gRPC middleware / interceptor 辅助 |
| `sdk/jwt` | JWKS manager、本地验签辅助 |
| `sdk/testing` | fake client、stub、test helper |

禁止接入方依赖：

```text
internal/apiserver/...；
internal/pkg/... 中非公开包；
IAM server 内部 domain 对象；
IAM infra repository；
Casbin enforcer。
```

---

## 6. SDK 初始化模型

### 6.1 Config

SDK 初始化应通过显式配置。

推荐配置项：

```go
package example

type IAMConfig struct {
    Endpoint       string
    GRPCEndpoint   string
    ServiceName    string
    ServiceToken   string
    Timeout        time.Duration
    Retry          RetryConfig
    TLS            TLSConfig
    JWKS           JWKSConfig
    DefaultTenant  string
}
```

实际类型以 `pkg/sdk` 为准。

配置来源可以是：

```text
配置文件；
环境变量；
Kubernetes Secret / ConfigMap；
启动参数；
测试代码显式传入。
```

不建议 SDK 自行读取太多全局环境变量。

更好的方式是：

```text
业务服务负责加载配置；
再把 Config 显式传给 SDK。
```

---

### 6.2 NewClient

推荐入口形态：

```go
client, err := sdk.NewClient(sdk.Config{
    Endpoint:     cfg.IAM.Endpoint,
    GRPCEndpoint: cfg.IAM.GRPCEndpoint,
    ServiceName:  "qs-server",
    ServiceToken: cfg.IAM.ServiceToken,
    Timeout:      2 * time.Second,
})
if err != nil {
    return err
}
```

如果存在 option 模式，也可以：

```go
client, err := sdk.NewClientWithOptions(
    sdk.WithEndpoint(cfg.IAM.Endpoint),
    sdk.WithGRPCEndpoint(cfg.IAM.GRPCEndpoint),
    sdk.WithServiceName("qs-server"),
    sdk.WithServiceToken(cfg.IAM.ServiceToken),
    sdk.WithTimeout(2*time.Second),
)
```

实际方法名以当前 SDK 为准。

文档示例表达使用方式，不作为最终 API 名称承诺。

---

### 6.3 Client Facade

SDK Client 应提供稳定 facade：

```go
client.AuthN()
client.Identity()
client.AuthZ()
client.IDP()
client.Close()
```

调用方不应该关心：

```text
底层使用 REST 还是 gRPC；
具体 proto client；
HTTP client 细节；
metadata 注入细节；
连接池细节。
```

这些由 SDK 管理。

---

## 7. Context / Deadline 模型

SDK 所有网络调用都应该接受 `context.Context`。

原因是 Go 的 `context.Context` 用于在 API 边界和进程间传递 deadline、cancellation signal 和 request-scoped values。

推荐 API 形态：

```go
principal, err := client.AuthN().VerifyToken(ctx, token)

decision, err := client.AuthZ().Check(ctx, authz.CheckRequest{
    Subject:  subject,
    TenantID: tenantID,
    Resource: "qs:evaluation:report:*",
    Action:   "read",
    Scope:    "origin:" + profileID,
})
```

SDK 内部可以提供默认 timeout，但不应忽略调用方传入的 context。

推荐原则：

```text
调用方传入 ctx；
SDK 在没有 deadline 时可附加默认 timeout；
SDK 不应吞掉 ctx cancellation；
SDK 应在请求完成后释放 timer；
长链路中应传播 request-id / trace-id。
```

业务侧示例：

```go
ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
defer cancel()

principal, err := iam.AuthN().VerifyToken(ctx, accessToken)
```

---

## 8. Metadata / Header 注入

SDK 应统一处理服务间调用 metadata。

### 8.1 gRPC metadata

gRPC 调用应注入：

```text
authorization: Bearer <service_token>
x-caller-service: qs-server
x-request-id: req_xxx
x-trace-id: trace_xxx
```

如果调用有 tenant 上下文，也可以注入：

```text
x-tenant-id: tenant-a
```

但业务语义字段，例如 AuthZ Check 的 tenant，应放在 request message 中。

---

### 8.2 REST header

REST 调用应注入：

```http
Authorization: Bearer <access_token-or-service-token>
X-Request-ID: req_xxx
X-Trace-ID: trace_xxx
X-Caller-Service: qs-server
```

---

### 8.3 脱敏规则

SDK 日志不应记录：

```text
Authorization header；
access token；
refresh token；
service token；
password；
AppSecret；
private key；
```

可以记录：

```text
method；
endpoint；
caller-service；
tenant；
request-id；
trace-id；
status code；
latency；
error code。
```

---

## 9. AuthN SDK

AuthN SDK 负责认证相关调用。

典型能力：

```text
Login；
RefreshToken；
VerifyToken；
GetPrincipal / Me；
JWKS；
Service Token。
```

### 9.1 Login

Login 适合受信任 Go 服务或服务端聚合场景。

前端登录通常直接走 REST。

示例：

```go
result, err := client.AuthN().Login(ctx, authn.LoginRequest{
    Method:     "password",
    Realm:      "default",
    Identifier: "alice@example.com",
    Password:   password,
})
if err != nil {
    return err
}

accessToken := result.AccessToken
```

注意：

```text
SDK 不应记录 password；
SDK 不应保存用户密码；
Login 返回的 Token 由调用方按安全策略保存。
```

---

### 9.2 RefreshToken

示例：

```go
result, err := client.AuthN().RefreshToken(ctx, refreshToken)
if err != nil {
    return err
}
```

RefreshToken 比 AccessToken 更敏感。

SDK 不应默认把 RefreshToken 持久化到磁盘。

---

### 9.3 VerifyToken

VerifyToken 是服务端最常用能力。

示例：

```go
principal, err := client.AuthN().VerifyToken(ctx, accessToken)
if err != nil {
    return err
}
```

返回结果应能表达：

```text
UserID；
Subject；
TenantID；
LoginIdentityID；
AuthMethod；
AMR；
ExpiresAt；
TokenVersion；
```

具体字段以 SDK public API 为准。

---

### 9.4 JWKS / TokenVerifier

SDK 可以提供 JWKS manager 和本地 TokenVerifier。

适合：

```text
降低 VerifyToken 网络开销；
在可接受最终一致的场景做本地签名校验；
服务启动时预热 JWKS；
定期刷新公钥集合。
```

但要注意：

```text
本地验签只能确认签名、过期时间、issuer、audience 等基础 claims；
本地验签不一定能感知 Session revoke、账号禁用、风险控制、RefreshToken rotation；
需要强状态控制时应调用远程 VerifyToken。
```

推荐 SDK 明确区分：

```text
OfflineVerify：本地验签；
OnlineVerify：远程验证；
HybridVerify：优先本地，必要时远程。
```

实际 API 名称以当前 SDK 为准。

---

## 10. Identity SDK

Identity SDK 负责 User、Profile、ProfileLink 查询和管理。

典型能力：

```text
GetUser；
BatchGetUsers；
GetProfile；
BatchGetProfiles；
ListProfileLinks；
ListProfilesByUser；
ListUsersByProfile。
```

### 10.1 查询当前用户

示例：

```go
user, err := client.Identity().GetUser(ctx, principal.UserID)
if err != nil {
    return err
}
```

---

### 10.2 查询 Profile

示例：

```go
profile, err := client.Identity().GetProfile(ctx, profileID)
if err != nil {
    return err
}
```

---

### 10.3 查询 ProfileLink

示例：

```go
links, err := client.Identity().ListProfileLinks(ctx, identity.ListProfileLinksRequest{
    UserID: principal.UserID,
})
if err != nil {
    return err
}
```

注意：

```text
ProfileLink 是身份关系，不是最终权限判定。
```

如果业务操作需要访问控制，仍应调用 AuthZ Check。

---

## 11. AuthZ SDK

AuthZ SDK 是业务服务接入 IAM 授权能力的核心。

典型能力：

```text
Check；
Allow；
AllowScoped；
GetAuthorizationSnapshot；
GrantAssignment；
RevokeAssignment；
GrantPermission；
RevokePermission。
```

管理类能力是否暴露给 SDK，取决于当前 SDK public API。

### 11.1 Check

Check 返回完整 AuthorizationDecision。

示例：

```go
decision, err := client.AuthZ().Check(ctx, authz.CheckRequest{
    Subject:  principal.Subject,
    TenantID: tenantID,
    Resource: "qs:evaluation:report:*",
    Action:   "read",
    Scope:    "origin:" + profileID,
})
if err != nil {
    return err
}
if !decision.Allowed {
    return ErrPermissionDenied
}
```

Check 应返回：

```text
Allowed；
Reason；
DenyCode；
MatchedRole；
MatchedPermission；
PolicyVersion；
```

具体字段以 SDK public API 为准。

---

### 11.2 Allow

Allow 是 Check 的便利封装。

示例：

```go
allowed, err := client.AuthZ().Allow(ctx,
    principal.Subject,
    tenantID,
    "qs:evaluation:report:*",
    "read",
)
if err != nil {
    return err
}
if !allowed {
    return ErrPermissionDenied
}
```

Allow 适合简单场景。

如果需要 reason、deny_code、policy_version，应使用 Check。

---

### 11.3 AllowScoped

AllowScoped 是带 ObjectScope 的便利封装。

示例：

```go
allowed, err := client.AuthZ().AllowScoped(ctx,
    principal.Subject,
    tenantID,
    "qs:evaluation:report:*",
    "read",
    "origin:"+profileID,
)
if err != nil {
    return err
}
if !allowed {
    return ErrPermissionDenied
}
```

适合 qs-server 这类存在 profile / origin / owner 范围的业务系统。

---

### 11.4 GetAuthorizationSnapshot

Snapshot 用于授权视图展示。

示例：

```go
snapshot, err := client.AuthZ().GetAuthorizationSnapshot(ctx, authz.SnapshotRequest{
    Subject: principal.Subject,
    TenantID: tenantID,
    AppName:  "qs",
})
if err != nil {
    return err
}
```

注意：

```text
Snapshot 不替代 Check；
Snapshot 可以用于展示、调试、缓存视图；
最终访问控制仍应调用 Check / Allow。
```

---

## 12. IDP SDK

IDP SDK 负责外部身份源配置相关调用。

典型能力可能包括：

```text
GetIdpApp；
ListIdpApps；
CreateIdpApp；
UpdateIdpApp；
ResolveIdpConfig。
```

实际能力以 SDK public API 为准。

注意：

```text
IDP AppSecret 不应在日志中输出；
SDK response 不应暴露明文 secret，除非是一次性创建返回且有明确安全策略；
IDP 配置不等于 LoginIdentity；
LoginIdentity 属于 AuthN 账号模型。
```

---

## 13. Middleware / Interceptor 集成

### 13.1 HTTP Middleware

Go Web 服务可以通过 middleware 接入 IAM。

典型流程：

```text
1. 从 Authorization header 读取 Bearer Token；
2. 调用 SDK VerifyToken 或 TokenVerifier；
3. 得到 Principal；
4. 将 Principal 注入 request context；
5. 后续 handler 从 context 读取 Principal。
```

示例：

```go
func IAMAuthMiddleware(iam *sdk.Client) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := ExtractBearerToken(r.Header.Get("Authorization"))
            if token == "" {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }

            ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
            defer cancel()

            principal, err := iam.AuthN().VerifyToken(ctx, token)
            if err != nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }

            ctx = principalctx.WithPrincipal(r.Context(), principal)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

上面是示意代码。

实际 helper 名称以当前 SDK 为准。

---

### 13.2 gRPC Interceptor

gRPC 服务可以通过 interceptor 接入 IAM。

典型流程：

```text
1. 从 incoming metadata 读取 authorization；
2. VerifyToken；
3. 注入 Principal 到 context；
4. 调用下游 handler。
```

SDK 可以提供：

```text
UnaryServerInterceptor；
StreamServerInterceptor；
PrincipalFromContext；
RequireAuth；
RequirePermission；
```

实际 API 以当前 SDK 为准。

---

### 13.3 Handler 中做 AuthZ Check

Middleware 负责认证。

Handler 负责根据具体业务对象做授权。

示例：

```go
func (h *ReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
    principal := principalctx.MustPrincipal(r.Context())
    reportID := chi.URLParam(r, "reportID")

    report, err := h.reports.Get(r.Context(), reportID)
    if err != nil {
        // handle error
        return
    }

    allowed, err := h.iam.AuthZ().AllowScoped(r.Context(),
        principal.Subject,
        report.TenantID,
        "qs:evaluation:report:*",
        "read",
        "origin:"+report.ProfileID,
    )
    if err != nil {
        // handle iam error
        return
    }
    if !allowed {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }

    // return report
}
```

这体现了边界：

```text
middleware 认证；
handler 根据业务对象构造 Resource / Action / Scope；
IAM AuthZ 判定；
业务系统执行或拒绝操作。
```

---

## 14. qs-server 推荐集成方式

qs-server 推荐使用 SDK 封装 IAM 接入。

### 14.1 启动阶段

```text
1. 读取 IAM 配置；
2. 创建 SDK client；
3. 初始化 TokenVerifier / AuthN client；
4. 初始化 AuthZ client；
5. 注入到 HTTP server / gRPC server / application services。
```

---

### 14.2 请求阶段

```text
Client
  -> qs-server with Bearer Token
  -> IAM SDK VerifyToken
  -> Principal injected into context
  -> handler loads business object
  -> handler builds AuthZ request
  -> IAM SDK AuthZ Check / AllowScoped
  -> allow / deny
```

---

### 14.3 qs-server 保存什么

qs-server 可以保存：

```text
iam_user_id；
iam_profile_id；
tenant_id；
业务对象 owner / origin；
业务对象与 IAM 主体的引用关系。
```

qs-server 不应该保存：

```text
LoginIdentity；
Credential；
AccessToken / RefreshToken；
Role；
Permission；
RoleBinding；
Casbin rule。
```

---

### 14.4 qs-server 的权限建模

qs-server 负责把业务对象映射到 IAM AuthZ 模型。

示例：

```text
测评报告读取：
resource = qs:evaluation:report:*
action = read
scope = origin:<profile_id>

问卷模板管理：
resource = qs:survey:questionnaire:*
action = update
scope = all:*

答卷查看：
resource = qs:survey:answersheet:*
action = read
scope = origin:<profile_id>
```

具体 ResourceKey / Action / Scope 必须与 IAM ResourceCatalog 保持一致。

---

## 15. 错误处理模型

SDK 应把 REST / gRPC 错误统一映射为稳定 Go error。

推荐错误分类：

```text
InvalidArgument；
Unauthenticated；
PermissionDenied；
NotFound；
AlreadyExists；
Conflict；
FailedPrecondition；
RateLimited；
DeadlineExceeded；
Unavailable；
Internal。
```

调用方应该可以：

```go
if errors.Is(err, sdk.ErrUnauthenticated) {
    // return 401
}
if errors.Is(err, sdk.ErrPermissionDenied) {
    // return 403
}
```

如果 SDK 提供 typed error，可以包含：

```text
Code；
Message；
RequestID；
TraceID；
HTTPStatus；
gRPCStatus；
Details。
```

---

## 16. Retry / Timeout / Circuit Breaker

SDK 可以提供默认 retry，但必须谨慎。

### 16.1 可重试场景

通常可有限重试：

```text
network temporary error；
Unavailable；
DeadlineExceeded；
ResourceExhausted；
```

### 16.2 不应重试场景

不应盲目重试：

```text
InvalidArgument；
Unauthenticated；
PermissionDenied；
NotFound；
FailedPrecondition；
```

### 16.3 写操作重试

写操作重试必须依赖幂等键：

```text
idempotency_key；
request_id；
业务唯一键。
```

例如：

```text
GrantAssignment；
RevokeAssignment；
CreateUser；
UpdateIdpApp。
```

如果没有幂等保证，SDK 不应默认自动重试写操作。

---

## 17. 缓存策略

### 17.1 可以缓存什么

可以缓存：

```text
JWKS；
User display info；
Profile display info；
AuthorizationSnapshot for display；
```

### 17.2 谨慎缓存什么

谨慎缓存：

```text
VerifyToken result；
AuthZ Check result；
Permission facts；
RoleBinding facts。
```

如果缓存 Check 结果，必须考虑：

```text
PolicyVersion；
TTL；
Subject；
Tenant；
Resource；
Action；
Scope；
revocation；
业务风险等级。
```

默认建议：

```text
高风险操作不缓存 Check；
普通读操作可以短 TTL 缓存，但必须有清晰一致性边界；
Snapshot 可用于展示缓存，不作为最终授权判定。
```

---

## 18. 安全边界

SDK 必须保护敏感信息。

### 18.1 不记录敏感信息

禁止日志记录：

```text
password；
access token；
refresh token；
service token；
Authorization header；
private key；
IDP AppSecret；
SMS OTP。
```

### 18.2 不暴露 internal 包

业务系统不应该 import：

```text
internal/apiserver/...；
internal/pkg/... 中非公开包；
infra/mysql；
infra/casbin；
```

SDK public API 应足够覆盖接入需求。

### 18.3 不把服务身份伪装成用户身份

worker / service 应使用 service identity。

不要使用管理员用户密码登录后执行系统任务。

---

## 19. 测试策略

### 19.1 SDK fake client

SDK 应提供 fake client，方便业务系统单测。

示例：

```go
iam := sdktest.NewFakeClient(
    sdktest.WithVerifyTokenResult(principal),
    sdktest.WithAuthZDecision(authz.Decision{Allowed: true}),
)
```

业务服务单测不应依赖真实 IAM Server。

---

### 19.2 Contract tests

SDK 需要测试自己与 REST / gRPC 契约的一致性。

建议：

```text
public API compile test；
REST client contract test；
gRPC client contract test；
error mapping test；
timeout / retry test；
principal context helper test。
```

---

### 19.3 qs-server 集成测试

qs-server 可以使用：

```text
fake IAM SDK；
IAM test container；
local IAM server；
recorded fixture；
```

根据测试层级选择。

不要让所有业务单测都依赖真实 IAM。

---

## 20. SDK 契约维护规则

### 20.1 Public API 稳定性

以下属于 breaking change：

```text
删除 public type；
删除 public method；
修改 public method signature；
修改 error code 语义；
修改 config 字段语义；
修改默认 timeout / retry 行为且无迁移说明；
改变 AuthZ Allow / Check 语义。
```

---

### 20.2 不暴露 proto 泄漏

SDK 可以内部使用 generated proto client。

但 public API 不一定要直接暴露 proto message。

建议 public API 使用更稳定的 SDK model。

如果直接暴露 proto message，要接受 proto 字段变化对 SDK API 的影响。

---

### 20.3 版本策略

SDK 版本应与 IAM server 契约保持兼容说明。

建议记录：

```text
SDK version；
compatible IAM server version；
required REST API version；
required gRPC proto version；
deprecated API；
migration guide。
```

---

## 21. 验证建议

修改 SDK 后，建议运行：

```bash
go test ./pkg/sdk/...
```

如果影响 gRPC client：

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc/...
go test ./pkg/sdk/...
```

如果影响 REST client：

```bash
make docs-swagger
make api-validate
go test ./internal/apiserver/transport/rest/...
go test ./pkg/sdk/...
```

如果影响业务接入示例：

```bash
go test ./examples/...
```

具体命令以项目 Makefile 和 CI 为准。

---

## 22. 后续文档入口

本文说明 Go SDK 接入模型。

继续阅读：

```text
04-业务系统接入链路-以qs-server为例.md
05-契约事实源与防漂移机制.md
```

也可以回看：

```text
00-接入总览-业务系统如何接入IAM.md
01-REST API契约-前端与管理端接入.md
02-gRPC API契约-服务间调用与内部集成.md
```

其中：

```text
第 04 篇说明 qs-server 如何组合 SDK VerifyToken / Identity Query / AuthZ Check；
第 05 篇说明 OpenAPI / proto / SDK public API / docs 如何防漂移。
```

---

## 23. 本文总结

Go SDK 是 Go 服务端接入 IAM 的工程化封装。

它承载：

```text
AuthN：Login、RefreshToken、VerifyToken、TokenVerifier、JWKS；
Identity：User、Profile、ProfileLink 查询；
AuthZ：Check、Allow、AllowScoped、Snapshot；
IDP：外部身份源配置封装；
Middleware：HTTP / gRPC Principal 注入；
Testing：fake client、stub、test helper。
```

SDK 的核心边界是：

```text
SDK 封装 REST / gRPC；
SDK 不复制 IAM 数据库；
SDK 不直接操作 Casbin；
SDK 不本地维护 Role / Permission / RoleBinding；
SDK 不替代 IAM Server；
SDK 不隐藏认证、授权、版本一致性边界。
```

如果只记住一句话：

> IAM Go SDK 是 qs-server 这类 Go 服务接入 IAM 的客户端产品层：它帮助业务服务完成 Token 验证、Principal 注入、Identity 查询和 AuthZ Check，但身份事实、认证状态、授权事实和最终权限判定仍然由 IAM Server 负责。
