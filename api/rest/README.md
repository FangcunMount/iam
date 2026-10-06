# REST API 契约

REST契约中AuthZ使用OpenAPI 3.0.3，其余四份使用3.1.0；规范版本与URL版本分别解释。OpenAPI是发布的机器描述，实际路径、安全与响应还需核对[运行时注册](../../internal/apiserver/transport/rest)。当前偏移和生成/验证责任见[契约治理](../../docs/04-接口与SDK/01-REST-gRPC与契约治理.md)。

## 契约文件

| 文件 | 说明 |
| ---- | ---- |
| [authn.v3.yaml](authn.v3.yaml) | v3认证、Challenge、LoginIdentity、Token、JWKS管理和signup；公开JWKS另有根路径/v2别名 |
| [authz.v4.yaml](authz.v4.yaml) | PermissionGrant、Role、Assignment、Resource 管理；继承接口退役，属性模式字段仅兼容合法空值；不含 `Check` |
| [identity.v2.yaml](identity.v2.yaml) | 当前用户、profiles、profile-links 查询；Profile/ProfileLink 创建命令走 gRPC |
| [idp.v2.yaml](idp.v2.yaml) | IDP 健康检查和微信应用配置 |
| [suggest.v2.yaml](suggest.v2.yaml) | 档案联想搜索；查询范围与手机号能力分别授权 |

## 当前路由口径

| 能力 | 路由 |
| ---- | ---- |
| 登录 | `POST /api/v3/authn/login` |
| 登录挑战 | `POST /api/v3/authn/challenges/phone-otp` |
| Token | `POST /api/v3/authn/refresh_token`、`POST /api/v3/authn/logout`、`POST /api/v3/authn/verify` |
| JWKS | `GET /.well-known/jwks.json`、`GET /api/v2/.well-known/jwks.json` |
| AuthN JWKS 管理 | `/api/v3/authn/admin/jwks/keys` 及其 publishable、retire、force-retire、cleanup 子路由 |
| LoginIdentity | `GET /api/v3/authn/login-identities`、`POST /api/v3/authn/login-identities/phone`、`DELETE /api/v3/authn/login-identities/{id}` |
| Signup | `POST /api/v3/authn/signups/wechat-miniprogram` |
| AuthZ 管理面 | `GET /api/v4/authz/health`、`/api/v4/authz/{roles,assignments,grants,role-inheritances,resources}` |
| Identity | `GET/PATCH /api/v2/identity/me`、`GET /api/v2/identity/me/profiles`、`GET/PATCH /api/v2/identity/profiles/{id}`、`GET /api/v2/identity/profile-links` |
| IDP | `/api/v2/idp/health`、`/api/v2/idp/wechat-apps/*` |
| Suggest | `GET /api/v2/suggest/profile` |
| Debug | `/debug/routes`、`/debug/modules`、`/debug/cache-governance/*` |

Identity的当前关系术语是`ProfileLink`。REST使用`/profile-links`；Profile与ProfileLink的创建/撤销由gRPC命令承接，REST保留查询、当前用户资料更新和Profile PATCH。修改能力不能从目录或DTO存在推定，实际输入与披露由[Identity模型](../../docs/02-业务模块/01-Identity/01-领域模型-User-Profile-ProfileLink.md)维护。

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
curl -X POST https://iam.example.com/api/v3/authn/login \
  -H "Content-Type: application/json" \
  -d '{"auth_method":"password","method_payload":{"username":"admin","password":"secret"}}'

curl https://iam.example.com/api/v2/identity/me \
  -H "Authorization: Bearer ${IAM_ACCESS_TOKEN}"

curl https://iam.example.com/api/v4/authz/roles \
  -H "Authorization: Bearer ${IAM_ACCESS_TOKEN}"
```

AuthZ REST v4只承接管理命令和查询，不存在REST `Check`。权限判定使用gRPC v4；下面的示例在仓库根目录读取本地proto，不依赖reflection；需先准备mTLS材料，实际caller还必须满足方法ACL及内容准入：

```bash
grpcurl \
  -import-path api/grpc -proto iam/authz/v4/authz.proto \
  -cacert "$IAM_CA_FILE" -cert "$IAM_CLIENT_CERT_FILE" -key "$IAM_CLIENT_KEY_FILE" \
  -d '{"subject":"user:1024","resource":"qs:answersheet:collection:answersheets","action":"admin_submit"}' \
  iam.example.com:443 iam.authz.v4.AuthorizationService/Check
```

## 验证

```bash
make docs-swagger
make api-validate
go test ./internal/apiserver/transport/rest
```

`make docs-swagger`调用PATH中的swag并规范component ID，不自动更新OpenAPI；`make docs-reset`则全量重建paths/schemas，可能丢失operation级手写元数据，应先审阅转换结果。`make api-validate`优先Docker、后备npx运行Spectral，随后比较已提交描述与选中的注册路由；当前匹配/选择缺口见[治理正文](../../docs/04-接口与SDK/01-REST-gRPC与契约治理.md)，不将该命令写成全合同等价证明。
