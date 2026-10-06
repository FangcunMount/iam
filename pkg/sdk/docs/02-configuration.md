# 配置详解

> 状态：已实现 · 说明当前SDK配置和构造边界，不代表连接、握手或部署验收。

## 🎯 30 秒搞懂

### 金字塔模型

```text
                    Config (配置根)
                        │
        ┌───────────────┼───────────────┐
        ↓               ↓               ↓
    基础配置          连接配置         可靠性配置
    Endpoint         TLS              Retry
    Timeout          Keepalive        CircuitBreaker
                                      
        ↓               ↓               ↓
    高级功能         内置链路开关       特性开关
    JWKS             Observability    LoadBalancer
    Metadata         Hook Options
```

### 配置层次

```text
┌─────────────────────────────────────────────────┐
│ Level 1: 必填配置 (开始使用)                      │
│  • Endpoint: "iam.example.com:8081"             │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ Level 2: 安全配置 (生产环境)                      │
│  • TLS: {CACert, ClientCert, ClientKey}        │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ Level 3: 可靠性配置 (企业级)                      │
│  • Retry: {MaxAttempts, Backoff}               │
│  • CircuitBreaker: {FailureThreshold}          │
│  • Keepalive: {Time, Timeout}                  │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ Level 4: 高级特性 (按需启用)                      │
│  • JWKS: 本地 JWT 验证                          │
│  • Observability: Metrics/Tracing              │
│  • LoadBalancer: 负载均衡策略                    │
└─────────────────────────────────────────────────┘
```

### 配置场景速查

| 场景 | 配置重点 | 实际验证出口 |
| ------ | --------- | ------ |
| 本机明文替身 | Endpoint、显式TLS.Enabled=false | 明文与默认TLS分别核对，不用于远端服务信任 |
| 服务间接入 | CA、客户端证书/私钥、ServerName | 带deadline的所需RPC、证书身份与方法ACL |
| 写命令 | 显式重试政策、请求预算 | 提交后丢响应与结果协调，不能只测配置解析 |
| 本地验签 | 显式Manager/Verifier及issuer/audience | key来源、claims及在线状态/撤销差异 |
| 可观测 | 显式开关、宿主collector/hook | registry/exporter和跨服务context桥接 |

### 配置模板

```go
// 本机明文替身；nil TLS会补成默认启用
&Config{Endpoint: "localhost:8081", TLS: &TLSConfig{Enabled: false}}

// 🧪 测试环境 (快速)
&Config{
    Endpoint: "iam-test.example.com:8081",
    TLS: &TLSConfig{Enabled: true, InsecureSkipVerify: true},
}

// 服务间装配片段，真实RPC与业务接受另验
&Config{
    Endpoint: "iam.example.com:8081",
    TLS: &TLSConfig{
        Enabled: true, CACert: "/etc/certs/ca.crt",
        ClientCert: "/etc/certs/client.crt", ClientKey: "/etc/certs/client.key",
        ServerName: "iam.example.com",
    },
    Retry: &RetryConfig{Enabled: false},
}
```

### 配置优先级

```text
宿主选择 FromEnv 或 FromViper（分别加载）
    ↓
宿主显式覆盖/合并，Viper环境与文件规则由宿主设置
    ↓
NewClient 对传入配置 WithDefaults（原地补全）
    ↓
options / TLS / service config / interceptor 形成实际行为
```

### 示例约定

- 上面的“配置模板”保留完整 `&sdk.Config{...}` 形态，适合直接照抄起步
- 下面各小节默认只展示 **`Config` 内部字段片段**
- `TLS:`、`Retry:`等片段嵌入Config；JWKS字段只是保存配置，NewClient不自动装配Manager/Verifier，宿主须显式使用
- 程序示例需独立编译与环境核验，见 [../_examples/mtls/main.go](../_examples/mtls/main.go)

---

## 📋 配置结构

```go
type Config struct {
    // 基础配置
    Endpoint        string                // gRPC 服务地址 (必填)
    Timeout         time.Duration         // 默认值 30s；当前默认 RPC 链不据此设置 deadline
    DialTimeout     time.Duration         // 连接超时时间
    
    // TLS 配置
    TLS             *TLSConfig
    
    // 连接保活
    Keepalive       *KeepaliveConfig
    
    // 重试配置
    Retry           *RetryConfig
    
    // JWKS 配置（用于本地 JWT 验证）
    JWKS            *JWKSConfig
    
    // 负载均衡
    LoadBalancer    string                // "round_robin" 或 "pick_first"
    
    // 熔断器配置
    CircuitBreaker  *CircuitBreakerConfig
    
    // 可观测性默认链路开关
    Observability   *ObservabilityConfig
    
    // 默认元数据
    Metadata        map[string]string
}
```

## 基础配置

### Endpoint（必填）

gRPC 服务地址，格式：`host:port`

```go
Endpoint: "iam.example.com:8081"
```

### Timeout

`Timeout` 默认填为 30 秒，但当前默认 RPC 调用链没有使用该字段设置请求 deadline。默认 timeout interceptor 是原样转发，内部方法级 timeout 工具也未自动装配；因此设置此字段不能保证请求在 30 秒内结束。调用方应显式设置每个请求的 context deadline。

```go
Timeout: 30 * time.Second // 配置字段值；当前不自动应用到 RPC
```

```go
callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
allowed, err := client.Authz().Allow(callCtx, "user:42", "qs:evaluation:collection:assessments", "retry")
```

示例中的 2 秒是调用方预算。JWKS HTTP `RequestTimeout` 与 keepalive `Timeout` 分别由各自实现使用，不应与上述 Config 字段混淆。

### DialTimeout

`DialTimeout` 默认 10 秒，用于 `DialContext` 的 context，不作用于后续 RPC。默认没有 `WithBlock`，`NewClient` 返回成功不代表已完成连接、TLS 握手、ACL 校验或服务就绪；连接验证需要带 deadline 的实际 RPC。调用方通过公开 `WithDialOptions` 自行选择阻塞 Dial 时，此预算才会约束相应等待。

```go
DialTimeout: 10 * time.Second
```

## TLS 配置

```go
type TLSConfig struct {
    Enabled            bool     // 是否启用 TLS
    CACert             string   // CA 证书文件路径
    CACertPEM          []byte   // CA 证书 PEM 内容（优先级高于文件）
    ClientCert         string   // 客户端证书文件路径（mTLS）
    ClientCertPEM      []byte   // 客户端证书 PEM 内容
    ClientKey          string   // 客户端私钥文件路径（mTLS）
    ClientKeyPEM       []byte   // 客户端私钥 PEM 内容
    ServerName         string   // 服务端证书名称校验与 SNI
    InsecureSkipVerify bool     // 跳过服务端证书链和名称验证
    MinVersion         uint16   // 最低 TLS 版本（默认 TLS 1.2）
}
```

CA内存PEM优先于文件，解析失败不回退；显式CA使用新建的根池，未提供CA时才使用系统根。客户端cert/key须来自完整PEM对或完整文件对，不按字段混合拼接。完整文件对再加半套PEM仍可能被忽略或构造失败；来源选择矩阵由[传输层与服务间安全](../../../docs/03-基础设施/05-传输层与服务间安全.md)维护。

`ServerName`参与名称校验及SNI；为空时gRPC可按目标authority取得校验名称。`InsecureSkipVerify`没有SDK环境门禁，设为true会跳过服务端证书链和名称验证。证书仅在构造时加载，没有客户端自动重读任务。公开`WithDialOptions`随后追加，可以覆盖自动TLS credentials或默认service config；配置字段本身不是最终传输保证。

### 示例：单向 TLS

```go
TLS: &TLSConfig{
    Enabled:    true,
    CACert:     "/etc/iam/certs/ca.crt",
    ServerName: "iam.example.com",
    MinVersion: tls.VersionTLS12,
}
```

### 示例：双向 mTLS

```go
TLS: &TLSConfig{
    Enabled:    true,
    CACert:     "/etc/iam/certs/ca.crt",
    ClientCert: "/etc/iam/certs/client.crt",
    ClientKey:  "/etc/iam/certs/client.key",
    ServerName: "iam.example.com",
    MinVersion: tls.VersionTLS13,
}
```

### 示例：使用 PEM 内容（不使用文件）

```go
caCertPEM := []byte(`-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAL...
-----END CERTIFICATE-----`)

TLS: &TLSConfig{
    Enabled:   true,
    CACertPEM: caCertPEM,
}
```

## Keepalive 配置

```go
type KeepaliveConfig struct {
    Time                time.Duration // 发送 keepalive ping 的间隔
    Timeout             time.Duration // 等待 keepalive ping 响应的超时时间
    PermitWithoutStream bool          // 是否在没有活跃 stream 时发送 keepalive
}
```

### 默认配置

```go
Keepalive: &KeepaliveConfig{
    Time:                5 * time.Minute,
    Timeout:             20 * time.Second,
    PermitWithoutStream: false,
}
```

## 重试配置

### 全局重试配置

```go
type RetryConfig struct {
    Enabled           bool          // 是否启用重试
    MaxAttempts       int           // 最多尝试数，包含首次请求
    InitialBackoff    time.Duration // 初始退避时间
    MaxBackoff        time.Duration // 最大退避时间
    BackoffMultiplier float64       // 退避时间乘数
    RetryableCodes    []string      // 可重试的 gRPC 状态码
}
```

### 示例：标准重试配置

```go
Retry: &RetryConfig{
    Enabled:           true,
    MaxAttempts:       3,
    InitialBackoff:    100 * time.Millisecond,
    MaxBackoff:        10 * time.Second,
    BackoffMultiplier: 2.0,
    RetryableCodes:    []string{"UNAVAILABLE", "RESOURCE_EXHAUSTED", "ABORTED"},
}
```

### 方法级重试配置（高级）

当前 SDK 只把 **全局** `RetryConfig` 作为公开稳定面。更细粒度的方法级 retry / timeout DSL 已经收回内部实现，不再承诺为公开 API；如果你需要更细控制，优先拆分调用路径或通过自定义 interceptor 解决。

## JWKS 配置

用于宿主显式构造本地JWKSManager/Verifier；NewClient不自动装配或补全JWKS子配置。Env/Viper loader默认RefreshInterval=5分钟、RequestTimeout=5秒，直接URL-only Manager没有后台刷新且HTTP timeout为0。CacheTTL约束上次成功update年龄，不限制静态seed年龄；完整规则见 [JWKS主文](../../../docs/02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md)。

```go
type JWKSConfig struct {
    URL              string            // JWKS 端点 URL (HTTP/HTTPS)
    GRPCEndpoint     string            // gRPC 降级端点
    RefreshInterval  time.Duration     // 刷新间隔
    RequestTimeout   time.Duration     // HTTP 请求超时
    CacheTTL         time.Duration     // 缓存 TTL
    HTTPClient       *http.Client      // 自定义 HTTP 客户端
    CustomHeaders    map[string]string // 自定义请求头
    FallbackOnError  bool              // 失败时使用缓存
}
```

### 示例：标准 JWKS 配置

```go
JWKS: &JWKSConfig{
    URL:             "https://iam.example.com/.well-known/jwks.json",
    RefreshInterval: 5 * time.Minute,
    RequestTimeout:  10 * time.Second,
    CacheTTL:        1 * time.Hour,
    FallbackOnError: true,
}
```

### 示例：JWKS + gRPC 降级

```go
JWKS: &JWKSConfig{
    URL:             "https://iam.example.com/.well-known/jwks.json",
    // 标准mTLS接入用WithAuthClient(client.Auth())；独立GRPCEndpoint固定insecure
    RefreshInterval: 5 * time.Minute,
}
```

## 熔断器配置

标准RPC路径须显式EnableCircuitBreaker；非nil公开CircuitBreakerConfig映射内部结构时当前未补内部FailureCodes默认，公开配置本身没有该字段。内部不会逐字段补默认，普通RPC失败可能被RecordSuccess。只给阈值不提供所述故障保护；nil配置配合启用开关才使用内部默认失败码，细节与候选见[宿主接入正文](../../../docs/04-接口与SDK/02-Go-SDK与业务系统接入.md#3-配置如何变成实际连接和每次调用)。这一缺口不等同于JWKS fetcher的独立熔断实现。

JWKS熔断器须显式WithCircuitBreakerConfig启用，作用于整个HTTP/gRPC/seed链；当前HalfOpenRequests未用于JWKS半开并发限制。它与标准RPC拦截器装配是两条路径。

```go
type CircuitBreakerConfig struct {
    FailureThreshold int           // 触发熔断的连续失败次数
    OpenDuration     time.Duration // 熔断器打开持续时间
    HalfOpenRequests int           // 半开状态允许的请求数
    SuccessThreshold int           // 半开→关闭所需的连续成功次数
}
```

### 示例：参数形状，不证明失败分类已生效

```go
CircuitBreaker: &CircuitBreakerConfig{
    FailureThreshold: 5,
    OpenDuration:     30 * time.Second,
    HalfOpenRequests: 3,
    SuccessThreshold: 2,
}
```

## 可观测性配置

```go
type ObservabilityConfig struct {
    EnableMetrics        bool   // 启用指标收集
    EnableTracing        bool   // 启用链路追踪
    EnableCircuitBreaker bool   // 启用熔断器
    EnableRequestID      bool   // 启用请求 ID 注入
    MetricsNamespace     string // Prometheus 指标命名空间
    MetricsSubsystem     string // Prometheus 指标子系统
    ServiceName          string // 保留配置字段；当前默认 tracing 链未消费
}
```

### 示例：完整可观测性配置

```go
Observability: &ObservabilityConfig{
    EnableMetrics:        true,
    EnableTracing:        true,
    EnableCircuitBreaker: true,
    EnableRequestID:      true,
    MetricsNamespace:     "myapp",
    MetricsSubsystem:     "iam_client",
    ServiceName:          "my-service",
}
```

`ObservabilityConfig` 只控制 SDK 内置默认链路是否启用 request-id / metrics / tracing / circuit breaker。

当前默认语义是保守的：

- 如果 `Config.Observability == nil`，SDK 不会自动注册 request-id / metrics / tracing / circuit breaker 拦截器。
- 如果你希望启用一组标准 observability 能力，可以显式使用 `sdk.DefaultObservabilityConfig()`。

如果你要接自己的监控或追踪系统，不再直接 import 低层 observability 包，而是实现两个稳定接口并通过 option 注入：

```go
cfg.Observability = sdk.DefaultObservabilityConfig()
cfg.Observability.EnableTracing = true // 默认函数的 tracing 为 false

client, err := sdk.NewClient(ctx, cfg,
    sdk.WithMetricsCollector(myMetrics),
    sdk.WithTracingHook(myTracing),
)
```

默认 Prometheus collector 只创建/累加，不自动 Register；公开 Client 不提供该内部对象或 HTTP metrics 端点。这里的 `myMetrics` 应由宿主拥有注册和导出，`myTracing` 应由宿主拥有 exporter、上下文桥接及关闭；仅开启开关不证明采集接线。两个 option 也需要非 nil 的 Observability 与对应 Enabled 字段才进入默认 unary 链。ServiceName 当前没有默认消费点，RPC service/method 从 FullMethod 提取，不能将该字段当作已设置 OTel resource。具体信号和既有 stub 测试边界见[观测正文](../../../docs/03-基础设施/06-可观测性就绪与关闭.md#10-sdk观测属于宿主不继承服务端接线)。

## 负载均衡

支持两种负载均衡策略：

- `round_robin`：轮询（默认）
- `pick_first`：选择第一个可用连接

```go
LoadBalancer: "round_robin"
```

## 环境变量映射

| 环境变量 | 配置字段 | 默认值 |
| --------- | --------- | -------- |
| `IAM_ENDPOINT` | `Endpoint` | - |
| `IAM_TIMEOUT` | `Timeout` | `30s` |
| `IAM_DIAL_TIMEOUT` | `DialTimeout` | `10s` |
| `IAM_TLS_ENABLED` | `TLS.Enabled` | `true` |
| `IAM_TLS_CA_CERT` | `TLS.CACert` | - |
| `IAM_TLS_CLIENT_CERT` | `TLS.ClientCert` | - |
| `IAM_TLS_CLIENT_KEY` | `TLS.ClientKey` | - |
| `IAM_TLS_SERVER_NAME` | `TLS.ServerName` | - |
| `IAM_TLS_SKIP_VERIFY` | `TLS.InsecureSkipVerify` | `false` |
| `IAM_RETRY_ENABLED` | `Retry.Enabled` | `true` |
| `IAM_RETRY_MAX_ATTEMPTS` | `Retry.MaxAttempts` | `3` |
| `IAM_KEEPALIVE_ENABLED` | `Keepalive` section | disabled |
| `IAM_KEEPALIVE_TIME` | `Keepalive.Time` | `5m` |
| `IAM_KEEPALIVE_TIMEOUT` | `Keepalive.Timeout` | `20s` |
| `IAM_KEEPALIVE_PERMIT_WITHOUT_STREAM` | `Keepalive.PermitWithoutStream` | `false` |
| `IAM_JWKS_URL` | `JWKS.URL` | - |
| `IAM_JWKS_REFRESH_INTERVAL` | `JWKS.RefreshInterval` | `5m` |
| `IAM_CIRCUIT_BREAKER_ENABLED` | `CircuitBreaker` section | disabled |
| `IAM_CIRCUIT_BREAKER_FAILURE_THRESHOLD` | `CircuitBreaker.FailureThreshold` | `5` |
| `IAM_OBSERVABILITY_ENABLED` | `Observability` section | disabled |
| `IAM_OBSERVABILITY_ENABLE_METRICS` | `Observability.EnableMetrics` | `false` |
| `IAM_OBSERVABILITY_ENABLE_REQUEST_ID` | `Observability.EnableRequestID` | `false` |
| `IAM_LOAD_BALANCER` | `LoadBalancer` | `round_robin` |

上述映射须与`NewClient`的默认填充一起理解：Env解析的false、Viper getter返回的布尔false会使TLS或Retry变为nil，随后`WithDefaults()`重新启用该项。要在程序配置中保留关闭，须传入非nil的`TLSConfig{Enabled:false}`或`RetryConfig{Enabled:false}`。因此`IAM_TLS_ENABLED=false`、`IAM_RETRY_ENABLED=false`及对应Viper字段不能据loader输出被解释为最终已关闭。该链是源码事实，当前没有完整关闭路径专项测试。

### 使用环境变量

完整环境变量 / mTLS 场景见：

- [../_examples/mtls/main.go](../_examples/mtls/main.go)

```bash
export IAM_ENDPOINT="iam.example.com:8081"
export IAM_TLS_ENABLED="true"
export IAM_TLS_CA_CERT="/etc/iam/certs/ca.crt"
export IAM_TIMEOUT="30s"
export IAM_RETRY_MAX_ATTEMPTS="5"
export IAM_OBSERVABILITY_ENABLED="true"
export IAM_OBSERVABILITY_ENABLE_METRICS="true"
```

```go
cfg, err := sdk.ConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

client, err := sdk.NewClient(ctx, cfg)
```

## YAML 配置文件

```yaml
iam:
  endpoint: "iam.example.com:8081"
  timeout: 30s
  dial_timeout: 10s
  load_balancer: "round_robin"
  
  tls:
    enabled: true
    ca_cert: "/etc/iam/certs/ca.crt"
    client_cert: "/etc/iam/certs/client.crt"
    client_key: "/etc/iam/certs/client.key"
    server_name: "iam.example.com"
    min_version: "1.3"
  
  retry:
    enabled: true
    max_attempts: 3
    initial_backoff: 100ms
    max_backoff: 10s
    backoff_multiplier: 2.0
    retryable_codes: ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
  
  jwks:
    url: "https://iam.example.com/.well-known/jwks.json"
    refresh_interval: 5m
    request_timeout: 10s
  
  circuit_breaker:
    failure_threshold: 5
    open_duration: 30s
    half_open_requests: 3
  
  observability:
    enable_metrics: true
    enable_tracing: true
    enable_circuit_breaker: true
    enable_request_id: true
    metrics_namespace: "myapp"
    metrics_subsystem: "iam_client"
    service_name: "my-service"
```

### 使用 Viper 加载

```go
import (
    "github.com/spf13/viper"
    sdk "github.com/FangcunMount/iam/v5/pkg/sdk"
    "github.com/FangcunMount/iam/v5/pkg/sdk/config"
)

func main() {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    
    if err := viper.ReadInConfig(); err != nil {
        log.Fatal(err)
    }
    
    cfg, err := config.FromViper(viper.GetViper())
    if err != nil {
        log.Fatal(err)
    }
    
    client, err := sdk.NewClient(context.Background(), cfg)
    // ...
}
```

## 配置验证

`NewClient` 会补齐默认值并调用 `Config.Validate()`；当前该 validator 只检查 endpoint 非空。随后构造 Dial options 可能因 TLS 文件读取、PEM 解析或 key pair 加载失败返回错误。客户端构造与实际握手/服务准入是不同阶段：

```go
client, err := sdk.NewClient(ctx, cfg)
if err != nil {
    // 配置检查或连接配置构造失败
    log.Fatal(err)
}
```

当前检查边界：

- 配置验证：`Endpoint` 为空会直接报错；负 timeout、retry 次数或负载均衡名称没有在 `Config.Validate()` 中逐项检查。不能把配置字段存在当作校验已实现。
- TLS 构造：启用TLS时按上述来源规则读取CA及完整client cert/key；加载失败会报错。validator不统一拒绝半套材料，混合PEM/file也不承诺回退到可用的一对，具体条件见[传输主文](../../../docs/03-基础设施/05-传输层与服务间安全.md)。
- 实际连接与 RPC：服务器证书名称/信任链、客户端证书是否被接受、证书服务身份是否符合方法 ACL，需通过握手及真实 RPC 验证。默认非阻塞 Dial 的构造成功不证明这些检查通过。

补充说明：

- `ConfigFromEnv` 还加载Keepalive、CircuitBreaker、Observability；输出字段不等于默认链已消费它们。
- `config.FromViper(...)`只读取宿主getter，文件解析/环境绑定由宿主做；可以映射Keepalive、CircuitBreaker、Observability段。
- 自定义 metrics / tracing collector 始终通过 `sdk.WithMetricsCollector(...)`、`sdk.WithTracingHook(...)` 注入。

AuthN独立REST子客户端与JWKS HTTP获取不继承统一gRPC配置的TLS、Timeout或Retry。REST默认`http.DefaultClient`没有总超时，请求仍使用调用方context；JWKS提供自定义HTTPClient后，RequestTimeout也不会另加请求deadline。宿主须分别提供HTTP transport、超时和重定向策略。当前默认客户端未强制HTTPS/同源：301/302/303可改POST为GET，307/308可重发body；标准敏感头有host/subdomain剥离规则，名单之外的自定义秘密头不会被同样自动剥离，亦未统一禁止降级。具体输入条件为源码推论，未作专项实验，见[传输层与服务间安全](../../../docs/03-基础设施/05-传输层与服务间安全.md)。

## 下一步

- [Token 生命周期](./03-token-lifecycle.md)
- [JWT 验证](./04-jwt-verification.md)
- [服务间认证](./05-service-auth.md)
- [示例索引](../_examples/README.md)
