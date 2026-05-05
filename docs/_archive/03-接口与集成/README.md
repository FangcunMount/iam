# 03-接口与集成

本文回答：接入方应该从哪里看 IAM 的 REST/gRPC 合同，如何选择在线校验、离线验签、授权判定、授权快照、Identity/ProfileLink 和 SDK 接入路径，以及哪些语义不能由 IAM 替业务系统代替承诺。

## 30 秒结论

- REST 字段和路由以 [../../api/rest/README.md](../../api/rest/README.md) 为准，gRPC service 和 message 以 [../../api/grpc/README.md](../../api/grpc/README.md) 为准。
- 运行时 adapter 在 [../../internal/apiserver/transport](../../internal/apiserver/transport)，接入文档只解释如何消费合同，不替代机器合同。
- Go 服务优先使用 [../../pkg/sdk/docs/README.md](../../pkg/sdk/docs/README.md)，减少手写 gRPC 拦截器、JWKS 刷新、错误映射和 service auth。
- `assignment` 是 REST/proto 对外 wire term；IAM 内部授权写模型以 `rolebinding` 为准。
- 用户与档案关系当前标准术语是 `ProfileLink`；它可承载亲属/监护类业务语义，但不是完整家庭业务规则引擎。

## 文档地图

| 文档 | 回答的问题 | 适合谁先看 |
| ---- | ---- | ---- |
| [01-REST契约与接入.md](01-REST契约与接入.md) | REST 合同、路由注册、安全边界、验证命令。 | 前端、网关、REST 接入方 |
| [02-gRPC契约与接入.md](02-gRPC契约与接入.md) | gRPC service 矩阵、mTLS/ACL、SDK 接入方式。 | 后端服务、平台服务 |
| [03-授权接入与边界.md](03-授权接入与边界.md) | Online Check、授权快照、assignment/rolebinding 边界。 | 需要接 IAM AuthZ 的业务系统 |
| [04-身份接入与ProfileLink边界.md](04-身份接入与ProfileLink边界.md) | User/Profile/ProfileLink 的 REST/gRPC 接入和边界。 | 需要接用户档案关系的业务系统 |
| [05-QS接入IAM.md](05-QS接入IAM.md) | 以 QS 类系统为例的最小接入路径。 | 新接入 IAM 的业务团队 |
| [06-IAM-QS竖切边界-Token与授权快照.md](06-IAM-QS竖切边界-Token与授权快照.md) | IAM 与业务系统在 token、身份上下文、授权快照上的责任切分。 | 架构评审、联调负责人 |

## 接入决策表

| 目标 | 优先方式 | 什么时候升级 |
| ---- | ---- | ---- |
| 用户登录 | REST AuthN 登录入口 | 多端多登录方式时使用 v2 显式登录 payload。 |
| 请求身份校验 | SDK/JWKS 离线验签 | 需要撤销、session 或账号状态实时语义时调用在线 Verify。 |
| 服务间调用 IAM | gRPC + mTLS + service token | 跨团队服务接入时补 ACL 和 audit 配置。 |
| 单次授权判定 | REST/gRPC AuthZ Check | 高频判定或本地策略需要改用授权快照。 |
| 本地授权缓存 | gRPC 授权快照 + `authz_version` | 收到版本事件或高风险操作时刷新/回源。 |
| 用户档案关系 | Identity/ProfileLink REST 或 gRPC | 涉及资源权限时叠加 AuthZ。 |

## 本组不替代什么

- 不替代 [../02-业务域](../02-业务域/README.md)：业务域讲领域模型和应用服务。
- 不替代 [../05-专题分析](../05-专题分析/README.md)：专题层讲关键链路深潜。
- 不替代 `api/` 机器合同：字段、枚举、required、path 以 OpenAPI/proto 为准。
- 不替代业务系统自己的资源模型：IAM 不替业务系统定义每个业务资源和 action 的完整含义。

## 权威契约

- REST：[../../api/rest/README.md](../../api/rest/README.md)
- gRPC：[../../api/grpc/README.md](../../api/grpc/README.md)
- SDK：[../../pkg/sdk/docs/README.md](../../pkg/sdk/docs/README.md)
- 路由实现：[../../internal/apiserver/transport/rest](../../internal/apiserver/transport/rest)
- gRPC 实现：[../../internal/apiserver/transport/grpc](../../internal/apiserver/transport/grpc)

## 维护验证

```bash
make docs-hygiene
go test ./internal/apiserver/transport/rest ./internal/apiserver/transport/grpc/... ./pkg/sdk/...
```

涉及合同变化时补跑：

```bash
make api-validate
make proto-gen
```
