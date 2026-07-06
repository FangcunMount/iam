# 领域模型：LoginIdentity / Credential / Challenge / Principal / Session / Token

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 模型定义

| 模型 | 定义 |
| --- | --- |
| LoginIdentity | User 绑定的登录身份，如 username、phone、wechat openid |
| Credential | 长期认证材料，如 password hash |
| Challenge | 短期认证挑战，如短信 OTP |
| Principal | 认证成功后的运行时主体表达 |
| Session | 服务端认证上下文 |
| AccessToken | 客户端访问资源携带的短期凭证 |
| RefreshToken | 用于续期 AccessToken 的凭证 |

## 边界

- 微信 openid / 企业微信 userid 是 LoginIdentity，不是 Credential。
- SMS OTP 是 Challenge，不是 Credential。
- Principal 是认证结果，不是 JWT。
- AccessToken 不承载完整授权模型。
