# IAM

IAM 是方寸山项目的身份与访问管理服务，当前运行单元是 `iam-apiserver`。它提供认证、Token/JWKS、授权判定、角色与策略管理、用户档案、ProfileLink 档案关系、IDP 集成和 SDK 接入能力。

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 当前事实源

| 类型 | 位置 | 说明 |
| ---- | ---- | ---- |
| 运行入口 | [cmd/apiserver/apiserver.go](cmd/apiserver/apiserver.go) | `iam-apiserver` 进程入口 |
| 生命周期 | [internal/apiserver/process](internal/apiserver/process) | 配置、资源、容器、HTTP/gRPC、后台任务与关闭流程 |
| 组合根 | [internal/apiserver/container](internal/apiserver/container) | AuthN、AuthZ、User、IDP、Suggest、CacheGovernance 模块装配 |
| REST 适配 | [internal/apiserver/transport/rest](internal/apiserver/transport/rest) | 路由注册、HTTP handler、中间件与 debug 面 |
| gRPC 适配 | [internal/apiserver/transport/grpc](internal/apiserver/transport/grpc) | proto 服务注册与 transport mapper |
| 应用/领域/基础设施 | [internal/apiserver/application](internal/apiserver/application)、[internal/apiserver/domain](internal/apiserver/domain)、[internal/apiserver/infra](internal/apiserver/infra) | 用例、领域模型、MySQL/Redis/Casbin/JWT/Outbox 等实现 |
| REST 契约 | [api/rest](api/rest) | OpenAPI 3.1 契约 |
| gRPC 契约 | [api/grpc](api/grpc) | Proto v1 契约与生成代码 |
| SDK | [pkg/sdk](pkg/sdk) | Go SDK、服务认证、JWT 验签、AuthZ 与 Identity 客户端 |

冲突时优先级是：源码与运行时行为、机器契约与配置、当前维护文档、历史文档与归档材料。

## 核心能力

- AuthN：v1/v2 登录、Refresh Token、Access Token 撤销、会话校验、Service Token、JWKS 发布与管理端保护。
- AuthZ：REST 与 gRPC 判定、授权快照、角色/资源/策略/assignment wire term，内部实现以 rolebinding 为准。
- Identity：用户资料、儿童档案、ProfileLink 档案关系查询与写入。
- IDP：微信应用配置与登录所需 IDP 能力。
- Suggest：`GET /api/v1/suggest/profile` 儿童档案联想搜索。
- CacheGovernance：只读缓存目录、family 状态和 debug 治理面。
- Transactional Outbox：授权版本变更等领域事件通过 outbox relay 持久发布。

## 快速开始

```bash
make deps
make build
make test
```

开发配置默认入口：

```bash
make run APISERVER_CONFIG=configs/apiserver.dev.yaml
curl http://localhost:8080/health
curl http://localhost:8080/.well-known/jwks.json
```

常用验证：

```bash
make docs-hygiene
make api-validate
go test ./...
```

`make api-validate` 会运行 Spectral Docker 镜像和本仓库的 OpenAPI/路由契约检查；本机需要 Docker daemon 可用。

## API 入口

REST 契约以 OpenAPI 文件为准：

- [api/rest/authn.v1.yaml](api/rest/authn.v1.yaml)
- [api/rest/authn.v2.yaml](api/rest/authn.v2.yaml)
- [api/rest/authz.v1.yaml](api/rest/authz.v1.yaml)
- [api/rest/identity.v1.yaml](api/rest/identity.v1.yaml)
- [api/rest/idp.v1.yaml](api/rest/idp.v1.yaml)
- [api/rest/suggest.v1.yaml](api/rest/suggest.v1.yaml)

gRPC 契约以 proto 文件为准：

- [api/grpc/iam/authn/v1/authn.proto](api/grpc/iam/authn/v1/authn.proto)
- [api/grpc/iam/authz/v1/authz.proto](api/grpc/iam/authz/v1/authz.proto)
- [api/grpc/iam/identity/v1/identity.proto](api/grpc/iam/identity/v1/identity.proto)
- [api/grpc/iam/idp/v1/idp.proto](api/grpc/iam/idp/v1/idp.proto)

说明性 API 文档见 [api/README.md](api/README.md)、[api/rest/README.md](api/rest/README.md) 和 [api/grpc/README.md](api/grpc/README.md)。

## 文档导航

- [docs/README.md](docs/README.md)：文档中心与阅读路径。
- [docs/00-概览](docs/00-概览)：系统地图、术语、代码组织和事实来源。
- [docs/01-运行时](docs/01-运行时)：启动链、REST/gRPC、HTTP 认证、健康检查和 debug 面。
- [docs/02-业务域](docs/02-业务域)：AuthN、AuthZ、ProfileLink、Suggest。
- [docs/03-接口与集成](docs/03-接口与集成)：REST、gRPC、授权、身份关系和 QS 接入。
- [docs/04-基础设施与运维](docs/04-基础设施与运维)：架构实践、CQRS、契约校验、迁移、Outbox。
- [docs/05-专题分析](docs/05-专题分析)：认证链路、授权链路、缓存治理、SDK 等深潜材料。
- [docs/_archive](docs/_archive)：历史文档，非权威事实层。

## 开发约定

- 代码变更触及路由、proto、OpenAPI、配置、迁移或架构边界时，同步更新文档。
- 活跃文档不得依赖 `docs/_archive` 中的历史事实。
- 提交前至少运行 `make docs-hygiene`；涉及 API 契约时运行 `make api-validate`。
- 文档写作规范见 [docs/CONTRIBUTING-DOCS.md](docs/CONTRIBUTING-DOCS.md)。
