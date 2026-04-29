# IAM 文档中心

本文回答：`iam` 的现行文档现在按什么结构组织、哪些目录属于主阅读路径、哪些目录已经归档，以及新读者应该从哪里开始读。

## 30 秒结论

- `docs/` 是**解释层**，负责讲清系统边界、主链路、设计取舍、运行与接入方式。
- `api/` 是**机器契约层**，`api/rest/*.yaml` 与 `api/grpc/**/*.proto` 比 prose 文档更靠前；接口字段、行为、错误语义以契约为准。
- 当前主运行单元是 `iam-apiserver`，入口在 [../cmd/apiserver/apiserver.go](../cmd/apiserver/apiserver.go)，核心代码分布在 `internal/apiserver/{interface,application,domain,infra}`。
- 当前现行业务域正文统一看 [02-业务域](./02-业务域/README.md)：`authn`、`authz`、`user` 都已经迁入新层。
- `suggest` 也已经迁入 [02-业务域](./02-业务域/README.md) 作为补充能力正文；`idp` 仍主要通过 `api/` 合同与代码装配点理解。
- 旧 `01-认证域`、`02-授权域`、`03-用户域` 已转入 [docs/_archive](./_archive/README.md)，只作为历史信息源，不再属于主阅读路径。
- 旧 `04-基础设施` 与 `ops` 也已转入 [docs/_archive](./_archive/README.md)；现行基础设施与运维正文统一看 [04-基础设施与运维](./04-基础设施与运维/README.md)。
- 契约相关变更提交前，至少应检查 `make docs-swagger`、`make proto-gen`、`make api-validate` 是否需要同步执行。

## 重点速查

| 想回答的问题 | 先打开哪里 |
| ---- | ---- |
| 这个项目整体是怎么分层的？ | [00-概览/01-系统架构总览.md](./00-概览/01-系统架构总览.md) |
| 如果只先维护最核心文档，应该从哪开始？ | [00-概览/03-阅读路径&代码组织与事实来源.md](./00-概览/03-阅读路径&代码组织与事实来源.md) |
| 当前认证域真正落地了什么？ | [02-业务域/01-authn-认证&Token&JWKS.md](./02-业务域/01-authn-认证&Token&JWKS.md) |
| 当前授权域真正落地了什么？ | [02-业务域/02-authz-角色&策略&资源&Assignment.md](./02-业务域/02-authz-角色&策略&资源&Assignment.md) |
| 当前用户、儿童、监护关系从哪看？ | [02-业务域/03-user-用户&儿童&Ref.md](./02-业务域/03-user-用户&儿童&Ref.md)、[05-专题分析/03-监护关系链路--用户&儿童&Ref 的协作.md](./05-专题分析/03-监护关系链路--用户&儿童&Ref 的协作.md) |
| 当前 Suggest 儿童联想搜索从哪看？ | [02-业务域/04-suggest-儿童联想搜索.md](./02-业务域/04-suggest-儿童联想搜索.md) |
| REST / gRPC 合同在哪里？ | [../api/rest/README.md](../api/rest/README.md)、[../api/grpc/README.md](../api/grpc/README.md)、[../api/README.md](../api/README.md) |
| 业务系统要接授权能力，今天到底能接到什么程度？ | [03-接口与集成/03-授权接入与边界.md](./03-接口与集成/03-授权接入与边界.md) |
| 业务系统要接身份与监护关系能力，今天到底能接到什么程度？ | [03-接口与集成/04-身份接入与监护关系边界.md](./03-接口与集成/04-身份接入与监护关系边界.md) |
| 为什么 `pkg/sdk` 也是 IAM 的主阅读路径之一？ | [05-专题分析/07-SDK封装与接入价值.md](./05-专题分析/07-SDK封装与接入价值.md) |
| IAM 缓存层今天到底怎么设计、治理面已经做到哪里？ | [05-专题分析/05-IAM缓存层--缓存层的设计与治理.md](./05-专题分析/05-IAM缓存层--缓存层的设计与治理.md) |
| IAM 当前各个 cache family 为什么大多还是 Redis `String`，`revoked_access_token` 为什么不是 `Set`？ | [05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md](./05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md) |
| 用户封禁、会话失效、access token 撤销、refresh token 删除到底怎么拆层？ | [05-专题分析/02-IAM认证语义拆层--用户状态、会话与Token边界.md](./05-专题分析/02-IAM认证语义拆层--用户状态、会话与Token边界.md) |
| 配置、部署、迁移从哪看？ | [04-基础设施与运维/README.md](./04-基础设施与运维/README.md)、[04-基础设施与运维/04-端口&证书与数据库迁移.md](./04-基础设施与运维/04-端口&证书与数据库迁移.md)、`configs/`、`build/docker/`、`scripts/` |
| 旧认证/授权/用户/基础设施/ops 文档从哪看？ | [_archive/README.md](./_archive/README.md) |

## 文档分层

| 层 | 作用 | 先读什么 |
| ---- | ---- | ---- |
| **00-概览** | 系统地图、术语、阅读路径、事实来源 | [README.md](./00-概览/README.md) |
| **01-运行时** | 进程、暴露面、gRPC、mTLS、健康检查 | [README.md](./01-运行时/README.md) |
| **02-业务域** | 现行业务域正文，承接 `authn / authz / 用户域（uc）` | [README.md](./02-业务域/README.md) |
| **03-接口与集成** | 契约解释层、接入方式、合同导航 | [README.md](./03-接口与集成/README.md) |
| **04-基础设施与运维** | 技术底座与运维交付入口 | [README.md](./04-基础设施与运维/README.md) |
| **05-专题分析** | 跨层主链路、设计取舍、当前保证与风险边界 | [README.md](./05-专题分析/README.md) |
| **_archive** | 历史文档与旧体系归档，不属于主阅读路径 | [README.md](./_archive/README.md) |
| **机器契约层** | OpenAPI、Proto、契约说明 | [../api/README.md](../api/README.md) |

## 阅读路径

这一节保留完整逐篇阅读清单；[00-概览/03-阅读路径&代码组织与事实来源.md](./00-概览/03-阅读路径&代码组织与事实来源.md) 现在只负责说明“如何选入口、维护时先修哪里、冲突时信什么”，不再重复整套清单。

### 新成员入门

1. [概览 README](./00-概览/README.md)
2. [系统架构总览](./00-概览/01-系统架构总览.md)
3. [核心概念术语](./00-概览/02-核心概念术语.md)
4. [业务域 README](./02-业务域/README.md)
5. [认证、Token、JWKS](./02-业务域/01-authn-认证&Token&JWKS.md)
6. [角色、策略、资源、Assignment](./02-业务域/02-authz-角色&策略&资源&Assignment.md)
7. [用户、儿童、Ref](./02-业务域/03-user-用户&儿童&Ref.md)
8. [六边形架构实践](./04-基础设施与运维/01-六边形架构实践.md)

### 后端开发者

1. [概览 README](./00-概览/README.md)
2. [系统架构总览](./00-概览/01-系统架构总览.md)
3. [运行时 README](./01-运行时/README.md)
4. [业务域 README](./02-业务域/README.md)
5. [认证、Token、JWKS](./02-业务域/01-authn-认证&Token&JWKS.md)
6. [角色、策略、资源、Assignment](./02-业务域/02-authz-角色&策略&资源&Assignment.md)
7. [用户、儿童、Ref](./02-业务域/03-user-用户&儿童&Ref.md)
8. [REST API 文档](../api/rest/README.md) 和 [gRPC API 文档](../api/grpc/README.md)
9. [基础设施与运维 README](./04-基础设施与运维/README.md)

### 核心主线

1. [阅读路径、代码组织与事实来源](./00-概览/03-阅读路径&代码组织与事实来源.md)
2. [系统架构总览](./00-概览/01-系统架构总览.md)
3. [认证、Token、JWKS](./02-业务域/01-authn-认证&Token&JWKS.md)
4. [认证链专题](./05-专题分析/01-认证链路--从登录请求到 Token 与 JWKS.md)
5. [认证语义拆层专题](./05-专题分析/02-IAM认证语义拆层--用户状态、会话与Token边界.md)
6. [角色、策略、资源、Assignment](./02-业务域/02-authz-角色&策略&资源&Assignment.md)
7. [授权判定链专题](./05-专题分析/03-授权判定链路--角色&策略&资源&Assignment&Casbin.md)
8. [用户、儿童、Ref](./02-业务域/03-user-用户&儿童&Ref.md)
9. [监护关系链专题](./05-专题分析/03-监护关系链路--用户&儿童&Ref 的协作.md)
10. [接口与集成 README](./03-接口与集成/README.md)

### 集成方

1. [QS 接入 IAM](./03-接口与集成/05-QS接入IAM.md)
2. [SDK 封装与接入价值](./05-专题分析/07-SDK封装与接入价值.md)
3. [接口与集成 README](./03-接口与集成/README.md)
4. [授权接入与边界](./03-接口与集成/03-授权接入与边界.md)
5. [身份接入与监护关系边界](./03-接口与集成/04-身份接入与监护关系边界.md)
6. [REST API 文档](../api/rest/README.md) 和 [gRPC API 文档](../api/grpc/README.md)
7. [基础设施与运维 README](./04-基础设施与运维/README.md)

## 目录总览

### 00-概览

| 文档 | 说明 | 阅读时间 |
| ---- | ---- | ---- |
| [README.md](./00-概览/README.md) | 概览层入口与本组边界 | 3 min |
| [01-系统架构总览.md](./00-概览/01-系统架构总览.md) | 系统边界、运行单元、主代码分层与契约层 | 10 min |
| [02-核心概念术语.md](./00-概览/02-核心概念术语.md) | 统一业务域、架构层与契约层的关键术语 | 6 min |
| [03-阅读路径&代码组织与事实来源.md](./00-概览/03-阅读路径&代码组织与事实来源.md) | 推荐阅读顺序、代码定位与事实来源优先级 | 8 min |

### 01-运行时

| 文档 | 说明 | 阅读时间 |
| ---- | ---- | ---- |
| [README.md](./01-运行时/README.md) | 运行时入口与本组边界 | 3 min |
| [01-服务入口&HTTP 与模块装配.md](./01-运行时/01-服务入口&HTTP 与模块装配.md) | `iam-apiserver` 启动链、容器初始化顺序、HTTP 暴露面与模块装配 | 10 min |
| [02-gRPC与mTLS.md](./01-运行时/02-gRPC与mTLS.md) | gRPC 运行时、mTLS、ACL、健康检查与注册服务 | 10 min |
| [03-HTTP认证中间件与身份上下文.md](./01-运行时/03-HTTP认证中间件与身份上下文.md) | JWT 中间件、上下文字段、`authz` 与 `RequireRole`/`RequirePermission` | 8 min |
| [04-健康检查&debug 路由与降级启动边界.md](./01-运行时/04-健康检查&debug 路由与降级启动边界.md) | 基础探针、debug 路由、部分初始化与运行时降级边界 | 6 min |

### 02-业务域

| 文档 | 说明 | 阅读时间 |
| ---- | ---- | ---- |
| [README.md](./02-业务域/README.md) | 业务域统一入口与迁移约定 | 4 min |
| [01-authn-认证&Token&JWKS.md](./02-业务域/01-authn-认证&Token&JWKS.md) | 认证域现状版正文 | 10 min |
| [02-authz-角色&策略&资源&Assignment.md](./02-业务域/02-authz-角色&策略&资源&Assignment.md) | 授权域现状版正文 | 10 min |
| [03-user-用户&儿童&Ref.md](./02-业务域/03-user-用户&儿童&Ref.md) | 用户域现状版正文，覆盖模型、监护关系、查询判定和事件边界 | 18 min |
| [04-suggest-儿童联想搜索.md](./02-业务域/04-suggest-儿童联想搜索.md) | Suggest 补充读侧能力现状版正文，覆盖接口、刷新链、配置和边界 | 8 min |

### 03-接口与集成

| 文档 | 说明 | 阅读时间 |
| ---- | ---- | ---- |
| [README.md](./03-接口与集成/README.md) | 接口与集成统一入口 | 4 min |
| [01-REST契约与接入.md](./03-接口与集成/01-REST契约与接入.md) | REST 合同、路由注册、Swagger 生成物与验证链 | 10 min |
| [02-gRPC契约与接入.md](./03-接口与集成/02-gRPC契约与接入.md) | gRPC 合同、metadata、错误语义与生成入口 | 10 min |
| [03-授权接入与边界.md](./03-接口与集成/03-授权接入与边界.md) | 授权能力当前可接边界 | 8 min |
| [04-身份接入与监护关系边界.md](./03-接口与集成/04-身份接入与监护关系边界.md) | 身份与监护能力当前可接边界 | 10 min |
| [05-QS接入IAM.md](./03-接口与集成/05-QS接入IAM.md) | 面向 `qs` 这类业务系统的现行接入路径 | 8 min |

### 04-基础设施与运维

| 文档 | 说明 | 阅读时间 |
| ---- | ---- | ---- |
| [README.md](./04-基础设施与运维/README.md) | 基础设施与运维统一入口 | 4 min |
| [01-六边形架构实践.md](./04-基础设施与运维/01-六边形架构实践.md) | interface / application / domain / infra 的真实分层与装配 | 10 min |
| [02-CQRS模式实践.md](./04-基础设施与运维/02-CQRS模式实践.md) | 当前 CQRS 的真实形态与读写边界 | 8 min |
| [03-命令&契约校验与开发流程.md](./04-基础设施与运维/03-命令&契约校验与开发流程.md) | `Makefile`、swagger / OpenAPI / proto 校验链与开发流程 | 10 min |
| [04-端口&证书与数据库迁移.md](./04-基础设施与运维/04-端口&证书与数据库迁移.md) | 端口、mTLS、Docker、migration 入口 | 10 min |
| [05-SQL Bootstrap 与初始化数据.md](./04-基础设施与运维/05-SQL%20Bootstrap%20与初始化数据.md) | schema.sql、bootstrap.sql 与 migration 的职责分工 | 6 min |

### 05-专题分析

| 文档 | 说明 | 阅读时间 |
| ---- | ---- | ---- |
| [README.md](./05-专题分析/README.md) | 专题分析入口 | 4 min |
| [01-认证链路--从登录请求到 Token 与 JWKS.md](./05-专题分析/01-认证链路--从登录请求到 Token 与 JWKS.md) | 登录、认证策略、Token 生命周期、JWKS 发布与轮换边界 | 10 min |
| [02-IAM认证语义拆层--用户状态、会话与Token边界.md](./05-专题分析/02-IAM认证语义拆层--用户状态、会话与Token边界.md) | 用户状态、会话、access token 撤销、refresh token 删除四层语义，以及在线/离线校验边界 | 12 min |
| [03-授权判定链路--角色&策略&资源&Assignment&Casbin.md](./05-专题分析/03-授权判定链路--角色&策略&资源&Assignment&Casbin.md) | 授权管理链、Casbin 模型、策略版本传播与当前判定边界 | 10 min |
| [04-监护关系链路--用户&儿童&Ref 的协作.md](./05-专题分析/04-监护关系链路--用户&儿童&Ref 的协作.md) | 建档、授监护、查询判定链，以及当前合同/运行时边界 | 10 min |
| [05-IAM缓存层--缓存层的设计与治理.md](./05-专题分析/05-IAM缓存层--缓存层的设计与治理.md) | IAM Cache Layer、cache family、只读治理面与运行边界 | 10 min |
| [06-IAM缓存层--数据结构选择与 Redis 建模判断.md](./05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md) | IAM cache family 的 Redis 建模选择、未来升级条件，以及 `revoked_access_token` 为什么当前不是 `Set` | 10 min |
| [07-SDK封装与接入价值.md](./05-专题分析/07-SDK封装与接入价值.md) | SDK 为什么不只是一个 wrapper，而是 IAM 的接入产品层 | 8 min |

### _archive

| 文档 | 说明 |
| ---- | ---- |
| [_archive/README.md](./_archive/README.md) | 历史文档入口与使用边界 |

## 事实入口

| 类型 | 位置 | 说明 |
| ---- | ---- | ---- |
| **源码** | `cmd/`、`internal/apiserver/`、`pkg/` | 运行时行为与边界的最终依据 |
| **REST 合同** | `api/rest/*.yaml` | OpenAPI 3.1 规范 |
| **gRPC 合同** | `api/grpc/**/*.proto` | Proto 契约与兼容性约束 |
| **Swagger 生成物** | `internal/apiserver/docs/swagger.yaml` | 从代码生成并参与契约比对 |
| **配置** | `configs/*.yaml`、`configs/grpc_acl.yaml`、`configs/mysql/schema.sql` | 运行配置、ACL、数据库基线 |
| **部署工件** | `build/docker/`、`scripts/` | Docker、校验脚本、开发辅助 |

## 文档规范

详细写作与维护约定见 [CONTRIBUTING-DOCS.md](./CONTRIBUTING-DOCS.md)。

当前统一约定：

- 先一针见血，再娓娓道来
- `docs/` 解释设计与边界，`api/` 负责机器契约
- 伪代码、示意图、模式图必须尽快回链到真实路径
- 已被新正文替代的旧文档移入 `_archive/`
- 改契约、路由、配置时，文档与验证脚本要同步更新
