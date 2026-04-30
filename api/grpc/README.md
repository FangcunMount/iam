# gRPC API 契约

IAM gRPC 面向可信服务间调用。当前只发布 v1 proto，所有服务由 `iam-apiserver` 同一进程注册，运行时注册在 [internal/apiserver/transport/grpc/registry.go](../../internal/apiserver/transport/grpc/registry.go)。

## Proto 布局

```text
api/grpc/iam/
├── authn/v1/authn.proto
├── authz/v1/authz.proto
├── identity/v1/identity.proto
└── idp/v1/idp.proto
```

新增字段只能追加，禁止复用 field number。新增 v2 proto 前必须同时提交 transport 注册、生成代码、SDK compile test 和契约文档。

## 服务矩阵

| Proto | Service | 当前能力 |
| ---- | ---- | ---- |
| [iam/authn/v1/authn.proto](iam/authn/v1/authn.proto) | `AuthService` | VerifyToken、RefreshToken、RevokeToken、RevokeRefreshToken、IssueServiceToken |
| [iam/authn/v1/authn.proto](iam/authn/v1/authn.proto) | `AccountOnboardingService` | CreateOperationAccount |
| [iam/authn/v1/authn.proto](iam/authn/v1/authn.proto) | `JWKSService` | GetJWKS |
| [iam/authz/v1/authz.proto](iam/authz/v1/authz.proto) | `AuthorizationService` | Check、GetAuthorizationSnapshot、GrantAssignment、RevokeAssignment |
| [iam/identity/v1/identity.proto](iam/identity/v1/identity.proto) | `IdentityRead` | GetUser、BatchGetUsers、SearchUsers、GetProfile、BatchGetProfiles |
| [iam/identity/v1/identity.proto](iam/identity/v1/identity.proto) | `ProfileLinkQuery` | HasProfileLink、ListProfiles、ListProfileLinks |
| [iam/identity/v1/identity.proto](iam/identity/v1/identity.proto) | `ProfileLinkCommand` | EstablishProfileLink、RevokeProfileLink、BatchRevokeProfileLinks、ImportProfileLinks |
| [iam/identity/v1/identity.proto](iam/identity/v1/identity.proto) | `IdentityLifecycle` | CreateUser、UpdateUser、DeactivateUser、BlockUser |
| [iam/idp/v1/idp.proto](iam/idp/v1/idp.proto) | `IDPService` | GetWechatApp |

## 安全与 metadata

- gRPC 配置在 `process` 层装配，支持 mTLS、service token、ACL 和 audit。
- 调用方传 `authorization: Bearer <service-token>`；服务端也可结合 mTLS 身份与 ACL 判断调用边界。
- 建议所有调用传 `x-request-id`，便于日志和 trace 对齐。

## Identity 关系术语

当前 proto 的关系服务是 `ProfileLinkQuery` 与 `ProfileLinkCommand`。`ProfileLink` 表示用户和 profile 之间的档案关系，可承载自有档案和亲属/监护类关系语义。旧关系名只保留为历史语义，不作为当前合同名。

## Go 调用示例

```go
ctx = metadata.AppendToOutgoingContext(ctx,
    "authorization", "Bearer "+serviceToken,
    "x-request-id", requestID,
)

authzClient := authzv1.NewAuthorizationServiceClient(conn)
snapshot, err := authzClient.GetAuthorizationSnapshot(ctx, &authzv1.GetAuthorizationSnapshotRequest{
    Subject: &authzv1.Subject{Type: "user", Id: "1024"},
    TenantId: "default",
    AppName:  "qs",
})

identityClient := identityv1.NewProfileLinkQueryClient(conn)
linked, err := identityClient.HasProfileLink(ctx, &identityv1.HasProfileLinkRequest{
    UserId: "1024",
    ProfileId: "2048",
})
```

## 验证

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc ./pkg/sdk
```

proto 与注册关系由 [internal/apiserver/transport/grpc/proto_contract_test.go](../../internal/apiserver/transport/grpc/proto_contract_test.go) 保护。
