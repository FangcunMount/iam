# 认证模块应用服务层

## 架构概览

应用服务层是六边形架构中的"应用层"，负责协调领域服务完成业务用例。本模块提供以下应用服务：

```text
application/
├── register/        # 注册服务
├── login/          # 登录服务  
├── token/          # Token 管理服务
├── account/        # 账号管理服务
└── uow/            # 工作单元（事务管理）
```

## 服务职责划分

### 1. AccountOnboarder - 注册服务

**职责**：处理用户注册流程

**核心方法**：

- `Register(ctx, OnboardingRequest)` - 统一注册接口

**注册流程**：

1. 创建或获取 User（通过手机号查找，避免重复）
2. 创建或获取 Account（通过 ExternalID+AppID 幂等检查）
3. 创建并绑定 Credential（根据 CredentialType）
4. 返回完整用户信息（包含 IsNewUser、IsNewAccount 标识）

**支持的注册方式**：

- `password` - 密码注册
- `phone` - 手机号注册
- `wechat` - 微信小程序
- `wecom` - 企业微信

**使用示例**：

```go
result, err := registerService.Register(ctx, OnboardingRequest{
    Name:  "张三",
    Phone: "+8613800138000",
    CredentialType: register.CredentialTypePassword,
    Password: ptr.String("SecurePass123!"),
})
```

---

### 2. LoginApplicationService - 登录服务

**职责**：处理用户登录和登出，完成认证并签发/撤销 Token

**核心方法**：

- `Login(ctx, LoginRequest)` - 统一登录接口
- `Logout(ctx, LogoutRequest)` - 统一登出接口

**登录流程**：

1. 根据 AuthType 选择认证策略
2. 执行认证（调用领域层 StrategyFactory）
3. 认证成功后签发 Token（AccessToken + RefreshToken）
4. 返回 Principal 和 TokenPair

**登出流程**：

1. 验证请求参数（至少提供 AccessToken 或 RefreshToken 之一）
2. 优先撤销 RefreshToken（更彻底，会使所有相关 AccessToken 失效）
3. 或撤销 AccessToken（仅撤销单个访问令牌）

**支持的认证方式**：

- `password` - 用户名密码
- `phone_otp` - 手机号验证码
- `wechat` - 微信小程序
- `wecom` - 企业微信
- `jwt_token` - JWT Token 认证（用于第三方集成）

**使用示例**：

```go
result, err := loginService.Login(ctx, LoginRequest{
    AuthType: login.AuthTypePassword,
    Username: ptr.String("user@example.com"),
    Password: ptr.String("password123"),
})
// result.TokenPair.AccessToken
// result.TokenPair.RefreshToken
```

---

### 3. TokenApplicationService - Token 管理服务

**职责**：Token 的验证、刷新、撤销

**核心方法**：

- `VerifyToken(ctx, accessToken)` - 验证 Access Token
- `RefreshToken(ctx, refreshToken)` - 刷新 Access Token
- `RevokeToken(ctx, accessToken)` - 撤销 Access Token
- `RevokeRefreshToken(ctx, refreshToken)` - 撤销 Refresh Token

**典型使用场景**：

#### 场景1：API 认证中间件

```go
// 在 HTTP 中间件中验证 Token
func AuthMiddleware(tokenService token.TokenApplicationService) gin.HandlerFunc {
    return func(c *gin.Context) {
        accessToken := c.GetHeader("Authorization")
        
        result, err := tokenService.VerifyToken(c, accessToken)
        if err != nil || !result.Valid {
            c.AbortWithStatus(401)
            return
        }
        
        // 将 Claims 存入 Context
        c.Set("user_id", result.Claims.UserID)
        c.Set("account_id", result.Claims.AccountID)
        c.Next()
    }
}
```

#### 场景2：Token 刷新

```go
// 客户端用 RefreshToken 获取新的 AccessToken
newTokens, err := tokenService.RefreshToken(ctx, oldRefreshToken)
```

#### 场景3：用户登出

```go
// 撤销 AccessToken
err := tokenService.RevokeToken(ctx, accessToken)

// 或撤销 RefreshToken（更彻底）
err := tokenService.RevokeRefreshToken(ctx, refreshToken)
```

---

### 4. AccountApplicationService - 账号管理服务

**职责**：账号和凭证的后续管理

**核心方法**：

- `GetAccountByID(ctx, accountID)` - 获取账号信息
- `UpdateAccountStatus(ctx, accountID, status)` - 更新账号状态
- `UpdateAccountProfile(ctx, accountID, profile)` - 更新账号资料
- `RotatePassword(ctx, req)` - 修改密码（需验证旧密码）
- `BindCredential(ctx, req)` - 绑定新凭证
- `UnbindCredential(ctx, req)` - 解绑凭证

**使用示例**：

```go
// 修改密码
err := accountService.RotatePassword(ctx, RotatePasswordRequest{
    AccountID:   accountID,
    OldPassword: "old123",
    NewPassword: "new456",
})

// 绑定微信小程序
err := accountService.BindCredential(ctx, BindCredentialRequest{
    AccountID:      accountID,
    CredentialType: account.CredentialTypeWechat,
    WechatAppID:    ptr.String("wx1234567890"),
    WechatOpenID:   ptr.String("oABC123XYZ"),
})
```

---

## 完整的认证流程示例

### 用户注册流程

```go
// 1. 用户注册
registerResult, err := registerService.Register(ctx, OnboardingRequest{
    Name:           "张三",
    Phone:          "+8613800138000",
    Email:          "zhangsan@example.com",
    CredentialType: register.CredentialTypePassword,
    Password:       ptr.String("SecurePass123!"),
})

// registerResult 包含:
// - User（用户基本信息）
// - Account（账号信息）
// - IsNewUser（是否新用户）
// - IsNewAccount（是否新账号）
```

### 用户登录流程

```go
// 2. 用户登录
loginResult, err := loginService.Login(ctx, LoginRequest{
    AuthType: login.AuthTypePassword,
    Username: ptr.String("zhangsan@example.com"),
    Password: ptr.String("SecurePass123!"),
})

// loginResult 包含:
// - Principal（认证主体）
// - TokenPair（AccessToken + RefreshToken）
// - UserID、AccountID、TenantID
```

### API 访问流程

```go
// 3. 访问受保护的 API
// 客户端在请求头中携带 AccessToken
// Authorization: Bearer <access_token>

// 中间件验证 Token
result, err := tokenService.VerifyToken(ctx, accessToken)
if !result.Valid {
    return errors.New("invalid token")
}

// 从 result.Claims 中获取用户信息
userID := result.Claims.UserID
accountID := result.Claims.AccountID
```

### Token 刷新流程

```go
// 4. AccessToken 过期后，使用 RefreshToken 刷新
newTokens, err := tokenService.RefreshToken(ctx, refreshToken)

// newTokens 包含:
// - AccessToken（新的访问令牌）
// - RefreshToken（新的刷新令牌）
```

### 用户登出流程

```go
// 5. 用户登出 - 使用 LoginApplicationService
err := loginService.Logout(ctx, LogoutRequest{
    RefreshToken: ptr.String(refreshToken), // 优先使用 RefreshToken（更彻底）
})
// 或
err := loginService.Logout(ctx, LogoutRequest{
    AccessToken: ptr.String(accessToken), // 只撤销 AccessToken
})
```

---

## 设计原则

### 1. 单一职责

- 每个应用服务只负责一类业务用例
- 避免服务之间职责重叠

### 2. 统一接口

- 同一类操作提供统一接口（如 Register、Login）
- 通过枚举类型（CredentialType、AuthType）区分不同场景
- 避免接口膨胀（不要为每种类型创建单独方法）

### 3. 事务边界

- 使用 UnitOfWork 管理事务边界
- 一个应用服务方法 = 一个事务
- 跨服务调用不在事务内

### 4. DTO 转换

- 应用层使用自己的 DTO（Request/Result）
- 不直接暴露领域对象给外层
- 在应用层完成类型转换

### 5. 错误处理

- 将领域错误转换为应用层错误码
- 提供清晰的错误信息

---

## 依赖注入示例

```go
// 创建应用服务（在容器中配置）
func NewApplicationServices(
    db *gorm.DB,
    strategyFactory *authService.StrategyFactory,
    tokenIssuer tokenPort.TokenIssuer,
    tokenVerifier tokenPort.TokenVerifier,
    tokenRefresher tokenPort.TokenRefresher,
) *ApplicationServices {
    
    // UnitOfWork
    uow := uow.NewUnitOfWork(db)
    
    // 应用服务
    return &ApplicationServices{
        Register: register.NewAccountOnboarder(uow),
        Login:    login.NewLoginApplicationService(strategyFactory, tokenIssuer),
        Token:    token.NewTokenApplicationService(tokenVerifier, tokenRefresher),
        Account:  account.NewAccountApplicationService(uow),
    }
}
```

---

## 与 HTTP 层的集成

HTTP Handler 只调用应用服务，不直接调用领域服务：

```go
// POST /api/v1/auth/login
func LoginHandler(loginService login.LoginApplicationService) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req LoginRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(400, gin.H{"error": err.Error()})
            return
        }
        
        // 调用应用服务
        result, err := loginService.Login(c, req)
        if err != nil {
            c.JSON(401, gin.H{"error": err.Error()})
            return
        }
        
        c.JSON(200, gin.H{
            "access_token":  result.TokenPair.AccessToken.Value,
            "refresh_token": result.TokenPair.RefreshToken.Value,
            "expires_in":    result.TokenPair.AccessToken.ExpiresAt.Unix(),
        })
    }
}
```

---

## 总结

**不需要 AuthApplicationService**，因为：

1. **Login** 已经处理了认证 + Token 签发
2. **Token** 已经处理了 Token 验证、刷新、撤销
3. 再加一个 Auth 会造成职责重叠和混淆

**正确的使用方式**：

- 注册 → `AccountOnboarder`
- 登录 → `LoginApplicationService.Login`
- 登出 → `LoginApplicationService.Logout`
- API 认证中间件 → `TokenApplicationService.VerifyToken`
- Token 刷新 → `TokenApplicationService.RefreshToken`
- 账号管理 → `AccountApplicationService`
