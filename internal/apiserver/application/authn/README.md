# Authn Application Layer

应用层编排 AuthN 用例，不实现 JWT 编解码或签名密钥生命周期（见 `infra/token`）。

## 包结构（按用户旅程）

| 包 | 职责 |
| --- | --- |
| `signup` | 首次开通 User + LoginIdentity + optional Credential（`SignupService.SignUp`） |
| `linking` | 已认证 User 绑定/解绑 LoginIdentity（`Linker`） |
| `signin` | 登录主线：Method → Proof → Authenticate → IssueToken |
| `session` | Transport 门面：`Login` / `RenewSession` / `Logout`；管理员撤销见 `Revoker` |
| `token` | 签发、刷新、撤销、验票 |
| `challenge` | SMS OTP 等短期挑战 |
| `jwks` | JWKS 发布与密钥管理 |
| `credential` | 长期 Credential 认证结果记录 |
| `uow` | signup 等多仓储事务 |

已删除遗留目录：`onboarding`、`login`（无装配引用）。

## 命名对照（新人常混淆）

| 层 | 登录 | 续期 |
| --- | --- | --- |
| HTTP / OpenAPI | `POST /authn/login` | `POST /authn/refresh_token` |
| session 门面 | `Login` | `RenewSession` |
| token 机制 | `IssueToken` | `RefreshToken` |

续期语义在应用层叫 **RenewSession**；token 包与 HTTP 仍保留 **RefreshToken**，避免与 OAuth 术语和存量客户端脱节。

验票不经 session：`token.VerifyToken`（`POST /authn/verify`）。

## 用例摘要

### SignupService

`PrepareStep → ResolveUserStep → EnsureLoginIdentityStep → EnsureCredentialStep`（事务内后三步）。

Transport：`OnboardingHandler` → `/api/v2/authn/signups/*`（Handler 名称历史遗留，实现为 signup 包）。

### session.ApplicationService

- `Login` → 注入的 `signin.SignIn`
- `RenewSession` → `token.RefreshToken`
- `Logout` → token 撤销

装配时由 assembler 构建 `signin.SignIn` 再注入 session；SignIn 只依赖 `AuthenticationGrantIssuer`，session 续期与登出分别依赖 `Refresher`、`Revoker`。Grant 领域服务在创建 Session 前完成 Admission，并统一建立 `Session + TokenSet`。

### token.Capabilities

组合根输出 `AuthenticationGrantIssuer`、`Refresher`、`Revoker`、`Verifier`、`ServiceTokenIssuer` 五个窄能力。它们不是统一门面；调用方只保存真正使用的接口。JWT 为 infra 实现细节。

### linking + 敏感解绑

敏感解绑检查 `UnlinkCommand.AuthenticatedAt`（近期认证窗口），**无**独立 `Reauthenticate` 应用 API。Transport 应从已验 access token 的 `auth_time` 填入该字段；再认证能力统一走 `token.VerifyToken` + 业务传参，后续可抽 step-up 用例。

## 依赖规则

- 可依赖 AuthN domain 与应用 port。
- 不得 import JWT 库或 `infra/token`。
- Domain 不得出现 JWT/JWKS/Token 编码细节。
