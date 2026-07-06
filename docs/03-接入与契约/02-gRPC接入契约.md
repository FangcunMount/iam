# gRPC 接入契约

## 事实源

gRPC 契约以 proto 为准：

- `../../api/grpc/iam/authn/v2/authn.proto`
- `../../api/grpc/iam/authz/v2/authz.proto`
- `../../api/grpc/iam/identity/v2/identity.proto`
- `../../api/grpc/iam/idp/v2/idp.proto`

运行时入口：

- `../../internal/apiserver/transport/grpc`

## 规则

- service、message、field 以 proto 为准。
- 传输层 mapper 不进入 domain。
- gRPC 适合可信服务间调用。

## Verify

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc
```
