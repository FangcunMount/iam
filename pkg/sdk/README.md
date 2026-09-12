# IAM SDK for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/FangcunMount/iam/v5/pkg/sdk.svg)](https://pkg.go.dev/github.com/FangcunMount/iam/v5/pkg/sdk)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

`pkg/sdk` 是 IAM 的官方 Go 接入入口。当前公开稳定面固定为：

- `pkg/sdk`
- `pkg/sdk/config`
- `pkg/sdk/auth/client`
- `pkg/sdk/auth/challenge`
- `pkg/sdk/auth/jwks`
- `pkg/sdk/auth/loginidentity`
- `pkg/sdk/auth/loginv3`
- `pkg/sdk/auth/verifier`
- `pkg/sdk/auth/signup`
- `pkg/sdk/authz`
- `pkg/sdk/identity`
- `pkg/sdk/idp`
- `pkg/sdk/errors`

`transport`、`observability` 和高级错误分析能力已经收回内部实现，不再作为公开稳定包。

当前 Go module major 为 v5，import 根路径为 `github.com/FangcunMount/iam/v5`。选择已经发布的 v5 标签安装；本文中的 Scope 新接口必须使用包含该实现的 SDK 版本，不能直接用现有 v5.1.0 替代。

Go module major 与线协议版本分别管理。AuthZ 使用 `api/grpc/iam/authz/v4`；不要仅因升级 SDK import path 而改写服务协议路径。

## 30 秒结论

- 如果你要接 IAM，优先从 `sdk.NewClient(...)` 开始。
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
│   ├── challenge/
│   ├── client/
│   ├── jwks/
│   ├── loginidentity/
│   ├── loginv3/
│   ├── signup/
│   ├── verifier/
├── authz/                     # 授权判定 client
├── identity/                  # 身份 / profile / profile-link client
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

    authnv3 "github.com/FangcunMount/iam/v5/api/grpc/iam/authn/v3"
    sdk "github.com/FangcunMount/iam/v5/pkg/sdk"
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

    resp, err := client.Auth().VerifyToken(ctx, &authnv3.VerifyTokenRequest{
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
    "github.com/FangcunMount/iam/v5/pkg/sdk/config"
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
    authclient "github.com/FangcunMount/iam/v5/pkg/sdk/auth/client"
    authjwks "github.com/FangcunMount/iam/v5/pkg/sdk/auth/jwks"
    authloginidentity "github.com/FangcunMount/iam/v5/pkg/sdk/auth/loginidentity"
    authloginv3 "github.com/FangcunMount/iam/v5/pkg/sdk/auth/loginv3"
    authsignup "github.com/FangcunMount/iam/v5/pkg/sdk/auth/signup"
    authverifier "github.com/FangcunMount/iam/v5/pkg/sdk/auth/verifier"
)

_ = authclient.NewClient
_ = authjwks.NewJWKSManager
_ = authloginidentity.NewClient
_ = authloginv3.NewClient
_ = authsignup.NewClient
_ = authverifier.NewTokenVerifier
```

### JWT 本地验证

```go
import (
    sdk "github.com/FangcunMount/iam/v5/pkg/sdk"
    authjwks "github.com/FangcunMount/iam/v5/pkg/sdk/auth/jwks"
    authverifier "github.com/FangcunMount/iam/v5/pkg/sdk/auth/verifier"
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

配置 mTLS 客户端证书后直接调用 SDK，服务端通过证书身份、ACL 和业务 AuthZ 逐层校验。参见 [服务间认证](docs/05-service-auth.md)。

## Identity / Profile 拆分式客户端

统一入口 `sdk.Client` 会复用同一个 gRPC connection，并暴露三个拆分式 identity 子客户端：

```go
identityClient := client.Identity()       // User / IdentityRead / IdentityLifecycle
profileClient := client.Profile()         // ProfileCommand
profileLinkClient := client.ProfileLink() // ProfileLinkQuery / ProfileLinkCommand
```

创建档案走 `Profile()`，不是 `Identity()`：

```go
resp, err := profileClient.CreateProfile(ctx, &identityv2.CreateProfileRequest{
    UserId:       "1001",
    LegalName:   "小明",
    Gender:      identityv2.Gender_GENDER_MALE,
    Dob:         "2018-01-01",
    IdCardNumber: "",
    Relation:    identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT,
})
```

如果调用方已经持有 gRPC 连接，可以只使用 `pkg/sdk/identity` 子包：

```go
identityClient := identity.NewClientFromConn(conn)
profileClient := identity.NewProfileClientFromConn(conn)
profileLinkClient := identity.NewProfileLinkClientFromConn(conn)
```

需要显式注入 generated gRPC client 或测试替身时，保留原始工厂：

```go
profileClient := identity.NewProfileClient(
    identityv2.NewProfileCommandClient(conn),
)
```

## 错误处理

```go
import sdkerrors "github.com/FangcunMount/iam/v5/pkg/sdk/errors"

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
import sdkerrors "github.com/FangcunMount/iam/v5/pkg/sdk/errors"

if iamErr, ok := sdkerrors.AsIAMError(err); ok {
    log.Printf("code=%s grpc=%s msg=%s", iamErr.Code, iamErr.GRPCCode, iamErr.Message)
}
```

服务端 V2 gRPC 错误映射已经统一：常见业务错误会稳定落到 `InvalidArgument`、`Unauthenticated`、`PermissionDenied`、`NotFound`、`AlreadyExists`、`FailedPrecondition`、`ResourceExhausted`、`Unavailable`、`DeadlineExceeded` 或 `Internal`。SDK 调用方不要解析错误消息文本。

## Metrics 与 Tracing Hook

SDK 的 request-id / metrics / tracing / circuit breaker 链路已经内聚到 `pkg/sdk/internal/...`，但默认不会自动启用。只有显式设置 `Config.Observability` 时，SDK 才会挂载对应默认拦截器；对外仍只保留 hook 注入点。

```go
import (
    "context"
    "time"

    sdk "github.com/FangcunMount/iam/v5/pkg/sdk"
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
| `auth/loginv3` | REST v2 显式登录 | 覆盖 `/api/v3/authn/login`；gRPC 登录走 `auth/client` |
| `auth/jwks` | Chain of Responsibility | Cache → HTTP → gRPC → Seed |
| `auth/verifier` | Strategy | Local / Remote / Fallback / Cache |
| `identity` | 拆分式 Identity SDK | `Client` 负责 User / IdentityRead / IdentityLifecycle；`ProfileClient` 负责 ProfileCommand；`ProfileLinkClient` 负责 ProfileLink query/command |
| `errors` | 小型 facade | 保留稳定谓词与映射，移除高级 matcher API |
| `pkg/sdk/internal/transport` | 内聚 plumbing | gRPC 连接、metadata、默认拦截器链 |

## 迁移说明

这轮 SDK 重构包含 breaking change：

- 历史 v2 import `github.com/FangcunMount/iam/v2/pkg/sdk/transport` 已删除
- 历史 v2 import `github.com/FangcunMount/iam/v2/pkg/sdk/observability` 已删除
- `pkg/sdk/errors` 的高级分析 / matcher / handler API 已收回内部
- `pkg/sdk/auth/loginv3` 是 REST AuthN v2 显式登录入口；`pkg/sdk/auth/client` 已对齐 gRPC v2 Login/token/JWKS/onboarding 契约
- `pkg/sdk/idp` 已对齐 v2 WeChat app 查询与 access token 获取/刷新契约
- `pkg/sdk/identity` 保持拆分式客户端：`Client`、`ProfileClient`、`ProfileLinkClient`；ProfileLink 查询支持 `include_revoked`

替代入口见 [07-migration-breaking-changes.md](./docs/07-migration-breaking-changes.md)。

## 文档

| 文档 | 说明 |
| ---- | ---- |
| [01-quick-start.md](./docs/01-quick-start.md) | 安装、基础示例、常见配置 |
| [02-configuration.md](./docs/02-configuration.md) | 配置结构、TLS、重试、JWKS、hook 注入 |
| [03-token-lifecycle.md](./docs/03-token-lifecycle.md) | token 校验、刷新、撤销、发牌边界 |
| [04-jwt-verification.md](./docs/04-jwt-verification.md) | JWKSManager / TokenVerifier |
| [05-service-auth.md](./docs/05-service-auth.md) | mTLS + ACL |
| [06-authz.md](./docs/06-authz.md) | `Authz().Check()` / `Allow()` |
| [07-migration-breaking-changes.md](./docs/07-migration-breaking-changes.md) | 本轮 breaking change 与替代入口 |

## 示例

| 示例 | 路径 | 说明 |
| ---- | ---- | ---- |
| 基础用法 | [_examples/basic/main.go](./_examples/basic/main.go) | `sdk.NewClient` + 基础调用 |
| mTLS | [_examples/mtls/main.go](./_examples/mtls/main.go) | TLS / Retry / Keepalive |
| JWT 验证 | [_examples/verifier/main.go](./_examples/verifier/main.go) | JWKS + verifier + 远程降级 |
| 服务间认证 | [_examples/mtls/main.go](./_examples/mtls/main.go) | mTLS |
| 授权判定 | [_examples/authz/main.go](./_examples/authz/main.go) | `Check` / `Allow` |

## AuthZ 数据范围接入

Scope 随角色分配保存，包含公司 `OrgID` 和 `STORES`（指定门店）或 `ALL_STORES`（公司全部门店）。IAM 负责返回资源、动作与范围的对应关系；业务系统负责验证当前运营身份和公司，并在查询、统计及写入中执行范围限制。普通 `Check` / `Allow` 的动作许可不能证明目标记录位于授权范围内。

- 使用 `Authz().GetScopedAuthorizationSnapshot` 获取并校验版本为 1 的范围快照。旧服务、非法范围或非无条件权限会返回错误；不能捕获错误后回退成全公司权限。
- 按当前公司、目标资源和具体动作匹配权限，再合并这些匹配项的范围。不同动作或公司的范围不能混用。
- 缺少范围表示没有数据范围授权。`ALL_STORES` 也不使门店归属为空的受试者自动可见。数据列表须先按当前归属过滤，再分页和计数；总部内容等不属于门店数据的入口使用其明确的业务规则。
- 管理角色分配时，请求 `IncludeAssignmentFacts=true`；SDK 同时通过 `ValidateAssignmentScopes` 校验完整分配事实。`Scope=nil` 表示该分配尚未配置，不表示全公司。
- 使用 `ReplaceScopedAssignments` 提交当前公司的受管角色及范围，并携带读取时的 `ExpectedPolicyVersion`。版本冲突时重新读取并核对用户意图，不自动用新版本覆盖其他操作人的修改。请求结果不明时先查询事实。

发布顺序为：发布含新接口的 SDK，更新消费者依赖并验证，随后在协调的维护窗口完成授权数据迁移和服务切换。仅升级 SDK 不会迁移存量分配；旧消费者也不会因为协议新增字段就自动执行数据范围过滤。

完整迁移、指纹复核、策略版本及回滚要求见[Assignment Scope 迁移维护手册](../../docs/operations/assignment-scope-migration.md)。
