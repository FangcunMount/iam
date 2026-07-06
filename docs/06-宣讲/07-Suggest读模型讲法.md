# Suggest 读模型讲法

> 状态：待补证据 · 宣讲第一版，已按金字塔结构重写；后续需要继续结合 `internal/apiserver/domain/suggest`、`application/suggest`、Suggest Snapshot、索引刷新、可见性过滤、REST/gRPC 契约、Suggest 模块文档和测试逐项核对。

---

## 1. 本文目标

本文用于回答：

```text
Suggest 模块在 IAM 中负责什么？
```

它是宣讲稿，不是完整领域模型文档，适用于：

```text
面试讲解 Suggest；
解释为什么 Suggest 是读模型；
解释 Suggest 与 Identity 的边界；
解释 ProfileSearchTerm / SuggestSnapshot / ProfileAccessScope；
解释手机号搜索为什么要权限控制、脱敏和限流；
解释搜索命中、可见性和授权之间的区别。
```

本文采用金字塔表达：

```text
先一句话定位；
再讲查询主链路；
再讲核心对象；
再讲安全边界；
最后讲常见追问。
```

---

## 2. 一句话定位

Suggest 是 IAM 的 Profile 联想搜索读模型，负责在允许的可见范围内，根据 keyword 找到可展示、可脱敏的 Profile 候选项。

更短一点：

```text
Suggest 管“安全地找候选 Profile”，不管 Profile 主数据写入，也不替代 AuthZ 授权。
```

---

## 3. 30 秒版本

```text
Suggest 模块负责 Profile 联想搜索，它不是 Identity 的核心身份域，而是从 Identity 的 Profile facts 派生出来的读模型。它会把姓名、拼音、手机号 token 等字段构造成 ProfileSearchTerm，再生成 SuggestSnapshot。查询时先对 keyword 做 normalize，再命中索引候选，然后结合 ProfileAccessScope、Identity facts 和 AuthZ 可见性过滤，最后排序、截断、脱敏返回 SuggestResult。这里最重要的边界是：索引命中不等于可见，可见候选也不等于拥有详情读取权限。
```

---

## 4. 1 分钟版本

```text
Suggest 是 IAM 的 Profile 联想搜索读模型，核心问题是“当前请求者在允许范围内，根据 keyword 能看到哪些 Profile 候选项”。它不负责创建 Profile，也不维护 ProfileLink，这些都属于 Identity。Suggest 只是从 Identity 的 Profile facts 中派生搜索 token 和 Snapshot，用来优化查询体验。

查询链路上，Suggest 会先 normalize keyword，再从 SuggestSnapshot 中匹配候选，然后必须做 ProfileAccessScope 和可见性过滤，再排序、limit、脱敏返回结果。手机号搜索是这里最需要强调的安全点：手机号可以作为匹配方式，但不能扩大可见范围；响应只能返回 mobile_mask，不能返回明文手机号；同时要有限流和审计。后续如果用户点击某个候选进入详情，仍然要重新走 AuthZ Check。
```

---

## 5. 3 分钟版本

```text
Suggest 是 IAM 中的 Profile 联想搜索读模型。它的核心价值不是维护身份主数据，而是让业务系统在输入姓名、手机号后缀、拼音或其他 keyword 时，能快速、安全地找到可选择的 Profile 候选项。

我把 Suggest 的讲解分成四层。

第一层是数据来源。Profile 主数据属于 Identity，比如儿童档案、患者档案、User 与 Profile 的关系等。Suggest 不拥有这些写模型，它只是读取或订阅这些身份事实，然后派生出搜索用的 ProfileSearchTerm 和 SuggestSnapshot。

第二层是索引模型。ProfileSearchTerm 是为了搜索而产生的 token，比如姓名归一化、拼音、手机号后缀或 hash token 等。SuggestSnapshot 是某一时刻可用于查询的索引快照。它可以 full refresh 重建，也可以 delta refresh 更新，构建完成后再原子切换，避免查询读到半构建状态。

第三层是查询链路。用户输入 keyword 后，Suggest 先做 normalize，再从 Snapshot 里匹配候选。但候选命中后不能直接返回，必须先做 ProfileAccessScope 和可见性过滤，再对可见候选排序、截断和脱敏。这里顺序很重要：应该先过滤再 limit，否则不可见候选会挤掉真正可见的候选，甚至产生安全侧信号。

第四层是安全边界。SuggestResult 只是脱敏候选展示，不是 Profile entity，也不是授权凭证。能搜索到某个 Profile，不代表可以读取详情、修改档案、导出数据。后续业务操作仍然要用 Resource、Action、Scope 重新走 AuthZ Check。手机号搜索也只能扩大匹配方式，不能扩大可见范围，并且必须脱敏、限流和审计。

所以 Suggest 的定位很清楚：它是一个可重建、可最终一致、可降级的读侧搜索能力，而不是 Identity 主数据，也不是 AuthZ 授权结果。
```

---

## 6. 金字塔结构

### 6.1 顶层结论

```text
Suggest 是 Profile 联想搜索读模型。
```

---

### 6.2 一条主链路

```text
keyword
  -> normalize
  -> match SuggestSnapshot candidates
  -> ProfileAccessScope / visibility filter
  -> rank
  -> limit
  -> mask
  -> SuggestResult
```

---

### 6.3 四个核心对象

| 对象 | 一句话 | 不是什么 |
| --- | --- | --- |
| `ProfileSearchTerm` | 从 Profile facts 派生的搜索 token | 不是 Profile 主字段本体，不是敏感字段明文仓库 |
| `SuggestSnapshot` | 可查询的搜索索引快照 | 不是 Profile 主数据，不是授权事实源 |
| `ProfileAccessScope` | 本次查询允许搜索的范围输入 | 不是 ProfileLink，不是 AuthZ Scope 本体 |
| `SuggestResult` | 脱敏后的候选展示结果 | 不是 Profile entity，不是 AuthorizationDecision |

---

### 6.4 三条核心边界

| 边界 | 说明 |
| --- | --- |
| Suggest vs Identity | Identity 管 Profile 主数据，Suggest 管派生搜索读模型 |
| Suggest vs AuthZ | Suggest 控制候选可搜索性，AuthZ 控制资源操作权限 |
| Suggest vs ProfileLink | ProfileLink 是身份关系事实，ProfileAccessScope 是查询范围输入 |

---

## 7. Suggest 对象讲法

### 7.1 ProfileSearchTerm

讲法：

```text
ProfileSearchTerm 是从 Profile facts 派生出来的搜索 token。它不是 Profile 主字段本身，而是为了支持 keyword 匹配、拼音匹配、手机号安全搜索等查询场景构造出的读侧索引项。
```

重点：

```text
ProfileSearchTerm 可重建；
ProfileSearchTerm 不拥有 Profile 主数据；
手机号等敏感字段不应明文裸存；
命中搜索 token 后仍要做可见性过滤。
```

---

### 7.2 SuggestSnapshot

讲法：

```text
SuggestSnapshot 是某一时刻的搜索读模型快照。查询链路读取 Snapshot，刷新链路构建新的 Snapshot，构建完成后再原子切换。
```

重点：

```text
Snapshot 是读模型；
Snapshot 不是 Identity 主数据；
Snapshot 可以落后于 Identity；
Snapshot 可以 full refresh 重建；
刷新失败不能污染当前可用 Snapshot；
Snapshot 命中不等于可见。
```

---

### 7.3 ProfileAccessScope

讲法：

```text
ProfileAccessScope 表示本次 Suggest 查询允许在哪个范围内找候选，比如当前用户关联档案、当前机构、当前项目或当前服务范围，具体以当前实现为准。
```

重点：

```text
ProfileAccessScope 是查询范围输入；
ProfileAccessScope 不是 ProfileLink；
ProfileAccessScope 不是 AuthZ Scope 本体；
它不能绕过 Identity/AuthZ 可见性过滤；
它不是最终授权决策。
```

---

### 7.4 SuggestResult

讲法：

```text
SuggestResult 是返回给调用方的脱敏候选结果，比如 profile_id、display_name、mobile_mask 等安全可展示字段。
```

重点：

```text
SuggestResult 不是 Profile entity；
SuggestResult 不是授权凭证；
SuggestResult 不应包含明文手机号、证件号、内部 search token；
用户选择候选后，后续详情或操作仍要 AuthZ Check。
```

---

## 8. 查询链路讲法

标准链路：

```text
SuggestProfile request
  -> validate keyword / limit
  -> normalize keyword
  -> detect mobile-like keyword，若需要
  -> match candidates from SuggestSnapshot
  -> apply ProfileAccessScope
  -> visibility filter with Identity/AuthZ facts
  -> rank visible candidates
  -> limit
  -> mask sensitive fields
  -> return SuggestResult
```

讲解重点：

```text
normalize 是为了提升召回；
Snapshot match 只是候选召回；
visibility filter 是安全边界；
rank/limit 是体验优化；
mask 是隐私保护；
安全边界要先于体验优化。
```

边界：

```text
不能索引命中直接返回；
不能先全局 limit 再过滤；
不能返回明文手机号；
不能把 SuggestResult 当授权结果。
```

---

## 9. 为什么要先过滤再截断

错误顺序：

```text
match candidates
  -> rank
  -> limit
  -> visibility filter
```

问题：

```text
不可见候选可能挤掉可见候选；
最终返回数量不稳定；
可能泄露不可见候选的排序侧信号；
不同用户的搜索体验和安全边界都变差。
```

推荐顺序：

```text
match candidates
  -> visibility filter
  -> rank visible candidates
  -> limit
  -> mask
```

讲解句：

```text
可见性是安全边界，排序和截断只是体验优化，所以必须先过滤再截断。
```

---

## 10. 手机号搜索讲法

一句话：

```text
手机号搜索只能扩大匹配方式，不能扩大可见范围。
```

标准链路：

```text
mobile-like keyword
  -> normalize / hash / suffix token
  -> check mobile search permission / policy
  -> rate limit
  -> match mobile search token
  -> visibility filter
  -> return mobile_mask only
  -> audit
```

讲解重点：

```text
手机号是高敏字段；
索引中不应裸存明文手机号；
手机号命中后仍要做可见性过滤；
响应只返回 mobile_mask；
手机号搜索要更严格限流和审计。
```

常见误解：

```text
能用手机号搜到，不代表能看到完整手机号；
能用手机号命中候选，不代表能访问详情；
手机号搜索开关不等于资源授权通过。
```

---

## 11. Suggest 与 Identity 的边界

Identity 回答：

```text
Profile 主数据是什么？User 和 Profile 是什么关系？
```

Suggest 回答：

```text
当前请求者能搜索到哪些 Profile 候选项？
```

正确关系：

```text
Identity Profile facts
  -> Suggest index builder
  -> ProfileSearchTerm / SuggestSnapshot
  -> Suggest query
  -> masked SuggestResult
```

禁止混用：

```text
Suggest 创建 Profile；
Suggest 修改 ProfileLink；
SuggestSnapshot 当 Profile 主数据；
SuggestResult 当 Profile entity；
把搜索索引反写回 Identity 主表。
```

讲解句：

```text
Identity 是事实源，Suggest 是由事实源派生出来的读模型。
```

---

## 12. Suggest 与 AuthZ 的边界

AuthZ 回答：

```text
Subject 能不能对 Resource 执行 Action？
```

Suggest 回答：

```text
当前请求者能不能搜索到某个 Profile 候选？
```

正确关系：

```text
SuggestResult(profile_id=P1)
  -> user selects candidate
  -> detail API builds Resource(profile:P1) + Action(profile.read)
  -> AuthZ Check
  -> allow / deny
```

禁止混用：

```text
搜索到候选就允许读取详情；
SuggestResult 当 AuthorizationDecision；
可见性过滤替代所有 AuthZ Check；
手机号搜索绕过 AuthZ；
JWT claims 直接决定 Suggest 可见性。
```

讲解句：

```text
Suggest 控制“候选能不能被搜索到”，AuthZ 控制“资源操作能不能被执行”。
```

---

## 13. Suggest 与 ProfileLink 的边界

ProfileLink 回答：

```text
User 和 Profile 是什么身份关系？
```

ProfileAccessScope 回答：

```text
本次查询允许在哪个范围内搜索候选？
```

正确关系：

```text
ProfileLink may be one fact
  -> ProfileAccessScope / VisibilityFilter
  -> SuggestResult
```

禁止混用：

```text
ProfileAccessScope 等于 ProfileLink；
有 ProfileLink 就直接返回候选；
无 ProfileLink 就一定不可搜索；
ProfileLink 直接替代 AuthZ visibility；
ProfileLink 直接替代 Permission。
```

讲解句：

```text
ProfileLink 是关系事实，ProfileAccessScope 是查询范围，VisibilityFilter 才决定候选是否可返回。
```

---

## 14. 为什么 Suggest 可以最终一致

讲法：

```text
Suggest 是读模型，所以允许相对 Identity 主数据存在短暂延迟。比如 Profile 刚创建时暂时搜不到，或 Profile 姓名刚修改时 Snapshot 仍显示旧值，这属于读侧最终一致问题。
```

但必须强调：

```text
最终一致不能造成越权；
索引滞后不能跳过可见性过滤；
敏感字段仍要脱敏；
刷新延迟需要可观测；
必要时可以降级为更保守的结果。
```

讲解句：

```text
读模型可以接受体验延迟，但不能接受安全越权。
```

---

## 15. 为什么 Suggest 可以降级

可降级场景：

```text
索引刷新失败；
Snapshot 加载失败；
搜索依赖超时；
手机号搜索被限流；
可见性依赖不可用；
索引版本过旧。
```

可接受降级：

```text
返回空候选；
禁用手机号搜索；
降低 limit；
只支持精确 ID 查询，若安全允许；
提示稍后重试。
```

不可接受降级：

```text
跳过可见性过滤；
返回未脱敏字段；
扩大搜索范围；
忽略手机号限流；
返回内部 search token。
```

讲解句：

```text
Suggest 可以降级，但只能向更保守的方向降级。
```

---

## 16. 典型业务场景讲法

### 16.1 家长搜索儿童档案

```text
家长输入儿童姓名；
Suggest normalize keyword；
Snapshot 命中候选 Profile；
VisibilityFilter 检查家长可见范围；
返回 display_name 和 mobile_mask 等脱敏字段；
点击详情后仍要 AuthZ Check。
```

重点：

```text
搜索候选不是详情授权；
ProfileLink 可以作为可见性事实；
但 ProfileLink 不等于 Permission。
```

---

### 16.2 医生搜索服务范围内儿童

```text
医生输入姓名或手机号后缀；
Suggest 命中候选；
ProfileAccessScope 限定在当前机构、项目或服务范围；
可见性过滤后返回脱敏候选；
医生查看详情或导出报告时重新 AuthZ Check。
```

重点：

```text
服务关系可能影响可见范围；
手机号搜索仍要限流和脱敏；
export 和 read 是不同 Action。
```

---

### 16.3 运营后台手机号搜索

```text
运营输入手机号；
系统识别 mobile-like keyword；
检查手机号搜索能力；
执行更严格限流和审计；
使用手机号 token 命中候选；
可见性过滤；
返回 mobile_mask，不返回明文手机号。
```

重点：

```text
手机号搜索是高风险读侧能力；
不能因为运营能搜索就默认能导出或查看所有详情；
后续操作仍要 AuthZ。
```

---

## 17. 面试追问展开点

| 追问 | 回答要点 |
| --- | --- |
| 为什么 Suggest 是读模型？ | 它由 Identity facts 派生，可重建、可最终一致、可降级，不承载 Profile 写入不变量 |
| 为什么不直接查 Profile 主表？ | 模糊搜索、拼音、手机号安全搜索、排序、脱敏和限流会污染 Identity 写模型 |
| Snapshot 命中能不能直接返回？ | 不能。命中只是候选召回，必须做可见性过滤和脱敏 |
| ProfileAccessScope 是不是 ProfileLink？ | 不是。ProfileAccessScope 是查询范围输入，ProfileLink 是身份关系事实 |
| SuggestResult 能不能当授权凭证？ | 不能。它只是候选展示，后续详情或操作仍要 AuthZ Check |
| 手机号搜索如何保证安全？ | token 化/脱敏、权限控制、限流、审计、可见性过滤，只返回 mobile_mask |
| Suggest 延迟会不会影响业务？ | 可能影响搜索体验，但不应影响 Identity 主数据和安全边界 |
| Suggest 失败怎么办？ | 可以保守降级，例如返回空候选、禁用手机号搜索，不能跳过安全过滤 |

---

## 18. 常见反模式

| 反模式 | 问题 | 推荐做法 |
| --- | --- | --- |
| Suggest 写 Profile 主数据 | 读写模型混淆 | Profile 写入归 Identity |
| 直接查 Identity 主表模糊搜索 | 性能和安全策略污染写模型 | 构建 SuggestSnapshot |
| Snapshot 命中直接返回 | 可能越权 | 先可见性过滤再返回 |
| 先 limit 再过滤 | 可见候选被不可见候选挤掉 | filter -> rank -> limit |
| 返回明文手机号 | 隐私泄露 | 只返回 mobile_mask |
| ProfileAccessScope 当 ProfileLink | 查询范围和关系事实混淆 | Scope 由多种 facts 计算 |
| SuggestResult 当授权凭证 | 后续接口可能越权 | 后续操作重新 AuthZ Check |
| 索引刷新失败覆盖旧 Snapshot | 查询不可用或数据错乱 | 构建成功后原子切换 |
| 降级时跳过权限过滤 | 安全事故 | 只能保守降级 |
| 手机号搜索无限流无审计 | 枚举风险 | 严格限流和审计 |

---

## 19. 推荐表达顺序

讲 Suggest 时建议按这个顺序：

```text
1. 先说 Suggest 是 Profile 联想搜索读模型；
2. 说明它不拥有 Profile 写模型；
3. 讲 ProfileSearchTerm / SuggestSnapshot；
4. 讲查询链路：normalize -> match -> filter -> rank -> limit -> mask；
5. 强调先过滤再截断；
6. 讲手机号搜索安全；
7. 回到 Identity / AuthZ / ProfileLink 边界；
8. 说明最终一致和保守降级。
```

不推荐：

```text
一上来讲搜索算法；
把 Suggest 讲成 Profile 主数据；
只讲性能，不讲安全；
把 ProfileAccessScope 讲成权限；
把 SuggestResult 讲成授权结果；
忽略手机号搜索的隐私风险。
```

---

## 20. 事实源回链

| 内容 | 事实源 |
| --- | --- |
| Suggest 模块 | [../02-业务模块/05-Suggest/README.md](../02-业务模块/05-Suggest/README.md) |
| Suggest 领域模型 | [../02-业务模块/05-Suggest/01-领域模型-ProfileSearchTerm-ProfileAccessScope-Snapshot.md](../02-业务模块/05-Suggest/01-领域模型-ProfileSearchTerm-ProfileAccessScope-Snapshot.md) |
| 索引刷新链路 | [../02-业务模块/05-Suggest/02-关键链路-索引刷新Full-Delta-Snapshot.md](../02-业务模块/05-Suggest/02-关键链路-索引刷新Full-Delta-Snapshot.md) |
| SuggestProfile 查询链路 | [../02-业务模块/05-Suggest/03-关键链路-SuggestProfile查询.md](../02-业务模块/05-Suggest/03-关键链路-SuggestProfile查询.md) |
| 手机号安全策略 | [../02-业务模块/05-Suggest/04-安全策略-手机号搜索-脱敏-限流.md](../02-业务模块/05-Suggest/04-安全策略-手机号搜索-脱敏-限流.md) |
| Suggest 读模型专题 | [../05-专题设计/06-Suggest为什么是读模型.md](../05-专题设计/06-Suggest为什么是读模型.md) |
| Identity 讲法 | [03-Identity讲法.md](03-Identity讲法.md) |
| AuthZ 讲法 | [05-AuthZ讲法.md](05-AuthZ讲法.md) |
| ProfileLink 专题 | [../05-专题设计/05-ProfileLink为什么不是Permission.md](../05-专题设计/05-ProfileLink为什么不是Permission.md) |

---

## 21. Verify

修改本文后至少执行：

```bash
make docs-hygiene
```

如果同步修改 Suggest 相关代码或契约，需要执行：

```bash
go test ./internal/apiserver/domain/suggest/...
go test ./internal/apiserver/application/suggest/...
go test ./internal/apiserver/domain/identity/...
go test ./internal/apiserver/application/identity/...
go test ./internal/apiserver/domain/authz/...
go test ./internal/apiserver/application/authz/...
make api-validate
make proto-gen
go test ./internal/pkg/architecture
```

---

## 22. 本文总结

Suggest 讲法可以压缩成：

```text
Suggest 是 Profile 联想搜索读模型；
ProfileSearchTerm 是搜索 token；
SuggestSnapshot 是可重建快照；
ProfileAccessScope 是查询范围输入；
SuggestResult 是脱敏候选展示；
索引命中不等于可见；
可见候选不等于有详情权限；
手机号搜索必须权限控制、脱敏、限流和审计。
```

宣讲时最重要的是：

```text
把 Suggest 和 Identity 写模型分开；
把搜索命中、可见性过滤、AuthZ 授权分开；
用手机号搜索讲清楚读侧安全设计；
用最终一致和保守降级体现读模型设计取舍。
```
