# 关键链路：索引刷新 Full / Delta

> 状态：已实现 · 已与 `ProfileIndexRefresher`、MySQL Loader、search Runtime/Store、container 和并发测试核对。

## 1. 结论

Suggest 索引只存在于进程内，不写入文件。启动时先执行 Full refresh，成功后才启动 Cron。Full 与 Delta 共用一把非阻塞刷新锁：任务重叠时后到任务返回 `refresh_in_progress` 并立即结束，不堆积刷新 goroutine。

```text
Full:  windowStart -> MySQL Full -> Runtime.Replace -> lastFetch=windowStart
Delta: lastFetch -> windowStart -> MySQL Delta(since) -> Runtime.ImportDelta -> lastFetch=windowStart
```

查询窗口在访问 Loader 之前捕获。这样查询期间发生的变更最晚会在下一次 Delta 中被重复读取，不会因为“查询结束后才取 now”而被游标跳过。

## 2. 刷新顺序

```mermaid
sequenceDiagram
    participant C as Cron
    participant R as ProfileIndexRefresher
    participant L as MySQL Loader
    participant RT as Runtime

    C->>R: RunFull / RunDelta
    alt refresh lock is busy
        R-->>C: refresh_in_progress
    else lock acquired
        R->>R: capture windowStart
        R->>L: Full() / Delta(lastFetch)
        L-->>R: terms / tombstones
        R->>RT: Replace / ImportDelta
        R->>R: lastFetch = windowStart
    end
```

Full 使用新 Store 原子替换当前指针；构建期间查询继续读取旧 Store。Delta 在当前 Store 的互斥区内撤销旧键，再导入 upsert 或删除 tombstone。

## 3. 游标不变量

- `lastFetch` 只在刷新锁内读写。
- Full 成功后将游标设为 Full 查询开始时间。
- Delta 成功后将游标设为 Delta 查询开始时间。
- 空 Delta 也是成功，会推进游标。
- Loader 或 Runtime 更新失败时不推进游标。
- 首次 Full 尚未成功时，Delta 直接结束。

默认 Delta SQL 返回变更的活跃 Profile 和软删除 tombstone。当前 tombstone 协议是 `DisplayName` 为空；自定义 `delta_sql` 必须保持该协议。

## 4. 敏感数据边界

Loader 与内存索引可以包含原始手机号，用于授权后的手机号搜索；响应仍只返回脱敏值。刷新数据不得写盘或写入日志。

从旧版本升级时，必须在所有旧实例退出后精确删除旧数据目录中的遗留 `snapshot.txt`，不得递归删除整个目录，也不得为该遗留文件制作新备份。

## 5. 失败策略

| 场景 | 行为 |
| --- | --- |
| Full 查询失败 | 保留旧 Runtime；启动阶段按 `required` 决定失败或降级 |
| Delta 查询失败 | 保留旧 Runtime 和旧游标 |
| Runtime 更新失败 | 保留旧游标 |
| 刷新任务重叠 | 后到任务跳过并记录非错误状态 |
| 后续 Cron 失败 | 记录不含候选数据的错误，继续使用当前索引 |

## 6. 事实源与验证

| 内容 | 路径 |
| --- | --- |
| Refresher、锁和游标 | `internal/apiserver/application/suggest/refresher.go` |
| MySQL Loader | `internal/apiserver/infra/mysql/suggest/loader.go` |
| Runtime / Store | `internal/apiserver/infra/suggest/search` |
| Cron 装配 | `internal/apiserver/container/suggest/module.go` |
| 配置 | `configs/apiserver.dev.yaml`、`configs/apiserver.prod.yaml` |

```bash
go test -race ./internal/apiserver/application/suggest/... \
  ./internal/apiserver/infra/suggest/... \
  ./internal/apiserver/container/suggest
```
