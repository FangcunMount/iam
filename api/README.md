# IAM API 契约

`api/` 是 IAM 的机器契约层。字段、路径、RPC、兼容性和错误语义以这里的 OpenAPI 与 proto 为准；`docs/` 只解释设计和接入方式。

## 目录

```text
api/
├── rest/
│   ├── authn.v1.yaml
│   ├── authn.v2.yaml
│   ├── authz.v1.yaml
│   ├── identity.v1.yaml
│   ├── idp.v1.yaml
│   └── suggest.v1.yaml
└── grpc/
    └── iam/{authn,authz,identity,idp}/v1/*.proto
```

## REST 能力

| 契约 | 当前能力 |
| ---- | ---- |
| [rest/authn.v1.yaml](rest/authn.v1.yaml) | v1 登录、登录准备、刷新、登出、验证、JWKS、账户和 signup |
| [rest/authn.v2.yaml](rest/authn.v2.yaml) | 使用 `auth_method + method_payload` 的显式登录契约 |
| [rest/authz.v1.yaml](rest/authz.v1.yaml) | 授权健康检查、判定、角色、assignment、策略、资源管理 |
| [rest/identity.v1.yaml](rest/identity.v1.yaml) | 当前用户、profiles、profile-links |
| [rest/idp.v1.yaml](rest/idp.v1.yaml) | IDP 健康检查和微信应用管理 |
| [rest/suggest.v1.yaml](rest/suggest.v1.yaml) | 儿童档案联想搜索 |

常用端点示例：

```text
POST /api/v1/authn/login
POST /api/v1/authn/refresh_token
POST /api/v1/authn/logout
GET  /.well-known/jwks.json
POST /api/v1/authz/check
GET  /api/v1/identity/me
POST /api/v1/identity/profiles
GET  /api/v1/identity/profile-links
GET  /api/v1/suggest/profile
```

实际注册位置在 [internal/apiserver/transport/rest](../internal/apiserver/transport/rest)，路由矩阵由 [internal/apiserver/transport/rest/router_matrix_test.go](../internal/apiserver/transport/rest/router_matrix_test.go) 保护。

## gRPC 能力

| 契约 | 服务 |
| ---- | ---- |
| [grpc/iam/authn/v1/authn.proto](grpc/iam/authn/v1/authn.proto) | `AuthService`、`AccountOnboardingService`、`JWKSService` |
| [grpc/iam/authz/v1/authz.proto](grpc/iam/authz/v1/authz.proto) | `AuthorizationService` |
| [grpc/iam/identity/v1/identity.proto](grpc/iam/identity/v1/identity.proto) | `IdentityRead`、`ProfileLinkQuery`、`ProfileLinkCommand`、`IdentityLifecycle` |
| [grpc/iam/idp/v1/idp.proto](grpc/iam/idp/v1/idp.proto) | `IDPService` |

服务注册位置在 [internal/apiserver/transport/grpc/registry.go](../internal/apiserver/transport/grpc/registry.go)，proto 与注册关系由 [internal/apiserver/transport/grpc/proto_contract_test.go](../internal/apiserver/transport/grpc/proto_contract_test.go) 保护。

## 安全约定

- REST 受保护路由使用 `Authorization: Bearer <JWT>`；公开面包括健康检查、登录、JWKS 和部分 public/info 路由。
- gRPC 由 `process` 层配置 mTLS、service token、ACL 和 audit interceptor。
- 离线 JWKS 验签只证明签名、issuer/audience 和过期时间；撤销、会话、用户或账号状态以在线 Verify 能力为准。

## 验证

```bash
make api-validate
go test ./internal/apiserver/transport/rest ./internal/apiserver/transport/grpc
```

`make api-validate` 需要 Docker daemon；它会运行 [scripts/validate-openapi.sh](../scripts/validate-openapi.sh)、[scripts/check-openapi-contracts.py](../scripts/check-openapi-contracts.py) 和 [scripts/check-route-contracts.py](../scripts/check-route-contracts.py)。
