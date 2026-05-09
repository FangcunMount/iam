# IAM 文档中心

## 本文档中心定位

`docs/` 是 IAM 项目的解释层，负责说明系统边界、运行链路、设计取舍、业务模型、接入方式、架构护栏和对外讲解材料。

它不是机器契约本身：

- REST 路径、字段、schema、错误响应以 [`../api/rest`](../api/rest) 为准。
- gRPC service、message、RPC 以 [`../api/grpc`](../api/grpc) 为准。
- SDK 公开 API 以 [`../pkg/sdk`](../pkg/sdk) 为准。
- 运行行为以源码和测试为准。
- `_archive/` 只保存历史材料，不作为当前事实源。

文档的职责是解释：

```text
为什么这样分层
为什么这样装配
为什么这样建模
如何阅读源码
如何接入系统
如何防止架构和文档漂移
如何对外讲清楚这个项目
```

---

## 30 秒结论

IAM 是一个面向业务系统接入的身份与访问管理服务，不是普通用户中心、登录系统、权限系统或微信登录模块。

新版文档体系按下面顺序建立心智模型：

```text
00-概览
  -> 01-运行时
  -> 02-认证 AuthN
  -> 03-授权 AuthZ
  -> 04-身份 Identity
  -> 05-接入与契约
  -> 06-架构护栏
  -> 07-专题分析
  -> 08-宣讲
```

其中：

| 层次 | 作用 |
| --- | --- |
| `00-概览` | 建立系统地图，说明 IAM 是什么、如何分层、如何从 Go 进程装配成服务 |
| `01-运行时` | 解释服务如何启动、装配 REST/gRPC、运行后台任务、优雅关闭和降级诊断 |
| `02-认证AuthN` | 解释登录、账号、Session、Access Token、Refresh Token、JWKS、KeyRotation、Verify |
| `03-授权AuthZ` | 解释 Role、Resource、Permission、RoleBinding、Check、PolicyVersion、Outbox |
| `04-身份Identity` | 解释 User、Profile、ProfileLink 及当前用户档案访问边界 |
| `05-接入与契约` | 解释 REST、gRPC、SDK 三类接入方式及机器契约事实源 |
| `06-架构护栏` | 解释架构测试、契约测试、SDK compile test、docs-hygiene 如何防漂移 |
| `07-专题分析` | 解释关键设计取舍：为什么这样做、替代方案、收益代价和不变量 |
| `08-宣讲` | 准备技术分享、面试表达、架构图和追问证据链 |

---

## 当前文档目录

```text
docs/
├── README.md
│
├── 00-概览/
│   ├── README.md
│   └── 01-系统架构总览.md
│
├── 01-运行时/
│   ├── README.md
│   ├── 01-服务入口与生命周期装配.md
│   ├── 02-Transport装配--REST路由与gRPC服务注册.md
│   ├── 03-配置与运行模式.md
│   ├── 04-后台任务与优雅关闭.md
│   └── 05-降级启动与健康检查.md
│
├── 02-认证AuthN/
│   ├── README.md
│   ├── 01-登录链路-从Login请求到Session与Token.md
│   ├── 02-认证语义-用户状态&会话&Token边界.md
│   ├── 03-JWKS与KeyRotation.md
│   └── 04-第三方登录与IDP协作.md
│
├── 03-授权AuthZ/
│   ├── README.md
│   ├── 01-授权模型-Role&Resource&Permission&RoleBinding.md
│   ├── 02-授权判定链路-从Check到Casbin.md
│   ├── 03-PolicyChangeCommitter与UoW.md
│   └── 04-授权版本事件与Outbox.md
│
├── 04-身份Identity/
│   ├── README.md
│   ├── 01-User与Profile模型.md
│   └── 02-ProfileLink链路-用户与用户档案关系协作.md
│
├── 05-接入与契约/
│   ├── README.md
│   ├── 01-REST API契约.md
│   ├── 02-gRPC API契约.md
│   └── 03-SDK接入模型.md
│
├── 06-架构护栏/
│   ├── README.md
│   ├── 01-架构测试与依赖边界.md
│   └── 02-文档事实源与防漂移机制.md
│
├── 07-专题分析/
│   ├── README.md
│   ├── 01-为什么IAM不是普通用户中心.md
│   ├── 02-为什么拆分AuthN-AuthZ-Identity-IDP.md
│   ├── 03-为什么AuthN需要Session与RefreshToken.md
│   ├── 04-为什么JWKS与在线Verify要并存.md
│   ├── 05-为什么AuthZ写入不是简单CRUD.md
│   ├── 06-为什么RoleBinding与Assignment要分层.md
│   ├── 07-为什么ProfileLink不能只是User字段.md
│   ├── 08-为什么IDP只做身份源基础设施.md
│   ├── 09-为什么需要TransactionalOutbox传播授权版本.md
│   ├── 10-为什么SDK是接入产品层而不是业务层.md
│   └── 11-系统演进路线.md
│
├── 08-宣讲/
│   ├── README.md
│   ├── 00-项目一句话定位.md
│   ├── 01-业务背景与问题.md
│   ├── 02-系统架构讲法.md
│   ├── 03-AuthN认证体系讲法.md
│   ├── 04-AuthZ授权体系讲法.md
│   ├── 05-Identity与ProfileLink讲法.md
│   ├── 06-IDP与第三方登录讲法.md
│   ├── 07-JWKS与Token安全讲法.md
│   ├── 08-Outbox与授权版本传播讲法.md
│   ├── 09-REST-gRPC-SDK接入讲法.md
│   ├── 10-工程质量与架构护栏讲法.md
│   ├── 11-30分钟技术分享脚本.md
│   ├── 12-架构图素材索引.md
│   └── 13-面试追问证据索引.md
│
└── _archive/
    └── README.md
```

> 注意：旧的 `02-业务域/`、`03-接口与集成/`、`04-基础设施与运维/`、旧 `05-专题分析/` 不再作为新版 active 文档入口。历史材料如需保留，应进入 `_archive/`。

---

## 快速导航

| 你想回答的问题 | 推荐阅读 |
| --- | --- |
| IAM 是什么，为什么不是普通用户中心 | [00-概览/README.md](00-概览/README.md)、[00-概览/01-系统架构总览.md](00-概览/01-系统架构总览.md)、[07-专题分析/01-为什么IAM不是普通用户中心.md](07-专题分析/01-为什么IAM不是普通用户中心.md) |
| 服务如何从入口启动到 REST/gRPC | [01-运行时/README.md](01-运行时/README.md)、[01-运行时/01-服务入口与生命周期装配.md](01-运行时/01-服务入口与生命周期装配.md) |
| REST/gRPC 如何从 container 获取能力 | [01-运行时/02-Transport装配--REST路由与gRPC服务注册.md](01-运行时/02-Transport装配--REST路由与gRPC服务注册.md) |
| 配置、运行模式和 degraded startup 如何理解 | [01-运行时/03-配置与运行模式.md](01-运行时/03-配置与运行模式.md)、[01-运行时/05-降级启动与健康检查.md](01-运行时/05-降级启动与健康检查.md) |
| 后台任务和 graceful shutdown 如何运行 | [01-运行时/04-后台任务与优雅关闭.md](01-运行时/04-后台任务与优雅关闭.md) |
| 登录如何变成 Session、Access Token、Refresh Token | [02-认证AuthN/README.md](02-认证AuthN/README.md)、[02-认证AuthN/01-登录链路-从Login请求到Session与Token.md](02-认证AuthN/01-登录链路-从Login请求到Session与Token.md) |
| Token、Session、用户状态边界是什么 | [02-认证AuthN/02-认证语义-用户状态&会话&Token边界.md](02-认证AuthN/02-认证语义-用户状态&会话&Token边界.md) |
| JWKS、KeyRotation、在线 Verify 如何工作 | [02-认证AuthN/03-JWKS与KeyRotation.md](02-认证AuthN/03-JWKS与KeyRotation.md)、[07-专题分析/04-为什么JWKS与在线Verify要并存.md](07-专题分析/04-为什么JWKS与在线Verify要并存.md) |
| 微信/企微登录和 IDP 如何协作 | [02-认证AuthN/04-第三方登录与IDP协作.md](02-认证AuthN/04-第三方登录与IDP协作.md)、[07-专题分析/08-为什么IDP只做身份源基础设施.md](07-专题分析/08-为什么IDP只做身份源基础设施.md) |
| 授权模型如何组织 | [03-授权AuthZ/README.md](03-授权AuthZ/README.md)、[03-授权AuthZ/01-授权模型-Role&Resource&Permission&RoleBinding.md](03-授权AuthZ/01-授权模型-Role&Resource&Permission&RoleBinding.md) |
| 一次授权判定如何走到 Casbin | [03-授权AuthZ/02-授权判定链路-从Check到Casbin.md](03-授权AuthZ/02-授权判定链路-从Check到Casbin.md) |
| 授权写入为什么不是简单 CRUD | [03-授权AuthZ/03-PolicyChangeCommitter与UoW.md](03-授权AuthZ/03-PolicyChangeCommitter与UoW.md)、[07-专题分析/05-为什么AuthZ写入不是简单CRUD.md](07-专题分析/05-为什么AuthZ写入不是简单CRUD.md) |
| 授权版本事件和 Outbox 如何传播 | [03-授权AuthZ/04-授权版本事件与Outbox.md](03-授权AuthZ/04-授权版本事件与Outbox.md)、[07-专题分析/09-为什么需要TransactionalOutbox传播授权版本.md](07-专题分析/09-为什么需要TransactionalOutbox传播授权版本.md) |
| User、Profile、ProfileLink 如何建模 | [04-身份Identity/README.md](04-身份Identity/README.md)、[04-身份Identity/01-User与Profile模型.md](04-身份Identity/01-User与Profile模型.md)、[04-身份Identity/02-ProfileLink链路-用户与用户档案关系协作.md](04-身份Identity/02-ProfileLink链路-用户与用户档案关系协作.md) |
| REST/gRPC/SDK 如何接入 | [05-接入与契约/README.md](05-接入与契约/README.md)、[05-接入与契约/01-REST API契约.md](05-接入与契约/01-REST API契约.md)、[05-接入与契约/02-gRPC API契约.md](05-接入与契约/02-gRPC API契约.md)、[05-接入与契约/03-SDK接入模型.md](05-接入与契约/03-SDK接入模型.md) |
| 架构边界如何被测试保护 | [06-架构护栏/README.md](06-架构护栏/README.md)、[06-架构护栏/01-架构测试与依赖边界.md](06-架构护栏/01-架构测试与依赖边界.md) |
| 文档事实源和防漂移机制是什么 | [06-架构护栏/02-文档事实源与防漂移机制.md](06-架构护栏/02-文档事实源与防漂移机制.md) |
| 为什么这样设计，而不是更简单方案 | [07-专题分析/README.md](07-专题分析/README.md) |
| 如何对外讲解、面试准备、技术分享 | [08-宣讲/README.md](08-宣讲/README.md)、[08-宣讲/11-30分钟技术分享脚本.md](08-宣讲/11-30分钟技术分享脚本.md)、[08-宣讲/13-面试追问证据索引.md](08-宣讲/13-面试追问证据索引.md) |

---

## 分层说明

### 00-概览

回答：

```text
这个系统是什么？
整体架构怎么分层？
新读者从哪里开始？
```

重点说明：

- IAM 的系统定位；
- 运行时外部关系；
- 代码分层；
- 核心模块；
- 读者路径；
- 事实源优先级。

---

### 01-运行时

回答：

```text
这个服务如何启动、装配、诊断和关闭？
```

重点说明：

- `cmd/apiserver -> app -> config -> process.Run`；
- `PrepareRun()` stage pipeline；
- MySQL、Redis、IDP encryption key、EventBus 资源准备；
- container bootstrap；
- REST/gRPC transport registration；
- background tasks；
- graceful shutdown；
- degraded startup 与 health/debug 边界。

---

### 02-认证 AuthN

回答：

```text
用户如何登录，登录态如何被管理和验证？
```

重点说明：

- 登录入口；
- LoginRequest；
- MethodRegistry / LoginMethod；
- ProofFactory / AuthCredential；
- Authenticator / AuthStrategy；
- Principal；
- SessionTokenIssuer / SessionTokenPairIssuer；
- TokenApplicationService；
- Reauthenticate；
- Session；
- Refresh Token；
- Access Token；
- JWKS；
- KeyRotation；
- Online Verify；
- IDP / WeChat / WeCom 协作。

---

### 03-授权 AuthZ

回答：

```text
用户能不能访问某个资源？
```

重点说明：

- Subject；
- Tenant；
- Role；
- Resource；
- Permission；
- Scope；
- RoleBinding；
- assignment wire term 与 rolebinding internal term；
- AuthorizationRequest；
- AuthorizationDecision；
- Casbin runtime engine；
- PolicyChange；
- PolicyChangeCommitter；
- Unit of Work；
- PolicyVersion；
- Transactional Outbox。

---

### 04-身份 Identity

回答：

```text
User、Profile、ProfileLink 如何表达业务身份关系？
```

重点说明：

- User 是登录主体；
- Profile 是业务档案；
- ProfileLink 是 User 与 Profile 的关系；
- self profile/link 是基础不变量；
- MyProfiles / MyProfileLinks 是当前用户视角的 guard；
- ProfileLink 不等于 AuthZ Permission；
- Suggest 只提供候选；
- AuthZ 才负责资源权限判定。

---

### 05-接入与契约

回答：

```text
外部系统如何接入 IAM？
```

重点说明：

- REST 适合 Web、App、管理后台和登录；
- gRPC 适合可信服务间调用；
- SDK 适合 Go 业务服务低成本接入；
- JWT 如何传递；
- JWKS 何时用于离线验签；
- 在线 Verify 何时必须使用；
- AuthZ Check 如何接入；
- public / protected / admin / debug 能力边界；
- OpenAPI / proto / SDK public API 是各自事实源。

---

### 06-架构护栏

回答：

```text
架构为什么不会轻易回退？
```

重点说明：

- domain 不依赖 infra/database；
- application 不依赖 transport；
- REST router 不依赖 container/global config；
- assembler 使用 typed deps；
- AuthZ 内部统一 rolebinding；
- Casbin facts 不进入 domain/transport；
- REST/gRPC/SDK 契约测试；
- docs-hygiene；
- `_archive` 不作为当前事实源。

---

### 07-专题分析

回答：

```text
为什么这样设计？
```

重点说明：

- IAM 为什么不是普通用户中心；
- 为什么拆 AuthN/AuthZ/Identity/IDP；
- 为什么 AuthN 需要 Session/RefreshToken；
- 为什么 JWKS 与在线 Verify 并存；
- 为什么 AuthZ 写入不是 CRUD；
- 为什么 RoleBinding 与 Assignment 分层；
- 为什么 ProfileLink 不能只是 User 字段；
- 为什么 IDP 只做身份源基础设施；
- 为什么需要 Transactional Outbox；
- 为什么 SDK 是接入产品层；
- 系统演进路线。

---

### 08-宣讲

回答：

```text
如何对外讲清楚 IAM？
```

重点说明：

- 项目一句话定位；
- 业务背景；
- 系统架构讲法；
- AuthN / AuthZ / Identity / IDP 讲法；
- JWKS / Token 安全讲法；
- Outbox 讲法；
- REST/gRPC/SDK 讲法；
- 工程质量讲法；
- 30 分钟技术分享脚本；
- 架构图素材；
- 面试追问证据索引。

---

## 推荐阅读路径

### 路径一：第一次了解 IAM

```text
00-概览/README.md
  -> 00-概览/01-系统架构总览.md
  -> 08-宣讲/00-项目一句话定位.md
  -> 08-宣讲/01-业务背景与问题.md
```

目标：

```text
先知道 IAM 是什么、为什么需要它、整体如何分层。
```

---

### 路径二：后端开发读源码

```text
00-概览/01-系统架构总览.md
  -> 01-运行时/README.md
  -> 01-运行时/01-服务入口与生命周期装配.md
  -> 01-运行时/02-Transport装配--REST路由与gRPC服务注册.md
  -> 02-认证AuthN/README.md
  -> 03-授权AuthZ/README.md
  -> 04-身份Identity/README.md
```

目标：

```text
先看运行时装配，再看核心业务域。
```

---

### 路径三：接入方阅读

```text
05-接入与契约/README.md
  -> 05-接入与契约/01-REST API契约.md
  -> 05-接入与契约/02-gRPC API契约.md
  -> 05-接入与契约/03-SDK接入模型.md
  -> ../api/rest/README.md
  -> ../api/grpc/README.md
  -> ../pkg/sdk/README.md
```

目标：

```text
知道什么时候用 REST、什么时候用 gRPC、什么时候用 SDK。
```

---

### 路径四：面试准备

```text
08-宣讲/README.md
  -> 08-宣讲/00-项目一句话定位.md
  -> 08-宣讲/02-系统架构讲法.md
  -> 08-宣讲/03-AuthN认证体系讲法.md
  -> 08-宣讲/04-AuthZ授权体系讲法.md
  -> 08-宣讲/05-Identity与ProfileLink讲法.md
  -> 08-宣讲/13-面试追问证据索引.md
```

目标：

```text
把项目讲清楚，并能回答追问。
```

---

### 路径五：架构评审

```text
00-概览/01-系统架构总览.md
  -> 06-架构护栏/README.md
  -> 06-架构护栏/01-架构测试与依赖边界.md
  -> 06-架构护栏/02-文档事实源与防漂移机制.md
  -> 07-专题分析/02-为什么拆分AuthN-AuthZ-Identity-IDP.md
  -> 07-专题分析/11-系统演进路线.md
```

目标：

```text
看清模块边界、依赖方向、契约防漂移和后续演进路线。
```

---

## 事实源优先级

文档中出现事实冲突时，按以下优先级判断：

1. **源码与运行时行为**  
   `cmd/`、`internal/apiserver/`、`internal/pkg/`、`pkg/`。

2. **机器契约、配置和迁移**  
   `api/rest`、`api/grpc`、`configs`、`internal/pkg/migration/migrations`。

3. **测试与架构护栏**  
   `internal/pkg/architecture`、REST/gRPC transport tests、SDK public API compile tests。

4. **当前维护文档**  
   `docs/00-概览` 到 `docs/08-宣讲`。

5. **历史文档与归档材料**  
   `_archive/` 只用于历史追溯，不作为当前事实源。

---

## 核心术语

| 术语 | 当前约定 |
| --- | --- |
| IAM | Identity and Access Management 服务，不等同于单纯用户管理后台 |
| AuthN | Authentication，负责登录、账号、Session、Token、JWKS |
| AuthZ | Authorization，负责 Role、Resource、Permission、RoleBinding、Check |
| Identity | 身份主体与业务档案关系，负责 User、Profile、ProfileLink |
| IDP | 第三方身份源基础设施，负责 WechatApp、SecretVault、外部 API |
| User | 登录主体，IAM 内部身份锚点 |
| Profile | 业务档案，例如本人档案、儿童档案、被测评者档案 |
| ProfileLink | User 与 Profile 的关系事实 |
| assignment | REST/proto 对外 wire term，表示角色分配 |
| rolebinding | 内部 application/domain 标准术语，表示 subject 与 role 的绑定 |
| Session | 在线登录态锚点 |
| Access Token | 短期访问凭证，当前实现为 JWT |
| Refresh Token | 服务端可控的续期凭证 |
| JWKS | 公钥发布机制，用于业务服务本地验签 |
| Online Verify | 在线认证状态判断，用于检查 revoked/session/user/account 状态 |
| PolicyVersion | tenant 级授权事实版本 |
| Transactional Outbox | 业务事实和事件记录同事务提交，再由 relay 异步发布 |
| process | 生命周期编排层 |
| container | 组合根，不处理请求，不写业务规则 |
| transport | REST/gRPC 协议适配层 |
| application | 用例编排层 |
| domain | 领域规则层 |
| infra | MySQL、Redis、Casbin、JWT、Outbox、WeChat API 等外部资源适配层 |
| SDK | Go 服务端接入产品层，不是业务层 |
| docs-hygiene | 活跃文档断链和退役事实引用检查 |
| `_archive` | 历史材料区，不作为当前事实源 |

---

## 代码与契约入口

| 主题 | 入口 |
| --- | --- |
| 根项目说明 | [`../README.md`](../README.md) |
| 进程入口 | [`../cmd/apiserver/apiserver.go`](../cmd/apiserver/apiserver.go) |
| 运行时生命周期 | [`../internal/apiserver/process`](../internal/apiserver/process) |
| 组合根 | [`../internal/apiserver/container`](../internal/apiserver/container) |
| REST transport | [`../internal/apiserver/transport/rest`](../internal/apiserver/transport/rest) |
| gRPC transport | [`../internal/apiserver/transport/grpc`](../internal/apiserver/transport/grpc) |
| 应用层 | [`../internal/apiserver/application`](../internal/apiserver/application) |
| 领域层 | [`../internal/apiserver/domain`](../internal/apiserver/domain) |
| 基础设施层 | [`../internal/apiserver/infra`](../internal/apiserver/infra) |
| REST 契约 | [`../api/rest`](../api/rest) |
| gRPC 契约 | [`../api/grpc`](../api/grpc) |
| SDK | [`../pkg/sdk`](../pkg/sdk) |
| 架构测试 | [`../internal/pkg/architecture`](../internal/pkg/architecture) |
| 文档检查脚本 | [`../scripts/check-docs-links.py`](../scripts/check-docs-links.py) |

---

## 文档维护规则

### 1. 不把文档写成源码转述

文档不是逐行解释代码。  
文档应该解释：

```text
为什么这样分层
为什么这样装配
为什么这样建模
失败边界在哪里
如何继续读源码
```

---

### 2. 不把 API 文档重复成字段清单

字段、路径、RPC 以 OpenAPI/proto/SDK 为准。  
`docs/` 只解释接入方式、语义边界和设计取舍。

---

### 3. 不从 `_archive` 复制当前事实

`_archive/` 可以用于历史追溯，但不能作为当前架构、当前代码、当前接口事实源。

---

### 4. 每篇文档必须能回链证据

文档中出现关键判断，应该能指向：

```text
源码路径
API 契约
配置文件
测试文件
当前事实层文档
```

---

### 5. 术语必须统一

尤其注意：

```text
ProfileLink 不要退回旧关系术语
内部 AuthZ 不要退回 assignment 包
Casbin 不要被写成业务语言
container 不要被写成 service 层
process 不要被写成普通 router/server 包
SDK 不要被写成业务层
IDP 不要被写成登录态所有者
```

---

### 6. 不恢复旧 active 目录体系

新版 active 文档入口是：

```text
00-概览
01-运行时
02-认证AuthN
03-授权AuthZ
04-身份Identity
05-接入与契约
06-架构护栏
07-专题分析
08-宣讲
_archive
```

不要恢复旧入口：

```text
02-业务域
03-接口与集成
04-基础设施与运维
05-专题分析
```

历史材料应归档到 `_archive/`。

---

## 发布检查清单

文档发布前至少检查：

```text
1. docs/README.md 能作为唯一总入口
2. 每个 active 目录都有 README.md
3. 每篇正文都有“本文回答”
4. 每篇正文都有“30 秒结论”
5. 每篇正文能回链源码、契约或测试
6. Mermaid 图能解释核心链路
7. 没有把旧目录作为 active 入口
8. 没有把 _archive 当成当前事实源
9. 术语统一：ProfileLink / RoleBinding / Assignment / Session / Outbox / SDK
10. OpenAPI / proto / SDK README 不互相矛盾
11. make docs-hygiene 通过
```

---

## 验证命令

基础文档卫生检查：

```bash
make docs-hygiene
```

架构边界与运行时装配检查：

```bash
go test ./internal/pkg/architecture \
  ./internal/apiserver/process \
  ./internal/apiserver/container \
  ./internal/apiserver/transport/rest \
  ./internal/apiserver/transport/grpc
```

REST 契约检查：

```bash
make docs-swagger
make api-validate
go test ./internal/apiserver/transport/rest
```

gRPC 契约检查：

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc
```

SDK 公开面检查：

```bash
go test ./pkg/sdk/...
```

业务链路按需检查：

```bash
go test ./internal/apiserver/application/authn/... \
  ./internal/apiserver/application/authz/... \
  ./internal/apiserver/application/identity/... \
  ./internal/apiserver/application/idp/...
```

---

## 本文总结

`docs/` 是 IAM 的解释层，不是机器契约本身。

新版文档体系的核心分工是：

```text
00-概览：建立系统地图
01-运行时：解释服务如何装配运行
02-认证AuthN：解释登录态和 token 生命周期
03-授权AuthZ：解释资源授权和版本传播
04-身份Identity：解释 User/Profile/ProfileLink
05-接入与契约：解释 REST/gRPC/SDK
06-架构护栏：解释如何防止边界和文档漂移
07-专题分析：解释为什么这样设计
08-宣讲：解释如何对外讲清楚
```

如果只记一句话：

> **这套文档的目标不是覆盖每一个源码文件，而是让读者能准确理解 IAM 的系统边界、核心链路、设计取舍、接入方式、架构护栏和对外表达方式。**
