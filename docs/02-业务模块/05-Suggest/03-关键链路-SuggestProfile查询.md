# 关键链路：SuggestProfile 查询

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

返回当前操作者有权查看的 Profile 候选项。

## 链路

```text
GET /api/v2/suggest/profile
  -> extract OperatingPrincipal
  -> normalize query
  -> resolve ProfileAccessScope
  -> runtime index match
  -> scope filter
  -> rank + limit
  -> mobile mask
```

## 关键边界

- 不能先全局 limit 再过滤 scope。
- 手机号形态关键词需要额外权限。
- 响应只返回 `mobile_mask`。
