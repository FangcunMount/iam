# 04-基础设施与运维

本文回答：IAM 的基础设施层、运维配置、验证命令、数据库迁移、证书端口和事件投递应该从哪里理解。本组不是业务域深潜，而是给开发、部署、排障和文档维护提供工程事实入口。

## 30 秒结论

- IAM 采用 `transport -> application -> domain` 的业务依赖方向，infra 作为 driven adapters 接入 MySQL、Redis、Casbin、JWT、IDP、Outbox 等外部能力。
- [../../internal/apiserver/container](../../internal/apiserver/container) 是组合根，允许看到多层类型，用来把端口和适配器接起来。
- 运行时配置重点看端口、TLS/mTLS、ACL、migration、outbox relay 和 debug 开关。
- 数据库演进以 migration 为准；`schema.sql` 和 `bootstrap.sql` 是基线材料，不替代迁移事实。
- 文档和合同维护必须跑 `make docs-hygiene`，涉及 REST/proto 时再跑 `make api-validate` 和 `make proto-gen`。

## 文档地图

| 文档 | 说明 | 适合场景 |
| ---- | ---- | ---- |
| [01-六边形架构实践.md](01-六边形架构实践.md) | 当前分层、端口、适配器和依赖方向 | 代码定位、架构评审 |
| [02-CQRS模式实践.md](02-CQRS模式实践.md) | 命令/查询边界和读写分离粒度 | 应用服务设计、接口拆分 |
| [03-命令&契约校验与开发流程.md](03-命令&契约校验与开发流程.md) | Make targets、OpenAPI/proto/docs hygiene | 提交前检查、CI 排障 |
| [04-端口&证书与数据库迁移.md](04-端口&证书与数据库迁移.md) | HTTP/gRPC 端口、mTLS、ACL、migration | 部署和环境配置 |
| [05-SQL Bootstrap 与初始化数据.md](05-SQL%20Bootstrap%20与初始化数据.md) | schema、bootstrap、migration 分工 | 初始化数据和基线维护 |
| [06-UoW与Transactional Outbox.md](06-UoW与Transactional%20Outbox.md) | UoW、事件 stage、outbox relay | 事务一致性和事件投递排障 |

## 快速定位

| 你要做什么 | 先看 |
| ---- | ---- |
| 判断某段代码应该放 application、domain 还是 infra | [01-六边形架构实践.md](01-六边形架构实践.md) |
| 给新命令或查询找落点 | [02-CQRS模式实践.md](02-CQRS模式实践.md) |
| 改 REST/proto 合同 | [03-命令&契约校验与开发流程.md](03-命令&契约校验与开发流程.md) |
| 配 gRPC mTLS 或 ACL | [04-端口&证书与数据库迁移.md](04-端口&证书与数据库迁移.md) |
| 新增表、索引或初始化数据 | [05-SQL Bootstrap 与初始化数据.md](05-SQL%20Bootstrap%20与初始化数据.md) |
| 排查授权版本事件没有投递 | [06-UoW与Transactional Outbox.md](06-UoW与Transactional%20Outbox.md) |

## 本组边界

- 不替代 [../01-运行时](../01-运行时/README.md)：运行时层更详细解释启动、注册、健康检查和 graceful shutdown。
- 不替代 [../02-业务域](../02-业务域/README.md)：业务域层更详细解释 AuthN/AuthZ/Identity/IDP/Suggest。
- 不替代 [../05-专题分析](../05-专题分析/README.md)：专题层更详细解释关键执行链路。
- 不替代机器合同和迁移文件：API 字段以 `api/` 为准，数据库结构以 migration 为准。

## 维护验证

```bash
make docs-hygiene
go test ./internal/pkg/architecture ./internal/apiserver/process ./internal/pkg/migration/... ./internal/apiserver/infra/...
```
