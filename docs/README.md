# IAM 文档中心

## 1. 文档中心定位

`docs/` 是 IAM 项目的解释层，它负责说明：

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

它不是机器契约本身，机器契约与运行事实以这些内容为准：

- REST 路径、字段、schema、错误响应以 [`../api/rest`](../api/rest) 为准。
- gRPC service、message、RPC 以 [`../api/grpc`](../api/grpc) 为准。
- Go SDK 公开 API 以 [`../pkg/sdk`](../pkg/sdk) 为准。
- 运行行为以源码和测试为准。

文档的职责不是逐行转述源码，而是帮助读者建立心智模型，回答：

- IAM 是什么
- 为什么这样分层，为什么这样建模
- 服务如何启动和装配和关闭
- AuthN / AuthZ / Identity / IDP 如何协作
- 业务系统如何接入 IAM，如何防止代码、契约和文档漂移
- 如何对外讲清楚这个项目

---

## 2. 30 秒结论

IAM 是一个面向业务系统接入的身份与访问管理服务。

它不是普通用户中心、登录系统、权限 CRUD 系统或微信登录模块，而是围绕“用户是谁、如何证明用户身份、用户能访问什么资源”三个核心问题，统一提供：

- 用户身份建模
- 账号认证
- 访问授权
- 第三方身份源接入
- Profile 联想搜索读模型

同时，它还提供 REST / gRPC / Go SDK 等多种接入方式，以便于业务系统接入和集成。

当前文档体系按下面目录结构组织：

| 目录 | 作用 |
| --- | --- |
| `00-概览` | 说明 IAM 是什么、如何分层、核心模块如何协作 |
| `01-运行时` | 解释服务如何启动、装配 REST/gRPC、运行后台任务、优雅关闭和资源释放 |
| `02-认证AuthN` | - 讲解 LoginIdentity、Credential、Session、Token、JWKS 核心领域模型；<br/>- 说明 Onboarding、Linking、Login、Token、Session 链路 |
| `03-授权AuthZ` | - 讲解 Subject、Role、Resource、Permission 核心领域模型；<br/>-描述授权写入链路、授权版本与事件传播链路、权限检查链路；<br/>- 说明 Casbin 在 AuthZ 中的角色 |
| `04-身份Identity` | - 讲解 User、Profile、ProfileLink 核心领域模型；<br/>-描述 ProfileLink 链路，以及与 AuthN、AuthZ 模块的边界 |
| `05-接入与契约` | - 说明 REST、gRPC、SDK 三类接入方式；<br/> - 说明 qs-server 接入链路；<br/> - 说明契约防漂移 |
| `06-架构护栏` | - 说明架构测试、契约测试、SDK compile test、docs-hygiene 如何防漂移 |
| `07-宣讲` | - 技术分享、对外讲解的表达和追问证据链 |
| `08-Suggest` | - 说明 Suggest Profile 联想搜索读模型 |

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
├── 08-Suggest/
│   ├── README.md
│   ├── 00-Suggest模块总览-Profile联想搜索读模型.md
│   ├── 01-查询链路-SuggestProfile从请求到索引过滤.md
│   ├── 02-权限范围-OperatingPrincipal与ProfileAccessScope.md
│   ├── 03-索引模型-ProfileSearchTerm-Trie-Hash-Runtime.md
│   ├── 04-刷新链路-Loader-Refresher-FullDelta-Snapshot.md
│   └── 05-安全与运维-手机号搜索-限流-指标-降级.md
│
└── _archive/
    └── README.md
```

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
| Profile 联想搜索如何工作 | [08-Suggest/README.md](08-Suggest/README.md)、[08-Suggest/00-Suggest模块总览-Profile联想搜索读模型.md](08-Suggest/00-Suggest模块总览-Profile联想搜索读模型.md)、[08-Suggest/01-查询链路-SuggestProfile从请求到索引过滤.md](08-Suggest/01-查询链路-SuggestProfile从请求到索引过滤.md) |
| 业务系统如何接入 IAM | [05-接入与契约/README.md](05-接入与契约/README.md)、[05-接入与契约/00-接入总览-业务系统如何接入IAM.md](05-接入与契约/00-接入总览-业务系统如何接入IAM.md) |
| REST/gRPC/SDK 如何划分 | [05-接入与契约/01-REST API契约-前端与管理端接入.md](05-接入与契约/01-REST API契约-前端与管理端接入.md)、[05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md](05-接入与契约/02-gRPC API契约-服务间调用与内部集成.md)、[05-接入与契约/03-SDK接入模型-Go服务端集成.md](05-接入与契约/03-SDK接入模型-Go服务端集成.md) |
| qs-server 如何接入 IAM | [05-接入与契约/04-业务系统接入链路-qs-server接入 IAM 详解.md](05-接入与契约/04-业务系统接入链路-qs-server接入 IAM 详解.md) |
| 契约事实源和防漂移机制是什么 | [05-接入与契约/05-契约事实源与防漂移机制.md](05-接入与契约/05-契约事实源与防漂移机制.md) |
| 架构边界如何被测试保护 | [06-架构护栏/README.md](06-架构护栏/README.md)、[06-架构护栏/01-架构测试与依赖边界.md](06-架构护栏/01-架构测试与依赖边界.md) |
| 文档事实源和防漂移机制是什么 | [06-架构护栏/02-文档事实源与防漂移机制.md](06-架构护栏/02-文档事实源与防漂移机制.md) |
| 如何对外讲解、面试准备、技术分享 | [07-宣讲/README.md](07-宣讲/README.md)、[07-宣讲/11-30分钟技术分享脚本.md](07-宣讲/11-30分钟技术分享脚本.md)、[07-宣讲/13-面试追问证据索引.md](07-宣讲/13-面试追问证据索引.md) |
| 面试应该画哪些图 | [07-宣讲/12-架构图素材索引.md](07-宣讲/12-架构图素材索引.md) |

---

## 5. 推荐阅读路径

### 5.1 第一次了解 IAM

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

### 5.2 后端开发读源码

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

### 5.3 AuthN 学习路径

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

### 5.4 AuthZ 学习路径

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

### 5.5 接入方学习路径

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

### 5.6 Suggest 模块学习路径

```text
08-Suggest/README.md
  -> 08-Suggest/00-Suggest模块总览-Profile联想搜索读模型.md
  -> 08-Suggest/01-查询链路-SuggestProfile从请求到索引过滤.md
  -> 08-Suggest/02-权限范围-OperatingPrincipal与ProfileAccessScope.md
  -> 08-Suggest/03-索引模型-ProfileSearchTerm-Trie-Hash-Runtime.md
  -> 08-Suggest/04-刷新链路-Loader-Refresher-FullDelta-Snapshot.md
  -> 08-Suggest/05-安全与运维-手机号搜索-限流-指标-降级.md
```

目标：

```text
理解 Suggest 为什么是 Profile 联想搜索读模型，如何在不拆独立服务的情况下完成高频 autocomplete、权限过滤、索引刷新、手机号安全、限流、指标和降级。
```

---

### 5.7 宣讲准备路径

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
把项目讲清楚，能回答追问。
```

---

### 5.8 技术分享路径

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

### 5.9 架构评审路径

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

## 6. 事实源优先级

文档中出现事实冲突时，按以下优先级判断：

1. **源码与运行时行为**  
   `cmd/`、`internal/apiserver/`、`internal/pkg/`、`pkg/`。

2. **机器契约、配置和迁移**  
   `api/rest`、`api/grpc`、`configs`、`internal/pkg/migration/migrations`、`pkg/sdk` public API。

3. **测试与架构护栏**  
   `internal/pkg/architecture`、REST/gRPC transport tests、SDK public API compile tests、docs-hygiene。

4. **当前事实层文档**  
   `docs/00-概览` 到 `docs/06-架构护栏`，以及 `docs/08-Suggest` 中的 Suggest 模块事实文档。

5. **表达层文档**  
   `docs/07-宣讲`。

6. **历史文档与归档材料**  
   `_archive/` 只用于历史追溯，不作为当前事实源。

---

## 7. 文档维护规则

### 7.1 不把文档写成源码转述

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

### 7.2 不把 API 文档重复成字段清单

字段、路径、RPC 以 OpenAPI、proto、SDK public API 为准。

`docs/` 只解释接入方式、语义边界和设计取舍。

---

### 7.3 不从 `_archive` 复制当前事实

`_archive/` 可以用于历史追溯，但不能作为当前架构、当前代码、当前接口事实源。

---

### 7.4 每篇文档必须能回链证据

文档中出现关键判断，应该能指向：

```text
源码路径；
API 契约；
配置文件；
测试文件；
当前事实层文档。
```

---

### 7.5 术语必须统一

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
Suggest 不要被写成 AuthN/AuthZ/Identity 核心域；
ProfileAccessScope 不要被写成逐条 Casbin 检查；
TenantDomain 不要被写成业务 org_id；
mobile_mask 不要退回明文 mobile；
```

---

## 8. 发布检查清单

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
9. 术语统一：ProfileLink / RoleBinding / Assignment / Session / Outbox / SDK / IDP / Suggest / ProfileAccessScope；
10. OpenAPI / proto / SDK README 不互相矛盾；
11. make docs-hygiene 通过。
12. Suggest 文档入口可从 docs/README.md 访问；
13. active docs 中没有 tenant_id 冒充 org_id 的描述；
14. active docs 中没有把 Suggest 写成 IAM 核心身份域；
15. active docs 中没有把 mobile_mask 写成明文 mobile。
```

---

## 9. 验证命令

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

Suggest 链路按需检查：

```bash
go test ./internal/apiserver/domain/suggest/... \
  ./internal/apiserver/application/suggest/... \
  ./internal/apiserver/infra/suggest/... \
  ./internal/apiserver/infra/mysql/suggest/... \
  ./internal/apiserver/transport/rest/suggest/...
```

---

## 10. 本文总结

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
08-Suggest：解释 Profile 联想搜索读模型的查询、权限、索引、刷新、安全与运维；
_archive：保存历史材料，不作为当前事实源。
```

如果只记一句话：

> 这套文档的目标不是覆盖每一个源码文件，而是让读者准确理解 IAM 的系统边界、核心链路、设计取舍、接入方式、架构护栏和对外表达方式。
