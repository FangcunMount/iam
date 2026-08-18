# IDP 应用凭据与 AppToken 缓存

> 状态：已实现 · 本文说明外部 provider 应用、密钥加密、轮换和访问令牌缓存；这些都不是 IAM 用户凭据或 IAM access token。

## 1. IDP 解决的是 provider 接入，不是用户登录态

IDP 管理“我们以哪个微信/企微应用调用 provider”：

```text
WechatApp
  -> encrypted Credentials
  -> provider AppAccessToken
  -> provider API
```

AuthN 管理“哪个外部身份映射到哪个 IAM User”。两者分开后，provider 应用轮换密钥不会改变 User/LoginIdentity，登录策略也不需要知道密文怎样持久化。

## 2. WechatApp 与 Credentials

`WechatApp` 包含 AppID、名称、应用类型、enabled/disabled/archived 状态和 Credentials。Credentials 当前建模三组秘密：

- `AuthSecret`：获取 provider token、交换登录 code 使用的 AppSecret；
- `MsgSecret`：CallbackToken 与消息 EncodingAESKey；
- `APISecureChannel`：预留的对称/非对称 API 安全材料。

边界必须明确：

| 名称 | 属于谁 | 用途 |
| --- | --- | --- |
| IDP AuthSecret | provider app | IAM 调 provider API |
| AuthN Credential | LoginIdentity | 用户证明自己控制登录身份 |
| IDP AppAccessToken | provider app | 调微信/企微 API |
| IAM AccessToken | Principal/Session | 调 IAM 或业务服务 |

## 3. 密文和指纹各做什么

AppSecret 使用 `SecretVault` AES-256-GCM 加密，持久格式是随机 nonce 与 ciphertext 拼接；启动装配要求 32 字节 master key。GCM 同时提供机密性和完整性，错误 key 或被篡改密文都会解密失败。

`Fingerprint = SHA-256(plaintext)` 不用于恢复 secret，只用于判断新输入与现有 secret 是否相同，以实现幂等轮换。Fingerprint 本身仍是敏感元数据：若 secret 低熵，攻击者可离线猜测；当前 AppSecret 预期高熵，且日志仍不应输出指纹或密文。

当前本地 Vault 的限制：

- master key 在进程内存中；
- 密文没有绑定 appID/version 作为 GCM AAD，数据库行间密文置换不能由 AEAD 上下文直接发现；
- `Sign` 未实现，明确要求 KMS/HSM；
- 不提供 master-key version 或 rewrap 流程。

生产强化方案是 envelope encryption：数据密钥加密 secret，KMS 主密钥只包装数据密钥；密文携带 key version，并把 appID/credential kind/version 作为 AAD。当前代码尚未实现，不能把本地 Vault 描述为 KMS 等价方案。

## 4. 当前轮换语义

`RotateAuthSecret`：

1. 拒绝空值、过短 secret 和 archived app；
2. fingerprint 相同则幂等返回；
3. 用 Vault 加密新 secret；
4. 覆盖当前 cipher/fingerprint；
5. version 加一并记录 LastRotatedAt；
6. repository 更新应用行。

`RotateMsgAESKey` 类似，并校验 43 字符 EncodingAESKey。

当前不是双凭据窗口：旧密文被直接覆盖，没有 previous secret、激活时间或回滚槽位。`RotateAPISymKey` 和 `RotateAPIAsymKey` 目前直接返回 nil，属于未实现占位；调用方不能据此声称 API secure channel 已完成轮换。

另一个实现细节是 rotater 调 Vault 时使用 `context.Background()`，不会继承请求取消和 deadline。加密当前是本地快速操作，影响有限；若替换为远程 KMS，这会成为不可控尾延迟和 shutdown 风险，应改为传入 ctx。

## 5. AppAccessToken 缓存

外部 provider token 有过期时间，不能每次 API 调用都重新获取。`AccessTokenCacher.EnsureToken` 使用：

- Redis JSON String 保存 token/ExpiresAt；
- 默认提前 120 秒判断即将过期；
- 按 appID 获取 10 秒分布式 refresh lease，防止多实例击穿 provider；
- holder 从 provider 拉取并回填；
- 非 holder 再读缓存，仍不可用则返回“refresh in progress”。

缓存读取错误设置为 `IgnoreGetError=true`，会继续尝试拿锁和回源；但锁本身依赖 Redis，所以 Redis 完全故障仍会使获取失败，而不是无条件直连 provider。这样避免所有实例在 Redis 故障时同时打爆外部 API。

## 6. TTL 和 stale 窗口

正常回填 TTL 为：

```text
ExpiresAt - now - refreshSkew
```

但最短强制为 60 秒。锁竞争失败后的 `RereadUsable` 只检查 token 非空，不再次检查 ExpiresAt。这意味着当 provider 返回的剩余寿命很短、Redis key 因 minimum TTL 仍存在时，竞争者可能拿到已经进入 skew、甚至实际过期的 token。

这是当前实现的可用性风险，不是理想行为。改进应至少让 reread 判断 `ExpiresAt > now`；是否允许 skew 内 stale token，应按 provider API 的容错明确区分，而不能只看非空。

## 7. 轮换与缓存的一致性

AuthSecret 轮换当前不会删除按 appID 缓存的 AppAccessToken。若 provider 允许旧 token 自然过期，这可以避免无意义刷新；若 provider 在 secret 轮换时立即使旧 token 失效，缓存会持续返回不可用 token，直到 TTL 或显式 Refresh。

更安全的选择是轮换提交后使 access-token cache 失效，或把 credential version 纳入缓存 key/entry 并在命中时校验。当前两者都未实现，运行手册应在 provider 轮换后主动刷新并做 API smoke test。

## 8. 备选设计

| 方案 | 优点 | 代价 | 当前选择 |
| --- | --- | --- | --- |
| 配置文件明文 secret | 简单 | 泄漏、轮换和审计差 | 不采用 |
| DB AES-GCM + 进程 master key | 易部署、密文落库 | key 管理和轮换能力有限 | 当前实现 |
| 云 KMS/envelope | 权限、审计、轮换更强 | 成本、延迟、可用性依赖 | 推荐演进 |
| 每请求获取 provider token | 无本地 stale | provider 压力和延迟高 | 不采用 |
| Redis + lease read-through | 多实例共享、防击穿 | Redis 依赖与 stale 策略复杂 | 当前实现 |

## 9. 面试追问

### 为什么加密后还要 fingerprint？

AES-GCM 使用随机 nonce，同一明文每次密文不同，不能用密文相等判断幂等。Fingerprint 允许比较是否同一个高熵 secret，但不能用于解密。

### singleflight 与分布式锁有什么区别？

进程内 singleflight 只能合并一个进程的请求；Redis lease 能协调多个实例。锁要有短 TTL，并在拿不到时重新读取缓存，否则会发生不必要的失败或惊群。

### Secret 轮换为什么不能只更新数据库？

还需考虑依赖该 secret 产生的 token/cache、provider 激活顺序、失败回滚、旧 secret 宽限和验证。当前实现只完成单槽覆盖，运维必须补主动刷新与 smoke test。

## 10. 事实来源与验证

- domain：`internal/apiserver/domain/idp/wechatapp`
- application：`internal/apiserver/application/idp/wechatapp`
- vault：`internal/apiserver/infra/crypto/secret_vault.go`
- Redis：`internal/apiserver/infra/cache/redis/accesstoken_cache.go`
- provider adapter：`internal/apiserver/container/idp/token_provider.go`
- repository：`internal/apiserver/infra/mysql/wechatapp`

```bash
go test ./internal/apiserver/domain/idp/... ./internal/apiserver/application/idp/... ./internal/apiserver/infra/cache/redis ./internal/apiserver/infra/mysql/wechatapp ./internal/apiserver/container/idp
```
