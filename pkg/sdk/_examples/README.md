# SDK 示例索引

这里放程序形态示例，须分别验证编译、可信输入和运行环境。`go test ./pkg/sdk/...`不自动遍历下划线目录。2026-10-07独立编译结果由[接入正文与台账](../../../docs/04-接口与SDK/02-Go-SDK与业务系统接入.md)记录；已单独编译的公开API库骨架也在那里。

编译成功也不证明可以连接实际IAM：basic未显式关闭默认TLS，示例字符串ID不是合法IAM数字ID；mtls构造成功不证明准入，Timeout不设置RPC deadline；verifier也须核对Manager清理及fallback范围。源码保留，本轮仅修文档，不把它们称作已验收生产程序。

## 示例列表

| 示例 | 路径 | 说明 | 2026-10-07编译 |
| ---- | ---- | ---- | ---- |
| 基础用法 | [basic/main.go](./basic/main.go) | 创建客户端、读取用户、判定档案关系 | 通过，未运行 |
| mTLS | [mtls/main.go](./mtls/main.go) | 生产环境 TLS / 重试 / Keepalive 配置 | 通过，未运行 |
| JWT 验证 | [verifier/main.go](./verifier/main.go) | 本地验证、JWKS、远程降级 | 通过，未运行 |
| 服务间认证 | [mtls/main.go](./mtls/main.go) | mTLS 证书连接 | 复用同一个mtls编译结果 |
| 授权判定 | [authz/main.go](./authz/main.go) | 旧Domain字段及Allow参数与现行v4不符，独立编译失败；当前用法见[AuthZ参考](../docs/06-authz.md) | 失败，两处旧API |
