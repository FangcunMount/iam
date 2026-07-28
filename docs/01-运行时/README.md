# 运行时：从启动到摘流量

> 状态：已实现 · 启动阶段、运行模式、传输注册、readiness 与关闭顺序已按当前 process/container 实现复核。

本目录解释 iam-apiserver 如何解析运行模式、创建资源、装配模块、注册 REST/gRPC、运行后台任务、判定 readiness 并安全关闭。

## 阅读路径

1. [启动、生命周期与组合根](01-启动与组合根.md)
2. [配置、运行模式与传输装配](02-配置与传输装配.md)
3. [后台任务、Readiness 与优雅关闭](03-后台任务就绪与优雅关闭.md)

专项记录：

- [IAM 重构最终验收记录](08-IAM重构最终验收记录.md) 是证据台账，不是架构正文。

## 一张图

```text
cmd/app
  -> process PrepareRun
     -> RuntimeProfile
     -> MySQL/Redis/EventBus/key
     -> Container module graph
     -> REST + gRPC registration
     -> background tasks
  -> RunGroup
  -> readiness/drain
  -> ordered shutdown
```

## 关键边界

- process 拥有生命周期，不写业务规则；
- container 选择实现和连线，不处理请求；
- transport 做协议适配，不访问具体 repository；
- application 编排用例，domain 守业务不变量，infra 实现端口；
- production/release 禁止 degraded startup；
- 端口监听、liveness、readiness 和业务数据新鲜度不能混为一谈。

## 代码入口

- `cmd/apiserver/apiserver.go`
- `internal/apiserver/app.go`
- `internal/apiserver/process`
- `internal/apiserver/container`
- `internal/apiserver/transport/{rest,grpc}`
- `internal/pkg/server`、`internal/pkg/grpc`

## 验证

```bash
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./internal/apiserver/process ./internal/apiserver/container/... ./internal/apiserver/transport/rest/... ./internal/apiserver/transport/grpc/...
```
