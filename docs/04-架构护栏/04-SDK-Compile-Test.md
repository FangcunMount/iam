# SDK Compile Test

## 30 秒结论

SDK 是外部 Go 服务接入 IAM 的稳定面。公开 API 不能只靠 README 约定，需要 compile test 保护。

## 规则

- SDK 不 import `internal/apiserver`。
- SDK public API 变化需要测试覆盖。
- SDK 文档入口在 `../../pkg/sdk/docs/README.md`。

## Verify

```bash
go test ./pkg/sdk/...
```
