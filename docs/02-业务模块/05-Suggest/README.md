# Suggest：Profile 联想搜索读模型

> 状态：已实现 · 已按当前 domain、application、MySQL、内存索引、REST、container、契约与测试重新核对。

## 本目录回答

Suggest 为 operating 后台提供低延迟的 Profile 候选搜索：它从 Identity 的 MySQL 事实构建可丢弃的进程内索引，在返回候选前读取 AuthZ 能力并自行组合数据可见范围，并对手机号搜索、
限流和输出脱敏做额外保护。

它不拥有 User、Profile、ProfileLink，不负责详情读取授权，也不把 TST/Hash 这种检索算法冒充成领域模型。

## 30 秒结论

- Suggest 是策略密集的查询域，不是事务聚合域。领域知识集中在 `profile`、`visibility`、`search` 三个 domain 子包。
- `SuggestibleProfile` 保证最小构造形状；默认 SQL 决定入索引资格，Scope 决定本次可见性，三者分别负责。Identity MySQL 仍是事实源，索引不回写。
- 非纯数字关键词走姓名/拼音/首字母 TST，输入 `*`、`.` 也有通配意义；纯数字走 ProfileID/手机号 Hash 精确召回。
- 非空关键词主链为：JWT/可选路由授权 → 绑定/Principal/限流 → scope → 准入 → 预算内召回 → 可见性过滤 → 去重排序 → limit → 手机号披露；排序只覆盖已召回集合。
- Full 构建新 Store 后原子切换；Delta 在当前 Store 写锁内应用 Upsert/Delete并撤销旧keys。同一Refresher互斥刷新，时间游标不是提交水位，成功/曾刷新不证明持续新鲜。
- 前置门禁与Scope成功后，手机号形态查询缺少本分支能力才返回 `200 + []`；响应默认只返回第一个手机号的掩码。
- 当前只提供 REST；没有 gRPC/Go SDK，也没有持久化索引快照。

## 文档结构

| 文档 | 唯一职责 |
| --- | --- |
| [模块总览](00-模块总览.md) | 问题空间、架构选择、运行边界和主要风险 |
| [模型与应用端口](01-模型与应用端口.md) | 字段来源/最小形状、号码表示/披露、Scope/能力分支、预算内选择与端口替换合同 |
| [Full/Delta 索引刷新](02-关键链路-索引刷新Full-Delta.md) | affected/tombstone、发布与重放、秒级时间/晚提交、调度与健康、恢复和自定义来源验收 |
| [SuggestProfile 查询](03-关键链路-SuggestProfile查询.md) | 请求优先级、能力/缓存/取消窗口、通配与预算、两类限流、错误/空结果和证据范围 |
| [模块边界与代码索引](04-模块边界与代码索引.md) | 实际装配/配置来源、能力/资源责任、替换合同、代码地图和护栏/验收范围 |
| [为什么采用派生读模型](../../06-专题设计/05-Suggest为什么是读模型.md) | 方案演化、替代设计和取舍 |

## 主链路

```text
Identity MySQL facts
  -> ProjectionSource.Full / Delta
  -> SuggestibleProfile / ProjectionChange
  -> process-local TST + Hash

JWT + AuthZ Resource/Action capabilities + local visibility facts
  -> visibility.Scope
  -> AdmissionPolicy
  -> CandidateRecaller
  -> SelectionPolicy
  -> MobileDisclosurePolicy
  -> GET /api/v2/suggest/profile
```

## 按任务阅读

| 任务 | 先读 |
| --- | --- |
| 修改搜索字段、合法状态或手机号输出 | [领域模型与应用端口](01-模型与应用端口.md) |
| 修改姓名、拼音、ID 或手机号召回 | [查询链路](03-关键链路-SuggestProfile查询.md) 与 [代码索引](04-模块边界与代码索引.md) |
| 修改可见范围或 list_all/search_by_mobile_all/search_by_mobile capability | [查询链路](03-关键链路-SuggestProfile查询.md) |
| 修改 Full/Delta SQL 或 eligibility | [索引刷新](02-关键链路-索引刷新Full-Delta.md) |
| 接入 Elasticsearch 或其他索引 | [模块边界与代码索引](04-模块边界与代码索引.md)：重新装配、发布者/代次与readiness/关闭合同 |
| 排查空结果、索引滞后或多实例差异 | [查询链路](03-关键链路-SuggestProfile查询.md)、[模块总览](00-模块总览.md) 与 [索引刷新](02-关键链路-索引刷新Full-Delta.md) |

## 当前限制与正文归属

1. 可见性过滤发生在最终 limit 前，但召回本身仍先受 `CandidateBudget` 限制；短前缀下可能出现“后面有可见项但召回窗口没有覆盖”的结果不足。
2. 默认 Loader 的 `OrgID` 是过渡占位，visibility reader 也只按 `profiles.created_by` 解析；它不是成熟的多组织数据权限模型。
3. `required=false` 只控制启动失败是否阻断进程。首次 Full 或调度初始化失败后会使用 `DegradedQuerier` 返回空数组，当前进程不会后台自愈，需要重启或后续实现恢复机制。
4. Full/Delta 由各实例独立执行；健康检查只证明本实例至少成功刷新过一次，不证明 generation 一致或满足数据新鲜度 SLA。
5. 原始手机号会存在于 Loader 结果和进程内 Hash 中；当前保证是响应默认脱敏、Suggest 专用日志不记录原始关键词、没有落盘 snapshot，并不是“内存中无敏感数据”。
6. 当前 Identity 保存 E.164，Suggest Hash 保留原值，但带 `+` 的关键词走文本意图；裸数字与带国家码 key 又不同。号码交接、长 ID 准入和全量/局部手机号能力取舍见 [模型主文](01-模型与应用端口.md)，现有 SQLite 裸号码 fixture 不证明真实 writer→查询闭环。

限制的判断与演进条件由 [模块总览](00-模块总览.md) 维护；索引一致性与健康细节见 [Full/Delta 刷新](02-关键链路-索引刷新Full-Delta.md)，授权、限流与披露顺序见 [查询链路](03-关键链路-SuggestProfile查询.md)。

## 事实源

发生冲突时按以下顺序判断：

1. `internal/apiserver/domain/suggest` 与 `internal/apiserver/application/suggest`
2. `internal/apiserver/infra/mysql/suggest` 与 `internal/apiserver/infra/suggest`
3. `internal/apiserver/transport/rest/suggest`、`internal/apiserver/container/suggest`
4. `api/rest/suggest.v2.yaml` 与生成的 Swagger
5. 聚焦测试、架构护栏和本目录文档

全局写作与验证规则见 [CONTRIBUTING-DOCS.md](../../CONTRIBUTING-DOCS.md)。

## 验证

```bash
go test -race -count=1 \
  ./internal/apiserver/domain/suggest/... \
  ./internal/apiserver/application/suggest/... \
  ./internal/apiserver/infra/suggest/... \
  ./internal/apiserver/infra/mysql/suggest/... \
  ./internal/apiserver/transport/rest/suggest/... \
  ./internal/apiserver/container/suggest/...

go test -count=1 ./internal/pkg/architecture/...
make api-validate
make docs-hygiene
make docs-facts
```
