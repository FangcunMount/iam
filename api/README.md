# IAM API 契约

`api/` 保存 IAM 发布的机器描述：REST OpenAPI、gRPC proto 与生成代码。它们需要与实际注册、mapper、错误出口和消费者共同核对；当前已有安全声明、响应封装及校验覆盖的偏移，不能从文件存在推出实现一致。事实来源、生成方向与修复责任由[契约治理正文](../docs/04-接口与SDK/01-REST-gRPC与契约治理.md)维护。

## 目录

```text
api/
├── rest/
│   ├── authn.v3.yaml
│   ├── authz.v4.yaml
│   ├── identity.v2.yaml
│   ├── idp.v2.yaml
│   └── suggest.v2.yaml
└── grpc/
    ├── iam/authn/v3/*.proto
    ├── iam/{identity,idp}/v2/*.proto
    └── iam/authz/v4/*.proto
```

## REST 能力

| 契约 | 当前能力 |
| ---- | ---- |
| [rest/authn.v3.yaml](rest/authn.v3.yaml) | 使用 `auth_method + method_payload` 的显式登录、登录准备、刷新、登出、验证、JWKS、账户和 signup |
| [rest/authz.v4.yaml](rest/authz.v4.yaml) | PermissionGrant、Role、Assignment、Resource 管理；继承路由返回410，条件/属性字段仅接受合法空兼容形状；不提供授权判定 |
| [rest/identity.v2.yaml](rest/identity.v2.yaml) | 当前用户、profiles、profile-links |
| [rest/idp.v2.yaml](rest/idp.v2.yaml) | IDP 健康检查和微信应用管理 |
| [rest/suggest.v2.yaml](rest/suggest.v2.yaml) | 档案联想搜索；查询范围与手机号能力分别授权 |

常用端点示例：

```text
POST /api/v3/authn/login
POST /api/v3/authn/refresh_token
POST /api/v3/authn/logout
GET  /.well-known/jwks.json
GET /api/v4/authz/roles
POST /api/v4/authz/grants
GET /api/v2/identity/me
GET /api/v2/identity/profile-links
GET /api/v2/suggest/profile
```

AuthZ REST v4 是角色、Assignment、PermissionGrant 与 Resource 的管理面，不提供权限判定端点。可信服务的授权判定使用 gRPC `iam.authz.v4.AuthorizationService/Check`。

实际注册位置在 [internal/apiserver/transport/rest](../internal/apiserver/transport/rest)，路由矩阵由 [internal/apiserver/transport/rest/router_matrix_test.go](../internal/apiserver/transport/rest/router_matrix_test.go) 保护。

## gRPC 能力

| 契约 | 服务 |
| ---- | ---- |
| [grpc/iam/authn/v3/authn.proto](grpc/iam/authn/v3/authn.proto) | `AuthService`、`AuthSignupService`、`AuthChallengeService`、`LoginIdentityService`、`NotificationRecipientService`、`JWKSService` |
| [grpc/iam/authz/v4/authz.proto](grpc/iam/authz/v4/authz.proto) | `AuthorizationService`：Check、授权快照、受限的已提交策略版本读取和 Assignment 服务间写入 |
| [grpc/iam/identity/v2/identity.proto](grpc/iam/identity/v2/identity.proto) | `IdentityRead`、`ProfileLinkQuery`、`ProfileCommand`、`ProfileLinkCommand`、`IdentityLifecycle` |
| [grpc/iam/idp/v2/idp.proto](grpc/iam/idp/v2/idp.proto) | `IDPService` |

服务注册位置在 [internal/apiserver/transport/grpc/registry.go](../internal/apiserver/transport/grpc/registry.go)，proto 与注册关系由 [internal/apiserver/transport/grpc/proto_contract_test.go](../internal/apiserver/transport/grpc/proto_contract_test.go) 保护。

## 安全约定

- REST 受保护路由使用 `Authorization: Bearer <JWT>`；公开面包括健康检查、登录、JWKS 和部分 public/info 路由。
- gRPC生产模板配置mTLS、方法ACL和audit；实际装配还需核对有效配置与模块内准入。标准链未装service Bearer/HMAC验证器，详见[传输安全](../docs/03-基础设施/05-传输层与服务间安全.md)。
- 离线 JWKS 验签只证明签名、issuer/audience 和过期时间；撤销、会话、用户或账号状态以在线 Verify 能力为准。

## 验证

```bash
make api-validate
go test ./internal/apiserver/transport/rest ./internal/apiserver/transport/grpc
```

`make api-validate` 优先使用Docker中的Spectral 6.15.0；Docker不可用时使用npx中的固定CLI版本。它读取已提交Swagger，运行lint、warning基线、schema/route比对和一个Gin注册测试，不重新生成Swagger或proto。当前通用schema匹配为零，注册比对也只选择v2及JWKS；绿色结果的范围见[契约治理](../docs/04-接口与SDK/01-REST-gRPC与契约治理.md)。
