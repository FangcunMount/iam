# 关键链路：REST 管理与路由授权

> 状态：已实现 · 本文以当前 Gin router、AuthN/AuthZ middleware、permission catalog 与 OpenAPI 为依据。

## 结论

REST v3 只承担 AuthZ 管理；外部服务的授权判定由 gRPC v3 `Check` 提供。IAM 自身的管理路由先由 AuthN middleware 建立 Principal，
再由 AuthZ middleware 使用明确的 Resource/Action 施权；服务间写入使用可信服务身份与 Assignment 约束。

## REST v3：管理接口

REST 路由统一挂在 `/api/v3/authz`：

| 资源 | 主要路径 | 用途 |
| --- | --- | --- |
| Role | `/api/v3/authz/roles` | 创建、查询、更新、删除角色 |
| Assignment | `/api/v3/authz/assignments` | 增量授予、撤销与查询直接关系 |
| PermissionGrant | `/api/v3/authz/grants` | 管理角色能力 |
| RoleInheritance | `/api/v3/authz/role-inheritances` | 管理角色继承边 |
| Resource | `/api/v3/authz/resources` | 管理资源和对象属性 schema |

完整 method/path 以 `api/rest/authz.v3.yaml` 为准。REST 不提供 `/api/v3/authz/check`；需要判定的可信服务调用 gRPC。

REST 是控制面，不是请求期权限决策面。若业务服务为了判定而调用 Role/Grant 列表并在本地重新实现 matcher，就会绕过快照、ConstraintSet 和 Decision 语义。
服务间正确路径见 [gRPC 服务间授权与 SDK](06-关键链路-gRPC服务间授权与SDK.md)。

## AuthZ REST 路由与 Permission 矩阵

| Method + Path | Resource | Action | 业务语义 |
| --- | --- | --- | --- |
| `POST /api/v3/authz/roles` | `iam:authz:collection:roles` | `create` | 创建 Role |
| `GET /api/v3/authz/roles` | 同上 | `list` | 列出 Role |
| `GET /api/v3/authz/roles/:id` | 同上 | `read` | 读取当前请求租户的 Role |
| `PUT /api/v3/authz/roles/:id` | 同上 | `update` | 更新 Role |
| `DELETE /api/v3/authz/roles/:id` | 同上 | `delete` | 删除未被引用 Role |
| `GET /api/v3/authz/roles/:id/assignments` | `iam:authz:collection:assignments` | `list` | 按 Role 列直接 Assignment |
| `POST /api/v3/authz/assignments/grant` | 同上 | `grant` | 增量授予 Assignment |
| `POST /api/v3/authz/assignments/revoke` | 同上 | `revoke` | 按 Subject+Role 撤销 |
| `DELETE /api/v3/authz/assignments/:id` | 同上 | `revoke` | 按 Assignment ID 撤销 |
| `GET /api/v3/authz/assignments/subject` | 同上 | `list` | 按 Subject 列直接 Assignment |
| `POST /api/v3/authz/grants` | `iam:authz:collection:permission_grants` | `create` | 创建 managed PermissionGrant |
| `DELETE /api/v3/authz/grants/:id` | 同上 | `revoke` | 撤销 Grant |
| `GET /api/v3/authz/roles/:id/grants` | 同上 | `list` | 列角色的 Grant |
| `POST /api/v3/authz/role-inheritances` | `iam:authz:collection:role_inheritances` | `grant` | 增加 child→parent 边 |
| `GET /api/v3/authz/role-inheritances` | 同上 | `list` | 列继承边 |
| `DELETE /api/v3/authz/role-inheritances/:id` | 同上 | `revoke` | 撤销继承边 |
| `POST /api/v3/authz/resources` | `iam:authz:collection:resources` | `create` | 注册 Resource catalog |
| `GET /api/v3/authz/resources` | 同上 | `list` | 列 Resource |
| `GET /api/v3/authz/resources/:id` | 同上 | `read` | 按 ID 读 Resource |
| `GET /api/v3/authz/resources/key/:key` | 同上 | `read` | 按 key 读 Resource |
| `PUT /api/v3/authz/resources/:id` | 同上 | `update` | 更新 action/schema |
| `DELETE /api/v3/authz/resources/:id` | 同上 | `delete` | 删除未被引用 Resource |
| `POST /api/v3/authz/resources/validate-action` | 同上 | `validate_action` | 验证 catalog 是否登记 Action |

`GET /api/v3/authz/health` 是例外：它在受保护路由组之前注册，只返回 `status=ok,module=authz`。它不证明 runtime snapshot、MySQL、
policy subscriber 或全局 readiness 正常。

## 路由注册的 fail-closed 边界

AuthZ router 先注册模块局部 health，然后要求 Role handler、JWT `AuthRequired` 和 `PermissionOrGlobal` 都存在才继续注册受保护组。可选 handler 不存在时，
对应子路由不注册，而不是以无中间件方式暴露。

组合根还会检查 AuthZ module status、路由依赖和 JWT middleware。如果模块可用但授权中间件不可用，受保护路由应该整组不注册，不应退化为只验 JWT。

## 身份与信任边界

| 调用面 | 可信身份 | 额外限制 |
| --- | --- | --- |
| IAM REST 管理路由 | AuthN 用户 JWT | `RequirePermissionOrGlobal(Resource, Action)` |
| 调试/运维路由 | AuthN 用户 JWT | 明确的运维 Resource/Action |

请求体中的 Subject、角色名或 actor 字符串不能替代传输层认证结果。AuthN middleware 只负责认证并写入可信请求上下文，不持有 Resource/Action，也不执行授权判定。

REST 路由上的 Principal 来自 AuthN token verifier 返回的已验证 claims。JWT middleware 将 UserID、LoginIdentityID、TenantDomain、
OrgID 和 TokenID 写入 request context。AuthZ `RouteDecisionService` 只使用其中 UserID 构造 `subject.Ref`，
用 TenantDomain 作为当前 Tenant，再把路由能力转换为领域 `Request`。

这意味着：

- URL/query/body 中的 `user_id` 是被管理对象，不是当前操作者身份。
- 客户端自报 Tenant header 不应覆盖已验证 token claims 的 Tenant 语义。
- handler 内使用的 changed-by 应从 request context 中的已验证 UserID 派生，而不是接受任意 body actor。
- 路由授权在 handler 之前完成，handler 的领域校验仍然必须保留，两者分别保护“能否做”与“事实是否合法”。

## `RequirePermissionOrGlobal`

Resource 目录写路由使用 `RequirePlatformPermission`，只对 platform 求值；应用服务再次验证可信 actor，保护进程内调用。租户管理员保留 read/list/validate_action，角色名称不构成授权证据。

其余采用 `RequirePermissionOrGlobal` 的管理路由授权顺序是：

1. 使用当前 Tenant 检查指定 Resource/Action；
2. 当前 Tenant 不允许时，使用平台域再次检查同一 Resource/Action；
3. 两次都不允许则拒绝。

更精确的错误组合如下：

| 当前 Tenant | 平台域 | 结果 |
| --- | --- | --- |
| allow | 不再检查 | 放行，记录 `domain_permission` |
| deny | allow | 放行，记录 `global_permission` |
| error | allow | 放行；平台匹配仍可成为独立证据 |
| deny | deny | 403 |
| error | deny/error | 500；策略不可用错误返回 503 |
| 当前已是 platform 且 deny | 不重复检查 | 403 |
| 当前已是 platform 且 error | 不重复检查 | 普通内部错误 500，策略不可用 503 |

AuthZ middleware 还对 `domain_permission`、`global_permission`、`denied`、`unauthenticated`、`error` 做低基数记录。这些结果是路由授权观测，
不代替 runtime Check 的 allowed/denied/error 指标。

这里没有 `super_admin`、`tenant_admin` 等角色名旁路。当前 bootstrap 通过平台域通配 PermissionGrant 提供全局能力，但中间件代码本身接受平台域内任何匹配 Grant。
若要把“只有平台通配可全局放行”提升为强不变量，需要额外代码或数据门禁。

当前代码注释说平台通配 Grant 是唯一全局授权机制，但实现并未检查 matched Grant 是否通配。所以文档必须以代码行为为准：“平台域中任何匹配的 PermissionGrant 均可放行”；
bootstrap 中的通配设计是当前数据基线。

## AuthZ 管理路由

Role、Assignment、Grant、RoleInheritance 和 Resource 路由分别绑定各自 Resource/Action。新增 handler 时必须同时更新：

- route registry；
- permission catalog；
- bootstrap/迁移所需 Grant；
- OpenAPI；
- route-contract 和 docs-facts 门禁。

不能用“已经登录”替代管理权限，也不能通过角色名称直接绕过 Grant。

REST handler 主要做四件事：绑定 DTO，从 URL/query/context 获取 ID 与 Tenant，构造 application command/query，将领域错误映射为 HTTP 响应。
它不应在 handler 内手工复制继承环、Grant schema 或事务版本校验。

用户端 REST 使用数据库 ID 定位 Role/Resource/Grant/Inheritance；服务间 Assignment gRPC 为降低对 IAM 内部 ID 的耦合使用 stable role name。
两条传输路径最终仍必须进入同一 application/domain/UoW 不变量。

## 跨模块路由如何复用 AuthZ

AuthZ route authorizer 不只保护 `/api/v3/authz` 路由：

| 模块/路由类型 | Resource | Action 特征 | 授权方式 |
| --- | --- | --- | --- |
| AuthN JWKS 管理 | `iam:authn:collection:jwks` | rotate/retire 等明确动作 | `RequirePermissionOrGlobal` |
| AuthN Session 撤销 | `iam:authn:collection:sessions` | `revoke`、`revoke_by_login_identity`、`revoke_by_user` | `RequirePermissionOrGlobal` |
| IDP WeChat App 管理 | `iam:idp:collection:wechat_apps` | CRUD/list | `RequirePermissionOrGlobal` |
| Suggest 搜索入口 | `iam:identity:collection:profiles` | `search` | 当前 Tenant `RequirePermission` |
| Cache governance debug | `iam:ops:collection:cache_governance` | `read` | 生产必须 `RequirePermissionOrGlobal` |

这意味着 permission catalog 已是跨模块的路由合同。改 AuthN 管理 URL 时，不能只更新 AuthN 文档；还必须确认 Resource/Action、
bootstrap Grant 与 route contract 仍对齐。

## AuthN 管理路由

AuthN 的公开 JWKS 与受保护管理接口要区分：

- 公共 JWKS 只用于验签公钥发布；
- 管理 JWKS 与 Session 撤销路由使用用户 JWT；
- 路由分别检查 `jwks` 或 `sessions` Resource 下的明确 Action；
- 同样遵循当前 Tenant 后平台域的授权顺序。

具体路径与动作见 [AuthN：JWKS 与本地验签](../02-AuthN/06-关键链路-JWKS与本地验签.md)和
[Session、Token 与 JWKS](../02-AuthN/03-Session-Token与JWKS.md)。

## Suggest 接入

Suggest 不再根据旧的超级管理员布尔标志或角色名决定搜索范围。当前规则是：

- 平台域命中 `iam:identity:collection:profiles/list`，得到 AllProfile capability；
- 手机号搜索还需要 `iam:identity:collection:profiles/search_by_mobile`；
- Tenant 范围与最终查询仍由 Suggest/Identity 的业务链路处理。

这里的 `TenantDomain` 是 IAM 授权域，不是“Casbin domain”的对外契约。

Suggest 有两层授权：外层路由先要求当前 Tenant 的 `profiles/search`，进入 provider 后再用平台域的 `profiles/list` 决定是否获得 AllProfile scope，
手机号搜索另需 `search_by_mobile`。任何一层都不读 `super_admin` 角色名或旧布尔字段。

## OpenAPI、Router 与 README 的责任

| 事实 | 首要真相源 |
| --- | --- |
| 运行时是否注册 method/path | Gin router |
| 对外 request/response schema | `api/rest/authz.v3.yaml` |
| 路由需要的 Resource/Action | router middleware 绑定 + permission catalog |
| 读者导航与边界 | `api/rest/README.md` 与本文 |

理想状态下四者一致，但它们不是同一层证据。OpenAPI 有路径不能证明当前组合根已注册；router 有路径也不能证明 README 中的 curl URL 没有遗留旧前缀。

docs-facts 现在会抽取 README 中带 HTTP method 的 URL，并与 OpenAPI 及少量明确的 runtime-only 路由对齐。这是反漂移检查，不是 OpenAPI 完整性或生产可达性验收。

## 失败语义

| 失败 | 预期类型 | 不应做的降级 |
| --- | --- | --- |
| token 缺失/无效 | 401/认证错误 | 进入授权或 handler |
| 已认证但两个 Tenant 都 deny | 403 | 根据 role name 放行 |
| routeAuth 未配置 | 500 | 只做 JWT 后放行 |
| authorization runtime 错误 | 500 | 转成 403 隐藏故障 |
| handler DTO/领域输入错误 | 4xx | 跳过 command constructor |
| 引用冲突/已存在 | 冲突类业务错误 | 伪装成成功并静默忽略 |
| DB/UoW 失败 | 5xx | 留下部分版本/事件 |

## 测试与门禁

- `router_permissions_test.go` 锁定 AuthZ 子路由的 Resource/Action 绑定。
- `router_matrix_test.go` 锁定路由注册矩阵与模块局部 health。
- AuthN middleware 测试锁定 token 验证与 Principal 上下文；AuthZ middleware 测试锁定 current Tenant→platform 顺序、allow/deny/error 组合。
- `check-route-contracts.py` 比对实际路由与 permission catalog/contract。
- `check-openapi-contracts.py` 比对 OpenAPI 关键契约。
- `check-docs-facts.py` 锁定 REST 管理与 gRPC Check 分工，并校验 README 请求 URL。

一项门禁通过只能证明它编码的事实。例如 router matrix 通过不证明生产 ingress 已暴露路由，docs URL 通过也不证明 request schema 每个字段都已验收。

## 接入变更清单

新增 Resource/Action 或调用方时，至少检查：

1. Resource 注册和 attribute schema；
2. PermissionGrant 数据与平台/租户边界；
3. route registry 与中间件；
4. gRPC 服务 ACL 和 Assignment constraints；
5. OpenAPI/proto/SDK；
6. bootstrap、维护校验与多实例 reload；
7. 拒绝路径、条件 Grant 和继承角色测试。

8. README 中带 HTTP method 的请求 URL，以及退役的 v2 AuthZ 前缀或 REST `check` 引用。

## 路由设计评审问题

1. 这个端点是管理授权事实，还是判定业务对象？后者应优先 gRPC Check。
2. Resource/Action 是真正的业务能力，还是为了迎合 HTTP verb 随意命名？
3. 这个动作需要当前 Tenant 能力，还是允许平台域 fallback？
4. 路由缺少 AuthZ 依赖时是不注册/返错，还是会意外放行？
5. OpenAPI、router、permission catalog、bootstrap 和 README 的 method/path 是否一致？

## 角色详情与不可用错误

Handler 从认证请求上下文提取租户，调用 `GetRoleByID(ctx, tenant.ID, roleID)`；SQL 同时限定 tenant_id 与 id。其他租户的角色和不存在 ID 均返回 `ErrRoleNotFound` / 404。平台匹配 Grant 只满足路由准入，不赋予跨租户详情读取。

任一首次检查返回 `ErrAuthorizationPolicyUnavailable` 时，中间件立即保留错误并返回 503。它不作为普通 DENY，也不继续寻找平台授权旁路。新鲜度合同见 [多实例策略收敛](04-关键链路-多实例策略收敛.md)。
