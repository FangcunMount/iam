# AuthN 讲法

AuthN 负责证明调用者是谁。

讲解主线：

```text
LoginIdentity / Credential / Challenge
  -> Principal
  -> Session
  -> AccessToken / RefreshToken
```

重点区分：

- Credential 是长期认证材料。
- Challenge 是短期认证挑战。
- Principal 是认证结果，不是 JWT。
- Token 是认证结果的访问凭证表达。

事实层入口：[../02-业务模块/02-AuthN/README.md](../02-业务模块/02-AuthN/README.md)。
