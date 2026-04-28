# 基础设施与运维

本文回答：`iam` 里哪些内容属于技术底座、哪些属于运维交付，这一层现在应该从哪几篇读起，以及它和 `运行时 / 业务域 / 接口与集成` 如何分工。

## 30 秒结论

- `04-基础设施与运维` 现在承接两类内容：一类是六边形架构、CQRS 这类横切技术机制；另一类是 `Makefile`、端口、证书、迁移、Docker、SQL bootstrap 这类交付与维护入口。
- 现行主路径已经切到这一层；旧 `docs/04-基础设施/` 和 `docs/ops/` 只保留为历史资料并归入 `_archive/`。
- 这组文档不重复解释业务域流程，也不替代 `api/` 契约；它更关心“代码为什么这样分层”“系统如何被启动、校验、部署、迁移”。
- 如果只先读 3 篇，建议按这个顺序：`六边形架构实践 -> 命令、契约校验与开发流程 -> 端口、证书与数据库迁移`。

## 重点速查

| 想回答的问题 | 先打开哪里 |
| ---- | ---- |
| 六边形架构在当前代码里怎么落地？ | [01-六边形架构实践.md](./01-六边形架构实践.md) |
| CQRS 在当前代码里怎么落地？ | [02-CQRS模式实践.md](./02-CQRS模式实践.md) |
| 构建、运行、测试、swagger、proto、OpenAPI 校验怎么做？ | [03-命令&契约校验与开发流程.md](./03-命令&契约校验与开发流程.md) |
| 端口、证书、Docker、数据库迁移从哪看？ | [04-端口&证书与数据库迁移.md](./04-端口&证书与数据库迁移.md) |
| 系统 bootstrap 基线数据现在如何管理？ | [05-SQL Bootstrap 与初始化数据.md](./05-SQL Bootstrap 与初始化数据.md) |
| IAM 缓存层今天到底怎么设计、治理面已经做到哪里？ | [../05-专题分析/05-IAM缓存层--缓存层的设计与治理.md](../05-专题分析/05-IAM缓存层--缓存层的设计与治理.md) |
| IAM 当前各个 cache family 为什么大多还是 Redis `String`，`revoked_access_token` 为什么不是 `Set`？ | [../05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md](../05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md) |
| 真实配置和工件入口在哪里？ | [../../configs/](../../configs/)、[../../build/docker/](../../build/docker/)、[../../scripts/](../../scripts/)、[../../Makefile](../../Makefile) |

## 当前文档

| 文档 | 说明 |
| ---- | ---- |
| [01-六边形架构实践.md](./01-六边形架构实践.md) | interface / application / domain / infra 的真实分层与装配 |
| [02-CQRS模式实践.md](./02-CQRS模式实践.md) | 当前 CQRS 的真实形态与读写边界 |
| [03-命令&契约校验与开发流程.md](./03-命令&契约校验与开发流程.md) | `Makefile`、swagger / OpenAPI / proto 校验链、开发命令面 |
| [04-端口&证书与数据库迁移.md](./04-端口&证书与数据库迁移.md) | dev/prod 端口、mTLS 证书、Docker 与 migration 入口 |
| [05-SQL Bootstrap 与初始化数据.md](./05-SQL Bootstrap 与初始化数据.md) | schema.sql、bootstrap.sql 与 migration 的职责分工 |
补充专题：

- [../05-专题分析/05-IAM缓存层--缓存层的设计与治理.md](../05-专题分析/05-IAM缓存层--缓存层的设计与治理.md)
  - 解释 IAM Cache Layer、family 分工、只读治理面与运行边界
- [../05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md](../05-专题分析/06-IAM缓存层--数据结构选择与 Redis 建模判断.md)
  - 专门回答“为什么当前 family 大多仍然是 `String`、为什么 `revoked_access_token` 不是 `Set`、为什么 session index 使用 `ZSet`”

## 与其他层的分工

| 层 | 负责什么 |
| ---- | ---- |
| `01-运行时` | 运行时进程、gRPC、mTLS、健康检查 |
| `02-业务域` | 认证 / 授权 / 用户等业务能力边界 |
| `03-接口与集成` | REST / gRPC 契约解释层与接入边界 |
| `04-基础设施与运维` | 通用技术模式、配置、部署、迁移、排障入口 |

## 真实入口

| 类型 | 位置 | 说明 |
| ---- | ---- | ---- |
| 命令入口 | [../../Makefile](../../Makefile) | 构建、运行、测试、校验、Docker、数据库工具 |
| 运行配置 | [../../configs/apiserver.dev.yaml](../../configs/apiserver.dev.yaml)、[../../configs/apiserver.prod.yaml](../../configs/apiserver.prod.yaml)、[../../configs/grpc_acl.yaml](../../configs/grpc_acl.yaml) | 端口、TLS、mTLS、ACL |
| 数据库迁移 | [../../internal/pkg/migration/](../../internal/pkg/migration/) | migration 实现、README、SQL 文件 |
| Docker 工件 | [../../build/docker/README.md](../../build/docker/README.md)、[../../build/docker/](../../build/docker/) | Dockerfile、compose、部署说明 |
| 开发脚本 | [../../scripts/](../../scripts/) | OpenAPI 校验、proto 生成、证书说明等 |

## 当前约定

1. 现行正文统一写在 `docs/04-基础设施与运维/`。
2. 旧 `docs/04-基础设施/` 和 `docs/ops/` 只作为历史资料保留在 `_archive/`。
3. 这层默认采用：
   `本文回答 -> 30 秒结论 -> 重点速查 -> 当前实现 -> 当前边界`
