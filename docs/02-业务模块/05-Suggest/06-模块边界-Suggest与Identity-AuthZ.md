# 模块边界：Suggest 与 Identity / AuthZ

## 与 Identity

Suggest 消费 Profile 读事实，构建搜索索引。Suggest 不修改 User、Profile、ProfileLink。

## 与 AuthZ

Suggest 使用 ProfileAccessScope 做数据可见范围。它不管理通用 Role、Permission、RoleBinding。

## 规则

- Suggest 是辅助读模型。
- ProfileAccessScope 不是 AuthZ 的替代模型。
- Suggest 不把 tenant/org 重构假设写成已实现事实。

设计取舍见 [../../05-专题设计/06-Suggest为什么是读模型.md](../../05-专题设计/06-Suggest为什么是读模型.md)。
