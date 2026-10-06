# 运行时：从启动到摘流量

> 状态：已实现 · 三篇运行时主文已逐篇深化；源码、局部回归与真实环境证明分别表述，当前没有完整关闭或生产验收结论。

本目录解释 iam-apiserver 如何解析运行模式、创建资源、装配模块、注册 REST/gRPC、运行后台任务、判定 readiness 并安全关闭。

## 阅读路径

1. [启动与组合根：装配顺序、准备副作用和失败边界](01-启动与组合根.md)：从标准入口到六阶段；固定模块计划、错误/状态/准入，准备失败及运行错误的不同清理合同。
2. [配置与传输装配：输入、生效条件和安全边界](02-配置与传输装配.md)：文件/env/flags、校验与映射的真实范围；模板/部署输入、REST注册和gRPC credentials的分层条件。
3. [后台任务、就绪与关闭：采样、停止和资源释放的合同](03-后台任务就绪与优雅关闭.md)：探针预算/在途drain、任务Start/Stop、两道资源门禁、关闭请求窗口和callback完成屏障。

专项记录：

- [IAM 重构与生产验收记录](08-IAM重构最终验收记录.md) 是证据台账，不是架构正文。

## 一张图

```text
cmd/app
  -> process PrepareRun
     -> RuntimeProfile
     -> MySQL/Redis/key + reliable messaging
     -> Container module graph
     -> REST + gRPC registration
     -> background tasks
  -> RunGroup
  -> readiness/drain
  -> ordered shutdown
```

图只列主阶段：Identity worker、Suggest Full/cron 与可选 mTLS reload 已在模块/传输构造阶段产生副作用；PrepareRun 失败没有统一清理它们。实际时机和候选改进归启动主文，不能把这条主线当成“所有任务在最后才启动”的保证。

## 关键边界

- process 拥有生命周期，不写业务规则；
- container 选择实现和连线，不处理请求；
- transport主要经application做协议适配；IDP查询例外见下文；
- application 编排用例，domain 守业务不变量，infra 实现端口；
- production/release 禁止 degraded startup；
- 端口监听、liveness、readiness 和业务数据新鲜度不能混为一谈。
- Container 的 initialized/Available 不表示逐模块成功；普通 Run error 也不自动执行信号关闭序列。
- ready 是采样，draining没有统一拒绝业务请求；任务取消不等于join，两个可靠停止timeout也不是全链路退出预算。

## 代码入口

- `cmd/apiserver/apiserver.go`
- `internal/apiserver/app.go`
- `internal/apiserver/process`
- `internal/apiserver/container`
- `internal/apiserver/transport/{rest,grpc}`
- `internal/pkg/server`、`internal/pkg/grpc`

## 验证

```bash
go test ./internal/apiserver/process ./internal/apiserver/container/... ./internal/apiserver/transport/rest/... ./internal/apiserver/transport/grpc/...
```

## 分层现状例外

transport经application访问基础设施是当前主要约束；IDP `GetWechatApp` 仍由gRPC service直接读取Repository并用Vault解密AppSecret，不能宣称全仓都遵循这一层间路径。敏感契约及责任见 [IDP接口与代码索引](../02-业务模块/04-IDP/04-模块边界与代码索引.md)。
