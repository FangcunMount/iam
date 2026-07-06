# 模块边界：AuthN 与 Identity / IDP / AuthZ

## 与 Identity

AuthN 通过 UserID 引用 Identity.User。AuthN 不拥有 User/Profile/ProfileLink 写模型。

## 与 IDP

IDP 提供外部身份源配置和外部身份声明。AuthN 决定外部身份如何映射到 LoginIdentity，并决定是否认证成功。

## 与 AuthZ

AuthN 不判断权限。AuthN 产出的 Principal 可以被接入层转换为 AuthZ 所需的 Subject。

## 规则

- 登录态不进入 Identity。
- 授权事实不进入 AuthN。
- IDP access token 不等于 IAM AccessToken。
