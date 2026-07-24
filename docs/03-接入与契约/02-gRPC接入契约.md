# gRPC 接入契约

> 状态：设计目标 · gRPC 接入总入口，待继续按 `api/grpc/**/*.proto`、`internal/apiserver/transport/grpc`、interceptor、错误模型、proto 生成物、契约测试和 SDK 逐项核对。

---

## 1. 本文回答

本文回答 10 个问题：

- IAM gRPC 接入契约以什么为准？
- 为什么本文不手写完整 service/message schema？
- gRPC API 当前暴露哪些业务模块？
- gRPC 请求如何携带认证信息？
- gRPC 中认证成功和授权通过如何区分？
- gRPC status code 应如何表达参数错误、认证错误、授权错误和系统错误？
- proto、transport/grpc、application use case 三者如何对齐？
- gRPC 与 REST 的契约边界是什么？
- 修改 gRPC 契约时应该同步检查哪些文件？
- 修改后应该执行哪些 Verify？

---

## 2. 30 秒结论

IAM gRPC 契约以 proto 文件为准：

```text
api/grpc/iam/identity/v2/identity.proto
api/grpc/iam/authn/v2/authn.proto
api/grpc/iam/authz/v2/authz.proto
api/grpc/iam/idp/v2/idp.proto
```

当前没有 Suggest gRPC proto、service 注册或 SDK；Suggest 只暴露 REST 契约。

gRPC runtime 入口：

```text
internal/apiserver/transport/grpc
```

gRPC 接入主线：

```text
gRPC request
  -> grpc server
  -> interceptor: trace / logging / authn / authz / rate limit
  -> service method
  -> proto message mapper
  -> application command/query
  -> domain/application result
  -> proto response mapper
  -> gRPC response / status error
```

最重要的边界：

```text
proto 是 gRPC 机器契约；
gRPC 文档不手写完整 message schema；
transport/grpc 不承载业务规则，只做协议适配；
认证由 AuthN 完成；
授权由 AuthZ Check 完成；
Token 验签成功不等于授权通过；
proto message 不是 domain entity；
mapper 不进入 domain；
gRPC 更适合可信服务间调用。
```

如果只记一句话：

> gRPC 契约看 proto，业务语义看 02-业务模块，运行时适配看 transport/grpc。

---

## 3. 事实源

### 3.1 机器事实源

gRPC 契约以 proto 文件为准：

| 模块 | proto 文件 | 业务入口 |
| --- | --- | --- |
| Identity | `../../api/grpc/iam/identity/v2/identity.proto` | [Identity](../02-业务模块/01-Identity/README.md) |
| AuthN | `../../api/grpc/iam/authn/v2/authn.proto` | [AuthN](../02-业务模块/02-AuthN/README.md) |
| AuthZ | `../../api/grpc/iam/authz/v2/authz.proto` | [AuthZ](../02-业务模块/03-AuthZ/README.md) |
| IDP | `../../api/grpc/iam/idp/v2/idp.proto` | [IDP](../02-业务模块/04-IDP/README.md) |
| Suggest | 当前未提供 gRPC 契约（仅 REST） | [Suggest](../02-业务模块/05-Suggest/README.md) |

注意：如果某个模块尚未暴露 gRPC 契约，应以当前 `api/grpc` 目录为准，不要在文档中把未实现 RPC 写成已存在事实。

---

### 3.2 运行时事实源

gRPC runtime 入口：

```text
../../internal/apiserver/transport/grpc
```

相关事实源：

| 事实 | 路径 |
| --- | --- |
| gRPC server / service register | `../../internal/apiserver/transport/grpc` |
| proto mapper | `../../internal/apiserver/transport/grpc` |
| interceptor | `../../internal/apiserver/transport/grpc` |
| AuthN interceptor / token verifier | `../../internal/apiserver/transport/grpc`、`../../internal/apiserver/application/authn`，具体以代码为准 |
| AuthZ interceptor / RouteAuthorizer | `../../internal/apiserver/transport/grpc`、`../../internal/apiserver/application/authz`，具体以代码为准 |
| Identity application | `../../internal/apiserver/application/identity` |
| AuthN application | `../../internal/apiserver/application/authn` |
| AuthZ application | `../../internal/apiserver/application/authz` |
| IDP application | `../../internal/apiserver/application/idp` |
| Suggest application | `../../internal/apiserver/application/suggest` |
| Container wiring | `../../internal/apiserver/container` |
| generated proto | `../../api/grpc`、`../../pkg`，具体以代码生成配置为准 |
| 架构测试 | `../../internal/pkg/architecture` |

注意：上表路径需要继续与当前源码核对。如果源码目录已调整，应以代码为准并同步更新本文。

---

## 4. 本文不手写完整 proto schema

本文不复制 proto 中的完整 service、message、field、enum 定义。

原因：

```text
proto 已经是机器可读契约；
手写 message schema 容易和 proto 漂移；
SDK、契约测试和服务间调用代码应从 proto 生成或校验；
本文应解释接入语义、模块边界、metadata、认证授权、错误模型和修改路径；
字段名、枚举、reserved、deprecated、oneof、optional 以 proto 为准。
```

正确关系：

```text
proto
  -> service / rpc / request message / response message / enum / package / option

gRPC 接入契约文档
  -> 接入语义 / 模块归属 / metadata / 认证授权 / 错误模型 / 修改检查清单

业务模块文档
  -> 领域模型 / 关键链路 / 模块边界 / 代码事实源
```

---

## 5. gRPC 分模块契约

gRPC 契约按业务模块拆分。

推荐理解方式：

```text
iam.identity.v2.*  -> Identity
iam.authn.v2.*     -> AuthN
iam.authz.v2.*     -> AuthZ
iam.idp.v2.*       -> IDP
iam.suggest.v2.*   -> Suggest，若已存在
```

具体 package、service、rpc、message 名称以 proto 为准。

---

### 5.1 Identity gRPC

Identity gRPC 负责内部身份事实的服务间接入。

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
Identity gRPC 不负责登录认证；
Identity gRPC 不签发 Token；
Identity gRPC 不管理 RoleBinding；
Identity gRPC 不解析微信 openid；
Identity gRPC 不执行 Profile suggest 搜索。
```

业务语义见 [Identity](../02-业务模块/01-Identity/README.md)。

---

### 5.2 AuthN gRPC

AuthN gRPC 负责认证能力和认证结果的服务间接入。

典型语义：

```text
register / onboarding；
login；
link login identity；
refresh token；
logout；
Principal resolve；
JWKS 或 token verification related RPC，具体以 proto 为准。
```

边界：

```text
AuthN gRPC 负责认证，不负责资源授权；
Token 验签成功不等于授权通过；
AuthN gRPC 不写 RoleBinding；
AuthN gRPC 不管理 provider app secret；
AuthN gRPC 不维护 Profile 搜索索引。
```

业务语义见 [AuthN](../02-业务模块/02-AuthN/README.md)。

---

### 5.3 AuthZ gRPC

AuthZ gRPC 负责授权策略管理、权限检查和服务间授权能力。

典型语义：

```text
Role；
Permission；
RoleBinding；
PolicyVersion；
Check；
BatchCheck / Filter，若已实现；
策略发布 / runtime reload，具体以 proto 为准。
```

边界：

```text
AuthZ gRPC 不负责登录；
AuthZ gRPC 不校验 password / otp；
AuthZ gRPC 不创建 User/Profile；
AuthZ gRPC 不解析 ExternalIdentity；
AuthZ gRPC 不维护 Suggest index。
```

业务语义见 [AuthZ](../02-业务模块/03-AuthZ/README.md)。

---

### 5.4 IDP gRPC

IDP gRPC 负责外部身份源配置、AppToken 内部能力和 ExternalIdentity 解析的服务间接入。

典型语义：

```text
WechatApp / ProviderApp 配置；
Credentials 轮换；
AppToken 获取，具体是否暴露以 proto 为准；
ExternalIdentity 解析；
provider callback 解析结果接入，若已实现。
```

边界：

```text
IDP gRPC 不创建 LoginIdentity；
IDP gRPC 不创建 User；
IDP gRPC 不签发 IAM Token；
IDP gRPC 不写 RoleBinding；
provider AppToken 不能作为 IAM AccessToken 暴露给调用方。
```

业务语义见 [IDP](../02-业务模块/04-IDP/README.md)。

---

### 5.5 Suggest 当前不是 gRPC 能力

当前仓库没有 Suggest proto、gRPC service 注册或 SDK。Suggest 通过 REST 暴露；如果未来新增 gRPC，必须先增加机器契约、service 注册、错误映射和测试，再把本节改成现行契约说明。

业务语义见 [Suggest](../02-业务模块/05-Suggest/README.md)。

---

## 6. gRPC 请求处理主线

标准 gRPC 请求处理链路：

```text
gRPC request
  -> server method match
  -> trace / request id interceptor
  -> logging interceptor
  -> authn interceptor，若接口需要认证
  -> authz interceptor / method authorizer，若接口需要授权
  -> rate limit interceptor，若接口需要限流
  -> service method decode proto message
  -> mapper to application command/query
  -> application result / error
  -> mapper to proto response
  -> gRPC response / status error
```

关键边界：

```text
service method 只做协议适配；
interceptor 提供通用横切能力；
application 承载用例编排；
domain 承载模型和不变量；
infra 承载数据库、缓存、provider、索引等技术细节；
proto message 不应直接穿透为 domain entity。
```

---

## 7. metadata 与认证接入

gRPC 认证通常通过 metadata 携带 Bearer Token。

典型 metadata：

```text
authorization: Bearer <access_token>
```

认证主线：

```text
gRPC metadata
  -> AuthN interceptor
  -> token verification / blacklist / session context，具体以代码为准
  -> Principal
  -> attach Principal to context.Context
```

边界：

```text
AccessToken 由 AuthN 签发；
RefreshToken 不应作为普通 gRPC metadata token 使用；
JWKS 用于本地验签公钥发布，不暴露私钥；
微信 access_token / provider AppToken 不是 IAM AccessToken；
认证失败应返回 Unauthenticated。
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

gRPC 层常见授权入口：

```text
method-level authorizer；
service method explicit Check；
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
授权失败应返回 PermissionDenied。
```

---

## 9. 错误模型

gRPC 错误应使用稳定的 status code，并在必要时使用 details 表达可公开的字段级错误。

建议使用：

```text
codes.InvalidArgument；
codes.Unauthenticated；
codes.PermissionDenied；
codes.NotFound；
codes.AlreadyExists；
codes.FailedPrecondition；
codes.Aborted；
codes.ResourceExhausted；
codes.Unavailable；
codes.DeadlineExceeded；
codes.Internal。
```

---

### 9.1 gRPC status 语义

| status code | 语义 | 常见场景 |
| --- | --- | --- |
| `OK` | 请求成功 | 查询成功、操作成功 |
| `InvalidArgument` | 参数错误 | request message 字段非法、业务参数非法 |
| `Unauthenticated` | 未认证 | token 缺失、无效、过期 |
| `PermissionDenied` | 无权限 | AuthZ deny、能力未开启、手机号搜索不允许 |
| `NotFound` | 不存在 | 资源不存在或按安全策略隐藏存在性 |
| `AlreadyExists` | 已存在 | 唯一约束冲突、重复创建 |
| `FailedPrecondition` | 前置条件不满足 | disabled app、状态不允许、策略未发布 |
| `Aborted` | 并发冲突 | 版本冲突、事务冲突 |
| `ResourceExhausted` | 限流或资源耗尽 | 手机号搜索、高频请求、配额不足 |
| `Unavailable` | 依赖不可用 | 下游服务、provider、runtime snapshot 不可用 |
| `DeadlineExceeded` | 超时 | provider timeout、内部依赖超时 |
| `Internal` | 内部错误 | 未预期错误 |

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

gRPC 接入必须遵循：

```text
所有需要身份的 RPC 必须显式走认证 interceptor；
管理类 RPC 必须经过 AuthZ Check；
Token、secret、password、otp、session_key、provider access_token 不进日志；
手机号、证件号等敏感字段默认脱敏；
Suggest 只返回 mobile_mask，不返回 mobile；
IDP 不把 provider AppToken 返回为 IAM Token；
status error 不泄露敏感实现细节；
metadata 和 message 日志要做脱敏。
```

proto message 应避免：

```text
返回 raw secret；
返回 app secret；
返回 provider access_token；
返回完整 RefreshToken，除非是明确的 AuthN token RPC；
返回明文手机号 / 证件号；
返回内部策略、RoleBinding、search token 等不该暴露的实现细节。
```

---

## 11. proto message 与领域模型边界

proto message 是协议层对象，不是领域对象。

正确关系：

```text
proto request message
  -> mapper
  -> application command/query
  -> domain object / value object
  -> application result
  -> mapper
  -> proto response message
```

禁止：

```text
service method 直接操作 domain repository；
proto message 直接作为 domain entity 保存；
domain import generated proto；
application 返回数据库 record 给 service method；
service method 绕过 application 直接调用 infra。
```

---

## 12. REST 与 gRPC 的关系

REST 与 gRPC 是两套接入协议，不应互相替代对方的契约。

推荐关系：

```text
REST：面向外部客户端、管理端、小程序、HTTP 接入；
gRPC：面向可信服务间调用、内部高性能接口、强类型契约。
```

要求：

```text
REST DTO 和 proto message 可以语义对齐，但不是同一个类型；
REST handler 和 gRPC service 都应调用 application use case；
业务规则不应分别写在 REST handler 和 gRPC service 中；
错误语义应尽量保持映射一致；
字段命名、枚举语义、脱敏策略应保持一致；
OpenAPI 和 proto 都应回链业务模块文档。
```

---

## 13. 版本与兼容性

gRPC 契约当前使用 v2 package 路径：

```text
iam.<module>.v2
```

版本治理建议：

```text
破坏性字段变更需要新版本或兼容策略；
字段编号不能复用；
删除字段应使用 reserved；
新增字段通常兼容，但调用方生成代码需检查；
枚举新增值要考虑老客户端默认处理；
oneof / optional 变更需要谨慎；
service/rpc 删除或重命名属于高风险；
错误语义变更需要同步调用方；
metadata 认证方式变更需要同步所有服务。
```

---

## 14. 修改 gRPC 契约的检查清单

修改 gRPC API 时，至少检查：

```text
1. api/grpc/**/*.proto 是否更新；
2. package / service / rpc / message / enum 是否符合版本规则；
3. field number 是否稳定，不复用；
4. 删除字段是否 reserved；
5. request/response message 是否与 application command/result 对齐；
6. metadata / authn / authz 要求是否正确；
7. status code 和 error details 是否正确；
8. transport/grpc service 是否同步；
9. proto mapper 是否同步；
10. application use case 是否同步；
11. 业务模块文档是否需要更新；
12. SDK 或 generated client/server 是否需要重新生成；
13. 契约测试、架构测试、单元测试是否覆盖。
```

按模块修改时还要检查：

| 模块 | 额外检查 |
| --- | --- |
| Identity | 是否误把认证/授权逻辑写入 Identity service |
| AuthN | Token 响应、RefreshToken 安全、JWKS 公开字段 |
| AuthZ | Check 语义、RoleBinding 冲突、PolicyVersion 传播 |
| IDP | provider secret 脱敏、AppToken 不外泄、callback 安全 |
| Suggest | `mobile_mask`、限流、可见性过滤、Store/AuthZ 边界 |

---

## 15. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| 文档手写完整 proto schema | 容易和 proto 漂移 | schema 以 proto 为准 |
| service method 直接写业务规则 | transport 吞并 application | service method 只做协议适配 |
| service method 直接访问 repository | 绕过应用层 | service method 调 application use case |
| Token 验签成功就放行 | 认证和授权混淆 | 继续执行 AuthZ Check |
| proto message 当 domain entity | 协议模型污染领域模型 | proto -> command/query -> domain |
| domain import generated proto | 依赖方向错误 | mapper 留在 transport/application 边界 |
| provider raw error 原样返回 | 泄露实现细节 | 映射为稳定 status code |
| Suggest 返回 mobile 明文 | 隐私泄露 | 只返回 mobile_mask |
| IDP 返回 provider access_token 给调用方 | 凭据泄露 | AppToken 仅服务端内部使用 |
| 修改 proto 不跑生成和测试 | 契约漂移 | proto、generated、service、test 同步修改 |

---

## 16. Verify

修改 gRPC 契约后至少执行：

```bash
make proto-gen
```

涉及 REST/gRPC 一致性或 OpenAPI 时执行：

```bash
make api-validate
```

涉及 gRPC transport：

```bash
go test ./internal/apiserver/transport/grpc/...
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

涉及 SDK / generated client：

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

## 17. 本文总结

gRPC 接入契约可以压缩成：

```text
proto 定义机器契约；
transport/grpc 做协议适配；
application 承载用例编排；
domain 承载模型和不变量；
AuthN 负责认证；
AuthZ 负责授权；
业务语义回链到 02-业务模块。
```

维护 gRPC 契约时最重要的工程规则是：

```text
不要在文档中手写完整 proto schema；
不要让 service method 承载业务规则；
不要把 proto message 当领域模型；
不要让 domain import generated proto；
不要把认证成功当授权通过；
不要把 provider token 当 IAM token；
不要在 response、metadata 或日志中泄露 secret、token、手机号、证件号等敏感信息；
proto、generated、service、application、SDK、测试必须同步演进。
```
