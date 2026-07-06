# 领域模型：ProfileSearchTerm / ProfileAccessScope / Snapshot

| 模型 | 定义 |
| --- | --- |
| OperatingPrincipal | 当前后台操作者 |
| ProfileAccessScope | 当前操作者可见 Profile 范围 |
| ProfileSearchTerm | 索引中的 Profile 搜索项 |
| Query | 规范化后的搜索请求 |
| SuggestSnapshot | 某一版本的索引快照 |
| SuggestResult | 脱敏后的候选结果 |

## 关键边界

- ProfileSearchTerm 是读模型，不是 Profile 写模型。
- ProfileAccessScope 是可见范围，不是 ProfileLink。
- 手机号搜索必须受权限、脱敏、限流约束。
