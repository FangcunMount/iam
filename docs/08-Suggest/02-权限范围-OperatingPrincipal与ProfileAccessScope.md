# 02-权限范围：OperatingPrincipal 与 ProfileAccessScope

> 本文回答：Suggest 查询为什么必须携带 `OperatingPrincipal`，`ProfileAccessScope` 如何表达当前操作员可见的 Profile 范围，`OrgIDs / OperatorID / ProfileIDs / AllowMobileSearch` 分别解决什么问题，为什么当前不应把 `TenantIDs` 作为数据权限主路径，以及 Suggest 为什么只消费 scope 而不直接承载完整 AuthZ。

---

## 30 秒结论

| 问题 | 结论 |
| ---- | ---- |
| OperatingPrincipal 是什么？ | 当前 operating 后台调用者的最小身份视图，回答“谁在查” |
| ProfileAccessScope 是什么？ | 当前调用者可见 Profile 范围的解析结果，回答“能看哪些 Profile” |
| Suggest 自己算权限吗？ | 不算完整权限；它只消费 `ProfileAccessScope` 并在索引层做过滤 |
| 当前数据权限主维度是什么？ | `AllProfile / ProfileIDs / OrgIDs / OperatorID` |
| 手机号搜索权限在哪里表达？ | `AllowMobileSearch` |
| TenantDomain 是什么？ | IAM 授权域，例如 `fangcun` / `platform`，不进入 Suggest 索引 |
| TenantIDs 现在能用吗？ | 当前不作为数据权限主路径，仅为未来 SaaS 隔离预留，已标记 Deprecated |
| 为什么不逐条调用 AuthZ？ | 高频 autocomplete 场景下逐条鉴权成本高、边界重、容易把 Suggest 做成权限中心 |
| ScopeProvider 放在哪里？ | application 定义端口，infra/suggest/access 做适配实现 |

核心关系：

```text
OperatingPrincipal
  ↓
ProfileAccessScopeProvider
  ↓
ProfileAccessScope
  ↓
SearchIndex match
  ↓
ScopePolicy filter
  ↓
visible ProfileSearchTerm
```

一句话：

> **Suggest 不判断“这个角色理论上拥有什么权限”，它只判断“这个 ProfileSearchTerm 是否落在当前请求已经解析出的可见范围内”。**

---

## 1. 为什么 Suggest 需要权限范围

Suggest 不是普通搜索。

如果只按 keyword 查询全局索引：

```text
keyword = zhang
  ↓
全局 Profile 索引
  ↓
返回所有叫 zhang 的 Profile
```

就会出现水平越权：

```text
操作员 A 看到不属于自己组织 / 自己负责范围的 Profile。
```

所以 Suggest 查询必须携带两个信息：

```text
1. 搜索词 keyword：想查什么；
2. 可见范围 scope：当前调用者能看哪些 Profile。
```

没有 scope 的 suggest，本质是全局数据泄露入口。

---

## 2. 权限边界：接口权限与数据权限

Suggest 有两层权限。

### 2.1 接口级权限

接口级权限回答：

```text
当前用户能不能调用 /api/v2/suggest/profile？
```

它通常由 route middleware 完成，例如：

```text
ResourceIAMProfileCollection + ActionSearch
```

如果没有接口权限，应该返回 403。

---

### 2.2 数据级权限

数据级权限回答：

```text
当前用户调用 suggest 后，能看到哪些 Profile？
```

它由 `ProfileAccessScope` 表达，并在 Store 查询中执行过滤。

接口级权限和数据级权限不能互相替代。

```text
有 search 接口权限
  ≠
能看全量 Profile
```

---

## 3. OperatingPrincipal

`OperatingPrincipal` 是 Suggest 查询使用的当前调用者视图。

它不是完整 User 聚合，不是 AuthN account，也不是 AuthZ policy。

它只是 Suggest 需要的最小身份输入。

典型字段：

```text
OperatorID
TenantDomain
OrgIDs
RoleCodes
IsSuperAdmin
```

---

## 4. OperatingPrincipal 字段语义

| 字段 | 说明 |
| ---- | ---- |
| OperatorID | 当前后台操作员 ID，用于限流、owner 过滤、审计日志 |
| TenantDomain | IAM 授权域，例如 `fangcun` / `platform` |
| OrgIDs | 当前上下文中可得的业务组织 ID 集合 |
| RoleCodes | 当前操作员角色编码，用于 scope provider 辅助判断 |
| IsSuperAdmin | 是否超级管理员 |

其中最容易混淆的是：

```text
TenantDomain 不是 OrgID。
```

当前约定：

```text
TenantDomain = IAM authorization domain
OrgID        = business organization scope
```

Suggest 索引不使用 `TenantDomain` 做数据权限过滤。

---

## 5. OperatingPrincipal 从哪里来

REST Handler 会从 Gin context 中提取 operating principal。

上下文来源通常包括：

```text
认证中间件解析 token；
请求上下文中的 user_id；
tenant domain；
业务 org_id；
角色 / 权限信息；
route authorization 运行时。
```

然后转换成：

```text
OperatingPrincipal
```

Handler 只负责提取和传递，不负责判断它能看哪些 Profile。

---

## 6. 为什么不直接把 Principal 传给 Store

错误设计：

```text
Store.Suggest(query, principal)
```

这样 Store 就必须知道：

```text
角色怎么判断；
超级管理员怎么判断；
组织范围怎么展开；
手机号权限怎么判断；
ProfileIDs 怎么预计算；
AuthZ runtime 怎么调用。
```

这会让 infra/search 膨胀成权限模块。

正确设计：

```text
OperatingPrincipal
  ↓
ProfileAccessScopeProvider
  ↓
ProfileAccessScope
  ↓
Store.Suggest(query, scope)
```

Store 只消费 scope，不理解完整 principal 权限逻辑。

---

## 7. ProfileAccessScope

`ProfileAccessScope` 是权限侧解析后的 Profile 可见范围。

它不是权限规则本身，而是权限规则执行后的结果。

典型结构：

```text
AllProfile
OrgIDs
OperatorID
ProfileIDs
AllowMobileSearch
```

可以理解为：

```text
当前操作员可以通过哪些维度看到 Profile。
```

---

## 8. ProfileAccessScope 字段语义

| 字段 | 说明 |
| ---- | ---- |
| AllProfile | 可以查看所有 Profile，通常是超级管理员或平台级能力 |
| ProfileIDs | 精确可见 Profile 集合，适合复杂权限预计算 |
| OrgIDs | 业务组织范围，适合机构 / 组织级管理权限 |
| OperatorID | 当前操作员 ID，适合“我负责的 Profile”场景 |
| AllowMobileSearch | 是否允许手机号形态关键词搜索 |
| TenantIDs | Deprecated，当前不作为数据权限主路径 |

---

## 9. AllProfile

`AllProfile=true` 表示当前调用者可见全部 Profile。

典型场景：

```text
平台超级管理员；
全局运营角色；
特殊排障角色。
```

使用风险：

```text
AllProfile 是最高级数据范围；
不能因为普通用户拥有 tenant domain 就授予 AllProfile；
不能因为拥有接口级 search 权限就授予 AllProfile。
```

建议：

```text
AllProfile 只能由明确的高级权限 / 超级管理员角色授予。
```

---

## 10. ProfileIDs

`ProfileIDs` 是最精确的可见范围。

它适合表达复杂权限：

```text
某个操作员被分配了若干 Profile；
某个医生负责若干 Profile；
某个运营临时授权查看几个 Profile；
某个组织树权限展开后得到一批 Profile；
某个业务关系无法用单一 OrgID 表达。
```

优点：

```text
精确；
不需要 Store 理解业务关系；
适合缓存和版本控制；
避免把复杂权限逻辑塞进 Suggest。
```

缺点：

```text
ProfileIDs 很大时需要缓存；
需要考虑权限变更后的失效；
需要 ProfileVisibilityIDsResolver 支撑。
```

---

## 11. OrgIDs

`OrgIDs` 表达业务组织范围。

适合场景：

```text
机构管理员查看本机构 Profile；
区域管理员查看多个机构 Profile；
组织树展开后查看子机构 Profile。
```

Suggest 中的 ProfileSearchTerm 会携带：

```text
OrgID
```

ScopePolicy 会判断：

```text
term.OrgID ∈ scope.OrgIDs
```

注意：

```text
OrgID 是业务组织 ID；
TenantDomain 是 IAM 授权域；
二者不能混用。
```

---

## 12. OperatorID

`OperatorID` 用于表达“当前操作员负责的 Profile”。

ProfileSearchTerm 中有：

```text
OwnerOperatorIDs
```

ScopePolicy 会判断：

```text
scope.OperatorID ∈ term.OwnerOperatorIDs
```

典型场景：

```text
医生只能看自己负责的儿童档案；
咨询师只能看分配给自己的 Profile；
普通运营只能看自己创建 / 负责的 Profile。
```

当前默认 Loader 的过渡实现中，`owner_operator_ids` 暂由 `profiles.created_by` 生成。

这只是过渡方案。

长期应使用真实的 Profile visibility read model。

---

## 13. AllowMobileSearch

`AllowMobileSearch` 专门控制手机号形态关键词搜索。

原因是手机号搜索比姓名搜索更敏感。

手机号搜索可能带来：

```text
手机号枚举；
用户存在性探测；
敏感日志泄露；
跨组织数据探测。
```

因此规则是：

```text
keyword 看起来像手机号
  ↓
AllowMobileSearch=false
  ↓
直接返回空数组
```

如果 `AllowMobileSearch=true`，也不是直接返回结果。

仍然必须经过：

```text
ProfileAccessScope filter
```

也就是说：

```text
有手机号搜索权限
  ≠
能搜索到所有手机号对应 Profile
```

---

## 14. TenantIDs 为什么 Deprecated

历史上系统中曾出现：

```text
tenant_id 被当作 org_id 使用；
DefaultTenantID 是 uint64；
业务组织范围和 IAM 授权域混用。
```

当前已经明确区分：

```text
TenantDomain = IAM 授权域，例如 fangcun / platform；
OrgID        = 业务组织范围，例如 1 / 2 / 3。
```

所以 Suggest 当前不应该使用 `TenantIDs` 作为数据权限主路径。

`TenantIDs` 只保留为未来 SaaS 多租户隔离预留。

当前数据权限判断应优先使用：

```text
AllProfile
ProfileIDs
OrgIDs
OperatorID
```

---

## 15. ProfileAccessScopeProvider

`ProfileAccessScopeProvider` 是 application 层定义的端口。

它负责：

```text
OperatingPrincipal -> ProfileAccessScope
```

接口语义：

```go
type ProfileAccessScopeProvider interface {
    ResolveProfileAccessScope(ctx context.Context, principal OperatingPrincipal) (ProfileAccessScope, error)
}
```

它不是领域服务，而是应用层端口。

具体实现放在 infra/suggest/access 中，负责适配 IAM AuthZ / route authorization / visibility resolver。

---

## 16. ScopeProvider 为什么不放在 domain

因为 ScopeProvider 可能依赖：

```text
RouteAuthorizationRuntime；
角色判断；
ProfileVisibilityIDsResolver；
缓存；
外部权限事实；
未来的组织树 / 分配关系。
```

这些都不是纯领域规则。

如果放进 domain，会导致 domain 反向依赖基础设施或 AuthZ runtime。

所以当前分层是：

```text
application/suggest/ports.go
  定义 ProfileAccessScopeProvider 端口

infra/suggest/access
  实现 OperatingProfileAccessScopeProvider
```

这符合 ports-and-adapters 思路。

---

## 17. OperatingProfileAccessScopeProvider

infra 实现负责把 operating principal 转为 ProfileAccessScope。

它可能做这些事：

```text
判断是否超级管理员；
判断是否具备 search_by_mobile；
解析角色或 route authorization；
调用 ProfileVisibilityIDsResolver；
填充 ProfileIDs / OrgIDs / OperatorID；
缓存可见 ProfileID 集合。
```

但它的输出必须收敛为：

```text
ProfileAccessScope
```

不要把这些复杂逻辑泄露给 Store。

---

## 18. ProfileVisibilityIDsResolver

`ProfileVisibilityIDsResolver` 用于处理复杂可见性。

典型接口语义：

```text
根据 OperatingPrincipal 解析可见 ProfileID 集合。
```

适用场景：

```text
组织树展开后得到 ProfileIDs；
业务分配表得到 ProfileIDs；
医生/咨询师负责关系得到 ProfileIDs；
临时授权得到 ProfileIDs；
多条件合并后的精确可见集合。
```

好处：

```text
Store 不需要理解组织树；
Store 不需要理解医生/咨询师关系；
Store 不需要调用 AuthZ；
Store 只做 profileID set 判断。
```

---

## 19. ScopePolicy

`ScopePolicy` 是 domain 层的轻量判断策略。

它只判断：

```text
某个 ProfileSearchTerm 是否落在当前 ProfileAccessScope 内。
```

典型判断顺序：

```text
1. AllProfile
2. ProfileIDs contains term.ProfileID
3. OrgIDs contains term.OrgID
4. OperatorID in term.OwnerOperatorIDs
5. TenantIDs contains term.TenantID（Deprecated）
```

这个判断是纯内存判断，不查 DB，不调 AuthZ。

---

## 20. CompiledProfileAccessScope

为了避免每次判断都线性扫描 slice，scope 会被编译为 set。

例如：

```text
ProfileIDs -> map[int64]struct{}
OrgIDs     -> map[int64]struct{}
TenantIDs  -> map[int64]struct{}
```

然后判断：

```text
O(1) contains
```

这对 Suggest 很重要。

原因：

```text
一次查询可能召回大量 candidate；
每个 candidate 都要做可见性判断；
如果 ProfileIDs 很大，线性 contains 会明显增加延迟。
```

---

## 21. Store 如何使用 Scope

Store 查询时会：

```text
1. 通过 Trie / Hash 召回 matchedIDs；
2. 编译 ProfileAccessScope；
3. 遍历 matchedIDs；
4. 取 ProfileSearchTerm；
5. 用 ScopePolicy 过滤；
6. 得到 visible terms；
7. 排序并截断。
```

伪代码：

```go
matchedIDs := match(query)
compiled := CompileProfileAccessScope(scope)
visible := make([]ProfileSearchTerm, 0)

for _, id := range matchedIDs {
    term, ok := terms[id]
    if !ok {
        continue
    }
    if ScopePolicy{}.AllowsCompiled(compiled, term) {
        visible = append(visible, term)
    }
}
```

重点：

```text
无权限 term 不进入 RankingPolicy。
```

---

## 22. 为什么不在 SQL 层直接过滤

有一个直觉设计是：

```text
每次 suggest 查询都直接带权限条件查 MySQL。
```

这当然可以保证权限，但会失去 Suggest 的主要价值。

原因：

1. Suggest 是高频 autocomplete。
2. 拼音 / 简拼匹配不适合每次动态 SQL。
3. 每次查询都 join 权限关系会增加数据库压力。
4. 查询体验更依赖低延迟。
5. MySQL 查询结果仍然要考虑排序、脱敏、限流。

当前设计是：

```text
刷新链路构建读模型；
查询链路在进程内索引中过滤。
```

这是一种读模型换查询性能的取舍。

---

## 23. 为什么不在返回前过滤

另一个直觉设计是：

```text
先查全局索引，返回前再过滤。
```

问题是：

```text
如果先 rank + limit，再过滤，召回会不足。
```

比如：

```text
全局前 20 个结果都无权限；
当前用户可见结果在第 50 个；
limit 后过滤会返回空。
```

所以权限过滤必须发生在：

```text
rank + limit 之前。
```

当前 Store 的设计就是为了保证这个顺序。

---

## 24. 为什么不逐条调用 AuthZ

错误设计：

```text
for each term:
    authz.Check(user, term.ProfileID, action)
```

问题：

1. 高频查询性能差。
2. N 个候选导致 N 次权限检查。
3. Suggest 变成 AuthZ 调用器。
4. 权限规则和搜索索引耦合。
5. 容易出现循环依赖。
6. 难以缓存和观测。

正确设计：

```text
权限侧一次性解析 ProfileAccessScope；
Suggest 只在内存中做范围判断。
```

---

## 25. 推荐 Scope 映射策略

当前推荐：

| 角色 / 场景 | Scope |
| ----------- | ----- |
| 超级管理员 | `AllProfile=true` |
| 机构管理员 | `OrgIDs=[managed org ids]` |
| 普通操作员 | `OperatorID=current operator` 或 `ProfileIDs=[assigned profile ids]` |
| 医生 / 咨询师 | `ProfileIDs=[responsible profile ids]` 或 `OperatorID` |
| 临时授权 | `ProfileIDs=[explicit profile ids]` |
| 手机号搜索权限 | `AllowMobileSearch=true` |
| 无明确数据权限 | 空 scope，返回空结果 |

尤其注意：

```text
普通用户不要默认获得 TenantIDs；
普通用户不要默认获得 AllProfile；
接口 search 权限不等于全局数据权限。
```

---

## 26. 空 Scope 的语义

如果 ScopeProvider 返回空范围：

```text
AllProfile=false
ProfileIDs=[]
OrgIDs=[]
OperatorID=0
```

那么查询结果应该是：

```text
[]
```

而不是错误。

原因：

```text
用户可能有接口访问权限，但没有任何可见 Profile；
对 autocomplete 来说，空结果是自然语义；
返回空结果比暴露权限细节更安全。
```

---

## 27. 手机号权限与 Scope 的关系

手机号权限分两层：

```text
1. 是否允许使用手机号形态关键词搜索：AllowMobileSearch；
2. 命中的 Profile 是否在可见范围：ProfileAccessScope filter。
```

所以：

| AllowMobileSearch | Profile 可见 | 返回 |
| ----------------- | ------------ | ---- |
| false | 不判断 | `[]` |
| true | false | `[]` |
| true | true | 脱敏结果 |

这避免了：

```text
有手机号权限的人搜索到所有 Profile；
无手机号权限的人通过手机号探测存在性。
```

---

## 28. 与 IAM AuthZ 的关系

Suggest 与 IAM AuthZ 的关系是：

```text
IAM AuthZ / route middleware：控制能否调用接口；
ProfileAccessScopeProvider：消费授权上下文，产出 Profile 可见范围；
Suggest Store：用 scope 过滤索引结果。
```

也就是说：

```text
AuthZ 决定“你有没有 search 能力”；
ScopeProvider 决定“search 时你能看到哪些 Profile”；
Store 执行“只返回这些 Profile”。
```

这三层不能混为一层。

---

## 29. 与 tenant/org 重构的关系

当前系统已经明确：

```text
TenantDomain = IAM 授权域，例如 fangcun / platform；
OrgID        = 业务组织范围，例如 1 / 2 / 3。
```

Suggest 中：

```text
OperatingPrincipal.TenantDomain
  用于标识 IAM 授权域、日志、权限 provider 判断。

ProfileSearchTerm.OrgID
  用于业务组织范围过滤。
```

不应该再出现：

```text
tenant_id 被当成 org_id；
DefaultTenantID=1；
TenantIDs 作为普通数据权限主路径。
```

---

## 30. 常见错误设计

### 30.1 用 TenantDomain 过滤 Profile

错误。

TenantDomain 是 string 授权域，ProfileSearchTerm 使用 OrgID 表示业务组织范围。

---

### 30.2 普通用户默认获得 TenantIDs

错误。

这会导致租户内全可见风险。

---

### 30.3 有 search 权限就 AllProfile

错误。

接口权限和数据范围权限不同。

---

### 30.4 手机号权限绕过 scope

错误。

`AllowMobileSearch=true` 只表示可以使用手机号形态关键词，不表示可以看所有手机号命中的 Profile。

---

### 30.5 Store 直接调用 AuthZ

错误。

Store 应该只消费 scope，不理解 AuthZ runtime。

---

## 31. 测试建议

建议至少覆盖：

```text
1. AllProfile=true 能看到全部 matched terms；
2. ProfileIDs 只放行指定 Profile；
3. OrgIDs 只放行指定 Org 的 Profile；
4. OperatorID 只放行 OwnerOperatorIDs 中包含自己的 Profile；
5. 空 scope 返回空；
6. AllowMobileSearch=false 时手机号形态关键词返回空；
7. AllowMobileSearch=true 但 scope 不可见时仍返回空；
8. TenantIDs 不应成为普通用户默认路径；
9. Scope 编译后 contains 行为正确；
10. Store 过滤发生在 RankingPolicy 之前。
```

---

## 32. 代码事实源

| 主题 | 文件 |
| ---- | ---- |
| OperatingPrincipal | `internal/apiserver/domain/suggest/principal.go` |
| ProfileAccessScope / ScopePolicy | `internal/apiserver/domain/suggest/scope.go` |
| ProfileSearchTerm | `internal/apiserver/domain/suggest/profile.go` |
| Service 编排 | `internal/apiserver/application/suggest/service.go` |
| ScopeProvider 端口 | `internal/apiserver/application/suggest/ports.go` |
| ScopeProvider 实现 | `internal/apiserver/infra/suggest/access/` |
| Visibility Resolver | `internal/apiserver/infra/mysql/suggest/profile_visibility_resolver.go` |
| Store 过滤 | `internal/apiserver/infra/suggest/search/store.go` |
| REST Principal 提取 | `internal/apiserver/transport/rest/suggest/principal.go` |

---

## 33. Verify

建议执行：

```bash
go test ./internal/apiserver/domain/suggest/...
go test ./internal/apiserver/application/suggest/...
go test ./internal/apiserver/infra/suggest/access/...
go test ./internal/apiserver/infra/mysql/suggest/...
go test ./internal/apiserver/infra/suggest/search/...
```

建议 grep：

```bash
grep -R "DefaultTenantID" docs internal pkg || true
grep -R "tenant_id.*org_id" docs internal pkg || true
grep -R "TenantIDs" internal/apiserver/domain/suggest internal/apiserver/infra/suggest || true
```

`TenantIDs` 如果出现，应确认是否带有 Deprecated / reserved 语义。

---

## 34. 下一篇

下一篇建议阅读：

[03-索引模型-ProfileSearchTerm-Trie-Hash-Runtime.md](./03-索引模型-ProfileSearchTerm-Trie-Hash-Runtime.md)

它会继续分析：

```text
ProfileSearchTerm 如何写入 Trie / Hash；
Trie 如何支持中文、拼音、简拼；
Hash 如何支持 ProfileID / 手机号精确匹配；
Runtime 如何原子替换当前索引；
profileKeys 如何支持增量更新时撤销旧 key。
```
