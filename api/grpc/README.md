# gRPC API 契约

IAM gRPC 面向可信服务间调用。当前只发布 v2 proto，所有服务由 `iam-apiserver` 同一进程注册，运行时注册在 [internal/apiserver/transport/grpc/registry.go](../../internal/apiserver/transport/grpc/registry.go)。

## Proto 布局

```text
api/grpc/iam/
├── authn/v2/authn.proto
├── authz/v2/authz.proto
├── identity/v2/identity.proto
└── idp/v2/idp.proto
```

新增字段只能追加，禁止复用 field number。proto、transport 注册、生成代码、SDK compile test 和契约文档必须同步更新。

## 服务矩阵

| Proto | Service | 当前能力 |
| ---- | ---- | ---- |
| [iam/authn/v2/authn.proto](iam/authn/v2/authn.proto) | `AuthService` | VerifyToken、RefreshToken、RevokeToken、RevokeRefreshToken、IssueServiceToken |
| [iam/authn/v2/authn.proto](iam/authn/v2/authn.proto) | `AccountOnboardingService` | CreateOperationAccount |
| [iam/authn/v2/authn.proto](iam/authn/v2/authn.proto) | `JWKSService` | GetJWKS |
| [iam/authz/v2/authz.proto](iam/authz/v2/authz.proto) | `AuthorizationService` | Check、GetAuthorizationSnapshot、GrantAssignment、RevokeAssignment |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `IdentityRead` | GetUser、BatchGetUsers、SearchUsers、GetProfile、BatchGetProfiles |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileLinkQuery` | HasProfileLink、ListProfiles、ListProfileLinks |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileCommand` | CreateProfile |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileLinkCommand` | EstablishProfileLink、RevokeProfileLink、BatchRevokeProfileLinks、ImportProfileLinks |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `IdentityLifecycle` | CreateUser、UpdateUser、DeactivateUser、BlockUser |
| [iam/idp/v2/idp.proto](iam/idp/v2/idp.proto) | `IDPService` | GetWechatApp |

## 安全与 metadata

- gRPC 配置在 `process` 层装配，支持 mTLS、service token、ACL 和 audit。
- 调用方传 `authorization: Bearer <service-token>`；服务端也可结合 mTLS 身份与 ACL 判断调用边界。
- 建议所有调用传 `x-request-id`，便于日志和 trace 对齐。

## Identity 关系术语

当前 proto 的关系服务是 `ProfileLinkQuery` 与 `ProfileLinkCommand`。`ProfileCommand.CreateProfile` 是创建 Profile 并建立 User -> ProfileLink 的组合门面，保证 Profile 与 ProfileLink 在同一个应用工作单元中提交。`ProfileLink` 表示用户和 profile 之间的档案关系，可承载自有档案和亲属/监护类关系语义。旧关系名只保留为历史语义，不作为当前合同名。

## Go 调用示例

```go
ctx = metadata.AppendToOutgoingContext(ctx,
    "authorization", "Bearer "+serviceToken,
    "x-request-id", requestID,
)

authzClient := authzv2.NewAuthorizationServiceClient(conn)
snapshot, err := authzClient.GetAuthorizationSnapshot(ctx, &authzv2.GetAuthorizationSnapshotRequest{
    Subject: &authzv2.Subject{Type: "user", Id: "1024"},
    TenantId: "default",
    AppName:  "qs",
})

identityClient := identityv2.NewProfileLinkQueryClient(conn)
linked, err := identityClient.HasProfileLink(ctx, &identityv2.HasProfileLinkRequest{
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
