# SDK Compile Test

> 状态：已实现 · 已与 `pkg/sdk/public_api_compile_test.go`、`go test ./pkg/sdk/...` 和 `internal/pkg/architecture` SDK no-internal 规则核对。

---

## 1. 本文回答

本文回答 8 个问题：

- IAM 为什么需要 SDK Compile Test？
- SDK Compile Test 保护什么？
- `pkg/sdk` public API 与 REST/gRPC 契约是什么关系？
- 为什么 SDK 不能 import IAM `internal` 包？
- 新增或修改 SDK public API 时应该补什么？
- SDK Compile Test 与契约测试有什么区别？
- 失败时如何排查？
- 修改后应该执行哪些 Verify？

本文是架构护栏中的 SDK 编译保护文档。Go SDK 接入模型见 [../03-接入与契约/03-Go-SDK接入模型.md](../03-接入与契约/03-Go-SDK接入模型.md)；架构测试见 [02-架构测试.md](02-架构测试.md)。

---

## 2. 30 秒结论

SDK Compile Test 把 Go SDK 的 **public API 可编译性** 固化成测试。

核心目标：

```text
pkg/sdk 对外暴露的类型、构造函数、方法签名可编译；
SDK 不 import IAM internal 包；
AuthN / AuthZ / Identity / IDP 主要 client 面保持稳定；
public API 变更能被 CI 发现，而不是等业务方升级后才发现。
```

如果只记一句话：

> SDK Compile Test 保护的是外部 Go 调用面，不是 IAM 内部业务逻辑。

---

## 3. 事实源

| 事实 | 路径 |
| --- | --- |
| public API compile test | `../../pkg/sdk/public_api_compile_test.go` |
| SDK 根包 | `../../pkg/sdk` |
| AuthN clients | `../../pkg/sdk/auth/...` |
| AuthZ client | `../../pkg/sdk/authz` |
| Identity clients | `../../pkg/sdk/identity` |
| IDP client | `../../pkg/sdk/idp` |
| SDK no-internal 规则 | `../../internal/pkg/architecture` |

---

## 4. 保护范围

`TestPublicAPISurfaceCompiles` 当前覆盖：

```text
sdk.Client / sdk.Config / ClientOption
auth/challenge / auth/loginv2 / auth/signup / auth/loginidentity
auth/jwks / auth/verifier / auth/serviceauth
authz.Client
identity.Client / ProfileClient / ProfileLinkClient
idp.Client
sdk/errors
```

它不替代：

```text
业务语义正确性（由 module tests 覆盖）；
REST/gRPC 字段级契约对齐（由 contract tests 覆盖）；
示例代码可读性（由 pkg/sdk/docs 和 README 维护）。
```

---

## 5. 关键边界

| 边界 | 说明 |
| --- | --- |
| public vs internal | 只有 `pkg/sdk` 下对外稳定面包进 compile test |
| SDK vs domain | SDK DTO 不是 domain entity |
| SDK vs OpenAPI/proto | SDK 不反向定义机器契约 |
| compile test vs 单元测试 | compile test 只保证 API 面可编译，不断言业务行为 |

---

## 6. 代码事实源

| 能力 | 路径 |
| --- | --- |
| public API compile test | `pkg/sdk/public_api_compile_test.go` |
| SDK 根入口 | `pkg/sdk/client.go` |
| 配置加载 | `pkg/sdk/config` |
| 错误模型 | `pkg/sdk/errors` |
| 架构 no-internal 规则 | `internal/pkg/architecture` |

---

## 7. Verify

```bash
go test ./pkg/sdk/...
go test ./internal/pkg/architecture/...
make docs-hygiene
```

---

## 8. 本文总结

SDK Compile Test 是 IAM 对外 Go 接入面的最低工程护栏。它与 architecture tests 的 no-internal 规则、契约测试的 OpenAPI/proto 对齐一起，防止 SDK 在演进中悄悄破坏外部调用方。
