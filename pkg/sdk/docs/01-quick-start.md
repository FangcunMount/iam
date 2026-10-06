# 快速开始

## 🎯 30 秒搞懂

### 金字塔概览

```text
                        👤 你的应用
                            ↓
              ┌─────────────────────────┐
              │     sdk.Client          │  ← 统一入口
              │  (一个连接，全部服务)     │
              └─────────────────────────┘
                    ↓   ↓   ↓
        ┌───────┬────────┬────────────┬──────────┬──────────────┐
        ↓       ↓        ↓            ↓          ↓
    Auth()   Authz()  Identity()   Profile()  ProfileLink()
    认证服务  授权判定  身份服务       档案命令    档案关系服务
        ↓       ↓        ↓            ↓          ↓
    验证Token  单次PDP  用户读写/档案读  创建档案    档案关系查询/命令
```

### 核心概念

| 概念 | 说明 | 3 秒记忆 |
| ------ | ------ | --------- |
| **Client** | 统一客户端 | 一个连接，访问所有服务 |
| **Config** | 配置对象 | 地址 + TLS + 重试 |
| **Auth** | 认证服务 | 验证/刷新/撤销 Token |
| **Authz** | 授权判定 | 单次权限检查（PDP） |
| **Identity** | 身份服务 | 用户读写 / 档案读取 |
| **Profile** | 档案命令 | 创建档案并建立关系 |
| **ProfileLink** | 档案关系 | 用户-档案关系查询与命令 |

### 3 行代码开始

```go
// 1️⃣ 创建客户端
client, _ := sdk.NewClient(ctx, &sdk.Config{Endpoint: "localhost:8081"})

// 2️⃣ 使用服务
user, _ := client.Identity().GetUser(ctx, "user-123")

// 3️⃣ 完成！
log.Printf("用户: %s", user.GetProfile().GetDisplayName())
```

### 使用流程

```text
┌─────────────────────────────────────────────────────────┐
│ 1. 配置                                                  │
│    Config{Endpoint, TLS, Retry, ...}                    │
└────────────┬────────────────────────────────────────────┘
             ↓
┌─────────────────────────────────────────────────────────┐
│ 2. 创建客户端                                             │
│    client := sdk.NewClient(ctx, config)                 │
└────────────┬────────────────────────────────────────────┘
             ↓
┌─────────────────────────────────────────────────────────┐
│ 3. 调用服务                                              │
│    client.Auth().VerifyToken(...)                       │
│    client.Authz().Allow(...)                            │
│    client.Identity().GetUser(...)                       │
│    client.Profile().CreateProfile(...)                  │
│    client.ProfileLink().HasProfileLink(...)             │
└─────────────────────────────────────────────────────────┘
```

---

## 📦 安装

```bash
go get github.com/FangcunMount/iam/v5@v3.0.0
```

## 示例约定

- 这篇文档默认省略 `package`、`import` 和 `ctx := context.Background()`。
- `最简示例` 会完整展示 `client` 的创建方式；后续示例若只强调调用或配置，只展示变化的部分。
- 需要直接复制运行时，优先看 [../_examples/basic/main.go](../_examples/basic/main.go)。

## 最简示例

文档里只保留最短用法，完整可运行程序见：

- [../_examples/basic/main.go](../_examples/basic/main.go)

```go
client, err := sdk.NewClient(ctx, &sdk.Config{
    Endpoint: "localhost:8081",
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

result, err := client.Auth().VerifyToken(ctx, &authnv3.VerifyTokenRequest{
    AccessToken: "your-token-here",
})
```

## 从环境变量加载配置

```bash
# 设置环境变量
export IAM_ENDPOINT="iam.example.com:8081"
export IAM_TLS_ENABLED="true"
export IAM_TLS_CA_CERT="/etc/iam/certs/ca.crt"
export IAM_TIMEOUT="30s"
```

```go
cfg, err := sdk.ConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

client, err := sdk.NewClient(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

## 常见配置场景

以下 3 个示例只展示 `cfg` 的差异；创建客户端仍然是：

```go
client, err := sdk.NewClient(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### 1. 开发环境（无 TLS）

```go
cfg := &sdk.Config{
    Endpoint: "localhost:8081",
    TLS: &sdk.TLSConfig{
        Enabled: false,
    },
}
```

### 2. 测试环境（TLS，跳过验证）

```go
cfg := &sdk.Config{
    Endpoint: "iam-test.example.com:8081",
    TLS: &sdk.TLSConfig{
        Enabled:            true,
        InsecureSkipVerify: true, // 仅用于测试！
    },
}
```

### 3. 生产环境（mTLS）

```go
cfg := &sdk.Config{
    Endpoint: "iam.example.com:8081",
    TLS: &sdk.TLSConfig{
        Enabled:    true,
        CACert:     "/etc/iam/certs/ca.crt",
        ClientCert: "/etc/iam/certs/client.crt",
        ClientKey:  "/etc/iam/certs/client.key",
        ServerName: "iam.example.com",
        MinVersion: tls.VersionTLS13,
    },
    Timeout:     30 * time.Second,
    DialTimeout: 10 * time.Second,
}
```

`NewClient` 默认构造共享连接而不阻塞等待就绪；返回成功不证明 TLS 握手或服务端 ACL 已通过。用带 deadline 的实际 RPC 验证接入。上面的 `Timeout: 30*time.Second` 只是当前配置默认值，默认 RPC 链不会据此设置请求 deadline；`DialTimeout` 也不会传递到后续调用。每次请求须自行设置 `context.WithTimeout`，详情见 [Timeout 与 DialTimeout](./02-configuration.md#timeout)。

## 基础操作示例

### 认证服务

```go
// 验证 Token
resp, err := client.Auth().VerifyToken(ctx, &authnv3.VerifyTokenRequest{
    AccessToken: token,
})

// 刷新 Token
resp, err := client.Auth().RefreshToken(ctx, &authnv3.RefreshTokenRequest{
    RefreshToken: refreshToken,
})

// 撤销 Token
_, err := client.Auth().RevokeToken(ctx, &authnv3.RevokeTokenRequest{
    AccessToken: token,
})
```

### 身份服务

```go
// 获取用户
user, err := client.Identity().GetUser(ctx, "user-id-123")

// 创建用户
user, err := client.Identity().CreateUser(ctx, &identityv2.CreateUserRequest{
    User: &identityv2.User{
        Profile: &identityv2.UserProfile{
            DisplayName: "张三",
            Email:       "zhangsan@example.com",
        },
    },
})

// 批量获取用户
users, err := client.Identity().BatchGetUsers(ctx, []string{"user-1", "user-2"})
```

### 档案命令服务

`Profile()` 只封装 gRPC `ProfileCommand`。创建档案并建立初始 `User -> Profile` 关系时走这里：

```go
resp, err := client.Profile().CreateProfile(ctx, &identityv2.CreateProfileRequest{
    UserId:       "1001",
    LegalName:   "小明",
    Gender:      identityv2.Gender_GENDER_MALE,
    Dob:         "2018-01-01",
    IdCardNumber: "",
    Relation:    identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT,
})
```

### 授权判定服务

```go
callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()

// 单次权限判定：Subject 来自可信身份，ID 为 IAM 数字 ID。
resp, err := client.Authz().Check(callCtx, &authzv4.CheckRequest{
    Subject:  "user:42",
    Resource: "qs:evaluation:collection:assessments",
    Action:   "retry",
})
if err != nil { return err }
if !resp.Allowed { return fmt.Errorf("permission denied") }

// 便捷判定只提取 Allowed，不返回命中授权与策略版本。
allowed, err := client.Authz().Allow(
    callCtx,
    "user:42",
    "qs:evaluation:collection:assessments",
    "retry",
)
if err != nil { return err }
if !allowed { return fmt.Errorf("permission denied") }
```

上述只检查动作许可，不证明目标记录处于授权数据范围。需要公司/门店范围时使用 [AuthZ SDK](./06-authz.md) 的 Scope helper，并按 [gRPC 授权与 SDK](../../../docs/02-业务模块/03-AuthZ/06-关键链路-gRPC服务间授权与SDK.md) 执行权限匹配和业务范围规则。

### 档案关系服务

```go
// 检查档案关系
linkResp, err := client.ProfileLink().HasProfileLink(ctx, "user-id", "profile-id")

// 列举关联档案
profiles, err := client.ProfileLink().GetUserProfiles(ctx, "user-id")
```

### 只使用 identity 子包

如果调用方已经持有 gRPC 连接，可以直接创建拆分式子客户端：

```go
identityClient := identity.NewClientFromConn(conn)
profileClient := identity.NewProfileClientFromConn(conn)
profileLinkClient := identity.NewProfileLinkClientFromConn(conn)
```

需要显式注入 generated gRPC client 或测试替身时，继续使用原始构造函数：

```go
profileClient := identity.NewProfileClient(
    identityv2.NewProfileCommandClient(conn),
)
```

## 错误处理

下面的示例默认你已经导入了 `pkg/sdk/errors`，并且已经拿到了 `client`。

```go
user, err := client.Identity().GetUser(ctx, "user-123")
if err != nil {
    switch {
    case errors.IsNotFound(err):
        log.Println("用户不存在")
    case errors.IsUnauthorized(err):
        log.Println("未认证，请重新登录")
    case errors.IsPermissionDenied(err):
        log.Println("权限不足")
    case errors.IsServiceUnavailable(err):
        log.Println("服务暂时不可用，请稍后重试")
    default:
        log.Printf("未知错误: %v", err)
    }
    return
}

log.Printf("用户: %s", user.GetProfile().GetDisplayName())
```

## 下一步

- [../_examples/README.md](../_examples/README.md) - 完整可运行示例索引
- [配置详解](./02-configuration.md) - 了解所有配置选项
- [Token 生命周期](./03-token-lifecycle.md) - 搞清 SDK 里的校验、刷新、撤销和 JWKS
- [JWT 验证](./04-jwt-verification.md) - 本地 JWT 验证
- [服务间认证](./05-service-auth.md) - 自动化服务间 Token 管理
- [授权判定](./06-authz.md) - 单次 PDP 与 `Authz()` 用法
