# REST 契约与接入

## 本文回答

本文只回答 6 件事：

1. REST 契约层与解释层今天怎么分工
2. 当前 REST 合同版图到底是什么
3. 当前路由注册与鉴权边界怎么读
4. 生成、重置与校验链今天怎么用
5. 接入方今天更推荐按什么顺序阅读
6. 运行时要去哪里继续核对

## 30 秒结论

- REST 的机器契约以 [../../api/rest/](../../api/rest/) 下的 `OpenAPI 3.1` 文件为准，`docs/` 只负责解释如何接入、如何验证、如何找到真实代码落点。
- 当前 REST 合同已经拆成 5 份：`authn`、`identity`、`authz`、`idp`、`suggest`；真实路由注册点分别落在 `internal/apiserver/interface/*/restful`，统一装配入口在 [../../internal/apiserver/routers.go](../../internal/apiserver/routers.go)。
- `internal/apiserver/docs/swagger.yaml` 和运行时 `/swagger/`、`/openapi/` 都是派生工件或调试入口，不是独立真值层；提交前应优先看 `api/rest/*.yaml`，再用 `make api-validate` 校验有没有漂移。
- 鉴权边界不能只靠概括性 README 判断：`identity`、`authz` 与 `suggest` 在路由层明确挂了 JWT 中间件；`authn` 以公开登录/JWKS 为主；`idp` 当前没有在 router 层统一挂 JWT。
- [../../api/rest/README.md](../../api/rest/README.md) 里仍有一批历史示例路径沿用旧写法，例如 `/api/v1/auth/*`、`/api/v1/profiles/*`；接入时应优先相信逐份 YAML 和真实 router，而不是只看这份总 README。

## 重点速查

| 关注点 | 当前答案 | 真实落点 |
| ---- | ---- | ---- |
| REST 合同根目录 | `OpenAPI 3.1` 机器契约 | [../../api/rest/](../../api/rest/) |
| 运行时统一装配点 | REST 路由最终都在这里挂到 `gin.Engine` | [../../internal/apiserver/routers.go](../../internal/apiserver/routers.go) |
| Authn 路由 | 登录、令牌、账户、JWKS | [../../internal/apiserver/interface/authn/restful/router.go](../../internal/apiserver/interface/authn/restful/router.go) |
| Identity 路由 | 用户、儿童、监护关系 | [../../internal/apiserver/interface/uc/restful/router.go](../../internal/apiserver/interface/uc/restful/router.go) |
| Authz 路由 | 角色、分配、策略、资源 | [../../internal/apiserver/interface/authz/restful/router.go](../../internal/apiserver/interface/authz/restful/router.go) |
| IDP 路由 | 微信应用管理 | [../../internal/apiserver/interface/idp/restful/router.go](../../internal/apiserver/interface/idp/restful/router.go) |
| Suggest 路由 | 儿童联想搜索 | [../../internal/apiserver/interface/suggest/restful/handler.go](../../internal/apiserver/interface/suggest/restful/handler.go) |
| Swagger 生成物 | 代码注解生成的比对工件 | [../../internal/apiserver/docs/swagger.yaml](../../internal/apiserver/docs/swagger.yaml) |
| 校验入口 | `spectral` + schema drift + route drift | `make api-validate`、[../../scripts/validate-openapi.sh](../../scripts/validate-openapi.sh) |
| 路径重置入口 | 从 swagger 拆分回 `api/rest/*.yaml` | `make docs-reset`、[../../scripts/reset-openapi-from-swagger.py](../../scripts/reset-openapi-from-swagger.py) |
| 本地 REST 监听 | 开发环境 `18081/18441` | [../04-基础设施与运维/04-端口&证书与数据库迁移.md](../04-基础设施与运维/04-端口&证书与数据库迁移.md)、[../../configs/apiserver.dev.yaml](../../configs/apiserver.dev.yaml) |

## 1. REST 契约层与解释层今天怎么分工

```mermaid
flowchart LR
    Contract["api/rest/*.yaml<br/>REST 机器契约"]
    Docs["docs/03-接口与集成<br/>解释层"]
    Runtime["interface/*/restful + routers.go<br/>运行时注册"]
    Swagger["internal/apiserver/docs/swagger.yaml<br/>生成物"]
    Validate["make api-validate<br/>校验链"]
    Caller["接入方 / 调试者"]

    Contract --> Docs
    Contract --> Runtime
    Runtime --> Swagger
    Contract --> Validate
    Swagger --> Validate
    Docs --> Caller
    Contract --> Caller
```

**图意**：REST 接入不是“只看 OpenAPI”或“只看 handler”。更准确的读法是：`api/rest` 给合同，router/handler 给运行时真相，`docs/` 给消费建议，swagger 与 `make api-validate` 负责把这几层拉回一致。

| 层 | 主要回答什么 |
| ---- | ---- |
| `api/rest/*.yaml` | 路径、方法、字段、请求体、响应体、`security`、兼容性边界 |
| `internal/apiserver/interface/*/restful/*.go` | 真实注册了哪些路由、handler 怎么装配、Swagger 注解从哪里来 |
| `internal/apiserver/docs/swagger.yaml` | 从代码生成出来、用于和 OpenAPI 做比对的派生工件 |
| `docs/03-接口与集成/*` | 调用方该看哪份合同、如何接、如何验证理解没漂移 |

这意味着：

- 改字段、路径、请求体、响应体时，优先改 `api/rest/*.yaml`
- 改 handler 注解、路由注册或模型后，要重新生成并校验 swagger
- `docs/` 不应该再复制一遍字段定义，只做导航、边界说明和验证指路

## 2. 当前 REST 合同版图到底是什么

### 2.1 合同文件与模块映射

| 合同文件 | 主要能力 | 运行时注册点 |
| ---- | ---- | ---- |
| [../../api/rest/authn.v1.yaml](../../api/rest/authn.v1.yaml) | 登录、刷新、验证、登出、账户、JWKS | [../../internal/apiserver/interface/authn/restful/router.go](../../internal/apiserver/interface/authn/restful/router.go) |
| [../../api/rest/identity.v1.yaml](../../api/rest/identity.v1.yaml) | 当前用户、儿童档案、监护关系 | [../../internal/apiserver/interface/uc/restful/router.go](../../internal/apiserver/interface/uc/restful/router.go) |
| [../../api/rest/authz.v1.yaml](../../api/rest/authz.v1.yaml) | 角色、策略、资源、Assignment | [../../internal/apiserver/interface/authz/restful/router.go](../../internal/apiserver/interface/authz/restful/router.go) |
| [../../api/rest/idp.v1.yaml](../../api/rest/idp.v1.yaml) | 微信应用管理与密钥轮换 | [../../internal/apiserver/interface/idp/restful/router.go](../../internal/apiserver/interface/idp/restful/router.go) |
| [../../api/rest/suggest.v1.yaml](../../api/rest/suggest.v1.yaml) | 儿童联想搜索 | [../../internal/apiserver/interface/suggest/restful/handler.go](../../internal/apiserver/interface/suggest/restful/handler.go) |

### 2.2 当前路径族

当前对外 REST 路径以 `servers.url = .../api/v1` 为基底，再叠加各份合同里的路径前缀：

| 路径族 | 典型路径 | 说明 |
| ---- | ---- | ---- |
| Public / Base | `/.well-known/jwks.json`、`/health`、`/ping`、`/api/v1/public/info` | 基础健康检查、公开信息与 JWKS |
| Authn | `/api/v1/authn/login`、`/api/v1/authn/refresh_token` | 登录、令牌生命周期、账户、JWKS 管理；其中 `authn/admin/jwks/*` 为管理员控制面 |
| Identity | `/api/v1/identity/me`、`/api/v1/identity/profiles/register` | 当前用户、儿童、监护关系 |
| Authz | `/api/v1/authz/roles`、`/api/v1/authz/policies` | 授权管理面 |
| IDP | `/api/v1/idp/wechat-apps` | 微信应用管理 |
| Suggest | `/api/v1/suggest/profile` | 儿童联想搜索 |

### 2.3 总 README 与逐份合同的边界

[../../api/rest/README.md](../../api/rest/README.md) 当前更像“总入口 + 历史示例集合”，而不是逐路径都严格与运行时同步的单一真值。

已核对到的当前差异包括：

- README 中仍大量使用旧前缀，例如 `/api/v1/auth/login`、`/api/v1/profiles/register`
- 当前逐份 OpenAPI 与 runtime router 已切到 `/api/v1/authn/*`、`/api/v1/identity/*` 这类新路径族

因此接入方的正确顺序应是：

1. 先看对应的 `api/rest/*.yaml`
2. 再看对应 router / handler
3. 最后把 `api/rest/README.md` 当总导航，而不是当逐路径的最终依据

## 3. 当前路由注册与鉴权边界怎么读

### 3.1 路由层已经明确能证明的部分

| 路径族 | 当前状态 | 证据 |
| ---- | ---- | ---- |
| `/.well-known/jwks.json` | `已实现`：公开端点，无需 JWT | [../../internal/apiserver/interface/authn/restful/router.go](../../internal/apiserver/interface/authn/restful/router.go) |
| `/api/v1/authn/*` 登录 / 刷新 / 验证 / 账户 | `已实现`：当前 router 层未统一挂 JWT 中间件 | [../../internal/apiserver/interface/authn/restful/router.go](../../internal/apiserver/interface/authn/restful/router.go) |
| `/api/v1/authn/admin/jwks/*` | `已实现`：当前要求 `JWT + admin role`；管理员鉴权能力不可用时 fail-closed 不注册 | [../../internal/apiserver/interface/authn/restful/router.go](../../internal/apiserver/interface/authn/restful/router.go)、[../../internal/apiserver/routers.go](../../internal/apiserver/routers.go) |
| `/api/v1/identity/*` | `已实现`：当前在 `api` 组上统一 `Use(deps.AuthMiddleware)` | [../../internal/apiserver/interface/uc/restful/router.go](../../internal/apiserver/interface/uc/restful/router.go) |
| `/api/v1/suggest/*` | `已实现`：当前在 `group` 上按依赖注入情况挂 JWT 中间件 | [../../internal/apiserver/interface/suggest/restful/handler.go](../../internal/apiserver/interface/suggest/restful/handler.go) |
| `/api/v1/authz/*` | `已实现`：`/health` 外当前统一挂 `AuthMiddleware` | [../../internal/apiserver/interface/authz/restful/router.go](../../internal/apiserver/interface/authz/restful/router.go)、[../../internal/apiserver/routers.go](../../internal/apiserver/routers.go) |
| `/api/v1/idp/*` | `待补证据`：当前 router 层没有统一 JWT guard | [../../internal/apiserver/interface/idp/restful/router.go](../../internal/apiserver/interface/idp/restful/router.go) |

### 3.2 不要讲过头的地方

- 不要把“顶层 OpenAPI 有 `bearerAuth`”直接讲成“所有 REST 路由都已经统一挂了 JWT 中间件”
- 不要把“README 里写了某条路径”直接讲成“当前运行时就是这条路径”
- 不要把“某个 handler 上有 Swagger 注解”直接讲成“它已经被对外正式承诺”；仍要回看 router 是否真的注册

## 4. 生成、重置与校验链今天怎么用

当前 REST 合同维护链是明确存在的：

| 动作 | 命令 | 作用 |
| ---- | ---- | ---- |
| 重新生成 swagger | `make docs-swagger` | 从代码注解生成 [../../internal/apiserver/docs/swagger.yaml](../../internal/apiserver/docs/swagger.yaml) |
| 从 swagger 拆分回 OpenAPI | `make docs-reset` | 按路径前缀规则重写 `api/rest/*.yaml` 的 `paths` / `tags` / `schemas` |
| 契约校验 | `make api-validate` | 执行 `spectral` lint、schema drift 比对、route drift 比对 |

对应脚本：

- [../../scripts/validate-openapi.sh](../../scripts/validate-openapi.sh)
- [../../scripts/check-openapi-contracts.py](../../scripts/check-openapi-contracts.py)
- [../../scripts/check-route-contracts.py](../../scripts/check-route-contracts.py)
- [../../scripts/reset-openapi-from-swagger.py](../../scripts/reset-openapi-from-swagger.py)

### 4.1 当前校验链覆盖了什么

| 检查 | 当前状态 |
| ---- | ---- |
| `api/rest/{authn,identity,authz,idp}.v1.yaml` 与 swagger 路由一致性 | `已实现` |
| `api/rest/{authn,identity,authz,idp}.v1.yaml` 与 swagger schema 一致性 | `已实现` |
| `suggest.v1.yaml` 纳入同一套 route/schema 比对 | `待补证据`：当前两个比对脚本都还没有把 `suggest` 加入 `REST_SPECS` |

这意味着：

- `suggest.v1.yaml` 虽然存在，也有 runtime route，但当前没有被 `check-openapi-contracts.py` / `check-route-contracts.py` 纳入同级别漂移校验
- 如果后续把 `suggest` 提升为更稳定的对外合同，应优先补上这条校验链

## 5. 接入方今天更推荐按什么顺序阅读

### 5.1 如果你是前端或外部集成方

1. 先看对应的 `api/rest/*.yaml`
2. 再看 [../../api/rest/README.md](../../api/rest/README.md) 获取整体导航
3. 若遇到路径或安全语义疑问，回到 router / handler 核实
4. 若涉及本地联调端口，再看 [../04-基础设施与运维/04-端口&证书与数据库迁移.md](../04-基础设施与运维/04-端口&证书与数据库迁移.md)

### 5.2 如果你在仓库里改 REST 能力

建议最少执行：

1. 改 router / handler / DTO / Swagger 注解
2. `make docs-swagger`
3. 如需按当前分包规则回写 OpenAPI，执行 `make docs-reset`
4. `make api-validate`
5. 同步更新本组解释文档或对应业务域文档

## 6. 运行时要去哪里继续核对

除了仓库文件本身，当前进程还暴露了两个与 REST 合同相关的查看入口：

| 路径 | 作用 | 注册位置 |
| ---- | ---- | ---- |
| `/openapi/` | 直接静态暴露 `api/rest` 嵌入文件 | [../../internal/apiserver/routers.go](../../internal/apiserver/routers.go) |
| `/swagger/` | Swagger UI | [../../internal/apiserver/routers.go](../../internal/apiserver/routers.go) |

这两个入口更适合调试和人工浏览；正式接入与代码评审，仍应回到仓库里的合同文件与校验命令。

## 继续往下读

| 文档 | 说明 |
| ---- | ---- |
| [README.md](./README.md) | 接口与集成层入口 |
| [02-gRPC契约与接入.md](./02-gRPC契约与接入.md) | gRPC 合同、metadata、接入方式 |
| [../../api/rest/README.md](../../api/rest/README.md) | REST 合同总导航 |
| [05-QS接入IAM.md](./05-QS接入IAM.md) | 业务方接入 IAM 的整体流程 |
| [../04-基础设施与运维/04-端口&证书与数据库迁移.md](../04-基础设施与运维/04-端口&证书与数据库迁移.md) | 端口、监听、开发/生产环境差异 |
