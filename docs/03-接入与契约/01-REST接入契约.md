# REST 接入契约

> 状态：规划改造 · 已完成当前事实盘点；正文仍含待实现或尚未收敛的设计内容，不得作为现有能力承诺。

---

## 1. 本文回答

本文回答 10 个问题：

- IAM REST 接入契约以什么为准？
- 为什么本文不手写完整 REST schema？
- REST API 如何按 Identity / AuthN / AuthZ / IDP / Suggest 拆分？
- REST 请求如何携带认证信息？
- 认证成功和授权通过在 REST 层如何区分？
- REST 错误响应应如何表达业务错误、认证错误、授权错误和参数错误？
- REST 契约如何与业务模块文档互相引用？
- OpenAPI、transport handler、application use case 三者如何对齐？
- 修改 REST 契约时应该同步检查哪些文件？
- 修改后应该执行哪些 Verify？

---

## 2. 30 秒结论

IAM REST 契约以 OpenAPI 文件为准：

```text
api/rest/identity.v2.yaml
api/rest/authn.v2.yaml
api/rest/authz.v2.yaml
api/rest/idp.v2.yaml
api/rest/suggest.v2.yaml
```

REST runtime 入口：

```text
internal/apiserver/transport/rest
```

REST 接入主线：

```text
HTTP request
  -> REST router
  -> middleware: trace / logging / authn / authz / rate limit
  -> handler DTO decode
  -> application command/query
  -> domain/application result
  -> handler DTO encode
  -> HTTP response
```

最重要的边界：

```text
OpenAPI 是 REST 机器契约；
REST 文档不手写完整 schema；
transport/rest 不承载业务规则，只做协议适配；
认证由 AuthN 完成；
授权由 AuthZ Check 完成；
Token 验签成功不等于授权通过；
REST DTO 不是 domain entity；
错误响应不应泄露敏感实现细节。
```

如果只记一句话：

> REST 契约看 OpenAPI，业务语义看 02-业务模块，运行时适配看 transport/rest。

---

## 3. 事实源

### 3.1 机器事实源

REST 契约以 OpenAPI 文件为准：

| 模块 | OpenAPI 文件 | 业务入口 |
| --- | --- | --- |
| Identity | `../../api/rest/identity.v2.yaml` | [Identity](../02-业务模块/01-Identity/README.md) |
| AuthN | `../../api/rest/authn.v2.yaml` | [AuthN](../02-业务模块/02-AuthN/README.md) |
| AuthZ | `../../api/rest/authz.v2.yaml` | [AuthZ](../02-业务模块/03-AuthZ/README.md) |
| IDP | `../../api/rest/idp.v2.yaml` | [IDP](../02-业务模块/04-IDP/README.md) |
| Suggest | `../../api/rest/suggest.v2.yaml` | [Suggest](../02-业务模块/05-Suggest/README.md) |

---

### 3.2 运行时事实源

REST runtime 入口：

```text
../../internal/apiserver/transport/rest
```

相关事实源：

| 事实 | 路径 |
| --- | --- |
| REST router / handler | `../../internal/apiserver/transport/rest` |
| REST DTO / mapper | `../../internal/apiserver/transport/rest` |
| AuthN middleware | `../../internal/apiserver/transport/rest`、`../../internal/apiserver/application/authn` |
| AuthZ middleware / RouteAuthorizer | `../../internal/apiserver/transport/rest`、`../../internal/apiserver/application/authz` |
| Identity application | `../../internal/apiserver/application/identity` |
| AuthN application | `../../internal/apiserver/application/authn` |
| AuthZ application | `../../internal/apiserver/application/authz` |
| IDP application | `../../internal/apiserver/application/idp` |
| Suggest application | `../../internal/apiserver/application/suggest` |
| Container wiring | `../../internal/apiserver/container` |
| SDK | `../../pkg/sdk` |
| 架构测试 | `../../internal/pkg/architecture` |

注意：上表路径需要继续与当前源码核对。如果源码目录已调整，应以代码为准并同步更新本文。

---

## 4. 本文不手写完整 REST schema

本文不复制 OpenAPI 中的完整 REST schema。

原因：

```text
OpenAPI 已经是机器可读契约；
手写 schema 容易和 OpenAPI 漂移；
SDK、契约测试、接口文档应从 OpenAPI 生成或校验；
本文应解释接入语义、模块边界、认证授权、错误模型和修改路径；
字段名、枚举、required、nullable、examples 以 OpenAPI 为准。
```

正确关系：

```text
OpenAPI
  -> path / method / request / response / security / status code

REST 接入契约文档
  -> 接入语义 / 模块归属 / 认证授权 / 错误模型 / 修改检查清单

业务模块文档
  -> 领域模型 / 关键链路 / 模块边界 / 代码事实源
```

---

## 5. REST 分模块契约

REST 契约按业务模块拆分。

推荐理解方式：

```text
/api/v2/identity/*  -> Identity
/api/v2/authn/*     -> AuthN
/api/v2/authz/*     -> AuthZ
/api/v2/idp/*       -> IDP
/api/v2/suggest/*   -> Suggest
```

具体 path 前缀和 endpoint 名称以 OpenAPI 为准。

---

### 5.1 Identity REST

Identity REST 负责内部身份事实的接入。

典型语义：

```text
User；
Profile；
Child；
Guardianship；
ProfileLink；
身份事实查询；
身份事实写入。
```

边界：

```text
Identity REST 不负责登录认证；
Identity REST 不签发 Token；
Identity REST 不管理 RoleBinding；
Identity REST 不解析微信 openid；
Identity REST 不执行 Profile suggest 搜索。
```

业务语义见 [Identity](../02-业务模块/01-Identity/README.md)。

---

### 5.2 AuthN REST

AuthN REST 负责登录认证、Token 和认证结果表达。

典型语义：

```text
register / onboarding；
login；
link login identity；
refresh token；
logout；
JWKS；
Principal / session related endpoint，具体以 OpenAPI 为准。
```

边界：

```text
AuthN REST 负责认证，不负责资源授权；
Token 验签成功不等于授权通过；
AuthN REST 不写 RoleBinding；
AuthN REST 不管理 provider app secret；
AuthN REST 不维护 Profile 搜索索引。
```

业务语义见 [AuthN](../02-业务模块/02-AuthN/README.md)。

---

### 5.3 AuthZ REST

AuthZ REST 负责授权策略管理和权限检查接入。

典型语义：

```text
Role；
Permission；
RoleBinding；
PolicyVersion；
Check；
策略发布 / runtime reload，具体以 OpenAPI 为准。
```

边界：

```text
AuthZ REST 不负责登录；
AuthZ REST 不校验 password / otp；
AuthZ REST 不创建 User/Profile；
AuthZ REST 不解析 ExternalIdentity；
AuthZ REST 不维护 Suggest index。
```

业务语义见 [AuthZ](../02-业务模块/03-AuthZ/README.md)。

---

### 5.4 IDP REST

IDP REST 负责外部身份源配置和解析接入。

典型语义：

```text
WechatApp 创建、查询、更新、启用和停用；
认证密钥与消息密钥轮换；
AccessToken 获取与强制刷新。

当前 REST 不单独暴露 ExternalIdentity 解析或 provider callback 管理接口。
```

边界：

```text
IDP REST 不创建 LoginIdentity；
IDP REST 不创建 User；
IDP REST 不签发 IAM Token；
IDP REST 不写 RoleBinding；
provider AppToken 不能作为 IAM AccessToken 返回给客户端。
```

业务语义见 [IDP](../02-业务模块/04-IDP/README.md)。

---

### 5.5 Suggest REST

Suggest REST 负责 Profile 联想搜索接入。

典型语义：

```text
SuggestProfile；
keyword 查询；
ProfileAccessScope；
手机号搜索安全策略；
masked result。
```

边界：

```text
Suggest REST 不创建 Profile；
Suggest REST 不写 ProfileLink；
Suggest REST 不管理 RoleBinding；
Suggest REST 不返回明文手机号或证件号；
Suggest REST 不允许 Store / Index 绕过 application 直接返回结果。
```

业务语义见 [Suggest](../02-业务模块/05-Suggest/README.md)。

---

## 6. REST 请求处理主线

标准 REST 请求处理链路：

```text
HTTP request
  -> router match
  -> trace / request id middleware
  -> logging middleware
  -> authn middleware，若接口需要认证
  -> authz middleware / RouteAuthorizer，若接口需要授权
  -> rate limit middleware，若接口需要限流
  -> handler decode request DTO
  -> application command/query
  -> application result / error
  -> handler encode response DTO
  -> HTTP response
```

关键边界：

```text
handler 只做协议适配；
middleware 提供通用横切能力；
application 承载用例编排；
domain 承载模型和不变量；
infra 承载数据库、缓存、provider、索引等技术细节；
REST DTO 不应直接穿透为 domain entity。
```

---

## 7. 认证接入

REST 认证通常基于 Bearer Token。

典型 header：

```http
Authorization: Bearer <access_token>
```

认证主线：

```text
Authorization header
  -> AuthN middleware
  -> token verification / blacklist / session context
  -> Principal
  -> attach Principal to request context
```

边界：

```text
AccessToken 由 AuthN 签发；
RefreshToken 不应作为普通 API Bearer token 使用；
JWKS 用于本地验签公钥发布，不暴露私钥；
微信 access_token / provider AppToken 不是 IAM AccessToken；
认证失败应返回 401 / Unauthenticated 语义。
```

---

## 8. 授权接入

认证成功后，不代表授权通过。

授权主线：

```text
Principal
  -> map to AuthZ Subject
  -> build Resource / Action / Scope
  -> AuthZ Check
  -> allow / deny
```

REST 层常见授权入口：

```text
route-level authorizer；
handler-level explicit Check；
application use case 内部 Check；
batch filter，用于 Suggest 等候选过滤场景。
```

边界：

```text
Token 验签成功不等于授权通过；
Principal 不是 Subject，需要显式映射；
ProfileLink 不是 RoleBinding；
ProfileAccessScope 不是 AuthZ Scope 本体；
Suggest index 命中不等于结果可见；
授权失败应返回 403 / PermissionDenied 语义。
```

---

## 9. 错误模型

REST 错误响应应稳定、可机器处理、不过度泄露内部细节。

建议错误字段：

```text
code：稳定错误码；
message：面向调用方的简短错误说明；
trace_id：排查 ID；
details：可选，字段级错误或安全可公开细节。
```

具体字段名以 OpenAPI 为准。

---

### 9.1 HTTP 状态语义

| 状态码 | 语义 | 常见场景 |
| --- | --- | --- |
| `200` | 请求成功 | 查询成功、操作成功 |
| `201` | 已创建 | 创建资源成功，若接口采用 |
| `204` | 成功无响应体 | 删除、注销、无内容操作，若接口采用 |
| `400` | 参数错误 | request schema invalid、业务参数非法 |
| `401` | 未认证 | token 缺失、无效、过期 |
| `403` | 无权限 | AuthZ deny、能力未开启、手机号搜索不允许 |
| `404` | 不存在 | 资源不存在或按安全策略隐藏存在性 |
| `409` | 冲突 | 唯一约束冲突、重复绑定、版本冲突 |
| `422` | 语义错误 | schema 合法但业务语义不满足，若接口采用 |
| `429` | 限流 | 手机号搜索、高频请求、暴力尝试 |
| `500` | 内部错误 | 未预期错误 |
| `503` | 服务不可用 | 下游不可用、runtime snapshot 不可用、provider timeout 等 |

---

### 9.2 错误边界

```text
认证错误不要伪装成授权错误；
授权错误不要泄露资源是否存在；
参数错误应指出安全可公开的字段问题；
provider 错误不应暴露 raw provider response；
手机号搜索被拒绝或限流时不应泄露是否有匹配 Profile；
内部错误不应暴露 SQL、Redis key、secret、token、stack trace。
```

---

## 10. 安全与隐私规则

REST 接入必须遵循：

```text
所有需要身份的接口必须显式声明 security；
管理类接口必须经过 AuthZ Check；
Token、secret、password、otp、session_key、provider access_token 不进日志；
手机号、证件号等敏感字段默认脱敏；
Suggest 只返回 mobile_mask，不返回 mobile；
IDP 不把 provider AppToken 返回为 IAM Token；
错误响应不泄露敏感实现细节；
请求和响应日志要做脱敏。
```

REST DTO 应避免：

```text
返回 raw secret；
返回 app secret；
返回 provider access_token；
返回完整 RefreshToken，除非是明确的 AuthN token endpoint；
返回明文手机号 / 证件号；
返回内部策略、RoleBinding、search token 等不该暴露的实现细节。
```

---

## 11. DTO 与领域模型边界

REST DTO 是协议层对象，不是领域对象。

正确关系：

```text
REST request DTO
  -> mapper
  -> application command/query
  -> domain object / value object
  -> application result
  -> mapper
  -> REST response DTO
```

禁止：

```text
handler 直接操作 domain repository；
REST DTO 直接作为 domain entity 保存；
domain import transport/rest DTO；
application 返回数据库 record 给 handler；
handler 绕过 application 直接调用 infra。
```

---

## 12. 版本与兼容性

REST 契约当前使用 v2 文件命名：

```text
*.v2.yaml
```

版本治理建议：

```text
破坏性字段变更需要新版本或兼容策略；
新增 response 字段通常可兼容，但 SDK 生成需检查；
删除字段、改字段类型、改枚举语义属于高风险；
错误码语义变更需要同步客户端；
security scheme 变更需要同步接入方；
OpenAPI examples 应与真实响应保持一致。
```

---

## 13. 修改 REST 契约的检查清单

修改 REST API 时，至少检查：

```text
1. api/rest/*.yaml 是否更新；
2. request/response schema 是否与 application command/result 对齐；
3. security scheme 是否正确；
4. status code 和 error response 是否正确；
5. transport/rest handler 是否同步；
6. DTO mapper 是否同步；
7. application use case 是否同步；
8. 业务模块文档是否需要更新；
9. SDK 是否需要重新生成或调整；
10. 契约测试、架构测试、单元测试是否覆盖。
```

按模块修改时还要检查：

| 模块 | 额外检查 |
| --- | --- |
| Identity | 是否误把认证/授权逻辑写入 Identity handler |
| AuthN | Token 响应、RefreshToken 安全、JWKS 公开字段 |
| AuthZ | Check 语义、RoleBinding 冲突、PolicyVersion 传播 |
| IDP | provider secret 脱敏、AppToken 不外泄、callback 安全 |
| Suggest | `mobile_mask`、限流、可见性过滤、Store/AuthZ 边界 |

---

## 14. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| 文档手写完整 REST schema | 容易和 OpenAPI 漂移 | schema 以 OpenAPI 为准 |
| handler 直接写业务规则 | transport 吞并 application | handler 只做协议适配 |
| handler 直接访问 repository | 绕过应用层 | handler 调 application use case |
| Token 验签成功就放行 | 认证和授权混淆 | 继续执行 AuthZ Check |
| REST DTO 当 domain entity | 协议模型污染领域模型 | DTO -> command/query -> domain |
| provider raw error 原样返回 | 泄露实现细节 | 映射为稳定错误码 |
| Suggest 返回 mobile 明文 | 隐私泄露 | 只返回 mobile_mask |
| IDP 返回 provider access_token 给客户端 | 凭据泄露 | AppToken 仅服务端内部使用 |
| 404/403 随意混用 | 存在性泄露或语义混乱 | 按安全策略统一 |
| 修改 OpenAPI 不改 handler/test | 契约漂移 | OpenAPI、handler、test 同步修改 |

---

## 15. Verify

修改 REST 契约后至少执行：

```bash
make api-validate
```

涉及 proto 或跨协议一致性时执行：

```bash
make proto-gen
```

涉及 REST transport：

```bash
go test ./internal/apiserver/transport/rest/...
```

涉及业务模块：

```bash
go test ./internal/apiserver/application/identity/...
go test ./internal/apiserver/application/authn/...
go test ./internal/apiserver/application/authz/...
go test ./internal/apiserver/application/idp/...
go test ./internal/apiserver/application/suggest/...
```

涉及容器装配：

```bash
go test ./internal/apiserver/container/...
```

涉及 SDK：

```bash
go test ./pkg/sdk/...
```

涉及分层依赖：

```bash
go test ./internal/pkg/architecture
```

修改本文后至少执行：

```bash
make docs-hygiene
```

---

## 16. 本文总结

REST 接入契约可以压缩成：

```text
OpenAPI 定义机器契约；
transport/rest 做协议适配；
application 承载用例编排；
domain 承载模型和不变量；
AuthN 负责认证；
AuthZ 负责授权；
业务语义回链到 02-业务模块。
```

维护 REST 契约时最重要的工程规则是：

```text
不要在文档中手写完整 schema；
不要让 handler 承载业务规则；
不要把 REST DTO 当领域模型；
不要把认证成功当授权通过；
不要把 provider token 当 IAM token；
不要在响应或日志中泄露 secret、token、手机号、证件号等敏感信息；
OpenAPI、handler、application、SDK、测试必须同步演进。
```
