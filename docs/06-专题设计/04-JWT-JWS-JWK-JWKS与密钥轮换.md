# JWT、JWS、JWK、JWKS 与密钥轮换

> 状态：已实现 · 概念边界与当前 IAM 的签名、发布、轮换和验证实现一致；运维细节回链到 AuthN JWKS 文档。

## 1. 概念边界

| 概念 | 作用 | 不负责 |
| --- | --- | --- |
| JWT | 表达 `iss/sub/aud/exp/nbf/iat` 等 claims | 单独不能证明可信 |
| JWS | 用私钥签名保护 JWT 完整性 | 不加密 payload |
| JWK | 用 JSON 表达一把密钥的公开参数 | `kid` 本身不是信任依据 |
| JWKS | 发布可验签的公钥集合 | 不发布私钥、不做授权 |
| Key Rotation | 管理 active → grace → retired 生命周期 | 不替代 Token/Session 撤销 |

IAM 的正确链路是：

```text
AuthN -> active private key -> signed AccessToken(kid)
     -> JWKS(active + unexpired grace public keys)
resource service -> signature + claims verification -> Principal -> AuthZ Check
```

## 2. AccessToken 与 RefreshToken

AccessToken 是短期资源访问凭证，可以通过 JWKS 本地验签。RefreshToken 是服务端管理的会话续期凭证，只能用于 refresh/logout 语义，不能被业务服务当作普通 Bearer Token。

本地验签至少校验：

- 本地算法 allowlist，禁止 `alg=none` 和算法混淆；
- 固定可信 issuer 和 audience；
- `exp`、`nbf` 及有限 clock skew；
- `kid` 对应的公钥、`kty/use/alg` 一致性；
- 签名通过后仍执行相应用例的 AuthZ。

## 3. 生命周期取舍

当前数据库在任意时刻最多一把 active。轮换时旧 active 进入 grace，新 key 成为 active；旧 AccessToken 在 grace 期间继续验证。

`grace_period` 至少覆盖 IAM 的 AccessToken TTL，并且小于轮换周期。安全事件可提前撤销非 active key，但这是有意打破旧 Token 可用性的应急操作。

发布数量上限不能优先于 Token 有效性：未过期 grace 超出 `max_publishable_keys` 时只告警，不提前撤销。

## 4. 私钥存储边界

当前实现使用 POSIX 目录中的 PEM：

- 目录 `0700`；
- 文件 `0600`；
- 同目录临时文件、`fsync`、原子重命名；
- 数据库提交前失败时删除候选文件；
- 启动验证 PEM 与数据库 public JWK 一致。

KMS/HSM 是未来的 `PrivateKeyStore/Resolver` 实现边界，不是当前能力。没有共享密钥卷时不得把当前实现扩展为多主机签名集群。

## 5. 常见反模式

| 反模式 | 后果 |
| --- | --- |
| JWKS 暴露私钥 | 签发能力泄露 |
| 信任 token header 的任意 `alg/jku/jwk` | 验签绕过或攻击者控制 key 来源 |
| 只验签，不校验 issuer/audience/时间 | 跨系统或过期 Token 被接受 |
| 轮换时立即删除旧公钥 | 未过期 AccessToken 大面积失败 |
| 多实例各自持有不同 PEM 目录 | 签名与发布状态分裂 |
| 把 JWT claims 当完整权限事实 | 权限撤销和变更无法及时生效 |

## 6. 事实源

- [AuthN JWKS 生命周期与本地验签](../02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md)
- `internal/apiserver/application/authn/jwks`
- `internal/apiserver/infra/token/keyset`
- `internal/apiserver/infra/mysql/jwks`
- `internal/pkg/migration/migrations/000016_jwks_single_active_guard.*.sql`
- `api/rest/authn.v2.yaml`

```bash
make api-validate
go test -race ./internal/apiserver/application/authn/jwks/... \
  ./internal/apiserver/infra/token/keyset/... \
  ./internal/apiserver/infra/mysql/jwks/...
```
