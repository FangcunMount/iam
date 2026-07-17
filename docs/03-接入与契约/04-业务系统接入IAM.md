
# 业务系统接入 IAM

> 状态：待补证据 · 业务系统接入总入口，待继续按 REST/OpenAPI、gRPC/proto、Go SDK、AuthN/AuthZ middleware、业务系统示例和集成测试逐项核对。

---

## 1. 本文回答

本文回答 10 个问题：

- 业务系统接入 IAM 时应该先做什么？
- REST、gRPC、Go SDK 三种接入方式如何选择？
- 业务系统如何接入登录认证和 Token 校验？
- 业务系统如何接入授权 Check？
- 业务系统什么时候需要调用 Identity？
- 业务系统什么时候需要调用 IDP？
- 业务系统什么时候需要调用 Suggest？
- 业务系统如何处理错误、超时、重试和限流？
- 业务系统接入 IAM 时最容易混淆哪些边界？
- 接入完成后应该执行哪些 Verify？

本文是业务系统接入 IAM 的落地指南，不替代 REST/OpenAPI、gRPC/proto 或 Go SDK 文档。
REST 接入见 [01-REST接入契约.md](01-REST接入契约.md)；
gRPC 接入见 [02-gRPC接入契约.md](02-gRPC接入契约.md)；
Go SDK 接入见 [03-Go-SDK接入模型.md](03-Go-SDK接入模型.md)；

---

## 2. 30 秒结论

业务系统接入 IAM 的标准顺序是：

```text
选择接入方式
  -> 接入 AuthN，获得或校验 Principal
  -> 接入 AuthZ，完成资源访问 Check
  -> 按需读取 Identity 身份事实
  -> 按需接入 IDP 外部身份源
  -> 按需接入 Suggest Profile 联想搜索
  -> 统一错误、超时、重试、日志和审计
```

三种接入方式：

| 场景 | 推荐入口 |
| --- | --- |
| Web/App/管理端 HTTP 调用 | REST |
| 可信服务间调用 | gRPC |
| Go 服务端集成 | Go SDK |

最重要的边界：

```text
认证成功不等于授权通过；
Token 验签成功不等于可以访问业务资源；
业务系统不应直接解析 provider proof；
业务系统不应把 openid/unionid 当 UserID；
业务系统不应把 ProfileLink 当 RoleBinding；
业务系统不应绕过 AuthZ Check；
业务系统不应绕过 Suggest 可见性过滤直接查索引；
业务系统不应记录 token、secret、手机号、证件号等敏感明文。
```

如果只记一句话：

> 业务系统先用 AuthN 确认“谁在请求”，再用 AuthZ 确认“能不能做”，最后才按场景读取 Identity、IDP 或 Suggest 能力。

---

## 3. 典型接入路径

```text
业务系统
  -> 选择 REST / gRPC / Go SDK
  -> 接入认证：Bearer Token / Principal
  -> 接入授权：Resource / Action / Scope / Check
  -> 接入身份事实：User / Profile / ProfileLink
  -> 接入外部身份源：ExternalIdentity / provider login，若需要
  -> 接入 Profile 搜索：SuggestProfile，若需要
  -> 统一错误模型、日志脱敏、超时、重试、监控
```

建议先完成最小闭环：

```text
1. 调通 AuthN Token 校验；
2. 能从 IAM 得到 Principal；
3. 能用 Principal 映射 AuthZ Subject；
4. 能对一个业务资源做 AuthZ Check；
5. 能按需读取 User/Profile 基本身份事实。
```

不要一开始就把登录、授权、Profile 搜索、外部 provider、SDK 封装全部混在一起做。

---

## 4. 接入方式选择

### 4.1 REST

适合：

```text
Web 管理端；
小程序后端；
跨语言调用；
需要 HTTP/JSON 的接入方；
外部系统低频管理接口；
调试和人工排查。
```

事实源：

```text
api/rest/*.yaml
internal/apiserver/transport/rest
```

优点：

```text
接入门槛低；
跨语言友好；
方便网关、浏览器、管理端和调试工具接入；
契约可用 OpenAPI 管理。
```

注意：

```text
不要绕过 OpenAPI 手写字段语义；
不要把 HTTP 200 直接当业务成功，仍需看业务错误模型；
不要把 Bearer Token 验签成功当授权通过。
```

详细说明见 [01-REST接入契约.md](01-REST接入契约.md)。

---

### 4.2 gRPC

适合：

```text
可信服务间调用；
内部高性能接口；
强类型契约；
批量 Check；
内部 Identity / AuthZ / Suggest 能力调用。
```

事实源：

```text
api/grpc/**/*.proto
internal/apiserver/transport/grpc
```

优点：

```text
强类型；
性能好；
适合服务间调用；
metadata/interceptor 适合统一认证、授权、trace。
```

注意：

```text
不要让业务系统 import IAM internal 包；
proto message 不是 domain entity；
不要把 gRPC metadata 中的 token 打印到日志；
status code 要按契约处理。
```

详细说明见 [02-gRPC接入契约.md](02-gRPC接入契约.md)。

---

### 4.3 Go SDK

适合：

```text
Go 业务服务；
希望屏蔽 HTTP/gRPC 细节；
需要统一错误模型、TokenSource、context、timeout；
需要 compile test 保护接入代码。
```

事实源：

```text
pkg/sdk
```

优点：

```text
调用面稳定；
类型友好；
封装认证、错误和重试；
可统一 IAM 接入最佳实践。
```

注意：

```text
SDK 不替代 OpenAPI/proto；
SDK 不 import IAM internal；
SDK DTO 不是 domain entity；
SDK 示例不能把未实现能力写成已实现事实。
```

详细说明见 [03-Go-SDK接入模型.md](03-Go-SDK接入模型.md)。

---

## 5. 接入 AuthN：认证

### 5.1 业务目标

业务系统接入 AuthN，是为了回答：

```text
当前请求者是谁？
这个请求是否携带有效 IAM AccessToken？
认证成功后得到的 Principal 是什么？
```

认证主线：

```text
Authorization: Bearer <access_token>
  -> IAM AuthN token verification
  -> Principal
  -> attach to request context
```

---

### 5.2 接入方式

REST：

```http
Authorization: Bearer <access_token>
```

gRPC metadata：

```text
authorization: Bearer <access_token>
```

Go SDK：

```text
TokenSource
  -> AccessToken
  -> SDK attaches header / metadata
```

---

### 5.3 边界

```text
AccessToken 由 AuthN 签发；
RefreshToken 不应作为普通 API Bearer token；
微信 access_token / provider AppToken 不是 IAM AccessToken；
Token 验签成功只说明已认证，不说明有业务权限；
业务系统不要自行解析 provider code/openid 来充当 Principal；
业务系统不要在日志中打印 token。
```

业务语义见 [AuthN](../02-业务模块/02-AuthN/README.md)。

---

## 6. 接入 AuthZ：授权 Check

### 6.1 业务目标

业务系统接入 AuthZ，是为了回答：

```text
当前 Subject 能不能对某个 Resource 执行某个 Action？
```

授权主线：

```text
Principal
  -> map to Subject
  -> build Resource / Action / Scope
  -> AuthZ Check
  -> allow / deny
```

---

### 6.2 接入位置

AuthZ Check 可以出现在：

```text
网关 / middleware；
业务 handler 入口；
application use case 内部；
批量候选过滤链路；
后台管理操作之前。
```

建议：

```text
粗粒度权限在路由或 middleware 层检查；
资源实例级权限在 application use case 内检查；
候选列表可用 batch filter；
写操作必须在实际写入前检查。
```

---

### 6.3 边界

```text
认证成功不等于授权通过；
Principal 不是 Subject，需要显式映射；
ProfileLink 不是 RoleBinding；
业务系统不应复制一份本地权限规则绕过 AuthZ；
业务系统不应根据 token claims 直接判断复杂业务权限；
授权失败应返回 403 / PermissionDenied 语义。
```

业务语义见 [AuthZ](../02-业务模块/03-AuthZ/README.md)。

---

## 7. 接入 Identity：身份事实

### 7.1 业务目标

业务系统接入 Identity，是为了读取或维护 IAM 内部身份事实。

Identity 回答：

```text
User 是谁？
Profile / Child 是谁？
User 与 Profile / Child 之间有什么关系？
监护关系、档案关系、基础身份字段如何表达？
```

典型场景：

```text
业务系统需要根据 UserID 查询用户信息；
业务系统需要根据 ProfileID 查询儿童档案；
业务系统需要确认 User 与 Child/Profile 的关系；
业务系统需要在 onboarding 后创建或读取身份事实；
业务系统需要把业务数据关联到 IAM User/Profile。
```

---

### 7.2 边界

```text
Identity 不负责登录认证；
Identity 不签发 Token；
Identity 不管理 RoleBinding；
Identity ProfileLink 不等于 AuthZ RoleBinding；
业务系统不应把 LoginIdentity 当 User；
业务系统不应把 openid/unionid 当 UserID。
```

业务语义见 [Identity](../02-业务模块/01-Identity/README.md)。

---

## 8. 接入 IDP：外部身份源

### 8.1 业务目标

业务系统接入 IDP，是为了使用外部 provider 身份源能力。

IDP 回答：

```text
微信/企微 code 如何解析？
外部 provider 返回的身份声明是什么？
WechatApp / Credentials / AppToken 如何管理？
provider callback 如何验签或解密？
```

典型场景：

```text
业务系统需要微信小程序登录；
业务系统需要企业微信登录；
业务系统需要接入 provider callback；
业务系统需要管理 provider app 配置；
业务系统需要将 provider proof 交给 IAM 解析为 ExternalIdentity。
```

---

### 8.2 推荐链路

```text
Client provider proof
  -> Business system
  -> IAM IDP ResolveExternalIdentity
  -> ExternalIdentity
  -> IAM AuthN login/link/onboarding
  -> Principal / Token
```

注意：很多场景可以直接通过 AuthN 的 provider login/link/onboarding 入口完成，不需要业务系统直接调用 IDP。是否直接调用 IDP，以当前 REST/gRPC/SDK 契约为准。

---

### 8.3 边界

```text
IDP 只解析 ExternalIdentity；
IDP 不创建 LoginIdentity；
IDP 不创建 User；
IDP 不签发 IAM Token；
provider AppToken 不是 IAM AccessToken；
业务系统不应保存或打印 app secret、session_key、provider access_token；
业务系统不应把 openid/unionid 直接当 IAM UserID。
```

业务语义见 [IDP](../02-业务模块/04-IDP/README.md)。

---

## 9. 接入 Suggest：Profile 联想搜索

### 9.1 业务目标

业务系统接入 Suggest，是为了在可见范围内搜索可选择的 Profile 候选项。

Suggest 回答：

```text
当前请求者在允许范围内，根据 keyword 能看到哪些 Profile 候选？
```

典型场景：

```text
后台运营搜索儿童档案；
家长选择自己的儿童；
业务系统根据姓名、拼音、手机号后四位选择测评对象；
业务系统需要返回有限数量的脱敏候选项。
```

---

### 9.2 查询主线

```text
keyword
  -> SuggestProfile
  -> Snapshot match candidates
  -> ProfileAccessScope
  -> Identity/AuthZ visibility filter
  -> rank / limit
  -> mask
  -> ProfileSuggestItem
```

---

### 9.3 边界

```text
Suggest 不创建 Profile；
Suggest 不写 ProfileLink；
Suggest 不管理 RoleBinding；
ProfileSuggestionIndex 不是 Profile 主数据；
索引命中不等于可见；
业务系统不应绕过 Suggest 直接查索引；
手机号搜索不能绕过 scope、可见性过滤、限流和审计；
ProfileSuggestItem 不返回明文手机号或证件号，只返回 mobile_mask 等脱敏字段。
```

业务语义见 [Suggest](../02-业务模块/05-Suggest/README.md)。

---

## 10. 推荐接入顺序

### 10.1 最小安全接入

```text
1. 接入 AuthN Token 校验；
2. 在业务请求上下文中保存 Principal；
3. 定义业务 Resource / Action / Scope；
4. 接入 AuthZ Check；
5. 处理 401 / 403 / 429 / 5xx；
6. 日志脱敏 token 和敏感字段。
```

适合：

```text
业务系统只需要保护 API，不需要复杂身份资料或搜索。
```

---

### 10.2 身份事实接入

在最小安全接入基础上增加：

```text
1. 读取 User / Profile / ProfileLink；
2. 把业务数据绑定到 IAM UserID / ProfileID；
3. 明确 ProfileLink 和 RoleBinding 的边界；
4. 对写操作增加 AuthZ Check。
```

适合：

```text
业务系统需要用户、儿童档案、监护关系或档案关系。
```

---

### 10.3 外部登录接入

在最小安全接入基础上增加：

```text
1. 选择 AuthN provider login/link/onboarding 入口；
2. 必要时接入 IDP ExternalIdentity 解析；
3. 不保存 provider proof；
4. 不把 openid/unionid 当 UserID；
5. 登录成功后只信任 IAM Principal / Token。
```

适合：

```text
业务系统需要微信小程序、公众号、开放平台或企业微信登录。
```

---

### 10.4 Profile 搜索接入

在身份事实接入基础上增加：

```text
1. 接入 SuggestProfile；
2. 明确 ProfileAccessScope；
3. 处理空结果、限流、权限不足；
4. 只展示脱敏字段；
5. 手机号搜索走 AllowMobileSearch / RateLimit / Audit 策略。
```

适合：

```text
业务系统需要搜索、选择、绑定或展示 Profile 候选。
```

---

## 11. 错误处理

业务系统至少要区分：

| 错误 | REST | gRPC | 处理建议 |
| --- | --- | --- | --- |
| 参数错误 | 400 / 422 | InvalidArgument | 修正请求参数 |
| 未认证 | 401 | Unauthenticated | 重新登录或刷新 Token |
| 无权限 | 403 | PermissionDenied | 不要重试，提示无权限或隐藏资源 |
| 不存在 | 404 | NotFound | 按业务语义处理，注意存在性隐藏 |
| 冲突 | 409 | AlreadyExists / Aborted / FailedPrecondition | 检查幂等、版本、重复绑定 |
| 限流 | 429 | ResourceExhausted | 退避、减少频率，不泄露匹配结果 |
| 超时 | 504 / 503 | DeadlineExceeded / Unavailable | 可重试，需结合幂等性 |
| 内部错误 | 500 | Internal | 记录 traceID，告警或重试策略 |

注意：

```text
不要把所有错误都当 500；
不要对非幂等写操作盲目重试；
不要在错误日志中打印 token、secret、手机号、证件号；
不要向用户暴露 raw provider error、SQL、Redis key 或堆栈。
```

---

## 12. 超时、重试与幂等

建议：

```text
所有 IAM 调用都带 context timeout；
读操作可以按策略重试；
写操作默认不重试，除非有幂等键；
登录、绑定、onboarding、RoleBinding 写入等操作需要特别谨慎；
429 / ResourceExhausted 应退避；
provider proof 类请求通常不能无限重试；
调用方要记录 traceID，便于跨系统排查。
```

边界：

```text
业务系统重试不能绕过 IAM 限流；
业务系统重试不能重复创建 User/Profile/RoleBinding；
业务系统不要在本地缓存复杂授权结果过久；
业务系统本地缓存身份事实时要考虑更新和失效。
```

---

## 13. 日志、审计与隐私

业务系统接入 IAM 后，日志必须脱敏。

禁止记录：

```text
AccessToken；
RefreshToken；
password；
otp；
session_key；
provider app secret；
provider access_token；
微信 code / auth_code 原文；
明文手机号；
明文证件号；
完整 search token。
```

建议记录：

```text
traceID；
requestID；
operatorID hash；
resource；
action；
scope；
IAM error code；
HTTP status / gRPC code；
latency；
result allow/deny；
脱敏后的手机号，例如 mobile_mask，若确有必要。
```

---

## 14. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| 只校验 Token，不做 AuthZ Check | 认证和授权混淆 | Token -> Principal -> AuthZ Check |
| 把 openid 当 UserID | 外部身份和内部身份混淆 | 通过 AuthN/Identity 映射 |
| 把 ProfileLink 当 RoleBinding | 身份关系和授权事实混淆 | ProfileLink 归 Identity，RoleBinding 归 AuthZ |
| 业务系统直接访问 IAM 数据库 | 绕过契约和边界 | 走 REST/gRPC/SDK |
| 业务系统 import IAM internal 包 | 强耦合且不可发布 | 只依赖公开契约或 SDK |
| Suggest 索引命中直接返回 | 越权泄露 | 走 SuggestProfile 可见性过滤 |
| 手机号搜索返回明文手机号 | 隐私泄露 | 只返回 mobile_mask |
| IDP provider AppToken 当 IAM Token | token 语义混淆 | AppToken 仅调用 provider API |
| AuthZ 不可用时默认放行 | 越权风险 | fail closed 或明确降级策略 |
| 日志打印 token / secret | 严重安全问题 | 统一脱敏和禁止记录 |

---

## 15. 事实源

| 事实 | 路径 |
| --- | --- |
| REST 契约 | `../../api/rest` |
| gRPC 契约 | `../../api/grpc` |
| Go SDK | `../../pkg/sdk` |
| REST transport | `../../internal/apiserver/transport/rest` |
| gRPC transport | `../../internal/apiserver/transport/grpc` |
| Identity | `../02-业务模块/01-Identity/README.md` |
| AuthN | `../02-业务模块/02-AuthN/README.md` |
| AuthZ | `../02-业务模块/03-AuthZ/README.md` |
| IDP | `../02-业务模块/04-IDP/README.md` |
| Suggest | `../02-业务模块/05-Suggest/README.md` |
| SDK docs | `../../pkg/sdk/README.md`、`../../pkg/sdk/docs/README.md` |
| 架构测试 | `../../internal/pkg/architecture` |

注意：上表路径需要继续与当前源码核对。如果源码目录已调整，应以代码为准并同步更新本文。

---

## 16. Verify

修改业务系统接入文档后至少执行：

```bash
make docs-hygiene
```

涉及 REST 契约：

```bash
make api-validate
go test ./internal/apiserver/transport/rest/...
```

涉及 gRPC 契约：

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc/...
```

涉及 Go SDK：

```bash
go test ./pkg/sdk/...
```

涉及 IAM 业务模块：

```bash
go test ./internal/apiserver/application/identity/...
go test ./internal/apiserver/application/authn/...
go test ./internal/apiserver/application/authz/...
go test ./internal/apiserver/application/idp/...
go test ./internal/apiserver/application/suggest/...
```

涉及容器装配和架构边界：

```bash
go test ./internal/apiserver/container/...
go test ./internal/pkg/architecture
```

---

## 17. 本文总结

业务系统接入 IAM 可以压缩成：

```text
选择 REST / gRPC / Go SDK
  -> 接入 AuthN，确认当前请求者
  -> 接入 AuthZ，确认能不能访问资源
  -> 按需接入 Identity 身份事实
  -> 按需接入 IDP 外部身份源
  -> 按需接入 Suggest Profile 搜索
  -> 统一错误、超时、重试、日志、审计和隐私治理
```

最重要的工程规则是：

```text
认证不等于授权；
外部身份不等于内部用户；
身份关系不等于授权绑定；
搜索命中不等于结果可见；
业务系统不直接访问 IAM 数据库；
业务系统不 import IAM internal；
业务系统不记录 token、secret、手机号、证件号等敏感明文；
业务系统通过 REST/gRPC/SDK 这三类公开接入方式使用 IAM。
```
