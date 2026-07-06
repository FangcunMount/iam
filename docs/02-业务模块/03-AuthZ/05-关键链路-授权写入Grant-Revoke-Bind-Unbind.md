# 关键链路：授权写入 Grant / Revoke / Bind / Unbind

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

把授权管理操作转换为一致的授权事实变更。

## 链路

```text
Grant / Revoke / Bind / Unbind
  -> Application Command
  -> PolicyAdministration
  -> AuthorizationPolicy
  -> PolicyChange
  -> Committer
  -> persist management facts + runtime facts
```

## 关键边界

- 授权写入不是简单 CRUD。
- 写入要同时维护管理事实和运行时事实。
- 写入成功后应推动 PolicyVersion 变化。
