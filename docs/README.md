# IAM 文档中心

## 1. 文档中心定位

`docs/` 是 IAM 项目的解释层。

它负责说明：

```text
系统边界；
运行链路；
领域模型；
设计取舍；
接入方式；
契约事实源；
架构护栏；
对外表达材料。
```

它不是机器契约本身。

机器契约与运行事实以这些内容为准：

- REST 路径、字段、schema、错误响应以 [`../api/rest`](../api/rest) 为准。
- gRPC service、message、RPC 以 [`../api/grpc`](../api/grpc) 为准。
- Go SDK 公开 API 以 [`../pkg/sdk`](../pkg/sdk) 为准。
- 运行行为以源码和测试为准。
- `_archive/` 只保存历史材料，不作为当前事实源。

文档的职责不是逐行转述源码，而是帮助读者回答：

```text
IAM 是什么；
为什么这样分层；
为什么这样建模；
服务如何启动和装配；
AuthN / AuthZ / Identity / IDP 如何协作；
业务系统如何接入 IAM；
如何防止代码、契约和文档漂移；
如何对外讲清楚这个项目。
```

---

## 2. 30 秒结论

IAM 是一个面向业务系统接入的身份与访问管理服务。

它不是普通用户中心、登录系统、权限 CRUD 系统或微信登录模块。

当前文档体系按下面顺序建立心智模型：

```text
00-概览
  -> 01-运行时
  -> 02-认证AuthN
  -> 03-授权AuthZ
  -> 04-身份Identity
  -> 05-接入与契约
  -> 06-架构护栏
  -> 07-宣讲
  -> _archive
```

其中：

| 目录 | 作用 |
| --- | --- |
| `00-概览` | 建立系统总览，说明 IAM 是什么、如何分层、核心模块如何协作 |
| `01-运行时` | 解释服务如何启动、装配 REST/gRPC、运行后台任务、优雅关闭和资源释放 |
| `02-认证AuthN` | 解释 User、LoginIdentity、Credential、Challenge、Principal、Session、Token、JWKS、Verify、IDP 协作 |
| `03-授权AuthZ` | 解释 Subject、Role、Resource、Permission、RoleBinding、Check、PolicyVersion、Outbox、Casbin runtime |
| `04-身份Identity` | 解释 User、Profile、ProfileLink 及当前用户档案访问边界 |
| `05-接入与契约` | 解释 REST、gRPC、SDK 三类接入方式、qs-server 接入链路和契约防漂移 |
| `06-架构护栏` | 解释架构测试、契约测试、SDK compile test、docs-hygiene 如何防漂移 |
| `07-宣讲` | 准备技术分享、面试表达、架构图和追问证据链 |
| `_archive` | 保存历史材料，不作为当前事实源 |

---

## 3. 当前文档目录

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
│   ├── 00-三进程协作总览.md
│   ├── 01-qs-apiserver启动与组合根.md
│   ├── 02-collection-server运行时.md
│   ├── 03-qs-worker运行时.md
│   ├── 04-进程间调用与gRPC.md
│   ├── 05-IAM认证与身份链路.md
│   ├── 06-后台任务与调度.md
│   └── 07-优雅关闭与资源释放.md
│
├── 02-认证AuthN/
│   ├── README.md
│   ├── 00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md
│   ├── 01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
│   ├── 02-Linking链路-登录身份绑定解绑与安全边界.md
│   ├── 03-Login链路-从登录请求到Principal.md
│   ├── 04-Token链路-从Principal到AccessToken与RefreshToken.md
│   ├── 05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md
│   ├── 06-JWT-JWS-JWK-JWKS边界与KeyRotation.md
│   ├── 07-第三方登录与IDP协作-WeChat-WeCom.md
│   └── 08-AuthN分层架构与事实源索引.md
│
├── 03-授权AuthZ/
│   ├── README.md
│   ├── 00-AuthZ模型总览-Subject-Role-Resource-Permission-RoleBinding.md
│   ├── 01-资源模型-ResourceKey-ResourcePattern-Action-Scope.md
│   ├── 02-角色模型-Role-RoleBinding-Subject.md
│   ├── 03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md
│   ├── 04-授权版本与事件传播链路-PolicyVersion-Outbox-RuntimeReload.md
│   ├── 05-权限检查链路-Check-Snapshot.md
│   ├── 06-Casbin运行时模型-pgFacts与四段Matcher.md
│   └── 07-AuthZ分层架构与事实源索引.md
│
├── 04-身份Identity/
│   ├── README.md
│   ├── 01-User与Profile模型.md
│   └── 02-ProfileLink链路--用户与儿童档案关系协作.md
│
├── 05-接入与契约/
│   ├── README.md
│   ├── 00-接入总览-业务系统如何接入IAM.md
│   ├── 01-REST API契约-前端与管理端接入.md
│   ├── 02-gRPC API契约-服务间调用与内部集成.md
│   ├── 03-SDK接入模型-Go服务端集成.md
│   ├── 04-业务系统接入链路-qs-server接入 IAM 详解.md
│   └── 05-契约事实源与防漂移机制.md
│
├── 06-架构护栏/
│   ├── README.md
│   ├── 01-架构测试与依赖边界.md
│   └── 02-文档事实源与防漂移机制.md
│
├── 07-宣讲/
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

> 注意：`07-专题分析/` 已归档，`08-宣讲/` 已调整为当前的 `07-宣讲/`。旧的 `02-业务域/`、`03-接口与集成/`、`04-基础设施与运维/`、旧 `05-专题分析/` 不再作为新版 active 文档入口。

---

## 4. 快速导航

| 你想回答的问题 | 推荐阅读 |
| --- | --- |
| IAM 是什么，为什么不是普通用户中心 | [00-概览/README.md](00-概览/README.md)、[00-概览/01-系统架构总览.md](00-概览/01-系统架构总览.md)、[07-宣讲/00-项目一句话定位.md](07-宣讲/00-项目一句话定位.md) |
| 服务如何启动、装配和关闭 | [01-运行时/README.md](01-运行时/README.md)、[01-运行时/01-qs-apiserver启动与组合根.md](01-运行时/01-qs-apiserver启动与组合根.md)、[01-运行时/07-优雅关闭与资源释放.md](01-运行时/07-优雅关闭与资源释放.md) |
| REST/gRPC 如何装配 | [01-运行时/04-进程间调用与gRPC.md](01-运行时/04-进程间调用与gRPC.md)、[05-接入与契约/01-REST API契约-前端与管理端接入.md](05-接入与契约/01-REST API契约-前端与管理端接入.md)、[05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md](05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md) |
| 登录身份、凭证、挑战如何建模 | [02-认证AuthN/00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md](02-认证AuthN/00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md) |
| 开通流程如何生成 LoginIdentity 与 Credential | [02-认证AuthN/01-Onboarding链路-从身份开通到LoginIdentity与Credential.md](02-认证AuthN/01-Onboarding链路-从身份开通到LoginIdentity与Credential.md) |
| 登录如何变成 Principal | [02-认证AuthN/03-Login链路-从登录请求到Principal.md](02-认证AuthN/03-Login链路-从登录请求到Principal.md) |
| Principal 如何变成 AccessToken / RefreshToken | [02-认证AuthN/04-Token链路-从Principal到AccessToken与RefreshToken.md](02-认证AuthN/04-Token链路-从Principal到AccessToken与RefreshToken.md) |
| Session、AccessToken、RefreshToken 边界是什么 | [02-认证AuthN/05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md](02-认证AuthN/05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md)、[07-宣讲/07-JWKS与Token安全讲法.md](07-宣讲/07-JWKS与Token安全讲法.md) |
| JWT/JWS/JWK/JWKS 与 KeyRotation 如何工作 | [02-认证AuthN/06-JWT-JWS-JWK-JWKS边界与KeyRotation.md](02-认证AuthN/06-JWT-JWS-JWK-JWKS边界与KeyRotation.md) |
| 微信/企微登录和 IDP 如何协作 | [02-认证AuthN/07-第三方登录与IDP协作-WeChat-WeCom.md](02-认证AuthN/07-第三方登录与IDP协作-WeChat-WeCom.md)、[07-宣讲/06-IDP与第三方登录讲法.md](07-宣讲/06-IDP与第三方登录讲法.md) |
| 授权模型如何组织 | [03-授权AuthZ/00-AuthZ模型总览-Subject-Role-Resource-Permission-RoleBinding.md](03-授权AuthZ/00-AuthZ模型总览-Subject-Role-Resource-Permission-RoleBinding.md) |
| Resource、Action、Scope 如何建模 | [03-授权AuthZ/01-资源模型-ResourceKey-ResourcePattern-Action-Scope.md](03-授权AuthZ/01-资源模型-ResourceKey-ResourcePattern-Action-Scope.md) |
| Role、RoleBinding、Subject 如何建模 | [03-授权AuthZ/02-角色模型-Role-RoleBinding-Subject.md](03-授权AuthZ/02-角色模型-Role-RoleBinding-Subject.md) |
| 授权写入为什么不是简单 CRUD | [03-授权AuthZ/03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md](03-授权AuthZ/03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md) |
| 授权版本事件和 Outbox 如何传播 | [03-授权AuthZ/04-授权版本与事件传播链路-PolicyVersion-Outbox-RuntimeReload.md](03-授权AuthZ/04-授权版本与事件传播链路-PolicyVersion-Outbox-RuntimeReload.md)、[07-宣讲/08-Outbox与授权版本传播讲法.md](07-宣讲/08-Outbox与授权版本传播讲法.md) |
| 一次权限 Check 如何判定 | [03-授权AuthZ/05-权限检查链路-Check-Snapshot.md](03-授权AuthZ/05-权限检查链路-Check-Snapshot.md) |
| Casbin 在 AuthZ 中是什么角色 | [03-授权AuthZ/06-Casbin运行时模型-pgFacts与四段Matcher.md](03-授权AuthZ/06-Casbin运行时模型-pgFacts与四段Matcher.md) |
| User、Profile、ProfileLink 如何建模 | [04-身份Identity/README.md](04-身份Identity/README.md)、[04-身份Identity/01-User与Profile模型.md](04-身份Identity/01-User与Profile模型.md)、[04-身份Identity/02-ProfileLink链路--用户与儿童档案关系协作.md](04-身份Identity/02-ProfileLink链路--用户与儿童档案关系协作.md) |
| 业务系统如何接入 IAM | [05-接入与契约/README.md](05-接入与契约/README.md)、[05-接入与契约/00-接入总览-业务系统如何接入IAM.md](05-接入与契约/00-接入总览-业务系统如何接入IAM.md) |
| REST/gRPC/SDK 如何划分 | [05-接入与契约/01-REST API契约-前端与管理端接入.md](05-接入与契约/01-REST API契约-前端与管理端接入.md)、[05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md](05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md)、[05-接入与契约/03-SDK接入模型-Go服务端集成.md](05-接入与契约/03-SDK接入模型-Go服务端集成.md) |
| qs-server 如何接入 IAM | [05-接入与契约/04-业务系统接入链路-qs-server接入 IAM 详解.md](05-接入与契约/04-业务系统接入链路-qs-server接入 IAM 详解.md) |
| 契约事实源和防漂移机制是什么 | [05-接入与契约/05-契约事实源与防漂移机制.md](05-接入与契约/05-契约事实源与防漂移机制.md) |
| 架构边界如何被测试保护 | [06-架构护栏/README.md](06-架构护栏/README.md)、[06-架构护栏/01-架构测试与依赖边界.md](06-架构护栏/01-架构测试与依赖边界.md) |
| 文档事实源和防漂移机制是什么 | [06-架构护栏/02-文档事实源与防漂移机制.md](06-架构护栏/02-文档事实源与防漂移机制.md) |
| 如何对外讲解、面试准备、技术分享 | [07-宣讲/README.md](07-宣讲/README.md)、[07-宣讲/11-30分钟技术分享脚本.md](07-宣讲/11-30分钟技术分享脚本.md)、[07-宣讲/13-面试追问证据索引.md](07-宣讲/13-面试追问证据索引.md) |
| 面试应该画哪些图 | [07-宣讲/12-架构图素材索引.md](07-宣讲/12-架构图素材索引.md) |

---

## 5. 分层说明

### 5.1 00-概览

回答：

```text
这个系统是什么？
整体架构怎么分层？
新读者从哪里开始？
```

重点说明：

- IAM 的系统定位；
- 系统外部关系；
- 代码分层；
- 核心模块；
- 读者路径；
- 事实源优先级。

---

### 5.2 01-运行时

回答：

```text
服务如何启动、装配、协作和关闭？
```

重点说明：

- 服务入口与生命周期；
- qs-apiserver 启动与组合根；
- collection-server 运行时；
- qs-worker 运行时；
- 进程间 gRPC 调用；
- IAM 认证与身份链路；
- 后台任务与调度；
- 优雅关闭与资源释放。

---

### 5.3 02-认证 AuthN

回答：

```text
用户如何登录，登录态如何被管理和验证？
```

重点说明：

- User / LoginIdentity / Credential / Challenge；
- Onboarding；
- Linking；
- Login -> Principal；
- Principal -> Session / AccessToken / RefreshToken；
- JWT / JWS / JWK / JWKS；
- KeyRotation；
- Online Verify；
- IDP / WeChat / WeCom 协作；
- AuthN 分层架构与事实源。

---

### 5.4 03-授权 AuthZ

回答：

```text
某个 Subject 能不能访问某个 Resource？
```

重点说明：

- Subject；
- Tenant；
- ResourceKey / ResourcePattern；
- Action；
- Scope；
- Role；
- Permission；
- RoleBinding；
- Assignment wire term；
- AuthorizationRequest；
- AuthorizationDecision；
- Check；
- Snapshot；
- Casbin runtime；
- PolicyAdministration；
- PolicyChangeCommitter；
- Unit of Work；
- PolicyVersion；
- Transactional Outbox；
- RuntimeReload。

---

### 5.5 04-身份 Identity

回答：

```text
User、Profile、ProfileLink 如何表达业务身份关系？
```

重点说明：

- User 是登录主体；
- Profile 是业务档案；
- ProfileLink 是 User 与 Profile 的关系事实；
- self / parent / grandparent / other 等关系类型；
- active / revoked 生命周期；
- MyProfiles / MyProfileLinks 当前用户视角 guard；
- ProfileLink 不等于 AuthZ Permission；
- AuthZ 才负责资源级权限判定。

---

### 5.6 05-接入与契约

回答：

```text
外部系统如何接入 IAM？
```

重点说明：

- REST 适合 Web、App、管理后台、登录和 HTTP 调试；
- gRPC 适合可信服务间调用；
- SDK 适合 Go 业务服务低成本接入；
- qs-server 如何接入 IAM；
- JWT 如何传递；
- JWKS 何时用于本地验签；
- 在线 Verify 何时必须使用；
- AuthZ Check 如何接入；
- public / protected / admin / debug 能力边界；
- OpenAPI / proto / SDK public API 是各自事实源。

---

### 5.7 06-架构护栏

回答：

```text
架构和契约为什么不会轻易漂移？
```

重点说明：

- domain 不依赖 infra/database；
- application 不依赖 transport；
- transport 不绕过 application；
- container 只是组合根；
- SDK 不 import internal；
- AuthZ 内部统一 rolebinding；
- Casbin facts 不进入 domain/transport；
- REST/gRPC/SDK 契约测试；
- docs-hygiene；
- `_archive` 不作为当前事实源。

---

### 5.8 07-宣讲

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

## 6. 推荐阅读路径

### 6.1 第一次了解 IAM

```text
00-概览/README.md
  -> 00-概览/01-系统架构总览.md
  -> 07-宣讲/00-项目一句话定位.md
  -> 07-宣讲/01-业务背景与问题.md
```

目标：

```text
先知道 IAM 是什么、为什么需要它、整体如何分层。
```

---

### 6.2 后端开发读源码

```text
00-概览/01-系统架构总览.md
  -> 01-运行时/README.md
  -> 01-运行时/01-qs-apiserver启动与组合根.md
  -> 01-运行时/04-进程间调用与gRPC.md
  -> 02-认证AuthN/README.md
  -> 03-授权AuthZ/README.md
  -> 04-身份Identity/README.md
```

目标：

```text
先看运行时装配，再看核心业务域。
```

---

### 6.3 AuthN 重写 / 学习路径

```text
02-认证AuthN/README.md
  -> 02-认证AuthN/00-AuthN模型总览-User-LoginIdentity-Credential-Challenge.md
  -> 02-认证AuthN/01-Onboarding链路-从身份开通到LoginIdentity与Credential.md
  -> 02-认证AuthN/02-Linking链路-登录身份绑定解绑与安全边界.md
  -> 02-认证AuthN/03-Login链路-从登录请求到Principal.md
  -> 02-认证AuthN/04-Token链路-从Principal到AccessToken与RefreshToken.md
  -> 02-认证AuthN/05-Session与Token边界-Principal-Session-AccessToken-RefreshToken.md
  -> 02-认证AuthN/06-JWT-JWS-JWK-JWKS边界与KeyRotation.md
  -> 02-认证AuthN/07-第三方登录与IDP协作-WeChat-WeCom.md
  -> 02-认证AuthN/08-AuthN分层架构与事实源索引.md
```

目标：

```text
按模型 -> 开通 -> 绑定 -> 登录 -> Token -> Session -> JWKS -> IDP -> 分层事实源的顺序理解 AuthN。
```

---

### 6.4 AuthZ 重写 / 学习路径

```text
03-授权AuthZ/README.md
  -> 03-授权AuthZ/00-AuthZ模型总览-Subject-Role-Resource-Permission-RoleBinding.md
  -> 03-授权AuthZ/01-资源模型-ResourceKey-ResourcePattern-Action-Scope.md
  -> 03-授权AuthZ/02-角色模型-Role-RoleBinding-Subject.md
  -> 03-授权AuthZ/03-授权写入链路-PolicyAdministration-PolicyChange-PolicyChangeCommitter.md
  -> 03-授权AuthZ/04-授权版本与事件传播链路-PolicyVersion-Outbox-RuntimeReload.md
  -> 03-授权AuthZ/05-权限检查链路-Check-Snapshot.md
  -> 03-授权AuthZ/06-Casbin运行时模型-pgFacts与四段Matcher.md
  -> 03-授权AuthZ/07-AuthZ分层架构与事实源索引.md
```

目标：

```text
按模型 -> 资源 -> 角色 -> 写入 -> 版本传播 -> Check -> Casbin -> 分层事实源的顺序理解 AuthZ。
```

---

### 6.5 接入方阅读路径

```text
05-接入与契约/README.md
  -> 05-接入与契约/00-接入总览-业务系统如何接入IAM.md
  -> 05-接入与契约/01-REST API契约-前端与管理端接入.md
  -> 05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md
  -> 05-接入与契约/03-SDK接入模型-Go服务端集成.md
  -> 05-接入与契约/04-业务系统接入链路-qs-server接入 IAM 详解.md
  -> 05-接入与契约/05-契约事实源与防漂移机制.md
  -> ../api/rest
  -> ../api/grpc
  -> ../pkg/sdk
```

目标：

```text
知道什么时候用 REST、什么时候用 gRPC、什么时候用 SDK，以及业务系统如何完整接入 IAM。
```

---

### 6.6 面试准备路径

```text
07-宣讲/README.md
  -> 07-宣讲/00-项目一句话定位.md
  -> 07-宣讲/01-业务背景与问题.md
  -> 07-宣讲/02-系统架构讲法.md
  -> 07-宣讲/03-AuthN认证体系讲法.md
  -> 07-宣讲/04-AuthZ授权体系讲法.md
  -> 07-宣讲/05-Identity与ProfileLink讲法.md
  -> 07-宣讲/07-JWKS与Token安全讲法.md
  -> 07-宣讲/08-Outbox与授权版本传播讲法.md
  -> 07-宣讲/09-REST-gRPC-SDK接入讲法.md
  -> 07-宣讲/10-工程质量与架构护栏讲法.md
  -> 07-宣讲/13-面试追问证据索引.md
```

目标：

```text
把项目讲清楚，并能回答追问。
```

---

### 6.7 技术分享路径

```text
07-宣讲/11-30分钟技术分享脚本.md
  -> 07-宣讲/12-架构图素材索引.md
  -> 07-宣讲/00-项目一句话定位.md
  -> 07-宣讲/02-系统架构讲法.md
  -> 07-宣讲/03-AuthN认证体系讲法.md
  -> 07-宣讲/04-AuthZ授权体系讲法.md
  -> 07-宣讲/05-Identity与ProfileLink讲法.md
  -> 07-宣讲/10-工程质量与架构护栏讲法.md
```

目标：

```text
先拿完整脚本，再选图，最后补细节。
```

---

### 6.8 架构评审路径

```text
00-概览/01-系统架构总览.md
  -> 01-运行时/README.md
  -> 06-架构护栏/README.md
  -> 06-架构护栏/01-架构测试与依赖边界.md
  -> 06-架构护栏/02-文档事实源与防漂移机制.md
  -> 05-接入与契约/05-契约事实源与防漂移机制.md
```

目标：

```text
看清模块边界、依赖方向、契约防漂移和事实源优先级。
```

---

## 7. 事实源优先级

文档中出现事实冲突时，按以下优先级判断：

1. **源码与运行时行为**  
   `cmd/`、`internal/apiserver/`、`internal/pkg/`、`pkg/`。

2. **机器契约、配置和迁移**  
   `api/rest`、`api/grpc`、`configs`、`internal/pkg/migration/migrations`、`pkg/sdk` public API。

3. **测试与架构护栏**  
   `internal/pkg/architecture`、REST/gRPC transport tests、SDK public API compile tests、docs-hygiene。

4. **当前事实层文档**  
   `docs/00-概览` 到 `docs/06-架构护栏`。

5. **表达层文档**  
   `docs/07-宣讲`。

6. **历史文档与归档材料**  
   `_archive/` 只用于历史追溯，不作为当前事实源。

---

## 8. 核心术语

| 术语 | 当前约定 |
| --- | --- |
| IAM | Identity and Access Management 服务，不等同于单纯用户管理后台 |
| AuthN | Authentication，负责登录、登录身份、凭证、Session、Token、JWKS、Verify |
| AuthZ | Authorization，负责 Subject、Role、Resource、Permission、RoleBinding、Check、PolicyVersion |
| Identity | 身份主体与业务档案关系，负责 User、Profile、ProfileLink |
| IDP | 第三方身份源基础设施，负责 Provider App、SecretVault、外部 API |
| User | 登录主体，IAM 内部身份锚点 |
| LoginIdentity | 某个 User 可使用的一种登录身份 |
| Credential | 稳定认证材料，如密码哈希、外部账号绑定、服务凭证等 |
| Challenge | 一次性或短期认证挑战，如验证码、OAuth code、微信 code |
| Principal | Login 成功后的认证主体摘要，是 Login 与 Token 链路的边界对象 |
| Profile | 业务档案，例如本人档案、儿童档案、被测评者档案 |
| ProfileLink | User 与 Profile 的关系事实 |
| Assignment | REST/proto/SDK 对外 wire term，表示角色分配 |
| RoleBinding | 内部 application/domain 标准术语，表示 subject 与 role 的绑定 |
| Session | 在线登录态锚点 |
| AccessToken | 短期访问凭证，当前实现为 JWT |
| RefreshToken | 服务端可控的续期凭证 |
| JWS | JWT 的签名表示形式 |
| JWK | 单个 JSON Web Key |
| JWKS | JWK Set，公钥发布机制，用于业务服务本地验签 |
| Online Verify | 在线认证状态判断，用于检查 revoked/session/user/account 状态 |
| PolicyVersion | tenant 级授权事实版本 |
| Transactional Outbox | 业务事实和事件记录同事务提交，再由 relay 异步发布 |
| RuntimeReload | 授权变更提交后刷新当前进程 runtime policy |
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

## 9. 代码与契约入口

| 主题 | 入口 |
| --- | --- |
| 根项目说明 | [`../README.md`](../README.md) |
| 进程入口 | [`../cmd/apiserver`](../cmd/apiserver) |
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

## 10. 文档维护规则

### 10.1 不把文档写成源码转述

文档不是逐行解释代码。

文档应该解释：

```text
为什么这样分层；
为什么这样装配；
为什么这样建模；
失败边界在哪里；
如何继续读源码。
```

---

### 10.2 不把 API 文档重复成字段清单

字段、路径、RPC 以 OpenAPI、proto、SDK public API 为准。

`docs/` 只解释接入方式、语义边界和设计取舍。

---

### 10.3 不从 `_archive` 复制当前事实

`_archive/` 可以用于历史追溯，但不能作为当前架构、当前代码、当前接口事实源。

---

### 10.4 每篇文档必须能回链证据

文档中出现关键判断，应该能指向：

```text
源码路径；
API 契约；
配置文件；
测试文件；
当前事实层文档。
```

---

### 10.5 术语必须统一

尤其注意：

```text
ProfileLink 不要退回旧关系术语；
内部 AuthZ 不要退回 assignment 包；
Casbin 不要被写成业务语言；
container 不要被写成 service 层；
process 不要被写成普通 router/server 包；
SDK 不要被写成业务层；
IDP 不要被写成登录态所有者；
JWKS 不要被写成在线状态判断；
Outbox 不要被写成 exactly-once。
```

---

### 10.6 不恢复旧 active 目录体系

当前 active 文档入口是：

```text
00-概览
01-运行时
02-认证AuthN
03-授权AuthZ
04-身份Identity
05-接入与契约
06-架构护栏
07-宣讲
_archive
```

不要恢复旧入口：

```text
02-业务域
03-接口与集成
04-基础设施与运维
05-专题分析
07-专题分析
08-宣讲
```

历史材料应归档到 `_archive/`。

---

## 11. 发布检查清单

文档发布前至少检查：

```text
1. docs/README.md 能作为唯一总入口；
2. 每个 active 目录都有 README.md；
3. 每篇正文都有清晰定位；
4. 每篇正文有 30 秒结论或核心结论；
5. 每篇正文能回链源码、契约或测试；
6. Mermaid 图能解释核心链路；
7. 没有把旧目录作为 active 入口；
8. 没有把 _archive 当成当前事实源；
9. 术语统一：ProfileLink / RoleBinding / Assignment / Session / Outbox / SDK / IDP；
10. OpenAPI / proto / SDK README 不互相矛盾；
11. make docs-hygiene 通过。
```

---

## 12. 验证命令

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

## 13. 本文总结

`docs/` 是 IAM 的解释层，不是机器契约本身。

当前文档体系的核心分工是：

```text
00-概览：建立系统地图；
01-运行时：解释服务如何装配运行；
02-认证AuthN：解释认证模型、登录态和 token 生命周期；
03-授权AuthZ：解释资源授权、权限检查和版本传播；
04-身份Identity：解释 User、Profile、ProfileLink；
05-接入与契约：解释 REST、gRPC、SDK 和业务系统接入；
06-架构护栏：解释如何防止边界、契约和文档漂移；
07-宣讲：解释如何对外讲清楚；
_archive：保存历史材料，不作为当前事实源。
```

如果只记一句话：

> 这套文档的目标不是覆盖每一个源码文件，而是让读者准确理解 IAM 的系统边界、核心链路、设计取舍、接入方式、架构护栏和对外表达方式。
