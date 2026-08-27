# Go SDK 与业务系统接入

> 状态：已实现 · `pkg/sdk` 是服务间 gRPC 的统一入口，同时提供少量 AuthN REST 子客户端、JWKS verifier 和 service-token 生命周期助手。

## 1. 先选调用形态

| 场景 | 推荐入口 | 安全语义 |
| --- | --- | --- |
| 业务服务调用 Identity/AuthZ/IDP | `sdk.NewClient` | gRPC + service identity |
| 用户登录/OTP/绑定 | `auth/loginv2`、`challenge`、`loginidentity` | REST，按 endpoint 公开/用户 token |
| 低延迟 JWT 验签 | `auth/jwks` + local verifier | 签名/claims，本地撤销窗口 |
| 强即时撤销验证 | `client.Auth().VerifyToken` / remote verifier | IAM 在线状态检查 |
| 服务自动获取 token | `auth/serviceauth` | 后台刷新 service token |

不要为了“统一”把终端用户密码交给服务间 SDK，也不要把本地 verify 结果描述成在线 Session 检查。

## 2. Unified Client

`sdk.NewClient(ctx, Config, options...)` 校验配置、建立一个共享 gRPC connection，并创建：

```text
Auth() / Authz() / Identity() / Profile() / ProfileLink() / IDP()
```

调用方必须 `Close()`。拆分 `Identity/Profile/ProfileLink` 是为了让命令边界清楚：创建 Profile 走 `Profile()`，关系查询/命令走 `ProfileLink()`，
不是把所有方法塞进一个万能 Identity client。

稳定 public surface 只包括 `pkg/sdk`、列出的子包和公开配置/错误 facade。`pkg/sdk/internal/{transport,observability,errorsx}` 可随内部实现变化，接入方不能
import。

## 3. 连接与重试

SDK Config 包含 endpoint、TLS、dial/request timeout、keepalive、retry、load balancer、circuit breaker、observability 和 metadata。
Dial 构建 interceptor 链，再在 DialTimeout context 中建立 gRPC connection。

重试只能用于明确幂等的方法或在服务端具有 idempotency key 的写操作。网络错误发生时，客户端无法判断请求是在到达前失败还是提交后响应丢失；对 Grant/Create 等写 RPC 盲重试可能重复副作用。

SDK 内部有 per-method retry defaults 和 error classification，但调用方仍应按具体 API 合同确认。不要把 `Unavailable` 一律无限重试；应有 bounded
attempts、backoff、deadline 和 circuit breaker。

## 4. ServiceAuthHelper 生命周期

`NewServiceAuthHelper` 构造时先同步获取 service token，失败则不返回半可用 helper；成功后启动 refresh loop，在 `RefreshBefore` 窗口提前续期。调用方用
`NewAuthenticatedContext` 注入 Authorization metadata，并在退出时 `Stop()`。

它解决 token 刷新，不解决服务授权：签发 token 的 subject/audience 仍需服务端 ACL/AuthZ，mTLS identity match 也应按生产配置启用。

helper 使用后台 goroutine；若应用忘记 Stop，会发生资源泄漏。应把它与业务进程 lifecycle 一起管理，而不是每个请求创建一个 helper。

## 5. Local、Remote 与 Fallback verifier

Local strategy：

- 从 JWKS manager 获取 key set；
- 校验算法、签名、issuer、audience、时间和 required claims；
- 不查询 revocation marker、Session 或 User/LoginIdentity 状态。

Remote strategy 调 IAM Verify，具备在线状态语义但增加网络依赖。Fallback strategy 在 primary 失败后尝试 fallback；其安全方向取决于顺序：

- remote -> local：IAM 故障时可继续接受签名有效 token，但牺牲即时撤销；
- local -> remote：本地 key/cache 问题时向 IAM 求证，不扩大信任；
- 对“签名明确无效/issuer 错误”是否应该 fallback，必须检查错误分类，不能把所有验证失败当网络故障。

高风险写接口不应在未评估的 remote->local fallback 下运行。策略选择必须成为接入方的安全配置，而不是 SDK 隐式魔法。

## 6. JWKS 缓存链

JWKS manager 支持 HTTP/gRPC/seed fetcher、cache、refresh 和 circuit breaker。缓存用于减少 IAM 热路径，但 key rotation 要满足：

```text
服务端先发布新公钥
  -> 客户端 refresh 能看到
  -> 服务端再用新私钥签名
  -> 旧公钥保留 grace 至旧 token 过期和缓存传播完成
```

客户端遇到未知 kid 时应触发受控 refresh，不能永久负缓存；也不能在 refresh 失败时丢弃仍能验证旧 token 的最后好快照。

## 7. 错误处理

统一使用 `pkg/sdk/errors`：

```text
AsIAMError
IsNotFound / IsUnauthorized / IsPermissionDenied / IsRetryable
GRPCCode / Message / ToHTTPStatus
```

不要解析中英文 message。服务端可能为安全原因收敛内部错误文案，但稳定 code/status 仍可支持程序分支。日志应记录 request ID、method、code、latency，不记录 bearer token、密码、OTP
或完整请求体。

## 8. 最小生产接入清单

1. 使用 DNS/服务发现 endpoint，不硬编码 Pod IP；
2. 启用 TLS/mTLS并校验 server name/CA；
3. 配置正确 audience 与 service identity；
4. 为每个请求设置 deadline；
5. 只对可确认幂等的方法启用 bounded retry；
6. 传递 request-id/trace context；
7. 明确 local/remote verify 与故障 fallback；
8. 进程退出时 Close Client、Stop ServiceAuth/JWKS manager；
9. 以 SDK compile test 和真实 staging call 验证升级；
10. 业务服务仍在敏感操作上执行服务端 AuthZ。

## 9. Public API 如何防漂移

`pkg/sdk/public_api_compile_test.go` 从外部测试包导入所有承诺的 public symbol，能发现 rename/remove、internal 泄漏和构造签名变化。它不能证明网络行为或语义兼容，
因此还需要子包单测、server contract test 和接入 E2E。

升级 SDK 时，至少分开报告：

- compile surface 是否通过；
- generated proto 是否变化；
- config default 是否变化；
- retry/timeout/TLS 行为是否变化；
- staging 是否真实连接成功。

## 10. 面试追问

### 为什么 SDK 不应暴露 transport internals？

一旦调用方直接依赖 interceptor/dial 细节，SDK 就无法重构连接、重试和 observability。稳定 facade 应以业务能力和少量可注入 hook 为边界。

### 本地验签失败后总能远程 fallback 吗？

不能。未知 kid/cache 故障适合远程求证；签名错误、issuer/audience 错误可能是攻击或配置错误，不应借 fallback 放宽规则。需要按错误类别决定。

### 为什么每个 RPC 都设置 deadline？

没有 deadline 的调用可无限占用 goroutine/connection，重试与 shutdown 也无法界定。deadline 是分布式资源预算，不只是用户体验参数。

## 11. 事实来源与验证

- public SDK：`pkg/sdk`
- SDK docs/examples：`pkg/sdk/docs`、`pkg/sdk/_examples`
- generated contracts：`api/grpc`
- server registration：`internal/apiserver/transport/grpc`

```bash
go test ./pkg/sdk/...
```
