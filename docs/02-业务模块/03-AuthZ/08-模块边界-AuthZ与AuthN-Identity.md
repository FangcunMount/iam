# 模块边界：AuthZ 与 AuthN / Identity

## 与 AuthN

AuthN 负责认证并产出 Principal。AuthZ 只消费授权主体引用，不决定登录是否成功。

## 与 Identity

AuthZ 通过 Subject 引用 Identity.User。AuthZ 不修改 User、Profile、ProfileLink。

## 规则

- Subject 不是 User 本体。
- RoleBinding 不是 ProfileLink。
- Permission 不是 Profile 关系。
- Token claims 不替代 AuthZ Check。
