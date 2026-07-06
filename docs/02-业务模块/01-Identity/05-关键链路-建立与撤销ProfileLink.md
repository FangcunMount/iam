# 关键链路：建立与撤销 ProfileLink

## 链路目标

表达 User 与 Profile 的关系事实，并支持查询和撤销。

## 建立关系

```text
validate user/profile
  -> validate relation rule
  -> create active ProfileLink
  -> persist
```

## 撤销关系

```text
load ProfileLink
  -> mark revoked
  -> persist revoked state
```

## 关键约束

- 不把 ProfileLink 写成授权策略。
- 不把 ProfileLink 写成 Suggest 可见范围。
- 查询链路需要区分 active 与 including revoked。
