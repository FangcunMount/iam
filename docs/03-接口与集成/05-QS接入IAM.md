# QS 接入 IAM

本文回答：`qs` 这类业务系统今天应该怎样接入 `iam`，什么时候优先用 SDK，什么时候直接看 REST / gRPC 合同，以及接入时最容易讲过头的边界有哪些。

## 30 秒结论

- 当前更推荐的接法是：`SDK + gRPC`，再按一致性要求二选一：
  - 高频、可接受最终一致撤销语义：`本地 JWKS 验签`
  - 要求 session revoke / 用户封禁 / 账号禁用即时生效：`IAM 在线 VerifyToken`
- 如果场景是“网关验证用户 JWT”，优先看 `pkg/sdk` 里的 `TokenVerifier` 和 `JWKS` 配置；如果场景是“后端按 ID 读用户 / 判定监护关系”，优先看 gRPC 与 SDK 的 `Identity()` / `Ref()` 客户端。
- `docs/03-接口与集成` 负责说明“怎么接”；真正的字段、服务和错误语义仍以 `api/rest/*.yaml`、`api/grpc/**/*.proto`、`pkg/sdk/docs/*` 为准。
- 授权：`authz` 已包含管理面与单次 PDP（REST `POST /authz/check`、gRPC `AuthorizationService/Check`、SDK `Authz()`）；批量/Explain/菜单仍通常需业务侧扩展，见 [03-授权接入与边界.md](./03-授权接入与边界.md)。
- 旧版《QS 接入 IAM 实践指南》已经归档到 [../_archive/00-概览/04-qs接入iam指南.md](../_archive/00-概览/04-qs接入iam指南.md)；现行接入口径以本文和关联文档为准。

## 重点速查

| 场景 | 当前更推荐的入口 | 真实落点 |
| ---- | ---- | ---- |
| 先判断 SDK 是否能直接承载场景 | 先看 SDK 价值说明 | [../05-专题分析/07-SDK封装与接入价值.md](../05-专题分析/07-SDK封装与接入价值.md) |
| 网关或 BFF 校验用户 JWT（最终一致即可） | SDK `TokenVerifier` + JWKS | [../../pkg/sdk/docs/04-jwt-verification.md](../../pkg/sdk/docs/04-jwt-verification.md)、[../../pkg/sdk/_examples/verifier/main.go](../../pkg/sdk/_examples/verifier/main.go) |
| 需要 access revoke / session revoke / 用户封禁即时生效 | 在线 `VerifyToken`（SDK / gRPC） | [02-gRPC契约与接入.md](./02-gRPC契约与接入.md)、[../../api/grpc/iam/authn/v1/authn.proto](../../api/grpc/iam/authn/v1/authn.proto) |
| 后端服务读取用户 / 儿童 / 监护关系 | SDK `Identity()` / `Ref()` | [../../pkg/sdk/docs/01-quick-start.md](../../pkg/sdk/docs/01-quick-start.md)、[../../api/grpc/iam/identity/v1/identity.proto](../../api/grpc/iam/identity/v1/identity.proto) |
| 服务间获取服务 Token | SDK `ServiceAuthHelper` | [../../pkg/sdk/docs/05-service-auth.md](../../pkg/sdk/docs/05-service-auth.md)、[../../api/grpc/iam/authn/v1/authn.proto](../../api/grpc/iam/authn/v1/authn.proto) |
| 查看 gRPC 合同与 metadata | gRPC 契约解释层 | [02-gRPC契约与接入.md](./02-gRPC契约与接入.md)、[../../api/grpc/README.md](../../api/grpc/README.md) |
| 查看 REST 路径与公开 JWKS | REST 契约解释层 | [01-REST契约与接入.md](./01-REST契约与接入.md)、[../../api/rest/authn.v1.yaml](../../api/rest/authn.v1.yaml) |
| 身份与监护边界 | 身份接入边界文档 | [04-身份接入与监护关系边界.md](./04-身份接入与监护关系边界.md) |
| mTLS、ACL、证书与健康检查 | 运行时 / 运维层 | [../01-运行时/02-gRPC与mTLS.md](../01-运行时/02-gRPC与mTLS.md)、[../04-基础设施与运维/04-端口&证书与数据库迁移.md](../04-基础设施与运维/04-端口&证书与数据库迁移.md) |

## 1. 先选接入路径，不要一上来就翻所有合同

对 `qs` 这类业务系统，今天最实用的接入路径可以先拆成 3 类：

| 路径 | 适合什么场景 | 当前建议 |
| ---- | ---- | ---- |
| 本地 JWT 验签 | 网关、BFF、高频认证校验 | 优先用 SDK 的 `TokenVerifier` + JWKS |
| gRPC 服务调用 | 后端服务查用户、查儿童、判监护关系、取服务 Token | 优先用 SDK 包装的 gRPC 客户端 |
| REST 调用 | 公开 JWKS、人工调试、前端/BFF 直接接登录类接口 | 只在确实需要 REST 语义时使用 |

一个更贴近当前代码的理解是：

1. 用户态请求进入 `qs` 时，先解决“这个 JWT 是否可信”。
2. 需要身份明细、儿童信息、监护关系时，再走 IAM 的 gRPC 能力。
3. 只有在必须走公开 HTTP 合同的地方，再回到 REST。

如果你还在判断“SDK 到底是不是应该优先看的主入口”，先读 [../05-专题分析/07-SDK封装与接入价值.md](../05-专题分析/07-SDK封装与接入价值.md)。

## 2. 当前更推荐的接法

### 2.1 用户 JWT：先按一致性要求选“本地验签”还是“在线权威校验”

SDK 已经提供本地验签能力：

- `TokenVerifier`
- `JWKSConfig`
- `JWKS` 拉取失败时的降级策略

对应资料：

- [../../pkg/sdk/docs/04-jwt-verification.md](../../pkg/sdk/docs/04-jwt-verification.md)
- [../../pkg/sdk/_examples/verifier/main.go](../../pkg/sdk/_examples/verifier/main.go)
- [../../api/rest/authn.v1.yaml](../../api/rest/authn.v1.yaml) 中的 `/.well-known/jwks.json`

但这条路径只能保证：

- 签名合法
- `exp / nbf` 时间合法

它**不能保证即时生效**：

- `revoked_access_token`
- `session revoke`
- `user blocked`
- `account disabled`

因此当前更准确的接入口径是：

| 场景 | 当前建议 |
| ---- | ---- |
| 高频请求链路，能接受最终一致撤销语义 | 优先本地 JWKS 验签 |
| 需要管理员踢人、踢会话、禁用账号立即生效 | 明确接入 IAM 在线 `VerifyToken` |

不要再把“所有网关场景都优先本地验签”讲成无条件推荐。

### 2.2 身份与监护查询：优先 SDK + gRPC

SDK 当前已经把最常用的服务收成统一入口：

- `client.Auth()`
- `client.Identity()`
- `client.Ref()`

对应资料：

- [../../pkg/sdk/docs/01-quick-start.md](../../pkg/sdk/docs/01-quick-start.md)
- [../../api/grpc/README.md](../../api/grpc/README.md)
- [02-gRPC契约与接入.md](./02-gRPC契约与接入.md)

对 `qs` 来说，最常见的几个动作是：

| 动作 | 当前更合适的能力 |
| ---- | ---- |
| 按 `user_id` 查用户 | `Identity().GetUser(...)` |
| 判定某用户是否是某孩子监护人 | `Ref().IsRef(...)` |
| 列出某监护人的孩子 | `Ref().ListProfiles(...)` |
| 列出某孩子的监护人 | `Ref().ListRefs(...)` |

这比直接对着 `proto` 或手写 gRPC client 更适合业务接入层。

### 2.3 服务间认证：优先 `ServiceAuthHelper`

如果 `qs` 的后端服务需要稳定调用 IAM 或其它内部 gRPC 服务，当前优先看：

- [../../pkg/sdk/docs/05-service-auth.md](../../pkg/sdk/docs/05-service-auth.md)
- [../../pkg/sdk/_examples/service_auth/main.go](../../pkg/sdk/_examples/service_auth/main.go)

当前可证明的能力包括：

- 自动申请服务 Token
- 提前刷新
- 重试 / 退避
- 熔断与降级

因此比起手工管理 `IssueServiceToken` 生命周期，现阶段更推荐直接复用 `ServiceAuthHelper`。

## 3. 接入时真正要看的事实入口

| 关注面 | 先看哪里 | 不要只看哪里 |
| ---- | ---- | ---- |
| SDK 用法 | `pkg/sdk/docs/*`、`pkg/sdk/_examples/*` | 旧版概览长文 |
| REST 路径与公开 HTTP 能力 | `api/rest/*.yaml` + [01-REST契约与接入.md](./01-REST契约与接入.md) | 只看 `api/rest/README.md` 的历史示例 |
| gRPC 服务矩阵与 metadata | `api/grpc/README.md` + [02-gRPC契约与接入.md](./02-gRPC契约与接入.md) | 只看某个 `proto` 是否“定义了 service” |
| 身份 / 监护边界 | [04-身份接入与监护关系边界.md](./04-身份接入与监护关系边界.md) | 自己猜 REST 与 gRPC 是完全等价的 |
| 运行时安全 | [../01-运行时/02-gRPC与mTLS.md](../01-运行时/02-gRPC与mTLS.md) | 旧版 gRPC 混合长文 |

## 4. 当前不要讲过头的几件事

### 4.1 不要把 `authz` 讲成「全家桶」授权中心

当前 `authz` 已包含：**管理面** + **单次 PDP**（REST `POST /authz/check`、gRPC `AuthorizationService/Check`、SDK `Authz()`）。  
仍不宜讲过头的是：批量判定、Explain、与前端菜单强绑定的默认方案，或「任意业务路由已自动鉴权」。相关边界见：

- [03-授权接入与边界.md](./03-授权接入与边界.md)
- [../02-业务域/02-authz-角色&策略&资源&Assignment.md](../02-业务域/02-authz-角色&策略&资源&Assignment.md)

因此今天不要把它讲成：

- IAM 已覆盖业务侧所需的全部授权产品能力（仍可能需自建批量与 UX）
- 「所有 HTTP 路径都已挂 `RequireRole`/`RequirePermission`」（需业务显式挂载中间件）

### 4.2 不要把身份 REST 和 gRPC 当成两套完全对称的壳

当前更准确的说法是：

- REST 更偏“当前登录用户上下文”
- gRPC 更偏“服务间显式 ID 查询 / 判定”

细节见：

- [04-身份接入与监护关系边界.md](./04-身份接入与监护关系边界.md)

### 4.3 不要把证书、ACL、端口细节抄进业务接入说明里

这些内容已经有专门现行文档：

- [../01-运行时/02-gRPC与mTLS.md](../01-运行时/02-gRPC与mTLS.md)
- [../04-基础设施与运维/04-端口&证书与数据库迁移.md](../04-基础设施与运维/04-端口&证书与数据库迁移.md)

`QS 接入 IAM` 这一篇只需要告诉读者“该去哪看”，不应再维护第二份证书操作手册。

## 5. 一条更贴近当前实现的推荐阅读路径

如果你现在就要让 `qs` 接 IAM，建议按这个顺序读：

1. [../../pkg/sdk/docs/01-quick-start.md](../../pkg/sdk/docs/01-quick-start.md)
2. [../../pkg/sdk/docs/04-jwt-verification.md](../../pkg/sdk/docs/04-jwt-verification.md)
3. [../../pkg/sdk/docs/05-service-auth.md](../../pkg/sdk/docs/05-service-auth.md)
4. [02-gRPC契约与接入.md](./02-gRPC契约与接入.md)
5. [04-身份接入与监护关系边界.md](./04-身份接入与监护关系边界.md)
6. [../01-运行时/02-gRPC与mTLS.md](../01-运行时/02-gRPC与mTLS.md)

如果只想先抓住最短主线，可以压缩成：

1. 先决定 JWT 是本地验签还是远程校验
2. 后端身份查询统一走 SDK + gRPC
3. 服务间认证直接复用 `ServiceAuthHelper`

## 6. 继续往下读

| 文档 | 说明 |
| ---- | ---- |
| [README.md](./README.md) | 接口与集成层入口 |
| [../05-专题分析/07-SDK封装与接入价值.md](../05-专题分析/07-SDK封装与接入价值.md) | SDK 的主轴定位、封装价值与当前边界 |
| [01-REST契约与接入.md](./01-REST契约与接入.md) | REST 路径、公开 JWKS、OpenAPI 校验链 |
| [02-gRPC契约与接入.md](./02-gRPC契约与接入.md) | gRPC 服务矩阵、metadata、生成与调试 |
| [03-授权接入与边界.md](./03-授权接入与边界.md) | 授权能力当前能接到什么程度 |
| [04-身份接入与监护关系边界.md](./04-身份接入与监护关系边界.md) | 身份 / 监护能力的真实边界 |
| [06-IAM-QS竖切边界-Token与授权快照.md](./06-IAM-QS竖切边界-Token与授权快照.md) | Token、`GetAuthorizationSnapshot`、`authz_version` 与 QS `CurrentAuthzSnapshot` / capability 的竖切边界 |
| [../../pkg/sdk/docs/README.md](../../pkg/sdk/docs/README.md) | SDK 文档总入口 |
| [../_archive/00-概览/04-qs接入iam指南.md](../_archive/00-概览/04-qs接入iam指南.md) | 旧版长稿原文，仅作历史资料 |
