# Session / AccessToken / RefreshToken 边界

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

- Session 是服务端认证上下文。
- AccessToken 是短期访问凭证。
- RefreshToken 是续期凭证。

三者不要混成一个“登录 token”。Session 负责服务端状态，AccessToken 负责资源访问，RefreshToken 负责续期。

## 模块回链

当前实现链路见 [../02-业务模块/02-AuthN/07-关键链路-Token签发刷新吊销.md](../02-业务模块/02-AuthN/07-关键链路-Token签发刷新吊销.md)。
