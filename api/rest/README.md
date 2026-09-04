# REST API 契约

REST 契约使用 OpenAPI 3.1。OpenAPI 文件是字段、路径、认证和错误响应的事实源；运行时注册在 [internal/apiserver/transport/rest](../../internal/apiserver/transport/rest)。

## 契约文件

| 文件 | 说明 |
| ---- | ---- |
| [authn.v2.yaml](authn.v2.yaml) | v2 认证、Challenge、LoginIdentity、Token、JWKS 和 signup |
| [authz.v3.yaml](authz.v3.yaml) | PermissionGrant、Role、Assignment、RoleInheritance、Resource 与属性 Schema 管理；不含 `Check` |
| [identity.v2.yaml](identity.v2.yaml) | 当前用户、profiles、profile-links 查询；Profile/ProfileLink 创建命令走 gRPC |
| [idp.v2.yaml](idp.v2.yaml) | IDP 健康检查和微信应用配置 |
| [suggest.v2.yaml](suggest.v2.yaml) | 儿童档案联想搜索 |

## 当前路由口径

| 能力 | 路由 |
| ---- | ---- |
| 登录 | `POST /api/v2/authn/login` |
| 登录挑战 | `POST /api/v2/authn/challenges/phone-otp` |
| Token | `POST /api/v2/authn/refresh_token`、`POST /api/v2/authn/logout`、`POST /api/v2/authn/verify` |
| JWKS | `GET /.well-known/jwks.json`、`GET /api/v2/.well-known/jwks.json` |
| AuthN JWKS 管理 | `/api/v2/authn/admin/jwks/keys` 及其 publishable、retire、force-retire、cleanup 子路由 |
| LoginIdentity | `GET /api/v2/authn/login-identities`、`POST /api/v2/authn/login-identities/phone`、`DELETE /api/v2/authn/login-identities/{id}` |
| Signup | `POST /api/v2/authn/signups/wechat-miniprogram` |
| AuthZ 管理面 | `GET /api/v3/authz/health`、`/api/v3/authz/{roles,assignments,grants,role-inheritances,resources}` |
| Identity | `GET /api/v2/identity/me`、`GET /api/v2/identity/me/profiles`、`GET /api/v2/identity/profiles/{id}`、`GET /api/v2/identity/profile-links` |
| IDP | `/api/v2/idp/health`、`/api/v2/idp/wechat-apps/*` |
| Suggest | `GET /api/v2/suggest/profile` |
| Debug | `/debug/routes`、`/debug/modules`、`/debug/cache-governance/*` |

Identity 的当前关系术语是 `ProfileLink`。REST 路由使用 `/profile-links`，不再使用旧关系路由。Profile 与 ProfileLink 的创建/撤销命令由 gRPC `ProfileCommand`、`ProfileLinkCommand` 承接，REST 仅保留查询和当前用户资料更新能力。

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

curl https://iam.example.com/api/v3/authz/roles \
  -H "Authorization: Bearer ${IAM_ACCESS_TOKEN}"
```

AuthZ REST v3 只承接管理命令和查询，不存在 REST `Check`。权限判定是可信服务间调用，使用 gRPC v3：

```bash
grpcurl \
  -H "authorization: Bearer ${IAM_SERVICE_TOKEN}" \
  -d '{"subject":"user:1024","domain":"default","resource":"qs:answersheet:collection:answersheets","action":"admin_submit"}' \
  iam.example.com:443 iam.authz.v3.AuthorizationService/Check
```

## 验证

```bash
make docs-swagger
make api-validate
go test ./internal/apiserver/transport/rest
```

`make api-validate` 会比较 `api/rest/*.yaml`、swagger 生成物和实际路由合同；需要 Docker daemon 可用。
