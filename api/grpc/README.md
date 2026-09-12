# gRPC API 契约

IAM gRPC 面向可信服务间调用。AuthN、Identity、IDP 使用 v2，AuthZ 使用 v3；所有服务由 `iam-apiserver` 同一进程注册，运行时注册在 [internal/apiserver/transport/grpc/registry.go](../../internal/apiserver/transport/grpc/registry.go)。

## Proto 布局

```text
api/grpc/iam/
├── authn/v3/authn.proto
├── authz/v4/authz.proto
├── identity/v2/identity.proto
└── idp/v2/idp.proto
```

新增字段只能追加，禁止复用 field number。proto、transport 注册、生成代码、SDK compile test 和契约文档必须同步更新。

## 服务矩阵

| Proto | Service | 当前能力 |
| ---- | ---- | ---- |
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `AuthService` | Login、VerifyToken、RefreshToken、RevokeToken、RevokeRefreshToken |
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `AuthSignupService` | SignUpWithWechatMiniProgram |
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `AuthChallengeService` | SendLoginPhoneOTP |
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `LoginIdentityService` | ListLoginIdentities、SendPhoneLinkChallenge、LinkPhone、LinkWechatMiniProgram、LinkWecom、UnlinkLoginIdentity |
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `JWKSService` | GetJWKS |
| [iam/authz/v4/authz.proto](iam/authz/v4/authz.proto) | `AuthorizationService` | Check、GetAuthorizationSnapshot、GrantAssignment、RevokeAssignment、ReplaceManagedAssignments、ReplaceScopedAssignments |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `IdentityRead` | GetUser、BatchGetUsers、SearchUsers、GetProfile、BatchGetProfiles |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileLinkQuery` | HasProfileLink、ListProfiles、ListProfileLinks |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileCommand` | CreateProfile |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileLinkCommand` | EstablishProfileLink、RevokeProfileLink、BatchRevokeProfileLinks、ImportProfileLinks |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `IdentityLifecycle` | CreateUser、UpdateUser、DeactivateUser、BlockUser |
| [iam/idp/v2/idp.proto](iam/idp/v2/idp.proto) | `IDPService` | GetWechatApp |

## 安全与 metadata

- gRPC 配置在 `process` 层装配，使用 mTLS、ACL 和 audit。
- 服务身份来自经过验证的 mTLS 证书；ACL 校验方法权限，随后进行业务授权。服务调用无需服务 Bearer。
- 建议所有调用传 `x-request-id`，便于日志和 trace 对齐。

## Identity 关系术语

当前 proto 的关系服务是 `ProfileLinkQuery` 与 `ProfileLinkCommand`。`ProfileCommand.CreateProfile` 是创建 Profile 并建立 User -> ProfileLink 的组合门面，保证 Profile 与 ProfileLink 在同一个应用工作单元中提交。`ProfileLink` 表示用户和 profile 之间的档案关系，可承载自有档案和亲属/监护类关系语义。旧关系名只保留为历史语义，不作为当前合同名。

## Go 调用示例

```go
ctx = metadata.AppendToOutgoingContext(ctx,
    "x-request-id", requestID,
)

authzClient := authzv4.NewAuthorizationServiceClient(conn)
snapshot, err := authzClient.GetAuthorizationSnapshot(ctx, &authzv4.GetAuthorizationSnapshotRequest{
    Subject: "user:1024",
    Domain:   "default",
    AppName:  "qs",
})

identityClient := identityv2.NewProfileLinkQueryClient(conn)
linked, err := identityClient.HasProfileLink(ctx, &identityv2.HasProfileLinkRequest{
    UserId: "1024",
    ProfileId: "2048",
})
```

AuthZ v4 的 `Check` 是可信服务使用的动作判定入口；REST 负责权限事实管理。角色继承已退役，快照 `roles` 与 `direct_roles` 使用相同应用过滤并表示直接角色集合。`ReplaceManagedAssignments` 只替换调用服务受管的角色子集；其响应 `direct_roles` 当前表示目标受管子集，若要读取持久化后的全部直接角色，应再次调用 `GetAuthorizationSnapshot`。

## 验证

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc ./pkg/sdk
```

proto 与注册关系由 [internal/apiserver/transport/grpc/proto_contract_test.go](../../internal/apiserver/transport/grpc/proto_contract_test.go) 保护。

Scope 消费者必须验证 `scope_contract_version=1`，按公司、资源和动作使用 `permissions.scopes`，由业务系统执行数据范围限制。未配置范围不表示全公司。管理调用使用完整 `assignment_scopes` 及 `ReplaceScopedAssignments`，提交 `expected_policy_version`；版本冲突须重新读取，不能盲目重试覆盖。旧 RPC 不能代替公司范围分配接口。
