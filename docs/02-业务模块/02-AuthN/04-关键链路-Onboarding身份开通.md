# 关键链路：Onboarding 身份开通

## 链路目标

首次开通 User、LoginIdentity，并在需要时创建 Credential。

## 链路

```text
Onboarding input
  -> validate User input
  -> create or resolve User through Identity
  -> create LoginIdentity
  -> optional Credential
  -> return onboarding result
```

## 关键边界

- Onboarding 不是 Login。
- Onboarding 不是已登录用户绑定更多身份。
- Credential 是可选的；外部 IDP 登录通常没有长期 Credential。
