# IAM SDK for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/FangcunMount/iam/v2/pkg/sdk.svg)](https://pkg.go.dev/github.com/FangcunMount/iam/v2/pkg/sdk)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

`pkg/sdk` 是 IAM 的官方 Go 接入入口。当前公开稳定面固定为：

- `pkg/sdk`
- `pkg/sdk/config`
- `pkg/sdk/auth/client`
- `pkg/sdk/auth/jwks`
- `pkg/sdk/auth/verifier`
- `pkg/sdk/auth/serviceauth`
- `pkg/sdk/authz`
- `pkg/sdk/identity`
- `pkg/sdk/idp`
- `pkg/sdk/errors`

`transport`、`observability` 和高级错误分析能力已经收回内部实现，不再作为公开稳定包。

## 30 秒结论

- 如果你要接 IAM，优先从 `sdk.NewClient(...)` 开始。
- 如果你只需要认证能力，优先使用 `pkg/sdk/auth/client`、`pkg/sdk/auth/jwks`、`pkg/sdk/auth/verifier`、`pkg/sdk/auth/serviceauth`。
- 统一错误判断入口是 `pkg/sdk/errors`，对外只保留 `IAMError`、`Wrap`、常用 `Is*` 谓词、`AsIAMError`、`GRPCCode`、`Message`、`ToHTTPStatus`。
- 自定义 metrics / tracing 通过 `sdk.WithMetricsCollector(...)`、`sdk.WithTracingHook(...)` 注入；是否启用 SDK 内置 observability 链路由 `Config.Observability` 显式控制。

## 包结构

```text
pkg/sdk/
├── sdk.go                     # 包说明
├── aliases.go                 # Config / ClientOption 等别名与便捷函数
├── client.go                  # sdk.Client / sdk.NewClient
├── context_helpers.go         # request-id / trace-id helper
├── config/                    # 公开配置定义、加载器、option
├── errors/                    # 公开错误 facade
├── auth/                      # 认证领域子包
│   ├── client/
│   ├── jwks/
│   ├── verifier/
│   └── serviceauth/
├── authz/                     # 授权判定 client
├── identity/                  # 身份 / profile-link client
├── idp/                       # IDP client
├── internal/
│   ├── transport/             # gRPC 连接、重试、metadata、拦截器
│   ├── observability/         # 默认 metrics / tracing / circuit breaker
│   └── errorsx/               # 高级错误分析 / matcher / handler
└── _examples/                 # 完整可运行示例
```

## 快速开始

```go
import (
    "context"
    "log"

    authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
    sdk "github.com/FangcunMount/iam/v2/pkg/sdk"
)

func main() {
    ctx := context.Background()

    client, err := sdk.NewClient(ctx, &sdk.Config{
        Endpoint: "localhost:8081",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    resp, err := client.Auth().VerifyToken(ctx, &authnv2.VerifyTokenRequest{
        AccessToken: "jwt-token",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("valid=%v", resp.GetValid())
}
```

从环境变量加载：

```go
cfg, err := sdk.ConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

client, err := sdk.NewClient(ctx, cfg)
```

从 Viper 加载：

```go
import (
    "github.com/spf13/viper"
    "github.com/FangcunMount/iam/v2/pkg/sdk/config"
)

v := viper.New()
v.SetConfigFile("config.yaml")
if err := v.ReadInConfig(); err != nil {
    log.Fatal(err)
}

cfg, err := config.FromViper(v)
if err != nil {
    log.Fatal(err)
}
```

## 直接使用认证子包

如果你只需要认证能力，不必创建 `sdk.Client`：

```go
import (
    authclient "github.com/FangcunMount/iam/v2/pkg/sdk/auth/client"
    authjwks "github.com/FangcunMount/iam/v2/pkg/sdk/auth/jwks"
    authserviceauth "github.com/FangcunMount/iam/v2/pkg/sdk/auth/serviceauth"
    authverifier "github.com/FangcunMount/iam/v2/pkg/sdk/auth/verifier"
)

_ = authclient.NewClient
_ = authjwks.NewJWKSManager
_ = authverifier.NewTokenVerifier
_ = authserviceauth.NewServiceAuthHelper
```

### JWT 本地验证

```go
import (
    sdk "github.com/FangcunMount/iam/v2/pkg/sdk"
    authjwks "github.com/FangcunMount/iam/v2/pkg/sdk/auth/jwks"
    authverifier "github.com/FangcunMount/iam/v2/pkg/sdk/auth/verifier"
)

jwksManager, err := authjwks.NewJWKSManager(
    &sdk.JWKSConfig{
        URL:             "https://iam.example.com/.well-known/jwks.json",
        RefreshInterval: 5 * time.Minute,
    },
    authjwks.WithCacheEnabled(true),
    authjwks.WithAuthClient(client.Auth()),
)
if err != nil {
    log.Fatal(err)
}
defer jwksManager.Stop()

verifier, err := authverifier.NewTokenVerifier(
    &sdk.TokenVerifyConfig{
        AllowedAudience: []string{"my-app"},
        AllowedIssuer:   "https://iam.example.com",
    },
    jwksManager,
    client.Auth(),
)
if err != nil {
    log.Fatal(err)
}

result, err := verifier.Verify(ctx, token, nil)
if err != nil {
    log.Fatal(err)
}
log.Printf("user=%s session=%s", result.Claims.UserID, result.Claims.SessionID)
```

### 服务间认证

```go
import (
    sdk "github.com/FangcunMount/iam/v2/pkg/sdk"
    authserviceauth "github.com/FangcunMount/iam/v2/pkg/sdk/auth/serviceauth"
)

helper, err := authserviceauth.NewServiceAuthHelper(&sdk.ServiceAuthConfig{
    ServiceID:      "qs-service",
    TargetAudience: []string{"iam-service"},
    TokenTTL:       time.Hour,
    RefreshBefore:  5 * time.Minute,
}, client.Auth())
if err != nil {
    log.Fatal(err)
}
defer helper.Stop()

authCtx, err := helper.NewAuthenticatedContext(ctx)
if err != nil {
    log.Fatal(err)
}

_, err = client.Identity().GetUser(authCtx, "user-123")
```

## 错误处理

```go
import sdkerrors "github.com/FangcunMount/iam/v2/pkg/sdk/errors"

resp, err := client.Identity().GetUser(ctx, "user-123")
if err != nil {
    switch {
    case sdkerrors.IsNotFound(err):
        log.Println("用户不存在")
    case sdkerrors.IsUnauthorized(err):
        log.Println("未认证")
    case sdkerrors.IsPermissionDenied(err):
        log.Println("权限不足")
    case sdkerrors.IsRetryable(err):
        log.Println("可重试错误")
    default:
        log.Printf("grpc=%s http=%d msg=%s", sdkerrors.GRPCCode(err), sdkerrors.ToHTTPStatus(err), sdkerrors.Message(err))
    }
    return
}

_ = resp
```

如果需要拿到结构化错误：

```go
import sdkerrors "github.com/FangcunMount/iam/v2/pkg/sdk/errors"

if iamErr, ok := sdkerrors.AsIAMError(err); ok {
    log.Printf("code=%s grpc=%s msg=%s", iamErr.Code, iamErr.GRPCCode, iamErr.Message)
}
```

## Metrics 与 Tracing Hook

SDK 的 request-id / metrics / tracing / circuit breaker 链路已经内聚到 `pkg/sdk/internal/...`，但默认不会自动启用。只有显式设置 `Config.Observability` 时，SDK 才会挂载对应默认拦截器；对外仍只保留 hook 注入点。

```go
import (
    "context"
    "time"

    sdk "github.com/FangcunMount/iam/v2/pkg/sdk"
)

type myMetrics struct{}

func (m *myMetrics) RecordRequest(method, code string, duration time.Duration) {}

type myTracing struct{}

func (t *myTracing) StartSpan(ctx context.Context, name string) (context.Context, func()) {
    return ctx, func() {}
}

func (t *myTracing) SetAttributes(context.Context, map[string]string) {}
func (t *myTracing) RecordError(context.Context, error) {}

client, err := sdk.NewClient(ctx, &sdk.Config{
    Endpoint:      "iam.example.com:8081",
    Observability: sdk.DefaultObservabilityConfig(),
}, sdk.WithMetricsCollector(&myMetrics{}), sdk.WithTracingHook(&myTracing{}))
```

## 设计亮点

| 模块 | 设计重点 | 说明 |
| ---- | ---- | ---- |
| `pkg/sdk` | 统一接入入口 | `sdk.Client` 负责装配连接与子客户端 |
| `auth/loginv2` | REST v2 显式登录 | 只覆盖 `/api/v2/authn/login` 已实现能力 |
| `auth/jwks` | Chain of Responsibility | Cache → HTTP → gRPC → Seed |
| `auth/verifier` | Strategy | Local / Remote / Fallback / Cache |
| `auth/serviceauth` | 状态型 helper | 刷新、退避、熔断、旧 token 回退 |
| `errors` | 小型 facade | 保留稳定谓词与映射，移除高级 matcher API |
| `internal/transport` | 内聚 plumbing | gRPC 连接、metadata、默认拦截器链 |

## 迁移说明

这轮 SDK 重构包含 breaking change：

- `pkg/sdk/transport` 已删除
- `pkg/sdk/observability` 已删除
- `pkg/sdk/errors` 的高级分析 / matcher / handler API 已收回内部
- `pkg/sdk/auth/loginv2` 是 REST AuthN v2 显式登录入口；`pkg/sdk/auth/client` 已对齐 gRPC v2 token/JWKS/onboarding 契约

替代入口见 [07-migration-breaking-changes.md](./docs/07-migration-breaking-changes.md)。

## 文档

| 文档 | 说明 |
| ---- | ---- |
| [01-quick-start.md](./docs/01-quick-start.md) | 安装、基础示例、常见配置 |
| [02-configuration.md](./docs/02-configuration.md) | 配置结构、TLS、重试、JWKS、hook 注入 |
| [03-token-lifecycle.md](./docs/03-token-lifecycle.md) | token 校验、刷新、撤销、发牌边界 |
| [04-jwt-verification.md](./docs/04-jwt-verification.md) | JWKSManager / TokenVerifier |
| [05-service-auth.md](./docs/05-service-auth.md) | ServiceAuthHelper |
| [06-authz.md](./docs/06-authz.md) | `Authz().Check()` / `Allow()` |
| [07-migration-breaking-changes.md](./docs/07-migration-breaking-changes.md) | 本轮 breaking change 与替代入口 |

## 示例

| 示例 | 路径 | 说明 |
| ---- | ---- | ---- |
| 基础用法 | [_examples/basic/main.go](./_examples/basic/main.go) | `sdk.NewClient` + 基础调用 |
| mTLS | [_examples/mtls/main.go](./_examples/mtls/main.go) | TLS / Retry / Keepalive |
| JWT 验证 | [_examples/verifier/main.go](./_examples/verifier/main.go) | JWKS + verifier + 远程降级 |
| 服务间认证 | [_examples/service_auth/main.go](./_examples/service_auth/main.go) | `ServiceAuthHelper` |
| 授权判定 | [_examples/authz/main.go](./_examples/authz/main.go) | `Check` / `Allow` |
