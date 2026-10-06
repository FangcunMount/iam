# gRPC API 契约

IAM gRPC面向可信服务间调用。AuthN使用v3，AuthZ使用v4，Identity/IDP使用v2；四份proto声明13个service、42个Unary RPC。标准进程按模块Available与服务依赖收集注册，声明数量不等于某个实例的实际出口。注册在[Registry](../../internal/apiserver/transport/grpc/registry.go)，生成工具、注册测试和兼容基线的实际边界见[契约治理](../../docs/04-接口与SDK/01-REST-gRPC与契约治理.md)。

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
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `NotificationRecipientService` | ResolveMiniProgramRecipients（仅受授权的 QS 服务和 AppID） |
| [iam/authn/v3/authn.proto](iam/authn/v3/authn.proto) | `JWKSService` | GetJWKS |
| [iam/authz/v4/authz.proto](iam/authz/v4/authz.proto) | `AuthorizationService` | Check、GetAuthorizationSnapshot、GetCommittedPolicyVersion、GrantAssignment、RevokeAssignment、ReplaceManagedAssignments、ReplaceScopedAssignments |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `IdentityRead` | GetUser、BatchGetUsers、SearchUsers、GetProfile、BatchGetProfiles |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileLinkQuery` | HasProfileLink、ListProfiles、ListProfileLinks |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileCommand` | CreateProfile |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `ProfileLinkCommand` | EstablishProfileLink、RevokeProfileLink、BatchRevokeProfileLinks、ImportProfileLinks |
| [iam/identity/v2/identity.proto](iam/identity/v2/identity.proto) | `IdentityLifecycle` | CreateUser、UpdateUser、DeactivateUser、BlockUser |
| [iam/idp/v2/idp.proto](iam/idp/v2/idp.proto) | `IDPService` | GetWechatApp、GetWechatAccessToken、RefreshWechatAccessToken |

## 安全与 metadata

- gRPC在process层按有效配置装配mTLS、ACL和audit，条件分支及标准启动门禁见[传输安全](../../docs/03-基础设施/05-传输层与服务间安全.md)。
- 标准mTLS业务链使用经验证证书的服务身份，方法ACL与内容/用户授权分别检查；标准链没有服务Bearer验证器，metadata不能替代证书身份。
- `x-request-id`等自定义ID只承担关联用途，当前不等于OTel标准传播或幂等合同；SDK传播与双日志上下文见[观测正文](../../docs/03-基础设施/06-可观测性就绪与关闭.md#7-关联id日志和真正的trace分别具备什么)。

## Identity 关系术语

当前proto的关系服务是`ProfileLinkQuery`与`ProfileLinkCommand`。CreateProfile组合门面在同一工作单元编排Profile和User→ProfileLink：顶层入口负责提交，Required借用外层事务时由宿主提交，callback返回不等于外层已提交。`ProfileLink`表示用户和档案之间的关系，可承载自有及亲属/监护语义；事务与失败由[创建链路](../../docs/02-业务模块/01-Identity/02-关键链路-创建User与Profile.md)维护。

## Go 调用示例

```go
ctx = metadata.AppendToOutgoingContext(ctx,
    "x-request-id", requestID,
)

authzClient := authzv4.NewAuthorizationServiceClient(conn)
snapshot, err := authzClient.GetAuthorizationSnapshot(ctx, &authzv4.GetAuthorizationSnapshotRequest{
    Subject: "user:1024",
    AppName:  "qs",
})

identityClient := identityv2.NewProfileLinkQueryClient(conn)
linked, err := identityClient.HasProfileLink(ctx, &identityv2.HasProfileLinkRequest{
    UserId: "1024",
    ProfileId: "2048",
})
```

AuthZ v4 的 `Check` 是可信服务使用的动作判定入口；REST 负责权限事实管理。角色继承已退役，快照 `roles` 与 `direct_roles` 使用相同应用过滤并表示直接角色集合。`ReplaceManagedAssignments` 只替换调用服务受管的角色子集；其响应 `direct_roles` 当前表示目标受管子集，若要读取持久化后的全部直接角色，应再次调用 `GetAuthorizationSnapshot`。

`GetCommittedPolicyVersion` 只允许 `qs-apiserver.svc` 服务证书调用，从 IAM 主业务库读取已提交的全局策略版本；它不返回用户权限，也不表示 IAM 内存授权快照已加载该版本。QS 必须在版本前进时淘汰旧缓存，并在 `GetAuthorizationSnapshot` 的 `policy_version` 追上已提交版本之前拒绝旧授权。调用失败或版本回退不能刷新 QS 的核验时限。

## 验证

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc ./pkg/sdk
```

现有[proto注册护栏](../../internal/apiserver/transport/grpc/proto_contract_test.go)只搜索service注册字符串；Identity另有实际GetServiceInfo集合断言。生成脚本仅在缺插件时安装固定版本，既有工具未校验；Make包装也有脚本错误被成功输出掩盖的源码窗口。因此需独立确认生成器真实退出、产物差异、方法实现/准入和旧消费者，不据上述两条命令证明完整兼容。

Scope 消费者必须验证 `scope_contract_version=1`，按公司、资源和动作使用 `permissions.scopes`，由业务系统执行数据范围限制。未配置范围不表示全公司。管理调用使用完整 `assignment_scopes` 及 `ReplaceScopedAssignments`，提交 `expected_policy_version`；版本冲突须重新读取，不能盲目重试覆盖。旧 RPC 不能代替公司范围分配接口。
