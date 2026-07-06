# 关键链路：权限检查 Check

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 链路目标

判断 Subject 是否能对 Resource 执行 Action。

## 链路

```text
RouteAuthorizer / REST / gRPC / SDK
  -> CheckCommand
  -> Checker
  -> AuthorizationRequest
  -> DecisionEngine
  -> Casbin Runtime
  -> AuthorizationDecision
```

## 关键边界

- Check 是读链路，不写授权事实。
- Check 返回决策，不做自动修复。
- Check 不应主动修改 runtime。
