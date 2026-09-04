# IAM 面试索引

> 状态：已实现 · 本页只组织可公开表达，不拥有技术事实；所有结论必须回到 canonical 文档或机器契约核验。

现场展示使用 [IAM 系统演讲稿](02-IAM系统演讲稿.md)；讲述顺序、完整口语、转场、追问和背诵检查见 [IAM 系统演讲底稿](01-IAM系统演讲底稿.md)。二者都只是表达层，不取代本页回链的事实文档。

## 1. 90 秒项目介绍

IAM 是一套面向业务系统接入的身份与访问管理服务，核心回答三个问题：用户是谁、怎样证明身份、能够访问什么资源。项目按变化原因拆成五个模块：Identity 管理 User、Profile 和 ProfileLink；
AuthN 管理 LoginIdentity、Principal、Session、Token 与 JWKS；AuthZ 管理 Subject、Role、Assignment、RoleInheritance、PermissionGrant、
Resource Schema 和授权决策；IDP 隔离微信、企微等外部身份提供方；Suggest 从 Identity 与 AuthZ 事实派生脱敏搜索读模型。运行时由 process、
container 和 transport 分别负责生命周期、依赖装配与 REST/gRPC 暴露。工程重点不是接口数量，而是事务、Redis 原子操作、Transactional Outbox、
AuthZ/JWKS/Suggest 进程内投影，以及 readiness、迁移和生产证据共同构成的一致性与安全边界。详细定位见 [IAM 系统定位](../00-概览/01-IAM系统定位.md) 和
[跨模块统一模型](../00-概览/06-跨模块统一模型.md)。

## 2. 三个核心案例

| 案例 | 可讲主线 | Canonical 证据 |
| --- | --- | --- |
| 外部身份如何安全进入 IAM | IDP Resolver 把 provider code 解析成请求级 `ExternalIdentity`；AuthN 的 SignIn、SignUp、Linking 再按各自语义映射为认证输入或 LoginIdentity ProviderKey。该对象不持久化，也不拥有 User、LoginIdentity 或 Session | [注册登录与身份绑定](../02-业务模块/02-AuthN/02-注册登录与身份绑定.md)、[外部身份解析与 AuthN 协作](../02-业务模块/04-IDP/02-外部身份解析与AuthN协作.md) |
| 授权事实与多实例运行时怎样一致 | MySQL 保存 Assignment/RoleInheritance/PermissionGrant 等管理事实，IAM 原生不可变快照执行判定，自有角色图计算有效角色。写入提交后通过 PolicyVersion、Outbox 和事件通知其他实例 reload，并显式保留无请求级全实例 barrier 的边界 | [授权判定与不可变快照](../02-业务模块/03-AuthZ/02-关键链路-授权判定与不可变快照.md)、[授权写入与受管 Assignment](../02-业务模块/03-AuthZ/03-关键链路-授权写入与受管Assignment.md)、[多实例策略收敛](../02-业务模块/03-AuthZ/04-关键链路-多实例策略收敛.md) |
| Suggest 为什么是派生读模型 | Identity 保存 Profile 主数据，Suggest 通过 Full/Delta 构建进程内索引，按 `visibility.Scope` 过滤并返回脱敏候选；搜索可见不等于详情授权 | [Suggest 为什么是读模型](../06-专题设计/05-Suggest为什么是读模型.md)、[Suggest 模块](../02-业务模块/05-Suggest/README.md) |

## 3. 高频问题到 canonical 文档

| 高频问题 | 先读 |
| --- | --- |
| Identity、AuthN、AuthZ 为什么不能合并？ | [身份认证与授权边界](../06-专题设计/01-身份认证与授权边界.md) |
| Session、Access Token、Refresh Token 和本地 JWKS 验签各保证什么？ | [Session、Token 与 JWKS](../02-业务模块/02-AuthN/03-Session-Token与JWKS.md) |
| 为什么最终退役 Casbin？ | [从 Casbin 到自有不可变角色图](../06-专题设计/06-从Casbin到自有不可变角色图.md) |
| direct roles 与 effective roles 为什么不能混用？ | [AuthZ 领域模型设计](../02-业务模块/03-AuthZ/01-领域模型设计.md) |
| REST v3 与 gRPC v3 分别做什么？ | [REST 管理与路由授权](../02-业务模块/03-AuthZ/05-关键链路-REST管理与路由授权.md)、[gRPC 服务间授权与 SDK](../02-业务模块/03-AuthZ/06-关键链路-gRPC服务间授权与SDK.md) |
| `ReplaceManagedAssignments` 是否会覆盖用户全部角色？ | [授权写入与受管 Assignment](../02-业务模块/03-AuthZ/03-关键链路-授权写入与受管Assignment.md) |
| MySQL、Redis、Outbox 和进程内投影怎样选择一致性策略？ | [事务缓存与事件一致性](../06-专题设计/02-事务缓存与事件一致性.md) |
| Suggest 为什么不是 Identity Repository？ | [Suggest 为什么是读模型](../06-专题设计/05-Suggest为什么是读模型.md) |
| 进程存活、可以接流量和派生状态新鲜为什么是不同状态？ | [后台任务、Readiness 与优雅关闭](../01-运行时/03-后台任务就绪与优雅关闭.md) |
| REST、gRPC 和 Go SDK 怎样避免契约漂移？ | [REST、gRPC 与契约治理](../04-接口与SDK/01-REST-gRPC与契约治理.md) |
| 测试通过为什么不等于生产验收完成？ | [测试、契约与验收证据](../05-工程质量与运维/02-测试契约与验收证据.md) |

## 4. 可公开的规模、结果和个人贡献

| 维度 | 可公开口径 | 证据 |
| --- | --- | --- |
| 项目规模 | 五个业务模块；REST 与 gRPC 双协议；当前 proto 共 12 个 gRPC service；仓库 Schema 目标为 000029，最近一次生产验收为 000028，仓库门禁、迁移执行和运行健康需分层表述 | [模块划分](../00-概览/02-模块划分与协作关系.md)、[接口契约治理](../04-接口与SDK/01-REST-gRPC与契约治理.md)、[MySQL 事务与迁移](../03-基础设施/01-MySQL事务与迁移.md) |
| 已交付结果 | `000019–000028` 已分批完成生产验收，目标库为 15 张运行表加 `schema_migrations`；迁移、发布和健康证据分别记录，不用单一绿色门禁代替全链路结论 | [退役审计台账](../05-工程质量与运维/06-遗留资产兼容层与数据库退役审计.md)、[验收记录](../01-运行时/08-IAM重构最终验收记录.md) |
| 个人贡献：架构与实现 | 可陈述本人实际负责的模块边界、Identity/AuthN/AuthZ 核心链路、运行时装配和一致性设计；不要把团队或基础组件能力全部归为个人产出 | [统一模型](../00-概览/06-跨模块统一模型.md)、[启动与组合根](../01-运行时/01-启动与组合根.md) |
| 个人贡献：迁移与生产闭环 | 可陈述本人推动的遗留资产审计、数据对账、forward migration、MySQL 8 门禁、备份、分批发布和只读验收；具体批次和结果只引用公开台账 | [退役审计台账](../05-工程质量与运维/06-遗留资产兼容层与数据库退役审计.md) |
| 个人贡献：工程治理 | 可陈述本人建立或维护的架构测试、契约校验、`docs-hygiene`、`docs-facts` 和证据分层；不能把仓库门禁通过表述成线上行为自动正确 | [架构护栏](../05-工程质量与运维/01-架构护栏与分层验证.md)、[文档治理](../05-工程质量与运维/05-文档治理.md) |
