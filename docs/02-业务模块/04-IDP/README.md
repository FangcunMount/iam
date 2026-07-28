# IDP：外部身份源接入

> 状态：已实现 · provider 应用、凭据、AppToken 与外部身份解析边界已按当前实现复核。

IDP 管理微信/企微应用配置、敏感凭据、provider AppToken 和外部 API 适配。它说明“外部 provider 返回了什么”，AuthN 再决定“这个声明在 IAM 中能登录成谁”。

## 阅读路径

1. [模块总览](00-模块总览.md)：建立应用凭据、provider token、外部身份和 IAM 认证的统一信任链。
2. [应用凭据与 AppToken 缓存](01-应用凭据与AppToken缓存.md)：加密、轮换、Redis lease 和 stale 风险。
3. [外部身份解析与 AuthN 协作](02-外部身份解析与AuthN协作.md)：code exchange、外部标识和 LoginIdentity 边界。
4. [外部身份信任模型与方案演化](03-外部身份信任模型与方案演化.md)：provider/app 作用域、信任提升、绑定冲突、轮换协议与替代架构。
5. [模块边界与代码索引](04-模块边界与代码索引.md)：定位 provider 接入、凭据轮换、缓存与敏感契约修改面。

## 责任边界

```text
Provider app / secret / provider token
  -> IDP adapter 得到外部身份声明
  -> AuthN LoginIdentity / Principal / Session
  -> Identity User
```

- IDP `Credentials` 不是 AuthN `Credential`。
- Provider `AppAccessToken` 不是 IAM access token。
- openid/unionid/wecom userid 都不是 IAM UserID。
- IDP 不创建 User、不签发 IAM token、不做资源授权。

## 当前实现要特别记住的四点

- SecretVault 是本地 AES-256-GCM 实现，不等价于具备 key version/审计/托管签名的 KMS。
- AppSecret 当前单槽覆盖轮换，没有 previous-secret 宽限或自动 cache invalidation。
- AppToken 用 Redis 共享缓存和短 lease 防多实例击穿；锁竞争 reread 的过期判断仍有缺口。
- `RotateAPISymKey`/`RotateAPIAsymKey` 目前是未实现占位，不能作为现有能力。
- gRPC `GetWechatApp` 当前会返回解密 AppSecret，token RPC 会返回 provider token；这是依赖外围强认证与禁止响应日志的高敏感暴露面。

## 代码入口

- domain：`internal/apiserver/domain/idp/wechatapp`
- application：`internal/apiserver/application/idp`
- persistence/cache/crypto：`internal/apiserver/infra/mysql/wechatapp`、`internal/apiserver/infra/cache/redis`、`internal/apiserver/infra/crypto`
- provider adapters：`internal/apiserver/infra/wechat`、`internal/apiserver/infra/wechatapi`
- composition：`internal/apiserver/container/idp`

## 验证

```bash
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./internal/apiserver/domain/idp/... ./internal/apiserver/application/idp/... ./internal/apiserver/infra/mysql/wechatapp ./internal/apiserver/container/idp
```
