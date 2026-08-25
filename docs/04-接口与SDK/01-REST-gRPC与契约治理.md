# REST、gRPC 与契约治理

> 状态：已实现 · 接口能力必须同时闭合“机器契约、运行时注册、实现、错误语义和测试”，不能只看其中一层。

## 1. 为什么同时提供 REST 和 gRPC

REST 适合浏览器、管理端和跨语言公开接入；gRPC 适合可信服务间调用、强类型生成代码和较稳定的内部命令/查询。当前不是所有能力双栈对称：

- Suggest 只提供 REST；
- Identity 的 Profile/ProfileLink 写命令主要在 gRPC；
- AuthN 部分面向终端的 login/challenge/linking 有 REST 客户端；
- 核心服务间能力通过 gRPC SDK 暴露。

不追求机械对称，可以减少重复 API；代价是接入者必须先按场景选择 transport，文档不能笼统说“所有模块都有 REST/gRPC”。

## 2. REST 契约闭环

OpenAPI 3.1 分模块保存于 `api/rest/*.v2.yaml` 与 `api/rest/authz.v3.yaml`。一次 REST 变更至少检查：

```text
OpenAPI path + schema + security
  <-> Router 实际 method/path
  <-> request/response DTO mapping
  <-> application capability
  <-> error HTTP status
  <-> SDK/caller（若有）
```

Router 在运行时根据 ModuleState 和 middleware availability 注册，因此静态 OpenAPI 可能描述“完整服务能力”，某个 degraded 实例却不注册 protected route。生产 release 不允许这种 critical degraded 形态；开发诊断时应以 `/debug/routes`/模块状态和 readiness 为准。

`router_matrix_test.go` 以实际 Gin route 表检查 public route 的 OpenAPI 覆盖，并保护已退役路径不回归。Swagger 注解生成物仍用于比对，但 canonical REST 文件在 `api/rest`，不能手工只改 `internal/apiserver/docs/swagger.yaml`。

## 3. gRPC 契约闭环

Proto 位于：

```text
api/grpc/iam/{authn,identity,idp}/v2/*.proto
api/grpc/iam/authz/v3/*.proto
```

`container.grpcRegistrations()` 只收集 available module 的 registration，`transport/grpc.Registry` 完成注册后才将服务标为 SERVING。`proto_contract_test.go` 检查 proto service 与 Go runtime registration 的对应关系；模块级 alignment test 进一步防止“proto 声明了 service，但 service wrapper 没有真正 Register”。

Proto 演进规则：

- 字段号发布后不得复用；删除字段应 reserved number/name；
- 新字段以 optional/default-safe 方式追加；
- 不改变既有 enum 数值含义；
- RPC rename/remove 属于 breaking change，应新版本或保留兼容期；
- 生成代码、server、SDK 和 compile test 同步提交。

## 4. 错误是契约的一部分

REST 调用方应依赖 HTTP status + 稳定业务 code；gRPC 调用方应依赖 `codes.Code` 和结构化错误，不解析 message 文本。

服务端通过 `internal/pkg/grpc.ToStatusError` 把注册的业务错误映射为标准 status，并把 Internal/Unknown/DataLoss 等动态内部错误统一转成安全公共文案。这样既避免泄漏 SQL/provider/密钥细节，也让 SDK 的 `IsNotFound`、`IsRetryable` 等谓词稳定工作。

直接 `status.Errorf(codes.Internal, err.Error())` 同时破坏安全和兼容性，architecture test 对此做静态守卫。

## 5. 认证与授权契约

REST protected route 使用 Bearer user token；gRPC 面向服务间调用，可叠加 mTLS、service token/HMAC/API key、ACL 和业务 AuthZ。接口文档必须标明：

- 哪些 endpoint 是 public；
- 需要 user token 还是 service identity；
- audience/tenant/metadata 怎样传递；
- route permission 与对象级 Check 的区别；
- local JWKS verify 的撤销窗口。

只写一个锁图标而不写主体类型与授权条件，对接入方没有足够信息。

## 6. 版本与兼容策略

路径/package 中的版本号是合同版本，不代表所有内部实现使用相同编号。AuthZ 已仅保留 v3，其他现有服务仍主要使用 v2。兼容应以外部可观察行为判断：字段、默认值、错误 code、排序/分页、幂等和副作用都属于合同。

常见非兼容变更：

- optional 字段变 required；
- 空列表改为 404；
- 同名字段从 ID 改 display label；
- 重试从幂等变重复创建；
- 本地验签策略默默切为在线或反之；
- route 被注册但安全 middleware 改变。

内部重构只要保持这些行为和 machine contract，不必为内部包名保留兼容。

## 7. 当前验证层次

```bash
# REST/OpenAPI lint、Swagger 与 route 比对（需要 Docker/Spectral）
make api-validate

# runtime route/proto registration
go test \
  ./internal/apiserver/transport/rest/... \
  ./internal/apiserver/transport/grpc/...

# 生成 proto 后检查 dirty diff
make proto-gen
git diff --exit-code
```

`api-validate` 通过不证明业务语义正确；transport test 通过也不证明 OpenAPI lint；两类证据必须分别报告。

## 8. 备选治理方案

### Code-first Swagger

handler 注解离实现近，但复杂 schema/多文件复用和 breaking review 较弱。当前保留生成 Swagger 并与 canonical split OpenAPI 比对，而不是把生成物当唯一手写源。

### Contract-first 生成 server stub

漂移更少，但迁移现有 Gin handler 成本高。gRPC 已天然使用 proto-first；REST 当前通过 route/contract checker补足。

### 只靠集成测试

能测行为但难覆盖所有字段、deprecated/reserved 和未注册 surface。静态 contract gate 与行为测试互补。

## 9. 面试追问

### OpenAPI 存在为什么还会漂移？

因为 Router 可以手写不同 path，DTO 可以漏字段，安全 middleware 可以变化，生成物也可能未更新。契约只是一个事实面，需要自动比对注册和实现。

### gRPC 向后兼容最重要的规则是什么？

永不复用 field number。未知字段可被新旧双方忽略，但复用会让旧数据被解释成完全不同的含义。

### 404 和空列表为什么也算合同？

调用方会根据它们决定重试、展示和分支。即使 schema 不变，状态语义变化也是可观察 breaking change。

## 10. 事实来源

- REST：`api/rest`、`internal/apiserver/transport/rest`
- gRPC：`api/grpc`、`internal/apiserver/transport/grpc`
- errors：`internal/pkg/grpc/error_mapper.go`、`internal/pkg/code`
- gates：`scripts/{validate-openapi.sh,check-openapi-contracts.py,check-route-contracts.py}`
