# 关键链路：索引刷新 Full / Delta / Snapshot

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

把 Profile 相关事实加载为进程内搜索索引。

## 链路

```text
Profile data source
  -> Loader
  -> Full / Delta refresh
  -> build Snapshot
  -> atomic runtime swap
```

## 关键边界

- Full 用于完整重建。
- Delta 用于增量刷新。
- Snapshot 用于运行时安全切换。
- 刷新失败不应产生半更新索引。
