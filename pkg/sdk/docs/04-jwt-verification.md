# JWT 本地验证

## 🎯 30 秒搞懂

### 金字塔架构

```text
                    JWT Token
                        │
                  Token Verifier
                 (验证器/策略选择)
                        │
        ┌───────────────┼───────────────┐
        ↓               ↓               ↓
    本地验证         远程验证         组合策略
    (最快)          (兜底)          (智能)
        ↓               ↓               ↓
    JWKS Manager    gRPC Call      缓存+降级
    (职责链)
        │
        ├─ Cache    (内存/最快)
        ├─ HTTP     (主要)
        ├─ gRPC     (降级)
        └─ Seed     (兜底)
```

### 双重设计模式

```text
┌──────────────────────────────────────────────────────┐
│ TokenVerifier (Strategy 策略模式)                     │
│                                                       │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│  │Local验证    │  │Remote验证   │  │Caching验证 │    │
│  │<1ms        │  │~50ms       │  │<1ms (缓存) │    │
│  └────────────┘  └────────────┘  └────────────┘    │
└──────────────────────────────────────────────────────┘
                        ↓
┌──────────────────────────────────────────────────────┐
│ JWKSManager (Chain of Responsibility 职责链)          │
│                                                       │
│  Cache → CircuitBreaker → HTTP → gRPC → Seed        │
│  ↓        ↓                ↓      ↓      ↓          │
│  快速     保护             主要   降级    兜底         │
└──────────────────────────────────────────────────────┘
```

### 工作流程

```text
1️⃣ 请求到达
   Token: "eyJhbGciOiJSUzI1..."
          ↓
2️⃣ Verifier 选择策略
   ┌─ 有 JWKS? → Local (本地验证)
   ├─ 无 JWKS? → Remote (远程验证)
   └─ 启用缓存? → Caching (缓存策略)
          ↓
3️⃣ JWKS Manager 获取密钥
   ┌─ Cache 命中? → 直接返回 (0.1ms)
   ├─ HTTP 成功? → 返回并缓存 (10ms)
   ├─ gRPC 成功? → 降级返回 (20ms)
   └─ Seed 存在? → 兜底返回 (0.5ms)
          ↓
4️⃣ 验证 JWT
   ✓ 签名验证
   ✓ 过期时间检查
   ✓ Audience/Issuer 校验
          ↓
5️⃣ 返回结果
   Valid: true
   UserID: "user-123"
   SessionID: "sid-123"
   Roles: ["admin", "user"]
```

### 性能对比

| 验证方式 | 延迟 | 可靠性 | 适用场景 |
| --------- | ------ | ------- | --------- |
| 🚀 **本地+缓存** | <1ms | ⭐⭐⭐⭐⭐ | 高频、可接受最终一致撤销语义的 API |
| ⚡ **本地验证** | 1-5ms | ⭐⭐⭐⭐ | 常规高频 API |
| 🌐 **远程验证** | 20-50ms | ⭐⭐⭐ | 需要权威判断 revoke / session / subject state 的操作 |
| 🔄 **Fallback** | 自适应 | ⭐⭐⭐⭐⭐ | 生产常用折中方案 |

### 降级链路

```text
正常: HTTP JWKS (10ms)
  ↓ 失败
降级1: gRPC JWKS (20ms)
  ↓ 失败
降级2: 内存 Cache (0.1ms)
  ↓ 失败
降级3: 本地 Seed (0.5ms)
  ↓ 失败
兜底: Remote 验证 (50ms)
```

### 3 行代码开始

```go
// 1️⃣ 创建 JWKS 管理器和验证器
jwksManager, _ := authjwks.NewJWKSManager(
    &sdk.JWKSConfig{URL: "https://iam.example.com/.well-known/jwks.json"},
)
verifier, _ := authverifier.NewTokenVerifier(
    &sdk.TokenVerifyConfig{AllowedAudience: []string{"my-app"}},
    jwksManager,
    nil,
)

// 2️⃣ 验证 Token
result, _ := verifier.Verify(ctx, token, nil)

// 3️⃣ 使用结果
if result.Valid {
    log.Printf("用户: %s, 角色: %v", result.Claims.UserID, result.Claims.Roles)
    log.Printf("会话: %s", result.Claims.SessionID)
}
```

---

## 📖 详细说明

### 为什么需要本地验证？

| 对比项 | 远程验证 | 本地验证 |
| ------- | --------- | --------- |
| 性能 | ❌ 50ms+ | ✅ <1ms |
| 可靠性 | ❌ 依赖 IAM 服务 | ✅ 本地独立 |
| 网络开销 | ❌ 每次请求 | ✅ 定期刷新 |
| 适合场景 | 需要权威状态判断 | 高频 API |

### 本地验签的边界

本地 JWKS 验签当前只能保证：

- 签名正确
- `exp` / `nbf` / `iss` / `aud` 等本地可判定声明正确
- JWT 内自带 claims 可直接读取，例如 `user_id`、`account_id`、`tenant_id`、`sid`

但它**不能保证**这些状态的即时生效：

- `revoked_access_token`
- `session(sid)` 已被 revoke
- 用户被封禁
- 账号被禁用或锁定

如果你的业务要求这些状态即时生效，就不要只做本地验签，而应调用在线 `Auth().VerifyToken(...)`。

---

## 🏗️ 架构设计

```text
TokenVerifier (Strategy 模式)
├── LocalVerifyStrategy     ← 使用 JWKS 本地验证
├── RemoteVerifyStrategy    ← 调用 IAM 服务验证
├── FallbackVerifyStrategy  ← 先本地，失败后远程
└── CachingVerifyStrategy   ← 添加结果缓存

JWKSManager (Chain of Responsibility 模式)
├── CacheFetcher           ← 内存缓存
├── CircuitBreakerFetcher  ← 熔断保护
├── HTTPFetcher            ← HTTP 获取
├── GRPCEndpointFetcher    ← gRPC 降级
└── SeedFetcher            ← 本地种子备份
```

## 快速开始

完整可运行示例见：

- [../_examples/verifier/main.go](../_examples/verifier/main.go)

### 示例约定

除非特别说明，下面的片段默认：

- 已存在 `ctx`
- 需要远程降级时已创建 `client`
- 已按需导入 `sdk`、`authjwks`、`authverifier`
- 应用配置型片段里的 `cfg` 指代你自己的聚合配置对象，至少包含 `JWKS`、`TokenVerify`、`CircuitBreaker` 等字段

文档里保留的是**最小可理解片段**；如果你需要 `package main + import + 启动代码` 的完整版本，直接看上面的 `_examples/verifier/main.go`。

下面涉及 SDK 认证子包时，统一约定这些 import 别名：

```go
import (
    sdk "github.com/FangcunMount/iam/pkg/sdk"
    authjwks "github.com/FangcunMount/iam/pkg/sdk/auth/jwks"
    authverifier "github.com/FangcunMount/iam/pkg/sdk/auth/verifier"
)
```

### 1. 最简配置

```go
jwksManager, err := authjwks.NewJWKSManager(
    &sdk.JWKSConfig{
        URL: "https://iam.example.com/.well-known/jwks.json",
    },
)
if err != nil {
    return err
}

verifier, err := authverifier.NewTokenVerifier(
    &sdk.TokenVerifyConfig{
        AllowedAudience: []string{"my-app"},
        AllowedIssuer:   "https://iam.example.com",
    },
    jwksManager,
    nil, // 不需要 gRPC 客户端
)
result, err := verifier.Verify(ctx, accessToken, nil)
```

### 2. 带远程降级

```go
client, err := sdk.NewClient(ctx, &sdk.Config{
    Endpoint: "iam.example.com:8081",
    TLS: &sdk.TLSConfig{
        Enabled: true,
        CACert:  "/etc/iam/certs/ca.crt",
    },
})

jwksManager, err := authjwks.NewJWKSManager(
    &sdk.JWKSConfig{
        URL:             "https://iam.example.com/.well-known/jwks.json",
        RefreshInterval: 5 * time.Minute,
    },
    authjwks.WithAuthClient(client.Auth()),
)
if err != nil {
    return err
}

verifier, err := authverifier.NewTokenVerifier(
    &sdk.TokenVerifyConfig{
        AllowedAudience:         []string{"my-app"},
        AllowedIssuer:           "https://iam.example.com",
        ForceRemoteVerification: false,
    },
    jwksManager,
    client.Auth(),
)
```

## TokenVerifyConfig 配置

```go
type TokenVerifyConfig struct {
    // AllowedAudience 允许的 audience 列表
    AllowedAudience []string
    
    // AllowedIssuer 允许的 issuer
    AllowedIssuer string
    
    // ClockSkew 时钟偏差容忍度（默认 1 分钟）
    ClockSkew time.Duration
    
    // RequireExpirationTime 是否要求 exp 声明
    RequireExpirationTime bool
    
    // ForceRemoteVerification 强制使用远程验证
    ForceRemoteVerification bool
    
    // RequiredClaims 必须存在的声明列表
    // 例如: []string{"sub", "aud", "exp", "iat", "user_id"}
    RequiredClaims []string
    
    // Algorithms 允许的签名算法列表
    // 支持: RS256, RS384, RS512, ES256, ES384, ES512, PS256, PS384, PS512, EdDSA
    // 如果为空，默认只允许 RS256
    Algorithms []string
}
```

### 配置示例

```go
&TokenVerifyConfig{
    AllowedAudience:         []string{"app-1", "app-2"},
    AllowedIssuer:           "https://iam.example.com",
    ClockSkew:               time.Minute,
    RequireExpirationTime:   true,
    ForceRemoteVerification: false,
    RequiredClaims:          []string{"sub", "user_id"},  // 必须包含这些声明
    Algorithms:              []string{"RS256", "ES256"},  // 只允许这些算法
}
```

## JWKSConfig 配置

```go
type JWKSConfig struct {
    // URL JWKS 端点 URL (HTTP/HTTPS)
    URL string
    
    // GRPCEndpoint gRPC 降级端点（HTTP 失败时使用）
    GRPCEndpoint string
    
    // RefreshInterval 刷新间隔
    RefreshInterval time.Duration
    
    // RequestTimeout HTTP 请求超时
    RequestTimeout time.Duration
    
    // CacheTTL 缓存 TTL
    CacheTTL time.Duration
    
    // HTTPClient 自定义 HTTP 客户端
    HTTPClient *http.Client
    
    // CustomHeaders 自定义请求头
    CustomHeaders map[string]string
    
    // FallbackOnError 失败时使用缓存
    FallbackOnError bool
}
```

### JWKS 配置示例

```go
&JWKSConfig{
    URL:             "https://iam.example.com/.well-known/jwks.json",
    GRPCEndpoint:    "iam.example.com:8081", // HTTP 失败时降级到 gRPC
    RefreshInterval: 5 * time.Minute,
    RequestTimeout:  10 * time.Second,
    CacheTTL:        1 * time.Hour,
    FallbackOnError: true,
    CustomHeaders: map[string]string{
        "X-API-Key": "your-api-key",
    },
}
```

## 验证结果

```go
type VerifyResult struct {
    Valid    bool        // Token 是否有效
    Claims   *TokenClaims
    RawToken jwt.Token   // 本地验签时可用；远程验证时通常为 nil
}

type TokenClaims struct {
    TokenID   string
    Subject   string
    SessionID string
    UserID    string
    AccountID string
    TenantID  string
    Issuer    string
    Audience  []string
    IssuedAt  time.Time
    ExpiresAt time.Time
    NotBefore time.Time
    Roles     []string
    Scopes    []string
    TokenType string
    AMR       []string
    Extra     map[string]interface{}
}
```

### 使用示例

```go
result, err := verifier.Verify(ctx, token, nil)
if err != nil {
    log.Printf("验证错误: %v", err)
    return
}

// 检查角色
if contains(result.Claims.Roles, "admin") {
    log.Println("管理员用户")
}

// 检查过期
if time.Now().After(result.Claims.ExpiresAt) {
    log.Println("Token 已过期")
}
```

## 高级用法

### 1. 自定义 JWKS Manager

```go
jwksCfg := &sdk.JWKSConfig{
    URL:             "https://iam.example.com/.well-known/jwks.json",
    RefreshInterval: 5 * time.Minute,
}

jwksManager, err := authjwks.NewJWKSManager(
    jwksCfg,
    authjwks.WithCacheEnabled(true),
    authjwks.WithAuthClient(client.Auth()), // 添加 gRPC 降级
    authjwks.WithCircuitBreakerConfig(&sdk.CircuitBreakerConfig{
        FailureThreshold: 3,
        OpenDuration:     30 * time.Second,
    }),
)
defer jwksManager.Stop()

// 手动刷新
err = jwksManager.ForceRefresh(ctx)
```

### 2. 验证选项

```go
// 自定义 audience 验证
result, err := verifier.Verify(ctx, token, &authverifier.VerifyOptions{
    ExpectedAudience: []string{"specific-app"},
})

// 远程策略下要求服务端返回 metadata
result, err := verifier.Verify(ctx, token, &authverifier.VerifyOptions{
    IncludeMetadata: true,
})
```

### 3. 策略选择

```go
selector := authverifier.NewStrategySelector(cfg.TokenVerify, jwksManager, client.Auth())

localStrategy, _ := selector.LocalStrategy()
remoteStrategy, _ := selector.RemoteStrategy()
fallbackStrategy, _ := selector.FallbackStrategy()

localVerifier := authverifier.NewTokenVerifierWithStrategy(localStrategy)
remoteVerifier := authverifier.NewTokenVerifierWithStrategy(remoteStrategy)
fallbackVerifier := authverifier.NewTokenVerifierWithStrategy(fallbackStrategy)

_, _, _ = localVerifier, remoteVerifier, fallbackVerifier
```

## JWKS 职责链

JWKS Manager 使用职责链模式，按顺序尝试：

1. **CacheFetcher** - 内存缓存（最快）
2. **CircuitBreakerFetcher** - 熔断保护
3. **HTTPFetcher** - HTTP 获取（主要方式）
4. **GRPCEndpointFetcher** - gRPC 降级（HTTP 失败时）
5. **SeedFetcher** - 本地种子备份（兜底）

### 配置职责链

```go
jwksManager, err := authjwks.NewJWKSManager(cfg,
    authjwks.WithCacheEnabled(true),          // 启用内存缓存
    authjwks.WithAuthClient(client.Auth()),   // gRPC 降级
    authjwks.WithSeedData(seedJWKSJSON),      // 本地种子数据
    authjwks.WithCircuitBreakerConfig(&sdk.CircuitBreakerConfig{
        FailureThreshold: 5,
        OpenDuration:     30 * time.Second,
    }),
)
```

## 性能优化

### 1. 启用缓存

```go
&JWKSConfig{
    RefreshInterval: 5 * time.Minute,  // 定期刷新
    CacheTTL:        1 * time.Hour,    // 缓存 1 小时
    FallbackOnError: true,             // 失败时使用缓存
}
```

### 2. 使用 CachingVerifyStrategy

```go
// 当前没有按调用粒度配置 CacheTTL 的 VerifyOptions。
// 如果你确实需要缓存验证结果，需要自行包装 CachingVerifyStrategy。
caching := authverifier.NewCachingVerifyStrategy(delegate, cache, 5*time.Minute)
verifier := authverifier.NewTokenVerifierWithStrategy(caching)
```

### 3. 预热缓存

```go
// 启动时预热 JWKS
err := jwksManager.ForceRefresh(ctx)
if err != nil {
    log.Printf("预热失败，将在后台刷新: %v", err)
}
```

## 错误处理

```go
result, err := verifier.Verify(ctx, token, nil)
if err != nil {
    switch {
    case errors.IsTokenExpired(err):
        log.Println("Token 已过期，请刷新")
        // 触发刷新流程
    case errors.IsTokenInvalid(err):
        log.Println("Token 格式无效")
        // 返回 401
    case errors.IsServiceUnavailable(err):
        log.Println("验证服务不可用，稍后重试")
        // 降级处理
    default:
        log.Printf("验证失败: %v", err)
    }
    return
}

_ = result
```

## 监控和观测

### 当前暴露面

当前 SDK 没有对外暴露统一的 `verifier.Stats()` 或 `result.Source`。

如果你要做观测，建议分两层：

- 对 `verifier.Verify(...)` 的成功/失败做调用级埋点
- 对 JWKS 获取链路单独做 HTTP / gRPC / 缓存命中监控

### Prometheus Metrics

```go
verifyCounter := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "jwt_verifications_total",
        Help: "Total JWT verifications",
    },
    []string{"result"},
)
prometheus.MustRegister(verifyCounter)

// 验证时记录
_, err := verifier.Verify(ctx, token, nil)
label := "ok"
if err != nil {
    label = "error"
}
verifyCounter.WithLabelValues(label).Inc()
```

## 生产环境建议

### 配置建议

```go
&TokenVerifyConfig{
    AllowedAudience:         []string{"your-app"},
    AllowedIssuer:           "https://iam.example.com",
    ClockSkew:               time.Minute,         // 容忍 1 分钟时钟偏差
    RequireExpirationTime:   true,               // 强制要求 exp
    ForceRemoteVerification: false,              // 本地优先
}

&JWKSConfig{
    URL:             "https://iam.example.com/.well-known/jwks.json",
    GRPCEndpoint:    "iam.example.com:8081",     // gRPC 降级
    RefreshInterval: 5 * time.Minute,            // 每 5 分钟刷新
    RequestTimeout:  10 * time.Second,           // HTTP 超时 10 秒
    CacheTTL:        1 * time.Hour,              // 缓存 1 小时
    FallbackOnError: true,                       // 失败时使用缓存
}
```

### 启动流程

```go
func setupJWTVerifier(ctx context.Context, client *sdk.Client) (*auth.TokenVerifier, error) {
    // 1. 创建 JWKS Manager
    jwksManager, err := authjwks.NewJWKSManager(
        cfg.JWKS,
        authjwks.WithCacheEnabled(true),
        authjwks.WithAuthClient(client.Auth()),
        authjwks.WithSeedData(seedJWKSJSON),
        authjwks.WithCircuitBreakerConfig(cfg.CircuitBreaker),
    )
    if err != nil {
        return nil, err
    }

    // 2. 预热缓存
    if err := jwksManager.ForceRefresh(ctx); err != nil {
        log.Printf("JWKS 预热失败，将在后台刷新: %v", err)
    }

    // 3. 创建 Verifier
    verifier, err := authverifier.NewTokenVerifier(
        cfg.TokenVerify,
        jwksManager,
        client.Auth(),
    )
    if err != nil {
        return nil, err
    }

    return verifier, nil
}
```

## 常见问题

### Q: 如何处理 JWKS 密钥轮换？

A: SDK 会自动刷新 JWKS。配置合理的 `RefreshInterval`（如 5 分钟）即可。

### Q: 本地验证失败会怎样？

A: 如果提供了 gRPC 客户端，会自动降级到远程验证。

### Q: 如何减少对 IAM 服务的依赖？

A: 使用本地种子缓存：

```go
seedJWKSJSON, _ := os.ReadFile("/var/cache/iam/jwks-seed.json")
authjwks.WithSeedData(seedJWKSJSON)
```

### Q: Token 验证性能如何？

A: 本地验证通常在 1ms 以内。若需要缓存验证结果，需要自行装配 `CachingVerifyStrategy`。

## 下一步

- [Token 生命周期](./03-token-lifecycle.md)
- [服务间认证](./05-service-auth.md)
- [授权判定（PDP）](./06-authz.md)
- [示例索引](../_examples/README.md)
