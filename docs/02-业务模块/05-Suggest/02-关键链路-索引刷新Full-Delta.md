# 关键链路：索引刷新 Full / Delta

> 状态：已实现 · 编排位于 `application/suggest/refreshindex`。

## 1. 30 秒结论

```text
cron / 启动
  → refreshindex.Refresher
  → ProjectionSource.Full / Delta (mysql)
  → IndexWriter.Replace / Apply (memory TST/Hash)
```

- Full 查询**开始时间**作为 Delta cursor
- `TryLock` 互斥；`refresh_in_progress` 指标
- Loader/Writer 失败**不推进** cursor
- 首次 Full 成功前 Delta no-op

## 2. 变更模型

Delta 在 mysql adapter 边界构造 `refreshindex.ProjectionChange`（Upsert/Delete），**不在 domain** 表达物理索引动作。

空 `DisplayName` 的 Delta 行 → `ChangeDelete`；有效行 → `ChangeUpsert`。

## 3. 端口

| 端口 | 实现 |
| --- | --- |
| `ProjectionSource` | `infra/mysql/suggest.Loader` |
| `IndexWriter` | `infra/suggest/index/memory.Runtime` |

配置：`suggest.full_sql` / `suggest.delta_sql` / `loader_placeholder_org_id` 归属 mysql LoaderConfig。
