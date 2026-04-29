# Authn Authentication Domain

`domain/authn/authentication` 只表达认证领域模型和领域策略。

## 当前边界

- `AuthCredential` 是已经由应用层选好登录方式后构造出的领域 proof。
- `Authenticator` 只按 `AuthCredential.Scenario()` 分派到构造期注入的 `AuthStrategy`。
- 领域策略只返回 `AuthDecision`：认证是否通过、失败原因、命中的 credential、以及是否需要 credential material rotation。
- 领域层可以依赖账号仓储、凭据仓储、密码哈希、OTP verifier、IdP 交换端口，因为这些是认证判定所需的 driven ports。

## 不属于本包的职责

- 登录请求字段推断、v1/v2 登录协议差异、以及 `jwt_token` 兼容登录属于 `application/authn/login`。
- access/service token 编码、验签、刷新、撤销属于 `application/authn/token` 和 `infra/token/jwt`。
- signing key 生命周期、key source、JWKS 构建与缓存属于 `infra/token/keyset`。
- REST/gRPC DTO、错误映射、响应 envelope 属于 transport。

## 当前 proof

- `PasswordCredential`：用户名密码 proof。
- `PhoneOTPCredential`：手机号 OTP proof。
- `WechatMiniCredential`：微信小程序 code proof，应用层负责补齐 app secret。
- `WecomCredential`：企业微信 code proof，应用层负责补齐 agent 和 corp secret。

新增认证方式时，优先流程是：

1. 在 application 登录 selector 中新增协议输入到 method payload 的映射。
2. 在 application method authenticator 中构造明确的 domain proof。
3. 在 domain 中新增 `AuthCredential` 和 `AuthStrategy`。
4. 在 assembler 中显式注入 strategy。
5. 补 characterization/contract tests，确认 token 与 transport 行为不漂移。
