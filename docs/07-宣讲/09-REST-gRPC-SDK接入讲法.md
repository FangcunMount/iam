# 09-REST / gRPC / SDK 接入讲法

## 1. 本文定位

本文是 `07-宣讲/` 中用于对外讲解 IAM 接入体系的表达材料。

它不替代 `docs/05-接入与契约/` 下的事实层文档，也不替代 OpenAPI、proto 或 SDK 源码。

事实层文档负责回答：

```text
业务系统如何接入 IAM；
REST API 契约如何组织；
gRPC API 契约如何组织；
Go SDK 如何封装服务端接入；
Suggest REST 如何服务 operating 后台 Profile autocomplete；
qs-server 如何接入 IAM；
接入契约如何防漂移。
```

本文负责回答：

```text
面试或技术分享中，REST / gRPC / SDK 应该怎么讲？
为什么 IAM 要同时提供 REST、gRPC 和 SDK？
三者分别服务什么调用方？
为什么 REST 适合前端和管理后台？
为什么 Suggest REST 属于管理端辅助查询入口？
为什么 gRPC 适合服务间调用？
为什么 SDK 是 Go 服务端接入产品层？
业务系统应该怎么选接入方式？
qs-server 如何完整接入 IAM？
这些契约如何防漂移？
```

一句话：

> 本文负责把 REST、gRPC、SDK 的事实层接入设计，整理成一套能面试、能白板、能技术分享、能被追问的接入体系表达；其中 Suggest REST 作为管理端 Profile autocomplete 的辅助查询入口讲解，不讲成完整搜索服务或 AuthZ 权限中心。

---

## 2. 接入体系一句话

最推荐说法：

```text
IAM 通过 REST、gRPC 和 Go SDK 提供分层接入：REST 面向 Web、App、管理后台、通用 HTTP 调试和 Suggest Profile autocomplete；gRPC 面向可信服务间调用；Go SDK 则把 Token 验证、Identity 查询、AuthZ Check、JWKS、本地验签、ServiceAuth、错误映射和超时重试封装成 Go 业务服务更容易使用的接入产品层。
```

更短版：

```text
REST 解决前端和管理端怎么接，gRPC 解决服务间怎么接，SDK 解决 Go 业务服务怎么低成本、少踩坑地接；Suggest REST 解决管理后台如何快速搜索当前可见 Profile。
```

再短一点：

```text
REST、gRPC、SDK 不是三套业务逻辑，而是同一套 IAM 能力的三种接入投影。
```

不要把它讲成：

```text
三套接口；
REST 只是调试；
gRPC 比 REST 高级所以都该用 gRPC；
SDK 是业务层；
SDK 可以替代 IAM Server；
Suggest REST 是完整搜索服务；
Suggest REST 可以绕过 ProfileAccessScope。
```

---

## 3. 30 秒讲法

```text
IAM 的接入体系分成 REST、gRPC 和 Go SDK 三层。REST 以 OpenAPI 为字段级事实源，适合 Web、App、管理后台、登录、HTTP 调试，以及 Suggest Profile autocomplete 这类管理端辅助查询；gRPC 以 proto 为字段级事实源，适合可信服务间调用，比如 VerifyToken、AuthZ Check、AuthorizationSnapshot、Identity / ProfileLink 查询和 IDP 内部能力；Go SDK 面向 Go 业务服务，封装 REST/gRPC/JWKS/ServiceAuth/AuthZ Check/Identity 查询/错误映射/配置和连接管理。三者不是重复建设，而是服务不同调用方，底层仍然回到同一套 IAM Server 的 AuthN、AuthZ、Identity、IDP 和 Suggest 能力。
```

适合场景：

```text
面试官问“外部系统怎么接 IAM”；
技术分享中快速介绍接入体系；
从系统架构过渡到业务系统接入。
```

---

## 4. 1 分钟讲法

```text
IAM 是基础服务，调用方不止一种，所以不能只提供一种接口。

前端、App、管理后台和 HTTP 调试场景需要 REST，因为 REST 对前端友好，OpenAPI 可以描述路径、字段、认证、错误响应，也适合登录、刷新 Token、当前用户、管理后台这些用户侧或管理侧能力。管理后台的 Profile 联想搜索也走 REST，例如 GET /api/v2/suggest/profile，它服务 operating 后台 autocomplete，并且返回结果必须经过 ProfileAccessScope 过滤，只返回 mobile_mask。

后端服务和 worker 更适合 gRPC，因为它们属于可信服务间调用，需要强类型 proto、deadline、metadata、service identity、AuthZ Check、Identity 查询和内部集成能力。

Go 业务服务虽然可以直接调用 REST 或 gRPC，但如果每个服务都自己写 gRPC dial、metadata、JWKS 拉取、Token Verify、AuthZ Check、Service Token、错误映射和超时重试，就会重复且容易漂移。所以 SDK 是 Go 服务端接入产品层，封装 IAM 常用接入能力，降低业务系统接入成本。

这三层不是三套业务实现。REST 和 gRPC 是机器契约，SDK 是 Go 封装，真正业务语义仍然在 IAM Server 的 AuthN、AuthZ、Identity、IDP 和 Suggest 模块。
```

适合场景：

```text
面试项目介绍中的接入部分；
技术分享 REST/gRPC/SDK 章节；
回答“为什么要同时有 REST、gRPC、SDK”。
```

---

## 5. 3 分钟讲法

```text
IAM 的接入体系，我会从调用方出发讲，而不是从接口文件出发讲。

第一类调用方是 Web、App、管理后台和普通 HTTP 客户端。这些场景最适合 REST，因为 REST 容易调试，OpenAPI 可以描述 path、schema、security、error response，也方便前端和管理端生成文档或 client。像用户登录、刷新 Token、退出登录、Me 当前用户视角、Identity 当前用户档案、AuthZ 管理接口、IDP 管理接口，都适合通过 REST 暴露。Suggest Profile autocomplete 也适合放在 REST，因为它主要服务 operating 后台输入联想，比如 GET /api/v2/suggest/profile?k=zhang&limit=20。这个接口不是完整搜索服务，而是管理端辅助查询入口，查询结果要经过 ProfileAccessScope 过滤，手机号形态关键词还需要 AllowMobileSearch，响应只返回 mobile_mask。

第二类调用方是后端服务和 worker，例如 qs-server、collection-server、qs-worker。它们更适合 gRPC，因为服务间调用需要强类型 proto、明确 deadline、metadata、service identity、重试和错误码映射。业务服务可以通过 gRPC 调 IAM 的 VerifyToken、AuthZ Check、GetAuthorizationSnapshot、GetUser、GetProfile、ListProfileLinks 等能力。

第三类调用方是 Go 业务服务。理论上 Go 服务可以直接调 REST 或 gRPC，但真实项目里这会带来大量重复接入代码：连接管理、TLS、metadata、service token、JWKS、本地验签、在线 Verify、AuthZ Check、错误映射、timeout、retry、fake client。IAM SDK 的定位就是把这些能力封装成 Go 服务端更稳定的接入产品层。SDK 不定义业务规则，不复制 IAM 数据库，也不本地实现授权。它只是把 REST/gRPC/JWKS/ServiceAuth 等能力封装给业务服务使用。

以 qs-server 为例，前端先通过 IAM REST 登录拿 AccessToken / RefreshToken；调用 qs-server 业务接口时携带 Bearer Token；qs-server 通过 SDK 或 gRPC VerifyToken 得到 Principal；然后按业务需要查询 User、Profile、ProfileLink；访问敏感资源前，qs-server 构造 resource/action/scope 调 IAM AuthZ Check；IAM 返回 AuthorizationDecision 后，qs-server 再决定放行或拒绝。这个链路说明 IAM 不是孤立系统，而是业务系统的认证授权基础服务。

最后，接入契约要防漂移。REST 字段以 OpenAPI 为准，Suggest REST 字段以 api/rest/suggest.v2.yaml 为准，gRPC 字段以 proto 为准，SDK public API 以 pkg/sdk 为准，server implementation 和 tests 决定真实行为。文档负责解释接入链路，不能替代机器契约。
```

适合场景：

```text
面试深聊业务系统接入；
技术分享接入体系章节；
回答“qs-server 如何接入 IAM”。
```

---

## 6. 推荐讲解顺序

不要从接口数量开始讲。

推荐顺序：

```text
1. 先讲调用方不同；
2. 再讲 REST 适合前端、管理端和 Suggest autocomplete；
3. 再讲 gRPC 适合可信服务间调用；
4. 再讲 SDK 适合 Go 服务端工程化接入；
5. 再讲 qs-server 接入主链路；
6. 再讲 REST/gRPC/SDK 不是三套业务逻辑；
7. 最后讲契约事实源和防漂移。
```

### 6.1 先讲调用方

```text
IAM 面向 Web、App、Admin、Backend Service、Worker、Go 业务服务，而不是单一调用方。
```

### 6.2 再讲 REST

```text
REST 适合用户侧、管理侧、HTTP 调试和管理端辅助查询。
```

其中 Suggest REST 的定位是：

```text
Profile autocomplete；
GET /api/v2/suggest/profile；
返回 mobile_mask；
结果经过 ProfileAccessScope 过滤。
```

### 6.3 再讲 gRPC

```text
gRPC 适合可信服务间调用和内部集成。
```

### 6.4 再讲 SDK

```text
SDK 适合 Go 服务端低成本接入 IAM。
```

### 6.5 最后讲防漂移

```text
OpenAPI、api/rest/suggest.v2.yaml、proto、pkg/sdk public API 和 tests 才是接入契约事实源。
```

---

## 7. 白板图讲法

### 7.1 图一：三层接入模型

```mermaid
flowchart TD
    Web["Web / App / Admin UI"]
    Backend["Backend Service / Worker"]
    GoService["Go Business Service"]

    REST["REST API<br/>OpenAPI"]
    GRPC["gRPC API<br/>Proto"]
    SDK["Go SDK<br/>Client Facade"]

    IAM["IAM Server<br/>AuthN / AuthZ / Identity / IDP / Suggest"]

    Web --> REST --> IAM
    Backend --> GRPC --> IAM
    GoService --> SDK
    SDK --> REST
    SDK --> GRPC
    SDK --> IAM
```

讲图时说：

```text
REST、gRPC、SDK 不是三套业务逻辑，而是同一套 IAM Server 能力面向不同调用方的接入投影。Suggest 也是 IAM Server 内的辅助读模型，主要通过 REST 暴露给管理端。
```

---

### 7.2 图二：接入方式选择图

```mermaid
flowchart LR
    Need["调用需求"]
    Login["用户登录 / 管理后台 / HTTP 调试"]
    SuggestNeed["Profile autocomplete"]
    S2S["服务间 Verify / AuthZ / Identity 查询"]
    GoApp["Go 服务接入 IAM"]

    REST["REST"]
    SuggestREST["Suggest REST"]
    GRPC["gRPC"]
    SDK["Go SDK"]

    Need --> Login --> REST
    Need --> SuggestNeed --> SuggestREST --> REST
    Need --> S2S --> GRPC
    Need --> GoApp --> SDK
```

讲图时说：

```text
选择接入方式不是看哪种技术更高级，而是看调用方是谁、链路是否可信、是否需要强类型、是否需要 SDK 封装。Profile autocomplete 是管理端交互，天然适合 REST。
```

---

### 7.3 图三：业务系统接入主链路

```mermaid
sequenceDiagram
    participant Client as Client / Frontend
    participant IAMRest as IAM REST AuthN
    participant QS as qs-server
    participant SDK as IAM SDK / gRPC
    participant IAM as IAM Server

    Client->>IAMRest: Login
    IAMRest-->>Client: AccessToken + RefreshToken
    Client->>QS: Business API with Bearer Token
    QS->>SDK: VerifyToken
    SDK->>IAM: AuthN Verify
    IAM-->>SDK: Principal
    SDK-->>QS: Principal
    QS->>SDK: Identity Query if needed
    SDK->>IAM: GetUser / GetProfile / ListProfileLinks
    IAM-->>SDK: Identity data
    QS->>SDK: AuthZ Check(resource, action, scope)
    SDK->>IAM: Check
    IAM-->>SDK: AuthorizationDecision
    SDK-->>QS: allow / deny
    QS-->>Client: business response
```

讲图时说：

```text
这张图说明业务系统接入 IAM 的完整链路：前端登录 IAM，业务请求进 qs-server，qs-server 通过 SDK/gRPC 验 Token、查身份、做 AuthZ Check。
```

---

### 7.4 图四：Suggest REST 查询链路

```mermaid
flowchart TD
    Admin["Admin / Operating UI"]
    REST["GET /api/v2/suggest/profile"]
    Handler["Suggest REST Handler"]
    Principal["OperatingPrincipal"]
    Scope["ProfileAccessScopeProvider"]
    Runtime["ProfileSuggestionRuntime"]
    Store["Trie / Hash Store"]
    Filter["ScopePolicy Filter"]
    DTO["mobile_mask DTO"]

    Admin --> REST --> Handler --> Principal
    Principal --> Scope
    Handler --> Runtime --> Store --> Filter --> DTO
    Scope --> Filter
```

讲图时说：

```text
Suggest REST 是管理端辅助查询入口。它不是直接 SQL 查询：请求先形成 OperatingPrincipal，再解析 ProfileAccessScope，然后从 Runtime 的 Trie/Hash 索引召回候选，经过 scope filter 后返回 mobile_mask DTO。
```

---

### 7.5 图五：契约事实源图

```mermaid
flowchart TD
    REST["REST Contract<br/>OpenAPI + handler + tests"]
    SuggestREST["Suggest REST Contract<br/>suggest.v2.yaml + handler + tests"]
    GRPC["gRPC Contract<br/>proto + generated code + tests"]
    SDK["SDK Contract<br/>pkg/sdk public API + tests"]
    Server["IAM Server Implementation"]
    Docs["docs/05 接入文档"]
    SuggestDocs["docs/08-Suggest"]
    Biz["Business Integration<br/>qs-server"]

    REST --> Server
    SuggestREST --> Server
    GRPC --> Server
    Server --> SDK
    REST --> SDK
    GRPC --> SDK
    SDK --> Biz
    REST --> Docs
    SuggestREST --> Docs
    SuggestREST --> SuggestDocs
    GRPC --> Docs
    SDK --> Docs
    Biz --> Docs
```

讲图时说：

```text
文档负责解释链路，但字段事实要回到 OpenAPI、api/rest/suggest.v2.yaml、proto、pkg/sdk public API 和 server implementation。
```

---

## 8. REST 要讲清楚什么

### 8.1 REST 的定位

```text
REST 面向 Web、App、管理后台、登录、当前用户视角、Profile autocomplete 和通用 HTTP 接入。
```

### 8.2 REST 的事实源

```text
api/rest
api/rest/suggest.v2.yaml
OpenAPI YAML / JSON
internal/apiserver/transport/rest
internal/apiserver/transport/rest/suggest
REST tests
```

一句话：

```text
OpenAPI 是 REST 字段级事实源，REST handler 是运行行为事实源；Suggest REST 还要回到 api/rest/suggest.v2.yaml 和 docs/08-Suggest 确认。
```

---

### 8.3 REST 覆盖的能力

```text
AuthN：Login、Refresh、Logout、Verify、JWKS、Me；
Identity：User、Profile、ProfileLink、当前用户视角；
AuthZ：Resource、Role、Permission、Assignment、Check、Snapshot、PolicyLinter；
IDP：WeChat / WeCom app 管理；
Suggest：GET /api/v2/suggest/profile，Profile autocomplete，返回经过 ProfileAccessScope 过滤的 mobile_mask 候选；
System：health、ready、metrics。
```

具体接口字段以 OpenAPI 为准。

Suggest 具体字段以：

```text
api/rest/suggest.v2.yaml
```

为准。

---

### 8.4 Suggest REST 要讲清楚什么

Suggest REST 的核心表达是：

```text
它是管理端辅助查询入口，不是完整搜索服务；
它服务 operating 后台 Profile autocomplete；
它支持中文名、拼音、简拼、ProfileID、手机号形态关键词；
它返回 mobile_mask，不返回明文 mobile；
它必须经过 ProfileAccessScope 过滤；
手机号形态关键词需要 AllowMobileSearch；
限流、指标和降级属于运行时护栏。
```

典型接口：

```http
GET /api/v2/suggest/profile?k={keyword}&limit={limit}
```

典型响应：

```json
[
  {
    "id": "10001",
    "name": "张三",
    "mobile_mask": "138****0000",
    "weight": 1
  }
]
```

讲解边界：

```text
能调用 suggest 接口，不等于能看到所有 Profile；
ProfileLink 不等于 ProfileAccessScope；
Suggest 不逐条调用完整 AuthZ Check；
Suggest 降级时返回空数组比绕过权限直接查 MySQL 更安全。
```

---

### 8.5 REST 适合讲的价值

```text
前端友好；
管理后台友好；
curl / Postman 调试方便；
OpenAPI 可生成文档和客户端；
适合登录、当前用户、管理面操作和 Profile autocomplete。
```

---

### 8.6 REST 不适合讲成什么

不要说：

```text
REST 是全部能力的唯一入口。
```

因为：

```text
可信服务间调用更适合 gRPC；
Go 业务服务更适合 SDK；
REST 文档不替代 OpenAPI。
```

也不要说：

```text
Suggest REST 是搜索平台；
Suggest REST 是权限判定接口；
Suggest REST 可以返回明文手机号。
```

因为：

```text
Suggest REST 只是管理端 Profile autocomplete 入口；
资源访问权仍由 AuthZ Check 判定；
手机号只能返回 mobile_mask。
```

---

## 9. gRPC 要讲清楚什么

### 9.1 gRPC 的定位

```text
gRPC 面向可信服务间调用、后端服务和 worker。
```

### 9.2 gRPC 的事实源

```text
api/grpc
.proto files
generated code
internal/apiserver/transport/grpc
gRPC tests
```

一句话：

```text
proto 是 gRPC 字段级事实源，gRPC service implementation 是运行行为事实源。
```

---

### 9.3 gRPC 覆盖的能力

```text
AuthN：Login、RefreshToken、VerifyToken、Service Token；
Identity：User、Profile、ProfileLink 查询；
AuthZ：Check、Snapshot、Assignment、Permission 管理；
IDP：外部身份源配置；
System：内部 health / runtime 状态。
```

当前 Suggest 主要通过 REST 面向管理端，不建议把 Suggest 作为服务间 gRPC 主能力来讲。

具体 service、rpc、message 以 proto 为准。

---

### 9.4 gRPC 适合的场景

```text
后端服务在线 VerifyToken；
服务间 AuthZ Check；
拉取 AuthorizationSnapshot；
系统侧 Identity 查询；
ProfileLink Query / Command；
IDP 内部高信任读取；
Service Token；
metadata / deadline / retry / audit。
```

---

### 9.5 gRPC 不适合讲成什么

不要说：

```text
gRPC 是前端直接调用入口。
```

它的定位是：

```text
可信服务间调用，不是普通公网客户端入口。
```

也不要说：

```text
gRPC 比 REST 高级，所以全部改成 gRPC。
```

协议选择看调用方，不看技术偏好。

---

## 10. SDK 要讲清楚什么

### 10.1 SDK 的定位

```text
SDK 是 Go 服务端接入 IAM 的产品化封装。
```

它不是：

```text
新的业务层；
新的授权引擎；
新的认证事实源；
IAM Server 替代品。
```

---

### 10.2 SDK 的事实源

```text
pkg/sdk
pkg/sdk public API
SDK tests
SDK examples
```

一句话：

```text
pkg/sdk public API 是 SDK 事实源，public API compile test 用来防止误删和签名漂移。
```

---

### 10.3 SDK 应该封装什么

```text
gRPC / REST client 初始化；
配置加载；
TLS / metadata / service identity；
AuthN VerifyToken；
JWKS 本地验签；
Refresh / Revoke；
Identity User / Profile / ProfileLink 查询；
AuthZ Check / Allow / AllowScoped；
AuthorizationSnapshot；
错误映射；
timeout / retry；
fake client / test helper。
```

实际 public API 以 `pkg/sdk` 为准。

---

### 10.4 SDK 的价值

```text
减少业务服务重复接入代码；
统一 IAM client 初始化；
统一错误映射；
统一 Token 验证策略；
统一 AuthZ Check 调用；
统一 Identity 查询；
统一测试 fake。
```

---

### 10.5 SDK 不应该做什么

SDK 不应该：

```text
import internal/apiserver；
访问 IAM 数据库；
本地维护 Role / Permission / RoleBinding；
直接操作 Casbin；
复制 Identity / AuthZ 业务规则；
把 JWKS 本地验签说成强状态 Verify；
替业务服务自动决定所有安全策略。
```

---

## 11. 三者的选择规则

| 场景 | 推荐接入 |
| --- | --- |
| 用户登录 | REST |
| 前端刷新 Token | REST |
| 管理后台 | REST |
| Profile 联想搜索 | REST / Suggest REST |
| 当前用户页面 | REST / 后端聚合 |
| curl / Postman 调试 | REST |
| 服务间 VerifyToken | gRPC / SDK |
| 服务间 AuthZ Check | gRPC / SDK |
| AuthorizationSnapshot | gRPC / SDK |
| Identity/Profile/ProfileLink 系统侧查询 | gRPC / SDK |
| Go 业务服务接入 | SDK |
| Worker 调 IAM | gRPC / SDK |
| API Gateway 本地验签 | JWKS / SDK TokenVerifier |
| 契约字段确认 | OpenAPI / api/rest/suggest.v2.yaml / proto / pkg/sdk |

简单规则：

```text
用户侧和管理侧优先 REST；
Profile autocomplete 走 Suggest REST；
服务间调用优先 gRPC；
Go 服务端优先 SDK；
字段级事实回到 OpenAPI / suggest.v2.yaml / proto / pkg/sdk。
```

---

## 12. 设计亮点讲法

### 12.1 亮点一：按调用方设计接入方式

推荐说法：

```text
前端/管理后台用 REST，后端服务用 gRPC，Go 服务用 SDK。
```

价值：

```text
不是为了炫技术，而是不同调用方需要不同契约形态。
```

---

### 12.2 亮点二：机器契约和 SDK 封装分离

推荐说法：

```text
OpenAPI / proto 是机器契约，SDK 是 Go 封装。
```

价值：

```text
SDK 变化不能替代契约事实源，契约变化也必须同步 SDK。
```

---

### 12.3 亮点三：SDK 不复制业务规则

推荐说法：

```text
SDK 只调用 IAM，不本地实现 AuthZ、ProfileLink、IDP 规则。
```

价值：

```text
避免业务规则出现第二事实源。
```

---

### 12.4 亮点四：qs-server 接入链路清楚

推荐说法：

```text
Client 登录 IAM，qs-server 验 Token、查身份、做 AuthZ Check。
```

价值：

```text
IAM 真正成为业务系统的身份与授权基础服务，而不是孤立模块。
```

---

### 12.5 亮点五：契约防漂移机制明确

推荐说法：

```text
REST 有 OpenAPI 和 route/schema contract，gRPC 有 proto contract，SDK 有 public API compile test，Suggest REST 有 suggest.v2.yaml 和 docs/08-Suggest，docs 有 docs-hygiene。
```

价值：

```text
接入契约不靠口头约定维护。
```

---

### 12.6 亮点六：Suggest REST 兼顾交互体验与安全边界

推荐说法：

```text
Suggest REST 服务管理端 Profile autocomplete，但不是简单开放搜索；它要经过 ProfileAccessScope 过滤，手机号形态关键词需要额外授权，响应只返回 mobile_mask，并通过限流、指标和降级保护接口安全与稳定性。
```

价值：

```text
把后台高频查询体验、安全边界和工程运行时统一起来，而不是让前端随便查全量 Profile 或直接扫 MySQL。
```

---

## 13. 面试回答模板

### Q1：为什么同时提供 REST、gRPC、SDK？

```text
因为调用方不同。REST 适合 Web、App、管理后台、登录和 Profile autocomplete；gRPC 适合后端服务间调用，比如 VerifyToken、AuthZ Check、Identity 查询；SDK 适合 Go 业务服务接入 IAM，封装连接、TLS、metadata、JWKS、Verify、ServiceAuth、AuthZ Check 和错误处理。它们不是重复业务逻辑，而是同一套 IAM 能力的不同接入投影。
```

---

### Q2：REST 和 gRPC 怎么划分？

```text
REST 更偏用户侧和管理侧，事实源是 OpenAPI，适合登录、当前用户、管理后台、Suggest autocomplete 和 HTTP 调试。gRPC 更偏服务间调用，事实源是 proto，适合 AuthN Verify、AuthZ Check、AuthorizationSnapshot、Identity/Profile/ProfileLink 系统侧接入和 IDP 高信任能力。
```

---

### Q3：SDK 为什么不是业务层？

```text
SDK 不定义 User、Session、Permission、ProfileLink 等业务规则，也不本地实现授权判定。它只是把 REST/gRPC/JWKS/ServiceAuth 这些接入能力封装成稳定 Go API。业务规则仍然在 IAM Server 的 domain/application 中。
```

---

### Q4：业务服务应该直接调 gRPC 还是用 SDK？

```text
如果是 Go 服务，优先用 SDK，因为 SDK 已经封装连接、配置、metadata、错误、JWKS、ServiceAuth 和常见 client。只有需要非常底层控制、跨语言调用或非 Go 语言时，才直接使用 gRPC proto。
```

---

### Q5：SDK 会不会隐藏太多安全细节？

```text
SDK 会降低接入成本，但不能隐藏安全语义。比如 JWKS 本地验签不等于在线 Verify，ProfileLink 不等于 AuthZ 权限，IDP 管理接口是高信任能力，Suggest autocomplete 也不等于全局可见 Profile。SDK 文档和接入文档必须把这些边界讲清楚。
```

---

### Q6：qs-server 是怎么接入 IAM 的？

```text
前端先通过 IAM REST 登录拿 AccessToken / RefreshToken，然后携带 Bearer Token 调 qs-server。qs-server 通过 SDK 或 gRPC VerifyToken 得到 Principal，再按业务需要查询 User/Profile/ProfileLink。访问敏感业务资源前，qs-server 构造 resource、action、scope 调 IAM AuthZ Check，收到 AuthorizationDecision 后决定放行或拒绝。
```

---

### Q7：为什么业务系统不直接读 IAM 数据库？

```text
因为 IAM 数据库是 IAM 内部事实源，不是接入契约。业务系统直接读库会绕过 AuthN/AuthZ/Identity 的语义、状态和权限边界，也会导致 schema 变化直接影响业务系统。正确方式是通过 REST/gRPC/SDK 接入。
```

---

### Q8：如何防止 REST/gRPC/SDK 漂移？

```text
REST 通过 OpenAPI、router matrix 和 route/schema contract 检查；Suggest REST 通过 api/rest/suggest.v2.yaml、transport/rest/suggest、docs/08-Suggest 和相关测试对齐；gRPC 通过 proto contract test 确认 proto service 有 runtime registration；SDK 通过 public API compile test 固定公开稳定面；文档通过 docs-hygiene 防止断链和旧事实回流。
```

---

### Q9：OpenAPI、proto、SDK 文档冲突时以谁为准？

```text
字段级契约以机器契约为准：REST 看 OpenAPI，Suggest REST 看 api/rest/suggest.v2.yaml，gRPC 看 proto，SDK 看 pkg/sdk public API。运行行为看 server implementation 和 tests。人类文档负责解释链路，不替代机器契约。
```

---

### Q10：REST/gRPC/SDK 是不是增加维护成本？

```text
会增加一定维护成本，所以必须有事实源和防漂移机制。但它换来的是调用方分层清晰：前端和管理端不用接 gRPC，后端服务不用手写 HTTP 接入，Go 业务服务不用重复封装 VerifyToken、AuthZ Check 和 JWKS，管理后台 Profile autocomplete 也可以通过 Suggest REST 保持统一安全边界。关键是用 OpenAPI、proto、SDK tests、Suggest tests 和文档规则控制维护成本。
```

---

### Q11：Suggest REST 和普通 REST 管理接口有什么区别？

```text
普通 REST 管理接口多是资源管理或命令/查询接口；Suggest REST 是高频 autocomplete 查询接口，主要服务管理端输入联想。它的关键不是返回尽可能多的数据，而是快速返回当前操作员可见的候选。它需要 ProfileAccessScope 过滤、手机号搜索额外授权、mobile_mask 脱敏、限流、指标和降级策略。
```

---

## 14. 不推荐的讲法

### 14.1 “我们有 REST、gRPC、SDK 三套接口”

问题：

```text
容易让人以为是三套业务逻辑。
```

推荐改成：

```text
REST、gRPC、SDK 是同一套 IAM 能力面向不同调用方的接入投影。
```

---

### 14.2 “SDK 是业务层”

问题：

```text
错误。SDK 是接入产品层，不定义业务规则。
```

推荐改成：

```text
SDK 封装 REST/gRPC/JWKS/ServiceAuth 等接入复杂度，业务规则仍在 IAM Server。
```

---

### 14.3 “gRPC 比 REST 高级，所以都用 gRPC”

问题：

```text
技术偏见。登录、管理后台和前端仍然适合 REST。
```

推荐改成：

```text
按调用方和场景选协议。
```

---

### 14.4 “REST 只是调试用”

问题：

```text
不准确。REST 是 Web、App、Admin、登录和 Suggest autocomplete 场景的正式契约。
```

---

### 14.5 “SDK 可以让业务方不用理解安全边界”

问题：

```text
不对。SDK 降低接入成本，但不能替代调用方理解 local verify、online verify、IDP secret、ProfileLink 与 AuthZ、Suggest 与 ProfileAccessScope 的边界。
```

---

### 14.6 “业务服务可以 import IAM internal 包”

问题：

```text
错误。业务服务应通过 REST/gRPC/SDK 接入，不能依赖 IAM internal 实现。
```

---

### 14.7 “Suggest REST 是完整搜索服务”

问题：

```text
错误。Suggest REST 是 Profile 联想搜索入口，不是全文检索平台。
```

推荐改成：

```text
Suggest REST 服务 operating 后台 autocomplete，解决候选召回、权限过滤、手机号安全和低延迟查询。
```

---

### 14.8 “Suggest REST 可以返回明文手机号”

问题：

```text
错误。生产环境只返回 mobile_mask，不返回明文 mobile。
```

---

### 14.9 “Suggest REST 能查到就代表有资源访问权”

问题：

```text
错误。Suggest 返回候选只代表在 autocomplete 场景可见，不代表可以访问任意业务资源。
```

资源访问权仍应通过 AuthZ Check 判定。

---

## 15. 与其他模块的关系

### 15.1 与 AuthN

```text
REST 提供登录、Refresh、Logout、Verify、JWKS；gRPC / SDK 提供服务间 Verify、Token/JWKS/ServiceAuth 相关能力。
```

### 15.2 与 AuthZ

```text
REST 提供 AuthZ 管理和 Check；gRPC / SDK 提供服务间 AuthorizationService、Check、Allow、AllowScoped、Snapshot。Suggest 消费 ProfileAccessScope 做 autocomplete 过滤，但不替代 AuthZ Check。
```

### 15.3 与 Identity

```text
REST 提供当前用户视角和管理能力；gRPC / SDK 提供系统侧 User/Profile/ProfileLink 查询与命令。Suggest 使用 ProfileSearchTerm 读模型，但 ProfileSearchTerm 不是 Profile 聚合本体。
```

### 15.4 与 IDP

```text
REST 提供 IDP 管理面；gRPC / SDK 提供高信任内部读取或服务间能力，具体以当前契约为准。
```

### 15.5 与 Suggest

```text
Suggest REST 提供 Profile autocomplete，事实源是 api/rest/suggest.v2.yaml、transport/rest/suggest、application/suggest、domain/suggest、infra/suggest 和 docs/08-Suggest。
```

### 15.6 与 qs-server

```text
qs-server 是业务系统接入 IAM 的样板：Bearer Token -> VerifyToken -> Principal -> Identity Query -> AuthZ Check -> AuthorizationDecision。
```

### 15.7 与架构护栏

```text
接入契约通过 OpenAPI、api/rest/suggest.v2.yaml、proto、SDK public API、contract tests、Suggest tests、docs-hygiene 防漂移。
```

---

## 16. 证据链索引

| 讲法 | 证据 |
| --- | --- |
| 接入总览 | `docs/05-接入与契约/00-接入总览-业务系统如何接入IAM.md` |
| REST API 契约 | `docs/05-接入与契约/01-REST API契约-前端与管理端接入.md` |
| gRPC API 契约 | `docs/05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md` |
| Go SDK 接入模型 | `docs/05-接入与契约/03-SDK接入模型-Go服务端集成.md` |
| qs-server 接入 IAM 详解 | `docs/05-接入与契约/04-业务系统接入链路-qs-server接入 IAM 详解.md` |
| 契约事实源与防漂移 | `docs/05-接入与契约/05-契约事实源与防漂移机制.md` |
| Suggest 总览 | `docs/08-Suggest/README.md` |
| Suggest 查询链路 | `docs/08-Suggest/01-查询链路-SuggestProfile从请求到索引过滤.md` |
| Suggest 权限范围 | `docs/08-Suggest/02-权限范围-OperatingPrincipal与ProfileAccessScope.md` |
| Suggest 安全与运维 | `docs/08-Suggest/05-安全与运维-手机号搜索-限流-指标-降级.md` |
| REST 机器契约 | `api/rest` |
| Suggest REST 机器契约 | `api/rest/suggest.v2.yaml` |
| gRPC 机器契约 | `api/grpc` |
| SDK public API | `pkg/sdk` |
| REST runtime | `internal/apiserver/transport/rest` |
| Suggest REST runtime | `internal/apiserver/transport/rest/suggest` |
| Suggest application | `internal/apiserver/application/suggest` |
| Suggest domain | `internal/apiserver/domain/suggest` |
| Suggest infra | `internal/apiserver/infra/suggest` |
| gRPC runtime | `internal/apiserver/transport/grpc` |
| SDK public API compile test | `pkg/sdk/public_api_compile_test.go` |
| 架构护栏 | `docs/06-架构护栏` |

不要把已归档的专题分析作为当前证据源。

---

## 17. 简历项目描述版本

```text
设计并完善 IAM 的 REST/gRPC/SDK 接入体系：REST 以 OpenAPI 为事实源，面向 Web、App 和管理后台，覆盖登录、Token、AuthZ、Identity、IDP、Suggest Profile autocomplete 等用户侧和管理侧接口；gRPC 以 proto 为事实源，面向可信服务间调用，提供 VerifyToken、AuthZ Check、AuthorizationSnapshot、Identity/Profile/ProfileLink 系统侧接入等能力；Go SDK 作为接入产品层，封装 REST/gRPC/JWKS/ServiceAuth/AuthZ Check/Identity 查询/错误映射和配置连接管理，降低 qs-server 等 Go 业务服务接入 IAM 的复杂度，并通过契约测试和 public API compile test 防止接口漂移。
```

更保守一点的版本：

```text
设计 IAM 的 REST/gRPC/SDK 接入体系：REST 面向前端和管理后台，gRPC 面向服务间调用，Go SDK 面向 Go 业务服务，并补充 Suggest REST 支持管理端 Profile autocomplete；通过 OpenAPI、proto、SDK public API、契约测试和文档检查维护接入契约一致性。
```

可以按真实贡献再压缩。

不要把尚未完整实现的跨语言 SDK、完整多语言客户端生成平台或所有协议能力说成已完成能力。

---

## 18. 30 分钟分享中的位置

如果做 30 分钟技术分享，REST/gRPC/SDK 接入建议占：

```text
4～5 分钟
```

结构：

```text
1 分钟：为什么需要三种接入方式；
1 分钟：REST 适合什么，包括 Suggest REST；
1 分钟：gRPC 适合什么；
1 分钟：SDK 封装什么；
1 分钟：qs-server 接入和契约防漂移。
```

不要在这里逐个念接口。

只需要强调：

```text
调用方不同；
接入方式不同；
底层能力相同；
事实源不同；
Suggest REST 是管理端辅助查询入口；
契约需要防漂移。
```

---

## 19. 本文总结

REST / gRPC / SDK 接入讲法的核心是：

```text
不要把它讲成“三套接口”。
```

应该讲成：

```text
三类调用方；
三种接入方式；
同一套 IAM 能力；
一套契约事实源和防漂移机制。
```

最推荐的表达：

```text
IAM 对外提供 REST、gRPC 和 Go SDK 三层接入。REST 以 OpenAPI 为事实源，面向 Web、App、管理后台、登录和 Suggest Profile autocomplete 场景；gRPC 以 proto 为事实源，面向可信服务间调用，提供 VerifyToken、AuthZ Check、Identity/Profile/ProfileLink 系统侧接入等能力；SDK 是 Go 服务端接入产品层，封装 REST/gRPC/JWKS/ServiceAuth/AuthZ Check/Identity 查询等复杂度，但不定义业务规则。三者服务不同调用方，底层仍然回到同一套 IAM Server 的 AuthN、AuthZ、Identity、IDP 和 Suggest 能力。
```

如果只记住一句话：

```text
REST、gRPC、SDK 不是三套业务逻辑，而是 IAM 面向前端、服务间调用和 Go 业务服务的三种接入投影；Suggest REST 是其中面向管理端 Profile autocomplete 的辅助查询入口；字段事实分别回到 OpenAPI、api/rest/suggest.v2.yaml、proto 和 pkg/sdk public API。
```
