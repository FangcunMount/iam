# IAM 文档中心

本文档中心是 IAM 的解释层，负责说明系统边界、运行链路、设计取舍和接入方式。接口字段、路由和 RPC 以 [../api](../api) 的机器契约为准；运行行为以代码和测试为准。

## 30 秒结论

- 当前主运行单元是 `iam-apiserver`，入口在 [../cmd/apiserver/apiserver.go](../cmd/apiserver/apiserver.go)。
- 运行时由 [../internal/apiserver/process](../internal/apiserver/process)、[../internal/apiserver/container](../internal/apiserver/container)、[../internal/apiserver/transport](../internal/apiserver/transport) 组织。
- AuthN、AuthZ、User、IDP、Suggest、CacheGovernance 由 container 装配；REST 和 gRPC 只是 transport adapter。
- 用户关系的当前代码和契约名是 `ProfileLink`；中文可解释为档案关系或监护关系语义。
- 授权公开合同仍使用 `assignment` wire term；内部应用实现以 `rolebinding` 为准。
- [./_archive](./_archive) 只保存历史材料，不属于活跃事实层。

## 阅读入口

| 你要回答的问题 | 先读 |
| ---- | ---- |
| 系统怎么分层、代码从哪里进入 | [00-概览/01-系统架构总览.md](00-概览/01-系统架构总览.md) |
| 术语和当前事实源怎么判定 | [00-概览/02-核心概念术语.md](00-概览/02-核心概念术语.md)、[00-概览/03-阅读路径&代码组织与事实来源.md](00-概览/03-阅读路径&代码组织与事实来源.md) |
| HTTP/gRPC 如何启动和注册 | [01-运行时/01-服务入口&HTTP 与模块装配.md](01-运行时/01-服务入口&HTTP 与模块装配.md)、[01-运行时/02-gRPC与mTLS.md](01-运行时/02-gRPC与mTLS.md) |
| AuthN、Token、JWKS 怎么工作 | [02-业务域/01-authn-认证&Token&JWKS.md](02-业务域/01-authn-认证&Token&JWKS.md)、[05-专题分析/01-认证链路--从登录请求到 Token 与 JWKS.md](05-专题分析/01-认证链路--从登录请求到%20Token%20与%20JWKS.md) |
| AuthZ、角色、策略和授权快照怎么工作 | [02-业务域/02-authz-角色&策略&资源&Assignment.md](02-业务域/02-authz-角色&策略&资源&Assignment.md)、[05-专题分析/03-授权判定链路--角色&策略&资源&Assignment&Casbin.md](05-专题分析/03-授权判定链路--角色&策略&资源&Assignment&Casbin.md) |
| 用户档案和 ProfileLink 怎么接入 | [02-业务域/03-user-用户&儿童&ProfileLink.md](02-业务域/03-user-用户&儿童&ProfileLink.md)、[05-专题分析/04-ProfileLink链路--用户&儿童档案关系协作.md](05-专题分析/04-ProfileLink链路--用户&儿童档案关系协作.md) |
| IDP、微信应用和外部身份怎么管理 | [02-业务域/05-idp-微信应用与外部身份.md](02-业务域/05-idp-微信应用与外部身份.md)、[05-专题分析/08-IDP与微信登录链路--WechatApp到AuthNProof.md](05-专题分析/08-IDP与微信登录链路--WechatApp到AuthNProof.md) |
| Suggest 联想搜索读模型怎么刷新和查询 | [02-业务域/04-suggest-儿童联想搜索.md](02-业务域/04-suggest-儿童联想搜索.md)、[05-专题分析/09-Suggest读模型链路--候选刷新到联想查询.md](05-专题分析/09-Suggest读模型链路--候选刷新到联想查询.md) |
| 缓存治理、Redis 建模和 Outbox 怎么排查 | [05-专题分析/05-IAM缓存层--缓存层的设计与治理.md](05-专题分析/05-IAM缓存层--缓存层的设计与治理.md)、[05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md](05-专题分析/06-IAM缓存层--数据结构选择与%20Redis%20建模判断.md)、[05-专题分析/10-授权版本事件链路--UoW到OutboxRelay.md](05-专题分析/10-授权版本事件链路--UoW到OutboxRelay.md) |
| 业务域里用了哪些设计模式 | [02-业务域/06-业务域设计模式地图.md](02-业务域/06-业务域设计模式地图.md) |
| REST/gRPC 合同在哪里 | [../api/rest/README.md](../api/rest/README.md)、[../api/grpc/README.md](../api/grpc/README.md) |
| SDK 怎么接入 | [05-专题分析/07-SDK封装与接入价值.md](05-专题分析/07-SDK封装与接入价值.md)、[../pkg/sdk/docs/README.md](../pkg/sdk/docs/README.md) |
| 文档怎么写、怎么验证 | [CONTRIBUTING-DOCS.md](CONTRIBUTING-DOCS.md) |

## 文档分层

| 层 | 作用 |
| ---- | ---- |
| [00-概览](00-概览/README.md) | 系统地图、术语、阅读路径、事实来源 |
| [01-运行时](01-运行时/README.md) | 进程生命周期、REST/gRPC、HTTP 认证、健康检查、debug 面 |
| [02-业务域](02-业务域/README.md) | AuthN、AuthZ、User/ProfileLink、Suggest、IDP 与业务域设计模式 |
| [03-接口与集成](03-接口与集成/README.md) | REST、gRPC、授权接入、身份接入、QS 接入 |
| [04-基础设施与运维](04-基础设施与运维/README.md) | 架构实践、CQRS、契约校验、端口证书、迁移、Outbox |
| [05-专题分析](05-专题分析/README.md) | 认证链路、授权链路、ProfileLink、缓存治理、Redis 建模、SDK、IDP、Suggest、Outbox 等跨模块深潜材料 |
| [_archive](_archive/README.md) | 历史文档，默认不参与活跃文档卫生检查 |

## 事实源优先级

1. 源码与运行时行为：`cmd/`、`internal/apiserver/`、`pkg/`。
2. 机器契约、配置和迁移：`api/rest`、`api/grpc`、`configs`、`internal/pkg/migration/migrations`。
3. 架构和契约测试：尤其是 `internal/pkg/architecture`、REST/gRPC transport tests、SDK compile tests。
4. 当前维护文档：本目录、[../api/README.md](../api/README.md)、[../pkg/sdk/docs/README.md](../pkg/sdk/docs/README.md)。
5. 历史文档与归档材料。

## 维护门禁

```bash
make docs-hygiene
go test ./...
```

涉及 REST、gRPC、OpenAPI 或 swagger 的改动还需要运行：

```bash
make api-validate
```

`make api-validate` 依赖 Docker daemon；Docker 不可用时，应至少单独运行仓库里的 OpenAPI/路由 Python 检查脚本并记录前置条件。
