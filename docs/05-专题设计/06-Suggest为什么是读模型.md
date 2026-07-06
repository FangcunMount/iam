# Suggest 为什么是读模型

## 30 秒结论

Suggest 是为 Profile autocomplete 构建的辅助读模型。它使用 Identity 的 Profile 事实和权限范围生成候选结果，但不拥有 Profile 写模型。

## 为什么不放进核心身份域

- Suggest 解决查询体验，不定义身份事实。
- Suggest 需要索引、刷新、限流、脱敏，这些是读侧能力。
- Suggest 可以降级，不应阻断 IAM 核心身份、认证、授权能力。

## 模块回链

当前实现链路见 [../02-业务模块/05-Suggest](../02-业务模块/05-Suggest/README.md)。
