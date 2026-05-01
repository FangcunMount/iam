# IAM

IAM 是方寸山项目的身份与访问管理服务，提供完整的认证、授权、用户档案管理和第三方身份集成能力。

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## 功能特性

- **认证系统（AuthN）**：显式登录、Refresh Token、Access Token 撤销、会话校验、Service Token、JWKS 发布与管理
- **授权系统（AuthZ）**：REST/gRPC 请求判定、授权快照、角色/资源/策略/assignment 管理
- **身份管理（Identity）**：用户资料、用户档案、ProfileLink 档案关系查询与写入
- **第三方集成（IDP）**：微信应用配置与登录集成
- **智能搜索（Suggest）**：档案联想搜索
- **缓存治理（CacheGovernance）**：只读缓存目录、状态管理和 debug 面板
- **事件发布（Transactional Outbox）**：域事件通过 outbox relay 持久发布

### 功能模块交互

```ASCII
┌────────────────────────────────────────────────────────────────┐
│                         External Clients                       │
│          (Web / App / WeChat / Third-party Service)            │
└────────────┬─────────────────────┬──────────────────────────────┘
             │                     │
             ▼                     ▼
    ┌─────────────────┐   ┌──────────────────┐
    │  REST Gateway   │   │   gRPC Gateway   │
    └────────┬────────┘   └────────┬─────────┘
             │                     │
             └────────────┬────────┘
                          │
         ┌────────────────▼────────────────┐
         │      认证模块 (AuthN)           │
         │ • 登录/刷新  • Token管理        │
         │ • 会话校验  • JWKS发布          │
         └────────────┬────────────────────┘
                      │
         ┌────────────┴────────────┐
         │                         │
         ▼                         ▼
    ┌─────────────┐         ┌──────────────┐
    │ 授权模块    │         │ 身份模块      │
    │ (AuthZ)     │         │ (Identity)    │
    │ • 权限判定  │         │ • 用户档案    │
    │ • 策略评估  │         │ • 儿童档案    │
    │ • 快照管理  │         │ • ProfileLink │
    └─────────────┘         └──────────────┘
         │                         │
         │     ┌───────────────────┤
         │     │                   │
         ▼     ▼                   ▼
    ┌─────────────────────────────────────┐
    │  IDP模块      Suggest模块           │
    │  (第三方认证)  (联想搜索)            │
    │  • 微信集成   • 档案搜索             │
    │  • 应用配置   • 拼音搜索             │
    └─────────────┬───────────────────────┘
                  │
    ┌─────────────▼──────────────────┐
    │   CacheGovernance (缓存治理)    │
    │ • 只读缓存目录                  │
    │ • 状态管理  • Debug 面板        │
    └─────────────┬──────────────────┘
                  │
    ┌─────────────▼──────────────────────────┐
    │    核心基础设施 (Infrastructure)       │
    │  ┌────────┐ ┌────────┐ ┌────────┐    │
    │  │ MySQL  │ │ Redis  │ │ Casbin │    │
    │  └────────┘ └────────┘ └────────┘    │
    │  ┌────────┐ ┌────────┐ ┌────────┐    │
    │  │  JWT   │ │Outbox  │ │ Event  │    │
    │  └────────┘ └────────┘ └────────┘    │
    └──────────────────────────────────────┘
```

## 软件架构

IAM 遵循六边形架构（Hexagonal Architecture）和 CQRS 模式，按以下层次组织：

| 组件 | 位置 | 职责 |
| ---- | ---- | ---- |
| 进程入口 | [cmd/apiserver/apiserver.go](cmd/apiserver/apiserver.go) | `iam-apiserver` 服务入口 |
| 生命周期管理 | [internal/apiserver/process](internal/apiserver/process) | 配置、资源初始化、容器启动、HTTP/gRPC 启动、优雅关闭 |
| 依赖装配 | [internal/apiserver/container](internal/apiserver/container) | AuthN、AuthZ、User、IDP、Suggest 等模块装配 |
| REST 适配层 | [internal/apiserver/transport/rest](internal/apiserver/transport/rest) | 路由注册、HTTP 处理器、中间件、debug 接口 |
| gRPC 适配层 | [internal/apiserver/transport/grpc](internal/apiserver/transport/grpc) | Proto 服务注册、transport mapper |
| 应用层 | [internal/apiserver/application](internal/apiserver/application) | 业务用例实现 |
| 领域层 | [internal/apiserver/domain](internal/apiserver/domain) | 领域模型与业务规则 |
| 基础设施层 | [internal/apiserver/infra](internal/apiserver/infra) | MySQL、Redis、Casbin、JWT、Outbox 等实现 |

### 系统层级架构

```ASCII
┌─────────────────────────────────────────────────────────────────┐
│                        外部系统与客户端                            │
│  (Web / Mobile App / 微信 / 第三方应用 / 内部服务)                 │
└────────────┬────────────────────────────────┬────────────────────┘
             │                                │
    ┌────────▼────────┐            ┌──────────▼──────────┐
    │   REST API      │            │   gRPC API         │
    │  (HTTP/HTTPS)   │            │  (mTLS)            │
    └────────┬────────┘            └──────────┬──────────┘
             │                                │
    ┌────────▼──────────────────────────────▼────────┐
    │          HTTP 中间件层                         │
    │  (认证、日志、限流、追踪)                      │
    └────────┬──────────────────────────────────────┘
             │
    ┌────────▼──────────────────────────────────────┐
    │          应用层 (Use Cases)                    │
    │  ┌──────────┐ ┌──────────┐ ┌──────────┐       │
    │  │ AuthN    │ │ AuthZ    │ │ Identity │ ...   │
    │  └──────────┘ └──────────┘ └──────────┘       │
    └────────┬──────────────────────────────────────┘
             │
    ┌────────▼──────────────────────────────────────┐
    │          领域层 (Domain Models)                │
    │  • User / Role / Resource / Policy             │
    │  • Token / Session / ProfileLink               │
    │  • Domain Events                              │
    └────────┬──────────────────────────────────────┘
             │
    ┌────────▼──────────────────────────────────────┐
    │      基础设施层 (Adapters & Ports)             │
    │  ┌──────────┐ ┌──────────┐ ┌──────────┐       │
    │  │ MySQL    │ │ Redis    │ │ Casbin   │ ...   │
    │  └──────────┘ └──────────┘ └──────────┘       │
    └────────┬──────────────────────────────────────┘
             │
    ┌────────▼──────────────────────────────────────┐
    │    外部依赖与存储                              │
    │  (DB / Cache / JWT / Event Queue)             │
    └─────────────────────────────────────────────┘
```

### 认证授权流程

```ASCII
Client Request
    │
    ├─► [REST Handler / gRPC Interceptor]
    │
    ├─► [HTTP 认证中间件]
    │   • Token 提取
    │   • JWT 验签
    │   • Session 校验
    │
    ├─► [AuthN 应用服务]
    │   • 登录/刷新 Token
    │   • 会话管理
    │   • Token 撤销
    │
    ├─► [AuthZ 应用服务]
    │   • 权限判定
    │   • 策略评估
    │   • 授权快照
    │
    └─► [响应]
        • Success ✓
        • Forbidden ✗
        • Unauthorized ✗
```

### 核心数据模型

```ASCII
User (用户)
├── ID
├── Name / Email
├── Credentials
└── Metadata
    │
    ├──► Role (角色)
    │    ├── ID
    │    ├── Name
    │    └── Permissions
    │         │
    │         └──► Resource (资源)
    │              ├── ID
    │              ├── Type
    │              └── Owner
    │
    ├──► Token (令牌)
    │    ├── AccessToken
    │    ├── RefreshToken
    │    └── ExpiresAt
    │
    ├──► Session (会话)
    │    ├── ID
    │    ├── CreatedAt
    │    └── Status
    │
    ├──► ProfileLink (档案关系)
    │    ├── UserID
    │    ├── ProfileID (儿童档案)
    │    └── Relationship
    │
    └──► IDPIdentity (第三方身份)
         ├── Provider (微信/等)
         ├── ExternalID
         └── Metadata
```

## 快速开始

### 依赖检查

确保本机已安装以下依赖：

- **Go**：1.25+（建议使用 1.25.9 或以上）
- **Make**：GNU make 工具
- **Docker**（可选）：运行 `make api-validate` 需要 Docker daemon
- **MySQL**：数据持久化
- **Redis**：缓存存储

### 构建

```bash
# 下载依赖
make deps

# 构建项目
make build

# 构建 Docker 镜像（可选）
make docker-build
```

### 运行

```bash
# 本地开发运行
make run APISERVER_CONFIG=configs/apiserver.dev.yaml

# 验证服务可用
curl http://localhost:8080/health
curl http://localhost:8080/.well-known/jwks.json

# 运行测试
make test

# 运行全套检查
make docs-hygiene    # 文档链接检查
make api-validate    # API 契约校验（需要 Docker）
```

### 请求处理流程

```ASCII
┌──────────────────────────────────────────────────────────────────┐
│                     1. 客户端请求                                │
│        HTTP POST /api/v2/authn/login  或  gRPC RPC Call         │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│                  2. 协议适配层 (Transport)                        │
│           ├─ REST: HTTP Router → Handler                        │
│           └─ gRPC: Service Stub → Interceptor                   │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│            3. 中间件链 (Middleware / Interceptor)                │
│         ├─ 日志记录                                              │
│         ├─ 追踪 (Tracing)                                        │
│         ├─ 验证 (Validation)                                     │
│         └─ 速率限制 (Rate Limiting)                              │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│          4. 应用层 (Application Service)                         │
│       ├─ 请求参数验证                                            │
│       ├─ 业务逻辑编排                                            │
│       └─ 领域模型交互                                            │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│          5. 领域层 (Domain Layer)                                │
│       ├─ 领域模型处理                                            │
│       ├─ 业务规则验证                                            │
│       └─ 事件发布                                                │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│      6. 基础设施层 (Infrastructure)                              │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│   │ Repository  │  │   Cache     │  │   Event     │            │
│   │ (DB Query)  │  │  (Redis)    │  │  (Outbox)   │            │
│   └─────────────┘  └─────────────┘  └─────────────┘            │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│          7. 数据持久化 (Persistence)                             │
│       ├─ MySQL: 主数据存储                                       │
│       ├─ Redis: 会话/缓存                                        │
│       └─ Outbox: 事件持久化                                      │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────┐
│          8. 响应返回 (Response)                                  │
│       ├─ 状态码 (200/400/401/403/500)                            │
│       ├─ 响应体 (JSON/Proto)                                     │
│       └─ 错误信息                                                │
└──────────────────────────────────────────────────────────────────┘
```

## 使用指南

### API 协议架构

```ASCII
┌─────────────────────────────────────────────────────────────┐
│                    Client Requests                          │
└────────────┬────────────────────────┬──────────────────────┘
             │                        │
    ┌────────▼────────┐      ┌───────▼──────────┐
    │  REST API       │      │  gRPC API        │
    ├─────────────────┤      ├──────────────────┤
    │ Protocol: HTTP  │      │ Protocol: HTTP/2 │
    │ Format: JSON    │      │ Format: Proto    │
    │ Security: TLS   │      │ Security: mTLS   │
    └────────┬────────┘      └───────┬──────────┘
             │                       │
    ┌────────▼──────────────────────▼──────┐
    │      五大领域服务 (Domain Services)   │
    │                                      │
    │  1️⃣  认证服务 (AuthN)                 │
    │      └─ /api/v2/authn/*              │
    │      └─ iam.authn.v2.Authn           │
    │                                      │
    │  2️⃣  授权服务 (AuthZ)                 │
    │      └─ /api/v2/authz/*              │
    │      └─ iam.authz.v2.Authz           │
    │                                      │
    │  3️⃣  身份服务 (Identity)              │
    │      └─ /api/v2/identity/*           │
    │      └─ iam.identity.v2.Identity     │
    │                                      │
    │  4️⃣  IDP 服务                         │
    │      └─ /api/v2/idp/*                │
    │      └─ iam.idp.v2.IDP               │
    │                                      │
    │  5️⃣  搜索服务 (Suggest)               │
    │      └─ /api/v2/suggest/*            │
    └──────────────────────────────────────┘
         │
         └─► 详见 api/ 目录中的 YAML 和 Proto 文件
```

### API 入门

本项目同时提供 **REST** 和 **gRPC** 两种 API 协议：

**REST 接口**（OpenAPI 3.1 规范）：

- [认证接口 (authn.v2.yaml)](api/rest/authn.v2.yaml)
- [授权接口 (authz.v2.yaml)](api/rest/authz.v2.yaml)
- [身份接口 (identity.v2.yaml)](api/rest/identity.v2.yaml)
- [IDP 集成 (idp.v2.yaml)](api/rest/idp.v2.yaml)
- [联想搜索 (suggest.v2.yaml)](api/rest/suggest.v2.yaml)

**gRPC 接口**（Proto v2 规范）：

- [认证服务 (authn.proto)](api/grpc/iam/authn/v2/authn.proto)
- [授权服务 (authz.proto)](api/grpc/iam/authz/v2/authz.proto)
- [身份服务 (identity.proto)](api/grpc/iam/identity/v2/identity.proto)
- [IDP 服务 (idp.proto)](api/grpc/iam/idp/v2/idp.proto)

API 详细说明见：[api/README.md](api/README.md)、[api/rest/README.md](api/rest/README.md) 和 [api/grpc/README.md](api/grpc/README.md)

### 文档体系结构

```ASCII
docs/
├── README.md (📍 文档导航中心)
│
├── 00-概览 (入门必读)
│   ├── 01-系统架构总览
│   ├── 02-核心概念术语
│   ├── 03-阅读路径&代码组织
│   ├── 04-架构设计与模式导览
│   └── README.md
│
├── 01-运行时 (运行与部署)
│   ├── 01-服务入口&HTTP与模块装配
│   ├── 02-gRPC与mTLS
│   ├── 03-HTTP认证中间件与身份上下文
│   ├── 04-健康检查&debug路由
│   ├── 05-后台任务&优雅关闭
│   └── README.md
│
├── 02-业务域 (功能详解)
│   ├── 01-authn-认证&Token&JWKS
│   ├── 02-authz-角色&策略&资源
│   ├── 03-user-用户&儿童&ProfileLink
│   ├── 04-suggest-儿童联想搜索
│   ├── 05-idp-微信应用与外部身份
│   ├── 06-业务域设计模式地图
│   └── README.md
│
├── 03-接口与集成 (集成指南)
│   ├── 01-REST契约与接入
│   ├── 02-gRPC契约与接入
│   ├── 03-授权接入与边界
│   ├── 04-身份接入与ProfileLink边界
│   ├── 05-QS接入IAM
│   ├── 06-IAM-QS竖切边界-Token与授权快照
│   └── README.md
│
├── 04-基础设施与运维 (架构与运维)
│   ├── 01-六边形架构实践
│   ├── 02-CQRS模式实践
│   ├── 03-命令&契约校验与开发流程
│   ├── 04-端口&证书与数据库迁移
│   ├── ... (其他运维内容)
│   └── README.md
│
├── 05-专题分析 (深度分析)
│   ├── 认证流程分析
│   ├── 授权流程分析
│   ├── 缓存治理深潜
│   ├── SDK使用指南
│   └── ... (其他专题)
│
└── _archive (历史文档 ⚠️ 非权威)
    ├── 08-IAM代码质量基线报告
    ├── CODE_EXPLORATION_REPORT
    ├── CODE_NAVIGATION_GUIDE
    └── rest-v1/ (已废弃的 API 版本)
```

### 文档导航

完整文档体系位于 [docs/](docs/) 目录，推荐按以下顺序阅读：

1. **[docs/00-概览](docs/00-概览)**：系统架构总览、核心概念、代码组织结构
2. **[docs/01-运行时](docs/01-运行时)**：服务启动、HTTP/gRPC、认证链路、健康检查
3. **[docs/02-业务域](docs/02-业务域)**：各功能模块深度解析（认证、授权、用户、建议等）
4. **[docs/03-接口与集成](docs/03-接口与集成)**：API 集成指南和上下游系统边界
5. **[docs/04-基础设施与运维](docs/04-基础设施与运维)**：架构实践、数据库迁移、Outbox 发布
6. **[docs/05-专题分析](docs/05-专题分析)**：认证流程、授权流程、缓存治理、SDK 使用

详见：[docs/README.md](docs/README.md)

### SDK 接入

Go 应用可通过官方 SDK 集成 IAM 服务：

```go
import "github.com/FangcunMount/iam/v2/pkg/sdk"
// 详见 pkg/sdk 目录
```

## 如何贡献

我们欢迎社区贡献！提交代码时，请遵循以下约定：

### 开发流程

1. **创建特性分支**：从 main 分支创建新的特性分支
2. **本地测试**：运行 `make test` 和 `make docs-hygiene` 确保代码质量
3. **API 变更验证**：如涉及接口变更，运行 `make api-validate`
4. **文档同步**：代码变更涉及以下内容时，必须同步更新文档：
   - 新增或修改路由
   - 新增或修改 Proto 定义
   - OpenAPI 契约变更
   - 配置项变更
   - 架构边界调整
5. **提交审核**：发起 Pull Request 并通过代码审核

### 文档编写规范

文档编写规范详见：[docs/CONTRIBUTING-DOCS.md](docs/CONTRIBUTING-DOCS.md)

关键点：

- 活跃文档应参考源码与运行时行为
- 不可依赖 `docs/_archive` 中的历史文档
- 优先级顺序：源码 > 机器契约 > 活跃文档 > 历史文档

## 许可证

本项目采用 **Apache License 2.0** 开源许可证，详见 [LICENSE](LICENSE) 文件。
