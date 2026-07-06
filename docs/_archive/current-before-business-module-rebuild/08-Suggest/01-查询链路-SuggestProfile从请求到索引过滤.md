# 01-查询链路：SuggestProfile 从请求到索引过滤

> 本文回答：`GET /api/v2/suggest/profile` 请求进入 IAM apiserver 后，如何经过 REST Handler、限流、OperatingPrincipal 提取、application Service 编排、ProfileAccessScope 解析、搜索策略选择、索引召回、权限过滤、排序、手机号脱敏，最终返回 Profile 联想结果。

---

## 30 秒结论

| 问题 | 结论 |
| ---- | ---- |
| REST 入口是什么？ | `GET /api/v2/suggest/profile?k={keyword}&limit={limit}` |
| Handler 做什么？ | 绑定参数、提取 `OperatingPrincipal`、执行限流、调用 `SuggestProfile` use case |
| Service 做什么？ | 校验 principal、构建 keyword、解析 scope、选择策略、查询索引、记录指标、返回脱敏 DTO |
| ScopeProvider 做什么？ | 把当前 operating 用户解析为 `ProfileAccessScope` |
| 搜索策略怎么选？ | 手机号形态无权限直接空；纯数字走 Hash；非数字走 Trie |
| 索引如何避免越权？ | Store 先扩大召回 profileIDs，再用 `ProfileAccessScope` 过滤，再排序截断 |
| 手机号如何保护？ | 手机号形态关键词需要额外权限；返回结果只暴露 `mobile_mask` |
| 查询失败如何表现？ | 未认证返回 401；接口无权限由 route middleware 返回 403；限流返回 429；无结果返回空数组 |

核心链路：

```text
HTTP Request
  ↓
REST Handler
  ↓
RateLimiter
  ↓
SuggestProfile Service
  ↓
ProfileAccessScopeProvider
  ↓
ProfileSearchStrategy
  ↓
ProfileSuggestionIndex
  ↓
match -> scope filter -> rank
  ↓
mobile mask
  ↓
Response
```

---

## 1. 查询入口

Suggest REST 入口：

```http
GET /api/v2/suggest/profile?k={keyword}&limit={limit}
```

示例：

```http
GET /api/v2/suggest/profile?k=zhang&limit=20
GET /api/v2/suggest/profile?k=zs
GET /api/v2/suggest/profile?k=10001
GET /api/v2/suggest/profile?k=13800138000
```

参数：

| 参数 | 必填 | 说明 |
| ---- | ---- | ---- |
| `k` | 是 | 搜索关键词，支持中文、拼音、简拼、纯数字 |
| `limit` | 否 | 返回条数上限；超过配置上限会被 Service 截断 |

返回：

```json
[
  {
    "id": "10001",
    "name": "张三",
    "mobile_mask": "138****0000",
    "weight": 1
  }
]
```

注意：

```text
返回的是 mobile_mask，不是明文 mobile。
```

---

## 2. 路由注册与中间件

Suggest 路由注册在：

```text
/api/v2/suggest/profile
```

路由会挂载上层注入的 middlewares。

这些中间件通常负责：

1. 认证请求。
2. 注入用户身份上下文。
3. 执行接口级权限判断。
4. 保证 Handler 可以从 Gin context 中恢复 `OperatingPrincipal`。

也就是说，Suggest 有两层权限：

```text
接口级权限：是否可以调用 suggest profile search；
数据级权限：调用后能看到哪些 Profile 候选。
```

接口级权限在 route middleware 中完成。

数据级权限在 application / index 查询链路中通过 `ProfileAccessScope` 完成。

---

## 3. Handler 职责

`transport/rest/suggest.Handler.Profile` 是 REST 查询入口。

它的职责非常明确：

```text
1. 绑定 query 参数；
2. 从 Gin context 提取 OperatingPrincipal；
3. 执行 RateLimiter；
4. 调用 application/suggest.Service；
5. 处理错误；
6. 转换 REST response DTO；
7. 返回结果。
```

Handler 不负责：

- 搜索算法。
- 权限范围计算。
- 索引查询。
- 排序规则。
- 手机号脱敏细节。

这些都下沉到 application/domain/infra。

---

## 4. Handler 执行流程

Handler 的流程可以抽象为：

```go
func (h *Handler) Profile(c *gin.Context) {
    query := bindQuery(k, limit)

    principal, ok := OperatingPrincipalFromGin(c)
    if !ok {
        return 401
    }

    if h.limits != nil {
        keyword := suggest.NewKeyword(query.K)
        mobile := keyword.IsDigits() && suggest.LooksLikeMobile(keyword.String())
        if !h.limits.Allow(principal.OperatorID, mobile) {
            return 429
        }
    }

    list, err := h.svc.SuggestProfile(c, SuggestProfileRequest{
        Principal: principal,
        Keyword: query.K,
        Limit: query.Limit,
    })

    return response(list)
}
```

这里有几个关键点。

### 4.1 Principal 必须存在

如果无法从上下文提取 operating principal，Handler 会返回 401。

这意味着 Suggest 查询必须依赖已认证身份。

```text
没有 operating principal，就不能执行 Profile suggest。
```

### 4.2 限流在调用 Service 前执行

RateLimiter 放在 Handler 层是合理的。

原因：

1. 它属于接口保护，不属于领域规则。
2. 可以在进入应用用例之前拦截高频请求。
3. 可以区分普通关键词和手机号形态关键词。
4. 可以避免无意义地访问 scope provider / index。

### 4.3 手机号形态影响限流桶

Handler 会先判断：

```text
keyword.IsDigits() && LooksLikeMobile(keyword)
```

如果是手机号形态，就使用更严格的 mobile limiter。

普通关键词与手机号关键词可以有不同 quota。

---

## 5. OperatingPrincipalFromGin

`OperatingPrincipalFromGin` 负责把 Gin context 中的身份上下文转换成 suggest domain 所需的 `OperatingPrincipal`。

它不是完整用户模型，而是 suggest 所需的最小 operating 身份视图。

典型字段：

```text
OperatorID
TenantDomain
OrgIDs
RoleCodes
IsSuperAdmin
```

当前最重要的是：

```text
OperatorID：用于限流、owner scope、日志。
TenantDomain：IAM 授权域，例如 fangcun / platform。
OrgIDs：业务组织范围，如果上游上下文能提供。
```

注意：

```text
TenantDomain 不是 org_id。
```

Suggest 数据权限主路径是：

```text
OrgIDs / OperatorID / ProfileIDs
```

而不是 TenantDomain。

---

## 6. Service 职责

`application/suggest.Service` 是查询用例编排者。

它依赖：

```text
Config
ProfileSuggestionRuntime
ProfileAccessScopeProvider
ProfileSearchStrategy[]
SuggestMetrics
```

Service 不直接依赖具体 MySQL、Trie、Hash、Gin、Redis。

这说明 Service 的职责是应用编排，而不是基础设施实现。

---

## 7. SuggestProfile 用例流程

`SuggestProfile(ctx, req)` 的流程可以分成 9 步。

```text
1. 检查 service/runtime/scopeProvider 是否可用。
2. 校验 OperatingPrincipal。
3. 构造 Keyword。
4. 空 keyword 直接返回空数组。
5. ResolveProfileAccessScope。
6. 手机号形态关键词记录安全日志。
7. 获取当前索引 runtime.Current()。
8. 构造 Query 并选择搜索策略。
9. 执行策略，记录指标，转换 DTO。
```

---

## 8. Step 1：检查运行时依赖

Service 如果没有 runtime 或 scopeProvider，会返回空数组。

```text
runtime == nil        -> []
scopeProvider == nil -> []
```

这是一种安全降级策略。

原因：

```text
Suggest 不是 IAM 核心能力；
Suggest 不应因为索引不可用拖垮 AuthN/AuthZ/Identity；
依赖缺失时返回空结果比返回全局数据更安全。
```

---

## 9. Step 2：校验 Principal

如果：

```text
OperatorID <= 0 && !IsSuperAdmin
```

Service 返回 `ErrUnauthenticated`。

这说明：

```text
普通请求必须有明确 operatorID；
超级管理员场景可以通过 IsSuperAdmin 放行。
```

Handler 捕获 `ErrUnauthenticated` 后返回 401。

---

## 10. Step 3：构造 Keyword

Service 会使用 domain 层的 `NewKeyword` 规范化关键词。

Keyword 的职责包括：

```text
trim space；
判断是否为空；
判断是否纯数字。
```

空关键词直接返回空数组。

这避免了：

```text
空关键词触发全量索引扫描。
```

---

## 11. Step 4：解析 ProfileAccessScope

Service 调用：

```go
scope, err := scopeProvider.ResolveProfileAccessScope(ctx, req.Principal)
```

`ProfileAccessScopeProvider` 负责把当前 operating 用户解析成 Profile 可见范围。

可能来源包括：

```text
接口级角色 / 权限；
超级管理员判断；
业务组织范围；
当前操作员负责范围；
ProfileVisibilityIDsResolver 预计算结果；
手机号搜索权限。
```

Service 不关心这些权限如何计算。

Service 只消费结果：

```text
ProfileAccessScope
```

这保持了 Suggest 的边界：

```text
Suggest 不做完整权限计算；
Suggest 只做已解析 scope 的索引过滤。
```

---

## 12. Step 5：手机号形态关键词日志

如果 keyword 是：

```text
纯数字 && LooksLikeMobile(keyword)
```

Service 会记录日志：

```text
operator_id
tenant_domain
allow_mobile_search
keyword_len
```

注意：

```text
不会记录明文手机号。
```

这是非常重要的安全边界。

手机号搜索风险包括：

1. 用户枚举手机号。
2. 通过返回空/非空探测用户存在性。
3. 日志泄露敏感信息。
4. 结果暴露明文手机号。

因此当前设计同时做了：

```text
额外授权；
限流；
不记录明文 keyword；
返回 mobile_mask。
```

---

## 13. Step 6：获取当前索引

Service 调用：

```go
index := runtime.Current()
```

如果 index 为空，返回空数组。

这可能发生在：

```text
模块刚启动但索引未构建；
刷新失败且非 required 模式降级；
测试环境未注入 runtime；
Suggest 被配置关闭。
```

返回空数组是安全行为。

---

## 14. Step 7：计算 Limit

Service 会读取请求中的 `limit`，并与配置上限比较。

规则：

```text
limit <= 0              -> 使用 cfg.MaxResults
limit > cfg.MaxResults  -> 使用 cfg.MaxResults
正常 limit              -> 使用请求值
```

这避免前端传入过大的 limit 导致索引扫描、排序、返回过重。

---

## 15. Step 8：构造 Query

Service 构造 domain Query：

```text
Keyword
Limit
InternalLimit
KeyPadLen
WildcardKeyCap
```

字段说明：

| 字段 | 说明 |
| ---- | ---- |
| Keyword | 规范化后的搜索词 |
| Limit | 最终返回上限 |
| InternalLimit | 内部召回上限，通常大于 Limit |
| KeyPadLen | 关键字 padding 长度，兼容搜索策略 |
| WildcardKeyCap | Trie 前缀扩展 key 上限 |

为什么需要 `InternalLimit`？

因为必须先扩大召回，再按 scope 过滤。

如果内部只召回 20 条，过滤后可能没有结果。

---

## 16. Step 9：选择搜索策略

Service 调用：

```go
strategy := selectProfileSearchStrategy(strategies, keyword, scope)
```

当前策略链大致分为：

```text
mobileDeniedStrategy
numericExactStrategy
prefixTextStrategy
```

选择逻辑的目标是：

```text
把不同 keyword 形态交给不同索引能力处理。
```

---

## 17. mobileDeniedStrategy

场景：

```text
keyword 看起来像手机号
且 scope.AllowMobileSearch == false
```

行为：

```text
直接返回空数组
```

这不是错误，而是安全策略。

原因：

```text
手机号搜索具备更强的用户枚举风险；
没有 search_by_mobile 权限时，不应该继续查询 Hash；
也不应该通过结果差异泄露信息。
```

---

## 18. numericExactStrategy

场景：

```text
keyword 是纯数字
```

行为：

```text
Hash 精确匹配 ProfileID 或手机号
```

注意：

```text
Hash 不是手机号前缀搜索；
Hash 是精确匹配；
手机号形态关键词仍然必须先通过 mobileDeniedStrategy。
```

查询后仍然会经过：

```text
ProfileAccessScope filter
```

也就是说，即使手机号命中了某个 Profile，如果当前操作员没有可见权限，也不会返回。

---

## 19. prefixTextStrategy

场景：

```text
keyword 不是纯数字
```

行为：

```text
Trie 查询中文名 / 拼音 / 简拼
```

例如：

```text
张      -> 张三
zhang  -> 张三
zs     -> 张三
```

策略本身不做权限判断，权限判断在 Store 中统一执行。

---

## 20. Store 查询链路

无论策略是 Hash 还是 Trie，最终都会进入 `ProfileSuggestionIndex.SuggestProfile(query, scope)`。

Store 内部流程：

```text
1. 根据 keyword 选择 Hash 或 Trie 召回 matched profileIDs；
2. CompileProfileAccessScope(scope)；
3. 遍历 matchedIDs；
4. 从 terms 取 ProfileSearchTerm；
5. ScopePolicy.AllowsCompiled 判断是否可见；
6. 收集 visible terms；
7. ObserveIndexFilter(matched, visible)；
8. RankingPolicy.RankForQuery(query, visible, limit)；
9. 返回结果。
```

核心伪代码：

```go
matchedIDs := match(query)
compiled := CompileProfileAccessScope(scope)

visible := make([]ProfileSearchTerm, 0)
for _, id := range matchedIDs {
    term := terms[id]
    if ScopePolicy.AllowsCompiled(compiled, term) {
        visible = append(visible, term)
    }
}

return RankingPolicy.RankForQuery(query, visible, query.Limit)
```

---

## 21. ScopePolicy 过滤逻辑

`ScopePolicy` 的判断顺序可以理解为：

```text
AllProfile
  ↓
ProfileIDs contains term.ProfileID
  ↓
OrgIDs contains term.OrgID
  ↓
OperatorID in term.OwnerOperatorIDs
  ↓
TenantIDs contains term.TenantID（Deprecated）
```

当前数据权限主路径是：

```text
ProfileIDs / OrgIDs / OperatorID
```

`TenantIDs` 是历史兼容和未来 SaaS 预留，不应作为当前普通数据权限路径。

---

## 22. 为什么权限过滤放在 Store 中

权限过滤放在 Store 中，而不是 Handler 或 DTO 层，原因是：

1. Store 能在排序前过滤，避免无权限数据污染排序。
2. Store 能基于 `InternalLimit` 扩大召回后过滤。
3. Store 能统一 Hash / Trie 两种召回方式的权限处理。
4. Handler 不应该理解搜索索引结构。
5. DTO 层过滤太晚，容易出现 limit 之后过滤导致召回不足。

正确职责是：

```text
Handler：入口保护；
Service：用例编排；
ScopeProvider：解析权限范围；
Store：执行索引召回 + scope filter；
DTO：安全输出。
```

---

## 23. RankingPolicy 排序

排序发生在权限过滤之后。

RankingPolicy 负责：

1. 按 ProfileID 去重。
2. 同一 Profile 保留更高 Weight。
3. 根据 Query 做前缀命中优先。
4. 按 Weight 降序。
5. 按 Limit 截断。

这保证：

```text
无权限数据不会参与最终排序；
同一个 Profile 多 key 命中时不会重复返回；
搜索词前缀命中的候选更靠前。
```

---

## 24. DTO 转换与手机号脱敏

Store 返回的是 domain 层的 `ProfileSearchTerm`。

Service 会转换为 application DTO：

```text
ProfileSuggestItem
```

字段：

```text
ProfileID
DisplayName
MobileMask
Weight
```

手机号处理规则：

```text
默认：MaskMobiles(t.Mobiles)
DisableMobileMask=true：返回第一个明文手机号
```

但组合根已经限制：

```text
production 环境禁止 DisableMobileMask=true
```

所以生产环境不会返回明文手机号。

---

## 25. REST Response DTO

REST 层最终返回：

```text
ProfileSuggestResponseItem
```

典型 JSON：

```json
{
  "id": "10001",
  "name": "张三",
  "mobile_mask": "138****0000",
  "weight": 1
}
```

注意：

```text
id 通常以字符串形式返回，避免前端 number 精度问题；
mobile_mask 是脱敏字段；
不返回完整 mobile。
```

---

## 26. 错误与返回语义

| 场景 | 返回 |
| ---- | ---- |
| 缺少参数 k | 400 |
| 缺少 operating principal | 401 |
| Service 返回 ErrUnauthenticated | 401 |
| 接口级权限不足 | 403，由 route middleware 返回 |
| RateLimiter 拒绝 | 429 |
| scope 无可见范围 | 200 + `[]` |
| index 未初始化 | 200 + `[]` |
| 手机号形态无权限 | 200 + `[]` |
| 无匹配结果 | 200 + `[]` |

为什么手机号无权限不是 403？

因为：

```text
接口级 search 权限已经在路由层判断；
手机号搜索是更细的搜索策略权限；
返回空数组可以减少手机号枚举侧信道。
```

---

## 27. 指标记录

查询链路中记录两类指标。

### 27.1 Service 层查询指标

Service 记录：

```text
strategy name
result count
是否手机号形态关键词
```

用于观察：

```text
查询量；
不同策略命中情况；
手机号形态查询量；
结果数量分布。
```

### 27.2 Store 层过滤指标

Store 记录：

```text
matched candidates
visible after scope
```

用于观察：

```text
索引召回数量；
权限过滤后剩余数量；
是否存在大量无权限召回；
InternalLimit 是否过小。
```

---

## 28. 为什么返回空数组优先于报错

Suggest 是辅助查询能力。

以下场景优先返回空数组：

```text
scopeProvider 未配置；
index 未初始化；
keyword 为空；
手机号形态无权限；
scope 过滤后无可见候选；
模块降级。
```

原因：

1. 不暴露内部状态。
2. 不影响主业务流程。
3. 避免安全侧信道。
4. Suggest 不是核心写操作。
5. 对前端 autocomplete 来说，空数组是自然结果。

但认证失败、参数错误、限流仍然应该返回明确错误。

---

## 29. 查询链路中的关键不变量

1. Handler 必须先提取 OperatingPrincipal。
2. Handler 必须在进入 Service 前执行 RateLimiter。
3. Service 必须先解析 ProfileAccessScope，再查询索引。
4. 手机号形态关键词必须检查 AllowMobileSearch。
5. Store 必须先 match，再 scope filter，再 rank。
6. RankingPolicy 必须在过滤后执行。
7. DTO 必须返回 mobile_mask。
8. 生产环境不得返回明文手机号。
9. 无索引或无 scope 时返回空数组，而不是全局结果。
10. Suggest 不得绕过 ProfileAccessScope 直接返回 Store 全局结果。

---

## 30. 常见错误设计

### 30.1 在 Handler 层过滤结果

错误原因：

```text
Handler 不应该理解索引结构；
Handler 过滤太晚；
limit 之后过滤会导致召回不足。
```

### 30.2 先 limit 再 scope filter

错误原因：

```text
无权限数据会占据前 N 个候选；
当前用户可见结果可能被截断掉。
```

### 30.3 手机号无权限返回 403

不一定适合。

对于 autocomplete，返回空数组更能减少枚举侧信道。

接口级权限不足仍然返回 403。

### 30.4 DTO 返回 mobile

错误。

生产环境只能返回：

```text
mobile_mask
```

### 30.5 Suggest 直接调用 Casbin 做逐条授权

错误方向。

Suggest 应消费 `ProfileAccessScope`，不应对每个候选调用 AuthZ。

否则会破坏性能和模块边界。

---

## 31. 代码事实源

| 主题 | 文件 |
| ---- | ---- |
| REST Handler | `internal/apiserver/transport/rest/suggest/handler.go` |
| Principal 提取 | `internal/apiserver/transport/rest/suggest/principal.go` |
| Service 用例 | `internal/apiserver/application/suggest/service.go` |
| 应用 DTO | `internal/apiserver/application/suggest/dto.go` |
| 应用端口 | `internal/apiserver/application/suggest/ports.go` |
| 搜索策略 | `internal/apiserver/application/suggest/search_strategy.go` |
| Query / Keyword | `internal/apiserver/domain/suggest/profile.go` |
| ScopePolicy | `internal/apiserver/domain/suggest/scope.go` |
| RankingPolicy | `internal/apiserver/domain/suggest/ranking.go` |
| 手机号脱敏 | `internal/apiserver/domain/suggest/mobile.go` |
| Store 查询 | `internal/apiserver/infra/suggest/search/store.go` |
| Hash 索引 | `internal/apiserver/infra/suggest/search/hash.go` |
| Trie 索引 | `internal/apiserver/infra/suggest/search/trie.go` |
| REST DTO | `internal/apiserver/transport/rest/suggest/dto.go` |

---

## 32. Verify

建议执行：

```bash
go test ./internal/apiserver/application/suggest/...
go test ./internal/apiserver/domain/suggest/...
go test ./internal/apiserver/infra/suggest/search/...
go test ./internal/apiserver/transport/rest/suggest/...
```

建议重点覆盖：

```text
1. 无 principal 返回 401；
2. 手机号形态无权限返回空；
3. 手机号形态有权限但 scope 不可见返回空；
4. Trie 召回后按 OrgID 过滤；
5. Hash 召回后按 ProfileIDs 过滤；
6. limit 大于 MaxResults 时被截断；
7. index nil 返回空；
8. mobile_mask 不返回明文手机号。
```

---

## 33. 下一篇

下一篇建议继续阅读：

[02-权限范围-OperatingPrincipal与ProfileAccessScope.md](./02-权限范围-OperatingPrincipal与ProfileAccessScope.md)

它会重点分析：

```text
OperatingPrincipal 如何表示 operating 用户；
ProfileAccessScope 如何表达可见范围；
OrgIDs / OperatorID / ProfileIDs 如何协作；
为什么 TenantIDs 当前不作为数据权限主路径；
手机号搜索权限如何进入 scope。
```
