# SDK 示例索引

这里放 **完整可运行** 的 SDK 示例。  
`pkg/sdk/docs/*` 只保留解释型短片段；需要复制运行时，优先看这里。

## 示例列表

| 示例 | 路径 | 说明 |
| ---- | ---- | ---- |
| 基础用法 | [basic/main.go](./basic/main.go) | 创建客户端、读取用户、判定档案关系 |
| mTLS | [mtls/main.go](./mtls/main.go) | 生产环境 TLS / 重试 / Keepalive 配置 |
| JWT 验证 | [verifier/main.go](./verifier/main.go) | 本地验证、JWKS、远程降级 |
| 服务间认证 | [service_auth/main.go](./service_auth/main.go) | `ServiceAuthHelper` 基础用法 |
| 授权判定 | [authz/main.go](./authz/main.go) | `Authz().Check()` / `Authz().Allow()` |
