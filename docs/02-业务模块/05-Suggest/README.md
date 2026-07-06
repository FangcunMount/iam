# Suggest

## 30 秒结论

Suggest 是 Profile 联想搜索读模型模块。它服务管理端或业务后台的 Profile autocomplete，基于进程内索引快速召回，再用 `ProfileAccessScope` 做可见范围过滤。

Suggest 不是核心身份域，不拥有 Profile 写模型，不负责认证或通用授权策略管理。

## 文档结构

| 文档 | 作用 |
| --- | --- |
| [00-模块总览.md](00-模块总览.md) | Suggest 职责和边界 |
| [01-领域模型-ProfileSearchTerm-ProfileAccessScope-Snapshot.md](01-领域模型-ProfileSearchTerm-ProfileAccessScope-Snapshot.md) | 搜索读模型 |
| [02-领域模型图.md](02-领域模型图.md) | 模型图 |
| [03-关键链路-SuggestProfile查询.md](03-关键链路-SuggestProfile查询.md) | 查询链路 |
| [04-关键链路-索引刷新Full-Delta-Snapshot.md](04-关键链路-索引刷新Full-Delta-Snapshot.md) | 索引刷新 |
| [05-安全策略-手机号搜索-脱敏-限流.md](05-安全策略-手机号搜索-脱敏-限流.md) | 安全策略 |
| [06-模块边界-Suggest与Identity-AuthZ.md](06-模块边界-Suggest与Identity-AuthZ.md) | 与 Identity/AuthZ 的边界 |
| [07-分层架构与代码索引.md](07-分层架构与代码索引.md) | 代码事实源 |
