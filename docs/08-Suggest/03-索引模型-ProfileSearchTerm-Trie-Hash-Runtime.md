# 03-索引模型：ProfileSearchTerm / Trie / Hash / Runtime

> 本文回答：Suggest 模块的进程内索引到底如何组织；`ProfileSearchTerm` 如何被写入 Trie / Hash；为什么 Trie 使用三叉搜索树；Hash 如何支持 ProfileID / 手机号精确匹配；`Store` 如何通过 `terms` 和 `profileKeys` 维护索引一致性；`Runtime` 如何原子替换当前索引；增量更新如何避免旧姓名、旧拼音、旧手机号残留。

---

## 30 秒结论

| 问题 | 结论 |
| ---- | ---- |
| 索引的最小数据单元是什么？ | `ProfileSearchTerm`，它是 Profile 面向 Suggest 查询的读模型投影 |
| Store 由什么组成？ | `Trie + Hash + terms + profileKeys + RWMutex` |
| Trie 解决什么？ | 中文名、拼音、简拼的前缀搜索 |
| Hash 解决什么？ | ProfileID、手机号的精确匹配 |
| terms 解决什么？ | `profileID -> ProfileSearchTerm`，统一保存完整索引项 |
| profileKeys 解决什么？ | 记录每个 profile 写入过哪些 Trie / Hash key，用于增量更新时撤销旧 key |
| Runtime 解决什么？ | 持有当前活动 Store，支持全量原子替换与增量导入 |
| 为什么 Trie 不直接保存完整 term？ | 同一个 Profile 会写入多个 key，保存完整 term 会重复占内存，更新时也难以统一替换 |
| 为什么需要可撤销旧 key？ | 姓名、拼音、手机号变化后，旧 key 不能继续搜到该 Profile |
| 查询顺序是什么？ | `match profileIDs -> load terms -> scope filter -> rank` |

一句话：

> **Suggest 索引不是一个单一 Trie，而是由文本前缀索引、数字精确索引、Profile term 存储、Profile key 反向记录、Runtime 原子切换共同组成的读模型运行时。**

---

## 1. 索引模型总览

Suggest 的索引结构可以抽象为：

```text
ProfileSearchTerm[]
  ↓ Load / ImportTerms
Store
  ├── Trie       文本前缀索引：中文名 / 拼音 / 简拼 -> profileIDs
  ├── Hash       精确索引：ProfileID / 手机号 -> profileIDs
  ├── terms      profileID -> ProfileSearchTerm
  ├── profileKeys profileID -> indexed keys
  └── mu         并发读写保护
```

查询时：

```text
keyword
  ↓
Trie 或 Hash 召回 profileIDs
  ↓
terms[profileID] 取完整 ProfileSearchTerm
  ↓
ProfileAccessScope 过滤
  ↓
RankingPolicy 排序截断
```

刷新时：

```text
MySQL Loader
  ↓
[]ProfileSearchTerm
  ↓
Runtime.Replace / Runtime.ImportDelta
  ↓
Store.Load / Store.ImportTerms
```

---

## 2. 为什么需要组合索引

Suggest 要支持不同类型的输入：

```text
张三        中文名
zhangsan   拼音
zs          简拼
10001       ProfileID
13800138000 手机号
```

这些输入的匹配方式不同。

| 输入类型 | 匹配方式 | 索引结构 |
| -------- | -------- | -------- |
| 中文名 | 前缀匹配 | Trie |
| 拼音 | 前缀匹配 | Trie |
| 简拼 | 前缀匹配 | Trie |
| ProfileID | 精确匹配 | Hash |
| 手机号 | 精确匹配 | Hash |

所以一个索引结构无法优雅覆盖所有场景。

当前设计把职责拆开：

```text
Trie 负责文本前缀；
Hash 负责数字精确；
terms 负责完整数据；
profileKeys 负责更新一致性。
```

---

## 3. ProfileSearchTerm：索引项读模型

`ProfileSearchTerm` 是 Suggest 索引的基本数据单元。

它不是 Profile 聚合本身，而是一个面向搜索和权限过滤的读模型投影。

典型结构：

```text
ProfileID
DisplayName
Mobiles
Weight
OrgID
OwnerOperatorIDs
```

字段语义：

| 字段 | 说明 |
| ---- | ---- |
| ProfileID | Profile 唯一 ID，也是索引主键 |
| DisplayName | 展示名，用于中文名 / 拼音 / 简拼索引 |
| Mobiles | 手机号列表，用于 Hash 精确匹配，返回时必须脱敏 |
| Weight | 排序权重 |
| OrgID | 业务组织范围，用于 scope filter |
| OwnerOperatorIDs | 负责该 Profile 的操作员 ID 集合，用于 scope filter |

当前还保留 `TenantID` 字段，但它已经不是数据权限主路径。

当前语义是：

```text
TenantDomain：IAM 授权域，不进入 Suggest 索引；
OrgID：业务组织范围，进入 Suggest 索引；
TenantID：Deprecated，仅为未来 SaaS 隔离预留。
```

---

## 4. ProfileSearchTerm 的不变量

一个可索引的 term 至少应该满足：

```text
ProfileID > 0
DisplayName 非空，或作为 tombstone 删除信号
Mobiles 中不应包含空字符串
OwnerOperatorIDs 应去重
Weight 有默认值
OrgID 表达业务组织范围
```

其中最特殊的是 `DisplayName`。

当前 Store 支持通过空 `DisplayName` 表示删除：

```text
ProfileSearchTerm{
  ProfileID: 1001,
  DisplayName: "",
}
```

语义是：

```text
删除 profileID=1001 的 terms、Trie keys、Hash keys。
```

这为 Delta refresh 的 tombstone 协议提供了基础。

---

## 5. Store 总体结构

Store 是真正执行查询和维护索引的对象。

它可以抽象为：

```go
type Store struct {
    trie        *Trie
    hash        *Hash
    terms       map[int64]ProfileSearchTerm
    profileKeys map[int64]profileKeySet
    mu          sync.RWMutex
}
```

职责：

| 成员 | 职责 |
| ---- | ---- |
| trie | 文本前缀索引 |
| hash | 数字精确索引 |
| terms | 保存完整 ProfileSearchTerm |
| profileKeys | 记录每个 profileID 写入过的 keys |
| mu | 并发读写保护 |

Store 是 infra/search 的核心聚合对象。

---

## 6. terms：完整 term 存储

`terms` 是：

```text
profileID -> ProfileSearchTerm
```

为什么需要它？

因为 Trie 和 Hash 只负责召回 `profileID`，不保存完整数据。

查询时：

```text
matched profileIDs
  ↓
terms[profileID]
  ↓
ProfileSearchTerm
```

这样有几个好处：

1. 避免在多个 key 下重复保存完整 term。
2. 同一个 Profile 多个 key 命中时只需要统一取一次 term。
3. 更新 Profile 时可以统一覆盖完整 term。
4. ScopePolicy 能拿到最新的 OrgID / OwnerOperatorIDs。
5. RankingPolicy 能使用最新的 Weight / DisplayName。

---

## 7. profileKeys：反向 key 记录

`profileKeys` 是：

```text
profileID -> profileKeySet
```

它记录某个 Profile 写入过哪些 Trie key 和 Hash key。

例如：

```text
ProfileID = 1001
DisplayName = 张三
Mobiles = [13800138000]
```

可能写入：

```text
Trie keys:
  张三
  zhangsan
  zs

Hash keys:
  1001
  13800138000
```

`profileKeys[1001]` 会记录这些 key。

---

## 8. 为什么 profileKeys 很重要

没有 `profileKeys`，增量更新会出现旧 key 残留。

例如 Profile 修改：

```text
旧数据：张三 / zhangsan / zs / 13800138000
新数据：李四 / lisi / ls / 13900139000
```

如果只追加新 key：

```text
李四
lisi
ls
13900139000
```

旧 key 仍然存在：

```text
张三
zhangsan
zs
13800138000
```

那么用户搜索旧姓名或旧手机号仍然能命中该 Profile。

这会造成：

```text
搜索结果不准确；
旧手机号泄露风险；
权限关系变化后旧索引污染；
Delta refresh 不可信。
```

`profileKeys` 的作用就是在更新前撤销旧 key。

---

## 9. ImportTerms 的核心流程

`ImportTerms` 负责把一组 ProfileSearchTerm 导入当前 Store。

核心流程：

```text
for term in terms:
    if term.ProfileID <= 0:
        continue

    removeOldKeys(term.ProfileID)

    if term.DisplayName == "":
        delete store.terms[term.ProfileID]
        delete store.profileKeys[term.ProfileID]
        continue

    keys := buildKeys(term)
    insert trie keys
    insert hash keys
    store.terms[term.ProfileID] = term
    store.profileKeys[term.ProfileID] = keys
```

这个流程同时支持：

```text
新增 Profile；
更新 Profile；
删除 Profile；
姓名变化；
手机号变化；
权限维度变化；
权重变化。
```

---

## 10. 删除语义：tombstone term

如果 Delta 数据源需要删除某个 Profile，可以返回 tombstone term：

```text
ProfileID = 1001
DisplayName = ""
```

Store 会执行：

```text
removeOldKeys(1001)
delete terms[1001]
delete profileKeys[1001]
```

这比“SQL 过滤 deleted_at 后不返回这条记录”更可靠。

因为如果 Delta SQL 不返回删除记录，Store 无法知道要删除旧索引。

所以 Delta 协议必须明确：

```text
删除 / 解绑 / 清空手机号 / 权限关系变化，都要能让 Store 收到可更新或可删除的 term。
```

---

## 11. Trie：文本前缀索引

Trie 负责：

```text
中文名
拼音
简拼
```

例如：

```text
张三 -> 张三
张三 -> zhangsan
张三 -> zs
```

查询时：

```text
张      -> 命中 张三
zhang  -> 命中 zhangsan
zs     -> 命中 zs
```

Trie 返回的是：

```text
[]profileID
```

而不是完整 term。

---

## 12. 当前 Trie 是三叉搜索树

当前 Trie 使用三叉搜索树模型。

节点结构可以抽象为：

```go
type node struct {
    r     rune
    small *node
    equal *node
    large *node
    end   bool
    value []int64
}
```

三条边的含义：

| 边 | 含义 |
| -- | ---- |
| small | 当前字符小于节点 rune |
| equal | 当前字符等于节点 rune，进入下一个字符 |
| large | 当前字符大于节点 rune |

三叉搜索树结合了：

```text
二叉搜索树的字符比较；
Trie 的前缀路径推进。
```

---

## 13. 三叉搜索树插入过程

插入 key：

```text
zhangsan -> 1001
```

会逐 rune 处理：

```text
z -> h -> a -> n -> g -> s -> a -> n
```

伪代码：

```text
insert(node, chars, index, profileID):
    ch = chars[index]

    if node == nil:
        node = newNode(ch)

    if ch < node.r:
        node.small = insert(node.small, chars, index, profileID)

    else if ch > node.r:
        node.large = insert(node.large, chars, index, profileID)

    else:
        if index == len(chars)-1:
            node.end = true
            node.value add profileID
        else:
            node.equal = insert(node.equal, chars, index+1, profileID)
```

关键点：

```text
small / large 不推进 index；
equal 才推进 index。
```

这正是三叉搜索树区别于普通 Trie 的地方。

---

## 14. 三叉搜索树查询过程

以前缀：

```text
zhang
```

为例。

流程：

```text
1. 找到 z-h-a-n-g 这条路径的末尾节点；
2. 收集该节点自身 value；
3. 递归收集 equal 子树下的所有 end 节点；
4. 得到所有以 zhang 开头的 key 对应 profileIDs。
```

例如可以匹配：

```text
zhang
zhangsan
zhangxiaoming
zhangxiaohua
```

最终返回：

```text
[]profileID
```

---

## 15. Trie 写入哪些文本 key

对于一个 `ProfileSearchTerm`：

```text
ProfileID = 1001
DisplayName = 张三
```

会生成多种文本 key。

| Key | 示例 | 说明 |
| --- | ---- | ---- |
| 原名 | 张三 | 中文名前缀 |
| 全拼 | zhangsan | 拼音前缀 |
| 简拼 | zs | 首字母前缀 |

这些 key 都会指向同一个 profileID。

所以：

```text
张
zhang
zs
```

都可能命中同一个 Profile。

---

## 16. 拼音与简拼的取舍

当前拼音生成是轻量策略。

设计目标不是做完整中文搜索引擎，而是满足常见姓名联想。

取舍包括：

```text
支持中文原名；
支持拼音；
支持简拼；
不过度展开多音字组合；
通过 WildcardKeyCap 控制前缀扩展规模。
```

这意味着它适合：

```text
后台 autocomplete；
小中规模 Profile 数据；
低延迟姓名搜索。
```

不适合替代：

```text
全文检索；
复杂分词；
模糊纠错；
拼音多音字完整搜索；
权重学习排序。
```

---

## 17. Trie 删除 profileID

当 Profile 更新或删除时，需要从旧 key 中移除 profileID。

Trie 提供类似能力：

```text
RemoveProfileID(key, profileID)
```

语义：

```text
找到 key 对应的 end 节点；
从 value 中移除 profileID；
如果 value 为空，可以保留结构节点，也可以做轻量清理。
```

当前重点是：

```text
旧 key 不再返回该 profileID。
```

即使树节点本身未完全物理删除，也不影响查询正确性。

---

## 18. Hash：精确索引

Hash 负责精确匹配：

```text
ProfileID
手机号
```

结构可以抽象为：

```go
type Hash struct {
    table map[string][]int64
}
```

写入：

```text
profileID -> profileID
mobile    -> profileID
```

例如：

```text
"1001"        -> [1001]
"13800138000" -> [1001]
```

Hash 返回的也是：

```text
[]profileID
```

---

## 19. Hash 为什么使用 string key

手机号不应该强制转成 int64。

原因：

```text
手机号本质是标识符，不是数学数字；
可能存在前导零；
可能存在不同国家/地区号码；
未来可能有区号或格式化字符；
转 int64 会丢失字符串语义。
```

所以 Hash 使用：

```text
map[string][]int64
```

而不是：

```text
map[int64][]int64
```

这是正确的建模选择。

---

## 20. Hash 不做手机号前缀搜索

当前 Hash 是精确索引。

所以：

```text
输入 13800138000 -> 可能命中
输入 138        -> 不作为手机号前缀搜索
```

这是安全上的保守设计。

手机号前缀搜索会显著增加枚举风险。

如果未来确实需要手机号前缀搜索，需要单独设计：

```text
更严格的权限；
更严格的限流；
审计日志；
结果数量控制；
可能需要后端人工审核场景。
```

当前不建议支持。

---

## 21. Hash 删除 profileID

Hash 也支持按 key 删除 profileID：

```text
RemoveProfileID(key, profileID)
```

流程：

```text
ids := table[key]
remove profileID from ids
if ids empty:
    delete table[key]
else:
    table[key] = ids
```

这用于：

```text
手机号变更；
ProfileID tombstone 删除；
旧 hash key 撤销。
```

---

## 22. buildKeys：生成索引 key

导入一个 term 时，Store 需要生成两类 key：

```text
Trie keys
Hash keys
```

### 22.1 Trie keys

来自：

```text
DisplayName 原文
DisplayName 拼音
DisplayName 简拼
```

### 22.2 Hash keys

来自：

```text
ProfileID string
Mobiles[]
```

然后：

```text
insert Trie keys -> trie
insert Hash keys -> hash
record keys -> profileKeys
```

---

## 23. 查询链路中的 Store

Store 查询时不关心来源是 REST 还是 gRPC，也不关心调用者角色。

它只接受：

```text
Query
ProfileAccessScope
```

然后执行：

```text
match -> scope filter -> rank
```

流程：

```text
if keyword.IsDigits():
    matchedIDs = hash.Match(keyword)
else:
    matchedIDs = trie.Match(keyword)

compiledScope = CompileProfileAccessScope(scope)
visible = filter(matchedIDs, compiledScope)
ranked = RankingPolicy.RankForQuery(query, visible, query.Limit)
```

---

## 24. 为什么 Store 需要 RWMutex

Store 会被并发访问：

```text
多个 HTTP 请求同时读索引；
后台 Delta refresh 可能导入新 terms；
Full refresh 可能替换 Runtime 当前 Store。
```

因此 Store 内部使用读写锁保护：

```text
查询：RLock
导入：Lock
```

注意：

```text
Full refresh 通常构建新 Store，再由 Runtime 原子替换；
Delta refresh 才会在当前 Store 上导入。
```

---

## 25. Runtime：当前活动索引

`ProfileSuggestionRuntime` 负责持有当前活动索引。

它提供：

```text
Current()
Replace(terms)
ImportDelta(terms)
```

抽象语义：

| 方法 | 说明 |
| ---- | ---- |
| Current | 返回当前活动 Store |
| Replace | 基于全量 terms 构建新 Store，并原子替换当前 Store |
| ImportDelta | 在当前 Store 上导入增量 terms |

Runtime 使用原子值持有当前 Store。

这避免了全量刷新期间阻塞查询。

---

## 26. Full Refresh 与 Runtime.Replace

Full refresh 流程：

```text
1. Loader.Full(ctx) 读取全量 ProfileSearchTerm；
2. search.Load(terms) 构建新 Store；
3. Runtime.Replace(newStore) 原子替换当前 Store；
4. 新请求开始读新 Store；
5. 老请求如果已经拿到旧 Store，继续完成。
```

优点：

```text
构建新索引时不影响旧索引查询；
替换瞬间很快；
查询链路不需要等待全量构建；
失败时可以继续使用旧索引。
```

这是一种典型的读模型热替换设计。

---

## 27. Delta Refresh 与 Runtime.ImportDelta

Delta refresh 流程：

```text
1. Loader.Delta(ctx, since) 读取变化 terms；
2. Runtime.ImportDelta(terms) 获取当前 Store；
3. Store.ImportTerms(terms)；
4. 对每个 term 撤销旧 key，再写入新 key。
```

Delta 适合：

```text
少量 Profile 更新；
姓名变更；
手机号变更；
owner/org/weight 变更；
tombstone 删除。
```

前提是：

```text
Delta 数据源必须能返回足够的信息，让 Store 知道该 upsert 还是 delete。
```

如果 Delta SQL 无法表达删除，仍然需要 Full refresh 兜底。

---

## 28. Snapshot 的位置

SnapshotWriter 是可选能力。

它可以把刷新后的候选写入文件，用于：

```text
排障；
兼容旧流程；
查看当前索引输入；
问题复现。
```

但 Snapshot 不是权威数据源。

当前权威数据仍然是：

```text
MySQL Loader / FullSQL / DeltaSQL
```

如果未来要用 Snapshot 做恢复，需要额外设计 SnapshotReader 和版本校验。

---

## 29. 索引一致性风险

Suggest 索引是读模型，因此天然存在一致性问题。

主要风险：

| 风险 | 说明 | 应对 |
| ---- | ---- | ---- |
| 旧姓名残留 | 姓名变更后旧 key 仍命中 | profileKeys + RemoveProfileID |
| 旧手机号残留 | 手机号变更后旧手机号仍命中 | Hash RemoveProfileID |
| 删除未同步 | Profile 删除后 Delta 没返回 tombstone | Delta tombstone / Full refresh 兜底 |
| 权限关系变更未同步 | owner/org 变化后旧 scope 仍可见 | Delta 更新 term / Full refresh 兜底 |
| 索引构建失败 | Full refresh 失败 | Runtime 保留旧 Store / DegradedService |

---

## 30. 性能取舍

### 30.1 Trie 查询

适合：

```text
中文名 / 拼音 / 简拼前缀查询
```

优势：

```text
避免遍历全部 key；
支持前缀展开；
对混合 rune key 友好。
```

限制：

```text
不是全文检索；
不是模糊搜索；
不是拼音多音字完整搜索。
```

### 30.2 Hash 查询

适合：

```text
ProfileID / 手机号精确匹配
```

优势：

```text
O(1) 查找；
简单直接。
```

限制：

```text
不支持手机号前缀；
不支持数字模糊查询。
```

### 30.3 terms + profileKeys

优势：

```text
减少重复存储；
支持统一更新；
支持旧 key 撤销。
```

代价：

```text
实现复杂度增加；
需要维护索引 key 反向记录。
```

---

## 31. 常见错误设计

### 31.1 Trie 保存完整 ProfileSearchTerm

问题：

```text
同一个 Profile 会写入多个 key；
完整 term 重复存储；
更新时容易不一致。
```

当前正确做法：

```text
Trie -> profileID
terms -> ProfileSearchTerm
```

---

### 31.2 Delta 只追加不删除旧 key

问题：

```text
旧姓名、旧拼音、旧手机号继续命中。
```

当前正确做法：

```text
profileKeys 记录旧 key；
更新前 removeOldKeys；
再写入新 keys。
```

---

### 31.3 手机号转 int64 做 Hash key

问题：

```text
手机号不是数学数字；
可能丢失前导零；
国际号码不兼容。
```

当前正确做法：

```text
Hash key 使用 string。
```

---

### 31.4 Full refresh 直接修改当前 Store

问题：

```text
全量构建期间阻塞查询；
失败可能污染当前索引。
```

当前正确做法：

```text
构建新 Store；
Runtime 原子替换。
```

---

### 31.5 Store 里做完整权限判断

问题：

```text
Store 会依赖 AuthZ / 组织树 / 业务关系；
infra/search 变成权限中心。
```

当前正确做法：

```text
Store 只消费 ProfileAccessScope。
```

---

## 32. 测试建议

建议重点覆盖：

```text
1. DisplayName 写入中文原名 / 拼音 / 简拼；
2. 中文前缀可命中；
3. 拼音前缀可命中；
4. 简拼前缀可命中；
5. ProfileID Hash 精确命中；
6. 手机号 Hash 精确命中；
7. 手机号前缀不命中；
8. ImportTerms 更新姓名后旧姓名不再命中；
9. ImportTerms 更新手机号后旧手机号不再命中；
10. tombstone term 删除 profile；
11. Runtime.Replace 不影响 Current 语义；
12. Runtime.ImportDelta 在当前 Store 上生效；
13. Scope filter 在 RankingPolicy 之前执行。
```

---

## 33. 代码事实源

| 主题 | 文件 |
| ---- | ---- |
| ProfileSearchTerm / Query | `internal/apiserver/domain/suggest/profile.go` |
| ScopePolicy | `internal/apiserver/domain/suggest/scope.go` |
| RankingPolicy | `internal/apiserver/domain/suggest/ranking.go` |
| Trie 三叉搜索树 | `internal/apiserver/infra/suggest/search/trie.go` |
| Hash 精确索引 | `internal/apiserver/infra/suggest/search/hash.go` |
| Store 聚合 | `internal/apiserver/infra/suggest/search/store.go` |
| Runtime | `internal/apiserver/infra/suggest/search/runtime.go` |
| SnapshotWriter | `internal/apiserver/infra/suggest/search/snapshot.go` |
| Loader | `internal/apiserver/infra/mysql/suggest/loader.go` |
| Refresher | `internal/apiserver/application/suggest/refresher.go` |

---

## 34. Verify

建议执行：

```bash
go test ./internal/apiserver/domain/suggest/...
go test ./internal/apiserver/infra/suggest/search/...
go test ./internal/apiserver/application/suggest/...
```

建议 grep：

```bash
grep -R "RemoveProfileID" internal/apiserver/infra/suggest/search
grep -R "profileKeys" internal/apiserver/infra/suggest/search
grep -R "DisplayName" internal/apiserver/infra/mysql/suggest internal/apiserver/infra/suggest/search
```

---

## 35. 下一篇

下一篇建议阅读：

[04-刷新链路-Loader-Refresher-FullDelta-Snapshot.md](./04-刷新链路-Loader-Refresher-FullDelta-Snapshot.md)

它会继续分析：

```text
MySQL Loader 如何生成 ProfileSearchTerm；
Full refresh 如何构建新 Store；
Delta refresh 如何更新当前 Store；
SnapshotWriter 的定位；
Required=false 时如何降级；
PlaceholderOrgID 为什么只是过渡方案。
```
