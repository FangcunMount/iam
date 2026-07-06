# IDP 与第三方登录讲法

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

IDP 负责外部身份源配置和外部身份声明，AuthN 负责登录态。

讲解主线：

```text
external code
  -> IDP resolves external identity
  -> AuthN maps LoginIdentity
  -> AuthN builds Principal
  -> AuthN issues Token
```

重点区分：

- 微信 access token 不是 IAM AccessToken。
- openid / userid 不是 User。
- IDP 不签发 IAM Token。

事实层入口：[../02-业务模块/04-IDP/README.md](../02-业务模块/04-IDP/README.md)。
