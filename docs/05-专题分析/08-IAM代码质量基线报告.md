# IAM 代码质量基线报告：2026-04-23

**本文回答**：截至 2026 年 4 月 23 日，`iam` 当前工作树的代码质量基线是什么、这一轮已经治理了哪些高风险坏味道、还剩哪些主要结构性问题，以及下一阶段应该优先重构哪里。

---

## 30 秒结论

如果只看一屏，先记住下面这张表：

| 维度 | 当前事实 |
| ---- | -------- |
| 分支 / 时间 | 基于当前工作树，检查日期为 2026-04-23 |
| 基础质量门禁 | `go test ./...` 当前可稳定通过 |
| 结构健康度 | 分层仍然清楚，`domain / application / interface / infra / container` 主边界成立 |
| 本轮已完成的关键治理 | 关闭 fail-open 启动与路由、删除默认 IDP 密钥、收口监护关系自助边界、把 Authz 双写改成单库事务、限制 Assignment 只写真实存在的 `user` 主体 |
| 规模快照 | 131 个 Go package，其中 54 个 package 含测试，约占 41.2% |
| 当前最大剩余坏味道 | 少数应用服务编排偏长、`serviceauth` 仍偏重、部分基础 TODO 尚未收口、接口层测试仍不够均匀 |
| 结论判断 | 代码库已经从“结构成熟但安全边界松散”推进到“主风险已收口、可继续治理复杂度债务”的状态 |

---

## 本次检查范围

本报告基于 2026-04-23 当前工作树，重点检查以下内容：

1. 进程启动与关键模块装配是否默认失败关闭
2. 认证中间件缺失时受保护路由是否仍可能暴露
3. 监护关系接口是否把“当前用户”作为授权边界
4. Authz 写路径是否仍存在 assignment / Casbin / version 的双写不一致风险
5. Assignment 写接口是否允许不可验证主体写入授权
6. 当前仓库仍有哪些典型的可维护性坏味道

本次使用的主要命令：

```bash
go test ./...
go list ./... | wc -l
go list -f '{{.Dir}} {{len .TestGoFiles}} {{len .XTestGoFiles}}' ./...
rg -n 'TODO|FIXME|XXX' internal cmd pkg
```

---

## 核心指标

### 代码与测试规模

| 指标 | 数值 |
| ---- | ---- |
| Go package 总数 | 131 |
| 含测试文件的 package 数 | 54 |
| 含测试 package 占比 | 41.2% |
| 本次 `TODO/FIXME/XXX` 命中数 | 11 |

### 当前最明显的大文件热点

下表按源码行数排序，排除了 `pb.go` 和 Swagger 生成文件这类生成产物：

| 文件 | 行数 | 当前判断 |
| ---- | ---- | ---- |
| `internal/apiserver/application/uc/user/services_impl.go` | 694 | 应用服务编排长度偏大 |
| `internal/apiserver/application/uc/profile/services_impl.go` | 617 | 读写协调逻辑继续堆积 |
| `internal/apiserver/application/authn/register/services_impl.go` | 587 | 认证注册流程编排继续拉长 |
| `internal/apiserver/server.go` | 570 | 启动装配与模块初始化仍偏重 |

这组数字说明：`iam` 当前的首要问题已经不再是“目录没有分层”，而是少数热点文件开始承担过多 orchestration 责任。SDK 的包边界这轮已经明显收口，但编排复杂度债务仍然存在于若干应用服务和状态型 helper 中。

---

## 本次已落地的高风险治理

### 1. 启动与路由从 fail-open 改为 fail-closed

这轮之前，`Authn` 模块初始化失败时，HTTP 路由仍可能继续启动，并通过空中间件把受保护接口暴露出去。这个问题本质上不是“代码风格差”，而是安全边界默认值错误。

本次已完成：

1. 新增 `server.allow-degraded-startup`，默认 `false`
2. 默认模式下，关键模块或安全配置缺失会直接阻断启动
3. 显式降级模式下，受保护路由不再注册，而不是挂空中间件继续放行
4. `/health` 会暴露认证与 Authz runtime 的 degraded 状态

主要落点：

- [`../../internal/pkg/options/server_run_options.go`](../../internal/pkg/options/server_run_options.go)
- [`../../internal/apiserver/server.go`](../../internal/apiserver/server.go)
- [`../../internal/apiserver/routers.go`](../../internal/apiserver/routers.go)
- [`../../internal/apiserver/interface/uc/restful/router.go`](../../internal/apiserver/interface/uc/restful/router.go)
- [`../../internal/apiserver/interface/authz/restful/router.go`](../../internal/apiserver/interface/authz/restful/router.go)
- [`../../internal/apiserver/interface/suggest/restful/handler.go`](../../internal/apiserver/interface/suggest/restful/handler.go)

### 2. 删除 IDP 加密密钥的硬编码默认值

原先 `idp.encryption-key` 缺失时，会退化成固定字符串默认密钥。这是典型的“为了启动成功而牺牲安全默认值”的坏味道。

本次已完成：

1. 删除容器层固定默认密钥
2. `idp.encryption-key` 缺失或格式非法时，严格模式直接启动失败
3. 只有显式降级模式才允许跳过相关模块，而且不会继续暴露依赖认证的接口

主要落点：

- [`../../internal/apiserver/server.go`](../../internal/apiserver/server.go)
- [`../../internal/apiserver/container/container.go`](../../internal/apiserver/container/container.go)

### 3. 监护关系接口收口为“当前用户自助视角”

原先 `ref` REST 接口直接信任请求里的 `userId / profileId`。这会让“已登录”与“有权操作该用户/儿童”之间出现断层。

本次已完成：

1. `grant` 始终使用 JWT 中的当前用户作为 `UserID`
2. 请求体中的 `userId` 只做兼容性校验，不再作为授权来源
3. `list` 默认查当前用户，不允许跨用户 `user_id`
4. 按 `profile_id` 查询前，必须先确认当前用户对该儿童存在活跃监护关系

主要落点：

- [`../../internal/apiserver/interface/uc/restful/handler/ref.go`](../../internal/apiserver/interface/uc/restful/handler/ref.go)
- [`../../internal/apiserver/interface/uc/restful/request/ref.go`](../../internal/apiserver/interface/uc/restful/request/ref.go)
- [`../../internal/apiserver/interface/uc/restful/handler/ref_test.go`](../../internal/apiserver/interface/uc/restful/handler/ref_test.go)

### 4. Authz 写路径改成单库事务，不再靠伪补偿维持一致性

原先 assignment / policy 变更会分别写业务表、Casbin 规则表和版本表，但这些动作没有被统一事务包起来；一旦发生半成功、进程崩溃或 runtime reload 异常，就会留下不一致窗口。

本次已完成：

1. 引入 Authz UoW，把 assignment / policy / version / `casbin_rule` 纳入同一个数据库事务
2. 关闭 Casbin `AutoSave` 作为事实写入路径
3. 事务提交后才做 `InvalidateCache + LoadPolicy`
4. runtime reload 失败只影响节点健康，不再伪回滚已提交事实

主要落点：

- [`../../internal/apiserver/application/authz/uow/uow.go`](../../internal/apiserver/application/authz/uow/uow.go)
- [`../../internal/apiserver/application/authz/shared/reloader.go`](../../internal/apiserver/application/authz/shared/reloader.go)
- [`../../internal/apiserver/application/authz/assignment/command_service.go`](../../internal/apiserver/application/authz/assignment/command_service.go)
- [`../../internal/apiserver/application/authz/policy/command_service.go`](../../internal/apiserver/application/authz/policy/command_service.go)
- [`../../internal/apiserver/infra/mysql/casbinrule/repo.go`](../../internal/apiserver/infra/mysql/casbinrule/repo.go)
- [`../../internal/apiserver/infra/casbin/adapter.go`](../../internal/apiserver/infra/casbin/adapter.go)

### 5. Assignment 写入口收口到“真实存在的 user 主体”

原先 Assignment 验证器没有真正检查主体是否存在，还允许 `group` 等无法在本仓库里校验真值的主体类型进入写路径。

本次已完成：

1. Validator 注入 `user` 仓储，写入前强制校验主体存在
2. `Grant / Revoke` 只允许 `user` 主体
3. REST / gRPC / Swagger 同步收口为“仅 user 可写”
4. 不存在的用户不会再写出悬空授权记录

主要落点：

- [`../../internal/apiserver/domain/authz/assignment/validator.go`](../../internal/apiserver/domain/authz/assignment/validator.go)
- [`../../internal/apiserver/interface/authz/restful/dto/assignment.go`](../../internal/apiserver/interface/authz/restful/dto/assignment.go)
- [`../../internal/apiserver/interface/authz/grpc/service.go`](../../internal/apiserver/interface/authz/grpc/service.go)
- [`../../api/rest/authz.v1.yaml`](../../api/rest/authz.v1.yaml)
- [`../../internal/apiserver/docs/swagger.yaml`](../../internal/apiserver/docs/swagger.yaml)

---

## 这轮治理带来的实质收益

| 收益 | 含义 |
| ---- | ---- |
| 安全默认值更硬 | 关键模块失效时，不再用“继续启动”伪装可用性 |
| 权限边界更清楚 | Ref 与 Authz 写入口都回到了“当前用户 / 可验证主体”语义 |
| 授权事实更一致 | 业务表、版本表、Casbin 规则表在同一事务提交 |
| 运行状态更可观测 | `/health` 不再只看进程活着，还会反映 auth/authz runtime 健康 |
| 文档与实现更一致 | Swagger、REST 契约和运行时行为已经同步收口 |

这说明 `iam` 现在的主要问题，已经从“关键安全边界默认值错误”转移到“复杂度和可维护性债务”。

---

## 仍然存在的主要坏味道

### 1. SDK 公开面已收口，但 `serviceauth` 仍偏重

本轮已经把 SDK 的公开面固定为 `sdk / config / auth / authz / identity / idp / errors`，同时把 `transport / observability` 和高级错误分析能力收回 `pkg/sdk/internal/...`。SDK 侧的边界和热点已经明显收口，剩余更突出的复杂度债务重新回到了启动装配和应用服务编排层。

### 2. 基础真值与时间语义仍有未收口 TODO

当前最值得继续追的 TODO 包括：

1. [`../../internal/pkg/meta/id.go`](../../internal/pkg/meta/id.go) 里的 ID 发号器占位实现
2. [`../../internal/apiserver/application/authn/jwks/key_management.go`](../../internal/apiserver/application/authn/jwks/key_management.go) 与 [`../../internal/apiserver/domain/authn/jwks/key_rotation.go`](../../internal/apiserver/domain/authn/jwks/key_rotation.go) 里对 `CreatedAt / UpdatedAt` 的临时时间填充
3. [`../../internal/apiserver/interface/authn/restful/router.go`](../../internal/apiserver/interface/authn/restful/router.go) 中仍标记“待实现”的端点

这些问题不一定立刻导致线上漏洞，但它们说明“真实领域真值”还有局部是靠临时实现维持。

### 3. 接口层测试还不够均匀

虽然包级测试占比已经到 41.2%，但很多接口适配层、装配层和运行时边界仍然没有成体系测试。当前测试更偏向领域层、仓储层和一部分应用层，真正最容易出“集成错误”的地方，保护还不够密。

---

## 建议的下一轮重构顺序

建议按下面顺序推进：

1. 继续拆 `pkg/sdk/auth/serviceauth` 的时序逻辑，把刷新调度、状态统计、熔断回退再做一次职责细分
2. 收口 `meta.ID` 发号实现与 JWKS 时间字段 TODO，补齐真正的领域真值来源
3. 继续把测试往接口层与装配层推进，优先覆盖启动矩阵、路由暴露矩阵、Authz runtime degraded 行为

如果只允许打一轮小规模重构，我建议先做第 2 项。

---

## 如何复现这份报告

在仓库根目录执行：

```bash
go test ./...
go list ./... | wc -l
go list -f '{{.Dir}} {{len .TestGoFiles}} {{len .XTestGoFiles}}' ./... | awk '$2+$3>0{count++} END{print count+0}'
rg -n 'TODO|FIXME|XXX' internal cmd pkg
```

如需自行查看大文件热点，可执行：

```bash
python3 - <<'PY'
import os
files = []
for dp, _, fns in os.walk('.'):
    if '/vendor/' in dp:
        continue
    for fn in fns:
        if fn.endswith('.go') and not fn.endswith('.pb.go'):
            p = os.path.join(dp, fn)
            with open(p, 'r', encoding='utf-8') as f:
                files.append((sum(1 for _ in f), p))
for n, p in sorted(files, reverse=True)[:10]:
    print(n, p)
PY
```

---

## 最终判断

`iam` 现在已经不是“目录分层混乱、缺少结构意识”的代码库；它真正的问题是：

1. 之前有几处关键安全边界默认值错误
2. 少数热点文件正在积累复杂度债务
3. 一些基础真值和接口层测试仍然没有完全收口

这次重构已经把第 1 类高风险问题显著压下去了。下一阶段不应该再把主要时间花在“补最基础的安全边界”上，而应该转向：

1. 拆大文件与长编排链
2. 补齐基础真值 TODO
3. 把测试保护从领域层继续推到接口层和装配层

如果按这个顺序推进，`iam` 的代码质量会从“安全边界纠偏完成”继续走到“复杂度也开始可控”的阶段。
