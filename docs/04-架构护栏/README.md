# 04-架构护栏

## 本目录定位

`04-架构护栏/` 说明 IAM 如何防止分层边界、契约和文档长期漂移。

## 文档结构

| 文档 | 作用 |
| --- | --- |
| [01-分层依赖边界.md](01-分层依赖边界.md) | 分层依赖规则 |
| [02-架构测试.md](02-架构测试.md) | 架构测试保护范围 |
| [03-契约测试.md](03-契约测试.md) | REST/gRPC 契约测试 |
| [04-SDK-Compile-Test.md](04-SDK-Compile-Test.md) | SDK 公开 API 保护 |
| [05-Docs-Hygiene.md](05-Docs-Hygiene.md) | 文档链接和旧事实防回流 |

## 最小验证

```bash
go test ./internal/pkg/architecture
make docs-hygiene
```
