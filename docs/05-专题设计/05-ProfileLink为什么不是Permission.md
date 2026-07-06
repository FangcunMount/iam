# ProfileLink 为什么不是 Permission

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

ProfileLink 是身份关系事实，Permission 是访问权声明。两者解决的问题不同：

| 概念 | 回答 |
| --- | --- |
| ProfileLink | User 和 Profile 是什么关系 |
| Permission | Subject 能否对 Resource 执行 Action |

把 ProfileLink 当 Permission 会让身份事实和授权策略耦合，后续很难维护。

## 模块回链

- Identity 模型见 [../02-业务模块/01-Identity](../02-业务模块/01-Identity/README.md)。
- AuthZ 模型见 [../02-业务模块/03-AuthZ](../02-业务模块/03-AuthZ/README.md)。
