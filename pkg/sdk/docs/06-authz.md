# 授权判定（PDP）

## 🎯 30 秒搞懂

### 金字塔架构

```text
                         你的业务代码
                              │
                          client.Authz()
                     (SDK 授权判定统一入口)
                              │
         ┌────────────────────┼────────────────────┐
         ↓                    ↓                    ↓
      Check()              Allow()               Raw()
   原始 CheckRequest      便捷布尔判定        原始 gRPC Client
         │                    │                    │
         └────────────────────┴────────────────────┘
                              ↓
      iam.authz.v2.AuthorizationService.Check(subject, domain, object, action)
                              ↓
                       Casbin Enforce(sub, dom, obj, act)
                              ↓
                    CheckResponse{ allowed: true/false }
```

### 设计图：PDP 四元组

```text
┌─────────────────────────────────────────────────────────────┐
│ subject  │ 谁在请求资源                                     │
│          │ user:<id> / group:<id> / service:<id>           │
├─────────────────────────────────────────────────────────────┤
│ domain   │ 在哪个租户域 / Casbin dom                        │
│          │ default / tenant-a                               │
├─────────────────────────────────────────────────────────────┤
│ object   │ 访问什么资源                                     │
│          │ resource:profile_profile / resource:report        │
├─────────────────────────────────────────────────────────────┤
│ action   │ 对资源做什么动作                                 │
│          │ read / write / delete / grant                   │
└─────────────────────────────────────────────────────────────┘
```

### 工程流程图

```text
1️⃣ 业务侧准备判定输入
   subject / domain / object / action
                ↓
2️⃣ SDK 发起 gRPC 调用
   client.Authz().Check(...) / Allow(...)
                ↓
3️⃣ 复用统一传输层
   mTLS / metadata / request-id / retry / errors.Wrap
                ↓
4️⃣ IAM 执行单次 PDP
   AuthorizationService.Check -> Casbin Enforce
                ↓
5️⃣ 返回结果
   CheckResponse{allowed} 或 bool
```

### 一句话结论

`client.Authz()` 是 IAM SDK 对 `iam.authz.v2.AuthorizationService/Check` 的轻封装，适合做**单次权限判定**；当前稳定能力是 `Check`、`Allow` 和 `Raw`。

### 当前能力边界

| 能力 | 当前状态 | 说明 |
| ---- | ---- | ---- |
| 单次权限判定 | ✅ 已支持 | `Check` / `Allow` |
| 原始 gRPC 访问 | ✅ 已支持 | `Raw()` |
| 批量判定 | ❌ 未封装 | 需业务侧自行扩展 |
| Explain / 调试原因 | ❌ 未封装 | 当前只返回 `allowed` |
| 策略管理 | ❌ 不在 SDK `Authz()` 范围 | 管理面属于 REST / 后台能力 |

### 3 行代码开始

```go
allowed, err := client.Authz().Allow(
    ctx,
    "user:user-123",
    "default",
    "resource:profile_profile",
    "read",
)
```

---

## 1. 什么时候优先看 `Authz()`

更适合使用 `client.Authz()` 的场景：

- 业务服务已经拿到了明确的 `subject / domain / object / action`
- 你只需要一个布尔判定结果，或一个最小的 `CheckResponse`
- 你希望复用 SDK 已有的连接、mTLS、metadata、重试和错误包装

不适合直接讲成 `Authz()` 已经覆盖的场景：

- 角色、资源、策略、Assignment 的管理
- 批量判定
- 带解释信息的判定
- 菜单树、按钮树、资源树裁剪

如果你需要的是授权管理面或接入边界，回看仓库主文档：

- [../../../docs/04-接口与SDK/README.md](../../../docs/04-接口与SDK/README.md)
- [../../../docs/02-业务模块/03-AuthZ/README.md](../../../docs/02-业务模块/03-AuthZ/README.md)

## 2. 快速开始

完整可运行示例见：

- [../_examples/authz/main.go](../_examples/authz/main.go)

### 示例约定

除非特别说明，下面的片段默认：

- 已存在 `ctx`
- 已创建 `client`
- 已按需导入 `sdk`、`authzv2`、`errors`
- 你已经在业务侧准备好了最终的 `subject / domain / object / action`

文档里保留的是**最小可理解片段**；如果你需要 `package main + import + 启动代码` 的完整版本，直接看上面的 `_examples/authz/main.go`。

### 2.1 基础调用

```go
client, err := sdk.NewClient(ctx, &sdk.Config{
    Endpoint: "localhost:8081",
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

resp, err := client.Authz().Check(ctx, &authzv2.CheckRequest{
    Subject: "user:user-123",
    Domain:  "default",
    Object:  "resource:profile_profile",
    Action:  "read",
})
```

### 2.2 便捷判定

如果你只关心最终是否允许，直接用 `Allow(...)` 更短：

```go
allowed, err := client.Authz().Allow(
    ctx,
    "user:user-123",
    "default",
    "resource:profile_profile",
    "read",
)
```

### 2.3 和服务间认证一起用

如果当前调用链需要服务间认证，先用 `ServiceAuthHelper` 包装上下文，再调用 `Authz()`：

```go
authCtx, err := helper.NewAuthenticatedContext(ctx)
if err != nil {
    return err
}

allowed, err := client.Authz().Allow(
    authCtx,
    "user:user-123",
    "tenant-a",
    "resource:report",
    "read",
)
```

## 3. 核心设计

### 3.1 `Check`、`Allow`、`Raw` 的分工

| 方法 | 适用场景 | 返回值 | 说明 |
| ---- | ---- | ---- | ---- |
| `Check` | 你需要直接对齐 proto | `*CheckResponse` | 最接近 gRPC 合同 |
| `Allow` | 你只关心允许 / 拒绝 | `bool` | 对 `Check` 的轻封装 |
| `Raw` | SDK 暂未封装更多调用风格 | `AuthorizationServiceClient` | 直接回退到原始 gRPC |

当前实现非常薄，核心路径就是：

```text
Allow(...)
  ↓
Check(&CheckRequest{subject, domain, object, action})
  ↓
authorizationService.Check(ctx, req)
  ↓
errors.Wrap(err)
```

这意味着 `Authz()` 的价值主要在于：

- 统一入口
- 统一错误包装
- 复用 SDK 既有传输层

而不是在 SDK 里再做一层复杂授权模型。

### 3.2 四元组怎么组织

#### `subject`

当前最常见的是：

```text
user:<user-id>
group:<group-id>
service:<service-id>
```

SDK 不替你推断 `subject`，调用方要自己传入最终字符串。  
这和服务端 gRPC 合同保持一致，见 [../../../api/grpc/iam/authz/v2/authz.proto](../../../api/grpc/iam/authz/v2/authz.proto)。

#### `domain`

`domain` 对应 Casbin 的租户域。

常见取值：

- `default`
- 某个明确租户 ID，例如 `tenant-a`

如果你的系统本身就是多租户，一定要把 `domain` 当成显式参数，不要在 SDK 调用层偷偷省略。

#### `object`

`object` 建议直接使用业务约定好的资源键。

例如：

```text
resource:profile_profile
resource:report
resource:survey
```

#### `action`

`action` 一般就是动作名字符串。

例如：

```text
read
write
delete
grant
```

## 4. 常见调用模式

### 4.1 进入业务逻辑前先做判定

```go
allowed, err := client.Authz().Allow(ctx, sub, dom, "resource:report", "read")
if err != nil {
    return err
}
if !allowed {
    return status.Error(codes.PermissionDenied, "forbidden")
}

// 再继续业务逻辑
```

### 4.2 保留原始响应

如果你不只需要布尔值，而是希望和未来的响应字段兼容，直接保留 `Check(...)`：

```go
resp, err := client.Authz().Check(ctx, &authzv2.CheckRequest{
    Subject: sub,
    Domain:  dom,
    Object:  obj,
    Action:  act,
})
if err != nil {
    return err
}

if !resp.Allowed {
    return status.Error(codes.PermissionDenied, "forbidden")
}
```

### 4.3 回退到原始 gRPC 客户端

如果 SDK 还没封装你要的调用风格，可以先退到 `Raw()`：

```go
raw := client.Authz().Raw()
resp, err := raw.Check(ctx, &authzv2.CheckRequest{
    Subject: sub,
    Domain:  dom,
    Object:  obj,
    Action:  act,
})
```

## 5. 错误处理

`Authz()` 和其它 SDK 子客户端一样，会用 `pkg/sdk/errors` 包装 gRPC 错误。

```go
allowed, err := client.Authz().Allow(ctx, sub, dom, obj, act)
if err != nil {
    switch {
    case errors.IsInvalidArgument(err):
        // 参数错误
    case errors.IsPermissionDenied(err):
        // 一般是上游业务自己返回的 PermissionDenied
    case errors.IsUnavailable(err):
        // IAM 服务不可用
    default:
        // 其它错误
    }
    return err
}
_ = allowed
```

## 6. 当前不要讲过头的几件事

- `Authz()` 当前只封装**单次 PDP**
- 它不是完整的授权管理 SDK
- 它不负责帮你构造 `subject / domain / object / action`
- 它不替你做批量判定、Explain、菜单裁剪

一句话说，`Authz()` 解决的是“已经拿到一条权限判断输入，稳定地发到 IAM 做判定”。

## 7. 继续往下读

- [快速开始](./01-quick-start.md)
- [Token 生命周期](./03-token-lifecycle.md)
- [服务间认证](./05-service-auth.md)
- [../README.md](../README.md)
- [../../../docs/04-接口与SDK/README.md](../../../docs/04-接口与SDK/README.md)
