# Go SDK 接入模型

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 事实源

Go SDK 公开 API 以 `../../pkg/sdk` 为准。

SDK 文档入口：

- `../../pkg/sdk/README.md`
- `../../pkg/sdk/docs/README.md`

## 定位

SDK 是业务 Go 服务接入 IAM 的产品化封装，不是 IAM 业务层本身。

## 规则

- SDK 不 import IAM internal 包。
- SDK public API 变化必须有 compile test 保护。
- SDK 文档可以解释用法，但不能替代 REST/gRPC 契约。

## Verify

```bash
go test ./pkg/sdk/...
```
