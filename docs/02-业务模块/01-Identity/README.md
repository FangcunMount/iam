# Identity

## 30 秒结论

Identity 是 IAM 的身份事实中心。它维护 `User`、`Profile`、`ProfileLink`，回答：

```text
系统内部这个人是谁？
这个人关联了哪些业务档案？
这些关系如何建立、查询和撤销？
```

## 文档结构

| 文档 | 作用 |
| --- | --- |
| [00-模块总览.md](00-模块总览.md) | Identity 职责和边界 |
| [01-领域模型-User-Profile-ProfileLink.md](01-领域模型-User-Profile-ProfileLink.md) | User、Profile、ProfileLink 模型 |
| [02-领域模型图.md](02-领域模型图.md) | 领域模型图 |
| [03-核心对象生命周期.md](03-核心对象生命周期.md) | User/Profile/ProfileLink 生命周期 |
| [04-关键链路-创建User与Profile.md](04-关键链路-创建User与Profile.md) | 创建身份主体和档案 |
| [05-关键链路-建立与撤销ProfileLink.md](05-关键链路-建立与撤销ProfileLink.md) | ProfileLink 关系协作 |
| [06-模块边界-Identity与AuthN-AuthZ-Suggest.md](06-模块边界-Identity与AuthN-AuthZ-Suggest.md) | 与其他模块边界 |
| [07-分层架构与代码索引.md](07-分层架构与代码索引.md) | 代码事实源 |
