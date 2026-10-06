# AuthZ SDK

SDK 传递资源/动作判定与范围快照，并校验部分响应合同。业务服务负责从可信身份取得 Subject、匹配权限并执行对象范围规则；mTLS 只认证调用服务。完整设计与管理接入以 [gRPC 授权与 SDK](../../../docs/02-业务模块/03-AuthZ/06-关键链路-gRPC服务间授权与SDK.md) 为准。

## 动作判定：拒绝与错误分开处理

通过 `client.Authz().Allow(ctx, subject, resource, action)` 执行资源与动作检查；需要拒绝原因、命中授权及策略版本时使用 `Check`。`Allow` 只提取响应的 `Allowed` 字段：`false, nil` 是一次有效的拒绝，非 nil error 是未取得可用判定，不能转为允许。

```go
callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
allowed, err := client.Authz().Allow(callCtx, "user:42", "qs:evaluation:collection:assessments", "retry")
if err != nil { return err }
if !allowed { return fmt.Errorf("permission denied") }
```

上面的超时是调用方示例预算，不是 SDK 默认保证。`Config.Timeout` 虽默认填为 30 秒，当前默认 RPC 链没有使用它设置 deadline；须由请求 context 提供。RPC 错误经 SDK 包装保留 gRPC status；`IsRetryable` 只分类错误，不判断写操作能否安全重发。

主体必须来自可信身份上下文。`Allow` 只检查动作，不证明目标对象在授权范围中；业务服务仍需校验当前机构、运营身份、受试者关系和对象状态。列表应在分页与统计前执行业务范围过滤。

## 范围快照：同一 RPC，增加本地合同校验

```go
scopeCtx, scopeCancel := context.WithTimeout(ctx, 2*time.Second)
defer scopeCancel()
snapshot, err := client.Authz().GetScopedAuthorizationSnapshot(scopeCtx,
    &authzv4.GetAuthorizationSnapshotRequest{Subject: "user:42", AppName: "qs"})
if err != nil { return err }
// 继续匹配资源/动作，再合并当前公司的范围并执行业务过滤。
```

`GetScopedAuthorizationSnapshot` 调用普通 `GetAuthorizationSnapshot` RPC，随后在本地要求正策略版本、`ScopeContractVersion=1`、`UNCONDITIONAL` 权限和合法 Scope。它不是新的读取 RPC，也不执行权限匹配或数据过滤。权限可以没有范围，零权限也是合法快照；消费者必须把缺少匹配范围作为无数据范围授权，不能回退为全公司权限。

例如公司 1 的 `retry` 范围为门店 7、`read` 范围为门店 8，重试门店 8 的记录时不能借用 `read` 范围。`ALL_STORES` 仍要求对象当前属于该公司且有门店归属，不能使未归属对象自动可见。通配资源/动作的匹配规则需由消费者明确；SDK 没有提供匹配或 Union helper。

快照的 `roles` 与 `direct_roles` 都返回按 Role.Name 应用前缀过滤的直接角色集合，角色继承已退役。`permissions` 独立按 Resource 的具体 AppName 投影；AppName 不是调用服务的命名空间准入证明。其他应用和无应用前缀的全局角色可能不在这些角色列表中，因此不能据它们构造完整管理集合。

## 分配管理：以部署定义的完整受管集合为边界

读取管理事实时设置 `IncludeAssignmentFacts=true`，服务端还会执行 replacement admission；SDK helper 随后调用 `ValidateAssignmentScopes`。返回的 `assignment_facts` 和逐条 `assignment_scopes` 包括其他应用及全局角色；`Scope=nil` 是明确未配置，不表示全公司。

本地 validator 检查完整标记、规范正 int64 字符串 ID、重复 RoleID/AssignmentID、角色字段一致和每个返回角色的分配覆盖。它不验证 `management_protection` 合法取值、RoleName 唯一性、分配与权限范围一致，也不能独立证明生产者没有漏掉一条 Assignment。校验失败是普通 Go error，不保证 gRPC code。

`ReplaceManagedAssignments` 的受管集合来自服务端部署白名单，不是按创建者或请求中的目标角色推导；它保留集合之外的分配，但会拒绝受管集合内已有 Scoped 分配。`ReplaceScopedAssignments` 使用独立 RPC，提交一个公司的完整受管集合内目标角色和范围，并携带正 `ExpectedPolicyVersion`；未提交的受管角色会在该公司范围内撤销，其他公司保留。若受管集合内存在未配置 Scope 的历史分配，当前 Scoped replacement 会拒绝并要求显式迁移，不会自动补成当前公司。比如部署受管集合为两个角色而请求只提交其中一个时，这是对该公司两个角色的整体目标状态，不是只修改所提交角色。

版本冲突返回 `Aborted`，需重读并核对用户意图。默认 retry 可能重发原请求，不会自动读取新版本；如果首次提交成功后响应丢失，重发的冲突不能证明首次失败。请求结果不明时先查询事实，不能盲目提高版本覆盖。旧服务对 Scoped RPC 返回 `Unimplemented` 时，不回退到普通替换。

## 条件授权退役与示例边界

`CheckObject` 已弃用，保留源码符号但直接返回“条件授权已退役”错误，不发起 RPC，也不转换为无条件允许。gRPC 非空 object_context 返回 InvalidArgument。协议字段号暂留，后续主版本再删除公共符号。

快照只产生 UNCONDITIONAL 权限。消费者收到旧 OBJECT_CHECK_REQUIRED 模式时不得将其当作动作权限，应刷新数据并保持拒绝。多角色授权仍取并集。参见 [AuthZ 维护手册](../../../docs/02-业务模块/03-AuthZ/08-条件授权退役维护手册.md)。

当前 `_examples/authz/main.go` 保留旧 `Domain` 字段和 `Allow` 参数；本轮单独编译已确认失败，分别是不存在的 `CheckRequest.Domain` 和 `Allow` 参数过多，不能作为当前可运行证明。本页和 [快速开始的 AuthZ 片段](./01-quick-start.md#授权判定服务) 已使用当前 API，完整接入见 canonical 正文。`go test ./pkg/sdk/...` 不会自动覆盖以下划线开头的 `_examples` 目录；public API compile test 也不逐一编译文档片段。需要另行编译示例，并分开验证接口、mTLS 真实连接和业务范围执行。
