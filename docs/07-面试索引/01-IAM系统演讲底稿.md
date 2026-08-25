# IAM 系统演讲底稿

> 状态：已实现 · 本文是个人理解、背诵和追问准备用的演讲底稿，不是展示给面试官的演讲页面，也不拥有技术事实。具体结论以回链的 canonical 文档、机器契约、源码和运行证据为准。

> 仓库事实提示（2026-08-19）：IDP 已实现 `ExternalIdentity` 领域值对象和统一 Resolver，AuthN 的 SignIn、SignUp、Linking provider-code 链路均已接入。该结论只代表当前源码与聚焦测试证据；是否已提交、通过 CI 并进入实际发布，仍需在面试前按对应证据层单独确认。

## 1. 怎样使用这份底稿

这份底稿有两种内容：

- **演讲通稿**：可以直接读出声，用来建立讲述节奏和转场。
- **理解与追问卡**：不需要逐字背，用来理解模型、一致性、失败语义和当前边界。

标准通稿按 **15 分钟**设计。面试官中途追问时，不要强行讲完；先回答当前问题，再用本文的“转场句”回到主线。

建议背诵顺序：

```text
先记骨架
  -> 能脱稿讲清每节的一句话结论
  -> 再加入三条案例链路
  -> 最后练习追问和失败窗口
```

## 2. 背诵骨架

### 2.1 一个核心判断

> IAM 不是一组登录和权限 CRUD 接口，而是把身份事实、认证事实和授权事实拆成稳定边界，再用外部身份适配、派生读模型和事件投影解决接入、一致性和安全问题。

### 2.2 三个核心问题

```text
用户是谁？        -> Identity
如何证明当前是他？ -> AuthN
他能对资源做什么？ -> AuthZ
```

### 2.3 五个模块

```text
三个核心模块：Identity / AuthN / AuthZ
两个辅助模块：IDP / Suggest
```

### 2.4 三条核心案例链

```text
外部身份接入：provider proof -> IDP ExternalIdentity -> AuthN -> Identity -> Session / Token
授权决策与传播：Identity User -> AuthZ -> MySQL / Casbin -> Outbox / NSQ
可见资料搜索：Identity facts + 授权范围 -> Suggest read model
```

### 2.5 两条工程底线

```text
投影不能变成第二份真相；
一层验证通过，不能代替其他证据层。
```

只要记住“**一个判断、三个问题、五个模块、三条链、两条底线**”，就不容易在追问中失去主线。

## 3. 15 分钟标准演讲通稿

### 3.1 开场：先讲问题，不先罗列技术（约 1 分钟）

**一句话目标：** 让听众先理解 IAM 为什么存在。

**通稿：**

> 我想介绍的项目是 IAM，也就是身份与访问管理系统。
>
> 我对这个项目最核心的理解是：它不是把用户表、登录接口和权限 CRUD 放在一起，而是要把“身份事实”、“认证证明”和“授权决策”拆成稳定的边界。
>
> 业务系统接入时会反复遇到三个问题：用户是谁？怎样证明当前请求者就是这个用户？他又能对某个资源做什么？IAM 就是围绕这三个问题建立的。

**转场句：**

> 围绕这三个问题，我们没有按 API 路径或数据库表拆分，而是按业务事实的变化原因拆成五个模块。

定位事实见 [IAM 系统定位](../00-概览/01-IAM系统定位.md)。

### 3.2 模块边界：为什么是三个核心模块加两个辅助模块（约 2 分钟）

**一句话目标：** 不是背模块名，而是讲出它们各自拥有什么事实。

![IAM 模块边界与协作关系](../_images/architecture/module-boundary.png)

**通稿：**

> 三个核心模块是 Identity、AuthN 和 AuthZ。
>
> Identity 回答“用户是谁”。它拥有 User、Profile 和 ProfileLink。User 是 IAM 内部的稳定身份锚点，Profile 是业务资料，ProfileLink 表达 User 与 Profile 之间可建立、查询和撤销的关系。Identity 不验证密码，不签发 Token，也不做资源授权。
>
> AuthN 回答“怎样证明当前是他”。它拥有 LoginIdentity、Credential、Challenge、Principal、Session 和 Token。LoginIdentity 解决一个用户可以通过用户名、手机号、微信或企微等不同入口进入同一个 User 的问题。Principal 是一次认证成功的运行时结果，Session 和 Token 把这次认证延续成可撤销、可刷新的登录状态。
>
> AuthZ 回答“能对资源做什么”。它不是只检查一个 `role == admin`，而是把 Subject、Tenant、Resource、Action 和受信对象属性一起放入授权请求中，再返回允许或拒绝的 Decision。
>
> 两个辅助模块是 IDP 和 Suggest。IDP 隔离微信、企微等 provider 的应用配置、凭据、AppToken 和协议差异，并把一次 provider proof 解析成请求级、已验证的 `ExternalIdentity`；它仍然不拥有 IAM User、LoginIdentity 或登录态。Suggest 从 Identity 事实派生联想搜索索引，但它不能回写 Profile，也不能成为通用授权引擎。

**转场句：**

> 单独看模块定义还比较抽象。下面我用三条真实链路串起这五个模块。第一条是一个微信或企微身份怎样最终变成 IAM 的可信登录态。

完整边界见 [模块划分与协作关系](../00-概览/02-模块划分与协作关系.md)。

### 3.3 案例一：外部身份如何变成可信登录态（约 3 分钟）

**一句话目标：** IDP 管 provider 差异，AuthN 管证明和登录态，Identity 管稳定 User。

**通稿：**

> 以微信登录为例，客户端拿到的 code 只是一份等待验证的外部证明，它不是 IAM User，也不能直接变成 Token。
>
> 当前代码中，AuthN 的 SignIn proof factory、SignUp prepare 和 Linking prepare 都通过 IDP 暴露的统一 Resolver capability 解析 provider code。Resolver 读取 WechatApp 配置和加密凭据，通过 provider adapter 与外部平台交换，再把 provider、realm、openid/unionid 或企微 userid、verifiedAt 组装成一个不可变的 `ExternalIdentity`。
>
> `ExternalIdentity` 只能证明“当前请求已经通过 provider 验证并得到某个外部标识”。它是请求级 proof，不持久化，也不包含 IAM User、LoginIdentity 或可复用凭据。AuthN 将它映射成自己能理解的微信/企微认证视图，再根据 provider、realm 和 identifier 查找 LoginIdentity，而 LoginIdentity 指向 Identity 中的稳定 User。
>
> 只有在绑定、身份状态和认证策略都满足之后，AuthN 才会产生 Principal，创建 Session，并签发 Access Token 和 Refresh Token。
>
> 这里我们没有做成纯离线 JWT。Access Token 是 RS256 签名声明，方便下游通过 JWKS 本地验签；但在线 Verify 还会检查 Session、Token 状态和当前主体状态。所以“签名正确”和“登录状态仍然有效”是两个强度不同的结论。
>
> 这条链路最重要的设计是不让 provider 的 openid 或 unionid 污染内部 User 模型，也不让 IDP 因为能调用微信 API 就越界决定 IAM 登录态。

**当前边界，需要主动讲准确：**

> `domain/idp/externalidentity.ExternalIdentity` 当前支持 `wechat_minip`、`wechat_open` 和 `wecom` 三种 provider，并已被 SignIn、SignUp、Linking 的 provider-code 链路共同消费。但它只是“请求级、已验证的外部身份证明”，不是新的用户聚合、登录绑定表或持久化身份主数据。SignUp 内部仍保留可信预解析 OpenID/UnionID 的兼容分支，并显式标记为 `TrustedLegacyInput`；该分支不能被描述成 provider 已验证结果。

**转场句：**

> 认证成功之后，下一个问题是这个用户能做什么。这里有一个容易误解的点：AuthN 不需要直接依赖 AuthZ，两者通过 Identity 中的稳定 User 对齐。

详细链路见 [注册、登录与身份绑定](../02-业务模块/02-AuthN/02-注册登录与身份绑定.md)、[Session、Token 与 JWKS](../02-业务模块/02-AuthN/03-Session-Token与JWKS.md)、[外部身份解析与 AuthN 协作](../02-业务模块/04-IDP/02-外部身份解析与AuthN协作.md)。

### 3.4 案例二：从可信 User 到资源授权和多实例收敛（约 3 分钟）

**一句话目标：** AuthZ 用 MySQL 保存授权事实，用原生不可变快照执行高频判定，用 Outbox 和事件促进多实例投影收敛。

**通稿：**

> Token 被验证后，transport 或 middleware 只会形成可信的 `UserID` 和 `TenantID` 请求上下文。资源服务再以 Identity User 为锚点构造 AuthZ Subject，加上 Resource、Action 和已加载对象的受信属性，调用 AuthZ Check。
>
> 所以 AuthN 和 AuthZ 会在一次请求中前后衔接，但不需要让 AuthN 领域模块直接把 Principal 转成 AuthZ 的 Subject。Identity User 是它们共同的稳定身份锚点。
>
> AuthZ 采用 RBAC 加对象属性条件。Role 聚合稳定能力，PermissionGrant 表达对 Resource 的单一 Action，ConstraintSet 限定对象属性；组织归属等关系仍由拥有事实的业务模块判断。这样既不把每个对象编码进角色名，也不把业务数据库复制到 IAM。
>
> 授权系统还有一个一致性问题。为了低延迟判定，每个 IAM 实例都有原生不可变授权快照。MySQL 中的 Assignment、RoleInheritance、PermissionGrant、Resource Schema 和 PolicyVersion 是权威事实；Casbin 只在快照内计算直接及继承 Role，既不保存也不执行权限规则。
>
> 当 Grant、Revoke、Assignment 或 RoleInheritance 发生变化时，AuthZ UnitOfWork 在同一个 MySQL 事务中写入管理事实、PolicyVersion 和 Outbox event。事务提交后，当前实例直接 reload；Outbox relay 在 EventBus 启用时把版本事件发到 NSQ，其他实例用独立 ephemeral channel 订阅，再从 MySQL 重新加载。
>
> Outbox 解决的是“数据库事实已经提交，但进程在 publish 前崩溃”这个双写窗口。它保证业务提交和发布意图原子，但 MQ 成功后如果 relay 在标记 published 之前崩溃，事件还会重复发送。因此当前是 at-least-once，不是 exactly-once，消费者必须幂等。
>
> 这套方案是最终一致的。当前没有请求级的全实例 loaded-version barrier。如果是 Grant 传播延迟，旧实例可能暂时拒绝，主要是可用性风险；如果是 Revoke 传播延迟，旧实例可能暂时允许，这是更重要的安全风险。

**转场句：**

> 授权决策不只用于高风险写操作。IAM 还有一个很典型的派生场景：管理端要快速搜索自己可见范围内的 Profile。

详细设计见 [AuthZ 授权模型](../02-业务模块/03-AuthZ/01-授权模型与匹配语义.md)、[授权写入与多实例一致性](../02-业务模块/03-AuthZ/03-授权写入与多实例一致性.md)、[事件与 Transactional Outbox](../03-基础设施/03-事件与Transactional-Outbox.md)。

### 3.5 案例三：Suggest 为什么是派生读模型（约 2 分钟）

**一句话目标：** Identity 保证身份事实，Suggest 针对查询形态构建可重建投影。

**通稿：**

> Profile 主数据的正确性和联想搜索的查询形态是两个不同问题。Identity 要维护 User、Profile 和 ProfileLink 的不变量；Suggest 要支持姓名、拼音、ID 和手机号等查询键的快速召回、排序、范围过滤和脱敏。
>
> 所以 Suggest 通过 Full 或 Delta loader 从 Identity facts 派生 `ProfileSearchTerm`，然后构建进程内 Trie 和 Hash 索引。查询时，系统根据可信 OperatingPrincipal 与身份/角色事实构造 `ProfileAccessScope`，先召回，再做 scope 过滤、排序、最终 limit 和脱敏。
>
> 这里必须区分“搜索可见”和“资源授权”。Suggest 返回一个脱敏 Profile 候选，只能证明它在本次搜索范围内可见，不能推导出详情读取、编辑或导出权限。后续动作仍然要通过自己的 AuthZ Resource、Action 和必要的对象 Check。
>
> Suggest 是可重建、最终一致的读模型。当前没有持久化索引快照，进程重启后依赖 Full refresh 重建；当前也只提供 REST，没有 Suggest gRPC 或 Go SDK。

**转场句：**

> 三条链路分别展示了身份事实、认证证明、授权决策和派生投影怎样协作。为了让这些边界不只停留在图上，代码层还需要一套稳定的分层和装配方式。

完整设计见 [Suggest 为什么采用派生读模型](../06-专题设计/05-Suggest为什么是读模型.md)。

### 3.6 分层架构和运行时：边界如何落到代码（约 2 分钟）

**一句话目标：** Domain 保留规则，Application 编排用例，Infra 实现端口，Container 只负责装配。

![IAM 分层架构](../_images/architecture/layer-architecture.png)

**通稿：**

> IAM 的进程外壳和业务分层是分开的。Process 负责配置、资源初始化、HTTP/gRPC 生命周期、后台任务和优雅关闭。Container 是 composition root，负责选择适配器、注入端口和导出 module capabilities，但它不执行业务用例。
>
> 请求进入以后，Transport 只处理 REST/gRPC 契约映射、认证上下文和错误映射；Application 编排用例、事务边界和跨模块端口；Domain 保留领域对象、业务规则和不变量；Infra 实现 MySQL、Redis、NSQ、Casbin、provider 和搜索索引等适配器。
>
> 依赖规则可以压缩成一句话：Transport 调用 Application，Application 执行 Domain 规则并依赖端口，Infra 实现端口，Container 在启动时完成选择和注入。
>
> 对外契约也不追求机械对称。REST 面向 Web、App 和管理端，gRPC 主要面向可信服务间调用，Go SDK 封装核心 gRPC、部分 AuthN REST 和 JWKS 验签。一个能力要真正成立，不能只看 OpenAPI 或 proto 文件，还要看运行时注册、handler/service、application capability、错误语义、SDK 和契约测试是否闭合。

**转场句：**

> 分层解决了代码边界，但它还不能单独证明系统正确。最后我想讲一下项目怎样区分不同强度的工程证据。

分层与装配见 [架构风格与设计原则](../00-概览/05-架构风格与设计原则.md)、[启动、生命周期与组合根](../01-运行时/01-启动与组合根.md)。

### 3.7 工程治理与结尾：每层证据只证明自己（约 2 分钟）

**一句话目标：** 不用“测试全绿”代替契约、装配、集成和生产证据。

**通稿：**

> 这个项目的另一个重点是证据分层。Domain 和 Application 测试保护业务规则；architecture test 防止 domain/application 反向依赖 infra 或 transport；OpenAPI、proto、route 和 registration test 防止契约与注册漂移；MySQL、Redis 和适配器集成测试保护事务、唯一性和原子操作；readiness、metrics 和 logs 才能反映运行时投影和后台任务状态；真正的发布结论还要绑定特定版本、环境和观测时间窗。
>
> 文档也使用同样的原则。`docs-hygiene` 检查链接、路径和历史术语，`docs-facts` 检查已编码的配置、迁移、事件、路由和模块装配事实。它们是必要门禁，但不能单独证明每一句业务描述或生产行为永远正确。
>
> 总结这个项目，我认为它的价值不在于有多少个登录和权限接口，而在于它为身份事实、认证证明、授权决策和派生投影建立了清晰边界，并且为事务、缓存、事件、多实例和发布证据保留了可验证的失败语义。

工程证据见 [测试契约与验收证据](../05-工程质量与运维/02-测试契约与验收证据.md)、[文档治理](../05-工程质量与运维/05-文档治理.md)。

## 4. 个人贡献段：需要根据真实职责定稿

> 下面三个版本不能全部直接使用。只保留与本人真实工作一致、能被代码、PR、测试或发布记录证明的表达。

### 4.1 如果主要负责架构与核心链路

> 我在这个项目中主要负责模块边界和核心链路的设计与落地。重点不是拆包本身，而是把 User、LoginIdentity、Session 和 Permission 等变化原因不同的事实分开，再用 application port 和 composition root 让它们在运行时协作。我同时对外部身份接入、Session/Token、AuthZ 策略传播等链路补充了事务和失败语义验证。

### 4.2 如果主要负责迁移与生产闭环

> 我在这个项目中主要推动遗留资产和数据结构的安全退役。我没有把“代码里没搜到”当成可删除证据，而是分开检查运行时装配、上下游调用、数据对账、备份恢复和发布后观察，再用 forward-only migration 分批完成退役。对每一批，我都把仓库门禁、CI 证据和特定环境验收分开记录。

### 4.3 如果主要负责工程治理

> 我在这个项目中主要补齐了架构、契约和文档的防漂移机制。包括用 architecture test 保护分层与模块边界，用 OpenAPI/proto/route/registration test 保护对外契约，用 `docs-hygiene` 和 `docs-facts` 检查链接、路径和可生成的关键事实。我会把这些门禁的结论限定在它们各自能证明的层次，不用一个绿色结果代替生产验收。

可公开的当前规模和交付结果统一见 [面试索引](README.md)，个人贡献必须在此基础上再按真实职责收窄。

## 5. 理解卡：对象怎样串成一条链

| 对象 | 它回答什么 | 生命周期 | 不能推导什么 |
| --- | --- | --- | --- |
| ExternalIdentity | IDP 已验证哪个 provider/realm/identifier | 一次请求级 provider proof | 不等于 LoginIdentity、IAM User 或登录态 |
| LoginIdentity | 某种 provider/realm/identifier 如何指向 User | 长期认证事实 | 不等于当前已认证 |
| User | IAM 内部这个人是谁 | 稳定身份事实 | 不等于登录凭据或权限 |
| Principal | 本次认证成功后的请求者是谁 | 一次认证结果 | 不等于拥有资源权限 |
| Session | 本次登录是否仍然有效、可撤销 | 一段登录期 | 不等于资源授权策略 |
| Access Token | 请求携带哪些签名声明 | 短期 | 本地验签不自动获得即时撤销语义 |
| Subject | AuthZ 中哪类主体请求授权 | 每次授权请求的主体引用 | 不等于 Principal 的全部认证细节 |
| Decision | 对当前 Resource/Action/ObjectAttributes 是否允许 | 单次判定 | 不能永久代表之后的策略 |

最简串联句：

> provider proof 先由 IDP 解析成请求级 ExternalIdentity，AuthN 再用它查找 LoginIdentity，LoginIdentity 指向稳定 User；一次成功认证产生 Principal 和 Session，Token 携带可验证声明；资源服务再以 User 为锚点构造 Subject，请求 AuthZ 对当前 Resource、Action 和受信对象属性做 Decision。

## 6. 理解卡：哪些是事实，哪些是投影

| 领域 | 权威事实 | 运行时/查询投影 | 重建或收敛方式 |
| --- | --- | --- | --- |
| Identity | MySQL 中的 User、Profile、ProfileLink | 业务查询 DTO，Suggest 不拥有主数据 | repository 从 MySQL 读取 |
| AuthN | LoginIdentity/Credential/JWKS metadata 与 Redis 中的在线状态 | JWT claims、JWKS 客户端快照 | 在线 Verify 或 JWKS refresh |
| AuthZ | MySQL Assignment/Inheritance/PermissionGrant/Resource Schema | 进程内不可变原生快照 | 完整构建后原子替换 |
| Suggest | Identity Profile facts 与当前可见性事实 | 进程内 Trie/Hash 索引 | Full/Delta refresh |
| 事件 | 业务表和 Outbox 发布意图 | MQ 消息只是协调信号 | relay 重试，consumer 幂等处理 |

判断投影是否越界只问三个问题：

1. 它能否从权威事实重建？
2. 它与权威事实冲突时，谁赢？
3. 它过旧时是偏向错误允许、错误拒绝，还是只影响搜索新鲜度？

## 7. 理解卡：一致性机制各自解决什么

| 机制 | 解决的问题 | 不解决的问题 |
| --- | --- | --- |
| MySQL transaction | 同库业务事实原子提交 | 数据库与 MQ 的共同原子性 |
| 唯一索引/条件更新 | 竞态下的唯一性和状态转换 | 跨存储事务 |
| Redis Lua/CAS | Session、Challenge、Refresh 等单存储内的原子状态操作 | MySQL 与 Redis 的分布式原子性 |
| Transactional Outbox | 业务提交与发布意图原子 | exactly-once 和消费者幂等 |
| PolicyVersion + reload | 观测并促进 AuthZ 投影收敛 | 全实例瞬时强一致 |
| Full/Delta refresh | Suggest 读模型重建和增量更新 | 与 Identity 同步提交 |

## 8. 高频追问与 30–45 秒答法

### 8.1 为什么 Identity、AuthN 和 AuthZ 不合并？

> 因为三者的事实、生命周期和安全语义不同。User 可以稳定存在，LoginIdentity 和 Credential 可以独立轮换或禁用，Session 可以撤销，授权策略又有自己的变更节奏。合并会让一个对象同时承担不相关的不变量，也会让认证与授权边界变得难以审计。

### 8.2 AuthN 和 AuthZ 到底是什么关系？

> 它们在请求链路上前后衔接，但不需要领域模块直接依赖。AuthN 验证请求者并产生可信 UserID/TenantID 上下文；资源服务以 Identity User 为锚点构造 AuthZ Subject，再对当前 Resource、Action 和受信对象属性做决策。它们通过稳定身份引用对齐，不互相拥有对方模型。

### 8.3 为什么不只用 JWT？

> JWT 适合携带可本地验签的签名声明，但签名正确不代表 Session 未撤销、User 仍可访问或 LoginIdentity 仍有效。所以项目同时保留短期 Access Token、Redis Session/Refresh 状态和在线 Verify。低风险高频路径可本地验签，要求即时撤销的高风险路径应使用在线验证或更短 TTL。

### 8.4 Access Token、Refresh Token 和 Session 各自是什么？

> Access Token 是短期访问声明，适合随请求传递并做签名验证；Refresh Token 是取得新 token pair 的高价值续期能力；Session 是服务端对整段登录期的在线状态。三者分开后，既能保留 JWT 本地验签的效率，又能通过 Session 和 Refresh 状态支持撤销与续期控制。

### 8.5 为什么 Casbin 只保留为角色图计算器？

> 最终授权不仅要解析角色，还要校验 Resource Schema、执行类型化 ConstraintSet、返回 matched Grant 和实际加载版本。MySQL 因此保存 Assignment、RoleInheritance、PermissionGrant 等管理事实，IAM 原生 runtime 执行权限判定；Casbin 只复用 domain-aware 角色继承图计算。事件也只是让快照 reload 的协调信号，不是策略真相。

### 8.6 有了 MQ，为什么还需要 Outbox？

> 数据库提交和 MQ publish 之间没有共同原子性。先提交数据库再发 MQ，进程可能在中间崩溃导致事件丢失；先发 MQ 再提交数据库，又可能让消费者看到未成立事实。Outbox 把发布意图与业务事实一起提交，再由 relay 恢复发布。它不保证 exactly-once，所以消费者仍必须幂等。

### 8.7 Revoke 怎样保证立即生效？

> 当前不能承诺多实例全球瞬时生效。写请求会在事务提交后直接 reload 当前实例，其他实例依赖 Outbox 事件重新加载。当前没有全实例 loaded-version barrier，因此高风险撤权需要明确这个最终一致窗口。如果业务要求返回成功即全局生效，就要引入 loaded/committed version 比较、实例回执或集中强一致判定，并承担延迟和可用性代价。

### 8.8 Suggest 已经过滤了可见范围，为什么还要 AuthZ？

> `ProfileAccessScope` 只是搜索所需的局部可见性投影，它只决定哪些脱敏候选可以进入当前搜索结果。搜到 Profile 不代表可以读取详情、修改或导出。后续操作需要自己的 Resource、Action 和必要的对象 Check，所以 Suggest 不能替代通用 AuthZ。

### 8.9 分层架构怎样防止模块变成大杂烩？

> 模块内部的 Transport、Application、Domain 和 Infra 职责分开，模块之间通过窄端口和 module capabilities 协作，具体适配器只在 Container 装配。Architecture test 用 Go import/AST 护栏防止 domain/application 反向依赖 infra 或 transport。它能防止一类结构退化，但不能单独证明运行时装配和业务语义正确。

### 8.10 测试全绿为什么不等于生产已验收？

> 不同测试只保护自己的证据层。单元测试不证明路由已注册，契约测试不证明 MySQL/Redis/MQ 实际连接，集成测试不证明生产使用了同一个 SHA 和配置。生产验收需要绑定发布版本、迁移、探针、可观测性、备份恢复和真实业务样本。

## 9. 别说过头：当前能力边界

| 不准确说法 | 当前可证明说法 |
| --- | --- |
| IDP 已统一支持各类 OAuth provider | 当前实现以微信生态 provider 为主，更通用的 provider 契约是演进方向 |
| `ExternalIdentity` 就是已绑定的 IAM 外部账户 | 它是 IDP 产出的请求级已验证 proof，不持久化，不拥有 User/LoginIdentity/Session；绑定关系仍由 AuthN LoginIdentity 表达 |
| AuthN 认证成功后直接调用 AuthZ | 请求通过 Identity User 锚点对齐认证上下文和 AuthZ Subject |
| JWT 签名正确就代表当前登录有效 | 本地 JWKS 验签不检查 Session、撤销标记和当前身份状态 |
| Casbin 是权限真相或完整执行引擎 | MySQL 授权事实是真相，IAM 原生快照执行权限规则，Casbin 只计算内存角色图 |
| Outbox 保证 exactly-once | Outbox 保证业务提交与发布意图原子，投递倾向 at-least-once |
| 授权变更在所有实例立即强一致 | 当前实例直接 reload，其他实例通过事件最终收敛，没有全实例 barrier |
| Suggest 搜到了就代表可读取详情 | Suggest 只返回当前查询范围内的脱敏候选 |
| 五个模块都有 REST、gRPC 和 SDK | Suggest 当前只有 REST，其他模块的 REST/gRPC 也非机械对称 |
| 仓库门禁通过就等于生产已验收 | 仓库、CI、部署版本、环境观测和真实流量是不同证据层 |

## 10. 压缩版本

### 10.1 90 秒版

> IAM 是一套面向业务系统的身份与访问管理服务，核心回答三个问题：用户是谁、如何证明当前是他、以及他能对某个资源做什么。
>
> 项目按变化原因拆成五个模块：Identity 管内部身份事实，AuthN 管登录证明、Session 和 Token，AuthZ 管资源授权决策；IDP 隔离外部 provider，Suggest 派生可见、脱敏的 Profile 搜索读模型。
>
> 工程上，MySQL 保存业务事实，Redis 承载在线状态，AuthZ 原生快照、JWKS 快照和 Suggest 索引都是可重建投影。AuthZ 策略通过事务、Outbox 和可选 NSQ 促进多实例收敛，语义是 at-least-once，不是 exactly-once。
>
> 它的价值不在于接口数量，而在于为身份、认证、授权和投影建立清晰的事实源、失败语义和验证边界。

### 10.2 5 分钟版取舍

5 分钟版只保留：

1. 3.1 开场；
2. 3.2 五个模块，每个只讲一句；
3. 3.3 外部身份链，用来串联 IDP、AuthN 和 Identity；
4. 3.4 只讲 AuthN 与 AuthZ 通过 Identity User 对齐，以及 MySQL/Casbin 的事实与投影边界；
5. 3.7 最后一段总结。

Suggest、Outbox 重复窗口、分层架构和工程门禁留给追问。

## 11. 排练检查表

- [ ] 不看文档，30 秒内说出“三问五模块”；
- [ ] 画出 provider proof -> ExternalIdentity -> LoginIdentity -> User -> Principal -> Session/Token 链路；
- [ ] 不画 AuthN -> AuthZ 模块直连，能说清 Identity User 锚点与请求上下文；
- [ ] 说清 MySQL 事实、Casbin 投影、Outbox 意图和 NSQ 通知的不同责任；
- [ ] 用“publish 成功、mark 失败”解释为什么不是 exactly-once；
- [ ] 说清 Revoke 传播延迟比 Grant 传播延迟更偏安全风险；
- [ ] 说清 Suggest 可见候选不等于详情授权；
- [ ] 能用一句话说出 Transport/Application/Domain/Infra/Container 依赖规则；
- [ ] 随机抽取第 8 节任意三题，45 秒内回答；
- [ ] 个人贡献段已删除不属于本人的表达，并且每个结果都能给出证据入口；
- [ ] 完整版控制在 13–17 分钟，90 秒版控制在 80–100 秒。

## 12. 事实导航

| 想追问的主题 | Canonical 入口 |
| --- | --- |
| 系统定位和模块边界 | [IAM 系统定位](../00-概览/01-IAM系统定位.md)、[模块划分与协作关系](../00-概览/02-模块划分与协作关系.md) |
| Identity/AuthN/AuthZ/IDP/Suggest 精确边界 | [Identity 跨模块边界](../02-业务模块/01-Identity/04-模块边界-Identity与AuthN-AuthZ-Suggest.md) |
| ExternalIdentity 模型、解析与 AuthN 映射链 | `internal/apiserver/domain/idp/externalidentity`、`internal/apiserver/application/idp/externalidentity`、`internal/apiserver/application/authn/externalidentity` |
| AuthN 模型、Session、Token 和 JWKS | [AuthN](../02-业务模块/02-AuthN/README.md) |
| AuthZ 模型、Casbin 和多实例传播 | [AuthZ](../02-业务模块/03-AuthZ/README.md) |
| IDP 凭据和 provider 协作 | [IDP](../02-业务模块/04-IDP/README.md) |
| Suggest 读模型 | [Suggest](../02-业务模块/05-Suggest/README.md) |
| 事务、缓存、Outbox 与一致性谱系 | [事务、缓存与事件一致性](../06-专题设计/02-事务缓存与事件一致性.md) |
| 分层架构与运行时装配 | [架构风格与设计原则](../00-概览/05-架构风格与设计原则.md)、[运行时](../01-运行时/README.md) |
| REST/gRPC/Go SDK 契约 | [接口与 SDK](../04-接口与SDK/README.md) |
| 架构、契约、迁移和生产证据 | [工程质量与运维](../05-工程质量与运维/README.md)、[验收记录](../01-运行时/08-IAM重构最终验收记录.md) |
