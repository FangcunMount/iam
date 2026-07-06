# Transactional Outbox 设计

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

Transactional Outbox 用于解决授权事实写入和事件发布之间的双写问题。

它不是消息队列，也不承诺 exactly-once。它保证业务事务内先记录待发布事件，再由 relay 异步发布。

## 模块回链

当前实现链路见 [../02-业务模块/03-AuthZ/06-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md](../02-业务模块/03-AuthZ/06-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md)。
