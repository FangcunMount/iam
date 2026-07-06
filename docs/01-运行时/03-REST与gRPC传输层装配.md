# REST 与 gRPC 传输层装配

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

REST 和 gRPC 是接入形态，不是业务模型。传输层负责路由、DTO、错误映射和中间件，不拥有领域规则。

## 事实源

| 接入形态 | 运行时 | 机器契约 |
| --- | --- | --- |
| REST | `../../internal/apiserver/transport/rest` | `../../api/rest` |
| gRPC | `../../internal/apiserver/transport/grpc` | `../../api/grpc` |
| Go SDK | `../../pkg/sdk` | `../../pkg/sdk` |

## Verify

```bash
make api-validate
make proto-gen
go test ./internal/apiserver/transport/grpc
```
