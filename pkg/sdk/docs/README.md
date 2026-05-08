# IAM SDK 文档索引

## 本文回答

这组文档回答两个问题：

- SDK 文档应该先看哪篇
- 文档和完整示例各自承担什么职责

## 30 秒结论

- `pkg/sdk/docs/*` 负责解释思路、参数、边界和最小可理解示例。
- `pkg/sdk/_examples/*` 负责放完整可运行程序。
- 如果你第一次接 IAM SDK，先看 [快速开始](./01-quick-start.md)。
- SDK 文档现在按“接入基础 → Token 主轴 → 授权判定 → 迁移说明”的顺序组织。
- 如果你已经知道要做什么，直接按下面的“我想...”跳转。

## 推荐阅读顺序

### 第一层：先把 SDK 跑起来

1. [快速开始](./01-quick-start.md)
2. [配置详解](./02-configuration.md)

### 第二层：搞清 Token 主轴

3. [Token 生命周期](./03-token-lifecycle.md)
4. [JWT 本地验证](./04-jwt-verification.md)
5. [服务间认证](./05-service-auth.md)

### 第三层：再接授权判定与迁移

6. [授权判定（PDP）](./06-authz.md)
7. [迁移说明](./07-migration-breaking-changes.md)

## 📚 文档列表

### 第一组：接入基础

1. **[快速开始](./01-quick-start.md)**
   - 安装和基础示例
   - 环境变量配置
   - 常见配置场景
   - 基础操作示例

2. **[配置详解](./02-configuration.md)**
   - 完整配置参数说明
   - TLS/mTLS 配置
   - 重试和超时配置
   - JWKS 配置
   - 熔断器配置
   - 环境变量映射
   - YAML 配置文件

### 第二组：Token 主轴

3. **[Token 生命周期](./03-token-lifecycle.md)**
   - 用户态 token 的消费边界
   - `VerifyToken` / `RefreshToken` / `Revoke*` / `GetJWKS`
   - `IssueServiceToken` 的调用边界

4. **[JWT 本地验证](./04-jwt-verification.md)**
   - TokenVerifier 使用
   - JWKS Manager 配置
   - 验证策略（Strategy 模式）
   - JWKS 职责链（Chain of Responsibility）
   - 性能优化
   - 监控和统计

5. **[服务间认证](./05-service-auth.md)**
   - ServiceAuthHelper 基础用法
   - 自动 Token 刷新
   - Jitter 和退避策略
   - 熔断保护
   - 状态监控
   - 生产环境最佳实践

### 第三组：授权判定

6. **[授权判定（PDP）](./06-authz.md)**
   - `Authz()` 的定位
   - `Check` / `Allow`
   - `subject / domain / object / action` 组织方式
   - 当前能力边界

7. **[迁移说明](./07-migration-breaking-changes.md)**
   - 本轮 breaking change
   - `transport` / `observability` 下线说明
   - `errors` 公开面收口
   - 替代入口与迁移示例

## 📌 当前文档边界

目前 `pkg/sdk/docs/` 已覆盖这些稳定主题：

- 快速开始
- 配置
- Token 生命周期
- JWT 本地验证
- 服务间认证
- 授权判定（PDP）
- 迁移说明

其它主题如果尚未单独成文，以这些事实入口为准：

- SDK 总览：[`pkg/sdk/README.md`](../README.md)
- 示例索引：[`pkg/sdk/_examples/README.md`](../_examples/README.md)
- 代码真值：`pkg/sdk/*.go`
- gRPC 合同：`api/grpc/**/*.proto`

## 🎯 快速导航

| 我想... | 先看 | 说明 |
| ------- | ---- | ---- |
| 快速开始使用 SDK | [快速开始](./01-quick-start.md) | 一屏建立心智模型，再看最简示例 |
| 配置开发 / 测试 / 生产环境 | [配置详解](./02-configuration.md) | 看 `Config`、TLS、超时、重试、hook 注入 |
| 搞清 token 怎么校验 / 刷新 / 撤销 | [Token 生命周期](./03-token-lifecycle.md) | 先建立 token 消费面的总心智模型 |
| 本地验证 JWT | [JWT 本地验证](./04-jwt-verification.md) | 看 verifier、JWKS、降级策略 |
| 实现服务间认证 | [服务间认证](./05-service-auth.md) | 看 helper、自动刷新、回退策略 |
| 做单次权限判定 | [授权判定（PDP）](./06-authz.md) | 看 `Authz().Check()` / `Allow()` |
| 接入身份 / 档案 / 档案关系 | [快速开始](./01-quick-start.md) | 看 `Identity()` / `Profile()` / `ProfileLink()` 与 `identity.New*FromConn` |
| 从旧 SDK 低层包迁移 | [迁移说明](./07-migration-breaking-changes.md) | 看公开面收口与替代入口，包含 REST AuthN v2 登录入口 |
| 直接复制完整程序 | [示例索引](../_examples/README.md) | 进入 `_examples` 看可运行代码 |

## 📖 文档约定

- 各篇文档会在正文里声明自己的“示例约定”；默认省略重复的 `package`、`import` 和基础 `ctx` 初始化。
- 文档内保留“最小可理解示例”，完整程序统一放在 [示例索引](../_examples/README.md)。
- 如果某个能力同时有“文档示例”和“完整示例”，先读文档，再去 `_examples` 复制运行。

## 🤝 贡献

发现文档问题？欢迎提交 Issue 或 PR：

- 文档源码：`pkg/sdk/docs/`
- 示例代码：`pkg/sdk/_examples/`

## 📝 更新日志

- **2026-04-23**: 新增迁移说明，收口 SDK 公开面到 `sdk / config / auth / authz / identity / idp / errors`
- **2026-03-27**: 收正索引页，明确 docs / `_examples` 分工，并纳入授权判定与 Token 生命周期文档
