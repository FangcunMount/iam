# 关键链路：SuggestProfile 查询与安全策略

> 状态：已实现 · REST-only；编排位于 `application/suggest/queryprofile`。

## 1. 30 秒结论

```text
GET /api/v2/suggest/profile?k=<keyword>&limit=<n>
```

```text
JWT → visibility.Principal
  → ScopeResolver (AuthZ facts + visibility IDs → Scope)
  → AdmissionPolicy (mobile_denied / numeric_exact / prefix_text)
  → CandidateRecaller (memory TST/Hash，仅召回)
  → SelectionPolicy (scope 过滤、去重、排序、limit)
  → MobileDisclosurePolicy
  → REST DTO
```

空关键词**不**访问 AuthZ、visibility 或 index。

## 2. 决策矩阵（行为不变）

| 关键词形态 | 手机号权限 | 结果 |
| --- | --- | --- |
| 空 / whitespace | — | `200` + `[]` |
| 7–15 位纯数字（mobile-shaped） | 否 | `200` + `[]`，**不访问 index** |
| 7–15 位纯数字 | 是 | 精确召回 |
| 其他纯数字 | — | 精确召回（档案 ID） |
| 非数字 | — | 文本前缀召回 |

指标：`DecisionKind` 在 infra metrics 映射为 `mobile_denied` / `numeric_exact` / `prefix_text`。

隐私：原始手机号只存在于进程内索引；响应默认脱敏，不写入文件或日志。

## 3. 已知限制（本轮不修复）

召回窗口在 scope 过滤前按 `CandidateBudget` 截断，可能导致可见结果不足；属既有行为，需另立需求。
