# 关键链路：微信 AccessToken 获取与缓存

## 链路目标

为外部微信 API 调用提供应用 access token，并通过缓存减少外部调用。

## 链路

```text
load WechatApp
  -> check app enabled
  -> read AccessTokenCache
  -> if miss, acquire refresh lock
  -> AppTokenProvider.Fetch
  -> cache with TTL
  -> return token
```

## 关键边界

- 微信 access token 是外部 API token。
- IAM AccessToken 由 AuthN 签发。
- 缓存失败不能被写成认证成功或失败的业务事实。
