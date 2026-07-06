# REST 接入契约

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 事实源

REST 契约以 OpenAPI 文件为准：

- `../../api/rest/authn.v2.yaml`
- `../../api/rest/authz.v2.yaml`
- `../../api/rest/identity.v2.yaml`
- `../../api/rest/idp.v2.yaml`
- `../../api/rest/suggest.v2.yaml`

运行时入口：

- `../../internal/apiserver/transport/rest`

## 规则

- 文档不手写完整 REST schema。
- 路径、字段、错误响应以 OpenAPI 为准。
- 业务解释回链到 [02-业务模块](../02-业务模块/README.md)。

## Verify

```bash
make api-validate
```
