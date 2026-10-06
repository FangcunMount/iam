# 接口与 SDK

> 状态：已实现 · REST、gRPC、SDK 的事实源、注册关系与接入边界已按当前契约和代码复核。

本目录解释 IAM 怎样把同一组应用能力暴露为 REST、gRPC 和 Go SDK，以及如何防止契约、注册、实现与调用方漂移。

## 阅读路径

1. [接口契约、生成链与兼容边界](01-REST-gRPC与契约治理.md)
2. [Go SDK 与业务系统接入](02-Go-SDK与业务系统接入.md)

## 事实源

```text
REST published description -> api/rest/*.yaml
gRPC services/messages     -> api/grpc/**/*.proto
runtime exposure           -> transport registration
Go public API              -> pkg/sdk + public_api_compile_test.go
behavior/security/errors    -> DTO + middleware + application + mapper
documentation               -> ownership, differences and evidence
```

## 最重要的边界

契约正文已逐篇核对：公开UI读取构建时嵌入的split OpenAPI，Swagger生成/reset是另两条操作；当前通用schema比较零命中、注册自动比对只选择v2及JWKS，生成器与跨版本兼容尚无完整门禁。正文给出覆盖、元数据合并、可复现生成及兼容fixture四类候选，未修改机器契约或实现。SDK正文已深化宿主身份/范围/提交与生命周期责任，给出单独编译的公开API接入骨架，并区分策略重试、默认配置及观测/JWKS资源所有权。

- 契约文件存在不等于 runtime 已注册；
- runtime handler 存在不等于契约已发布；
- SDK 方法存在不等于服务端兼容；
- 本地 JWT 验签不等于 IAM 在线验证；
- 前端能力展示不等于服务端授权。

SDK配置默认值也不等于实际生效行为：当前Timeout不自动设置RPC deadline，默认连接策略重试却覆盖全部方法；非nil自定义熔断配置遗漏失败码，loader false及构造默认另有分支。公共符号编译检查不覆盖所有文档片段或下划线示例；旧AuthZ可执行示例的实际编译失败与本轮修正片段分别记录。具体事实由[SDK接入](02-Go-SDK与业务系统接入.md)维护，服务身份、范围消费及写入恢复由[gRPC授权](../02-业务模块/03-AuthZ/06-关键链路-gRPC服务间授权与SDK.md)维护。
