# SDK 迁移说明

## 本文回答

这篇文档只回答 4 件事：

1. 这轮 SDK 重构到底破坏了什么
2. 现在哪些包还算公开稳定入口
3. 旧代码应该迁到哪里
4. 哪些能力已经不再承诺为公开 API

## 30 秒结论

- 公开稳定入口现在固定为：`pkg/sdk`、`pkg/sdk/config`、`pkg/sdk/auth/client`、`pkg/sdk/auth/loginv2`、`pkg/sdk/auth/jwks`、`pkg/sdk/auth/verifier`、`pkg/sdk/auth/serviceauth`、`pkg/sdk/authz`、`pkg/sdk/identity`、`pkg/sdk/idp`、`pkg/sdk/errors`
- `pkg/sdk/transport` 和 `pkg/sdk/observability` 已经移入 `pkg/sdk/internal/...`，不再对外公开
- `pkg/sdk/errors` 只保留小型 facade；高级 `Analyze / matcher / handler` 能力已收回内部
- `pkg/sdk/auth` 兼容 façade 已删除，认证入口统一切到 `client`、`jwks`、`verifier`、`serviceauth`
- REST AuthN v2 登录入口为 `pkg/sdk/auth/loginv2`；gRPC token/JWKS/onboarding 客户端统一走 `pkg/sdk/auth/client` 的 v2 契约
- 2026-05 的契约整理仍在 v2 下进行：`AuthService.Login`、`IDPService.GetWechatAccessToken/RefreshWechatAccessToken`、ProfileLink `include_revoked` 已进入 v2 proto 和 SDK
- `sdk.NewTokenVerifier(...)`、`sdk.NewJWKSManager(...)`、`sdk.NewJWKSManagerWithClient(...)`、`sdk.NewServiceAuthHelper(...)` 已删除
- `sdk.NewClient(...)` 不再隐式启用 request-id / metrics / circuit breaker；这些能力现在由 `Config.Observability` 显式控制
- JWT `tenant_id` claim 现表示 **IAM 授权域**（string，如 `fangcun`）；业务组织请读 `org_id` claim 或 `TokenClaims.BusinessOrgID()`

## JWT tenant_id 语义变更

| 字段 / 方法 | 含义 |
| ----------- | ---- |
| `TokenClaims.TenantDomain` / `AuthorizationDomain()` | IAM 授权域（Casbin domain） |
| `TokenClaims.OrgID` / `BusinessOrgID()` | 业务组织 ID（JWT `org_id` 透传） |
| `TokenClaims.TenantID` | **Deprecated**：与 `TenantDomain` 相同，**不要**再 `ParseUint` 当 org |

迁移建议：

- 旧代码：`strconv.ParseUint(claims.TenantID, 10, 64)` → 改为 `claims.BusinessOrgID()`
- 授权域判断：使用 `claims.AuthorizationDomain()` 或 `claims.TenantDomain`
- 历史 token（`tenant_id` 为数值且无 `org_id`）：SDK 将 tenant 归一化为 `fangcun`，**不会**从 tenant 推导 org

gRPC `VerifyToken` 响应的 `TokenClaims` 已增加 `org_id` 字段（field 22）。

## 新的公开边界

### 保留为公开入口

- `pkg/sdk`
- `pkg/sdk/config`
- `pkg/sdk/auth/client`
- `pkg/sdk/auth/loginv2`
- `pkg/sdk/auth/jwks`
- `pkg/sdk/auth/verifier`
- `pkg/sdk/auth/serviceauth`
- `pkg/sdk/authz`
- `pkg/sdk/identity`
- `pkg/sdk/idp`
- `pkg/sdk/errors`

### 已移入内部实现

- `pkg/sdk/transport`
- `pkg/sdk/observability`
- `pkg/sdk/errors` 的高级分析 / matcher / handler 能力

## 替代关系

| 旧入口 | 新入口 |
| ---- | ---- |
| `pkg/sdk/transport` | `pkg/sdk` + `pkg/sdk/config` |
| `pkg/sdk/observability` | `Config.Observability` + `sdk.WithMetricsCollector(...)` / `sdk.WithTracingHook(...)` |
| `pkg/sdk/auth` | `pkg/sdk/auth/client`、`pkg/sdk/auth/loginv2`、`pkg/sdk/auth/jwks`、`pkg/sdk/auth/verifier`、`pkg/sdk/auth/serviceauth` |
| `sdk.NewTokenVerifier(...)` | `authverifier.NewTokenVerifier(...)` |
| `sdk.NewJWKSManager(...)` | `authjwks.NewJWKSManager(...)` |
| `sdk.NewJWKSManagerWithClient(...)` | `authjwks.NewJWKSManager(..., authjwks.WithAuthClient(client.Auth()))` |
| `sdk.NewServiceAuthHelper(...)` | `authserviceauth.NewServiceAuthHelper(..., client.Auth())` |
| `errors.Analyze(err)` | 不再公开；调用方只使用 `AsIAMError`、`GRPCCode`、`Message`、`ToHTTPStatus` |
| `errors.AuthErrors.Match(err)` | 用 `errors.IsUnauthorized(err)`、`errors.IsPermissionDenied(err)` 等谓词代替 |
| `errors.NewErrorHandler(...)` | 不再公开；调用方直接写自己的分支处理 |

## 常见迁移示例

### 0. v2 契约整理：登录、IDP token、ProfileLink include_revoked

本轮不发布 v3；REST/gRPC/SDK 直接在 v2 下整理契约。

- gRPC 登录新增 `iam.authn.v2.AuthService.Login`，请求继续使用 `auth_method + method_payload`。
- Go SDK `pkg/sdk/auth/client.Client` 新增 `Login(ctx, *authnv2.LoginRequest)`。
- IDP v2 gRPC 新增 `GetWechatAccessToken` 和 `RefreshWechatAccessToken`，Go SDK `pkg/sdk/idp.Client` 同步新增同名方法。
- ProfileLink v2 gRPC `ListProfilesRequest` 和 `ListProfileLinksRequest` 新增 `include_revoked`；Go SDK 透传 proto 字段，并提供 `GetUserProfilesIncludingRevoked` 便捷方法。
- REST ProfileLink 列表新增 `include_revoked` query 参数，旧 `active` 参数暂时兼容；新代码优先使用 `include_revoked=true`。
- gRPC 错误映射已收口到服务端共享表：400/401/403/404/409/423/429/500/502/503/504 分别映射到稳定 gRPC code；SDK 继续以 `pkg/sdk/errors.GRPCCode`、`ToHTTPStatus` 和 `Is*` 谓词作为唯一公开错误判断入口。
- Identity 批量读取和 ProfileLink 关系用户查询已改为批量查询路径；返回顺序、缺失用户容忍、`include_revoked` 语义保持不变。

旧 REST AuthN v2 登录 JSON 形状不变：

```json
{
  "auth_method": "password",
  "method_payload": {
    "username": "alice",
    "password": "secret"
  }
}
```

### 1. 移除对 `pkg/sdk/transport` 的直接 import

旧写法：

```go
import "github.com/FangcunMount/iam/v2/pkg/sdk/transport"

_ = transport.RequestIDInterceptor
```

新写法：

```go
import sdk "github.com/FangcunMount/iam/v2/pkg/sdk"

ctx = sdk.WithRequestID(ctx, "req-123")
```

如果你原来直接操作 retry / timeout / metadata 拦截器，说明你已经越过了 SDK 公开边界。这类低层 plumbing 现在视为内部实现；对外稳定面只保留 `Config` 和 `ClientOption`。

### 2. 移除对 `pkg/sdk/observability` 的直接 import

旧写法：

```go
import "github.com/FangcunMount/iam/v2/pkg/sdk/observability"

client, err := sdk.NewClient(ctx, cfg,
    sdk.WithUnaryInterceptors(
        observability.MetricsUnaryInterceptor(metrics),
    ),
)
```

新写法：

```go
type myMetrics struct{}

func (m *myMetrics) RecordRequest(method, code string, duration time.Duration) {}

client, err := sdk.NewClient(ctx, cfg, sdk.WithMetricsCollector(&myMetrics{}))
```

Tracing 同理，改为实现 `sdk.TracingHook` / `config.TracingHook` 并通过 `sdk.WithTracingHook(...)` 注入。

如果你还希望启用 SDK 自带的 request-id / metrics / tracing / circuit breaker 默认链路，需要显式设置：

```go
cfg.Observability = sdk.DefaultObservabilityConfig()
client, err := sdk.NewClient(ctx, cfg, sdk.WithMetricsCollector(&myMetrics{}))
```

### 3. 从旧认证入口迁到职责子包

旧写法：

```go
verifier, err := sdk.NewTokenVerifier(verifyCfg, jwksCfg, client)
```

新写法：

```go
import authjwks "github.com/FangcunMount/iam/v2/pkg/sdk/auth/jwks"
import authverifier "github.com/FangcunMount/iam/v2/pkg/sdk/auth/verifier"

jwksManager, err := authjwks.NewJWKSManager(jwksCfg,
    authjwks.WithCacheEnabled(true),
    authjwks.WithAuthClient(client.Auth()),
)
verifier, err := authverifier.NewTokenVerifier(verifyCfg, jwksManager, client.Auth())
```

对应关系：

- `client.Auth()` → `*authclient.Client`
- `sdk.NewJWKSManager` / `sdk.NewJWKSManagerWithClient` → `auth/jwks.NewJWKSManager`
- `sdk.NewTokenVerifier` → `auth/verifier.NewTokenVerifier`
- `sdk.NewServiceAuthHelper` → `auth/serviceauth.NewServiceAuthHelper`

### 4. 从高级错误分析迁回稳定 facade

旧写法：

```go
details := errors.Analyze(err)
if errors.AuthErrors.Match(err) {
    // ...
}
```

新写法：

```go
switch {
case errors.IsUnauthorized(err):
    // ...
case errors.IsPermissionDenied(err):
    // ...
case errors.IsRetryable(err):
    // ...
default:
    log.Printf("grpc=%s http=%d", errors.GRPCCode(err), errors.ToHTTPStatus(err))
}
```

服务端现在统一按 gRPC code 暴露错误语义。SDK 不承诺还原服务端内部 `component-base` 错误码；调用方应按 `GRPCCode`、`ToHTTPStatus` 或 `IsNotFound/IsRateLimited/IsRetryable` 等谓词分支。

## 这轮不再承诺为公开稳定 API 的能力

- 方法级 retry DSL
- 默认 Prometheus / OTel / circuit breaker 具体实现类型
- 高级错误分析器、matcher、handler 链
- 低层 gRPC transport 细节

这些能力仍然存在于 SDK 内部，但它们现在属于实现细节，不再作为对外兼容承诺的一部分。

## 建议的迁移顺序

1. 先删掉 `pkg/sdk/transport`、`pkg/sdk/observability` 的 import
2. 再把 `sdk.NewTokenVerifier`、`sdk.NewJWKSManager*`、`sdk.NewServiceAuthHelper` 改成直接调用认证子包
3. 最后把 `pkg/sdk/errors` 的高级 API 调用改成稳定谓词和映射函数

## 下一步

- [SDK 总览](../README.md)
- [配置详解](./02-configuration.md)
- [JWT 本地验证](./04-jwt-verification.md)
- [服务间认证](./05-service-auth.md)
