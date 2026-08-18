# AuthN：认证、会话与令牌

> 状态：已实现 · 认证模型、核心用例、Session/Token/JWKS 与已知失败窗口已按当前实现复核。

AuthN 证明“当前请求者是谁”，维持认证会话，并把认证结果转化为可验证令牌。它不维护 User/Profile 档案，不配置外部身份源，也不回答资源授权问题。

## 阅读路径

1. [模块总览](00-模块总览.md)：先建立 proof → identity → principal → session → token → authorization 的统一模型。
2. [领域模型与认证策略](01-领域模型与认证策略.md)：区分 LoginIdentity、Credential、Challenge、Principal、Session 和 Token。
3. [注册、登录与身份绑定](02-注册登录与身份绑定.md)：理解三条写链路的事务、幂等和并发边界。
4. [Session、Token 与 JWKS](03-Session-Token与JWKS.md)：理解在线状态、刷新轮换、撤销与两种验签语义。
5. [登录身份绑定](03-关键链路-Linking登录身份绑定.md)：深入理解绑定、解绑和最后一个 active identity 的并发保护。
6. [登录认证](04-关键链路-Login登录认证.md)：深入理解认证策略、失败记录和锁定语义。
7. [Token 签发、刷新与吊销](05-关键链路-Token签发刷新吊销.md)：深入理解 Redis 状态机和失败窗口。
8. [JWKS 与本地验签](06-关键链路-JWKS与本地验签.md)：深入理解密钥生命周期和离线验证边界。
9. [模块边界](07-模块边界-AuthN与Identity-IDP-AuthZ.md)：理解 User、外部身份和 Subject 的跨模块转换。
10. [分层架构与代码索引](08-分层架构与代码索引.md)：定位新增 provider、claims、refresh、JWKS 的完整修改面。

跨模块内容：

- [Identity：User、Profile 与状态一致性](../01-Identity/README.md)
- [IDP：外部身份源](../04-IDP/README.md)
- [AuthZ：资源授权](../03-AuthZ/README.md)
- [密码学、密钥与令牌](../../03-基础设施/04-密码学密钥与令牌.md)
- [Redis 与缓存一致性](../../03-基础设施/02-Redis与缓存一致性.md)

## 责任边界

```text
IDP 解析外部 provider 身份
  -> AuthN 把证明映射为 LoginIdentity / Principal / Session / Token
  -> Identity 提供 User 当前状态
  -> AuthZ 对 Principal 对应主体做资源授权
```

## 当前实现要特别记住的四点

- SignUp 的外部身份解析在事务外，本地 User/LoginIdentity/Credential 在一个 MySQL UoW 中提交。
- 用户 access token 是 RS256 JWT，但 IAM 在线验证仍检查撤销标记、Session 和主体状态。
- Refresh token 使用 Redis Lua 原子轮换；当前先延长 Session，再轮换 token，失败时存在 TTL 已变化的窗口。
- SDK 本地 JWKS 验签不具备 IAM 在线验证的即时撤销语义。

## 代码入口

- domain：`internal/apiserver/domain/authn`
- application：`internal/apiserver/application/authn`
- infra：`internal/apiserver/infra/{mysql,redis}/authn`
- transport/container：`internal/apiserver/transport/{rest,grpc}`、`internal/apiserver/container/authn`
- client SDK：`pkg/sdk/auth`

## 验证

```bash
go test ./internal/apiserver/domain/authn/... ./internal/apiserver/application/authn/... ./pkg/sdk/auth/...
```
