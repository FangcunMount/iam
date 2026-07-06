# 关键链路：授权版本传播 PolicyVersion / Outbox / RuntimeReload

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

在授权事实变化后，让运行时授权引擎最终看到新版本。

## 链路

```text
PolicyChange committed
  -> PolicyVersion persisted
  -> Outbox event staged
  -> local RuntimeReload
  -> relay publishes version changed event
  -> consumers reload runtime
```

## 关键边界

- Outbox 解决数据库写入和事件发布的双写问题。
- Outbox 不等于消息队列。
- RuntimeReload 不应放在数据库事务内。
- 设计取舍见 [../../05-专题设计/03-Transactional-Outbox设计.md](../../05-专题设计/03-Transactional-Outbox设计.md)。
