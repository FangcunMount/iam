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
│  │签名/声明    │  │在线状态    │  │宿主显式装配│    │
│  └────────────┘  └────────────┘  └────────────┘    │
└──────────────────────────────────────────────────────┘
                        ↓
┌──────────────────────────────────────────────────────┐
│ JWKSManager (Chain of Responsibility 职责链)          │
│                                                       │
│  Cache → 可选 CircuitBreaker → HTTP → gRPC → Seed  │
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
   └─ JWKS 基础设施失败? → Remote fallback
          ↓
3️⃣ JWKS Manager 获取密钥
   ┌─ Fresh Cache 命中? → 返回当前缓存集合
   ├─ HTTP 成功? → 替换集合并重设缓存时间
   ├─ gRPC 成功? → 替换集合并重设缓存时间
   └─ Seed 成功? → 同样重设缓存时间，不证明源端最新
          ↓
4️⃣ 验证 JWT
   ✓ 算法 allowlist
   ✓ 签名验证
   ✓ 过期时间检查
   ✓ Audience/Issuer 校验
   ✓ Required Claims
          ↓
5️⃣ 返回结果
   Valid: true
   UserID: "123"
   SessionID: "sid-123"
   AMR: ["pwd"]
```

### 验证合同对比

| 验证方式 | 可取得的事实 | 接入责任 |
| --- | --- | --- |
| 本地验签/公钥缓存 | 签名、配置约束与本地声明 | 明确密钥更新和在线状态窗口 |
| 远程验证 | IAM调用时密钥/登录态/准入 | 设置deadline；不形成在途请求撤销屏障 |
| Fallback | 本地先验，仅获取失败或空集合可远端求证 | 未知kid/签名/claims失败直接拒绝 |
| 显式验证结果缓存 | 复用宿主缓存的结果 | token-only key等限制另见下文，默认未装配 |

本仓库没有这些路径的延迟benchmark或生产性能证据，不给毫秒或可靠性星级承诺。

### 降级链路

```text
密钥获取: Fresh Cache → HTTP JWKS → gRPC JWKS → Seed
  ↓ 刷新失败
旧缓存: 仅在 FallbackOnError=true 且未超过 CacheTTL 时继续使用
  ↓ 无可用密钥
Token 验证: Remote fallback

签名、算法、过期时间、issuer/audience、required claims 失败
  → 直接拒绝，不做 Remote fallback
```

### 最小构造与验证

```go
// 1️⃣ 创建 JWKS 管理器和验证器
jwksManager, err := authjwks.NewJWKSManager(
    &sdk.JWKSConfig{
        URL: "https://iam.example.com/.well-known/jwks.json",
        RefreshInterval: 5 * time.Minute,
        RequestTimeout: 5 * time.Second,
    },
)
if err != nil { return err }
defer jwksManager.Stop()
verifier, err := authverifier.NewTokenVerifier(
    &sdk.TokenVerifyConfig{
        AllowedAudience: []string{"my-app"},
        AllowedIssuer: "https://iam.example.com",
        RequireExpirationTime: true,
    },
    jwksManager,
    nil,
)
if err != nil { return err }

// 2️⃣ 验证 Token
result, err := verifier.Verify(ctx, token, nil)
if err != nil { return err }

// 3️⃣ 使用结果
if result.Valid && result.Claims != nil {
	log.Printf("用户: %s, 认证手段: %v", result.Claims.UserID, result.Claims.AMR)
    log.Printf("会话: %s", result.Claims.SessionID)
}
```

---

## 📖 详细说明

### 为什么需要本地验证？

| 对比项 | 远程验证 | 本地验证 |
| ------- | --------- | --------- |
| 调用成本 | 每次RPC及在线存储读取 | 本地密码学计算与按需获取公钥 |
| 状态来源 | IAM当前读取 | 已取得公钥及JWT声明 |
| 网络开销 | ❌ 每次请求 | ✅ 定期刷新 |
| 适合场景 | 需要权威状态判断 | 高频 API |

### 本地验签的边界

SDK本地策略成功表示以下步骤通过，范围以所选KeySet和pinned JWX的实际校验为准：

- 使用已取得KeySet选出的密钥与算法验证签名；配置和受保护header只允许RS256，所选JWK.alg没有再次与header.alg对齐
- 固定可信`iss`、非空期望`aud`与支持的access用途约束通过；时间声明按pinned JWX及ClockSkew配置检查
- 可读取JWT自带claims，例如`user_id`、`org_id`（业务组织透传）、`sid`；读取不等于通过IAM身份不变量或当前业务资格检查

但它**不能保证**这些状态的即时生效：

- `revoked_access_token`
- `session(sid)` 已被 revoke
- 用户被封禁
- 登录入口被禁用；Credential锁定不属于在线Admission的逐次检查

需要调用时状态检查时，调用在线`Auth().VerifyToken(...)`；它不终止已经开始的读取。本地默认不要求exp存在，RequireExpirationTime/RequiredClaims只增加存在检查，不校验IAM的SID/身份非零/sub=UserID等领域不变量。pinned JWX还会跳过Unix值为0的exp/iat/nbf时间检查；已签名`exp:0`可满足存在检查而没有有效截止门禁，属于源码推论，未作专项实验。

SDK没有统一执行IAM公钥profile校验；异常受信KeySet可能让实际验签算法不同于header声明，这也是带输入前提的源码推论，不是生产行为证明。服务端与SDK的完整对照见[密码材料、密钥存储与令牌验签](../../../docs/03-基础设施/04-密码学密钥与令牌.md)；轮换、seed年龄、获取安全与失败边界由[JWKS主文](../../../docs/02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md)维护。

---

## 🏗️ 架构设计

```text
TokenVerifier (Strategy 模式)
├── LocalVerifyStrategy     ← 使用 JWKS 本地验证
├── RemoteVerifyStrategy    ← 调用 IAM 服务验证
├── FallbackVerifyStrategy  ← 先本地，仅 JWKS 基础设施失败后远程
└── CachingVerifyStrategy   ← 添加结果缓存

JWKSManager (Chain of Responsibility 模式)
├── CacheFetcher           ← 内存缓存
├── CircuitBreakerFetcher  ← 显式启用，包住获取链
├── HTTPFetcher            ← HTTP 获取
├── GRPCFetcher/Endpoint   ← 已有AuthClient优先，独立endpoint固定insecure
└── SeedFetcher            ← 本地种子备份
```

## 快速开始

程序示例及独立编译/环境状态见[示例索引](../_examples/README.md)：

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
    sdk "github.com/FangcunMount/iam/v5/pkg/sdk"
    authjwks "github.com/FangcunMount/iam/v5/pkg/sdk/auth/jwks"
    authverifier "github.com/FangcunMount/iam/v5/pkg/sdk/auth/verifier"
)
```

### 1. 最简配置

`AllowedIssuer` 必须来自可信部署配置，表示权威颁发者。调用级 `ExpectedIssuer` 是可选的额外约束，不能覆盖它；两者不一致时，本地和远程策略都会拒绝验证，且不会请求 JWKS 或远程验证服务。`ExpectedAudience` 可以按当前资源选择，但必须非空。

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
if err != nil { return err }

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
    
    // ClockSkew 时钟偏差容忍度（零值为0，不自动补1分钟）
    ClockSkew time.Duration
    
    // RequireExpirationTime 是否要求 exp 声明
    RequireExpirationTime bool
    
    // ForceRemoteVerification 强制使用远程验证
    ForceRemoteVerification bool
    
    // RequiredClaims 必须存在的声明列表
    // 例如: []string{"sub", "aud", "exp", "iat", "user_id"}
    RequiredClaims []string
    
    // Algorithms 允许的签名算法列表
    // 当前只接受RS256；其他值会拒绝配置
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
    Algorithms:              []string{"RS256"},
}
```

## JWKSConfig 配置

NewClient只保存该配置，不装配Manager/Verifier或填JWKS默认值。Env/Viper loader才默认RefreshInterval=5分钟/RequestTimeout=5秒；直接URL-only Manager没有后台刷新且HTTP timeout为0。ForceRefresh绕cache但仍经过熔断/seed，成功未必拿到源端最新集合；seed成功重新计缓存年龄，不能用CacheTTL约束seed公钥年龄。独立endpoint/Stop/畸形seed与并发刷新限制见 [JWKS主文](../../../docs/02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md)。

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
    
    // CacheTTL 旧密钥允许继续使用的最大时长
    CacheTTL time.Duration
    
    // HTTPClient 自定义 HTTP 客户端
    HTTPClient *http.Client
    
    // CustomHeaders 自定义请求头
    CustomHeaders map[string]string
    
    // FallbackOnError 刷新失败时，在 CacheTTL 内使用旧缓存
    FallbackOnError bool
}
```

### JWKS 配置示例

```go
&JWKSConfig{
    URL:             "https://iam.example.com/.well-known/jwks.json",
    // 生产mTLS接入通过WithAuthClient复用已配置Client，独立GRPCEndpoint为明文路径
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
    TokenID         string
    Subject         string
    SessionID       string
    UserID          string
    LoginIdentityID string
    OrgID           string // 业务组织（JWT org_id 透传）
    Issuer          string
    Audience        []string
    IssuedAt        time.Time
    ExpiresAt       time.Time
    NotBefore       time.Time
    TokenType       string
    AMR             []string
    AuthenticatedAt time.Time
    Attributes      map[string]string
    Extra           map[string]interface{}
}

// BusinessOrgID() (uint64, bool) 读取业务 org_id；无 claim 时 ok=false
```

### 使用示例

```go
result, err := verifier.Verify(ctx, token, nil)
if err != nil {
    log.Printf("验证错误: %v", err)
    return
}

if orgID, ok := result.Claims.BusinessOrgID(); ok {
    log.Printf("org_id=%d", orgID)
}

// 角色和权限不进入 AuthN JWT；请调用 AuthZ 能力完成授权判断。

// 检查过期
if time.Now().After(result.Claims.ExpiresAt) {
    log.Println("Token 已过期")
}
```

`VerifyOptions.AllowedTokenTypes` 默认仅包含 `TOKEN_TYPE_ACCESS`。本地和远端策略拒绝退役类型与未知类型，即使调用方显式配置允许也不能恢复支持。

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
if err != nil { return err }
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
2. **CircuitBreakerFetcher** - 显式启用；包住整条HTTP/gRPC/seed链
3. **HTTPFetcher** - HTTP 获取（主要方式）
4. **GRPCFetcher / GRPCEndpointFetcher** - 已提供AuthClient优先，否则独立endpoint（后者固定insecure）
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
    RefreshInterval: 5 * time.Minute,  // 刷新尝试间隔
    CacheTTL:        1 * time.Hour,    // 旧密钥最大可用时长
    FallbackOnError: true,             // 刷新失败时仅在 CacheTTL 内使用旧缓存
}
```

### 2. 使用 CachingVerifyStrategy

```go
// 当前没有按调用粒度配置 CacheTTL 的 VerifyOptions。
// 如果你确实需要缓存验证结果，需要自行包装 CachingVerifyStrategy。
caching := authverifier.NewCachingVerifyStrategy(delegate, cache, 5*time.Minute)
verifier := authverifier.NewTokenVerifierWithStrategy(caching)
```

该wrapper以token-only key命中，不重新检查exp/aud/issuer/options或裁TTL；显式strategy构造器没有remoteStrategy，ForceRemote不会旁路它。默认Verifier没有装配结果缓存，具体约束与例子见 [Token主文](../../../docs/02-业务模块/02-AuthN/05-关键链路-Token签发刷新吊销.md)。

### 3. 预热缓存

```go
// 启动时预热 JWKS
err := jwksManager.ForceRefresh(ctx)
if err != nil {
    log.Print("JWKS预热失败；后台刷新须RefreshInterval>0且cache存在")
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
    // 使用WithAuthClient(client.Auth())复用mTLS连接，独立endpoint不继承TLS
    RefreshInterval: 5 * time.Minute,            // 每 5 分钟刷新
    RequestTimeout:  10 * time.Second,           // HTTP 超时 10 秒
    CacheTTL:        1 * time.Hour,              // 缓存 1 小时
    FallbackOnError: true,                       // 失败时使用缓存
}
```

### 启动流程

此处cfg沿用宿主聚合配置约定（含TokenVerify），不是sdk.Config新增字段。成功返回的cleanup由宿主唯一拥有者在停止新Verify后调用一次；它只发Manager停止信号，不join在途刷新、不关闭借用SDK Client。constructor仍用Background首取；总预算与默认链隐藏endpoint连接的关闭缺口见[宿主接入正文](../../../docs/04-接口与SDK/02-Go-SDK与业务系统接入.md#7-http验签与后台资源要分别拥有)。

```go
func setupJWTVerifier(ctx context.Context, client *sdk.Client) (*authverifier.TokenVerifier, func(), error) {
    // 1. 创建 JWKS Manager
    jwksManager, err := authjwks.NewJWKSManager(
        cfg.JWKS,
        authjwks.WithCacheEnabled(true),
        authjwks.WithAuthClient(client.Auth()),
        authjwks.WithSeedData(seedJWKSJSON),
        authjwks.WithCircuitBreakerConfig(cfg.CircuitBreaker),
    )
    if err != nil {
        return nil, nil, err
    }

    // 2. 预热缓存
    if err := jwksManager.ForceRefresh(ctx); err != nil {
        log.Print("JWKS 预热失败；请核对后台刷新条件")
    }

    // 3. 创建 Verifier
    verifier, err := authverifier.NewTokenVerifier(
        cfg.TokenVerify,
        jwksManager,
        client.Auth(),
    )
    if err != nil {
        jwksManager.Stop()
        return nil, nil, err
    }

    return verifier, jwksManager.Stop, nil
}
```

## 常见问题

### Q: 如何处理 JWKS 密钥轮换？

A: Manager识别到cache且RefreshInterval>0才启动后台刷新；未知kid不自动刷新，刷新也可能只拿到旧seed。宿主须显式构造Manager/Verifier、核对新kid接受结果；完整轮换边界见 [JWKS主文](../../../docs/02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md)。

### Q: 本地验证失败会怎样？

A: 只有 JWKS 获取等基础设施故障才会在配置了 gRPC 客户端时降级到远程验证。签名、算法、过期时间、issuer/audience 或 required claims 验证失败会直接拒绝，远程验证也不会放宽这些约束。

### Q: 如何减少对 IAM 服务的依赖？

A: 使用本地种子缓存：

```go
seedJWKSJSON, _ := os.ReadFile("/var/cache/iam/jwks-seed.json")
authjwks.WithSeedData(seedJWKSJSON)
```

### Q: Token 验证性能如何？

A: 本仓库没有可支持固定延迟的benchmark，需在实际算法、硬件和流量下测量。结果缓存由宿主显式装配，并承担前述调用参数/过期限制。

## 下一步

- [Token 生命周期](./03-token-lifecycle.md)
- [服务间认证](./05-service-auth.md)
- [授权判定（PDP）](./06-authz.md)
- [示例索引](../_examples/README.md)

## 必填受众与一次切换

本地和远程验证必须具备预期 issuer 与非空 audience。多个预期受众采用任一匹配语义；空元素或显式空列表不是关闭校验的方式。直接调用 Auth().VerifyToken 时也必须提交 ExpectedAudience；缺失返回 InvalidArgument。资源服务从自己的配置取得期望值，不能使用未验证 Token 的 aud 作为期望值。

IAM 自身使用 iam-api，QS API 使用 qs-api，Collection API 使用 collection-api。新令牌包含这三个受众；旧令牌缺少 iam-api 时需刷新或重新登录。本地 JWKS 验证仍不具备在线撤销与准入的即时语义。
