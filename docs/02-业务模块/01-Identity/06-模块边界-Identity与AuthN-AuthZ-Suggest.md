# 模块边界：Identity 与 AuthN / AuthZ / Suggest

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 与 AuthN

AuthN 通过 `Principal.UserID` 指向 Identity.User。AuthN 不拥有 User 写模型。

## 与 AuthZ

AuthZ 通过 `Subject` 引用 User。Subject 是授权引用，不是 User 本体。

## 与 Suggest

Suggest 使用 Profile 相关读事实构建索引。Suggest 不修改 Profile，也不把 ProfileLink 当成可见范围。

## 规则

- Identity 只维护身份事实。
- 跨模块只能通过应用层端口或明确的查询能力协作。
- 不允许为了方便查询把认证、授权、搜索规则塞回 Identity。
