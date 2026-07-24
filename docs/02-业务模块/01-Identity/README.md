# Identity

> 状态：已实现 · 已按当前 domain、application、MySQL、REST、gRPC、container 和测试核对。

## 本目录回答

Identity 负责维护 IAM 的身份事实：

```text
User        IAM 内部身份主体
Profile     业务档案或被服务对象
ProfileLink User 与 Profile 的关系事实
```

Identity 不负责登录认证、令牌签发、权限判定、外部身份源配置或联想搜索索引。

## 30 秒结论

- `User`、`Profile`、`ProfileLink` 是三个独立模型。
- Phone 当前可选；非空时由 application 预检查，并由数据库活跃手机号唯一索引兜底。
- 对外 `CreateProfile` 会在同一事务中同时创建 `Profile` 和 `ProfileLink`。
- REST 目前只提供 Identity 查询和更新；创建、关系建立/撤销、批量导入等写能力由 gRPC 提供。
- `ProfileLink.Revoke` 是实体级幂等；公开撤销接口再次撤销已撤销关系会返回错误。
- User 被 `Block` 或 `Deactivate` 后，会在同一 MySQL 事务写入本地安全 Outbox，并最终幂等撤销 AuthN Session。
- Suggest 当前通过 Full/Delta SQL 读取 Identity 表构建索引，不是 Profile 事件订阅。

## 文档结构

本目录包含 6 篇主文档：

| 文档 | 唯一职责 |
| --- | --- |
| [00-模块总览.md](00-模块总览.md) | 职责、能力矩阵和当前边界 |
| [01-领域模型-User-Profile-ProfileLink.md](01-领域模型-User-Profile-ProfileLink.md) | 模型字段、状态和持久化不变量 |
| [02-关键链路-创建User与Profile.md](02-关键链路-创建User与Profile.md) | `CreateUser` 与组合建档真实链路 |
| [03-关键链路-建立与撤销ProfileLink.md](03-关键链路-建立与撤销ProfileLink.md) | 建立、查询、撤销、批量和历史约束 |
| [04-模块边界-Identity与AuthN-AuthZ-Suggest.md](04-模块边界-Identity与AuthN-AuthZ-Suggest.md) | 已实现的跨模块协作和未实现能力 |
| [05-分层架构与代码索引.md](05-分层架构与代码索引.md) | 高价值代码入口、契约和验证命令 |

## 按任务阅读

| 任务 | 先读 |
| --- | --- |
| 修改模型字段或不变量 | [01-领域模型](01-领域模型-User-Profile-ProfileLink.md) |
| 修改创建用户或建档流程 | [02-创建链路](02-关键链路-创建User与Profile.md) |
| 修改关系建立、撤销或查询 | [03-ProfileLink 链路](03-关键链路-建立与撤销ProfileLink.md) |
| 判断跨模块依赖是否合理 | [04-模块边界](04-模块边界-Identity与AuthN-AuthZ-Suggest.md) |
| 定位代码、契约或测试 | [05-代码索引](05-分层架构与代码索引.md) |

## 事实源

发生冲突时按以下顺序判断：

1. `internal/apiserver/domain/identity` 与 `internal/apiserver/application/identity`
2. `internal/apiserver/infra/mysql/{user,profile,profilelink}` 与 migration
3. `api/rest/identity.v2.yaml`、`api/grpc/iam/identity/v2/identity.proto`
4. transport、container 和测试
5. 本目录文档

全局写作与验证规则见 [CONTRIBUTING-DOCS.md](../../CONTRIBUTING-DOCS.md)。
