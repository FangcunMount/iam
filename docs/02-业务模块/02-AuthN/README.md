# AuthN：身份核验与登录态管理

> 状态：已实现 · 当前设计与实现。 本页是阅读与维护入口。正文中的代码事实不代表已完成部署或业务验收。

AuthN 维护 LoginIdentity 与 Credential，核验请求者对登录入口的控制，检查登录准入，再建立 Session、颁发访问和续期令牌。身份核验成功产生 Principal；四个环节全部完成才是登录成功。User/Profile 主数据属于 Identity，provider 配置与外部交换属于 IDP，资源动作授权属于 AuthZ。

```text
身份核验 → 登录准入 → 会话建立 → 令牌颁发
```

## 按问题阅读

| 问题 | 主文与维护归属 |
| --- | --- |
| 模块为什么存在、能力怎样分组 | [模块总览](00-模块总览.md) |
| 各对象是什么、事实来源和生命周期是什么 | [领域模型与身份核验策略](01-领域模型与认证策略.md) |
| 首次开通、User 修复、Credential ensure、三仓储事务 | [注册、登录与身份绑定](02-注册登录与身份绑定.md)，正文维护 SignUp |
| REST/gRPC 操作者来源、新入口证明、扫码失败窗口、锚点与最后 active、解绑后会话效力 | [Linking 主链路](03-关键链路-Linking登录身份绑定.md) |
| 方法/设备实际合同、锁定与在途竞争、OTP格式/消费、外部回退、补偿与重试 | [Login 主链路](04-关键链路-Login登录认证.md) |
| Session/Token对象校验、存储恢复、声明投影、寿命与兼容证据 | [Session、Token 与 JWKS](03-Session-Token与JWKS.md) |
| 初始颁发、Verify结果、Refresh寿命/回退/轮换、Logout与任务撤销的失败窗口 | [Token 主链路](05-关键链路-Token签发刷新吊销.md) |
| 密钥/PEM激活失败窗口、发布/缓存、未知kid、seed年龄、轮换/退役与恢复验收 | [JWKS 主链路](06-关键链路-JWKS与本地验签.md) |
| 跨模块传递什么、事务与同步/异步协作如何选择 | [模块边界](07-模块边界-AuthN与Identity-IDP-AuthZ.md) |
| 找代码、评估改动面、选择验证入口 | [分层与代码索引](08-分层架构与代码索引.md) |

第一次阅读按总览 → 领域模型 → 目标用例进入；排查刷新、撤销或密钥问题直接进入相应链路。保留既有文件名及两个 `03`，以标题定位主题。

## 维护规则

同一规则由表中主文讲透，其他页只保留理解上下文所需的摘要并回链。领域模型页定义对象，不维护第二套完整登录时序；Session 模型页定义状态、投影和寿命，操作顺序由 Token 主链路维护。新能力、替代方案及历史迁移必须分别标明。

当前应保持的边界是：应用 SignIn 协调 Admission、SessionCreator 与 InitialTokenIssuer 并负责新会话补偿；领域 token 能力负责 mint、验证、刷新、撤销规则。用户 access JWT 绑定 Session；服务调用使用 mTLS + ACL。本地 JWKS 验签不具备在线撤销语义。管理路由在单一授权空间检查明确的 Resource/Action，不按角色名旁路。

跨模块入口：[Identity](../01-Identity/README.md)、[IDP](../04-IDP/README.md)、[AuthZ](../03-AuthZ/README.md)。共享机制见 [Redis](../../03-基础设施/02-Redis与缓存一致性.md)、[密码学](../../03-基础设施/04-密码学密钥与令牌.md)；协议接入见 [接口与 SDK](../../04-接口与SDK/README.md)。

## 证据与验证

源码入口为 `internal/apiserver/{domain,application,container}/authn`、`internal/apiserver/transport/{rest,grpc}`、`pkg/sdk/auth`。具体函数与测试由各主文维护。

```bash
make docs-hygiene docs-facts
go test ./internal/apiserver/domain/authn/... ./internal/apiserver/application/authn/... ./pkg/sdk/auth/...
```

检查命令是验证入口，不是本页已经执行的结果；MySQL/Redis、消费者缓存、多实例与实际环境证据分别报告。
