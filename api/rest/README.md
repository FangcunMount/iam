# REST API 契约

REST 契约使用 OpenAPI 3.1。OpenAPI 文件是字段、路径、认证和错误响应的事实源；运行时注册在 [internal/apiserver/transport/rest](../../internal/apiserver/transport/rest)。

## 契约文件

| 文件 | 说明 |
| ---- | ---- |
| [authn.v2.yaml](authn.v2.yaml) | v2 认证、Token、JWKS、账户和 signup |
| [authz.v2.yaml](authz.v2.yaml) | 授权判定、角色、assignment、策略、资源 |
| [identity.v2.yaml](identity.v2.yaml) | 当前用户、profiles、profile-links |
| [idp.v2.yaml](idp.v2.yaml) | IDP 健康检查和微信应用配置 |
| [suggest.v2.yaml](suggest.v2.yaml) | 儿童档案联想搜索 |

## 当前路由口径

| 能力 | 路由 |
| ---- | ---- |
| 登录 | `POST /api/v2/authn/login` |
| 登录准备 | `POST /api/v2/authn/login/prep/phone-otp` |
| Token | `POST /api/v2/authn/refresh_token`、`POST /api/v2/authn/logout`、`POST /api/v2/authn/verify` |
| JWKS | `GET /.well-known/jwks.json`、`GET /api/v2/.well-known/jwks.json` |
| 账户 | `/api/v2/authn/accounts/*`、`/api/v2/authn/signups/wechat-miniprogram` |
| 授权 | `/api/v2/authz/health`、`/api/v2/authz/check`、`/api/v2/authz/{roles,assignments,policies,resources}` |
| Identity | `/api/v2/identity/me`、`/api/v2/identity/profiles`、`/api/v2/identity/profile-links` |
| IDP | `/api/v2/idp/health`、`/api/v2/idp/wechat-apps/*` |
| Suggest | `GET /api/v2/suggest/profile` |
| Debug | `/debug/routes`、`/debug/modules`、`/debug/cache-governance/*` |

Identity 的当前关系术语是 `ProfileLink`。REST 路由使用 `/profile-links`，不再使用旧关系路由。

## 运行时注册

- 总路由入口：[internal/apiserver/transport/rest/router.go](../../internal/apiserver/transport/rest/router.go)
- 模块路由：[internal/apiserver/transport/rest/module_routes.go](../../internal/apiserver/transport/rest/module_routes.go)
- AuthN 路由：[internal/apiserver/transport/rest/authn/router.go](../../internal/apiserver/transport/rest/authn/router.go)
- AuthZ 路由：[internal/apiserver/transport/rest/authz/router.go](../../internal/apiserver/transport/rest/authz/router.go)
- Identity 路由：[internal/apiserver/transport/rest/identity/router.go](../../internal/apiserver/transport/rest/identity/router.go)
- IDP 路由：[internal/apiserver/transport/rest/idp/router.go](../../internal/apiserver/transport/rest/idp/router.go)
- Suggest 路由：[internal/apiserver/transport/rest/suggest/handler.go](../../internal/apiserver/transport/rest/suggest/handler.go)

受保护模块路由依赖 JWT middleware；认证模块不可用时 protected routes fail closed，不注册需要身份上下文的能力。

## 示例

```bash
curl -X POST https://iam.example.com/api/v2/authn/login \
  -H "Content-Type: application/json" \
  -d '{"auth_method":"password","method_payload":{"username":"admin","password":"secret"}}'

curl https://iam.example.com/api/v2/identity/me \
  -H "Authorization: Bearer ${IAM_ACCESS_TOKEN}"

curl -X POST https://iam.example.com/api/v2/authz/check \
  -H "Authorization: Bearer ${IAM_ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"resource":"qs:answer-sheet","action":"submit"}'
```

## 验证

```bash
make docs-swagger
make api-validate
go test ./internal/apiserver/transport/rest
```

`make api-validate` 会比较 `api/rest/*.yaml`、swagger 生成物和实际路由合同；需要 Docker daemon 可用。
