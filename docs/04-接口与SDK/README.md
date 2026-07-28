# 接口与 SDK

> 状态：已实现 · REST、gRPC、SDK 的事实源、注册关系与接入边界已按当前契约和代码复核。

本目录解释 IAM 怎样把同一组应用能力暴露为 REST、gRPC 和 Go SDK，以及如何防止契约、注册、实现与调用方漂移。

## 阅读路径

1. [REST、gRPC 与契约治理](01-REST-gRPC与契约治理.md)
2. [Go SDK 与业务系统接入](02-Go-SDK与业务系统接入.md)

## 事实源

```text
REST fields/paths/security -> api/rest/*.yaml
gRPC services/messages     -> api/grpc/**/*.proto
runtime exposure           -> transport registration
Go public API              -> pkg/sdk + public_api_compile_test.go
documentation              -> explanation only
```

## 最重要的边界

- 契约文件存在不等于 runtime 已注册；
- runtime handler 存在不等于契约已发布；
- SDK 方法存在不等于服务端兼容；
- 本地 JWT 验签不等于 IAM 在线验证；
- 前端能力展示不等于服务端授权。
